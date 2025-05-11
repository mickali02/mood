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

// ValidEmotions defines a list of pre-approved emotion names.
// Presentation Point: "A predefined set of emotions, could be used for default choices or validation."
var ValidEmotions = []string{"Happy", "Sad", "Angry", "Anxious", "Calm", "Excited", "Neutral"}

// --- Struct Definitions ---
// These structs define the shape of our data, both for database interaction and for display (like stats).

// EmotionDetail stores the name, emoji, and color for a distinct emotion.
// Used for populating filter dropdowns
type EmotionDetail struct {
	Name  string
	Emoji string
	Color string
}

// EmotionCount stores an emotion's details along with its occurrence count.
// Used in statistics for displaying emotion distribution. JSON tags define how it's serialized.
type EmotionCount struct {
	Name  string `json:"name"`  // Emotion's name.
	Emoji string `json:"emoji"` // Associated emoji.
	Color string `json:"color"` // Color code for the emotion.
	Count int    `json:"count"` // How many times this emotion was logged.
}

// WeeklyCount stores the count of mood entries for a specific week.
// Used for time-based aggregation in statistics.
type WeeklyCount struct {
	Week  string `json:"week"` // e.g., "2024-23" (Year-WeekNumber)
	Count int    `json:"count"`
}

// MoodStats aggregates all statistics for the stats page.
// This struct is populated and passed to the stats template.
type MoodStats struct {
	TotalEntries      int            `json:"totalEntries"`      // Total number of mood entries.
	MostCommonEmotion *EmotionCount  `json:"mostCommonEmotion"` // Pointer to the most frequent emotion.
	EmotionCounts     []EmotionCount `json:"emotionCounts"`     // Slice of all emotion counts.
	WeeklyCounts      []WeeklyCount  `json:"weeklyCounts"`      // Mood entries count per week.
	LatestMood        *Mood          `json:"latestMood"`        // Pointer to the most recently logged mood.
	AvgEntriesPerWeek float64        `json:"avgEntriesPerWeek"` // Average number of entries logged per week.
}

// FilterCriteria holds parameters for filtering mood entries on the dashboard.
// This struct encapsulates all criteria used for searching and filtering moods.
type FilterCriteria struct {
	TextQuery string    // Search term for title, content, or emotion.
	Emotion   string    // Specific emotion to filter by (e.g., "Happy::😊").
	StartDate time.Time // Start of the date range for filtering.
	EndDate   time.Time // End of the date range.
	Page      int       // Current page number for pagination.
	PageSize  int       // Number of entries per page.
	UserID    int64     // ID of the user whose moods are being filtered (ensures data privacy).
}

// Metadata holds pagination information calculated based on filtered results.
// Used by templates to render pagination controls (like 'Page 1 of 5').
type Metadata struct {
	CurrentPage  int `json:"current_page,omitempty"`  // Current page being displayed.
	PageSize     int `json:"page_size,omitempty"`     // Number of items per page.
	FirstPage    int `json:"first_page,omitempty"`    // Always 1.
	LastPage     int `json:"last_page,omitempty"`     // The total number of pages.
	TotalRecords int `json:"total_records,omitempty"` // Total number of records matching the filter.
}

// calculateMetadata computes pagination metadata.
// Helper function to determine total pages, current page, etc., for pagination.
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

// Mood struct defines the structure of a single mood entry, mapping to the 'moods' database table.
// JSON tags guide how this struct is marshalled/unmarshalled to/from JSON.
// This is the core data model for a mood entry, reflecting the database schema.
type Mood struct {
	ID        int64     `json:"id"`         // Unique identifier (Primary Key).
	CreatedAt time.Time `json:"created_at"` // Timestamp of creation (auto-set by DB).
	UpdatedAt time.Time `json:"updated_at"` // Timestamp of last update (auto-set by DB).
	Title     string    `json:"title"`      // Title of the mood entry.
	Content   string    `json:"content"`    // Detailed content (can be HTML from Quill editor).
	Emotion   string    `json:"emotion"`    // Name of the emotion.
	Emoji     string    `json:"emoji"`      // Emoji representing the emotion.
	Color     string    `json:"color"`      // Hex color code for the emotion.
	UserID    int64     `json:"user_id"`    // Foreign key linking to the 'users' table.
}

