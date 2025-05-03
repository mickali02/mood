// internal/data/mood_test.go
package data

import (
	"context"      // Used in newTestDB
	"database/sql" // Used throughout
	"errors"       // Used for error checking
	"os"           // Used in newTestDB for os.Getenv
	"reflect"      // For DeepEqual comparison
	"testing"      // Core testing package
	"time"         // Used for time operations

	_ "github.com/lib/pq" // Import the driver anonymously

	// Import validator package to access its functions/variables
	"github.com/mickali02/mood/internal/validator"
)

// --- Test Helper Functions ---

// newTestDB connects to the test database defined by MOODNOTES_TEST_DB_DSN.
// It fatally logs the test if connection fails.
func newTestDB(t *testing.T) *sql.DB {
	t.Helper() // Marks this as a test helper

	dsn := os.Getenv("MOODNOTES_TEST_DB_DSN")
	if dsn == "" {
		t.Fatal("MOODNOTES_TEST_DB_DSN environment variable not set")
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("Failed to open test database connection: %s", err)
	}

	// Ping the database to verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = db.PingContext(ctx)
	if err != nil {
		db.Close() // Close before fataling
		t.Fatalf("Failed to ping test database (%s): %s", dsn, err)
	}

	// Optional: Check if migrations were applied - good sanity check
	// We expect the schema_migrations table to exist.
	var exists bool
	err = db.QueryRowContext(ctx,
		"SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_schema = 'public' AND table_name = 'schema_migrations')").Scan(&exists)
	if err != nil || !exists {
		db.Close()
		t.Fatalf("Test DB schema check failed (schema_migrations table not found or error: %v). Did you run 'make testdb/migrations/up'?", err)
	}

	return db
}

// cleanupTestDB removes all data from the moods table and resets sequences.
// It ensures tests start with a clean slate.
func cleanupTestDB(t *testing.T, db *sql.DB) {
	t.Helper()
	// TRUNCATE is faster than DELETE and resets sequences with RESTART IDENTITY.
	// CASCADE handles potential foreign key dependencies if added later.
	_, err := db.Exec("TRUNCATE TABLE moods RESTART IDENTITY CASCADE")
	if err != nil {
		// Use t.Fatalf to stop the test immediately on cleanup failure,
		// as it indicates a serious problem preventing test isolation.
		t.Fatalf("Failed to cleanup test database (truncate moods): %s", err)
	}
}

// --- Test Functions for Statistics ---
// (Keep the existing passing tests for statistics:
// TestMoodModel_GetTotalMoodCount, TestMoodModel_GetEmotionCounts,
// TestMoodModel_GetMonthlyEntryCounts, TestMoodModel_GetAllStats)
func TestMoodModel_GetTotalMoodCount(t *testing.T) {
	if testing.Short() {
		t.Skip("postgres: skipping integration test in short mode")
	}
	db := newTestDB(t)
	defer db.Close()
	defer cleanupTestDB(t, db)
	model := MoodModel{DB: db}
	t.Run("NoEntries", func(t *testing.T) {
		count, err := model.GetTotalMoodCount()
		if err != nil {
			t.Fatalf("Expected no error, got %s", err)
		}
		if count != 0 {
			t.Errorf("Expected count 0, got %d", count)
		}
	})
	t.Run("WithEntries", func(t *testing.T) {
		_, err := db.Exec(`INSERT INTO moods (title, content, emotion, emoji, color) VALUES ('T1','','H','h','#fff'), ('T2','','S','s','#000')`)
		if err != nil {
			t.Fatalf("Failed to insert test data: %s", err)
		}
		count, err := model.GetTotalMoodCount()
		if err != nil {
			t.Fatalf("Expected no error, got %s", err)
		}
		if count != 2 {
			t.Errorf("Expected count 2, got %d", count)
		}
	})
}

