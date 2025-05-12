// mood/cmd/web/render.go
package main

import (
	"bytes"
	"fmt"
	"net/http"
)

// render retrieves a template, executes it, and writes to the response.
func (app *application) render(w http.ResponseWriter, status int, page string, data *TemplateData) error {
	ts, ok := app.templateCache[page]
	if !ok {
		err := fmt.Errorf("template %q does not exist", page)
		app.logger.Error("template lookup failed", "template", page, "error", err.Error())
		// Return the error to be handled by the caller, which might call serverError
		return err
	}

	buf := new(bytes.Buffer)

	// Execute the template associated with the page name (e.g., "login.tmpl", "dashboard.tmpl")
	err := ts.ExecuteTemplate(buf, page, data) // Use the 'page' name which is the base name of the file
	if err != nil {
		err = fmt.Errorf("failed to execute template %q: %w", page, err)
		app.logger.Error("template execution failed", "template", page, "error", err.Error())
		return err
	}

	// *** SET Content-Type header BEFORE WriteHeader ***
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)

	_, err = buf.WriteTo(w)
	if err != nil {
		err = fmt.Errorf("failed to write template buffer to response: %w", err)
		app.logger.Error("response writing failed", "error", err.Error())
		// If this fails, the response might be partially written or headers already sent.
		// Logging is good, but returning error might lead to another WriteHeader if not careful.
		// For now, this is acceptable as serverError also checks if headers are sent.
		return err
	}

	return nil
}
