// mood/cmd/web/handlers.go
package main

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	// Adjust import paths if your module name is different
	"github.com/mickali02/mood/internal/data"
	"github.com/mickali02/mood/internal/validator"
)

// --- Home Handler ---
func (app *application) home(w http.ResponseWriter, r *http.Request) {
	// Optional: Redirect to moods list or render a dedicated home page
	// http.Redirect(w, r, "/moods", http.StatusSeeOther)
	// return

	templateData := NewTemplateData()
	templateData.Title = "Mood Tracker Home"
	templateData.HeaderText = "Welcome!"
	err := app.render(w, http.StatusOK, "home.tmpl", templateData) // Assumes home.tmpl exists
	if err != nil {
		app.serverError(w, r, err)
	}
}

// --- Mood Handlers ---

// listMoods retrieves and displays all mood entries.
func (app *application) listMoods(w http.ResponseWriter, r *http.Request) {
	moods, err := app.moods.GetAll()
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	templateData := NewTemplateData()
	templateData.Title = "Your Mood Entries"
	templateData.HeaderText = "Recent Moods"
	templateData.Moods = moods
	err = app.render(w, http.StatusOK, "moods.tmpl", templateData)
	if err != nil {
		app.serverError(w, r, err)
	}
}

// showMoodForm displays the form for creating a new mood entry.
func (app *application) showMoodForm(w http.ResponseWriter, r *http.Request) {
	templateData := NewTemplateData()
	templateData.Title = "New Mood Entry"
	templateData.HeaderText = "Log Your Mood"
	err := app.render(w, http.StatusOK, "mood_form.tmpl", templateData)
	if err != nil {
		app.serverError(w, r, err)
	}
}

// createMood handles the submission of the new mood form.
func (app *application) createMood(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		app.clientError(w, http.StatusMethodNotAllowed)
		return
	}

	err := r.ParseForm()
	if err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	title := r.PostForm.Get("title")
	content := r.PostForm.Get("content")
	emotion := r.PostForm.Get("emotion")

	mood := &data.Mood{
		Title:   title,
		Content: content,
		Emotion: emotion,
	}

	v := validator.NewValidator()
	data.ValidateMood(v, mood) // Use the mood validator from data package

	if !v.ValidData() {
		templateData := NewTemplateData()
		templateData.Title = "New Mood Entry (Error)"
		templateData.HeaderText = "Log Your Mood"
		templateData.FormErrors = v.Errors
		templateData.FormData = map[string]string{
			"title":   title,
			"content": content,
			"emotion": emotion,
		}

		app.logger.Warn("Validation failed for new mood entry", "errors", v.Errors)
		err := app.render(w, http.StatusUnprocessableEntity, "mood_form.tmpl", templateData)
		if err != nil {
			app.serverError(w, r, err)
		}
		return
	}

	err = app.moods.Insert(mood)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	app.logger.Info("Mood entry created successfully", "id", mood.ID)
	// Optional: Add flash message here for success
	http.Redirect(w, r, "/moods", http.StatusSeeOther)
}

// showEditMoodForm displays the form for editing an existing mood entry.
func (app *application) showEditMoodForm(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		app.logger.Error("Invalid ID parameter for mood edit form", "id", r.PathValue("id"), "error", err)
		app.notFound(w) // Use notFound helper
		return
	}

	mood, err := app.moods.Get(id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			app.logger.Warn("Mood entry not found for edit", "id", id)
			app.notFound(w)
		} else {
			app.serverError(w, r, err)
		}
		return
	}

	templateData := NewTemplateData()
	templateData.Title = fmt.Sprintf("Edit Mood Entry #%d", mood.ID)
	templateData.HeaderText = "Update Your Mood Entry"
	templateData.Mood = mood // Pass the fetched mood to the template
	// Pre-populate FormData from the fetched mood in case of validation errors on Update
	templateData.FormData = map[string]string{
		"title":   mood.Title,
		"content": mood.Content,
		"emotion": mood.Emotion,
	}

	err = app.render(w, http.StatusOK, "mood_edit_form.tmpl", templateData)
	if err != nil {
		app.serverError(w, r, err)
	}
}

