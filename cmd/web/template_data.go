// mood/cmd/web/template_data.go
package main

import (
	// Adjust import path if your module name is different
	"github.com/mickali02/mood/internal/data"
)

// EmotionDetails holds display info for an emotion
type EmotionDetails struct {
	Name  string // e.g., "Happy"
	Emoji string // e.g., "😊"
	Color string // e.g., "emotion-happy" (CSS class)
}

// Define the global mapping for emotions - Centralized map
var EmotionMap = map[string]EmotionDetails{
	"Happy":   {Name: "Happy", Emoji: "😊", Color: "emotion-happy"},
	"Sad":     {Name: "Sad", Emoji: "😢", Color: "emotion-sad"},
	"Angry":   {Name: "Angry", Emoji: "😠", Color: "emotion-angry"},
	"Anxious": {Name: "Anxious", Emoji: "😟", Color: "emotion-anxious"},
	"Calm":    {Name: "Calm", Emoji: "😌", Color: "emotion-calm"},
	"Excited": {Name: "Excited", Emoji: "🤩", Color: "emotion-excited"},
	"Neutral": {Name: "Neutral", Emoji: "😐", Color: "emotion-neutral"},
	// Add more based on data.ValidEmotions
}

// TemplateData holds data passed to HTML templates
type TemplateData struct {
	Title      string // Page title
	HeaderText string // Optional header text

	// Form handling
	FormErrors map[string]string // Validation errors (field -> message)
	FormData   map[string]string // Submitted form data (for repopulation)

	// Page-specific data
	Moods    []*data.Mood     // Slice of moods for the list page
	Mood     *data.Mood       // Single mood for edit/detail page
	Emotions []EmotionDetails // List of available emotions for dropdowns
}

// NewTemplateData creates a default TemplateData instance
func NewTemplateData() *TemplateData {
	// Populate the Emotions slice from the map for easy iteration in templates
	emotionsList := make([]EmotionDetails, 0, len(data.ValidEmotions)) // Use ValidEmotions for order
	for _, key := range data.ValidEmotions {                           // Iterate using the defined order
		if details, ok := EmotionMap[key]; ok {
			emotionsList = append(emotionsList, details)
		} else {
			// Fallback if an emotion in ValidEmotions is missing from EmotionMap
			emotionsList = append(emotionsList, EmotionDetails{Name: key, Emoji: "?", Color: "emotion-unknown"})
		}
	}

	return &TemplateData{
		Title:      "Mood Tracker", // Default title
		HeaderText: "How are you feeling?",
		FormErrors: make(map[string]string),
		FormData:   make(map[string]string),
		Emotions:   emotionsList, // Pass the ordered list
	}
}

// Helper function (can be used directly or via template funcs)
func GetEmotionDetails(emotion string) EmotionDetails {
	if details, ok := EmotionMap[emotion]; ok {
		return details
	}
	// Fallback for unknown/unexpected emotion values
	return EmotionDetails{Name: emotion, Emoji: "❓", Color: "emotion-unknown"}
}