// ValidateMood checks the mood struct for adherence to business rules (e.g., non-empty fields, max lengths).
// It uses the custom validator to accumulate errors.
// Server-side validation for mood entries. Ensures data integrity before database operations.
func ValidateMood(v *validator.Validator, mood *Mood) {
	// Validate Title: must be provided and within length limits.
	v.Check(validator.NotBlank(mood.Title), "title", "must be provided")
	v.Check(validator.MaxLength(mood.Title, 100), "title", "must not be more than 100 characters long")

	// Validate Content: Sanitize HTML first, then check if plain text is not blank.
	p := bluemonday.StrictPolicy() // Use HTML sanitizer.
	plainTextContent := p.Sanitize(mood.Content)
	v.Check(validator.NotBlank(plainTextContent), "content", "must be provided")

	// Validate Emotion fields: name, emoji, color.
	v.Check(validator.NotBlank(mood.Emotion), "emotion", "name must be provided")
	v.Check(validator.MaxLength(mood.Emotion, 50), "emotion", "name must not be more than 50 characters long")
	v.Check(validator.NotBlank(mood.Emoji), "emoji", "must be provided")
	v.Check(utf8.RuneCountInString(mood.Emoji) >= 1, "emoji", "must contain at least one character")
	v.Check(utf8.RuneCountInString(mood.Emoji) <= 4, "emoji", "is too long for a typical emoji")
	v.Check(validator.NotBlank(mood.Color), "color", "must be provided")
	v.Check(validator.Matches(mood.Color, validator.HexColorRX), "color", "must be a valid hex color code (e.g., #FFD700)")
}

// MoodModel provides methods for database operations on mood entries.
// It embeds a `*sql.DB` connection pool.
// This 'MoodModel' encapsulates all database logic for moods (CRUD operations).
type MoodModel struct {
	DB *sql.DB
}

// Insert adds a new mood entry to the database.
// The 'Create' part of CRUD. Inserts a new mood, returning its generated ID and timestamps.
func (m *MoodModel) Insert(mood *Mood) error {
	// 1. Validate UserID: Ensure a valid user is associated.
	if mood.UserID < 1 {
		return errors.New("invalid user ID provided for mood insert")
	}

	// 2. SQL Query: Defines the INSERT statement.
	//    `RETURNING id, created_at, updated_at` gets back DB-generated values.
	query := `
        INSERT INTO moods (title, content, emotion, emoji, color, user_id)
        VALUES ($1, $2, $3, $4, $5, $6)
        RETURNING id, created_at, updated_at`

	// 3. Arguments: Prepare arguments for the SQL query.
	args := []any{mood.Title, mood.Content, mood.Emotion, mood.Emoji, mood.Color, mood.UserID}

	// 4. Execute Query: Use a context with timeout for resilience.
	//    `QueryRowContext` executes the query and expects one row in return.
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// 5. Scan Results: Populate the mood struct's ID and timestamps from the returned row.
	err := m.DB.QueryRowContext(ctx, query, args...).Scan(&mood.ID, &mood.CreatedAt, &mood.UpdatedAt)
	if err != nil {
		// Handle specific PostgreSQL errors, like foreign key violation (user_id doesn't exist).
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23503" { // "23503" is foreign_key_violation.
			return fmt.Errorf("mood insert failed: user with ID %d does not exist: %w", mood.UserID, err)
		}
		return fmt.Errorf("mood insert: %w", err)
	}
	return nil
}

// Get retrieves a specific mood entry by its ID and the owner's UserID.
// Including UserID ensures users can only access their own moods.
// The 'Read' part of CRUD. Fetches a single mood, ensuring user ownership.
func (m *MoodModel) Get(id int64, userID int64) (*Mood, error) {
	// 1. Validate Inputs: Ensure IDs are positive.
	if id < 1 || userID < 1 {
		return nil, ErrRecordNotFound // Invalid IDs imply record won't be found.
	}
	// 2. SQL Query: Selects a mood by its ID and the user_id.
	query := `
        SELECT id, created_at, updated_at, title, content, emotion, emoji, color, user_id
        FROM moods
        WHERE id = $1 AND user_id = $2` // Ownership check.

	// 3. Execute Query with Context:
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var mood Mood // Struct to hold the fetched data.
	// 4. Scan Row: Populate the mood struct.
	err := m.DB.QueryRowContext(ctx, query, id, userID).Scan(
		&mood.ID, &mood.CreatedAt, &mood.UpdatedAt,
		&mood.Title, &mood.Content, &mood.Emotion,
		&mood.Emoji, &mood.Color, &mood.UserID,
	)

	// 5. Handle Errors:
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) { // If no rows found, it's a "record not found" error.
			return nil, ErrRecordNotFound
		}
		return nil, fmt.Errorf("mood get: %w", err)
	}
	return &mood, nil
}

