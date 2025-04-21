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

// --- Handler for the Dashboard Page (HTMX Swaps Larger Block) ---
func (app *application) showDashboardPage(w http.ResponseWriter, r *http.Request) {
	// 1. Read filter parameters
	searchQuery := r.URL.Query().Get("query")
	filterCombinedEmotion := r.URL.Query().Get("emotion")
	filterStartDateStr := r.URL.Query().Get("start_date")
	filterEndDateStr := r.URL.Query().Get("end_date")

	// 2. Parse date strings
	var filterStartDate, filterEndDate time.Time
	var dateParseError error
	if filterStartDateStr != "" {
		filterStartDate, dateParseError = time.Parse("2006-01-02", filterStartDateStr)
		if dateParseError != nil {
			app.logger.Warn("Invalid start date format", "date", filterStartDateStr, "error", dateParseError)
			filterStartDate = time.Time{}
		}
	}
	if filterEndDateStr != "" {
		filterEndDate, dateParseError = time.Parse("2006-01-02", filterEndDateStr)
		if dateParseError != nil {
			app.logger.Warn("Invalid end date format", "date", filterEndDateStr, "error", dateParseError)
			filterEndDate = time.Time{}
		}
		if !filterStartDate.IsZero() && !filterEndDate.IsZero() && filterEndDate.Before(filterStartDate) {
			app.logger.Warn("End date before start date", "start", filterStartDateStr, "end", filterEndDateStr)
			filterEndDate = time.Time{}
		}
	}

	// 3. Create FilterCriteria struct
	criteria := data.FilterCriteria{
		TextQuery: searchQuery,
		Emotion:   filterCombinedEmotion,
		StartDate: filterStartDate,
		EndDate:   filterEndDate,
	}

	// 4. Fetch filtered moods
	app.logger.Info("Fetching filtered moods", "criteria", fmt.Sprintf("%+v", criteria))
	moods, err := app.moods.GetFiltered(criteria)
	if err != nil {
		app.logger.Error("Failed to fetch filtered moods", "error", err)
		moods = []*data.Mood{}
	}

	// 5. Fetch distinct emotions
	availableEmotions, err := app.moods.GetDistinctEmotionDetails()
	if err != nil {
		app.logger.Error("Failed to fetch distinct emotions", "error", err)
		availableEmotions = []data.EmotionDetail{}
	}

	// 6. Prepare template data
	templateData := NewTemplateData()
	templateData.Title = "Dashboard"
	templateData.SearchQuery = searchQuery
	templateData.FilterEmotion = filterCombinedEmotion
	templateData.FilterStartDate = filterStartDateStr
	templateData.FilterEndDate = filterEndDateStr
	templateData.Moods = moods
	templateData.HasMoodEntries = len(moods) > 0 // Use len(moods) which reflects filters
	templateData.AvailableEmotions = availableEmotions

	// 7. Handle HTMX request
	if r.Header.Get("HX-Request") == "true" {
		app.logger.Info("Handling HTMX request for dashboard content area") // Log reflects larger block
		ts, ok := app.templateCache["dashboard.tmpl"]
		if !ok {
			app.serverError(w, r, fmt.Errorf("template %q does not exist", "dashboard.tmpl"))
			return
		}
		// Execute the block containing filters AND list
		err = ts.ExecuteTemplate(w, "dashboard-content", templateData) // <-- Use block name for the wrapper
		if err != nil {
			app.serverError(w, r, fmt.Errorf("failed to execute template block 'dashboard-content': %w", err))
		}
	} else {
		app.logger.Info("Handling full page request for dashboard")
		// Render the whole page normally for initial load
		err = app.render(w, http.StatusOK, "dashboard.tmpl", templateData)
		if err != nil {
			// app.render logs its own errors, but maybe log context here
			app.logger.Error("Full page render failed", "error", err)
		}
	}
} // End showDashboardPage

// --- Landing Page Handler (No Changes) ---
func (app *application) showLandingPage(w http.ResponseWriter, r *http.Request) {
	templateData := NewTemplateData()
	templateData.Title = "Feel Flow - Special Welcome"
	err := app.render(w, http.StatusOK, "landing.tmpl", templateData)
	if err != nil {
		app.serverError(w, r, err)
	}
}

// --- About Page Handler (No Changes) ---
func (app *application) showAboutPage(w http.ResponseWriter, r *http.Request) {
	templateData := NewTemplateData()
	templateData.Title = "About Feel Flow"
	err := app.render(w, http.StatusOK, "about.tmpl", templateData)
	if err != nil {
		app.serverError(w, r, err)
	}
}