func TestMoodModel_GetEmotionCounts(t *testing.T) {
	if testing.Short() {
		t.Skip("postgres: skipping integration test in short mode")
	}
	db := newTestDB(t)
	defer db.Close()
	defer cleanupTestDB(t, db)
	model := MoodModel{DB: db}
	t.Run("NoEntries", func(t *testing.T) {
		counts, err := model.GetEmotionCounts()
		if err != nil {
			t.Fatalf("Expected no error, got %s", err)
		}
		if len(counts) != 0 {
			t.Errorf("Expected 0 emotion counts, got %d", len(counts))
		}
	})
	t.Run("WithEntries", func(t *testing.T) {
		_, err := db.Exec(`INSERT INTO moods (title, content, emotion, emoji, color) VALUES ('T1','','Happy','😊','#FFD700'),('T2','','Sad','😢','#6495ED'),('T3','','Happy','😊','#FFD700'),('T4','','Calm','😌','#90EE90'),('T5','','Happy','😊','#FFD700')`)
		if err != nil {
			t.Fatalf("Failed to insert test data: %s", err)
		}
		counts, err := model.GetEmotionCounts()
		if err != nil {
			t.Fatalf("Expected no error, got %s", err)
		}
		expected := []EmotionCount{{Name: "Happy", Emoji: "😊", Color: "#FFD700", Count: 3}, {Name: "Calm", Emoji: "😌", Color: "#90EE90", Count: 1}, {Name: "Sad", Emoji: "😢", Color: "#6495ED", Count: 1}}
		if !reflect.DeepEqual(counts, expected) {
			t.Errorf("Mismatch in emotion counts.\nExpected: %+v\nGot:      %+v", expected, counts)
		}
	})
}

func TestMoodModel_GetMonthlyEntryCounts(t *testing.T) {
	if testing.Short() {
		t.Skip("postgres: skipping integration test in short mode")
	}
	db := newTestDB(t)
	defer db.Close()
	defer cleanupTestDB(t, db)
	model := MoodModel{DB: db}
	t.Run("NoEntries", func(t *testing.T) {
		counts, err := model.GetMonthlyEntryCounts()
		if err != nil {
			t.Fatalf("Expected no error, got %s", err)
		}
		if len(counts) != 0 {
			t.Errorf("Expected 0 monthly counts, got %d", len(counts))
		}
	})
	t.Run("WithEntries", func(t *testing.T) {
		_, err := db.Exec(`INSERT INTO moods (title, content, emotion, emoji, color, created_at) VALUES ('Jan1','','N','😐','#B0C4DE','2024-01-15 10:00:00+00'),('Feb1','','H','😊','#FFD700','2024-02-05 11:00:00+00'),('Jan2','','S','😢','#6495ED','2024-01-20 12:00:00+00'),('Feb2','','H','😊','#FFD700','2024-02-25 13:00:00+00'),('Old','','C','😌','#90EE90','2023-12-10 09:00:00+00')`)
		if err != nil {
			t.Fatalf("Failed to insert test data: %s", err)
		}
		counts, err := model.GetMonthlyEntryCounts()
		if err != nil {
			t.Fatalf("Expected no error, got %s", err)
		}
		expected := []MonthlyCount{{Month: "Dec 2023", Count: 1}, {Month: "Jan 2024", Count: 2}, {Month: "Feb 2024", Count: 2}}
		if !reflect.DeepEqual(counts, expected) {
			t.Errorf("Mismatch in monthly counts.\nExpected: %+v\nGot:      %+v", expected, counts)
		}
	})
}

