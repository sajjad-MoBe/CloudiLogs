package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"strings"
	"time"

	// "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/gocql/gocql"
	"github.com/segmentio/kafka-go"
)

const (
	cassandraKeyspace = "logsystem"
	cassandraTable    = "logs"
	clickhouseTable   = "logs"
	kafkaTopic        = "log-events"
	maxRetries        = 10
	retryInterval     = 5 * time.Second
)

// KafkaLogMessage defines the structure of the message received from Kafka.
type KafkaLogMessage struct {
	ProjectID string          `json:"project_id"`
	Payload   json.RawMessage `json:"payload"`
}

// LogPayload defines the structure of the actual log data.
type LogPayload struct {
	Name           string            `json:"name"`
	Timestamp      time.Time         `json:"timestamp"`
	SearchableKeys map[string]string `json:"searchable_keys"`
	FullPayload    json.RawMessage   `json:"full_payload"`
}

func main() {
	log.Println("Starting Log Processor Service...")

	// --- Cassandra Setup ---
	session := connectToCassandra()
	initCassandra(session)
	session.Close()

	cluster := gocql.NewCluster(strings.Split(os.Getenv("CASSANDRA_HOSTS"), ",")...)
	cluster.Keyspace = cassandraKeyspace
	session, err := cluster.CreateSession()
	if err != nil {
		log.Fatalf("Failed to connect to Cassandra with keyspace: %v", err)
	}
	defer session.Close()

	log.Println("Cassandra connection and schema verified.")

	// --- ClickHouse Setup ---
	chConn := connectToClickHouse()
	defer chConn.Close()
	initClickHouse(chConn)
	log.Println("ClickHouse connection and schema verified.")

	// --- Kafka Setup ---
	kafkaBroker := os.Getenv("KAFKA_BROKER")
	if kafkaBroker == "" {
		log.Fatal("KAFKA_BROKER environment variable is not set.")
	}
	log.Printf("Connecting to Kafka broker at %s", kafkaBroker)
	kafkaReader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  []string{kafkaBroker},
		Topic:    kafkaTopic,
		GroupID:  "log-processors",
		MinBytes: 10e3, // 10KB
		MaxBytes: 10e6, // 10MB
	})
	defer kafkaReader.Close()
	log.Println("Kafka reader created.")

	// --- Main Processing Loop ---
	log.Println("Starting log processing loop...")
	for {
		m, err := kafkaReader.ReadMessage(context.Background())
		if err != nil {
			log.Printf("Error reading message from Kafka: %v", err)
			continue
		}

		var kafkaMsg KafkaLogMessage
		if err := json.Unmarshal(m.Value, &kafkaMsg); err != nil {
			log.Printf("Error unmarshalling Kafka message: %v. Message: %s", err, string(m.Value))
			continue
		}

		var logPayload LogPayload
		if err := json.Unmarshal(kafkaMsg.Payload, &logPayload); err != nil {
			log.Printf("Error unmarshalling log payload: %v. Payload: %s", err, string(kafkaMsg.Payload))
			continue
		}

		logID := gocql.TimeUUID()

		// Insert into Cassandra
		if err := session.Query(
			`INSERT INTO logs (project_id, event_timestamp, log_id, payload) VALUES (?, ?, ?, ?)`,
			kafkaMsg.ProjectID, logPayload.Timestamp, logID, string(logPayload.FullPayload),
		).Exec(); err != nil {
			log.Printf("Failed to insert log into Cassandra: %v", err)
			continue
		}

		// Insert into ClickHouse
		if err := chConn.Exec(context.Background(),
			`INSERT INTO logs (project_id, event_name, event_timestamp, log_id, searchable_keys) VALUES (?, ?, ?, ?, ?)`,
			kafkaMsg.ProjectID, logPayload.Name, logPayload.Timestamp, logID.String(), logPayload.SearchableKeys,
		); err != nil {
			log.Printf("Failed to insert log into ClickHouse: %v", err)
			continue
		}

		log.Printf("Processed log for project %s with ID %s", kafkaMsg.ProjectID, logID)
	}
}