// --- Mood Handlers ---

// showMoodForm (Uses DefaultEmotions from NewTemplateData)
func (app *application) showMoodForm(w http.ResponseWriter, r *http.Request) {
	templateData := NewTemplateData() // Populates .DefaultEmotions
	templateData.Title = "New Mood Entry"
	templateData.HeaderText = "Log Your Mood"
	templateData.FormData = make(map[string]string) // Ensure FormData exists

	err := app.render(w, http.StatusOK, "mood_form.tmpl", templateData)
	if err != nil {
		app.serverError(w, r, err)
	}
}

// createMood (Handles custom emotion fields)
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

	// Read submitted data, including new hidden fields
	title := r.PostForm.Get("title")
	content := r.PostForm.Get("content")
	emotionName := r.PostForm.Get("emotion") // From hidden input#final_emotion_name
	emoji := r.PostForm.Get("emoji")         // From hidden input#final_emotion_emoji
	color := r.PostForm.Get("color")         // From hidden input#final_emotion_color

	// Populate the Mood struct with all fields
	mood := &data.Mood{
		Title:   title,
		Content: content,
		Emotion: emotionName,
		Emoji:   emoji,
		Color:   color,
	}

	// Validate the mood struct (including new fields)
	v := validator.NewValidator()
	// Make sure your validator package includes the HexColorRX regex and Matches function
	data.ValidateMood(v, mood)

	if !v.ValidData() {
		// Validation failed, re-render the form with errors and submitted data
		templateData := NewTemplateData()
		templateData.Title = "New Mood Entry (Error)"
		templateData.HeaderText = "Log Your Mood"
		templateData.FormErrors = v.Errors
		// Repopulate FormData with ALL submitted values for hidden fields too
		// Also include the radio button choice to help JS re-select it
		templateData.FormData = map[string]string{
			"title":          title,
			"content":        content,
			"emotion":        emotionName,                      // Value from hidden #final_emotion_name
			"emoji":          emoji,                            // Value from hidden #final_emotion_emoji
			"color":          color,                            // Value from hidden #final_emotion_color
			"emotion_choice": r.PostForm.Get("emotion_choice"), // Value from selected radio
		}

		app.logger.Warn("Validation failed for new mood entry", "errors", v.Errors)
		err := app.render(w, http.StatusUnprocessableEntity, "mood_form.tmpl", templateData)
		if err != nil {
			app.serverError(w, r, err)
		}
		return
	}

	// --- Successful creation path ---
	err = app.moods.Insert(mood) // Insert now includes emoji/color
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	app.logger.Info("Mood entry created successfully", "id", mood.ID)
	http.Redirect(w, r, "/dashboard", http.StatusSeeOther) // Redirect to dashboard
}

// showEditMoodForm (Needs updating for custom emotions)
func (app *application) showEditMoodForm(w http.ResponseWriter, r *http.Request) {
	// TODO: Update this function and mood_edit_form.tmpl to support editing custom emotions.
	// This will likely involve:
	// 1. Fetching the mood (already done, includes emoji/color).
	// 2. Passing DefaultEmotions to the template.
	// 3. Updating the template to use radios + modal like mood_form.tmpl.
	// 4. Using JS on the edit form to pre-select the correct radio (default or other)
	//    and pre-fill the modal if the current mood is custom.

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		app.logger.Error("Invalid ID parameter for edit", "id", r.PathValue("id"), "error", err)
		app.notFound(w)
		return
	}

	mood, err := app.moods.Get(id) // Get now fetches emoji/color too
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			app.logger.Warn("Mood not found for edit", "id", id)
			app.notFound(w)
		} else {
			app.serverError(w, r, err)
		}
		return
	}

	templateData := NewTemplateData()
	templateData.Title = fmt.Sprintf("Edit Mood Entry #%d", mood.ID)
	templateData.HeaderText = "Update Your Mood Entry"
	templateData.Mood = mood // Pass the full mood object (includes emoji/color)

	// Pre-populate FormData (used if update validation fails)
	templateData.FormData = map[string]string{
		"title":   mood.Title,
		"content": mood.Content,
		"emotion": mood.Emotion,
		"emoji":   mood.Emoji, // Pass existing emoji
		"color":   mood.Color, // Pass existing color
		// We'll need JS to determine the initial 'emotion_choice' radio state
	}

	// Render the OLD edit form template for now.
	// It won't correctly handle editing custom details yet.
	app.logger.Warn("Rendering OLD edit form - does not support custom emotion editing yet")
	err = app.render(w, http.StatusOK, "mood_edit_form.tmpl", templateData)
	if err != nil {
		app.serverError(w, r, err)
	}
}

