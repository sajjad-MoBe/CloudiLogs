package main

import (
	"context"
	"fmt"
	"log"


	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/gocql/gocql"
)

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
