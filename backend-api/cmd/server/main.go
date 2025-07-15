package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	_ "github.com/jackc/pgx/v5/stdlib"
	"golang.org/x/crypto/bcrypt"
)

var (
	db    *sql.DB
	store *sessions.CookieStore
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

	r := mux.NewRouter()
	r.HandleFunc("/health", healthCheckHandler).Methods("GET")
	// Add user and project handlers here later

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

func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	if err := db.Ping(); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Database not healthy")
		return
	}
	respondWithJSON(w, http.StatusOK, map[string]string{"status": "Backend API is healthy"})
}

func respondWithError(w http.ResponseWriter, code int, message string) {
	respondWithJSON(w, code, map[string]string{"error": message})
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, _ := json.Marshal(payload)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}

func hashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	return string(bytes), err
}

func checkPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}
