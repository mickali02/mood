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
	"github.com/microcosm-cc/bluemonday"
)

var ValidEmotions = []string{"Happy", "Sad", "Angry", "Anxious", "Calm", "Excited", "Neutral"}

type EmotionDetail struct {
	Name  string
	Emoji string
	Color string
}

type EmotionCount struct {
	Name  string `json:"name"`
	Emoji string `json:"emoji"`
	Color string `json:"color"`
	Count int    `json:"count"`
}

type MonthlyCount struct {
	Month string `json:"month"`
	Count int    `json:"count"`
}

// MoodStats aggregates all statistics for the stats page
type MoodStats struct {
	TotalEntries      int            `json:"totalEntries"`
	MostCommonEmotion *EmotionCount  `json:"mostCommonEmotion"`
	EmotionCounts     []EmotionCount `json:"emotionCounts"`
	MonthlyCounts     []MonthlyCount `json:"monthlyCounts"`
	// --- NEW FIELDS ---
	LatestMood        *Mood   `json:"latestMood"`        // Pointer to the latest mood entry
	AvgEntriesPerWeek float64 `json:"avgEntriesPerWeek"` // Average entries per week
	// --- END NEW FIELDS ---
}

// --- FilterCriteria struct Definition ---
type FilterCriteria struct {
	TextQuery string
	Emotion   string
	StartDate time.Time
	EndDate   time.Time
	Page      int
	PageSize  int
}

// --- Metadata struct ---
type Metadata struct {
	CurrentPage  int `json:"current_page,omitempty"`
	PageSize     int `json:"page_size,omitempty"`
	FirstPage    int `json:"first_page,omitempty"`
	LastPage     int `json:"last_page,omitempty"`
	TotalRecords int `json:"total_records,omitempty"`
}

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

func ValidateMood(v *validator.Validator, mood *Mood) {
	v.Check(validator.NotBlank(mood.Title), "title", "must be provided")
	v.Check(validator.MaxLength(mood.Title, 100), "title", "must not be more than 100 characters long")
	p := bluemonday.StrictPolicy()
	plainTextContent := p.Sanitize(mood.Content)
	v.Check(validator.NotBlank(plainTextContent), "content", "must be provided")
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

func (m *MoodModel) GetFiltered(filters FilterCriteria) ([]*Mood, Metadata, error) {
	baseQuery := `
        FROM moods
        WHERE 1=1`
	args := []any{}
	paramIndex := 1

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
		baseQuery += fmt.Sprintf(" AND created_at <= $%d", paramIndex)
		args = append(args, filters.EndDate)
		paramIndex++
	}

	totalRecordsQuery := `SELECT count(*) ` + baseQuery
	ctxCount, cancelCount := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancelCount()

	var totalRecords int
	err := m.DB.QueryRowContext(ctxCount, totalRecordsQuery, args...).Scan(&totalRecords)
	if err != nil {
		return nil, Metadata{}, fmt.Errorf("count query execution: %w", err)
	}

	if filters.PageSize <= 0 {
		filters.PageSize = 4
	}
	if filters.Page <= 0 {
		filters.Page = 1
	}
	metadata := calculateMetadata(totalRecords, filters.Page, filters.PageSize)

	if totalRecords > 0 && filters.Page > metadata.LastPage {
		return []*Mood{}, metadata, nil
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

func (m *MoodModel) GetAll() ([]*Mood, Metadata, error) {
	return m.GetFiltered(FilterCriteria{Page: 1, PageSize: 4})
}

func (m *MoodModel) Search(query string) ([]*Mood, Metadata, error) {
	filters := FilterCriteria{TextQuery: query, Page: 1, PageSize: 4}
	return m.GetFiltered(filters)
}

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

func (m *MoodModel) GetTotalMoodCount() (int, error) {
	query := `SELECT COUNT(*) FROM moods`
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var total int
	err := m.DB.QueryRowContext(ctx, query).Scan(&total)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, nil
		}
		return 0, fmt.Errorf("mood count query: %w", err)
	}
	return total, nil
}

func (m *MoodModel) GetEmotionCounts() ([]EmotionCount, error) {
	query := `
        SELECT emotion, emoji, color, COUNT(*)
        FROM moods
        WHERE emotion IS NOT NULL AND emoji IS NOT NULL AND color IS NOT NULL
        GROUP BY emotion, emoji, color
        ORDER BY COUNT(*) DESC, emotion ASC`

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := m.DB.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("emotion counts query: %w", err)
	}
	defer rows.Close()

	counts := []EmotionCount{}
	for rows.Next() {
		var ec EmotionCount
		err := rows.Scan(&ec.Name, &ec.Emoji, &ec.Color, &ec.Count)
		if err != nil {
			return nil, fmt.Errorf("emotion counts scan: %w", err)
		}
		counts = append(counts, ec)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("emotion counts rows iteration: %w", err)
	}

	return counts, nil
}

