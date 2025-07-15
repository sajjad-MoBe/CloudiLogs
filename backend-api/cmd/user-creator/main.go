package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	username := flag.String("username", "", "Username for the new user")
	password := flag.String("password", "", "Password for the new user")
	fullName := flag.String("fullname", "", "Full name of the new user")
	email := flag.String("email", "", "Email of the new user")
	flag.Parse()

	if *username == "" || *password == "" || *email == "" {
		flag.Usage()
		os.Exit(1)
	}

	cockroachDBURL := os.Getenv("COCKROACHDB_URL")
	if cockroachDBURL == "" {
		log.Fatal("COCKROACHDB_URL not set.")
	}

	db, err := sql.Open("pgx", cockroachDBURL)
	if err != nil {
		log.Fatalf("Failed to connect to CockroachDB: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping CockroachDB: %v", err)
	}

	hashedPassword, err := hashPassword(*password)
	if err != nil {
		log.Fatalf("Failed to hash password: %v", err)
	}

	var userID string
	err = db.QueryRow("INSERT INTO users (username, hashed_password, full_name, email) VALUES ($1, $2, $3, $4) RETURNING id",
		*username, hashedPassword, *fullName, *email).Scan(&userID)
	if err != nil {
		log.Fatalf("Failed to create user: %v", err)
	}

	fmt.Printf("User created successfully with ID: %s\n", userID)
}

func hashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	return string(bytes), err
}
