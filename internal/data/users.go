// mood/internal/data/users.go
package data

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/mickali02/mood/internal/validator"
	"golang.org/x/crypto/bcrypt"
)

// Define user-specific errors for consistent error handling across the application.
// Standardized errors for common user-related issues like duplicate email or invalid login.
var (
	ErrDuplicateEmail     = errors.New("duplicate email")     // Error when trying to register an email already in use.
	ErrRecordNotFound     = errors.New("record not found")    // Error when a user record cannot be found.
	ErrInvalidCredentials = errors.New("invalid credentials") // Error for failed login attempts.
	ErrEditConflict       = errors.New("edit conflict")       // Placeholder for optimistic locking
)

// User struct defines the structure of a user, mapping to the 'users' database table.
// JSON tags control serialization; `json:"-"` hides a field (like Password) from JSON output.
// This is our User model, representing a user in the system. Note the custom 'password' type for security.
type User struct {
	ID        int64     `json:"id"`         // Unique identifier (Primary Key).
	CreatedAt time.Time `json:"created_at"` // Timestamp of user creation.
	Name      string    `json:"name"`       // User's display name.
	Email     string    `json:"email"`      // User's email address (used for login, must be unique).
	Password  password  `json:"-"`          // Custom type to handle password hashing and comparison.
	Activated bool      `json:"activated"`  // Flag indicating if the user account is active.
}

// password is a custom struct to manage user passwords securely.
// It stores both the plaintext (temporarily during setting) and the hashed version.
// A dedicated 'password' struct to encapsulate password hashing logic using bcrypt.
type password struct {
	plaintext *string // Pointer to plaintext password; nil if not being set/changed.
	hash      []byte  // Bcrypt hash of the password.
}

// Set generates a bcrypt hash for a given plaintext password and stores it.
// The cost factor (12) determines hashing strength.
// The Set method securely hashes passwords using bcrypt before they are stored.
func (p *password) Set(plaintextPassword string) error {
	// Generate bcrypt hash with a cost factor of 12.
	hash, err := bcrypt.GenerateFromPassword([]byte(plaintextPassword), 12)
	if err != nil {
		return err
	}
	p.plaintext = &plaintextPassword // Store plaintext temporarily
	p.hash = hash                    // Store the generated hash.
	return nil
}

// Hash returns the stored bcrypt hash of the password.
// Used when inserting/updating the user's password_hash in the database.
func (p *password) Hash() []byte {
	return p.hash
}

// Matches compares a plaintext password against the stored bcrypt hash.
// Returns true if they match, false otherwise. Handles bcrypt comparison errors.
// The Matches method safely compares a submitted password with the stored hash during login.
func (p *password) Matches(plaintextPassword string) (bool, error) {
	// Compare the hash with the plaintext password.
	err := bcrypt.CompareHashAndPassword(p.hash, []byte(plaintextPassword))
	if err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			return false, nil // Passwords do not match.
		}
		return false, err // Other error during comparison.
	}
	return true, nil // Passwords match.
}

// ValidateUser checks the user struct fields for validity (e.g., not blank, correct format, length).
// Server-side validation for user data ensures data integrity for new or updated user profiles.
func ValidateUser(v *validator.Validator, user *User) {
	// Validate Name: must be provided and within length limits.
	v.Check(validator.NotBlank(user.Name), "name", "Name must be provided")
	v.Check(validator.MaxLength(user.Name, 100), "name", "Must not be more than 100 characters")

	// Validate Email: must be provided, valid format, and within length limits.
	v.Check(validator.NotBlank(user.Email), "email", "Email must be provided")
	v.Check(validator.MaxLength(user.Email, 254), "email", "Must not be more than 254 characters")
	v.Check(validator.Matches(user.Email, validator.EmailRX), "email", "Must be a valid email address")

	// Validate Password (if being set/changed):
	// `user.Password.plaintext` is non-nil only when `Set()` was called (e.g., signup, password change).
	if user.Password.plaintext != nil {
		v.Check(validator.NotBlank(*user.Password.plaintext), "password", "Password must be provided")
		v.Check(validator.MinLength(*user.Password.plaintext, 8), "password", "Must be at least 8 characters long")
		v.Check(validator.MaxLength(*user.Password.plaintext, 72), "password", "Must not be more than 72 characters")
	} else if user.ID == 0 && len(user.Password.hash) == 0 {
		// Special case: For a new user (ID=0) where password was never set (hash is empty).
		v.AddError("password", "Password must be provided")
	}
}

// ValidatePasswordUpdate checks fields specific to a password change operation.
// Specific validation rules for the password change form.
func ValidatePasswordUpdate(v *validator.Validator, currentPassword, newPassword, confirmPassword string) {
	v.Check(validator.NotBlank(currentPassword), "current_password", "Current password must be provided")
	v.Check(validator.NotBlank(newPassword), "new_password", "New password must be provided")
	v.Check(validator.MinLength(newPassword, 8), "new_password", "New password must be at least 8 characters long")
	v.Check(validator.MaxLength(newPassword, 72), "new_password", "New password must not be more than 72 characters long")
	v.Check(validator.NotBlank(confirmPassword), "confirm_password", "Confirm new password must be provided")
	v.Check(newPassword == confirmPassword, "confirm_password", "New passwords do not match")
}

