// mood/cmd/web/template_data.go
package main

import (
	"html/template"
	"time"

	"github.com/mickali02/mood/internal/data"
)

// displayMood struct definition
type displayMood struct {
	ID         int64
	CreatedAt  time.Time // Needs time import
	UpdatedAt  time.Time // Needs time import
	Title      string
	Content    template.HTML // Needs html/template import
	RawContent string
	Emotion    string
	Emoji      string
	Color      string
}

// EmotionDetails struct definition
type EmotionDetails struct {
	Name  string
	Emoji string
	Color string
}

// EmotionMap definition
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
	DisplayMoods      []displayMood        // For dashboard entries
	Mood              *data.Mood           // For edit form
	DefaultEmotions   []EmotionDetails     // For forms
	AvailableEmotions []data.EmotionDetail // For dashboard filter
	// --- Metadata field uses data.Metadata ---
	Metadata data.Metadata // For pagination controls
}

// NewTemplateData creates a default TemplateData instance
func NewTemplateData() *TemplateData {
	// Populate the STATIC DefaultEmotions slice
	defaultEmotionsList := make([]EmotionDetails, 0, len(data.ValidEmotions))
	for _, key := range data.ValidEmotions {
		if details, ok := EmotionMap[key]; ok {
			defaultEmotionsList = append(defaultEmotionsList, details)
		} else {
			defaultEmotionsList = append(defaultEmotionsList, EmotionDetails{Name: key, Emoji: "❓", Color: "#cccccc"})
		}
	}

	return &TemplateData{
		Title:           "Mood Tracker",
		HeaderText:      "How are you feeling?",
		FormErrors:      make(map[string]string),
		FormData:        make(map[string]string),
		DefaultEmotions: defaultEmotionsList,
		DisplayMoods:    make([]displayMood, 0),
		Metadata:        data.Metadata{}, // Initialize metadata using type from data package
	}
}

// GetEmotionDetails function (no changes)
func GetEmotionDetails(emotionName string) EmotionDetails {
	if details, ok := EmotionMap[emotionName]; ok {
		return details
	}
	return EmotionDetails{Name: emotionName, Emoji: "❓", Color: "#cccccc"}
}

func (app *application) newTemplateData() *TemplateData {
	return NewTemplateData()
}
