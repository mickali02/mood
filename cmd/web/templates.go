// mood/cmd/web/templates.go

package main

import (
	"html/template"
	"path/filepath"
	"time"
	// "strings" // May need if using string funcs
)

var functions = template.FuncMap{
	// ... (Existing functions like HumanDate, AddMinutes, GetEmotionDetails) ...
	"GetEmotionDetails": func(emotionName string) EmotionDetails {
		// ... (implementation) ...
		if details, ok := EmotionMap[emotionName]; ok {
			return details
		}
		return EmotionDetails{Name: emotionName, Emoji: "❓", Color: "emotion-unknown"}
	},
	"HumanDate": func(t time.Time) string {
		// ... (implementation) ...
		if t.IsZero() {
			return ""
		}
		return t.Format("Jan 02, 2006 at 15:04")
	},
	"AddMinutes": func(t time.Time, minutes int) time.Time {
		// ... (implementation) ...
		return t.Add(time.Duration(minutes) * time.Minute)
	},

	// --- NEW Pagination Helper Functions ---
	"add": func(a, b int) int {
		return a + b
	},
	"sub": func(a, b int) int {
		return a - b
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