func TestMoodModel_GetAllStats(t *testing.T) {
	if testing.Short() {
		t.Skip("postgres: skipping integration test in short mode")
	}
	db := newTestDB(t)
	defer db.Close()
	defer cleanupTestDB(t, db)
	model := MoodModel{DB: db}
	t.Run("NoEntries", func(t *testing.T) {
		stats, err := model.GetAllStats()
		if err != nil {
			t.Fatalf("Expected no error, got %s", err)
		}
		if stats == nil {
			t.Fatal("Expected stats struct, got nil")
		}
		if stats.TotalEntries != 0 {
			t.Errorf("Expected TotalEntries 0, got %d", stats.TotalEntries)
		}
		if stats.MostCommonEmotion != nil {
			t.Errorf("Expected MostCommonEmotion nil, got %+v", stats.MostCommonEmotion)
		}
		if len(stats.EmotionCounts) != 0 {
			t.Errorf("Expected 0 EmotionCounts, got %d", len(stats.EmotionCounts))
		}
		if len(stats.MonthlyCounts) != 0 {
			t.Errorf("Expected 0 MonthlyCounts, got %d", len(stats.MonthlyCounts))
		}
	})
	t.Run("WithEntries", func(t *testing.T) {
		_, err := db.Exec(`INSERT INTO moods (title, content, emotion, emoji, color, created_at) VALUES ('JH','','Happy','😊','#FFD700','2024-01-10 10:00:00+00'),('JS','','Sad','😢','#6495ED','2024-01-15 11:00:00+00'),('FH1','','Happy','😊','#FFD700','2024-02-05 12:00:00+00'),('FH2','','Happy','😊','#FFD700','2024-02-20 13:00:00+00'),('FC','','Calm','😌','#90EE90','2024-02-25 14:00:00+00')`)
		if err != nil {
			t.Fatalf("Failed to insert test data: %s", err)
		}
		stats, err := model.GetAllStats()
		if err != nil {
			t.Fatalf("Expected no error, got %s", err)
		}
		if stats == nil {
			t.Fatal("Expected stats struct, got nil")
		}
		if stats.TotalEntries != 5 {
			t.Errorf("Expected TotalEntries 5, got %d", stats.TotalEntries)
		}
		expectedMostCommon := EmotionCount{Name: "Happy", Emoji: "😊", Color: "#FFD700", Count: 3}
		if stats.MostCommonEmotion == nil {
			t.Fatalf("Expected MostCommonEmotion %+v, got nil", expectedMostCommon)
		}
		if !reflect.DeepEqual(*stats.MostCommonEmotion, expectedMostCommon) {
			t.Errorf("Mismatch in MostCommonEmotion.\nExpected: %+v\nGot:      %+v", expectedMostCommon, *stats.MostCommonEmotion)
		}
		expectedEmotionCounts := []EmotionCount{{Name: "Happy", Emoji: "😊", Color: "#FFD700", Count: 3}, {Name: "Calm", Emoji: "😌", Color: "#90EE90", Count: 1}, {Name: "Sad", Emoji: "😢", Color: "#6495ED", Count: 1}}
		if !reflect.DeepEqual(stats.EmotionCounts, expectedEmotionCounts) {
			t.Errorf("Mismatch in EmotionCounts.\nExpected: %+v\nGot:      %+v", expectedEmotionCounts, stats.EmotionCounts)
		}
		expectedMonthlyCounts := []MonthlyCount{{Month: "Jan 2024", Count: 2}, {Month: "Feb 2024", Count: 3}}
		if !reflect.DeepEqual(stats.MonthlyCounts, expectedMonthlyCounts) {
			t.Errorf("Mismatch in MonthlyCounts.\nExpected: %+v\nGot:      %+v", expectedMonthlyCounts, stats.MonthlyCounts)
		}
	})
}

// --- NEW Test Functions for CRUD and Others ---

func TestMoodModel_Insert(t *testing.T) {
	if testing.Short() {
		t.Skip("postgres: skipping integration test in short mode")
	}
	db := newTestDB(t)
	defer db.Close()
	defer cleanupTestDB(t, db)
	model := MoodModel{DB: db}

	t.Run("InsertValidMood", func(t *testing.T) {
		mood := &Mood{
			Title:   "Test Insert",
			Content: "<p>This is content</p>",
			Emotion: "Neutral",
			Emoji:   "😐",
			Color:   "#B0C4DE",
		}

		err := model.Insert(mood)
		if err != nil {
			t.Fatalf("Insert failed: %v", err)
		}

		// Check if ID and Timestamps were populated
		if mood.ID == 0 {
			t.Errorf("Expected non-zero ID after insert, got 0")
		}
		if mood.CreatedAt.IsZero() {
			t.Errorf("Expected non-zero CreatedAt after insert")
		}
		if mood.UpdatedAt.IsZero() {
			t.Errorf("Expected non-zero UpdatedAt after insert")
		}

		// Optional: Verify by fetching
		fetchedMood, err := model.Get(mood.ID)
		if err != nil {
			t.Fatalf("Failed to fetch inserted mood: %v", err)
		}
		if fetchedMood == nil {
			t.Fatal("Fetched mood is nil after insert")
		}
		if fetchedMood.Title != mood.Title {
			t.Errorf("Expected Title %q, got %q", mood.Title, fetchedMood.Title)
		}
	})
}

