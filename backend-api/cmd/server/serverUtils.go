package main

import (
	"encoding/json"
	"net/http"
)

func HealthCheckHandler(w http.ResponseWriter, r *http.Request) {
	if err := db.Ping(); err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Database not healthy")
		return
	}
	RespondWithJSON(w, http.StatusOK, map[string]string{"status": "Backend API is healthy"})
}

func RespondWithError(w http.ResponseWriter, code int, message string) {
	RespondWithJSON(w, code, map[string]string{"error": message})
}

func RespondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, _ := json.Marshal(payload)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}