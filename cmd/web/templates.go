// mood/cmd/web/templates.go
package main

import (
	"html/template"
	"path/filepath"
	"time" // Import time package for template functions if needed
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

	// Find all files matching the pattern "*.tmpl" in the html directory.
	pages, err := filepath.Glob("./ui/html/*.tmpl")
	if err != nil {
		return nil, err
	}

	for _, page := range pages {
		fileName := filepath.Base(page) // Get the filename (e.g., "moods.tmpl")

		// Create a new template set with the base filename, add functions,
		// and parse the main page template file.
		ts, err := template.New(fileName).Funcs(functions).ParseFiles(page)
		if err != nil {
			return nil, err
		}

		// --- Optional: Add Layouts/Partials ---
		// If you had base layouts or partial templates, parse them here.
		// Example:
		// ts, err = ts.ParseGlob("./ui/html/layouts/*.layout.tmpl")
		// if err != nil {
		//     return nil, err
		// }
		// ts, err = ts.ParseGlob("./ui/html/partials/*.partial.tmpl")
		// if err != nil {
		//     return nil, err
		// }
		// --- End Optional ---

		// Add the parsed template set to the cache.
		cache[fileName] = ts
	}

	return cache, nil
}
