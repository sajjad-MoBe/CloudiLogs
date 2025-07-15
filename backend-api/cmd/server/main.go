package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/lib/pq"
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

	apiRouter := r.PathPrefix("/api").Subrouter()
	apiRouter.HandleFunc("/users", createUserHandler).Methods("POST")
	apiRouter.HandleFunc("/auth/login", loginHandler).Methods("POST")
	apiRouter.HandleFunc("/auth/logout", logoutHandler).Methods("POST")
	apiRouter.HandleFunc("/auth/me", meHandler).Methods("GET")
	apiRouter.HandleFunc("/projects", projectsHandler).Methods("GET", "POST")
	apiRouter.HandleFunc("/projects/{projectId}/apikey", getProjectAPIKeyHandler).Methods("GET")

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

type CreateUserRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	FullName string `json:"full_name"`
	Email    string `json:"email"`
}

func createUserHandler(w http.ResponseWriter, r *http.Request) {
	var req CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	hashedPassword, err := hashPassword(req.Password)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to hash password")
		return
	}

	var userID string
	err = db.QueryRow("INSERT INTO users (username, hashed_password, full_name, email) VALUES ($1, $2, $3, $4) RETURNING id",
		req.Username, hashedPassword, req.FullName, req.Email).Scan(&userID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to create user")
		return
	}

	respondWithJSON(w, http.StatusCreated, User{ID: userID, Username: req.Username})
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	var userID, hashedPassword string
	err := db.QueryRow("SELECT id, hashed_password FROM users WHERE username = $1", req.Username).Scan(&userID, &hashedPassword)
	if err != nil {
		if err == sql.ErrNoRows {
			respondWithError(w, http.StatusUnauthorized, "Invalid username or password")
		} else {
			respondWithError(w, http.StatusInternalServerError, "Database error")
		}
		return
	}

	if !checkPasswordHash(req.Password, hashedPassword) {
		respondWithError(w, http.StatusUnauthorized, "Invalid username or password")
		return
	}

	session, _ := store.Get(r, "logsys-session")
	session.Values["user_id"] = userID
	session.Save(r, w)

	respondWithJSON(w, http.StatusOK, map[string]string{"status": "success"})
}

func logoutHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "logsys-session")
	session.Options.MaxAge = -1
	session.Save(r, w)
	respondWithJSON(w, http.StatusOK, map[string]string{"status": "success"})
}

func meHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "logsys-session")
	userID, ok := session.Values["user_id"].(string)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var user User
	err := db.QueryRow("SELECT id, username FROM users WHERE id = $1", userID).Scan(&user.ID, &user.Username)
	if err != nil {
		if err == sql.ErrNoRows {
			respondWithError(w, http.StatusNotFound, "User not found")
		} else {
			respondWithError(w, http.StatusInternalServerError, "Database error")
		}
		return
	}

	respondWithJSON(w, http.StatusOK, user)
}

type CreateProjectRequest struct {
	Name           string   `json:"name"`
	SearchableKeys []string `json:"searchable_keys"`
	LogTTLSeconds  int      `json:"log_ttl_seconds"`
	Description    string   `json:"description"`
}

func projectsHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "logsys-session")
	userID, ok := session.Values["user_id"].(string)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	switch r.Method {
	case "GET":
		getProjectsHandler(w, r, userID)
	case "POST":
		createProjectHandler(w, r, userID)
	default:
		respondWithError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

func getProjectsHandler(w http.ResponseWriter, r *http.Request, userID string) {
	rows, err := db.Query(`
		SELECT p.id, p.name, p.searchable_keys, p.log_ttl_seconds, p.owner_id, p.description
		FROM projects p
		JOIN user_project_access upa ON p.id = upa.project_id
		WHERE upa.user_id = $1
	`, userID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Database error")
		return
	}
	defer rows.Close()

	projects := []Project{}
	for rows.Next() {
		var p Project
		var searchableKeys pq.StringArray
		if err := rows.Scan(&p.ID, &p.Name, &searchableKeys, &p.LogTTLSeconds, &p.OwnerID, &p.Description); err != nil {
			respondWithError(w, http.StatusInternalServerError, "Failed to scan project")
			return
		}
		p.SearchableKeys = []string(searchableKeys)
		projects = append(projects, p)
	}

	respondWithJSON(w, http.StatusOK, projects)
}

func createProjectHandler(w http.ResponseWriter, r *http.Request, userID string) {
	var req CreateProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	apiKey, err := generateAPIKey()
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to generate API key")
		return
	}

	var projectID string
	tx, err := db.Begin()
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to start transaction")
		return
	}
	defer tx.Rollback()

	err = tx.QueryRow("INSERT INTO projects (name, api_key, searchable_keys, log_ttl_seconds, owner_id, description) VALUES ($1, $2, $3, $4, $5, $6) RETURNING id",
		req.Name, apiKey, req.SearchableKeys, req.LogTTLSeconds, userID, req.Description).Scan(&projectID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to create project")
		return
	}

	_, err = tx.Exec("INSERT INTO user_project_access (user_id, project_id, role) VALUES ($1, $2, $3)", userID, projectID, "admin")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to grant project access")
		return
	}

	if err := tx.Commit(); err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to commit transaction")
		return
	}

	respondWithJSON(w, http.StatusCreated, Project{ID: projectID, Name: req.Name, APIKey: apiKey, SearchableKeys: req.SearchableKeys, LogTTLSeconds: req.LogTTLSeconds, OwnerID: userID, Description: req.Description})
}

func generateAPIKey() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func getProjectAPIKeyHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "logsys-session")
	userID, ok := session.Values["user_id"].(string)
	if !ok {
		respondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	vars := mux.Vars(r)
	projectID := vars["projectId"]

	var apiKey string
	// Check if user has access to the project
	var accessCheck int
	err := db.QueryRow("SELECT 1 FROM user_project_access WHERE user_id = $1 AND project_id = $2", userID, projectID).Scan(&accessCheck)
	if err != nil {
		if err == sql.ErrNoRows {
			respondWithError(w, http.StatusForbidden, "You do not have access to this project")
		} else {
			respondWithError(w, http.StatusInternalServerError, "Database error on access check")
		}
		return
	}

	// If user has access, get the API key
	err = db.QueryRow("SELECT api_key FROM projects WHERE id = $1", projectID).Scan(&apiKey)
	if err != nil {
		if err == sql.ErrNoRows {
			// This should not happen if the access check passed, but handle it just in case
			respondWithError(w, http.StatusNotFound, "Project not found")
		} else {
			respondWithError(w, http.StatusInternalServerError, "Database error on API key fetch")
		}
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"api_key": apiKey})
}