// Update modifies an existing mood entry in the database.
// It requires the Mood ID and the owner's UserID for an ownership check.
// The 'Update' part of CRUD. Modifies an existing mood, again checking ownership.
func (m *MoodModel) Update(mood *Mood) error {
	// 1. Validate IDs: Ensure mood and user IDs are valid.
	if mood.ID < 1 || mood.UserID < 1 {
		return ErrRecordNotFound
	}
	// 2. SQL Query: Updates specified fields, sets `updated_at` to current time.
	//    `WHERE` clause includes both `id` and `user_id` for security.
	query := `
        UPDATE moods
        SET title = $1, content = $2, emotion = $3, emoji = $4, color = $5, updated_at = NOW()
        WHERE id = $6 AND user_id = $7
        RETURNING updated_at` // Return the new `updated_at` timestamp.

	args := []any{mood.Title, mood.Content, mood.Emotion, mood.Emoji, mood.Color, mood.ID, mood.UserID}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// 3. Execute and Scan: Update the `UpdatedAt` field in the mood struct.
	err := m.DB.QueryRowContext(ctx, query, args...).Scan(&mood.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) { // If no row was updated (ID/UserID mismatch).
			return ErrRecordNotFound
		}
		return fmt.Errorf("mood update: %w", err)
	}
	return nil
}

// Delete removes a mood entry from the database by its ID and owner's UserID.
// The 'Delete' part of CRUD. Removes a mood entry, with ownership check.
func (m *MoodModel) Delete(id int64, userID int64) error {
	// 1. Validate IDs.
	if id < 1 || userID < 1 {
		return ErrRecordNotFound
	}
	// 2. SQL Query: Deletes based on ID and UserID.
	query := `DELETE FROM moods WHERE id = $1 AND user_id = $2`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// 3. Execute Deletion: `ExecContext` is used as we don't expect rows back.
	result, err := m.DB.ExecContext(ctx, query, id, userID)
	if err != nil {
		return fmt.Errorf("mood delete exec: %w", err)
	}

	// 4. Check Rows Affected: Ensure a row was actually deleted.
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("mood delete rows affected: %w", err)
	}

	if rowsAffected == 0 { // If 0 rows affected, means ID/UserID didn't match.
		return ErrRecordNotFound
	}
	return nil
}

