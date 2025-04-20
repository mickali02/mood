// mood/internal/validator/validator.go
package validator

import (
	"regexp" // Ensure regexp is imported
	"strings"
	"unicode/utf8"
)

// --- Regular Expressions ---

// EmailRX is a compiled regular expression for basic email format validation.
var EmailRX = regexp.MustCompile("^[a-zA-Z0-9.!#$%&'*+/=?^_`{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$")

// HexColorRX validates a hex color code (e.g., #fff, #FfFf, #ff00ff, #FF00FFAA)
var HexColorRX = regexp.MustCompile(`^#([0-9a-fA-F]{3}|[0-9a-fA-F]{4}|[0-9a-fA-F]{6}|[0-9a-fA-F]{8})$`) // <-- ADDED

// --- Validator Type ---

type Validator struct {
	Errors map[string]string
}

// New creates a new Validator instance.
func NewValidator() *Validator {
	return &Validator{
		Errors: make(map[string]string),
	}
}

// ValidData returns true if the Errors map doesn't contain any entries.
func (v *Validator) ValidData() bool {
	return len(v.Errors) == 0
}

// AddError adds an error message to the map (so long as no error already exists for
// the given key).
func (v *Validator) AddError(field string, message string) {
	if _, exists := v.Errors[field]; !exists {
		v.Errors[field] = message
	}
}

// Check adds an error message to the map only if a validation check is not 'ok'.
func (v *Validator) Check(ok bool, field string, message string) {
	if !ok {
		v.AddError(field, message)
	}
}

// --- Generic Helper Functions ---

// NotBlank returns true if a string is not empty after trimming whitespace.
func NotBlank(value string) bool {
	return strings.TrimSpace(value) != ""
}

// MaxLength returns true if a string's length (in runes) is no more than n.
func MaxLength(value string, n int) bool {
	return utf8.RuneCountInString(value) <= n
}

// MinLength returns true if a string's length (in runes) is at least n.
func MinLength(value string, n int) bool {
	return utf8.RuneCountInString(value) >= n
}

// PermittedValue returns true if a value is in a list of permitted values.
func PermittedValue[T comparable](value T, permittedValues ...T) bool {
	for i := range permittedValues {
		if value == permittedValues[i] {
			return true
		}
	}
	return false
}

// --- Specific Helper Functions ---

// IsValidEmail checks if a string looks like a valid email address.
func IsValidEmail(email string) bool {
	if !MaxLength(email, 254) || !EmailRX.MatchString(email) {
		return false
	}
	return true
}

// Matches checks if a string value matches a specific regexp pattern.
func Matches(value string, rx *regexp.Regexp) bool { // <-- ADDED FUNCTION
	return rx.MatchString(value)
}
