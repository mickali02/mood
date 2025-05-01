// mood/internal/data/mood.go
package data

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mickali02/mood/internal/validator"
)

var ValidEmotions = []string{"Happy", "Sad", "Angry", "Anxious", "Calm", "Excited", "Neutral"}

type EmotionDetail struct {
	Name  string
	Emoji string
	Color string
}

// --- FilterCriteria struct Definition ---
type FilterCriteria struct {
	TextQuery string
	Emotion   string
	StartDate time.Time
	EndDate   time.Time
	// --- Pagination fields ---
	Page     int
	PageSize int
}

// --- Metadata struct ---
type Metadata struct {
	CurrentPage  int `json:"current_page,omitempty"`
	PageSize     int `json:"page_size,omitempty"`
	FirstPage    int `json:"first_page,omitempty"`
	LastPage     int `json:"last_page,omitempty"`
	TotalRecords int `json:"total_records,omitempty"`
}

// Helper to calculate metadata
func calculateMetadata(totalRecords, page, pageSize int) Metadata {
	if totalRecords == 0 {
		return Metadata{}
	}
	return Metadata{
		CurrentPage:  page,
		PageSize:     pageSize,
		FirstPage:    1,
		LastPage:     int(math.Ceil(float64(totalRecords) / float64(pageSize))),
		TotalRecords: totalRecords,
	}
}

// Mood struct
type Mood struct {
	ID        int64     `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	Emotion   string    `json:"emotion"`
	Emoji     string    `json:"emoji"`
	Color     string    `json:"color"`
}

// ValidateMood
func ValidateMood(v *validator.Validator, mood *Mood) {
	v.Check(validator.NotBlank(mood.Title), "title", "must be provided")
	v.Check(validator.MaxLength(mood.Title, 100), "title", "must not be more than 100 characters long")
	v.Check(validator.NotBlank(mood.Content), "content", "must be provided")
	v.Check(validator.NotBlank(mood.Emotion), "emotion", "name must be provided")
	v.Check(validator.MaxLength(mood.Emotion, 50), "emotion", "name must not be more than 50 characters long")
	v.Check(validator.NotBlank(mood.Emoji), "emoji", "must be provided")
	v.Check(utf8.RuneCountInString(mood.Emoji) >= 1, "emoji", "must contain at least one character")
	v.Check(utf8.RuneCountInString(mood.Emoji) <= 4, "emoji", "is too long for a typical emoji")
	v.Check(validator.NotBlank(mood.Color), "color", "must be provided")
	v.Check(validator.Matches(mood.Color, validator.HexColorRX), "color", "must be a valid hex color code (e.g., #FFD700)")
}

type MoodModel struct {
	DB *sql.DB
}

// Insert
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

// Get
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

// Update
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

// Delete
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

// GetFiltered (with pagination)
func (m *MoodModel) GetFiltered(filters FilterCriteria) ([]*Mood, Metadata, error) {
	baseQuery := `
        FROM moods
        WHERE 1=1`
	args := []any{}
	paramIndex := 1

	// Apply WHERE clauses
	if filters.TextQuery != "" {
		searchTerm := "%" + strings.TrimSpace(filters.TextQuery) + "%"
		baseQuery += fmt.Sprintf(" AND (title ILIKE $%d OR content ILIKE $%d OR emotion ILIKE $%d)", paramIndex, paramIndex, paramIndex)
		args = append(args, searchTerm)
		paramIndex++
	}
	if filters.Emotion != "" {
		parts := strings.SplitN(filters.Emotion, "::", 2)
		if len(parts) == 2 {
			emotionName := parts[0]
			emotionEmoji := parts[1]
			if emotionName != "" && emotionEmoji != "" {
				baseQuery += fmt.Sprintf(" AND emotion = $%d AND emoji = $%d", paramIndex, paramIndex+1)
				args = append(args, emotionName, emotionEmoji)
				paramIndex += 2
			}
		} else {
			baseQuery += fmt.Sprintf(" AND emotion = $%d", paramIndex)
			args = append(args, filters.Emotion)
			paramIndex++
		}
	}
	if !filters.StartDate.IsZero() {
		baseQuery += fmt.Sprintf(" AND created_at >= $%d", paramIndex)
		args = append(args, filters.StartDate)
		paramIndex++
	}
	if !filters.EndDate.IsZero() {
		baseQuery += fmt.Sprintf(" AND created_at <= $%d", paramIndex) // Use <= for end date
		args = append(args, filters.EndDate)
		paramIndex++
	}

	// Query for Total Records
	totalRecordsQuery := `SELECT count(*) ` + baseQuery
	ctxCount, cancelCount := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancelCount()

	var totalRecords int
	err := m.DB.QueryRowContext(ctxCount, totalRecordsQuery, args...).Scan(&totalRecords)
	if err != nil {
		return nil, Metadata{}, fmt.Errorf("count query execution: %w", err)
	}

	// Calculate Metadata
	if filters.PageSize <= 0 {
		filters.PageSize = 4
	}
	if filters.Page <= 0 {
		filters.Page = 1
	}
	metadata := calculateMetadata(totalRecords, filters.Page, filters.PageSize)

	// Query for Paginated Moods
	if metadata.LastPage > 0 && filters.Page > metadata.LastPage {
		return []*Mood{}, metadata, nil // Return empty moods if page is out of bounds
	}

	selectQuery := `SELECT id, created_at, updated_at, title, content, emotion, emoji, color ` +
		baseQuery +
		` ORDER BY created_at DESC LIMIT $` + fmt.Sprint(paramIndex) +
		` OFFSET $` + fmt.Sprint(paramIndex+1)

	limit := filters.PageSize
	offset := (filters.Page - 1) * filters.PageSize
	queryArgs := append(args, limit, offset)

	ctxQuery, cancelQuery := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelQuery()

	rows, err := m.DB.QueryContext(ctxQuery, selectQuery, queryArgs...)
	if err != nil {
		return nil, metadata, fmt.Errorf("paginated query execution: %w", err)
	}
	defer rows.Close()

	// Scan results
	moods := make([]*Mood, 0, filters.PageSize)
	for rows.Next() {
		var mood Mood
		err := rows.Scan(
			&mood.ID, &mood.CreatedAt, &mood.UpdatedAt,
			&mood.Title, &mood.Content, &mood.Emotion,
			&mood.Emoji, &mood.Color,
		)
		if err != nil {
			return nil, metadata, fmt.Errorf("paginated scan row: %w", err)
		}
		moods = append(moods, &mood)
	}
	if err = rows.Err(); err != nil {
		return nil, metadata, fmt.Errorf("paginated rows iteration: %w", err)
	}

	return moods, metadata, nil
}

// GetAll (updated signature)
func (m *MoodModel) GetAll() ([]*Mood, Metadata, error) {
	return m.GetFiltered(FilterCriteria{Page: 1, PageSize: 4})
}

// Search (updated signature)
func (m *MoodModel) Search(query string) ([]*Mood, Metadata, error) {
	filters := FilterCriteria{TextQuery: query, Page: 1, PageSize: 4}
	return m.GetFiltered(filters)
}

// GetDistinctEmotionDetails (no changes)
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

	emotionDetailsList := make([]EmotionDetail, 0)
	for rows.Next() {
		var detail EmotionDetail
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
