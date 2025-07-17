package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/gocql/gocql"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/segmentio/kafka-go"
)

var (
	db          *sql.DB
	store       *sessions.CookieStore
	kafkaWriter *kafka.Writer
	chConn      clickhouse.Conn
	cassandra   *gocql.Session
)

type User struct {
	ID       string `json:"id"`
	Username string `json:"username"`
}

type Project struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	APIKey         string   `json:"api_key,omitempty"`
	SearchableKeys []string `json:"searchable_keys,omitempty"`
	LogTTLSeconds  int      `json:"log_ttl_seconds"`
	OwnerID        string   `json:"owner_id"`
	Description    string   `json:"description,omitempty"`
}

func main() {
	log.Println("Starting Backend API server...")
	var err error

	sessionKey := os.Getenv("SESSION_KEY")
	if sessionKey == "" {
		log.Fatal("SESSION_KEY not set.")
	}
	store = sessions.NewCookieStore([]byte(sessionKey))
	store.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   86400 * 7, // 7 days
		HttpOnly: true,
	}

	cockroachDBURL := os.Getenv("COCKROACHDB_URL")
	if cockroachDBURL == "" {
		log.Fatal("COCKROACHDB_URL not set.")
	}

	maxRetries := 10
	for i := 0; i < maxRetries; i++ {
		db, err = sql.Open("pgx", cockroachDBURL)
		if err == nil {
			err = db.Ping()
			if err == nil {
				break
			}
		}
		log.Printf("Failed to connect to CockroachDB (attempt %d/%d): %v. Retrying in 5 seconds...", i+1, maxRetries, err)
		time.Sleep(5 * time.Second)
	}
	if err != nil {
		log.Fatalf("Failed to connect to CockroachDB after %d retries: %v", maxRetries, err)
	}
	log.Println("Successfully connected to CockroachDB.")

	kafkaBroker := os.Getenv("KAFKA_BROKER")
	if kafkaBroker == "" {
		kafkaBroker = "kafka:9092"
	}
	kafkaWriter = &kafka.Writer{
		Addr:     kafka.TCP(kafkaBroker),
		Topic:    "log-events",
		Balancer: &kafka.LeastBytes{},
	}
	log.Printf("Kafka writer configured for broker at %s", kafkaBroker)

	clickhouseHost := os.Getenv("CLICKHOUSE_HOST")
	if clickhouseHost == "" {
		clickhouseHost = "clickhouse"
	}
	chConn, err = clickhouse.Open(&clickhouse.Options{
		Addr: []string{clickhouseHost + ":9000"},
		Auth: clickhouse.Auth{
			Database: "default",
		},
		DialTimeout:  time.Second,
		MaxOpenConns: 10,
		MaxIdleConns: 5,
	})
	if err != nil {
		log.Fatalf("Failed to connect to ClickHouse: %v", err)
	}
	log.Println("Successfully connected to ClickHouse.")

	cassandraHost := os.Getenv("CASSANDRA_HOSTS")
	if cassandraHost == "" {
		cassandraHost = "cassandra1"
	}
	cluster := gocql.NewCluster(cassandraHost)
	cluster.Keyspace = "logsystem"
	cassandra, err = cluster.CreateSession()
	if err != nil {
		log.Fatalf("Failed to connect to Cassandra: %v", err)
	}
	log.Println("Successfully connected to Cassandra.")

	r := mux.NewRouter()
	r.HandleFunc("/health", HealthCheckHandler).Methods("GET")

	apiRouter := r.PathPrefix("/api").Subrouter()
	apiRouter.HandleFunc("/users", createUserHandler).Methods("POST")
	apiRouter.HandleFunc("/auth/login", loginHandler).Methods("POST")
	apiRouter.HandleFunc("/auth/logout", logoutHandler).Methods("POST")
	apiRouter.HandleFunc("/auth/me", meHandler).Methods("GET")
	apiRouter.HandleFunc("/projects", projectsHandler).Methods("GET", "POST")
	apiRouter.HandleFunc("/projects/{projectId}/apikey", getProjectAPIKeyHandler).Methods("GET")
	apiRouter.HandleFunc("/projects/{projectId}/logs", logsHandler).Methods("GET", "POST")
	apiRouter.HandleFunc("/projects/{projectId}/logs/aggregated", getAggregatedLogsHandler).Methods("GET")
	apiRouter.HandleFunc("/projects/{projectId}/logs/{logId}", getLogHandler).Methods("GET")

	port := os.Getenv("BACKEND_API_PORT")
	if port == "" {
		port = "8081"
	}

	srv := &http.Server{
		Handler:      r,
		Addr:         ":" + port,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	log.Printf("Backend API listening on port %s", port)
	log.Fatal(srv.ListenAndServe())
}