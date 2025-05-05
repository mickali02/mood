// internal/data/users.go
package data

import (
	"context" // <-- Added
	"database/sql"
	"errors"
	"strings" // <-- Added
	"time"

	"golang.org/x/crypto/bcrypt" // <-- Added
	// Import validator if needed for password validation methods later
	// "github.com/mickali02/mood/internal/validator"
)

// Define user-specific errors
var (
	ErrDuplicateEmail     = errors.New("duplicate email")
	ErrRecordNotFound     = errors.New("record not found")
	ErrInvalidCredentials = errors.New("invalid credentials")
	// Add more errors as needed (e.g., ErrPasswordMismatch for authentication)
)

// User struct definition
type User struct {
	ID        int64     `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Password  password  `json:"-"` // Use custom password type
	Activated bool      `json:"activated"`
	// Version int `json:"-"` // Optional version for optimistic locking
}

// Custom password type
type password struct {
	plaintext *string // Pointer allows distinguishing between unset and empty
	hash      []byte
}

// Set calculates the bcrypt hash of a plaintext password and stores both
// the hash and the plaintext versions in the password struct.
func (p *password) Set(plaintextPassword string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(plaintextPassword), 12)
	if err != nil {
		return err
	}

	p.plaintext = &plaintextPassword // Store the plaintext (as pointer)
	p.hash = hash                    // Store the hash
	return nil
}

// Matches checks whether the provided plaintext password matches the
// stored hash.
func (p *password) Matches(plaintextPassword string) (bool, error) {
	// Compare the provided plaintext password with the stored hash.
	err := bcrypt.CompareHashAndPassword(p.hash, []byte(plaintextPassword))
	if err != nil {
		// If the error is specifically bcrypt.ErrMismatchedHashAndPassword,
		// it means the password doesn't match. Return false and nil error.
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			return false, nil
		}
		// For any other error (e.g., invalid hash format), return the error.
		return false, err
	}

	// If err is nil, the password matches.
	return true, nil
}

// --- UserModel Struct Definition ---
// UserModel wraps the connection pool.
type UserModel struct {
	DB *sql.DB // Pointer to the database connection pool
}

// --- UserModel Methods ---

// Insert a new user record into the database.
// Note: This version takes a pointer to a User struct.
// It assumes the password hash has already been set using password.Set().
func (m *UserModel) Insert(user *User) error {
	query := `
        INSERT INTO users (name, email, password_hash, activated)
        VALUES ($1, $2, $3, $4)
        RETURNING id, created_at` // Return ID and CreatedAt after insert

	args := []any{
		user.Name,
		user.Email,
		user.Password.hash, // Insert the pre-computed hash
		user.Activated,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Use QueryRowContext to get the returned ID and created_at timestamp.
	// Scan them back into the user struct.
	err := m.DB.QueryRowContext(ctx, query, args...).Scan(&user.ID, &user.CreatedAt)
	if err != nil {
		// Check for unique constraint violation on email
		// Ensure "users_email_key" is your actual constraint name (check with \d users in psql)
		if err.Error() != "" && strings.Contains(err.Error(), `duplicate key value violates unique constraint "users_email_key"`) {
			return ErrDuplicateEmail
		}
		return err // Return other errors directly
	}

	return nil
}

// Authenticate checks if a user exists with the given email and password.
// It returns the user ID on success.
// Authenticate checks if a user exists with the given email and password AND is activated.
// It returns the user ID on success.
func (m *UserModel) Authenticate(email, plaintextPassword string) (int64, error) {
	var id int64
	var hashedPassword []byte

	// Query to find the activated user by email and get their ID and hashed password.
	query := `
        SELECT id, password_hash FROM users
        WHERE email = $1 AND activated = TRUE`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Execute query, scanning the ID and hash into variables.
	err := m.DB.QueryRowContext(ctx, query, email).Scan(&id, &hashedPassword)
	if err != nil {
		// If no row is found (wrong email or user not activated), return ErrInvalidCredentials.
		if errors.Is(err, sql.ErrNoRows) {
			return 0, ErrInvalidCredentials
		}
		// For any other database error, return it directly.
		return 0, err
	}

	// Compare the provided plaintext password with the stored hash.
	err = bcrypt.CompareHashAndPassword(hashedPassword, []byte(plaintextPassword))
	if err != nil {
		// If the hashes don't match, return ErrInvalidCredentials.
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			return 0, ErrInvalidCredentials
		}
		// For any other bcrypt error, return it directly.
		return 0, err
	}

	// If the password is correct, return the user ID and a nil error.
	return id, nil
} // end Authenticate

// Get retrieves a specific user based on their ID.
func (m *UserModel) Get(id int64) (*User, error) {
	if id < 1 {
		return nil, ErrRecordNotFound // Return specific error for invalid ID
	}

	query := `
        SELECT id, created_at, name, email, password_hash, activated
        FROM users
        WHERE id = $1`

	var user User

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Scan the password hash directly into user.Password.hash
	err := m.DB.QueryRowContext(ctx, query, id).Scan(
		&user.ID,
		&user.CreatedAt,
		&user.Name,
		&user.Email,
		&user.Password.hash, // Scan into the hash field of the password struct
		&user.Activated,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrRecordNotFound // Use our specific error
		}
		return nil, err // Return other errors
	}

	// User found and scanned correctly
	return &user, nil
}

// GetByEmail retrieves user details based on email address.
func (m *UserModel) GetByEmail(email string) (*User, error) {
	query := `
        SELECT id, created_at, name, email, password_hash, activated
        FROM users
        WHERE email = $1`

	var user User

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, email).Scan(
		&user.ID,
		&user.CreatedAt,
		&user.Name,
		&user.Email,
		&user.Password.hash,
		&user.Activated,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrRecordNotFound
		}
		return nil, err
	}

	return &user, nil
}

// Update allows changing user details like name, email, password hash, activation status.
// Add optimistic locking later if needed using a version column.
func (m *UserModel) Update(user *User) error {
	query := `
        UPDATE users
        SET name = $1, email = $2, password_hash = $3, activated = $4
        WHERE id = $5
        RETURNING id` // Optionally return something to confirm update

	args := []any{
		user.Name,
		user.Email,
		user.Password.hash, // Assumes hash is updated if password changed
		user.Activated,
		user.ID,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Use QueryRowContext if you are using RETURNING, otherwise use ExecContext
	err := m.DB.QueryRowContext(ctx, query, args...).Scan(&user.ID) // Example scan if returning ID
	if err != nil {
		// Check for duplicate email error on update
		if err.Error() != "" && strings.Contains(err.Error(), `duplicate key value violates unique constraint "users_email_key"`) {
			return ErrDuplicateEmail
		}
		// Check if the record was not found for update (could happen if ID is wrong)
		// QueryRowContext returns sql.ErrNoRows if RETURNING finds no row.
		if errors.Is(err, sql.ErrNoRows) {
			return ErrRecordNotFound
		}
		return err
	}

	return nil
}