func (m *MoodModel) GetMonthlyEntryCounts() ([]MonthlyCount, error) {
	query := `
        SELECT TO_CHAR(created_at, 'Mon YYYY') AS month_year, COUNT(*) as count
        FROM moods
        GROUP BY month_year, date_trunc('month', created_at)
        ORDER BY date_trunc('month', created_at) ASC`

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := m.DB.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("monthly counts query: %w", err)
	}
	defer rows.Close()

	counts := []MonthlyCount{}
	for rows.Next() {
		var mc MonthlyCount
		err := rows.Scan(&mc.Month, &mc.Count)
		if err != nil {
			return nil, fmt.Errorf("monthly counts scan: %w", err)
		}
		counts = append(counts, mc)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("monthly counts rows iteration: %w", err)
	}

	return counts, nil
}

// --- NEW: GetLatestMood fetches the single most recent mood entry ---
func (m *MoodModel) GetLatestMood() (*Mood, error) {
	query := `
        SELECT id, created_at, updated_at, title, content, emotion, emoji, color
        FROM moods
        ORDER BY created_at DESC
        LIMIT 1`
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var mood Mood
	err := m.DB.QueryRowContext(ctx, query).Scan(
		&mood.ID, &mood.CreatedAt, &mood.UpdatedAt,
		&mood.Title, &mood.Content, &mood.Emotion,
		&mood.Emoji, &mood.Color,
	)
	if err != nil {
		// If no rows are found, it's not necessarily an application error,
		// just means there are no moods yet. Return nil mood and nil error.
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // No mood found, not an error for stats purposes
		}
		// For other errors, wrap and return them
		return nil, fmt.Errorf("latest mood get: %w", err)
	}
	return &mood, nil
}

// --- NEW: GetFirstEntryDate fetches the timestamp of the earliest mood entry ---
func (m *MoodModel) GetFirstEntryDate() (time.Time, error) {
	query := `SELECT MIN(created_at) FROM moods`
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var firstDate sql.NullTime // Use sql.NullTime to handle potential NULL result if no rows
	err := m.DB.QueryRowContext(ctx, query).Scan(&firstDate)
	if err != nil {
		// sql.ErrNoRows isn't expected here because MIN() on an empty table usually returns NULL
		// but handle it defensively just in case. More likely is a NULL scan.
		if errors.Is(err, sql.ErrNoRows) {
			return time.Time{}, nil // No entries, return zero time
		}
		return time.Time{}, fmt.Errorf("first entry date query: %w", err)
	}

	if !firstDate.Valid {
		return time.Time{}, nil // No entries (MIN returned NULL), return zero time
	}

	return firstDate.Time, nil
}

// GetAllStats aggregates all statistics in one call.
func (m *MoodModel) GetAllStats() (*MoodStats, error) {
	total, err := m.GetTotalMoodCount()
	if err != nil {
		return nil, fmt.Errorf("failed to get total count: %w", err)
	}

	// Initialize stats struct
	stats := &MoodStats{
		TotalEntries:      total,
		EmotionCounts:     []EmotionCount{}, // Initialize slices
		MonthlyCounts:     []MonthlyCount{},
		LatestMood:        nil, // Initialize pointer
		AvgEntriesPerWeek: 0.0, // Initialize float
	}

	// If no entries, return the initialized struct with zero total
	if total == 0 {
		return stats, nil
	}

	// --- Fetch other stats only if there are entries ---

	// Fetch Latest Mood
	latestMood, err := m.GetLatestMood()
	if err != nil {
		// Log this error but don't fail the whole stats generation if possible
		// For now, we'll return the error. Could be made more resilient later.
		return nil, fmt.Errorf("failed to get latest mood: %w", err)
	}
	stats.LatestMood = latestMood // Assign, even if it's nil (handled in template)

	// Fetch Emotion Counts
	emotionCounts, err := m.GetEmotionCounts()
	if err != nil {
		return nil, fmt.Errorf("failed to get emotion counts: %w", err)
	}
	stats.EmotionCounts = emotionCounts
	if len(emotionCounts) > 0 {
		stats.MostCommonEmotion = &emotionCounts[0] // Already sorted
	}

	// Fetch Monthly Counts
	monthlyCounts, err := m.GetMonthlyEntryCounts()
	if err != nil {
		return nil, fmt.Errorf("failed to get monthly counts: %w", err)
	}
	stats.MonthlyCounts = monthlyCounts

	// Calculate Average Entries Per Week
	firstEntryDate, err := m.GetFirstEntryDate()
	if err != nil {
		return nil, fmt.Errorf("failed to get first entry date: %w", err)
	}

	// Check if firstEntryDate is valid (not the zero value)
	if !firstEntryDate.IsZero() {
		duration := time.Since(firstEntryDate) // Duration since first entry
		weeks := duration.Hours() / (24 * 7)   // Calculate weeks elapsed

		// Handle edge case: If duration is very short (e.g., < 1 week),
		// avoid division by zero or artificially high averages.
		// Treat durations less than a week as 1 week for calculation.
		if weeks < 1.0 && weeks >= 0 {
			weeks = 1.0
		}

		// Calculate average if weeks is positive
		if weeks > 0 {
			stats.AvgEntriesPerWeek = float64(total) / weeks
		} else {
			// Should only happen if first entry is in the future (data issue) or exactly now.
			// If exactly now and total > 0, average is effectively infinite, maybe show total?
			// Setting to total might be reasonable for the first week.
			stats.AvgEntriesPerWeek = float64(total)
		}
	}
	// If firstEntryDate is zero (shouldn't happen if total > 0, but defensive), avg remains 0.0

	return stats, nil
}
