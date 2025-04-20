// mood/cmd/web/handlers.go
package main

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/mickali02/mood/internal/data"
	"github.com/mickali02/mood/internal/validator"
)

// mood/cmd/web/handlers.go

// --- Handler for the Dashboard Page (Revised Logic with Filters) ---
func (app *application) showDashboardPage(w http.ResponseWriter, r *http.Request) {
	// 1. Read filter parameters from URL query
	searchQuery := r.URL.Query().Get("query")
	filterEmotion := r.URL.Query().Get("emotion")
	filterStartDateStr := r.URL.Query().Get("start_date") // expecting YYYY-MM-DD
	filterEndDateStr := r.URL.Query().Get("end_date")     // expecting YYYY-MM-DD

	// 2. Parse date strings (handle errors gracefully)
	var filterStartDate, filterEndDate time.Time
	var dateParseError error

	if filterStartDateStr != "" {
		// Use a layout that matches the HTML date input format
		filterStartDate, dateParseError = time.Parse("2006-01-02", filterStartDateStr)
		if dateParseError != nil {
			app.logger.Warn("Invalid start date format received", "date", filterStartDateStr, "error", dateParseError)
			// Ignore invalid date for filtering, but keep the string for the template
			filterStartDate = time.Time{} // Reset to zero value
		}
	}
	if filterEndDateStr != "" {
		filterEndDate, dateParseError = time.Parse("2006-01-02", filterEndDateStr)
		if dateParseError != nil {
			app.logger.Warn("Invalid end date format received", "date", filterEndDateStr, "error", dateParseError)
			filterEndDate = time.Time{} // Reset to zero value
		}
		// Optional: Ensure end date is not before start date if both provided and valid
		if !filterStartDate.IsZero() && !filterEndDate.IsZero() && filterEndDate.Before(filterStartDate) {
			app.logger.Warn("End date is before start date, ignoring end date filter", "start", filterStartDateStr, "end", filterEndDateStr)
			// Clear the parsed end date so it's not used in the query
			filterEndDate = time.Time{}
			// Keep filterEndDateStr so the user sees what they entered, even if ignored
		}
	}

	// 3. Create FilterCriteria struct
	criteria := data.FilterCriteria{
		TextQuery: searchQuery,
		Emotion:   filterEmotion,
		StartDate: filterStartDate, // Use the parsed time.Time values
		EndDate:   filterEndDate,   // Use the parsed time.Time values
	}

	// 4. Fetch moods using the combined filters
	app.logger.Info("Fetching filtered moods", "criteria", fmt.Sprintf("%+v", criteria)) // Log criteria
	moods, err := app.moods.GetFiltered(criteria)

	// Handle database errors (excluding ErrNoRows which means empty list/search)
	if err != nil && !errors.Is(err, sql.ErrNoRows) { // No need to check ErrNoRows from GetFiltered
		app.logger.Error("Failed to fetch filtered moods for dashboard", "error", err, "criteria", fmt.Sprintf("%+v", criteria))
		// Proceed to render, template will show empty state
		moods = []*data.Mood{} // Ensure moods is an empty slice on error
	}

	// 5. Prepare template data
	templateData := NewTemplateData()
	templateData.Title = "Your Dashboard"
	templateData.SearchQuery = searchQuery            // Pass text query back
	templateData.FilterEmotion = filterEmotion        // Pass selected emotion back
	templateData.FilterStartDate = filterStartDateStr // Pass original date strings back for form value
	templateData.FilterEndDate = filterEndDateStr     // Pass original date strings back for form value
	templateData.Moods = moods                        // Pass the filtered moods
	templateData.HasMoodEntries = len(moods) > 0      // Base flag on filtered results

	// 6. Render the template
	renderErr := app.render(w, http.StatusOK, "dashboard.tmpl", templateData)
	if renderErr != nil {
		app.serverError(w, r, renderErr)
	}
}

