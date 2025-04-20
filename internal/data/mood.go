// mood/internal/data/mood.go
package data

import (
	"context"
	"database/sql"
	"errors"
	"fmt" // <-- Added import
	"strings"
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

// --- NEW: FilterCriteria struct ---
// Holds all possible criteria for filtering moods.
type FilterCriteria struct {
	TextQuery string    // For searching title, content, emotion
	Emotion   string    // Specific emotion to filter by
	StartDate time.Time // Start of date range (inclusive)
	EndDate   time.Time // End of date range (inclusive)
}

// MoodModel struct provides methods for interacting with the mood data.
type MoodModel struct {
	DB *sql.DB
}

// Insert adds a new mood entry into the 'moods' table.
// (No changes to this method)
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
// (No changes to this method)
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
// (No changes to this method)
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
// (No changes to this method)
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

// --- NEW: GetFiltered method ---
// GetFiltered retrieves mood entries based on combined filter criteria.
func (m *MoodModel) GetFiltered(filters FilterCriteria) ([]*Mood, error) {
	// Start with the base query
	baseQuery := `
        SELECT id, created_at, updated_at, title, content, emotion
        FROM moods
        WHERE 1=1` // Base condition to easily append AND clauses

	// Slice to hold query arguments dynamically
	args := []any{}
	paramIndex := 1 // Start parameter index at $1

	// --- Add conditions based on filters ---

	// Text Query (ILIKE on title, content, emotion)
	if filters.TextQuery != "" {
		searchTerm := "%" + strings.TrimSpace(filters.TextQuery) + "%"
		// Note: Using the same parameter index ($1, $1, $1) is okay here as it refers to the same value.
		baseQuery += fmt.Sprintf(" AND (title ILIKE $%d OR content ILIKE $%d OR emotion ILIKE $%d)", paramIndex, paramIndex, paramIndex)
		args = append(args, searchTerm)
		paramIndex++
	}

	// Emotion Filter (Exact match, case-sensitive unless DB collation differs)
	if filters.Emotion != "" {
		baseQuery += fmt.Sprintf(" AND emotion = $%d", paramIndex)
		args = append(args, filters.Emotion)
		paramIndex++
	}

	// Start Date Filter (created_at >= start date)
	// Check if StartDate is not the zero value for time.Time
	if !filters.StartDate.IsZero() {
		baseQuery += fmt.Sprintf(" AND created_at >= $%d", paramIndex)
		args = append(args, filters.StartDate)
		paramIndex++
	}

	// End Date Filter (created_at <= end date)
	// Adjust end date to include the whole day (e.g., up to 23:59:59.999...)
	if !filters.EndDate.IsZero() {
		// Go to the beginning of the *next* day and check for < that.
		// This correctly includes entries made at 23:59:59 on the end date.
		endDateStartOfNextDay := filters.EndDate.Truncate(24 * time.Hour).Add(24 * time.Hour)
		baseQuery += fmt.Sprintf(" AND created_at < $%d", paramIndex)
		args = append(args, endDateStartOfNextDay)
		paramIndex++
	}

	// Add final ordering
	baseQuery += " ORDER BY created_at DESC"

	// --- Execute the query ---
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second) // Increased timeout slightly
	defer cancel()

	// Log the built query and arguments for debugging
	// fmt.Println("Executing Query:", baseQuery)
	// fmt.Println("With Args:", args)

	rows, err := m.DB.QueryContext(ctx, baseQuery, args...)
	if err != nil {
		// Don't wrap sql.ErrNoRows here, QueryContext doesn't return it for empty results
		return nil, fmt.Errorf("error executing filtered query: %w", err)
	}
	defer rows.Close()

	// --- Scan results ---
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
			return nil, fmt.Errorf("error scanning mood row: %w", err)
		}
		moods = append(moods, &mood)
	}

	// Check for errors during iteration
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error during mood row iteration: %w", err)
	}

	// If no errors and slice is empty, it means no rows matched the filters.
	return moods, nil
}

// --- REVISED: Search method (Calls GetFiltered) ---
// Search now acts as a convenience wrapper around GetFiltered.
func (m *MoodModel) Search(query string) ([]*Mood, error) {
	filters := FilterCriteria{
		TextQuery: query,
		// Other filters (Emotion, StartDate, EndDate) are zero/empty by default
	}
	return m.GetFiltered(filters)
}

// --- REVISED: GetAll method (Calls GetFiltered) ---
// GetAll now acts as a convenience wrapper around GetFiltered with no criteria.
func (m *MoodModel) GetAll() ([]*Mood, error) {
	// Call GetFiltered with empty criteria struct to get all moods
	return m.GetFiltered(FilterCriteria{})
}
