package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"log"
	"github.com/gorilla/mux"
	"github.com/lib/pq"
)


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
		RespondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	switch r.Method {
	case "GET":
		getProjectsHandler(w, userID)
	case "POST":
		createProjectHandler(w, r, userID)
	default:
		RespondWithError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

func getProjectsHandler(w http.ResponseWriter, userID string) {
	rows, err := db.Query(`
		SELECT p.id, p.name, p.searchable_keys, p.log_ttl_seconds, p.owner_id, p.description
		FROM projects p
		JOIN user_project_access upa ON p.id = upa.project_id
		WHERE upa.user_id = $1
	`, userID)
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Database error")
		return
	}
	defer rows.Close()

	projects := []Project{}
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.ID, &p.Name, pq.Array(&p.SearchableKeys), &p.LogTTLSeconds, &p.OwnerID, &p.Description); err != nil {
			// Log the detailed error for debugging
			log.Printf("Scan error: %v", err)
			RespondWithError(w, http.StatusInternalServerError, "Failed to scan project")
			return
		}
		projects = append(projects, p)
	}

	RespondWithJSON(w, http.StatusOK, projects)
}

func createProjectHandler(w http.ResponseWriter, r *http.Request, userID string) {
	var req CreateProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	apiKey, err := generateAPIKey()
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Failed to generate API key")
		return
	}

	var projectID string
	tx, err := db.Begin()
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Failed to start transaction")
		return
	}
	defer tx.Rollback()

	err = tx.QueryRow("INSERT INTO projects (name, api_key, searchable_keys, log_ttl_seconds, owner_id, description) VALUES ($1, $2, $3, $4, $5, $6) RETURNING id",
		req.Name, apiKey, pq.Array(req.SearchableKeys), req.LogTTLSeconds, userID, req.Description).Scan(&projectID)
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Failed to create project")
		return
	}

	_, err = tx.Exec("INSERT INTO user_project_access (user_id, project_id, role) VALUES ($1, $2, 'admin')", userID, projectID)
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Failed to grant project access")
		return
	}

	if err := tx.Commit(); err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Failed to commit transaction")
		return
	}

	RespondWithJSON(w, http.StatusCreated, Project{ID: projectID, Name: req.Name, APIKey: apiKey, SearchableKeys: req.SearchableKeys, LogTTLSeconds: req.LogTTLSeconds, OwnerID: userID, Description: req.Description})
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
		RespondWithError(w, http.StatusUnauthorized, "Unauthorized")
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
			RespondWithError(w, http.StatusForbidden, "You do not have access to this project")
		} else {
			RespondWithError(w, http.StatusInternalServerError, "Database error on access check")
		}
		return
	}

	// If user has access, get the API key
	err = db.QueryRow("SELECT api_key FROM projects WHERE id = $1", projectID).Scan(&apiKey)
	if err != nil {
		if err == sql.ErrNoRows {
			// This should not happen if the access check passed, but handle it just in case
			RespondWithError(w, http.StatusNotFound, "Project not found")
		} else {
			RespondWithError(w, http.StatusInternalServerError, "Database error on API key fetch")
		}
		return
	}

	RespondWithJSON(w, http.StatusOK, map[string]string{"api_key": apiKey})
}