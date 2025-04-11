// mood/internal/data/mood.go
package data

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/mickali02/mood/internal/validator"
)

// Define the list of valid emotions - Centralized list
var ValidEmotions = []string{"Happy", "Sad", "Angry", "Anxious", "Calm", "Excited", "Neutral"} // Add/modify as needed

// Mood struct represents a single mood entry in the database.
type Mood struct {
	ID        int64     `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	Emotion   string    `json:"emotion"`
}

// ValidateMood checks the mood struct fields against validation rules.
func ValidateMood(v *validator.Validator, mood *Mood) {
	v.Check(validator.NotBlank(mood.Title), "title", "must be provided")
	v.Check(validator.MaxLength(mood.Title, 100), "title", "must not be more than 100 characters long")

	v.Check(validator.NotBlank(mood.Content), "content", "must be provided")
	v.Check(validator.MaxLength(mood.Content, 1000), "content", "must not be more than 1000 characters long")

	v.Check(validator.NotBlank(mood.Emotion), "emotion", "must be selected")
	v.Check(validator.PermittedValue(mood.Emotion, ValidEmotions...), "emotion", "is not a valid emotion")
}

// MoodModel struct provides methods for interacting with the mood data.
type MoodModel struct {
	DB *sql.DB
}

// Insert adds a new mood entry into the 'moods' table.
func (m *MoodModel) Insert(mood *Mood) error {
	query := `
        INSERT INTO moods (title, content, emotion)
        VALUES ($1, $2, $3)
        RETURNING id, created_at, updated_at`

	args := []any{mood.Title, mood.Content, mood.Emotion}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, args...).Scan(&mood.ID, &mood.CreatedAt, &mood.UpdatedAt)
	if err != nil {
		return err
	}
	return nil
}

// Get retrieves a specific mood entry by its ID.
func (m *MoodModel) Get(id int64) (*Mood, error) {
	if id < 1 {
		return nil, sql.ErrNoRows
	}

	query := `
        SELECT id, created_at, updated_at, title, content, emotion
        FROM moods
        WHERE id = $1`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var mood Mood
	err := m.DB.QueryRowContext(ctx, query, id).Scan(
		&mood.ID,
		&mood.CreatedAt,
		&mood.UpdatedAt,
		&mood.Title,
		&mood.Content,
		&mood.Emotion,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, sql.ErrNoRows
		}
		return nil, err
	}

	return &mood, nil
}

// Update modifies an existing mood entry in the database.
func (m *MoodModel) Update(mood *Mood) error {
	if mood.ID < 1 {
		return sql.ErrNoRows
	}

	query := `
        UPDATE moods
        SET title = $1, content = $2, emotion = $3, updated_at = NOW()
        WHERE id = $4
        RETURNING updated_at`

	args := []any{
		mood.Title,
		mood.Content,
		mood.Emotion,
		mood.ID,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, args...).Scan(&mood.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return sql.ErrNoRows
		}
		return err
	}
	return nil
}

// Delete removes a specific mood entry from the database.
func (m *MoodModel) Delete(id int64) error {
	if id < 1 {
		return sql.ErrNoRows
	}

	query := `
        DELETE FROM moods
        WHERE id = $1`

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
		return sql.ErrNoRows // Record was not found to delete
	}

	return nil
}

// GetAll retrieves all mood entries, ordered by creation date (newest first).
func (m *MoodModel) GetAll() ([]*Mood, error) {
	query := `
        SELECT id, created_at, updated_at, title, content, emotion
        FROM moods
        ORDER BY created_at DESC`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	rows, err := m.DB.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Use make to initialize slice with capacity, potentially more efficient
	moods := make([]*Mood, 0)
	for rows.Next() {
		var mood Mood
		err := rows.Scan(
			&mood.ID,
			&mood.CreatedAt,
			&mood.UpdatedAt,
			&mood.Title,
			&mood.Content,
			&mood.Emotion,
		)
		if err != nil {
			return nil, err
		}
		moods = append(moods, &mood)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return moods, nil
}