// UserModel provides methods for database operations related to users.
// It embeds a `*sql.DB` connection pool.
// The UserModel encapsulates all database interaction logic for users (CRUD, authentication).
type UserModel struct {
	DB *sql.DB
}

// --- UserModel Methods ---
// Presentation Point: "These methods on UserModel handle creating, fetching, updating, and authenticating users."

// Insert adds a new user record to the 'users' table.
// Creates a new user in the database after signup.
func (m *UserModel) Insert(user *User) error {
	// SQL query to insert user data and return DB-generated ID and CreatedAt.
	query := `
        INSERT INTO users (name, email, password_hash, activated)
        VALUES ($1, $2, $3, $4)
        RETURNING id, created_at`

	args := []any{
		user.Name,
		user.Email,
		user.Password.hash,
		user.Activated,
	}

	// Execute query with a timeout context.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Scan the returned ID and CreatedAt back into the user struct.
	err := m.DB.QueryRowContext(ctx, query, args...).Scan(&user.ID, &user.CreatedAt)
	if err != nil {
		// Handle PostgreSQL unique constraint violation for email.
		if strings.Contains(err.Error(), `duplicate key value violates unique constraint "users_email_key"`) {
			return ErrDuplicateEmail
		}
		return err
	}
	return nil
}

// Get retrieves a user by their unique ID.
// Fetches a user's details from the database by their ID.
func (m *UserModel) Get(id int64) (*User, error) {
	if id < 1 { // Basic validation for ID.
		return nil, ErrRecordNotFound
	}
	// SQL query to select user data by ID.
	query := `
        SELECT id, created_at, name, email, password_hash, activated
        FROM users
        WHERE id = $1`

	var user User // Struct to hold the fetched data.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Execute query and scan results into the user struct.
	err := m.DB.QueryRowContext(ctx, query, id).Scan(
		&user.ID,
		&user.CreatedAt,
		&user.Name,
		&user.Email,
		&user.Password.hash,
		&user.Activated,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) { //User not found
			return nil, ErrRecordNotFound
		}
		return nil, err // Other database errors.
	}
	return &user, nil //Success
}

// GetByEmail retrieves a user by their email address.
// Useful for checking if an email already exists or for login.
// Fetches user details by email, often used during login or signup checks.
func (m *UserModel) GetByEmail(email string) (*User, error) {
	query := `
        SELECT id, created_at, name, email, password_hash, activated
        FROM users
        WHERE email = $1` // Query by email.

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

// Update modifies a user's profile information (name, email).
// Updates user's name and email in the database.
func (m *UserModel) Update(user *User) error {
	// SQL query to update name and email for a given user ID.
	query := `
        UPDATE users
        SET name = $1, email = $2 
        WHERE id = $3
        RETURNING id` // RETURNING id to confirm update happened on the correct record.

	args := []any{
		user.Name,
		user.Email,
		user.ID,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, args...).Scan(&user.ID) // Scan is used with RETURNING.
	if err != nil {
		switch {
		case strings.Contains(err.Error(), `duplicate key value violates unique constraint "users_email_key"`):
			return ErrDuplicateEmail // Email conflict.
		case errors.Is(err, sql.ErrNoRows): // Should not happen if ID exists, but defensive.
			return ErrRecordNotFound
		default:
			return err
		}
	}
	return nil // Success.
}

// UpdatePassword changes a user's password_hash in the database.
// Specifically updates the user's hashed password.
func (m *UserModel) UpdatePassword(userID int64, newPasswordHash []byte) error {
	query := `
		UPDATE users
		SET password_hash = $1
		WHERE id = $2`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// ExecContext is used for UPDATEs that don't return rows (unless RETURNING is used differently).
	result, err := m.DB.ExecContext(ctx, query, newPasswordHash, userID)
	if err != nil {
		return err
	}

	// Check if any row was actually updated.
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 { // No user found with that ID.
		return ErrRecordNotFound
	}
	return nil // Success.
}

// Authenticate verifies a user's email and password against the database.
// It also checks if the user account is activated.
// Returns the user's ID on success, or an error.
// Core login logic: verifies email, compares password hash, and checks if account is active.
func (m *UserModel) Authenticate(email, plaintextPassword string) (int64, error) {
	var id int64
	var hashedPassword []byte
	// SQL query to get ID and hashed password for an active user with the given email.
	query := `
        SELECT id, password_hash FROM users
        WHERE email = $1 AND activated = TRUE`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Fetch user's ID and stored hash.
	err := m.DB.QueryRowContext(ctx, query, email).Scan(&id, &hashedPassword)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) { // User not found or not activated.
			return 0, ErrInvalidCredentials
		}
		return 0, err // Other database error.
	}

	// Compare submitted password with the stored hash.
	err = bcrypt.CompareHashAndPassword(hashedPassword, []byte(plaintextPassword))
	if err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) { // Passwords don't match.
			return 0, ErrInvalidCredentials
		}
		return 0, err // Error during comparison.
	}
	return id, nil // Authentication successful, return user ID.
}

// Delete removes a user and their associated data (via database cascades) by ID.
// Permanently deletes a user account from the database.
func (m *UserModel) Delete(id int64) error {
	if id < 1 { // Basic ID validation.
		return ErrRecordNotFound
	}
	query := `DELETE FROM users WHERE id = $1` // SQL to delete user by ID.

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	result, err := m.DB.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 { // No user found with that ID to delete.
		return ErrRecordNotFound
	}
	return nil // Success.
}