// --- NEW Handler for the Separate Landing Page ---
func (app *application) showLandingPage(w http.ResponseWriter, r *http.Request) {
	templateData := NewTemplateData()                  // Use your existing helper
	templateData.Title = "Feel Flow - Special Welcome" // Set a distinct title

	// Render the new landing.tmpl template
	err := app.render(w, http.StatusOK, "landing.tmpl", templateData)
	if err != nil {
		app.serverError(w, r, err) // Use your existing serverError helper
	}
}

// --- NEW Handler for the About Page ---
func (app *application) showAboutPage(w http.ResponseWriter, r *http.Request) {
	templateData := NewTemplateData() // Use your existing helper
	templateData.Title = "About Feel Flow"
	// No other specific data needed for a static about page usually

	// Render the about.tmpl template
	err := app.render(w, http.StatusOK, "about.tmpl", templateData)
	if err != nil {
		app.serverError(w, r, err) // Use your existing serverError helper
	}
}

// --- Home Handler ---
func (app *application) home(w http.ResponseWriter, r *http.Request) {

	templateData := NewTemplateData()
	templateData.Title = "Feel Flow"
	templateData.HeaderText = "Welcome To Feel Flow!"
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
		// ... (error handling - render mood_form again - remains the same) ...
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

	// --- Successful creation path ---
	err = app.moods.Insert(mood)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	app.logger.Info("Mood entry created successfully", "id", mood.ID)
	// --- REDIRECT CHANGE HERE ---
	http.Redirect(w, r, "/dashboard", http.StatusSeeOther) // <-- Target changed to /dashboard
}

// showEditMoodForm displays the form for editing an existing mood entry.
func (app *application) showEditMoodForm(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		app.logger.Error("Invalid ID parameter for mood edit form", "id", r.PathValue("id"), "error", err)
		app.notFound(w)
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
	templateData.Mood = mood
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

	mood := &data.Mood{
		ID:      id, // Crucial: Set the ID for the update
		Title:   title,
		Content: content,
		Emotion: emotion,
	}

	v := validator.NewValidator()
	data.ValidateMood(v, mood)

	if !v.ValidData() {
		// Fetch original mood again to pass to template for context if needed,
		// although we are primarily showing the attempted (invalid) data back.
		// Re-fetching might be redundant if template only needs mood.ID

		templateData := NewTemplateData()
		templateData.Title = fmt.Sprintf("Edit Mood Entry #%d (Error)", id) // Use ID from path
		templateData.HeaderText = "Update Your Mood Entry"
		// Decide if you want to pass the original or attempted mood here.
		// Passing attempted `mood` struct ensures user sees what they typed wrong.
		// If originalMood was fetched, you could potentially pass it too, e.g., templateData.OriginalMood = originalMood
		templateData.Mood = mood // Pass the struct with attempted data
		templateData.FormErrors = v.Errors
		// Repopulate FormData from the submitted (invalid) data
		templateData.FormData = map[string]string{
			"title":   title,
			"content": content,
			"emotion": emotion,
		}
		// If not passing attempted mood struct, ensure mood ID is available if template needs it.
		// templateData.Mood = &data.Mood{ID: id} // Minimal mood if needed just for ID in form action

		app.logger.Warn("Validation failed for mood update", "id", id, "errors", v.Errors)
		// Re-render the EDIT form
		renderErr := app.render(w, http.StatusUnprocessableEntity, "mood_edit_form.tmpl", templateData)
		if renderErr != nil {
			app.serverError(w, r, renderErr)
		}
		return
	}

	// --- Successful update path ---
	err = app.moods.Update(mood) // mood struct already has ID, Title, Content, Emotion
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
	// --- REDIRECT CHANGE HERE ---
	http.Redirect(w, r, "/dashboard", http.StatusSeeOther) // Redirect to Dashboard
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
	}

	// --- REDIRECT CHANGE HERE ---
	http.Redirect(w, r, "/dashboard", http.StatusSeeOther) // Redirect to Dashboard
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

// Ensure you replace the existing updateMood and deleteMood functions in your
// mood/cmd/web/handlers.go file with these updated versions.
