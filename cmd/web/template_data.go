// mood/cmd/web/template_data.go
package main

import (
	"github.com/mickali02/mood/internal/data"
)

// EmotionDetails holds display info for an emotion
type EmotionDetails struct {
	// These fields below DO NOT CHANGE for this task
	Name  string
	Emoji string
	Color string
}

// Define the global mapping for emotions - Centralized map
// This map DOES NOT CHANGE for this task
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
	HasMoodEntries bool       // <-- ADD THIS LINE HERE
	LatestMood     *data.Mood // <-- ADD THIS LINE HERE

	// Form handling
	FormErrors map[string]string
	FormData   map[string]string

	// Page-specific data
	Moods    []*data.Mood
	Mood     *data.Mood // Used for edit form primarily
	Emotions []EmotionDetails
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

	// The initialization adds the new fields with their zero values (false and nil)
	return &TemplateData{
		Title:      "Mood Tracker",
		HeaderText: "How are you feeling?",
		FormErrors: make(map[string]string),
		FormData:   make(map[string]string),
		Emotions:   emotionsList,
		// HasMoodEntries defaults to false
		// LatestMood defaults to nil
	}
}

// Helper function (can be used directly or via template funcs)
// This function DOES NOT CHANGE for this task
func GetEmotionDetails(emotion string) EmotionDetails {
	if details, ok := EmotionMap[emotion]; ok {
		return details
	}
	// Fallback for unknown/unexpected emotion values
	return EmotionDetails{Name: emotion, Emoji: "❓", Color: "emotion-unknown"}
}
