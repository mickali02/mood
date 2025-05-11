// internal/data/mood_test.go
package data

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	_ "github.com/lib/pq"

	"github.com/mickali02/mood/internal/validator"
)

// --- Test Helper Functions ---

// newTestDB connects to the test database defined by MOODNOTES_TEST_DB_DSN.
func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("MOODNOTES_TEST_DB_DSN")
	if dsn == "" {
		t.Fatal("MOODNOTES_TEST_DB_DSN environment variable not set")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("Failed to open test database connection: %s", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = db.PingContext(ctx)
	if err != nil {
		db.Close()
		t.Fatalf("Failed to ping test database (%s): %s", dsn, err)
	}
	// Check migrations table exists
	var exists bool
	err = db.QueryRowContext(ctx,
		"SELECT EXISTS (SELECT FROM information_schema.tables WHERE table_schema = 'public' AND table_name = 'schema_migrations')").Scan(&exists)
	if err != nil || !exists {
		db.Close()
		t.Fatalf("Test DB schema check failed (schema_migrations table not found or error: %v). Did you run 'make testdb/migrations/up'?", err)
	}
	return db
}

// cleanupTestDB removes all data from relevant tables and resets sequences.
func cleanupTestDB(t *testing.T, db *sql.DB) {
	t.Helper()
	// Truncate moods first due to foreign key from moods to users
	_, err := db.Exec("TRUNCATE TABLE moods RESTART IDENTITY CASCADE")
	if err != nil {
		t.Fatalf("Failed to cleanup test database (truncate moods): %s", err)
	}
	// Truncate users *after* moods
	_, err = db.Exec("TRUNCATE TABLE users RESTART IDENTITY CASCADE")
	if err != nil {
		t.Fatalf("Failed to cleanup test database (truncate users): %s", err)
	}
}

// Helper to insert a test user and return their ID
func insertTestUser(t *testing.T, db *sql.DB) int64 {
	t.Helper()
	// Use unique emails for each test user insertion if needed across tests
	// This helps avoid duplicate email errors if cleanup fails or tests run in parallel unexpectedly
	email := fmt.Sprintf("testuser_%d@example.com", time.Now().UnixNano())
	user := &User{Name: "Test User", Email: email, Activated: true}
	err := user.Password.Set("password")
	if err != nil {
		t.Fatalf("Failed to set test user password: %v", err)
	}
	userModel := UserModel{DB: db}
	err = userModel.Insert(user)
	// Handle potential duplicate email error gracefully during setup
	if err != nil && errors.Is(err, ErrDuplicateEmail) {
		// If duplicate, try fetching the existing user
		existingUser, getErr := userModel.GetByEmail(email)
		if getErr != nil {
			t.Fatalf("Failed to insert test user (%v) and failed to fetch existing user (%v)", err, getErr)
		}
		t.Logf("Test user %s already existed, using ID %d", email, existingUser.ID)
		return existingUser.ID
	} else if err != nil {
		t.Fatalf("Failed to insert test user: %v", err)
	}

	if user.ID == 0 {
		t.Fatal("Test user ID is 0 after insert")
	}
	return user.ID
}

// --- Test Functions for Statistics (Updated for Weekly) ---
func TestMoodModel_GetTotalMoodCount(t *testing.T) {
	if testing.Short() {
		t.Skip("postgres: skipping integration test in short mode")
	}
	db := newTestDB(t)
	defer db.Close()
	defer cleanupTestDB(t, db)
	testUserID := insertTestUser(t, db)
	model := MoodModel{DB: db}

	t.Run("NoEntries", func(t *testing.T) {
		count, err := model.GetTotalMoodCount(testUserID)
		if err != nil {
			t.Fatalf("Expected no error, got %s", err)
		}
		if count != 0 {
			t.Errorf("Expected count 0, got %d", count)
		}
	})
	t.Run("WithEntries", func(t *testing.T) {
		_, err := db.Exec(`INSERT INTO moods (title, content, emotion, emoji, color, user_id) VALUES ('T1','','H','h','#fff', $1), ('T2','','S','s','#000', $1)`, testUserID)
		if err != nil {
			t.Fatalf("Failed to insert test data: %s", err)
		}
		count, err := model.GetTotalMoodCount(testUserID)
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
	testUserID := insertTestUser(t, db)
	model := MoodModel{DB: db}

	t.Run("NoEntries", func(t *testing.T) {
		counts, err := model.GetEmotionCounts(testUserID)
		if err != nil {
			t.Fatalf("Expected no error, got %s", err)
		}
		if len(counts) != 0 {
			t.Errorf("Expected 0 emotion counts, got %d", len(counts))
		}
	})
	t.Run("WithEntries", func(t *testing.T) {
		_, err := db.Exec(`INSERT INTO moods (title, content, emotion, emoji, color, user_id) VALUES ('T1','','Happy','😊','#FFD700', $1),('T2','','Sad','😢','#6495ED', $1),('T3','','Happy','😊','#FFD700', $1),('T4','','Calm','😌','#90EE90', $1),('T5','','Happy','😊','#FFD700', $1)`, testUserID)
		if err != nil {
			t.Fatalf("Failed to insert test data: %s", err)
		}
		counts, err := model.GetEmotionCounts(testUserID)
		if err != nil {
			t.Fatalf("Expected no error, got %s", err)
		}
		// Expected order is by count DESC, then name ASC
		expected := []EmotionCount{{Name: "Happy", Emoji: "😊", Color: "#FFD700", Count: 3}, {Name: "Calm", Emoji: "😌", Color: "#90EE90", Count: 1}, {Name: "Sad", Emoji: "😢", Color: "#6495ED", Count: 1}}
		if !reflect.DeepEqual(counts, expected) {
			t.Errorf("Mismatch in emotion counts.\nExpected: %+v\nGot:      %+v", expected, counts)
		}
	})
}

// ** UPDATED Test for GetAllStats **
func TestMoodModel_GetAllStats(t *testing.T) {
	if testing.Short() {
		t.Skip("postgres: skipping integration test in short mode")
	}
	db := newTestDB(t)
	defer db.Close()
	defer cleanupTestDB(t, db)
	testUserID := insertTestUser(t, db)
	model := MoodModel{DB: db}

	t.Run("NoEntries", func(t *testing.T) {
		stats, err := model.GetAllStats(testUserID)
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
		// Check WeeklyCounts is empty
		if len(stats.WeeklyCounts) != 0 {
			t.Errorf("Expected 0 WeeklyCounts, got %d", len(stats.WeeklyCounts))
		}
		if stats.LatestMood != nil {
			t.Errorf("Expected LatestMood nil, got %+v", stats.LatestMood)
		}
		if stats.AvgEntriesPerWeek != 0.0 {
			t.Errorf("Expected AvgEntriesPerWeek 0.0, got %f", stats.AvgEntriesPerWeek)
		}
	})
	t.Run("WithEntries", func(t *testing.T) {
		// Insert data similar to GetWeeklyEntryCounts test
		_, err := db.Exec(`INSERT INTO moods (title, content, emotion, emoji, color, created_at, user_id) VALUES
            ('Week1-1','','Happy','😊','#FFD700','2024-01-01 10:00:00+00', $1),
            ('Week1-2','','Sad','😢','#6495ED','2024-01-03 11:00:00+00', $1),
            ('Week2-1','','Happy','😊','#FFD700','2024-01-08 12:00:00+00', $1),
            ('Week2-2','','Happy','😊','#FFD700','2024-01-10 13:00:00+00', $1),
            ('Week2-3','','Calm','😌','#90EE90','2024-01-14 14:00:00+00', $1)`,
			testUserID)
		if err != nil {
			t.Fatalf("Failed to insert test data: %s", err)
		}
		stats, err := model.GetAllStats(testUserID)
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
		// Verify WeeklyCounts
		expectedWeeklyCounts := []WeeklyCount{{Week: "2024-01", Count: 2}, {Week: "2024-02", Count: 3}}
		if !reflect.DeepEqual(stats.WeeklyCounts, expectedWeeklyCounts) {
			t.Errorf("Mismatch in WeeklyCounts.\nExpected: %+v\nGot:      %+v", expectedWeeklyCounts, stats.WeeklyCounts)
		}
		// Verify Latest Mood (should be the last one inserted chronologically)
		if stats.LatestMood == nil || stats.LatestMood.Title != "Week2-3" {
			t.Errorf("Expected LatestMood 'Week2-3', got %+v", stats.LatestMood)
		}
		// Basic check for AvgEntriesPerWeek (value depends on when test is run vs '2024-01-01')
		if stats.AvgEntriesPerWeek <= 0 {
			t.Errorf("Expected positive AvgEntriesPerWeek, got %f", stats.AvgEntriesPerWeek)
		}
	})
}

// --- CRUD Tests (Unchanged - already handle UserID) ---

func TestMoodModel_Insert(t *testing.T) {
	if testing.Short() {
		t.Skip("postgres: skipping integration test in short mode")
	}
	db := newTestDB(t)
	defer db.Close()
	defer cleanupTestDB(t, db)
	testUserID := insertTestUser(t, db)
	model := MoodModel{DB: db}

	t.Run("InsertValidMood", func(t *testing.T) {
		mood := &Mood{
			Title: "Test Insert", Content: "<p>This is content</p>",
			Emotion: "Neutral", Emoji: "😐", Color: "#B0C4DE",
			UserID: testUserID,
		}
		err := model.Insert(mood)
		if err != nil {
			t.Fatalf("Insert failed: %v", err)
		}
		if mood.ID == 0 {
			t.Errorf("Expected non-zero ID after insert, got 0")
		}
		if mood.CreatedAt.IsZero() {
			t.Errorf("Expected non-zero CreatedAt after insert")
		}
		if mood.UpdatedAt.IsZero() {
			t.Errorf("Expected non-zero UpdatedAt after insert")
		}

		fetchedMood, errGet := model.Get(mood.ID, testUserID)
		if errGet != nil {
			t.Fatalf("Failed to fetch inserted mood: %v", errGet)
		}
		if fetchedMood == nil {
			t.Fatal("Fetched mood is nil after insert")
		}
		if fetchedMood.Title != mood.Title {
			t.Errorf("Expected Title %q, got %q", mood.Title, fetchedMood.Title)
		}
		if fetchedMood.UserID != testUserID {
			t.Errorf("Expected UserID %d, got %d", testUserID, fetchedMood.UserID)
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
	testUserID := insertTestUser(t, db)
	otherUserID := insertTestUser(t, db)
	model := MoodModel{DB: db}

	moodUser1 := &Mood{Title: "User1 Mood", Content: "...", Emotion: "Happy", Emoji: "😊", Color: "#FFD700", UserID: testUserID}
	err := model.Insert(moodUser1)
	if err != nil {
		t.Fatalf("Setup insert failed: %v", err)
	}
	moodUser2 := &Mood{Title: "User2 Mood", Content: "...", Emotion: "Sad", Emoji: "😢", Color: "#6495ED", UserID: otherUserID}
	err = model.Insert(moodUser2)
	if err != nil {
		t.Fatalf("Setup insert failed: %v", err)
	}

	t.Run("GetExistingOwned", func(t *testing.T) {
		fetchedMood, err := model.Get(moodUser1.ID, testUserID)
		if err != nil {
			t.Fatalf("Get failed for existing owned ID %d: %v", moodUser1.ID, err)
		}
		if fetchedMood == nil {
			t.Fatal("Get returned nil for existing owned ID")
		}
		if fetchedMood.ID != moodUser1.ID || fetchedMood.UserID != testUserID {
			t.Errorf("Mismatch in fetched mood: Got ID %d, UserID %d", fetchedMood.ID, fetchedMood.UserID)
		}
	})

	t.Run("GetExistingNotOwned", func(t *testing.T) {
		_, err := model.Get(moodUser2.ID, testUserID)
		if !errors.Is(err, ErrRecordNotFound) {
			t.Errorf("Expected ErrRecordNotFound when getting non-owned mood, got %v", err)
		}
	})

	t.Run("GetNonExistentPositiveID", func(t *testing.T) {
		_, err := model.Get(int64(999999), testUserID)
		if !errors.Is(err, ErrRecordNotFound) {
			t.Errorf("Expected ErrRecordNotFound for non-existent ID, got %v", err)
		}
	})

	t.Run("GetZeroID", func(t *testing.T) {
		_, err := model.Get(0, testUserID)
		if !errors.Is(err, ErrRecordNotFound) {
			t.Errorf("Expected ErrRecordNotFound for ID 0, got %v", err)
		}
	})

	t.Run("GetNegativeID", func(t *testing.T) {
		_, err := model.Get(-1, testUserID)
		if !errors.Is(err, ErrRecordNotFound) {
			t.Errorf("Expected ErrRecordNotFound for ID -1, got %v", err)
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
	testUserID := insertTestUser(t, db)
	otherUserID := insertTestUser(t, db)
	model := MoodModel{DB: db}

	originalMood := &Mood{Title: "Original Title", Content: "...", Emotion: "Sad", Emoji: "😢", Color: "#6495ED", UserID: testUserID}
	err := model.Insert(originalMood)
	if err != nil {
		t.Fatalf("Setup insert failed: %v", err)
	}
	otherUserMood := &Mood{Title: "Other User Mood", Content: "...", Emotion: "Angry", Emoji: "😠", Color: "#DC143C", UserID: otherUserID}
	err = model.Insert(otherUserMood)
	if err != nil {
		t.Fatalf("Setup insert failed: %v", err)
	}
	originalUpdatedAt := originalMood.UpdatedAt
	time.Sleep(10 * time.Millisecond) // Ensure time difference for UpdatedAt check

	t.Run("UpdateOwned", func(t *testing.T) {
		moodToUpdate := &Mood{
			ID: originalMood.ID, Title: "Updated Title", Content: "Updated Content",
			Emotion: "Excited", Emoji: "🤩", Color: "#FF69B4",
			UserID: testUserID,
		}
		err := model.Update(moodToUpdate)
		if err != nil {
			t.Fatalf("Update failed for owned ID %d: %v", moodToUpdate.ID, err)
		}

		updatedMood, errGet := model.Get(originalMood.ID, testUserID)
		if errGet != nil {
			t.Fatalf("Failed to fetch mood after update: %v", errGet)
		}
		if updatedMood.Title != moodToUpdate.Title {
			t.Errorf("Title mismatch after update")
		}
		if !updatedMood.UpdatedAt.After(originalUpdatedAt) {
			t.Errorf("Expected UpdatedAt to be newer")
		}
	})

	t.Run("UpdateNotOwned", func(t *testing.T) {
		moodToUpdate := &Mood{
			ID:    otherUserMood.ID,
			Title: "Attempted Update Title", Content: "...", Emotion: "Neutral", Emoji: "😐", Color: "#ccc",
			UserID: testUserID, // Try update as wrong user
		}
		err := model.Update(moodToUpdate)
		if !errors.Is(err, ErrRecordNotFound) {
			t.Errorf("Expected ErrRecordNotFound when updating non-owned mood, got %v", err)
		}
		fetchedOther, _ := model.Get(otherUserMood.ID, otherUserID)
		if fetchedOther.Title == moodToUpdate.Title {
			t.Error("Non-owned mood was incorrectly updated")
		}
	})

	t.Run("UpdateNonExistent", func(t *testing.T) {
		moodToUpdate := &Mood{
			ID: 999999, Title: "...", Content: "...", Emotion: "Neutral", Emoji: "😐", Color: "#ccc",
			UserID: testUserID,
		}
		err := model.Update(moodToUpdate)
		if !errors.Is(err, ErrRecordNotFound) {
			t.Errorf("Expected ErrRecordNotFound when updating non-existent ID, got %v", err)
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
	testUserID := insertTestUser(t, db)
	otherUserID := insertTestUser(t, db)
	model := MoodModel{DB: db}

	moodToDelete := &Mood{Title: "To Be Deleted", Content: "...", Emotion: "Angry", Emoji: "😠", Color: "#DC143C", UserID: testUserID}
	err := model.Insert(moodToDelete)
	if err != nil {
		t.Fatalf("Setup insert failed: %v", err)
	}
	moodToKeep := &Mood{Title: "Keep Me", Content: "...", Emotion: "Happy", Emoji: "😊", Color: "#FFD700", UserID: testUserID}
	err = model.Insert(moodToKeep)
	if err != nil {
		t.Fatalf("Setup insert failed: %v", err)
	}
	otherUserMood := &Mood{Title: "Other Keep", Content: "...", Emotion: "Calm", Emoji: "😌", Color: "#90EE90", UserID: otherUserID}
	err = model.Insert(otherUserMood)
	if err != nil {
		t.Fatalf("Setup insert failed: %v", err)
	}

	t.Run("DeleteOwned", func(t *testing.T) {
		err := model.Delete(moodToDelete.ID, testUserID)
		if err != nil {
			t.Fatalf("Delete failed for owned ID %d: %v", moodToDelete.ID, err)
		}
		_, errGet := model.Get(moodToDelete.ID, testUserID)
		if !errors.Is(errGet, ErrRecordNotFound) {
			t.Errorf("Expected ErrRecordNotFound after deleting owned ID, got %v", errGet)
		}
		keptMood, errGetKeep := model.Get(moodToKeep.ID, testUserID)
		if errGetKeep != nil || keptMood == nil {
			t.Errorf("Owned mood that should have been kept was affected")
		}
		otherKeptMood, errGetOther := model.Get(otherUserMood.ID, otherUserID)
		if errGetOther != nil || otherKeptMood == nil {
			t.Errorf("Other user's mood was affected")
		}
	})

	t.Run("DeleteNotOwned", func(t *testing.T) {
		err := model.Delete(otherUserMood.ID, testUserID)
		if !errors.Is(err, ErrRecordNotFound) {
			t.Errorf("Expected ErrRecordNotFound when deleting non-owned mood, got %v", err)
		}
		otherKeptMood, errGetOther := model.Get(otherUserMood.ID, otherUserID)
		if errGetOther != nil || otherKeptMood == nil {
			t.Errorf("Non-owned mood was incorrectly deleted")
		}
	})

	t.Run("DeleteNonExistent", func(t *testing.T) {
		err := model.Delete(int64(999999), testUserID)
		if !errors.Is(err, ErrRecordNotFound) {
			t.Errorf("Expected ErrRecordNotFound when deleting non-existent ID, got %v", err)
		}
	})

	t.Run("DeleteZeroID", func(t *testing.T) {
		err := model.Delete(0, testUserID)
		if !errors.Is(err, ErrRecordNotFound) {
			t.Errorf("Expected ErrRecordNotFound when deleting ID 0, got %v", err)
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
	testUserID1 := insertTestUser(t, db)
	testUserID2 := insertTestUser(t, db)
	model := MoodModel{DB: db}

	t.Run("NoEntriesForUser", func(t *testing.T) {
		details, err := model.GetDistinctEmotionDetails(testUserID1)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if len(details) != 0 {
			t.Errorf("Expected empty slice, got %d items", len(details))
		}
	})

	t.Run("WithEntriesAndDuplicatesForUser", func(t *testing.T) {
		_, err := db.Exec(`INSERT INTO moods (title, content, emotion, emoji, color, user_id) VALUES
            ('U1E1', '', 'Happy', '😊', '#FFD700', $1), ('U1E2', '', 'Sad', '😢', '#6495ED', $1),
            ('U1E3', '', 'Happy', '😊', '#FFD700', $1), ('U1E4', '', 'Calm', '😌', '#90EE90', $1),
            ('U2E1', '', 'Angry', '😠', '#DC143C', $2), ('U2E2', '', 'Happy', '😊', '#FFD700', $2)`,
			testUserID1, testUserID2)
		if err != nil {
			t.Fatalf("Failed to insert test data: %v", err)
		}

		details1, err1 := model.GetDistinctEmotionDetails(testUserID1)
		if err1 != nil {
			t.Fatalf("Expected no error for user 1, got %v", err1)
		}
		// Expected order is alphabetical by emotion name
		expected1 := []EmotionDetail{
			{Name: "Calm", Emoji: "😌", Color: "#90EE90"},
			{Name: "Happy", Emoji: "😊", Color: "#FFD700"},
			{Name: "Sad", Emoji: "😢", Color: "#6495ED"},
		}
		if !reflect.DeepEqual(details1, expected1) {
			t.Errorf("Mismatch for user 1.\nExpected: %+v\nGot:      %+v", expected1, details1)
		}

		details2, err2 := model.GetDistinctEmotionDetails(testUserID2)
		if err2 != nil {
			t.Fatalf("Expected no error for user 2, got %v", err2)
		}
		expected2 := []EmotionDetail{
			{Name: "Angry", Emoji: "😠", Color: "#DC143C"},
			{Name: "Happy", Emoji: "😊", Color: "#FFD700"},
		}
		if !reflect.DeepEqual(details2, expected2) {
			t.Errorf("Mismatch for user 2.\nExpected: %+v\nGot:      %+v", expected2, details2)
		}
	})
}

func TestMoodModel_GetFiltered(t *testing.T) {
	if testing.Short() {
		t.Skip("postgres: skipping integration test in short mode")
	}
	db := newTestDB(t)
	defer db.Close()
	defer cleanupTestDB(t, db)
	testUserID1 := insertTestUser(t, db)
	testUserID2 := insertTestUser(t, db)
	model := MoodModel{DB: db}

	baseTime := time.Date(2024, 5, 10, 12, 0, 0, 0, time.UTC)
	_, err := db.Exec(`INSERT INTO moods (title, content, emotion, emoji, color, created_at, user_id) VALUES
        ('U1 Day 5 Calm', 'Relax', 'Calm', '😌', '#90EE90', $1, $6), ('U1 Day 4 Happy', 'Good', 'Happy', '😊', '#FFD700', $2, $6),
        ('U1 Day 3 Sad', 'Down', 'Sad', '😢', '#6495ED', $3, $6), ('U1 Day 2 Target', 'Tgt', 'Calm', '😌', '#90EE90', $4, $6),
        ('U2 Day 1 Happy', 'Start', 'Happy', '😊', '#FFD700', $5, $7)`,
		baseTime.AddDate(0, 0, -0), baseTime.AddDate(0, 0, -1), baseTime.AddDate(0, 0, -2),
		baseTime.AddDate(0, 0, -3), baseTime.AddDate(0, 0, -4),
		testUserID1, testUserID2)
	if err != nil {
		t.Fatalf("Setup failed: Could not insert moods: %v", err)
	}
	totalUser1Records := 4

	t.Run("NoFilters_User1_Page1", func(t *testing.T) {
		filters := FilterCriteria{Page: 1, PageSize: 3, UserID: testUserID1}
		moods, metadata, err := model.GetFiltered(filters)
		if err != nil {
			t.Fatalf("GetFiltered failed: %v", err)
		}
		if len(moods) != 3 {
			t.Errorf("Expected 3 moods for user 1, got %d", len(moods))
		}
		if moods[0].Title != "U1 Day 5 Calm" || moods[1].Title != "U1 Day 4 Happy" || moods[2].Title != "U1 Day 3 Sad" {
			t.Errorf("Unexpected mood order/content")
		}
		expectedMeta := Metadata{CurrentPage: 1, PageSize: 3, FirstPage: 1, LastPage: 2, TotalRecords: totalUser1Records}
		if !reflect.DeepEqual(metadata, expectedMeta) {
			t.Errorf("Metadata mismatch.\nExpected: %+v\nGot:      %+v", expectedMeta, metadata)
		}
	})

	t.Run("NoFilters_User2", func(t *testing.T) {
		filters := FilterCriteria{Page: 1, PageSize: 10, UserID: testUserID2}
		moods, metadata, err := model.GetFiltered(filters)
		if err != nil {
			t.Fatalf("GetFiltered failed: %v", err)
		}
		if len(moods) != 1 {
			t.Errorf("Expected 1 mood for user 2, got %d", len(moods))
		}
		if moods[0].Title != "U2 Day 1 Happy" {
			t.Errorf("Unexpected mood for user 2")
		}
		expectedMeta := Metadata{CurrentPage: 1, PageSize: 10, FirstPage: 1, LastPage: 1, TotalRecords: 1}
		if !reflect.DeepEqual(metadata, expectedMeta) {
			t.Errorf("Metadata mismatch.\nExpected: %+v\nGot:      %+v", expectedMeta, metadata)
		}
	})

	t.Run("FilterText_User1", func(t *testing.T) {
		filters := FilterCriteria{TextQuery: "Target", Page: 1, PageSize: 10, UserID: testUserID1}
		moods, _, err := model.GetFiltered(filters)
		if err != nil {
			t.Fatalf("GetFiltered failed: %v", err)
		}
		if len(moods) != 1 {
			t.Errorf("Expected 1 mood matching 'Target' for user 1, got %d", len(moods))
		}
		if moods[0].Title != "U1 Day 2 Target" {
			t.Errorf("Expected target mood")
		}
	})

	// Add more filter tests specific to user 1...
}

// --- Validator Tests (Unchanged) ---

func TestValidator_NotBlank(t *testing.T) {
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
	valid := []string{"#fff", "#FFF", "#ff00ff", "#FF00FF", "#000000aa", "#12345678"}
	invalid := []string{"#ff", "fff", "#gggggg", "#12345", "#1234567", "#123456789"}
	for _, s := range valid {
		if !validator.Matches(s, validator.HexColorRX) {
			t.Errorf("Expected true for valid hex color %q", s)
		}
	}
	for _, s := range invalid {
		if validator.Matches(s, validator.HexColorRX) {
			t.Errorf("Expected false for invalid hex color %q", s)
		}
	}
}