func TestMoodModel_Get(t *testing.T) {
	if testing.Short() {
		t.Skip("postgres: skipping integration test in short mode")
	}
	db := newTestDB(t)
	defer db.Close()
	defer cleanupTestDB(t, db)
	model := MoodModel{DB: db}

	// Setup: Insert a mood to fetch
	insertedMood := &Mood{
		Title:   "Test Get",
		Content: "Content to get",
		Emotion: "Happy",
		Emoji:   "😊",
		Color:   "#FFD700",
	}
	err := model.Insert(insertedMood) // Use Insert helper
	if err != nil || insertedMood.ID == 0 {
		t.Fatalf("Setup failed: Could not insert mood for Get test: %v", err)
	}

	t.Run("GetExisting", func(t *testing.T) {
		fetchedMood, err := model.Get(insertedMood.ID)

		if err != nil {
			t.Fatalf("Get failed for existing ID %d: %v", insertedMood.ID, err)
		}
		if fetchedMood == nil {
			t.Fatal("Get returned nil for existing ID")
		}

		// Compare relevant fields (don't compare timestamps directly if precision differs)
		if fetchedMood.ID != insertedMood.ID ||
			fetchedMood.Title != insertedMood.Title ||
			fetchedMood.Content != insertedMood.Content ||
			fetchedMood.Emotion != insertedMood.Emotion ||
			fetchedMood.Emoji != insertedMood.Emoji ||
			fetchedMood.Color != insertedMood.Color {
			t.Errorf("Mismatch between inserted and fetched mood.\nExpected: %+v\nGot:      %+v", insertedMood, fetchedMood)
		}
		// Check timestamps are not zero
		if fetchedMood.CreatedAt.IsZero() || fetchedMood.UpdatedAt.IsZero() {
			t.Error("Fetched mood has zero timestamp(s)")
		}
	})

	t.Run("GetNonExistentPositiveID", func(t *testing.T) {
		nonExistentID := int64(999999)
		_, err := model.Get(nonExistentID)

		if !errors.Is(err, sql.ErrNoRows) {
			t.Errorf("Expected sql.ErrNoRows for non-existent ID %d, got %v", nonExistentID, err)
		}
	})

	t.Run("GetZeroID", func(t *testing.T) {
		_, err := model.Get(0)
		// The Get function itself should return ErrNoRows for ID < 1
		if !errors.Is(err, sql.ErrNoRows) {
			t.Errorf("Expected sql.ErrNoRows for ID 0, got %v", err)
		}
	})

	t.Run("GetNegativeID", func(t *testing.T) {
		_, err := model.Get(-1)
		if !errors.Is(err, sql.ErrNoRows) {
			t.Errorf("Expected sql.ErrNoRows for ID -1, got %v", err)
		}
	})
}

func TestMoodModel_Update(t *testing.T) {
	if testing.Short() {
		t.Skip("postgres: skipping integration test in short mode")
	}
	db := newTestDB(t)
	defer db.Close()
	defer cleanupTestDB(t, db)
	model := MoodModel{DB: db}

	// Setup: Insert a mood to update
	originalMood := &Mood{
		Title:   "Original Title",
		Content: "Original Content",
		Emotion: "Sad",
		Emoji:   "😢",
		Color:   "#6495ED",
	}
	err := model.Insert(originalMood)
	if err != nil || originalMood.ID == 0 {
		t.Fatalf("Setup failed: Could not insert mood for Update test: %v", err)
	}
	originalUpdatedAt := originalMood.UpdatedAt // Store original update time

	// Short delay to ensure UpdatedAt changes noticeably
	time.Sleep(10 * time.Millisecond)

	t.Run("UpdateExisting", func(t *testing.T) {
		moodToUpdate := &Mood{
			ID:      originalMood.ID, // Use the same ID
			Title:   "Updated Title",
			Content: "Updated Content",
			Emotion: "Excited",
			Emoji:   "🤩",
			Color:   "#FF69B4",
			// CreatedAt should not be set here - Update doesn't modify it
		}

		err := model.Update(moodToUpdate)
		if err != nil {
			t.Fatalf("Update failed for existing ID %d: %v", moodToUpdate.ID, err)
		}

		// Verify by fetching
		updatedMood, errGet := model.Get(originalMood.ID)
		if errGet != nil {
			t.Fatalf("Failed to fetch mood after update: %v", errGet)
		}

		if updatedMood.Title != moodToUpdate.Title ||
			updatedMood.Content != moodToUpdate.Content ||
			updatedMood.Emotion != moodToUpdate.Emotion ||
			updatedMood.Emoji != moodToUpdate.Emoji ||
			updatedMood.Color != moodToUpdate.Color {
			t.Errorf("Mismatch between updated data and fetched mood.\nExpected Update: %+v\nGot Fetch:      %+v", moodToUpdate, updatedMood)
		}

		// Check UpdatedAt was modified
		if !updatedMood.UpdatedAt.After(originalUpdatedAt) {
			t.Errorf("Expected UpdatedAt (%v) to be after original (%v)", updatedMood.UpdatedAt, originalUpdatedAt)
		}
		// Check CreatedAt was NOT modified
		// Use time.Equal for precise timestamp comparison
		if !updatedMood.CreatedAt.Equal(originalMood.CreatedAt) {
			t.Errorf("Expected CreatedAt to remain unchanged, original %v, got %v", originalMood.CreatedAt, updatedMood.CreatedAt)
		}
	})

	t.Run("UpdateNonExistent", func(t *testing.T) {
		moodToUpdate := &Mood{
			ID:      999999, // Non-existent ID
			Title:   "Doesn't Matter",
			Content: "Doesn't Matter",
			Emotion: "Neutral",
			Emoji:   "😐",
			Color:   "#B0C4DE",
		}
		err := model.Update(moodToUpdate)
		if !errors.Is(err, sql.ErrNoRows) {
			t.Errorf("Expected sql.ErrNoRows when updating non-existent ID, got %v", err)
		}
	})
}

