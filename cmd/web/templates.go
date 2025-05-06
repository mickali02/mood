// mood/cmd/web/templates.go

package main

import (
	"html/template"
	"path/filepath"
	"reflect" // Import reflect for a more robust isZero
	"time"
)

// isZero is a helper function for the 'default' template function.
// It checks if a value is the zero value for its type.
func isZero(v interface{}) bool {
	if v == nil {
		return true
	}
	// Use reflect for a more general check of zero values
	// This handles pointers, slices, maps, strings, numbers, etc.
	val := reflect.ValueOf(v)
	switch val.Kind() {
	case reflect.Ptr, reflect.Interface:
		return val.IsNil()
	case reflect.Slice, reflect.Map, reflect.Array, reflect.String:
		return val.Len() == 0
	case reflect.Bool:
		return !val.Bool() // Treat false as zero/unset for default. Change to 'return false' if false should be a valid given value.
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return val.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return val.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return val.Float() == 0
	case reflect.Complex64, reflect.Complex128:
		return val.Complex() == 0
	case reflect.Struct:
		// For structs, time.Time is a common one we want to check if IsZero()
		if t, ok := v.(time.Time); ok {
			return t.IsZero()
		}
		// A general struct is zero if all its fields are zero.
		// This is more complex and usually not needed for simple 'default' usage.
		// For simplicity, we'll consider non-time structs as non-zero unless explicitly nil.
		return false
	}
	return false // Default to false if type not handled or not obviously zero
}

var functions = template.FuncMap{
	"GetEmotionDetails": func(emotionName string) EmotionDetails {
		if details, ok := EmotionMap[emotionName]; ok {
			return details
		}
		return EmotionDetails{Name: emotionName, Emoji: "❓", Color: "emotion-unknown"}
	},
	"HumanDate": func(t time.Time) string {
		if t.IsZero() {
			return ""
		}
		return t.Format("Jan 02, 2006 at 15:04") // Standard format
	},
	"AddMinutes": func(t time.Time, minutes int) time.Time {
		return t.Add(time.Duration(minutes) * time.Minute)
	},
	"add": func(a, b int) int {
		return a + b
	},
	"sub": func(a, b int) int {
		return a - b
	},
	// default function: returns the default value 'dflt' if 'given' is nil or zero,
	// otherwise returns the 'given' value.
	// It now only takes one 'given' argument for simplicity with the pipe.
	// Usage: {{ .Value | default "Fallback" }}
	// Or for map access: {{ index .Map "key" | default .User.Name }}
	"default": func(dflt interface{}, given interface{}) interface{} {
		if isZero(given) {
			return dflt
		}
		return given
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

		// Look for and parse any 'fragment' templates (*.tmpl files in fragments dir)
		// This adds definitions like {{define "mood-list"}} to the set `ts`.
		ts, err = ts.ParseGlob("./ui/html/fragments/*.tmpl")
		if err != nil {
			return nil, err
		}

		// Add the template set to the cache.
		cache[name] = ts
	}

	return cache, nil
}
