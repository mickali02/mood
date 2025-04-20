// mood/cmd/web/template_data.go
package main

import (
	"github.com/mickali02/mood/internal/data" // Ensure data package is imported
)

// EmotionDetails holds display info for an emotion in the WEB layer
// (often identical to data.EmotionDetail, but kept separate for clean architecture)
type EmotionDetails struct {
	Name  string
	Emoji string
	Color string // Represents the hex color code (e.g., #FFD700)
}

// Define the global mapping for DEFAULT emotions - used by NewTemplateData
// Use hex colors here to match DB defaults and avoid confusion
var EmotionMap = map[string]EmotionDetails{
	"Happy":   {Name: "Happy", Emoji: "😊", Color: "#FFD700"},
	"Sad":     {Name: "Sad", Emoji: "😢", Color: "#6495ED"},
	"Angry":   {Name: "Angry", Emoji: "😠", Color: "#DC143C"},
	"Anxious": {Name: "Anxious", Emoji: "😟", Color: "#FF8C00"},
	"Calm":    {Name: "Calm", Emoji: "😌", Color: "#90EE90"}, // Adjusted hex
	"Excited": {Name: "Excited", Emoji: "🤩", Color: "#FF69B4"},
	"Neutral": {Name: "Neutral", Emoji: "😐", Color: "#B0C4DE"}, // Adjusted hex
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
	Moods             []*data.Mood         // Holds filtered or all moods for display
	Mood              *data.Mood           // For pre-filling the edit form
	DefaultEmotions   []EmotionDetails     // Static list (defaults) for mood form input
	AvailableEmotions []data.EmotionDetail // Dynamic list from DB data layer
}

// NewTemplateData creates a default TemplateData instance
func NewTemplateData() *TemplateData {
	// Populate the STATIC DefaultEmotions slice (for the mood form)
	defaultEmotionsList := make([]EmotionDetails, 0, len(data.ValidEmotions)) // Renamed variable for clarity
	for _, key := range data.ValidEmotions {
		if details, ok := EmotionMap[key]; ok {
			defaultEmotionsList = append(defaultEmotionsList, details)
		} else {
			// Fallback for any defaults listed in ValidEmotions but missing from EmotionMap
			defaultEmotionsList = append(defaultEmotionsList, EmotionDetails{Name: key, Emoji: "❓", Color: "#cccccc"}) // Use hex default
		}
	}

	// Initialize TemplateData with defaults
	return &TemplateData{
		Title:           "Mood Tracker",
		HeaderText:      "How are you feeling?",
		FormErrors:      make(map[string]string),
		FormData:        make(map[string]string),
		DefaultEmotions: defaultEmotionsList, // <-- Assigns the default list correctly
		// AvailableEmotions is initialized as nil here, populated by handler
		// Other fields (SearchQuery, Filter*, HasMoodEntries) default to zero values ("", "", "", false)
	}
}

// Helper function (can be used directly or via template funcs)
// Gets details for DEFAULT emotions. Might need adjustment if displaying custom ones often.
func GetEmotionDetails(emotionName string) EmotionDetails {
	if details, ok := EmotionMap[emotionName]; ok {
		return details
	}
	// Fallback for unknown/custom emotion names when using this helper
	// Returning generic values might be better than erroring.
	// NOTE: The dashboard list now uses mood.Emoji and mood.Color directly from the DB data.
	return EmotionDetails{Name: emotionName, Emoji: "❓", Color: "#cccccc"} // Use hex default
}