func TestMoodModel_Delete(t *testing.T) {
	if testing.Short() {
		t.Skip("postgres: skipping integration test in short mode")
	}
	db := newTestDB(t)
	defer db.Close()
	defer cleanupTestDB(t, db)
	model := MoodModel{DB: db}

	// Setup: Insert a mood to delete
	moodToDelete := &Mood{
		Title:   "To Be Deleted",
		Content: "Delete me",
		Emotion: "Angry",
		Emoji:   "😠",
		Color:   "#DC143C",
	}
	err := model.Insert(moodToDelete)
	if err != nil || moodToDelete.ID == 0 {
		t.Fatalf("Setup failed: Could not insert mood for Delete test: %v", err)
	}
	existingID := moodToDelete.ID // Store the ID

	// Setup: Insert another mood that should remain
	moodToKeep := &Mood{Title: "Keep Me", Content: "", Emotion: "Happy", Emoji: "😊", Color: "#FFD700"}
	err = model.Insert(moodToKeep) // Assign error to check it
	if err != nil {
		t.Fatalf("Setup failed: Could not insert mood to keep: %v", err)
	}

	t.Run("DeleteExisting", func(t *testing.T) {
		err := model.Delete(existingID)
		if err != nil {
			t.Fatalf("Delete failed for existing ID %d: %v", existingID, err)
		}

		// Verify it's gone
		_, errGet := model.Get(existingID)
		if !errors.Is(errGet, sql.ErrNoRows) {
			t.Errorf("Expected sql.ErrNoRows after deleting ID %d, got %v", existingID, errGet)
		}

		// Verify other data is still present
		keptMood, errGetKeep := model.Get(moodToKeep.ID)
		if errGetKeep != nil || keptMood == nil {
			t.Errorf("Mood that should have been kept (ID %d) was affected by delete or fetch failed: %v", moodToKeep.ID, errGetKeep)
		}
	})

	t.Run("DeleteNonExistent", func(t *testing.T) {
		nonExistentID := int64(999999)
		err := model.Delete(nonExistentID)
		if !errors.Is(err, sql.ErrNoRows) {
			t.Errorf("Expected sql.ErrNoRows when deleting non-existent ID %d, got %v", nonExistentID, err)
		}
	})

	t.Run("DeleteZeroID", func(t *testing.T) {
		err := model.Delete(0)
		// The Delete function itself should return ErrNoRows for ID < 1
		if !errors.Is(err, sql.ErrNoRows) {
			t.Errorf("Expected sql.ErrNoRows when deleting ID 0, got %v", err)
		}
	})
}

