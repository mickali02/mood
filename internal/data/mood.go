// mood/internal/data/mood.go
package data

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	// "regexp" // <-- REMOVED unused import
	"strings"
	"time"
	"unicode/utf8" // <-- ADDED import for RuneCountInString

	"github.com/mickali02/mood/internal/validator"
)

// Define the list of valid emotions - Centralized list
var ValidEmotions = []string{"Happy", "Sad", "Angry", "Anxious", "Calm", "Excited", "Neutral"}

// --- Local struct for distinct emotion details ---
type EmotionDetail struct {
	Name  string
	Emoji string
	Color string // Hex Color Code
}

// --- FilterCriteria struct Definition --- // <-- ADDED Definition
type FilterCriteria struct {
	TextQuery string
	Emotion   string
	StartDate time.Time // Use time.Time for actual filtering
	EndDate   time.Time
}

// Mood struct - Added Emoji and Color
type Mood struct {
	ID        int64     `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	Emotion   string    `json:"emotion"` // Name of the emotion
	Emoji     string    `json:"emoji"`   // Emoji character
	Color     string    `json:"color"`   // Hex color code (e.g., #FFD700)
}

// ValidateMood - Add validation for custom fields
func ValidateMood(v *validator.Validator, mood *Mood) {
	v.Check(validator.NotBlank(mood.Title), "title", "must be provided")
	v.Check(validator.MaxLength(mood.Title, 100), "title", "must not be more than 100 characters long")

	v.Check(validator.NotBlank(mood.Content), "content", "must be provided")

	v.Check(validator.NotBlank(mood.Emotion), "emotion", "name must be provided")
	v.Check(validator.MaxLength(mood.Emotion, 50), "emotion", "name must not be more than 50 characters long")

	v.Check(validator.NotBlank(mood.Emoji), "emoji", "must be provided")
	// Use utf8.RuneCountInString
	v.Check(utf8.RuneCountInString(mood.Emoji) >= 1, "emoji", "must contain at least one character")
	v.Check(utf8.RuneCountInString(mood.Emoji) <= 4, "emoji", "is too long for a typical emoji")

	v.Check(validator.NotBlank(mood.Color), "color", "must be provided")
	v.Check(validator.Matches(mood.Color, validator.HexColorRX), "color", "must be a valid hex color code (e.g., #FFD700)")
}

// MoodModel struct (no changes)
type MoodModel struct {
	DB *sql.DB
}

// Insert - Add emoji and color
func (m *MoodModel) Insert(mood *Mood) error {
	query := `
        INSERT INTO moods (title, content, emotion, emoji, color)
        VALUES ($1, $2, $3, $4, $5)
        RETURNING id, created_at, updated_at`
	args := []any{mood.Title, mood.Content, mood.Emotion, mood.Emoji, mood.Color}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	err := m.DB.QueryRowContext(ctx, query, args...).Scan(&mood.ID, &mood.CreatedAt, &mood.UpdatedAt)
	if err != nil {
		return fmt.Errorf("mood insert: %w", err)
	}
	return nil
}

// Get - Add emoji and color
func (m *MoodModel) Get(id int64) (*Mood, error) {
	if id < 1 {
		return nil, sql.ErrNoRows
	}
	query := `
        SELECT id, created_at, updated_at, title, content, emotion, emoji, color
        FROM moods WHERE id = $1`
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	var mood Mood
	err := m.DB.QueryRowContext(ctx, query, id).Scan(
		&mood.ID, &mood.CreatedAt, &mood.UpdatedAt,
		&mood.Title, &mood.Content, &mood.Emotion,
		&mood.Emoji, &mood.Color,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, sql.ErrNoRows
		}
		return nil, fmt.Errorf("mood get: %w", err)
	}
	return &mood, nil
}

// Update - Add emoji and color
func (m *MoodModel) Update(mood *Mood) error {
	if mood.ID < 1 {
		return sql.ErrNoRows
	}
	query := `
        UPDATE moods SET title = $1, content = $2, emotion = $3, emoji = $4, color = $5, updated_at = NOW()
        WHERE id = $6 RETURNING updated_at`
	args := []any{mood.Title, mood.Content, mood.Emotion, mood.Emoji, mood.Color, mood.ID}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	err := m.DB.QueryRowContext(ctx, query, args...).Scan(&mood.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return sql.ErrNoRows
		}
		return fmt.Errorf("mood update: %w", err)
	}
	return nil
}

// Delete (No changes needed)
func (m *MoodModel) Delete(id int64) error {
	if id < 1 {
		return sql.ErrNoRows
	}
	query := `DELETE FROM moods WHERE id = $1`
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	result, err := m.DB.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("mood delete exec: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("mood delete rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// GetFiltered - Uses FilterCriteria
func (m *MoodModel) GetFiltered(filters FilterCriteria) ([]*Mood, error) { // <-- Uses FilterCriteria
	baseQuery := `
        SELECT id, created_at, updated_at, title, content, emotion, emoji, color
        FROM moods WHERE 1=1`
	args := []any{}
	paramIndex := 1

	if filters.TextQuery != "" {
		searchTerm := "%" + strings.TrimSpace(filters.TextQuery) + "%"
		baseQuery += fmt.Sprintf(" AND (title ILIKE $%d OR content ILIKE $%d OR emotion ILIKE $%d)", paramIndex, paramIndex, paramIndex)
		args = append(args, searchTerm)
		paramIndex++
	}
	if filters.Emotion != "" {
		baseQuery += fmt.Sprintf(" AND emotion = $%d", paramIndex)
		args = append(args, filters.Emotion)
		paramIndex++
	}
	if !filters.StartDate.IsZero() {
		baseQuery += fmt.Sprintf(" AND created_at >= $%d", paramIndex)
		args = append(args, filters.StartDate)
		paramIndex++
	}
	if !filters.EndDate.IsZero() {
		endDateStartOfNextDay := filters.EndDate.Truncate(24 * time.Hour).Add(24 * time.Hour)
		baseQuery += fmt.Sprintf(" AND created_at < $%d", paramIndex)
		args = append(args, endDateStartOfNextDay)
		paramIndex++
	}
	baseQuery += " ORDER BY created_at DESC"

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	rows, err := m.DB.QueryContext(ctx, baseQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("filtered query execution: %w", err)
	}
	defer rows.Close()

	moods := make([]*Mood, 0)
	for rows.Next() {
		var mood Mood
		err := rows.Scan(
			&mood.ID, &mood.CreatedAt, &mood.UpdatedAt,
			&mood.Title, &mood.Content, &mood.Emotion,
			&mood.Emoji, &mood.Color,
		)
		if err != nil {
			return nil, fmt.Errorf("filtered scan row: %w", err)
		}
		moods = append(moods, &mood)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("filtered rows iteration: %w", err)
	}
	return moods, nil
}

// GetAll - Calls GetFiltered
func (m *MoodModel) GetAll() ([]*Mood, error) {
	return m.GetFiltered(FilterCriteria{}) // <-- Uses FilterCriteria
}

// Search - Calls GetFiltered
func (m *MoodModel) Search(query string) ([]*Mood, error) {
	filters := FilterCriteria{TextQuery: query} // <-- Uses FilterCriteria
	return m.GetFiltered(filters)
}

// GetDistinctEmotionDetails - Returns local []EmotionDetail
func (m *MoodModel) GetDistinctEmotionDetails() ([]EmotionDetail, error) {
	query := `
        SELECT DISTINCT emotion, emoji, color FROM moods
        WHERE emotion IS NOT NULL AND emoji IS NOT NULL AND color IS NOT NULL
        ORDER BY emotion ASC`
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	rows, err := m.DB.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("distinct emotion query: %w", err)
	}
	defer rows.Close()

	emotionDetailsList := make([]EmotionDetail, 0) // Use local type
	for rows.Next() {
		var detail EmotionDetail // Use local type
		err := rows.Scan(&detail.Name, &detail.Emoji, &detail.Color)
		if err != nil {
			return nil, fmt.Errorf("distinct emotion scan: %w", err)
		}
		emotionDetailsList = append(emotionDetailsList, detail)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("distinct emotion rows iteration: %w", err)
	}
	return emotionDetailsList, nil
}
