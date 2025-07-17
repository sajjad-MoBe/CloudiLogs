package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/gocql/gocql"
	// "github.com/segmentio/kafka-go"
)

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
			}
		}
		log.Printf("ClickHouse connection failed: %v. Retrying in %v...", err, retryInterval)
		time.Sleep(retryInterval)
	}
	log.Fatalf("Could not connect to ClickHouse after %d attempts: %v", maxRetries, err)
	return nil
}