func TestMoodModel_GetDistinctEmotionDetails(t *testing.T) {
	if testing.Short() {
		t.Skip("postgres: skipping integration test in short mode")
	}
	db := newTestDB(t)
	defer db.Close()
	defer cleanupTestDB(t, db)
	model := MoodModel{DB: db}

	t.Run("NoEntries", func(t *testing.T) {
		details, err := model.GetDistinctEmotionDetails()
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if len(details) != 0 {
			t.Errorf("Expected empty slice, got %d items", len(details))
		}
	})

	t.Run("WithEntriesAndDuplicates", func(t *testing.T) {
		// Insert test data with duplicates
		_, err := db.Exec(`
            INSERT INTO moods (title, content, emotion, emoji, color) VALUES
            ('E1', '', 'Happy', '😊', '#FFD700'),
            ('E2', '', 'Sad', '😢', '#6495ED'),
            ('E3', '', 'Happy', '😊', '#FFD700'), -- Duplicate Happy
            ('E4', '', 'Calm', '😌', '#90EE90'),
            ('E5', '', 'Sad', '😢', '#6495ED'),   -- Duplicate Sad
            ('E6', '', 'Excited', '🤩', '#FF69B4'),
            ('E7', '', 'Happy', '😊', '#FFD700')  -- Duplicate Happy
        `)
		if err != nil {
			t.Fatalf("Failed to insert test data: %v", err)
		}

		details, err := model.GetDistinctEmotionDetails()
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		// Expected result (distinct, sorted alphabetically by Name)
		expected := []EmotionDetail{
			{Name: "Calm", Emoji: "😌", Color: "#90EE90"},
			{Name: "Excited", Emoji: "🤩", Color: "#FF69B4"},
			{Name: "Happy", Emoji: "😊", Color: "#FFD700"},
			{Name: "Sad", Emoji: "😢", Color: "#6495ED"},
		}

		if !reflect.DeepEqual(details, expected) {
			t.Errorf("Mismatch in distinct emotion details.\nExpected: %+v\nGot:      %+v", expected, details)
		}
	})
}

