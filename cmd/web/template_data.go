// mood/cmd/web/template_data.go
package main

import (
	"html/template"
	"net/http" // Ensure this is imported
	"time"

	"github.com/justinas/nosurf" // <-- Import nosurf
	"github.com/mickali02/mood/internal/data"
	// Import your sessions package if not already done via main.go's application struct
	// "github.com/golangcollege/sessions"
)

// displayMood struct definition (unchanged)
type displayMood struct {
	ID         int64
	CreatedAt  time.Time
	UpdatedAt  time.Time
	Title      string
	Content    template.HTML
	RawContent string
	Emotion    string
	Emoji      string
	Color      string
}

// EmotionDetails struct definition (unchanged)
type EmotionDetails struct {
	Name  string
	Emoji string
	Color string
}

// EmotionMap definition (unchanged)
var EmotionMap = map[string]EmotionDetails{
	"Happy":   {Name: "Happy", Emoji: "😊", Color: "#FFD700"},
	"Sad":     {Name: "Sad", Emoji: "😢", Color: "#6495ED"},
	"Angry":   {Name: "Angry", Emoji: "😠", Color: "#DC143C"},
	"Anxious": {Name: "Anxious", Emoji: "😟", Color: "#FF8C00"},
	"Calm":    {Name: "Calm", Emoji: "😌", Color: "#90EE90"},
	"Excited": {Name: "Excited", Emoji: "🤩", Color: "#FF69B4"},
	"Neutral": {Name: "Neutral", Emoji: "😐", Color: "#B0C4DE"},
}

// TemplateData holds data passed to HTML templates
type TemplateData struct {
	Title           string
	HeaderText      string
	HasMoodEntries  bool
	SearchQuery     string
	FilterEmotion   string
	FilterStartDate string
	FilterEndDate   string

	FormErrors map[string]string
	FormData   map[string]string

	// Page-specific data
	DisplayMoods      []displayMood
	Mood              *data.Mood
	DefaultEmotions   []EmotionDetails
	AvailableEmotions []data.EmotionDetail
	Metadata          data.Metadata

	Flash string // Flash field for session messages

	// --- Fields for Stats Page ---
	Stats             *data.MoodStats
	EmotionCountsJSON string
	MonthlyCountsJSON string
	Quote             string

	// --- Field for Authentication State ---
	IsAuthenticated bool `json:"is_authenticated"`

	// --- ADD CSRF Token field ---
	CSRFToken string `json:"csrf_token"` // <-- ADDED THIS FIELD
	// --- END ADD ---

	// --- Optional: Add field for current user ---
	// User *data.User
	// --- End add field ---
}

// NewTemplateData creates a *basic* default TemplateData instance.
// Authentication status, Flash message, and CSRF token are added later by app.newTemplateData.
func NewTemplateData() *TemplateData {
	// (Existing logic for DefaultEmotions unchanged)
	defaultEmotionsList := make([]EmotionDetails, 0, len(data.ValidEmotions))
	for _, key := range data.ValidEmotions {
		if details, ok := EmotionMap[key]; ok {
			defaultEmotionsList = append(defaultEmotionsList, details)
		} else {
			defaultEmotionsList = append(defaultEmotionsList, EmotionDetails{Name: key, Emoji: "❓", Color: "#cccccc"})
		}
	}

	// Initialize the struct with default/zero values for all fields
	return &TemplateData{
		Title:             "Mood Tracker",
		HeaderText:        "How are you feeling?",
		FormErrors:        make(map[string]string),
		FormData:          make(map[string]string),
		DefaultEmotions:   defaultEmotionsList,
		DisplayMoods:      make([]displayMood, 0),
		AvailableEmotions: make([]data.EmotionDetail, 0),
		Metadata:          data.Metadata{},
		Flash:             "",    // Populated later
		IsAuthenticated:   false, // Populated later
		CSRFToken:         "",    // Populated later

		// --- Initialize Stats Fields ---
		Stats:             nil,
		EmotionCountsJSON: "[]",
		MonthlyCountsJSON: "[]",
		Quote:             "",
	}
}

// GetEmotionDetails function (unchanged)
func GetEmotionDetails(emotionName string) EmotionDetails {
	if details, ok := EmotionMap[emotionName]; ok {
		return details
	}
	return EmotionDetails{Name: emotionName, Emoji: "❓", Color: "#cccccc"}
}

// newTemplateData HELPER METHOD ON application
// Populates base data, authentication status, flash messages, and CSRF token.
func (app *application) newTemplateData(r *http.Request) *TemplateData {
	// Create the basic template data struct.
	td := NewTemplateData()

	// Add the authentication status.
	td.IsAuthenticated = app.isAuthenticated(r)

	// Add the flash message.
	td.Flash = app.session.PopString(r, "flash")

	// Add the CSRF token.
	td.CSRFToken = nosurf.Token(r) // <-- Use nosurf to get the token for the current request

	// Add current user information (Optional, implement later if needed)
	// if td.IsAuthenticated {
	//    userID := app.getUserIDFromSession(r) // Assuming GetInt64 helper exists or use Get().(int64)
	//    user, err := app.users.Get(userID)
	//    if err == nil {
	//        td.User = user
	//    } else if !errors.Is(err, data.ErrRecordNotFound) { // Log unexpected errors
	// 		 app.logger.Error("Failed to get user for template data", "userID", userID, "error", err)
	//    }
	// }

	// Return the populated template data.
	return td
}

// isAuthenticated HELPER METHOD ON application
// Checks the session for the authenticatedUserID key.
func (app *application) isAuthenticated(r *http.Request) bool {
	return app.session.Exists(r, "authenticatedUserID")
}
