package database

import (
	"database/sql"
	"fmt"
	"log"
	"sync"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// Operation types for async database operations
type OperationType int

const (
	OpUserExists OperationType = iota
	OpEmailExists
	OpSaveUser
	OpInit
)

// Request represents a database operation request
type Request struct {
	Op       OperationType
	UserID   int64
	Email    string
	Response chan Response
}

// Response represents a database operation response
type Response struct {
	Exists bool
	Error  error
}

// Database handles all database operations
type Database struct {
	db      *sql.DB
	reqChan chan Request
	wg      sync.WaitGroup
}

// New creates a new Database instance with async operations
func New(databaseURL string) (*Database, error) {
	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test the connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Set connection pool settings for PostgreSQL
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)

	database := &Database{
		db:      db,
		reqChan: make(chan Request, 100), // Buffered channel for async operations
	}

	// Initialize database schema
	if err := database.initDB(); err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	// Start worker goroutine to process database operations
	database.wg.Add(1)
	go database.worker()

	log.Println("PostgreSQL database initialized with async operations")
	return database, nil
}

// initDB creates the necessary database schema
func (d *Database) initDB() error {
	// Create table if not exists
	createTableQuery := `
		CREATE TABLE IF NOT EXISTS users (
			user_id BIGINT PRIMARY KEY,
			email VARCHAR(255) NOT NULL UNIQUE,
			created_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
		)
	`
	_, err := d.db.Exec(createTableQuery)
	if err != nil {
		return fmt.Errorf("failed to create table: %w", err)
	}
	return nil
}

// worker processes database operations asynchronously
func (d *Database) worker() {
	defer d.wg.Done()

	for req := range d.reqChan {
		switch req.Op {
		case OpUserExists:
			exists, err := d.userExistsSync(req.UserID)
			req.Response <- Response{Exists: exists, Error: err}

		case OpEmailExists:
			exists, err := d.emailExistsSync(req.Email)
			req.Response <- Response{Exists: exists, Error: err}

		case OpSaveUser:
			err := d.saveUserEmailSync(req.UserID, req.Email)
			req.Response <- Response{Error: err}
		}
		close(req.Response)
	}
}

// UserExists checks if a user exists asynchronously
func (d *Database) UserExists(userID int64) (bool, error) {
	respChan := make(chan Response, 1)
	req := Request{
		Op:       OpUserExists,
		UserID:   userID,
		Response: respChan,
	}

	d.reqChan <- req
	resp := <-respChan

	return resp.Exists, resp.Error
}

// EmailExists checks if an email already exists asynchronously
func (d *Database) EmailExists(email string) (bool, error) {
	respChan := make(chan Response, 1)
	req := Request{
		Op:       OpEmailExists,
		Email:    email,
		Response: respChan,
	}

	d.reqChan <- req
	resp := <-respChan

	return resp.Exists, resp.Error
}

// SaveUserEmail saves a user's email asynchronously
func (d *Database) SaveUserEmail(userID int64, email string) error {
	respChan := make(chan Response, 1)
	req := Request{
		Op:       OpSaveUser,
		UserID:   userID,
		Email:    email,
		Response: respChan,
	}

	d.reqChan <- req
	resp := <-respChan

	return resp.Error
}

// userExistsSync is the synchronous implementation called by the worker
func (d *Database) userExistsSync(userID int64) (bool, error) {
	var exists bool
	query := "SELECT 1 FROM users WHERE user_id = $1 LIMIT 1"
	err := d.db.QueryRow(query, userID).Scan(&exists)

	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to check user existence: %w", err)
	}

	return true, nil
}

// emailExistsSync is the synchronous implementation called by the worker
func (d *Database) emailExistsSync(email string) (bool, error) {
	var exists bool
	query := "SELECT 1 FROM users WHERE email = $1 LIMIT 1"
	err := d.db.QueryRow(query, email).Scan(&exists)

	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to check email existence: %w", err)
	}

	return true, nil
}

// saveUserEmailSync is the synchronous implementation called by the worker
func (d *Database) saveUserEmailSync(userID int64, email string) error {
	query := "INSERT INTO users (user_id, email) VALUES ($1, $2)"
	_, err := d.db.Exec(query, userID, email)
	if err != nil {
		return fmt.Errorf("failed to save user email: %w", err)
	}
	return nil
}

// Close closes the database connection and waits for pending operations
func (d *Database) Close() error {
	close(d.reqChan)
	d.wg.Wait()
	return d.db.Close()
}
