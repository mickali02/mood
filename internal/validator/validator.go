// mood/internal/validator/validator.go
package validator

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// EmailRX is a compiled regular expression for basic email format validation.
// (Keep if you might add email fields later, otherwise optional)
var EmailRX = regexp.MustCompile("^[a-zA-Z0-9.!#$%&'*+/=?^_`{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$")

// Validator struct holds a map of validation errors.
type Validator struct {
	Errors map[string]string
}

// NewValidator creates and returns a new Validator instance.
func NewValidator() *Validator {
	return &Validator{
		Errors: make(map[string]string),
	}
}

// ValidData returns true if the Errors map is empty.
func (v *Validator) ValidData() bool {
	return len(v.Errors) == 0
}

// AddError adds an error message to the map if the key doesn't exist.
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

// NotBlank returns true if a string is not empty after trimming whitespace.
func NotBlank(value string) bool {
	return strings.TrimSpace(value) != ""
}

// MaxLength returns true if a string contains at most 'n' runes.
func MaxLength(value string, n int) bool {
	return utf8.RuneCountInString(value) <= n
}

// MinLength returns true if a string contains at least 'n' runes.
func MinLength(value string, n int) bool {
	return utf8.RuneCountInString(value) >= n
}

// PermittedValue returns true if a value is present in a list of permitted values.
func PermittedValue[T comparable](value T, permittedValues ...T) bool {
	for i := range permittedValues {
		if value == permittedValues[i] {
			return true
		}
	}
	return false
}

// IsValidEmail returns true if a string matches the EmailRX pattern.
// (Keep if you might add email fields later, otherwise optional)
func IsValidEmail(email string) bool {
	if MaxLength(email, 254) && EmailRX.MatchString(email) {
		return true
	}
	return false
}