// updateMood (UPDATED to handle custom emotion fields)
func (app *application) updateMood(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		app.clientError(w, http.StatusMethodNotAllowed)
		return
	}

	// Get ID from path
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		app.logger.Error("Invalid ID parameter for mood update", "id", r.PathValue("id"), "error", err)
		app.notFound(w)
		return
	}

	// Parse the form data
	err = r.ParseForm()
	if err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	// Read submitted data, including hidden fields for emotion details
	title := r.PostForm.Get("title")
	content := r.PostForm.Get("content")
	emotionName := r.PostForm.Get("emotion") // From hidden input#final_emotion_name
	emoji := r.PostForm.Get("emoji")         // From hidden input#final_emotion_emoji
	color := r.PostForm.Get("color")         // From hidden input#final_emotion_color

	// Populate the Mood struct with ID and all submitted fields
	mood := &data.Mood{
		ID:      id, // Set the ID for the update
		Title:   title,
		Content: content,
		Emotion: emotionName,
		Emoji:   emoji,
		Color:   color,
	}

	// Validate the mood struct (including new fields)
	v := validator.NewValidator()
	data.ValidateMood(v, mood) // ValidateMood checks emoji/color format

	if !v.ValidData() {
		// Validation failed, re-render the EDIT form with errors and submitted data
		templateData := NewTemplateData()
		templateData.Title = fmt.Sprintf("Edit Mood Entry #%d (Error)", id)
		templateData.HeaderText = "Update Your Mood Entry"
		// Pass the *attempted* mood data back so JS can potentially re-select/pre-fill
		templateData.Mood = mood
		templateData.FormErrors = v.Errors
		// Repopulate FormData with ALL submitted values for hidden fields too
		templateData.FormData = map[string]string{
			"title":          title,
			"content":        content,
			"emotion":        emotionName,                      // Value from hidden #final_emotion_name
			"emoji":          emoji,                            // Value from hidden #final_emotion_emoji
			"color":          color,                            // Value from hidden #final_emotion_color
			"emotion_choice": r.PostForm.Get("emotion_choice"), // Value from selected radio
		}

		app.logger.Warn("Validation failed for mood update", "id", id, "errors", v.Errors)
		// Re-render the EDIT form template
		err := app.render(w, http.StatusUnprocessableEntity, "mood_edit_form.tmpl", templateData)
		if err != nil {
			app.serverError(w, r, err)
		}
		return
	}

	// --- Successful update path ---
	err = app.moods.Update(mood) // Update method now includes emoji/color fields
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Mood was likely deleted between showing the form and submitting it
			app.logger.Warn("Mood entry not found for update", "id", id)
			app.notFound(w)
		} else {
			app.serverError(w, r, err)
		}
		return
	}

	app.logger.Info("Mood entry updated successfully", "id", mood.ID)
	http.Redirect(w, r, "/dashboard", http.StatusSeeOther) // Redirect back to dashboard
}

// deleteMood (Redirect already updated)
func (app *application) deleteMood(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		app.clientError(w, http.StatusMethodNotAllowed)
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		app.notFound(w)
		return
	}

	err = app.moods.Delete(id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			app.logger.Warn("Attempted to delete non-existent mood entry", "id", id)
		} else {
			app.serverError(w, r, err)
			return
		}
	} else {
		app.logger.Info("Mood entry deleted successfully", "id", id)
	}
	http.Redirect(w, r, "/dashboard", http.StatusSeeOther) // Redirect to Dashboard
}

// --- Error Helpers (No Changes) ---
func (app *application) serverError(w http.ResponseWriter, r *http.Request, err error) {
	var (
		method = r.Method
		uri    = r.URL.RequestURI()
	)
	app.logger.Error("Internal server error", "error", err.Error(), "method", method, "uri", uri)
	http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
}

func (app *application) clientError(w http.ResponseWriter, status int) {
	http.Error(w, http.StatusText(status), status)
}

func (app *application) notFound(w http.ResponseWriter) {
	app.clientError(w, http.StatusNotFound)
}
