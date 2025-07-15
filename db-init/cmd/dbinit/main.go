package main

import (
	"context"
	"database/sql"
	"log"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

const schema = `
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username STRING(255) UNIQUE NOT NULL,
    hashed_password BYTES NOT NULL,
    full_name STRING(255),
    email STRING(255) UNIQUE,
    is_active BOOL DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE IF NOT EXISTS projects (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name STRING(255) NOT NULL,
    api_key STRING(64) UNIQUE NOT NULL,
    searchable_keys STRING[],
    log_ttl_seconds INT NOT NULL,
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    description STRING,
    is_active BOOL DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now(),
    UNIQUE (owner_id, name)
);
CREATE INDEX IF NOT EXISTS projects_api_key_idx ON projects (api_key);
CREATE INDEX IF NOT EXISTS projects_owner_id_idx ON projects (owner_id);

CREATE TABLE IF NOT EXISTS user_project_access (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    assigned_at TIMESTAMPTZ DEFAULT now(),
    PRIMARY KEY (user_id, project_id)
);
`

func main() {
	log.Println("Starting CockroachDB Initializer...")

	cockroachDBURL := os.Getenv("COCKROACHDB_URL")
	if cockroachDBURL == "" {
		log.Fatal("COCKROACHDB_URL environment variable is not set.")
	}

	var db *sql.DB
	var err error
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
	defer db.Close()

	log.Println("Successfully connected to CockroachDB.")

	log.Println("Creating database if it doesn't exist...")
	if _, err := db.ExecContext(context.Background(), "CREATE DATABASE IF NOT EXISTS logsdb"); err != nil {
		log.Fatalf("Failed to create database: %v", err)
	}
	log.Println("Database created successfully (or already existed).")

	log.Println("Applying database schema...")
	if _, err := db.ExecContext(context.Background(), schema); err != nil {
		log.Fatalf("Failed to apply schema: %v", err)
	}
	log.Println("Database schema applied successfully (or already existed).")

	log.Println("CockroachDB Initialization complete.")
}
