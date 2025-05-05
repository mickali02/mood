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

	"github.com/lib/pq"
	"github.com/mickali02/mood/internal/validator"
	"github.com/microcosm-cc/bluemonday"
)

// Use the user-specific ErrRecordNotFound defined in users.go if possible,
// otherwise keep a local one or ensure consistency. Assuming users.go defines it.
// var ErrRecordNotFound = errors.New("record not found")

var ValidEmotions = []string{"Happy", "Sad", "Angry", "Anxious", "Calm", "Excited", "Neutral"}

// --- Struct Definitions (EmotionDetail, EmotionCount, MonthlyCount are unchanged) ---
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
	LatestMood        *Mood          `json:"latestMood"`
	AvgEntriesPerWeek float64        `json:"avgEntriesPerWeek"`
}

// --- FilterCriteria struct Definition ---
type FilterCriteria struct {
	TextQuery string
	Emotion   string
	StartDate time.Time
	EndDate   time.Time
	Page      int
	PageSize  int
	UserID    int64 // <-- ADDED UserID for filtering
}

// --- Metadata struct (unchanged) ---
type Metadata struct {
	CurrentPage  int `json:"current_page,omitempty"`
	PageSize     int `json:"page_size,omitempty"`
	FirstPage    int `json:"first_page,omitempty"`
	LastPage     int `json:"last_page,omitempty"`
	TotalRecords int `json:"total_records,omitempty"`
}

// calculateMetadata (unchanged)
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
	UserID    int64     `json:"user_id"` // <-- Already added
}

// ValidateMood (unchanged)
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
	v.Check(utf8.RuneCountInString(mood.Emoji) <= 4, "emoji", "is too long for a typical emoji") // Limit emoji length
	v.Check(validator.NotBlank(mood.Color), "color", "must be provided")
	v.Check(validator.Matches(mood.Color, validator.HexColorRX), "color", "must be a valid hex color code (e.g., #FFD700)")
}

type MoodModel struct {
	DB *sql.DB
}

// Insert - Takes UserID from the Mood struct
func (m *MoodModel) Insert(mood *Mood) error {
	if mood.UserID < 1 {
		return errors.New("invalid user ID provided for mood insert")
	}

	query := `
        INSERT INTO moods (title, content, emotion, emoji, color, user_id) -- Added user_id
        VALUES ($1, $2, $3, $4, $5, $6)                            -- Added $6
        RETURNING id, created_at, updated_at`

	args := []any{mood.Title, mood.Content, mood.Emotion, mood.Emoji, mood.Color, mood.UserID}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, args...).Scan(&mood.ID, &mood.CreatedAt, &mood.UpdatedAt)
	if err != nil {
		// Check for potential foreign key violation (if user_id doesn't exist in users table)
		// The specific error string might vary slightly depending on PostgreSQL version/config
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23503" { // 23503 is foreign_key_violation
			return fmt.Errorf("mood insert failed: user with ID %d does not exist: %w", mood.UserID, err)
		}
		return fmt.Errorf("mood insert: %w", err)
	}
	return nil
}

// Get - Now requires mood ID and the UserID it should belong to
func (m *MoodModel) Get(id int64, userID int64) (*Mood, error) { // <-- Added userID parameter
	if id < 1 || userID < 1 {
		return nil, ErrRecordNotFound // Use shared error
	}
	query := `
        SELECT id, created_at, updated_at, title, content, emotion, emoji, color, user_id -- Added user_id
        FROM moods
        WHERE id = $1 AND user_id = $2` // Added user_id check

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var mood Mood
	err := m.DB.QueryRowContext(ctx, query, id, userID).Scan( // Pass userID as arg
		&mood.ID, &mood.CreatedAt, &mood.UpdatedAt,
		&mood.Title, &mood.Content, &mood.Emotion,
		&mood.Emoji, &mood.Color, &mood.UserID, // Scan UserID
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Mood ID doesn't exist OR doesn't belong to this user.
			return nil, ErrRecordNotFound // Use shared error
		}
		return nil, fmt.Errorf("mood get: %w", err)
	}
	return &mood, nil
}

