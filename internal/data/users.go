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

// Define user-specific errors
var (
	ErrDuplicateEmail     = errors.New("duplicate email")
	ErrRecordNotFound     = errors.New("record not found")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrEditConflict       = errors.New("edit conflict") // For optimistic locking if/when versioning is added
)

// User struct definition (Language field removed)
type User struct {
	ID        int64     `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Password  password  `json:"-"` // Use custom password type
	Activated bool      `json:"activated"`
	// Version int    `json:"-"` // Optional version for optimistic locking
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
	p.plaintext = &plaintextPassword
	p.hash = hash
	return nil
}

// Hash returns the stored password hash.
// This is an exported method to allow access from other packages.
func (p *password) Hash() []byte {
	return p.hash
}

// Matches checks whether the provided plaintext password matches the stored hash.
func (p *password) Matches(plaintextPassword string) (bool, error) {
	err := bcrypt.CompareHashAndPassword(p.hash, []byte(plaintextPassword))
	if err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// --- Validator for User (Language validation removed) ---
func ValidateUser(v *validator.Validator, user *User) {
	v.Check(validator.NotBlank(user.Name), "name", "Name must be provided")
	v.Check(validator.MaxLength(user.Name, 100), "name", "Must not be more than 100 characters")

	v.Check(validator.NotBlank(user.Email), "email", "Email must be provided")
	v.Check(validator.MaxLength(user.Email, 254), "email", "Must not be more than 254 characters")
	v.Check(validator.Matches(user.Email, validator.EmailRX), "email", "Must be a valid email address")

	// If user.Password.plaintext is not nil, it means a password is being set/validated (e.g., signup or password change).
	if user.Password.plaintext != nil {
		v.Check(validator.NotBlank(*user.Password.plaintext), "password", "Password must be provided")
		v.Check(validator.MinLength(*user.Password.plaintext, 8), "password", "Must be at least 8 characters long")
		v.Check(validator.MaxLength(*user.Password.plaintext, 72), "password", "Must not be more than 72 characters")
	} else if user.ID == 0 && len(user.Password.hash) == 0 {
		// This case is for a new user (ID is 0) where SetPassword was never called (hash is empty, plaintext is nil).
		v.AddError("password", "Password must be provided")
	}
}

// --- Validator for Password Change (remains unchanged) ---
func ValidatePasswordUpdate(v *validator.Validator, currentPassword, newPassword, confirmPassword string) {
	v.Check(validator.NotBlank(currentPassword), "current_password", "Current password must be provided")
	v.Check(validator.NotBlank(newPassword), "new_password", "New password must be provided")
	v.Check(validator.MinLength(newPassword, 8), "new_password", "New password must be at least 8 characters long")
	v.Check(validator.MaxLength(newPassword, 72), "new_password", "New password must not be more than 72 characters long")
	v.Check(validator.NotBlank(confirmPassword), "confirm_password", "Confirm new password must be provided")
	v.Check(newPassword == confirmPassword, "confirm_password", "New passwords do not match")
}

// --- UserModel Struct Definition (remains unchanged) ---
type UserModel struct {
	DB *sql.DB
}

// --- UserModel Methods ---

// Insert a new user record into the database. (Language removed)
func (m *UserModel) Insert(user *User) error {
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

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, args...).Scan(&user.ID, &user.CreatedAt)
	if err != nil {
		if strings.Contains(err.Error(), `duplicate key value violates unique constraint "users_email_key"`) {
			return ErrDuplicateEmail
		}
		return err
	}
	return nil
}

// Get retrieves a specific user based on their ID. (Language removed)
func (m *UserModel) Get(id int64) (*User, error) {
	if id < 1 {
		return nil, ErrRecordNotFound
	}
	query := `
        SELECT id, created_at, name, email, password_hash, activated
        FROM users
        WHERE id = $1`

	var user User
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, id).Scan(
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

// GetByEmail retrieves user details based on email address. (Language removed)
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

// Update allows changing user profile details: name, email. (Language removed)
func (m *UserModel) Update(user *User) error {
	query := `
        UPDATE users
        SET name = $1, email = $2 
        WHERE id = $3
        RETURNING id`

	args := []any{
		user.Name,
		user.Email,
		user.ID,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, args...).Scan(&user.ID)
	if err != nil {
		switch {
		case strings.Contains(err.Error(), `duplicate key value violates unique constraint "users_email_key"`):
			return ErrDuplicateEmail
		case errors.Is(err, sql.ErrNoRows):
			return ErrRecordNotFound
		default:
			return err
		}
	}
	return nil
}

// UpdatePassword updates a user's password hash in the database. (remains unchanged)
func (m *UserModel) UpdatePassword(userID int64, newPasswordHash []byte) error {
	query := `
		UPDATE users
		SET password_hash = $1
		WHERE id = $2`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	result, err := m.DB.ExecContext(ctx, query, newPasswordHash, userID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return ErrRecordNotFound
	}
	return nil
}

// Authenticate checks if a user exists with the given email and password AND is activated. (remains unchanged)
func (m *UserModel) Authenticate(email, plaintextPassword string) (int64, error) {
	var id int64
	var hashedPassword []byte
	query := `
        SELECT id, password_hash FROM users
        WHERE email = $1 AND activated = TRUE`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, email).Scan(&id, &hashedPassword)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, ErrInvalidCredentials
		}
		return 0, err
	}

	err = bcrypt.CompareHashAndPassword(hashedPassword, []byte(plaintextPassword))
	if err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			return 0, ErrInvalidCredentials
		}
		return 0, err
	}
	return id, nil
}

// Delete a user by ID. (remains unchanged)
func (m *UserModel) Delete(id int64) error {
	if id < 1 {
		return ErrRecordNotFound
	}
	query := `DELETE FROM users WHERE id = $1`

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
	if rowsAffected == 0 {
		return ErrRecordNotFound
	}
	return nil
}
