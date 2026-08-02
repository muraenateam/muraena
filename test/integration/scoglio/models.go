package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"log"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"
)

// cookieSecret is the HMAC signing key for self-contained session cookies.
// Fixed key so tokens survive Scoglio restarts — this is a test app, not production.
var cookieSecret = []byte("scoglio-test-hmac-secret-key-2024")

// User represents a registered user.
type User struct {
	ID        int64
	Email     string
	Password  string // bcrypt hash
	Role      string
	CreatedAt time.Time
}

const schema = `
CREATE TABLE IF NOT EXISTS users (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	email TEXT UNIQUE NOT NULL,
	password TEXT NOT NULL,
	role TEXT NOT NULL DEFAULT 'user',
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
`

// initDB opens the SQLite database and creates the users table.
func initDB(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	// Enable WAL mode for concurrent reads.
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set WAL mode: %w", err)
	}

	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("create schema: %w", err)
	}

	return db, nil
}

// seedUsers creates default users if the users table is empty.
func seedUsers(db *sql.DB) error {
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count); err != nil {
		return fmt.Errorf("count users: %w", err)
	}
	if count > 0 {
		return nil
	}

	seeds := []struct {
		Email    string
		Password string
		Role     string
	}{
		{"admin@scoglio.local", "Admin123!", "admin"},
		{"user@scoglio.local", "User456!", "user"},
	}

	for _, s := range seeds {
		hash, err := bcrypt.GenerateFromPassword([]byte(s.Password), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("hash password for %s: %w", s.Email, err)
		}
		_, err = db.Exec("INSERT INTO users (email, password, role) VALUES (?, ?, ?)",
			s.Email, string(hash), s.Role)
		if err != nil {
			return fmt.Errorf("insert user %s: %w", s.Email, err)
		}
		log.Printf("Seeded user: %s (role: %s)", s.Email, s.Role)
	}

	return nil
}

// createUser inserts a new user with a bcrypt-hashed password.
func createUser(db *sql.DB, email, password, role string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	_, err = db.Exec("INSERT INTO users (email, password, role) VALUES (?, ?, ?)",
		email, string(hash), role)
	return err
}

// getUserByEmail retrieves a user by email.
func getUserByEmail(db *sql.DB, email string) (*User, error) {
	u := &User{}
	err := db.QueryRow("SELECT id, email, password, role, created_at FROM users WHERE email = ?", email).
		Scan(&u.ID, &u.Email, &u.Password, &u.Role, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	return u, nil
}

// getUserByID retrieves a user by ID.
func getUserByID(db *sql.DB, id int64) (*User, error) {
	u := &User{}
	err := db.QueryRow("SELECT id, email, password, role, created_at FROM users WHERE id = ?", id).
		Scan(&u.ID, &u.Email, &u.Password, &u.Role, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	return u, nil
}

// signToken creates an HMAC-signed self-contained token encoding user info.
// Format: base64(email|role) + "." + hex(hmac-sha256)
// No server-side session state needed — the cookie IS the session.
func signToken(email, role string) string {
	payload := base64.RawURLEncoding.EncodeToString([]byte(email + "|" + role))
	mac := hmac.New(sha256.New, cookieSecret)
	mac.Write([]byte(payload))
	sig := hex.EncodeToString(mac.Sum(nil))
	return payload + "." + sig
}

// verifyToken validates an HMAC-signed token and returns (email, role) if valid.
// Returns empty strings if the token is invalid or tampered.
func verifyToken(token string) (email, role string) {
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 {
		return "", ""
	}
	payload, sigHex := parts[0], parts[1]

	// Verify HMAC.
	mac := hmac.New(sha256.New, cookieSecret)
	mac.Write([]byte(payload))
	expectedSig := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(sigHex), []byte(expectedSig)) {
		return "", ""
	}

	// Decode payload.
	decoded, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return "", ""
	}
	fields := strings.SplitN(string(decoded), "|", 2)
	if len(fields) != 2 {
		return "", ""
	}
	return fields[0], fields[1]
}

// randomToken generates a 32-byte hex token (used for NECRO_BRO and JSESSIONID).
func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// createSessionTokens generates the 3 cookie values for a user.
// MURAENA_SESS is a self-contained signed token (no DB lookup needed).
// NECRO_BRO and JSESSIONID are random tokens (not used for auth).
func createSessionTokens(user *User) (muraenaSess, necroBro, jsessionID string, err error) {
	muraenaSess = signToken(user.Email, user.Role)

	necroBro, err = randomToken()
	if err != nil {
		return "", "", "", fmt.Errorf("generate NECRO_BRO token: %w", err)
	}
	jsessionID, err = randomToken()
	if err != nil {
		return "", "", "", fmt.Errorf("generate JSESSIONID token: %w", err)
	}

	return muraenaSess, necroBro, jsessionID, nil
}
