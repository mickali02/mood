// mood/cmd/web/template_data.go
package main

import (
	"html/template"
	"time"

	"github.com/mickali02/mood/internal/data"
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

	// --- NEW Fields for Stats Page ---
	Stats             *data.MoodStats // Holds the aggregated stats (pointer type)
	EmotionCountsJSON string          // JSON string for emotion chart data
	MonthlyCountsJSON string          // JSON string for monthly chart data
	Quote             string          // Encouraging quote for stats page
	// --- END NEW Fields ---
}

// NewTemplateData creates a default TemplateData instance
func NewTemplateData() *TemplateData {
	// (Existing logic for DefaultEmotions unchanged)
	defaultEmotionsList := make([]EmotionDetails, 0, len(data.ValidEmotions)) // Using ValidEmotions from data package
	for _, key := range data.ValidEmotions {
		if details, ok := EmotionMap[key]; ok {
			defaultEmotionsList = append(defaultEmotionsList, details)
		} else {
			// Fallback if an emotion in ValidEmotions isn't in EmotionMap (shouldn't happen ideally)
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
		AvailableEmotions: make([]data.EmotionDetail, 0), // Initialize empty slice
		Metadata:          data.Metadata{},               // Initialize empty struct
		Flash:             "",                            // Initialize empty string

		// --- Initialize NEW Fields ---
		Stats:             nil,  // Initialize stats pointer as nil
		EmotionCountsJSON: "[]", // Default to empty JSON array string
		MonthlyCountsJSON: "[]", // Default to empty JSON array string
		Quote:             "",   // Initialize empty quote
		// --- END Initialize NEW Fields ---
	}
}

// GetEmotionDetails function (unchanged)
func GetEmotionDetails(emotionName string) EmotionDetails {
	if details, ok := EmotionMap[emotionName]; ok {
		return details
	}
	// Return a default/unknown representation
	return EmotionDetails{Name: emotionName, Emoji: "❓", Color: "#cccccc"}
}

// newTemplateData function (helper, unchanged)
// This simply calls NewTemplateData, so it benefits from the updates above.
func (app *application) newTemplateData() *TemplateData {
	return NewTemplateData()
}