// NOTE: Testing GetFiltered thoroughly can be complex due to many combinations.
// These tests cover basic pagination and one example of each filter type.
// Add more combinations if specific filter interactions are critical.
func TestMoodModel_GetFiltered(t *testing.T) {
	if testing.Short() {
		t.Skip("postgres: skipping integration test in short mode")
	}
	db := newTestDB(t)
	defer db.Close()
	defer cleanupTestDB(t, db)
	model := MoodModel{DB: db}

	// --- Setup: Insert base data ---
	// Insert in reverse chronological order so default fetch (newest first) is easy to predict
	baseTime := time.Date(2024, 5, 10, 12, 0, 0, 0, time.UTC)
	_, err := db.Exec(`
        INSERT INTO moods (title, content, emotion, emoji, color, created_at) VALUES
        ('Day 5 Calm', 'Relaxing day', 'Calm', '😌', '#90EE90', $1),
        ('Day 4 Happy', 'Good news!', 'Happy', '😊', '#FFD700', $2),
        ('Day 3 Sad', 'Feeling down', 'Sad', '😢', '#6495ED', $3),
        ('Day 2 Calm FilterTarget', 'Another calm one', 'Calm', '😌', '#90EE90', $4),
        ('Day 1 Happy', 'Start of week', 'Happy', '😊', '#FFD700', $5)
    `, baseTime.AddDate(0, 0, -0), // Day 5
		baseTime.AddDate(0, 0, -1), // Day 4
		baseTime.AddDate(0, 0, -2), // Day 3
		baseTime.AddDate(0, 0, -3), // Day 2
		baseTime.AddDate(0, 0, -4)) // Day 1
	if err != nil {
		t.Fatalf("Setup failed: Could not insert moods for GetFiltered test: %v", err)
	}
	totalBaseRecords := 5

	// --- Test Cases ---

	t.Run("NoFilters_Page1", func(t *testing.T) {
		filters := FilterCriteria{Page: 1, PageSize: 3}
		moods, metadata, err := model.GetFiltered(filters)

		if err != nil {
			t.Fatalf("GetFiltered failed: %v", err)
		}
		if len(moods) != 3 {
			t.Errorf("Expected 3 moods, got %d", len(moods))
		}
		// Check if newest are first (based on title)
		if moods[0].Title != "Day 5 Calm" || moods[1].Title != "Day 4 Happy" || moods[2].Title != "Day 3 Sad" {
			t.Errorf("Expected newest moods first, got titles: %s, %s, %s", moods[0].Title, moods[1].Title, moods[2].Title)
		}
		// Check metadata
		expectedMeta := Metadata{CurrentPage: 1, PageSize: 3, FirstPage: 1, LastPage: 2, TotalRecords: totalBaseRecords}
		if !reflect.DeepEqual(metadata, expectedMeta) {
			t.Errorf("Metadata mismatch.\nExpected: %+v\nGot:      %+v", expectedMeta, metadata)
		}
	})

	t.Run("NoFilters_Page2", func(t *testing.T) {
		filters := FilterCriteria{Page: 2, PageSize: 3}
		moods, metadata, err := model.GetFiltered(filters)

		if err != nil {
			t.Fatalf("GetFiltered failed: %v", err)
		}
		if len(moods) != 2 { // Remaining moods
			t.Errorf("Expected 2 moods, got %d", len(moods))
		}
		if moods[0].Title != "Day 2 Calm FilterTarget" || moods[1].Title != "Day 1 Happy" {
			t.Errorf("Expected remaining moods, got titles: %s, %s", moods[0].Title, moods[1].Title)
		}
		expectedMeta := Metadata{CurrentPage: 2, PageSize: 3, FirstPage: 1, LastPage: 2, TotalRecords: totalBaseRecords}
		if !reflect.DeepEqual(metadata, expectedMeta) {
			t.Errorf("Metadata mismatch.\nExpected: %+v\nGot:      %+v", expectedMeta, metadata)
		}
	})

	t.Run("FilterText", func(t *testing.T) {
		filters := FilterCriteria{TextQuery: "FilterTarget", Page: 1, PageSize: 10} // PageSize > results
		moods, metadata, err := model.GetFiltered(filters)

		if err != nil {
			t.Fatalf("GetFiltered failed: %v", err)
		}
		if len(moods) != 1 {
			t.Errorf("Expected 1 mood matching 'FilterTarget', got %d", len(moods))
		}
		if moods[0].Title != "Day 2 Calm FilterTarget" {
			t.Errorf("Expected mood 'Day 2 Calm FilterTarget', got %s", moods[0].Title)
		}
		expectedMeta := Metadata{CurrentPage: 1, PageSize: 10, FirstPage: 1, LastPage: 1, TotalRecords: 1}
		if !reflect.DeepEqual(metadata, expectedMeta) {
			t.Errorf("Metadata mismatch.\nExpected: %+v\nGot:      %+v", expectedMeta, metadata)
		}
	})

	t.Run("FilterEmotion", func(t *testing.T) {
		filters := FilterCriteria{Emotion: "Happy::😊", Page: 1, PageSize: 10}
		moods, metadata, err := model.GetFiltered(filters)

		if err != nil {
			t.Fatalf("GetFiltered failed: %v", err)
		}
		if len(moods) != 2 {
			t.Errorf("Expected 2 moods matching 'Happy', got %d", len(moods))
		}
		if moods[0].Title != "Day 4 Happy" || moods[1].Title != "Day 1 Happy" { // Newest Happy first
			t.Errorf("Expected Happy moods, got titles: %s, %s", moods[0].Title, moods[1].Title)
		}
		expectedMeta := Metadata{CurrentPage: 1, PageSize: 10, FirstPage: 1, LastPage: 1, TotalRecords: 2}
		if !reflect.DeepEqual(metadata, expectedMeta) {
			t.Errorf("Metadata mismatch.\nExpected: %+v\nGot:      %+v", expectedMeta, metadata)
		}
	})

	t.Run("FilterStartDate", func(t *testing.T) {
		// Find moods on or after Day 3 (inclusive)
		startDate, _ := time.Parse("2006-01-02", "2024-05-08") // Corresponds to baseTime.AddDate(0,0,-2)
		filters := FilterCriteria{StartDate: startDate, Page: 1, PageSize: 10}
		moods, metadata, err := model.GetFiltered(filters)

		if err != nil {
			t.Fatalf("GetFiltered failed: %v", err)
		}
		if len(moods) != 3 { // Day 3, 4, 5
			t.Errorf("Expected 3 moods on or after %s, got %d", startDate.Format("2006-01-02"), len(moods))
		}
		if moods[0].Title != "Day 5 Calm" || moods[1].Title != "Day 4 Happy" || moods[2].Title != "Day 3 Sad" {
			t.Errorf("Expected moods from Day 3 onwards, got titles: %s, %s, %s", moods[0].Title, moods[1].Title, moods[2].Title)
		}
		expectedMeta := Metadata{CurrentPage: 1, PageSize: 10, FirstPage: 1, LastPage: 1, TotalRecords: 3}
		if !reflect.DeepEqual(metadata, expectedMeta) {
			t.Errorf("Metadata mismatch.\nExpected: %+v\nGot:      %+v", expectedMeta, metadata)
		}
	})

	t.Run("FilterEndDate", func(t *testing.T) {
		// Find moods on or before Day 2 (inclusive)
		// The EndDate needs to cover the whole day.
		parsedEndDate, _ := time.Parse("2006-01-02", "2024-05-07")     // Corresponds to baseTime.AddDate(0,0,-3)
		endDate := parsedEndDate.Add(24*time.Hour - 1*time.Nanosecond) // End of the day
		filters := FilterCriteria{EndDate: endDate, Page: 1, PageSize: 10}
		moods, metadata, err := model.GetFiltered(filters)

		if err != nil {
			t.Fatalf("GetFiltered failed: %v", err)
		}
		if len(moods) != 2 { // Day 1, 2
			t.Errorf("Expected 2 moods on or before %s, got %d", parsedEndDate.Format("2006-01-02"), len(moods))
		}
		if moods[0].Title != "Day 2 Calm FilterTarget" || moods[1].Title != "Day 1 Happy" {
			t.Errorf("Expected moods up to Day 2, got titles: %s, %s", moods[0].Title, moods[1].Title)
		}
		expectedMeta := Metadata{CurrentPage: 1, PageSize: 10, FirstPage: 1, LastPage: 1, TotalRecords: 2}
		if !reflect.DeepEqual(metadata, expectedMeta) {
			t.Errorf("Metadata mismatch.\nExpected: %+v\nGot:      %+v", expectedMeta, metadata)
		}
	})

	t.Run("FilterNoResults", func(t *testing.T) {
		filters := FilterCriteria{TextQuery: "NonExistentText", Page: 1, PageSize: 10}
		moods, metadata, err := model.GetFiltered(filters)

		if err != nil {
			t.Fatalf("GetFiltered failed: %v", err)
		}
		if len(moods) != 0 {
			t.Errorf("Expected 0 moods, got %d", len(moods))
		}
		// Check TotalRecords for the no-result case
		if metadata.TotalRecords != 0 {
			t.Errorf("Metadata mismatch for no results. Expected TotalRecords 0, got %d", metadata.TotalRecords)
		}
	})

}