// Update - Takes UserID from the Mood struct and checks ownership in WHERE
func (m *MoodModel) Update(mood *Mood) error { // <-- Mood struct now contains UserID
	if mood.ID < 1 || mood.UserID < 1 {
		return ErrRecordNotFound // Use shared error
	}
	query := `
        UPDATE moods
        SET title = $1, content = $2, emotion = $3, emoji = $4, color = $5, updated_at = NOW()
        WHERE id = $6 AND user_id = $7 -- Added user_id check
        RETURNING updated_at`

	args := []any{mood.Title, mood.Content, mood.Emotion, mood.Emoji, mood.Color, mood.ID, mood.UserID} // Use mood.UserID

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := m.DB.QueryRowContext(ctx, query, args...).Scan(&mood.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Mood ID didn't exist OR didn't belong to this user.
			return ErrRecordNotFound // Use shared error
		}
		return fmt.Errorf("mood update: %w", err)
	}
	return nil
}

// Delete - Requires mood ID and the UserID it must belong to
func (m *MoodModel) Delete(id int64, userID int64) error { // <-- Added userID parameter
	if id < 1 || userID < 1 {
		return ErrRecordNotFound // Use shared error
	}
	query := `DELETE FROM moods WHERE id = $1 AND user_id = $2` // Added user_id check

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	result, err := m.DB.ExecContext(ctx, query, id, userID) // Pass userID as arg
	if err != nil {
		return fmt.Errorf("mood delete exec: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("mood delete rows affected: %w", err)
	}

	if rowsAffected == 0 {
		// Mood ID didn't exist OR didn't belong to this user.
		return ErrRecordNotFound // Use shared error
	}
	return nil
}

// GetFiltered - Now requires UserID in the FilterCriteria
func (m *MoodModel) GetFiltered(filters FilterCriteria) ([]*Mood, Metadata, error) {
	if filters.UserID < 1 {
		// Return empty results and metadata if UserID is invalid, rather than an error,
		// as this might be called internally before auth check in some cases (though shouldn't be).
		// Log a warning instead.
		// Consider returning an error if UserID is absolutely required here.
		// For now, return empty.
		return []*Mood{}, Metadata{}, errors.New("invalid user ID provided for filtering moods") // Return error is safer
	}

	// Start building the WHERE clause, always include user_id check first
	baseQuery := `
        FROM moods
        WHERE user_id = $1` // Filter by user ID is mandatory
	args := []any{filters.UserID}
	paramIndex := 2 // Start next parameter index at 2

	// Add other filters dynamically
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
		} else if filters.Emotion != "" { // Handle if only emotion name is passed (e.g., if filter changes)
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

	// --- Count Query (Includes user_id filter via baseQuery) ---
	totalRecordsQuery := `SELECT count(*) ` + baseQuery
	ctxCount, cancelCount := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancelCount()

	var totalRecords int
	err := m.DB.QueryRowContext(ctxCount, totalRecordsQuery, args...).Scan(&totalRecords)
	if err != nil {
		return nil, Metadata{}, fmt.Errorf("count query execution: %w", err)
	}

	// --- Metadata Calculation (remains the same logic) ---
	if filters.PageSize <= 0 {
		filters.PageSize = 4
	}
	if filters.Page <= 0 {
		filters.Page = 1
	}
	metadata := calculateMetadata(totalRecords, filters.Page, filters.PageSize)
	if totalRecords == 0 { // Optimization: return early if no records match filters
		return []*Mood{}, metadata, nil
	}
	if filters.Page > metadata.LastPage {
		return []*Mood{}, metadata, nil // Requested page beyond last page
	}

	// --- Main Select Query (Includes user_id filter and dynamic filters) ---
	selectQuery := `SELECT id, created_at, updated_at, title, content, emotion, emoji, color, user_id ` + // Added user_id
		baseQuery + // WHERE clause includes user_id and other filters
		` ORDER BY created_at DESC LIMIT $` + fmt.Sprint(paramIndex) +
		` OFFSET $` + fmt.Sprint(paramIndex+1)

	limit := filters.PageSize
	offset := (filters.Page - 1) * filters.PageSize
	queryArgs := append(args, limit, offset) // Append limit and offset to the existing args

	ctxQuery, cancelQuery := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelQuery()

	rows, err := m.DB.QueryContext(ctxQuery, selectQuery, queryArgs...)
	if err != nil {
		return nil, metadata, fmt.Errorf("paginated query execution: %w", err)
	}
	defer rows.Close()

	// Use make with capacity for slight performance improvement
	moods := make([]*Mood, 0, filters.PageSize)
	for rows.Next() {
		var mood Mood
		err := rows.Scan(
			&mood.ID, &mood.CreatedAt, &mood.UpdatedAt,
			&mood.Title, &mood.Content, &mood.Emotion,
			&mood.Emoji, &mood.Color, &mood.UserID, // Added UserID scan
		)
		if err != nil {
			// Return partial results potentially, or fail entirely? Failing is safer.
			return nil, metadata, fmt.Errorf("paginated scan row: %w", err)
		}
		moods = append(moods, &mood)
	}

	if err = rows.Err(); err != nil {
		return nil, metadata, fmt.Errorf("paginated rows iteration: %w", err)
	}

	return moods, metadata, nil
}

// GetDistinctEmotionDetails - Now needs UserID to show only emotions used by that user
func (m *MoodModel) GetDistinctEmotionDetails(userID int64) ([]EmotionDetail, error) { // <-- Added userID parameter
	if userID < 1 {
		return nil, errors.New("invalid user ID provided for distinct emotions")
	}
	query := `
        SELECT DISTINCT emotion, emoji, color FROM moods
        WHERE emotion IS NOT NULL AND emoji IS NOT NULL AND color IS NOT NULL
          AND user_id = $1 -- Added user_id filter
        ORDER BY emotion ASC`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	rows, err := m.DB.QueryContext(ctx, query, userID) // Pass userID
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

// --- Stat Helper Functions Modified to Accept UserID ---

func (m *MoodModel) GetTotalMoodCount(userID int64) (int, error) { // <-- Added userID parameter
	if userID < 1 {
		return 0, errors.New("invalid user ID")
	}
	query := `SELECT COUNT(*) FROM moods WHERE user_id = $1` // Added WHERE
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	var total int
	err := m.DB.QueryRowContext(ctx, query, userID).Scan(&total) // Pass userID
	if err != nil {
		return 0, fmt.Errorf("mood count query: %w", err)
	}
	return total, nil
}

func (m *MoodModel) GetEmotionCounts(userID int64) ([]EmotionCount, error) { // <-- Added userID parameter
	if userID < 1 {
		return nil, errors.New("invalid user ID")
	}
	query := `
        SELECT emotion, emoji, color, COUNT(*)
        FROM moods
        WHERE emotion IS NOT NULL AND emoji IS NOT NULL AND color IS NOT NULL
          AND user_id = $1 -- Added WHERE
        GROUP BY emotion, emoji, color
        ORDER BY COUNT(*) DESC, emotion ASC`
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	rows, err := m.DB.QueryContext(ctx, query, userID) // Pass userID
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

func (m *MoodModel) GetMonthlyEntryCounts(userID int64) ([]MonthlyCount, error) { // <-- Added userID parameter
	if userID < 1 {
		return nil, errors.New("invalid user ID")
	}
	query := `
        SELECT TO_CHAR(created_at, 'Mon YYYY') AS month_year, COUNT(*) as count
        FROM moods
        WHERE user_id = $1 -- Added WHERE
        GROUP BY month_year, date_trunc('month', created_at)
        ORDER BY date_trunc('month', created_at) ASC`
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	rows, err := m.DB.QueryContext(ctx, query, userID) // Pass userID
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

func (m *MoodModel) GetLatestMood(userID int64) (*Mood, error) { // <-- Added userID parameter
	if userID < 1 {
		return nil, errors.New("invalid user ID")
	}
	query := `
        SELECT id, created_at, updated_at, title, content, emotion, emoji, color, user_id -- Added user_id
        FROM moods
        WHERE user_id = $1 -- Added WHERE
        ORDER BY created_at DESC
        LIMIT 1`
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	var mood Mood
	err := m.DB.QueryRowContext(ctx, query, userID).Scan( // Pass userID
		&mood.ID, &mood.CreatedAt, &mood.UpdatedAt,
		&mood.Title, &mood.Content, &mood.Emotion,
		&mood.Emoji, &mood.Color, &mood.UserID, // Scan UserID
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		} // Okay if user has no moods yet
		return nil, fmt.Errorf("latest mood get: %w", err)
	}
	return &mood, nil
}

func (m *MoodModel) GetFirstEntryDate(userID int64) (time.Time, error) { // <-- Added userID parameter
	if userID < 1 {
		return time.Time{}, errors.New("invalid user ID")
	}
	query := `SELECT MIN(created_at) FROM moods WHERE user_id = $1` // Added WHERE
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	var firstDate sql.NullTime
	err := m.DB.QueryRowContext(ctx, query, userID).Scan(&firstDate) // Pass userID
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return time.Time{}, nil
		} // Should be NULL instead
		return time.Time{}, fmt.Errorf("first entry date query: %w", err)
	}
	if !firstDate.Valid {
		return time.Time{}, nil
	} // Correctly handles NULL if no moods for user
	return firstDate.Time, nil
}

// GetAllStats - Now accepts UserID and passes it to helper functions
func (m *MoodModel) GetAllStats(userID int64) (*MoodStats, error) { // <-- Added userID parameter
	if userID < 1 {
		return nil, errors.New("invalid user ID for getting stats")
	}

	total, err := m.GetTotalMoodCount(userID) // Pass userID
	if err != nil {
		return nil, fmt.Errorf("failed to get total count: %w", err)
	}

	stats := &MoodStats{TotalEntries: total, EmotionCounts: []EmotionCount{}, MonthlyCounts: []MonthlyCount{}, AvgEntriesPerWeek: 0.0}
	if total == 0 {
		return stats, nil
	}

	latestMood, err := m.GetLatestMood(userID) // Pass userID
	if err != nil {
		return nil, fmt.Errorf("failed to get latest mood: %w", err)
	}
	stats.LatestMood = latestMood

	emotionCounts, err := m.GetEmotionCounts(userID) // Pass userID
	if err != nil {
		return nil, fmt.Errorf("failed to get emotion counts: %w", err)
	}
	stats.EmotionCounts = emotionCounts
	if len(emotionCounts) > 0 {
		stats.MostCommonEmotion = &emotionCounts[0]
	}

	monthlyCounts, err := m.GetMonthlyEntryCounts(userID) // Pass userID
	if err != nil {
		return nil, fmt.Errorf("failed to get monthly counts: %w", err)
	}
	stats.MonthlyCounts = monthlyCounts

	firstEntryDate, err := m.GetFirstEntryDate(userID) // Pass userID
	if err != nil {
		return nil, fmt.Errorf("failed to get first entry date: %w", err)
	}

	// Calculate AvgEntriesPerWeek (logic remains same)
	if !firstEntryDate.IsZero() {
		duration := time.Since(firstEntryDate)
		weeks := duration.Hours() / (24 * 7)
		if weeks < 1.0 && weeks >= 0 {
			weeks = 1.0
		}
		if weeks > 0 {
			stats.AvgEntriesPerWeek = float64(total) / weeks
		} else {
			stats.AvgEntriesPerWeek = float64(total)
		}
	}

	return stats, nil
}

// --- Remove or Comment Out Deprecated/Unused ---
// func (m *MoodModel) GetAll() ([]*Mood, Metadata, error) { ... }
// func (m *MoodModel) Search(query string) ([]*Mood, Metadata, error) { ... }
