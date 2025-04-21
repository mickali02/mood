// mood/cmd/web/templates.go
package main

import (
	"html/template"
	"path/filepath"
	"time"
)

// Define template helper functions
var functions = template.FuncMap{
	// Function to get emotion details within the template
	"GetEmotionDetails": func(emotionName string) EmotionDetails {
		if details, ok := EmotionMap[emotionName]; ok {
			return details
		}
		// Return a default/unknown value if not found in map
		return EmotionDetails{Name: emotionName, Emoji: "❓", Color: "emotion-unknown"}
	},
	// Example time formatting function (if needed directly in template)
	"HumanDate": func(t time.Time) string {
		if t.IsZero() {
			return ""
		}
		// Format time like "Jan 02, 2006 at 3:04 PM"
		return t.Format("Jan 02, 2006 at 15:04")
	},
	// Function needed for time comparison in moods.tmpl
	// It's generally better to do comparisons in the handler, but this works
	"AddMinutes": func(t time.Time, minutes int) time.Time {
		return t.Add(time.Duration(minutes) * time.Minute)
	},
}

// newTemplateCache parses HTML templates on startup and stores them.
func newTemplateCache() (map[string]*template.Template, error) {
	cache := map[string]*template.Template{}

	// Get all the 'page' templates (like dashboard.tmpl, mood_form.tmpl)
	pages, err := filepath.Glob("./ui/html/*.tmpl")
	if err != nil {
		return nil, err
	}

	for _, page := range pages {
		name := filepath.Base(page)

		// Create a new template set starting with the page template.
		// Add the template functions.
		ts, err := template.New(name).Funcs(functions).ParseFiles(page)
		if err != nil {
			return nil, err
		}

		// --- MODIFIED PART ---
		// Look for and parse any 'fragment' templates (*.tmpl files in fragments dir)
		// This adds definitions like {{define "mood-list"}} to the set `ts`.
		ts, err = ts.ParseGlob("./ui/html/fragments/*.tmpl")
		if err != nil {
			return nil, err
		}
		// --- END MODIFIED PART ---

		// Add the template set to the cache.
		cache[name] = ts
	}

	return cache, nil
}