// --- Add Tests for Validator ---

func TestValidator_NotBlank(t *testing.T) {
	// Use validator package name to call functions
	if validator.NotBlank("") {
		t.Error("Expected false for empty string")
	}
	if validator.NotBlank("   ") {
		t.Error("Expected false for whitespace string")
	}
	if !validator.NotBlank("a") {
		t.Error("Expected true for non-blank string")
	}
	if !validator.NotBlank(" a ") {
		t.Error("Expected true for trimmed non-blank string")
	}
}

func TestValidator_MatchesHexColor(t *testing.T) {
	// Use validator package name to call function and access variable
	valid := []string{"#fff", "#FFF", "#ff00ff", "#FF00FF", "#000000aa", "#12345678"}
	invalid := []string{"#ff", "fff", "#gggggg", "#12345", "#1234567", "#123456789"}

	for _, s := range valid {
		if !validator.Matches(s, validator.HexColorRX) { // Use validator.Matches and validator.HexColorRX
			t.Errorf("Expected true for valid hex color %q", s)
		}
	}
	for _, s := range invalid {
		if validator.Matches(s, validator.HexColorRX) { // Use validator.Matches and validator.HexColorRX
			t.Errorf("Expected false for invalid hex color %q", s)
		}
	}
}

// Consider adding tests for ValidateMood itself if the logic becomes complex,
// though testing the helpers it uses (like NotBlank, Matches) covers much of it.
// Testing ValidateMood would involve creating Mood structs with invalid data
// and asserting that the correct errors are added to the validator.
