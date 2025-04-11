// mood/cmd/web/render.go
package main

import (
	"bytes"
	"fmt"
	"net/http"
	// "sync" // Uncomment if using sync.Pool
)

/* // Uncomment if using sync.Pool for buffer optimization
var bufferPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer) // Use new() for pointer
	},
}
*/

// render retrieves a template, executes it, and writes to the response.
func (app *application) render(w http.ResponseWriter, status int, page string, data *TemplateData) error {
	// --- Template Lookup ---
	ts, ok := app.templateCache[page]
	if !ok {
		// Use fmt.Errorf for error wrapping
		err := fmt.Errorf("template %q does not exist", page)
		// Log the specific error clearly
		app.logger.Error("template lookup failed", "template", page, "error", err.Error())
		// It's better to call serverError here to handle the response
		// Or return the error and let the caller handle it. Returning is cleaner.
		return err
	}

	// --- Buffer Initialization ---
	// buf := bufferPool.Get().(*bytes.Buffer) // Uncomment if using sync.Pool
	// buf.Reset() // Uncomment if using sync.Pool
	// defer bufferPool.Put(buf) // Uncomment if using sync.Pool

	// Use a simple buffer if not using sync.Pool
	buf := new(bytes.Buffer) // More idiomatic way to get a pointer to a buffer

	// --- Template Execution ---
	// Execute the template into the buffer.
	// Pass the template data directly.
	err := ts.ExecuteTemplate(buf, page, data) // Use ExecuteTemplate with page name
	if err != nil {
		err = fmt.Errorf("failed to execute template %q: %w", page, err)
		app.logger.Error("template execution failed", "template", page, "error", err.Error())
		return err // Return error for caller to handle
	}

	// --- Response Writing ---
	// Set content type header before writing status, though after is usually fine too.
	// w.Header().Set("Content-Type", "text/html; charset=utf-8") // Good practice
	w.WriteHeader(status) // Set the HTTP status code

	// Write the contents of the buffer to the http.ResponseWriter.
	_, err = buf.WriteTo(w)
	if err != nil {
		// It's hard to recover if writing to the response fails. Log it.
		err = fmt.Errorf("failed to write template buffer to response: %w", err)
		app.logger.Error("response writing failed", "error", err.Error())
		// Return error, though the response might be partially written.
		return err
	}

	return nil
}