// GetFiltered retrieves a paginated and filtered list of moods for a specific user.
// Powers the dashboard. Dynamically builds SQL for filtering by text, emotion, date, and handles pagination.
func (m *MoodModel) GetFiltered(filters FilterCriteria) ([]*Mood, Metadata, error) {
	// 1. Validate UserID.
	if filters.UserID < 1 {
		return []*Mood{}, Metadata{}, errors.New("invalid user ID provided for filtering moods")
	}

	// 2. Dynamic Query Building: Start with a base query and append conditions.
	baseQuery := `
        FROM moods
        WHERE user_id = $1` // Always filter by the logged-in user.
	args := []any{filters.UserID}
	paramIndex := 2

	// 2a. Add Text Search Filter (if provided).
	//     Searches title, content, and emotion fields case-insensitively.
	if filters.TextQuery != "" {
		searchTerm := "%" + strings.TrimSpace(filters.TextQuery) + "%"
		// ILIKE for case-insensitive search.
		baseQuery += fmt.Sprintf(" AND (title ILIKE $%d OR content ILIKE $%d OR emotion ILIKE $%d)", paramIndex, paramIndex, paramIndex)
		args = append(args, searchTerm)
		paramIndex++
	}
	// 2b. Add Emotion Filter (if provided).
	//     Handles combined "EmotionName::Emoji" format from dropdown.
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
		} else if filters.Emotion != "" {
			baseQuery += fmt.Sprintf(" AND emotion = $%d", paramIndex)
			args = append(args, filters.Emotion)
			paramIndex++
		}
	}
	// 2c. Add Date Filters (if provided).
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

	// 3. Get Total Record Count (for pagination).
	//    Executes a `COUNT(*)` query with the same filters.
	totalRecordsQuery := `SELECT count(*) ` + baseQuery
	ctxCount, cancelCount := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancelCount()

	var totalRecords int
	err := m.DB.QueryRowContext(ctxCount, totalRecordsQuery, args...).Scan(&totalRecords)
	if err != nil {
		return nil, Metadata{}, fmt.Errorf("count query execution: %w", err)
	}

	// 4. Calculate Pagination Metadata.
	if filters.PageSize <= 0 {
		filters.PageSize = 4
	}
	if filters.Page <= 0 {
		filters.Page = 1
	}
	metadata := calculateMetadata(totalRecords, filters.Page, filters.PageSize)
	// If no records or requested page is beyond last page, return empty results.
	if totalRecords == 0 {
		return []*Mood{}, metadata, nil
	}
	if filters.Page > metadata.LastPage {
		return []*Mood{}, metadata, nil
	}

	// 5. Construct Final Select Query with Ordering, Limit, and Offset.
	//    Orders by `created_at DESC` to show newest first.
	//    `LIMIT` for page size, `OFFSET` for current page.
	selectQuery := `SELECT id, created_at, updated_at, title, content, emotion, emoji, color, user_id ` +
		baseQuery + // Filter conditions.
		` ORDER BY created_at DESC LIMIT $` + fmt.Sprint(paramIndex) + // ORDER BY and LIMIT.
		` OFFSET $` + fmt.Sprint(paramIndex+1) // OFFSET.

	limit := filters.PageSize
	offset := (filters.Page - 1) * filters.PageSize
	queryArgs := append(args, limit, offset) // Add limit and offset to arguments.

	// 6. Execute Paginated Query.
	ctxQuery, cancelQuery := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelQuery()

	rows, err := m.DB.QueryContext(ctxQuery, selectQuery, queryArgs...)
	if err != nil {
		return nil, metadata, fmt.Errorf("paginated query execution: %w", err)
	}
	defer rows.Close()

	// 7. Scan Results into Mood Structs.
	moods := make([]*Mood, 0, filters.PageSize) // Pre-allocate slice capacity.
	for rows.Next() {
		var mood Mood
		err := rows.Scan(
			&mood.ID, &mood.CreatedAt, &mood.UpdatedAt,
			&mood.Title, &mood.Content, &mood.Emotion,
			&mood.Emoji, &mood.Color, &mood.UserID,
		)
		if err != nil {
			return nil, metadata, fmt.Errorf("paginated scan row: %w", err)
		}
		moods = append(moods, &mood)
	}

	if err = rows.Err(); err != nil { // Check for errors during row iteration.
		return nil, metadata, fmt.Errorf("paginated rows iteration: %w", err)
	}

	return moods, metadata, nil // Return fetched moods and pagination info.

}

// GetDistinctEmotionDetails fetches unique emotion, emoji, and color combinations logged by a user.
// Used to populate the emotion filter dropdown on the dashboard.
// Helper to get unique emotions for the filter dropdown, making it user-specific.
func (m *MoodModel) GetDistinctEmotionDetails(userID int64) ([]EmotionDetail, error) {
	// 1. Validate UserID.
	if userID < 1 {
		return nil, errors.New("invalid user ID provided for distinct emotions")
	}
	// 2. SQL Query: Selects distinct combinations, ordered by emotion name.
	query := `
        SELECT DISTINCT emotion, emoji, color FROM moods
        WHERE emotion IS NOT NULL AND emoji IS NOT NULL AND color IS NOT NULL
          AND user_id = $1
        ORDER BY emotion ASC`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// 3. Execute Query.
	rows, err := m.DB.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("distinct emotion query: %w", err)
	}
	defer rows.Close()

	// 4. Scan Results into EmotionDetail structs.
	emotionDetailsList := make([]EmotionDetail, 0)
	for rows.Next() {
		var detail EmotionDetail
		err := rows.Scan(&detail.Name, &detail.Emoji, &detail.Color)
		if err != nil {
			return nil, fmt.Errorf("distinct emotion scan: %w", err)
		}
		emotionDetailsList = append(emotionDetailsList, detail)
	}
	if err = rows.Err(); err != nil { // Check for iteration errors.
		return nil, fmt.Errorf("distinct emotion rows iteration: %w", err)
	}
	return emotionDetailsList, nil
}