// updateMood handles the submission of the mood edit form.
func (app *application) updateMood(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		app.clientError(w, http.StatusMethodNotAllowed)
		return
	}

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		app.logger.Error("Invalid ID parameter for mood update", "id", r.PathValue("id"), "error", err)
		app.notFound(w)
		return
	}

	err = r.ParseForm()
	if err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	title := r.PostForm.Get("title")
	content := r.PostForm.Get("content")
	emotion := r.PostForm.Get("emotion")

	// It's often good practice to fetch the original record to ensure it exists
	// and maybe for comparison or logging, but Update handles ErrNoRows too.
	// For simplicity, we proceed directly to validation & update.

	mood := &data.Mood{
		ID:      id, // Crucial: Set the ID for the update
		Title:   title,
		Content: content,
		Emotion: emotion,
	}

	v := validator.NewValidator()
	data.ValidateMood(v, mood)

	if !v.ValidData() {
		templateData := NewTemplateData()
		templateData.Title = fmt.Sprintf("Edit Mood Entry #%d (Error)", mood.ID)
		templateData.HeaderText = "Update Your Mood Entry"
		// Pass the *attempted* mood data back, not necessarily the original fetched one
		templateData.Mood = mood
		templateData.FormErrors = v.Errors
		// Repopulate FormData from the submitted (invalid) data
		templateData.FormData = map[string]string{
			"title":   title,
			"content": content,
			"emotion": emotion,
		}

		app.logger.Warn("Validation failed for mood update", "id", id, "errors", v.Errors)
		// Re-render the EDIT form
		err := app.render(w, http.StatusUnprocessableEntity, "mood_edit_form.tmpl", templateData)
		if err != nil {
			app.serverError(w, r, err)
		}
		return
	}

	err = app.moods.Update(mood)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			app.logger.Warn("Mood entry not found for update", "id", id)
			app.notFound(w)
		} else {
			app.serverError(w, r, err)
		}
		return
	}

	app.logger.Info("Mood entry updated successfully", "id", mood.ID)
	// Optional: Add flash message here for success
	http.Redirect(w, r, "/moods", http.StatusSeeOther)
}

// deleteMood handles the deletion of a mood entry.
func (app *application) deleteMood(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		app.clientError(w, http.StatusMethodNotAllowed)
		return
	}

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		app.logger.Error("Invalid ID parameter for mood delete", "id", r.PathValue("id"), "error", err)
		app.notFound(w)
		return
	}

	err = app.moods.Delete(id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Item already deleted, arguably not an error for the *user*
			app.logger.Warn("Attempted to delete non-existent mood entry", "id", id)
			// Redirecting is fine, maybe add a flash message "Item already deleted"
		} else {
			app.serverError(w, r, err)
			return // Don't redirect on server error
		}
	} else {
		app.logger.Info("Mood entry deleted successfully", "id", id)
		// Optional: Add flash message here for success
	}

	http.Redirect(w, r, "/moods", http.StatusSeeOther)
}

// --- Error Helpers ---

func (app *application) serverError(w http.ResponseWriter, r *http.Request, err error) {
	var (
		method = r.Method
		uri    = r.URL.RequestURI()
	)
	// Log detailed error
	app.logger.Error("Internal server error", "error", err.Error(), "method", method, "uri", uri)
	// Send generic response to client
	http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
}

func (app *application) clientError(w http.ResponseWriter, status int) {
	http.Error(w, http.StatusText(status), status)
}

// notFound is a convenience wrapper for sending a 404 Not Found response.
func (app *application) notFound(w http.ResponseWriter) {
	app.clientError(w, http.StatusNotFound)
}
