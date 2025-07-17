package main

import (
	"encoding/json"
	"net/http"
	"golang.org/x/crypto/bcrypt"
	"database/sql"
)

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
		RespondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	hashedPassword, err := hashPassword(req.Password)
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Failed to hash password")
		return
	}

	var userID string
	err = db.QueryRow("INSERT INTO users (username, hashed_password, full_name, email) VALUES ($1, $2, $3, $4) RETURNING id",
		req.Username, hashedPassword, req.FullName, req.Email).Scan(&userID)
	if err != nil {
		RespondWithError(w, http.StatusInternalServerError, "Failed to create user")
		return
	}

	RespondWithJSON(w, http.StatusCreated, User{ID: userID, Username: req.Username})
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	var userID, hashedPassword string
	err := db.QueryRow("SELECT id, hashed_password FROM users WHERE username = $1", req.Username).Scan(&userID, &hashedPassword)
	if err != nil {
		if err == sql.ErrNoRows {
			RespondWithError(w, http.StatusUnauthorized, "Invalid username or password")
		} else {
			RespondWithError(w, http.StatusInternalServerError, "Database error")
		}
		return
	}

	if !checkPasswordHash(req.Password, hashedPassword) {
		RespondWithError(w, http.StatusUnauthorized, "Invalid username or password")
		return
	}

	session, _ := store.Get(r, "logsys-session")
	session.Values["user_id"] = userID
	session.Save(r, w)

	RespondWithJSON(w, http.StatusOK, map[string]string{"status": "success"})
}

func logoutHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "logsys-session")
	session.Options.MaxAge = -1
	session.Save(r, w)
	RespondWithJSON(w, http.StatusOK, map[string]string{"status": "success"})
}

func meHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := store.Get(r, "logsys-session")
	userID, ok := session.Values["user_id"].(string)
	if !ok {
		RespondWithError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	var user User
	err := db.QueryRow("SELECT id, username FROM users WHERE id = $1", userID).Scan(&user.ID, &user.Username)
	if err != nil {
		if err == sql.ErrNoRows {
			RespondWithError(w, http.StatusNotFound, "User not found")
		} else {
			RespondWithError(w, http.StatusInternalServerError, "Database error")
		}
		return
	}

	RespondWithJSON(w, http.StatusOK, user)
}