// --- Stat Helper Functions (User-Specific) ---
// These are helpers for the Stats page, calculating various metrics from the user's mood data.

// GetTotalMoodCount returns the total number of mood entries for a user

func (m *MoodModel) GetTotalMoodCount(userID int64) (int, error) {
	// ... (Implementation with UserID check, SQL query, context, scan) ...
	if userID < 1 {
		return 0, errors.New("invalid user ID")
	}
	query := `SELECT COUNT(*) FROM moods WHERE user_id = $1`
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	var total int
	err := m.DB.QueryRowContext(ctx, query, userID).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("mood count query: %w", err)
	}
	return total, nil
}

// GetEmotionCounts returns a list of emotions and their counts for a user, ordered by frequency.
func (m *MoodModel) GetEmotionCounts(userID int64) ([]EmotionCount, error) {
	// ... (Implementation with UserID check, SQL query with GROUP BY and ORDER BY, context, scan loop) ...
	if userID < 1 {
		return nil, errors.New("invalid user ID")
	}
	query := `
        SELECT emotion, emoji, color, COUNT(*)
        FROM moods
        WHERE emotion IS NOT NULL AND emoji IS NOT NULL AND color IS NOT NULL
          AND user_id = $1
        GROUP BY emotion, emoji, color
        ORDER BY COUNT(*) DESC, emotion ASC`
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	rows, err := m.DB.QueryContext(ctx, query, userID)
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

// GetWeeklyEntryCounts fetches mood entry counts grouped by ISO week for a user.
func (m *MoodModel) GetWeeklyEntryCounts(userID int64) ([]WeeklyCount, error) {
	// ... (Implementation with UserID check, SQL query using TO_CHAR for week, GROUP BY, context, scan loop) ...
	if userID < 1 {
		return nil, errors.New("invalid user ID")
	}
	query := `
        SELECT
            TO_CHAR(created_at, 'IYYY-IW') AS week_year,
            COUNT(*) as count
        FROM
            moods
        WHERE
            user_id = $1
        GROUP BY
            week_year,
            date_trunc('week', created_at)
        ORDER BY
            date_trunc('week', created_at) ASC;
    `
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := m.DB.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("weekly counts query: %w", err)
	}
	defer rows.Close()

	counts := []WeeklyCount{}
	for rows.Next() {
		var wc WeeklyCount
		err := rows.Scan(&wc.Week, &wc.Count)
		if err != nil {
			return nil, fmt.Errorf("weekly counts scan: %w", err)
		}
		counts = append(counts, wc)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("weekly counts rows iteration: %w", err)
	}
	return counts, nil
}

// GetLatestMood fetches the most recent mood entry for a user.
func (m *MoodModel) GetLatestMood(userID int64) (*Mood, error) {
	// ... (Implementation with UserID check, SQL query with ORDER BY created_at DESC LIMIT 1, context, scan) ...
	if userID < 1 {
		return nil, errors.New("invalid user ID")
	}
	query := `
        SELECT id, created_at, updated_at, title, content, emotion, emoji, color, user_id
        FROM moods
        WHERE user_id = $1
        ORDER BY created_at DESC
        LIMIT 1`
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	var mood Mood
	err := m.DB.QueryRowContext(ctx, query, userID).Scan(
		&mood.ID, &mood.CreatedAt, &mood.UpdatedAt,
		&mood.Title, &mood.Content, &mood.Emotion,
		&mood.Emoji, &mood.Color, &mood.UserID,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("latest mood get: %w", err)
	}
	return &mood, nil
}

