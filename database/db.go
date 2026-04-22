package database

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type Database struct {
	db *sql.DB
}

func New(databaseURL string) (*Database, error) {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)

	database := &Database{db: db}

	if err := database.initDB(); err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	log.Println("PostgreSQL database initialized")
	return database, nil
}

func (d *Database) initDB() error {
	_, err := d.db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			user_id    BIGINT PRIMARY KEY,
			username   VARCHAR(255),
			email      VARCHAR(255) NOT NULL UNIQUE,
			created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create table: %w", err)
	}
	return nil
}

func (d *Database) UserExists(userID int64) (bool, error) {
	var exists bool
	err := d.db.QueryRow("SELECT 1 FROM users WHERE user_id = $1 LIMIT 1", userID).Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to check user existence: %w", err)
	}
	return true, nil
}

func (d *Database) EmailExists(email string) (bool, error) {
	var exists bool
	err := d.db.QueryRow("SELECT 1 FROM users WHERE email = $1 LIMIT 1", email).Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to check email existence: %w", err)
	}
	return true, nil
}

func (d *Database) SaveUserEmail(userID int64, username, email string) error {
	_, err := d.db.Exec(
		"INSERT INTO users (user_id, username, email) VALUES ($1, $2, $3)",
		userID, username, email,
	)
	if err != nil {
		return fmt.Errorf("failed to save user email: %w", err)
	}
	return nil
}

func (d *Database) Close() error {
	return d.db.Close()
}
