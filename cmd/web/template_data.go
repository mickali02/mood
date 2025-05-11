// mood/cmd/web/template_data.go
package main

import (
	"errors"
	"html/template"
	"net/http" // Ensure this is imported
	"time"

	"github.com/justinas/nosurf" // <-- Import nosurf
	"github.com/mickali02/mood/internal/data"
)

// displayMood struct definition (unchanged)
type displayMood struct {
	ID           int64
	CreatedAt    time.Time
	UpdatedAt    time.Time
	Title        string
	Content      template.HTML
	ShortContent string
	RawContent   string
	Emotion      string
	Emoji        string
	Color        string
}

// EmotionDetails struct definition (unchanged)
type EmotionDetails struct {
	Name  string
	Emoji string
	Color string
}

// EmotionMap definition (unchanged)
var EmotionMap = map[string]EmotionDetails{
	"Happy":   {Name: "Happy", Emoji: "😊", Color: "#FFCA28"},
	"Sad":     {Name: "Sad", Emoji: "😢", Color: "#5C8DDE"},
	"Angry":   {Name: "Angry", Emoji: "😠", Color: "#E53935"},
	"Anxious": {Name: "Anxious", Emoji: "😟", Color: "#FFA000"},
	"Calm":    {Name: "Calm", Emoji: "😌", Color: "#69B36C"},
	"Excited": {Name: "Excited", Emoji: "🤩", Color: "#F06292"},
	"Neutral": {Name: "Neutral", Emoji: "😐", Color: "#A4B8D0"},
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
	UserName        string

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
	Quote             string

	// --- Field for Authentication State ---
	IsAuthenticated bool `json:"is_authenticated"`

	CSRFToken string     `json:"csrf_token"`
	User      *data.User `json:"user"`

	// --- Fields for Profile Page Pagination ---
	ProfileCurrentPage int
	ProfileTotalPages  int
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
		UserName:          "",
		User:              nil, // Initialize User as nil

		// --- Initialize Stats Fields ---
		Stats:             nil,
		EmotionCountsJSON: "[]",
		Quote:             "",

		// --- Initialize Profile Pagination Fields ---
		ProfileCurrentPage: 1, // Default to page 1
		ProfileTotalPages:  2, // We have 2 logical pages for profile settings
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
	td := NewTemplateData()
	td.IsAuthenticated = app.isAuthenticated(r)
	td.Flash = app.session.PopString(r, "flash")
	td.CSRFToken = nosurf.Token(r)

	if td.IsAuthenticated {
		userID := app.getUserIDFromSession(r)
		if userID > 0 { // Ensure userID is valid before fetching
			user, err := app.users.Get(userID)
			if err == nil {
				td.User = user
				td.UserName = user.Name // Keep UserName populated for convenience if templates use it
			} else if !errors.Is(err, data.ErrRecordNotFound) {
				app.logger.Error("Failed to get user for template data", "userID", userID, "error", err)
			}
		}
	}
	return td
}

// isAuthenticated HELPER METHOD ON application
// Checks the session for the authenticatedUserID key.
func (app *application) isAuthenticated(r *http.Request) bool {
	return app.session.Exists(r, "authenticatedUserID")
}