// GetFirstEntryDate fetches the timestamp of the user's very first mood entry.
// Used to calculate the duration for average entries per week.
func (m *MoodModel) GetFirstEntryDate(userID int64) (time.Time, error) {
	// ... (Implementation with UserID check, SQL query with MIN(created_at), context, scan into sql.NullTime) ...
	if userID < 1 {
		return time.Time{}, errors.New("invalid user ID")
	}
	query := `SELECT MIN(created_at) FROM moods WHERE user_id = $1`
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	var firstDate sql.NullTime
	err := m.DB.QueryRowContext(ctx, query, userID).Scan(&firstDate)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return time.Time{}, nil
		}
		return time.Time{}, fmt.Errorf("first entry date query: %w", err)
	}
	if !firstDate.Valid {
		return time.Time{}, nil
	}
	return firstDate.Time, nil
}

// GetAllStats - Fetches all stats, now using weekly counts
func (m *MoodModel) GetAllStats(userID int64) (*MoodStats, error) {
	// 1. Validate UserID.
	if userID < 1 {
		return nil, errors.New("invalid user ID for getting stats")
	}

	// 2. Get Total Entries.
	total, err := m.GetTotalMoodCount(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get total count: %w", err)
	}

	// 3. Initialize Stats Struct.
	stats := &MoodStats{
		TotalEntries:      total,
		EmotionCounts:     []EmotionCount{},
		WeeklyCounts:      []WeeklyCount{},
		AvgEntriesPerWeek: 0.0,
	}

	// 4. Early Exit if No Entries: If no moods, no further stats to calculate.
	if total == 0 {
		return stats, nil
	}

	// 5. Fetch Latest Mood.
	latestMood, err := m.GetLatestMood(userID)
	if err != nil {
		// Don't return error if it's just sql.ErrNoRows
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("failed to get latest mood: %w", err)
		}
		// latestMood will remain nil, which is acceptable
	}
	stats.LatestMood = latestMood

	// 6. Fetch Emotion Counts and Determine Most Common.
	emotionCounts, err := m.GetEmotionCounts(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get emotion counts: %w", err)
	}
	stats.EmotionCounts = emotionCounts
	if len(emotionCounts) > 0 {
		stats.MostCommonEmotion = &emotionCounts[0] // Assumes GetEmotionCounts orders by frequency.
	}

	// 7. Fetch Weekly Counts.
	weeklyCounts, err := m.GetWeeklyEntryCounts(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get weekly counts: %w", err)
	}
	stats.WeeklyCounts = weeklyCounts

	// 8. Fetch First Entry Date (for calculating average).
	firstEntryDate, err := m.GetFirstEntryDate(userID)
	if err != nil { // GetFirstEntryDate handles ErrNoRows by returning zero time.
		return nil, fmt.Errorf("failed to get first entry date: %w", err)
	}

	// 9. Calculate Average Entries Per Week.
	if !firstEntryDate.IsZero() { // Only if there's a first entry.
		duration := time.Since(firstEntryDate) // Duration since first entry.
		weeks := duration.Hours() / (24 * 7)   // Convert duration to weeks.
		if weeks < 1.0 && weeks >= 0 {         // If less than a week, count as 1 week.
			weeks = 1.0
		}
		if weeks > 0 {
			stats.AvgEntriesPerWeek = float64(total) / weeks
		} else { // Should not happen if weeks >= 1, but defensive.
			stats.AvgEntriesPerWeek = float64(total) // Or 0, depending on desired behavior for edge case.
		}
	}

	return stats, nil
}

// DeleteAllByUserID removes all mood entries for a specific user.
// Used for the "Reset Entries" feature on the profile page.
// Data management: Allows a user to clear all their mood data.
func (m *MoodModel) DeleteAllByUserID(userID int64) error {
	// 1. Validate UserID.
	if userID < 1 {
		return errors.New("invalid user ID provided for deleting moods")
	}

	// 2. SQL Query: Deletes all moods where user_id matches.
	query := `DELETE FROM moods WHERE user_id = $1`

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second) // Longer timeout for potentially many deletes.
	defer cancel()

	result, err := m.DB.ExecContext(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("mood delete all by user_id exec: %w", err)
	}

	_, err = result.RowsAffected()
	if err != nil {
		return fmt.Errorf("mood delete all by user_id rows affected: %w", err)
	}

	return nil
}
