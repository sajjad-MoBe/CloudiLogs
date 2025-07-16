package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
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

// LogPayload defines the structure of the log data received from Kafka.
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

		projectID := string(m.Key)
		var payload LogPayload
		if err := json.Unmarshal(m.Value, &payload); err != nil {
			log.Printf("Error unmarshalling log payload: %v. Message: %s", err, string(m.Value))
			continue
		}

		logID := gocql.TimeUUID()

		// Insert into Cassandra
		if err := session.Query(
			`INSERT INTO logs (project_id, event_timestamp, log_id, payload) VALUES (?, ?, ?, ?)`,
			projectID, payload.Timestamp, logID, string(m.Value),
		).Exec(); err != nil {
			log.Printf("Failed to insert log into Cassandra: %v", err)
			// Decide on error handling: continue, retry, or dead-letter queue
			continue
		}

		// Insert into ClickHouse
		if err := chConn.Exec(context.Background(),
			`INSERT INTO logs (project_id, event_name, event_timestamp, log_id, searchable_keys) VALUES (?, ?, ?, ?, ?)`,
			projectID, payload.Name, payload.Timestamp, logID.String(), payload.SearchableKeys,
		); err != nil {
			log.Printf("Failed to insert log into ClickHouse: %v", err)
			// Decide on error handling
			continue
		}

		log.Printf("Processed log for project %s with ID %s", projectID, logID)
	}
}

func connectToCassandra() *gocql.Session {
	cassandraHosts := strings.Split(os.Getenv("CASSANDRA_HOSTS"), ",")
	if len(cassandraHosts) == 0 || cassandraHosts[0] == "" {
		log.Fatal("CASSANDRA_HOSTS environment variable is not set")
	}

	var session *gocql.Session
	var err error

	for i := 0; i < maxRetries; i++ {
		log.Printf("Connecting to Cassandra cluster at %v (attempt %d/%d)", cassandraHosts, i+1, maxRetries)
		cluster := gocql.NewCluster(cassandraHosts...)
		cluster.Keyspace = "system"
		session, err = cluster.CreateSession()
		if err == nil {
			log.Println("Successfully connected to Cassandra.")
			return session
		}
		log.Printf("Cassandra connection failed: %v. Retrying in %v...", err, retryInterval)
		time.Sleep(retryInterval)
	}
	log.Fatalf("Could not connect to Cassandra after %d attempts: %v", maxRetries, err)
	return nil
}

func connectToClickHouse() clickhouse.Conn {
	clickhouseHost := os.Getenv("CLICKHOUSE_HOST")
	if clickhouseHost == "" {
		log.Fatal("CLICKHOUSE_HOST environment variable is not set")
	}
	clickhouseAddr := fmt.Sprintf("%s:9000", clickhouseHost)

	var conn clickhouse.Conn
	var err error

	for i := 0; i < maxRetries; i++ {
		log.Printf("Connecting to ClickHouse at %s (attempt %d/%d)", clickhouseAddr, i+1, maxRetries)
		conn, err = clickhouse.Open(&clickhouse.Options{
			Addr:        []string{clickhouseAddr},
			Auth:        clickhouse.Auth{Database: "default"},
			DialTimeout: time.Second * 5,
		})
		if err == nil {
			if err := conn.Ping(context.Background()); err == nil {
				log.Println("Successfully connected to ClickHouse.")
				return conn
			} else {
				err = fmt.Errorf("ping failed: %w", err)
			}
		}
		log.Printf("ClickHouse connection failed: %v. Retrying in %v...", err, retryInterval)
		time.Sleep(retryInterval)
	}
	log.Fatalf("Could not connect to ClickHouse after %d attempts: %v", maxRetries, err)
	return nil
}

func initCassandra(session *gocql.Session) {
	log.Println("Initializing Cassandra schema...")
	// Create Keyspace
	err := session.Query(fmt.Sprintf(`
		CREATE KEYSPACE IF NOT EXISTS %s
		WITH replication = {'class': 'SimpleStrategy', 'replication_factor': '3'}
	`, cassandraKeyspace)).Exec()
	if err != nil {
		log.Fatalf("Failed to create Cassandra keyspace: %v", err)
	}

	// Create Table
	err = session.Query(fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s.%s (
			project_id text,
			event_timestamp timestamp,
			log_id timeuuid,
			payload text,
			PRIMARY KEY (project_id, event_timestamp, log_id)
		) WITH CLUSTERING ORDER BY (event_timestamp DESC, log_id DESC)
	`, cassandraKeyspace, cassandraTable)).Exec()
	if err != nil {
		log.Fatalf("Failed to create Cassandra table: %v", err)
	}
	log.Println("Cassandra schema initialized successfully.")
}

func initClickHouse(conn clickhouse.Conn) {
	log.Println("Initializing ClickHouse schema...")
	err := conn.Exec(context.Background(), fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			project_id String,
			event_name String,
			event_timestamp DateTime,
			log_id UUID,
			searchable_keys Map(String, String),
			received_at DateTime DEFAULT now()
		) ENGINE = MergeTree()
		PARTITION BY toYYYYMM(event_timestamp)
		ORDER BY (project_id, event_name, event_timestamp)
	`, clickhouseTable))

	if err != nil {
		log.Fatalf("Failed to create ClickHouse table: %v", err)
	}
	log.Println("ClickHouse schema initialized successfully.")
}
