// mood/cmd/web/template_data.go
package main

import (
	"github.com/mickali02/mood/internal/data"
)

// EmotionDetails holds display info for an emotion
type EmotionDetails struct {
	// No changes needed here
	Name  string
	Emoji string
	Color string
}

// Define the global mapping for emotions - Centralized map
// No changes needed here
var EmotionMap = map[string]EmotionDetails{
	"Happy":   {Name: "Happy", Emoji: "😊", Color: "emotion-happy"},
	"Sad":     {Name: "Sad", Emoji: "😢", Color: "emotion-sad"},
	"Angry":   {Name: "Angry", Emoji: "😠", Color: "emotion-angry"},
	"Anxious": {Name: "Anxious", Emoji: "😟", Color: "emotion-anxious"},
	"Calm":    {Name: "Calm", Emoji: "😌", Color: "emotion-calm"},
	"Excited": {Name: "Excited", Emoji: "🤩", Color: "emotion-excited"},
	"Neutral": {Name: "Neutral", Emoji: "😐", Color: "emotion-neutral"},
}

// TemplateData holds data passed to HTML templates
type TemplateData struct {
	Title          string
	HeaderText     string
	HasMoodEntries bool   // Flag based on final mood list count
	SearchQuery    string // Text search query

	// --- Filter fields ---
	FilterEmotion   string // Currently selected emotion filter
	FilterStartDate string // Currently selected start date (YYYY-MM-DD string for template)
	FilterEndDate   string // Currently selected end date (YYYY-MM-DD string for template)

	// Form handling (primarily for add/edit forms)
	FormErrors map[string]string
	FormData   map[string]string

	// Page-specific data
	Moods    []*data.Mood     // Holds filtered or all moods for the dashboard/list
	Mood     *data.Mood       // For pre-filling the edit form
	Emotions []EmotionDetails // For populating emotion filter/form options

	// LatestMood     *data.Mood // REMOVED - Dashboard now shows list
}

// NewTemplateData creates a default TemplateData instance
func NewTemplateData() *TemplateData {
	// Populate the Emotions slice from the map for easy iteration in templates
	emotionsList := make([]EmotionDetails, 0, len(data.ValidEmotions))
	for _, key := range data.ValidEmotions {
		if details, ok := EmotionMap[key]; ok {
			emotionsList = append(emotionsList, details)
		} else {
			// Fallback if an emotion in ValidEmotions is missing from EmotionMap
			emotionsList = append(emotionsList, EmotionDetails{Name: key, Emoji: "?", Color: "emotion-unknown"})
		}
	}

	// Initialize TemplateData with defaults
	return &TemplateData{
		Title:      "Mood Tracker",
		HeaderText: "How are you feeling?",
		FormErrors: make(map[string]string),
		FormData:   make(map[string]string),
		Emotions:   emotionsList,
		// SearchQuery defaults to ""
		// FilterEmotion defaults to ""
		// FilterStartDate defaults to ""
		// FilterEndDate defaults to ""
		// HasMoodEntries defaults to false
	}
}

// Helper function (can be used directly or via template funcs)
// No changes needed here
func GetEmotionDetails(emotion string) EmotionDetails {
	if details, ok := EmotionMap[emotion]; ok {
		return details
	}
	// Fallback for unknown/unexpected emotion values
	return EmotionDetails{Name: emotion, Emoji: "❓", Color: "emotion-unknown"}
}
