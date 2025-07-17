package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
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


type Log struct {
	ID        string          `json:"id"`
	ProjectID string          `json:"project_id"`
	EventName string          `json:"event_name"`
	Timestamp time.Time       `json:"timestamp"`
	Payload   json.RawMessage `json:"payload"`
}

func logsHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		queryLogsHandler(w, r)
	case "POST":
		logIngestionHandler(w, r)
	default:
		RespondWithError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

type KafkaLogMessage struct {
	ProjectID string          `json:"project_id"`
	Payload   json.RawMessage `json:"payload"`
}

func logIngestionHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	projectID := vars["projectId"]
	apiKey := r.Header.Get("X-API-KEY")

	// Validate API Key
	var dbProjectID string
	err := db.QueryRow("SELECT id FROM projects WHERE id = $1 AND api_key = $2", projectID, apiKey).Scan(&dbProjectID)
	if err != nil {
		if err == sql.ErrNoRows {
			RespondWithError(w, http.StatusUnauthorized, "Invalid API Key for this project")
		} else {
			log.Printf("API Key validation DB error: %v", err)
			RespondWithError(w, http.StatusInternalServerError, "Error validating API key")
		}
		return
	}

	// Read the log payload
	body, err := io.ReadAll(r.Body)
	if err != nil {
		RespondWithError(w, http.StatusBadRequest, "Could not read request body")
		return
	}
	defer r.Body.Close()

	if len(body) == 0 {
		RespondWithError(w, http.StatusBadRequest, "Request body is empty")
		return
	}

	// Create the structured message for Kafka
	kafkaMsg := KafkaLogMessage{
		ProjectID: projectID,
		Payload:   json.RawMessage(body),
	}

	kafkaMsgBytes, err := json.Marshal(kafkaMsg)
	if err != nil {
		log.Printf("Failed to marshal Kafka message: %v", err)
		RespondWithError(w, http.StatusInternalServerError, "Failed to process log")
		return
	}

	// Construct the message for Kafka
	// We use the project ID as the key to ensure logs for the same project go to the same partition
	msg := kafka.Message{
		Key:   []byte(projectID),
		Value: kafkaMsgBytes,
	}

	// Write the message to Kafka
	err = kafkaWriter.WriteMessages(r.Context(), msg)
	if err != nil {
		log.Printf("Failed to write message to Kafka: %v", err)
		RespondWithError(w, http.StatusInternalServerError, "Failed to process log")
		return
	}

	RespondWithJSON(w, http.StatusAccepted, map[string]string{"status": "log accepted"})
}

type AggregatedLog struct {
	EventName  string    `json:"event_name"`
	TotalCount int       `json:"total_count"`
	LastSeen   time.Time `json:"last_seen"`
}

func getAggregatedLogsHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "logsys-session")
	userID, ok := session.Values["user_id"].(string)
	if !ok {
		RespondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	vars := mux.Vars(r)
	projectID := vars["projectId"]

	// Check if user has access to the project
	var accessCheck int
	err := db.QueryRow("SELECT 1 FROM user_project_access WHERE user_id = $1 AND project_id = $2", userID, projectID).Scan(&accessCheck)
	if err != nil {
		if err == sql.ErrNoRows {
			RespondWithError(w, http.StatusForbidden, "You do not have access to this project")
		} else {
			RespondWithError(w, http.StatusInternalServerError, "Database error on access check")
		}
		return
	}

	// Build the aggregation query
	sql := "SELECT event_name, count(), max(event_timestamp) FROM logs WHERE project_id = ? GROUP BY event_name"
	args := []interface{}{projectID}

	// Execute the query
	rows, err := chConn.Query(r.Context(), sql, args...)
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Failed to query aggregated logs from ClickHouse")
		return
	}
	defer rows.Close()

	var aggregatedLogs []AggregatedLog
	for rows.Next() {
		var aggLog AggregatedLog
		var totalCount uint64
		if err := rows.Scan(&aggLog.EventName, &totalCount, &aggLog.LastSeen); err != nil {
			RespondWithError(w, http.StatusInternalServerError, "Failed to scan aggregated log")
			return
		}
		aggLog.TotalCount = int(totalCount)
		aggregatedLogs = append(aggregatedLogs, aggLog)
	}

	RespondWithJSON(w, http.StatusOK, aggregatedLogs)
}

func queryLogsHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "logsys-session")
	userID, ok := session.Values["user_id"].(string)
	if !ok {
		RespondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	vars := mux.Vars(r)
	projectID := vars["projectId"]

	// Check if user has access to the project
	var accessCheck int
	err := db.QueryRow("SELECT 1 FROM user_project_access WHERE user_id = $1 AND project_id = $2", userID, projectID).Scan(&accessCheck)
	if err != nil {
		if err == sql.ErrNoRows {
			RespondWithError(w, http.StatusForbidden, "You do not have access to this project")
		} else {
			RespondWithError(w, http.StatusInternalServerError, "Database error on access check")
		}
		return
	}

	eventName := r.URL.Query().Get("event_name")
	startTime := r.URL.Query().Get("start_time")
	endTime := r.URL.Query().Get("end_time")
	searchKeysStr := r.URL.Query().Get("search_keys")
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	var args []interface{}
	sql := "SELECT log_id, event_name, event_timestamp FROM logs WHERE project_id = ?"
	args = append(args, projectID)

	if eventName != "" {
		sql += " AND event_name = ?"
		args = append(args, eventName)
	}
	if startTime != "" {
		sql += " AND event_timestamp >= ?"
		args = append(args, startTime)
	}
	if endTime != "" {
		sql += " AND event_timestamp <= ?"
		args = append(args, endTime)
	}

	if searchKeysStr != "" {
		searchKeys, err := parseSearchKeys(searchKeysStr)
		if err != nil {
			RespondWithError(w, http.StatusBadRequest, "Invalid search_keys format")
			return
		}
		for k, v := range searchKeys {
			sql += " AND searchable_keys[?] = ?"
			args = append(args, k, v)
		}
	}

	limit := 100
	if limitStr != "" {
		limit, _ = strconv.Atoi(limitStr)
	}
	sql += " LIMIT ?"
	args = append(args, limit)

	if offsetStr != "" {
		offset, _ := strconv.Atoi(offsetStr)
		sql += " OFFSET ?"
		args = append(args, offset)
	}

	rows, err := chConn.Query(r.Context(), sql, args...)
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Failed to query logs from ClickHouse")
		return
	}
	defer rows.Close()

	var logs []Log
	for rows.Next() {
		var logItem Log
		if err := rows.Scan(&logItem.ID, &logItem.EventName, &logItem.Timestamp); err != nil {
			RespondWithError(w, http.StatusInternalServerError, "Failed to scan log from ClickHouse")
			return
		}
		logItem.ProjectID = projectID

		// Fetch the full payload from Cassandra
		var payloadStr string
		if err := cassandra.Query("SELECT payload FROM logs WHERE project_id = ? AND event_timestamp = ? AND log_id = ?",
			logItem.ProjectID, logItem.Timestamp, logItem.ID).Scan(&payloadStr); err != nil {
			if err != gocql.ErrNotFound {
				log.Printf("Failed to query log payload from Cassandra: %v", err)
			}
		} else {
			logItem.Payload = json.RawMessage(payloadStr)
		}

		logs = append(logs, logItem)
	}

	RespondWithJSON(w, http.StatusOK, logs)
}

func parseSearchKeys(searchKeysStr string) (map[string]string, error) {
	searchKeys := make(map[string]string)
	pairs := strings.Split(searchKeysStr, ",")
	for _, pair := range pairs {
		kv := strings.Split(pair, ":")
		if len(kv) != 2 {
			return nil, fmt.Errorf("invalid key-value pair: %s", pair)
		}
		searchKeys[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
	}
	return searchKeys, nil
}

func getLogHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "logsys-session")
	userID, ok := session.Values["user_id"].(string)
	if !ok {
		RespondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	vars := mux.Vars(r)
	projectID := vars["projectId"]
	logID := vars["logId"]

	// Check if user has access to the project
	var accessCheck int
	err := db.QueryRow("SELECT 1 FROM user_project_access WHERE user_id = $1 AND project_id = $2", userID, projectID).Scan(&accessCheck)
	if err != nil {
		if err == sql.ErrNoRows {
			RespondWithError(w, http.StatusForbidden, "You do not have access to this project")
		} else {
			RespondWithError(w, http.StatusInternalServerError, "Database error on access check")
		}
		return
	}

	var log Log
	if err := cassandra.Query("SELECT project_id, event_name, event_timestamp, payload FROM logs WHERE log_id = ?", logID).Scan(&log.ProjectID, &log.EventName, &log.Timestamp, &log.Payload); err != nil {
		if err == gocql.ErrNotFound {
			RespondWithError(w, http.StatusNotFound, "Log not found")
		} else {
			RespondWithError(w, http.StatusInternalServerError, "Failed to query log from Cassandra")
		}
		return
	}
	log.ID = logID

	RespondWithJSON(w, http.StatusOK, log)
}
