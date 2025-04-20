// mood/cmd/web/handlers.go
package main

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time" // Make sure time is imported

	"github.com/mickali02/mood/internal/data"
	"github.com/mickali02/mood/internal/validator"
)

// --- Handler for the Dashboard Page (Corrected Types) ---
func (app *application) showDashboardPage(w http.ResponseWriter, r *http.Request) {
	// ... (1. Read filters, 2. Parse dates - remain the same) ...
	searchQuery := r.URL.Query().Get("query")
	filterEmotion := r.URL.Query().Get("emotion")
	filterStartDateStr := r.URL.Query().Get("start_date")
	filterEndDateStr := r.URL.Query().Get("end_date")
	var filterStartDate, filterEndDate time.Time
	var dateParseError error
	if filterStartDateStr != "" { /* ... date parsing ... */
		filterStartDate, dateParseError = time.Parse("2006-01-02", filterStartDateStr)
		if dateParseError != nil {
			app.logger.Warn("Invalid start date format", "date", filterStartDateStr, "error", dateParseError)
			filterStartDate = time.Time{}
		}
	}
	if filterEndDateStr != "" { /* ... date parsing ... */
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

	// 3. Create FilterCriteria struct (using data package type)
	criteria := data.FilterCriteria{ // <-- Use data.FilterCriteria
		TextQuery: searchQuery,
		Emotion:   filterEmotion,
		StartDate: filterStartDate,
		EndDate:   filterEndDate,
	}

	// 4. Fetch filtered moods
	app.logger.Info("Fetching filtered moods", "criteria", fmt.Sprintf("%+v", criteria))
	moods, err := app.moods.GetFiltered(criteria)
	if err != nil {
		app.logger.Error("Failed to fetch filtered moods", "error", err, "criteria", fmt.Sprintf("%+v", criteria))
		moods = []*data.Mood{}
	}

	// --- 5. Fetch distinct emotions using data method ---
	availableEmotions, err := app.moods.GetDistinctEmotionDetails() // Returns []data.EmotionDetail
	if err != nil {
		app.logger.Error("Failed to fetch distinct emotions for filter", "error", err)
		availableEmotions = []data.EmotionDetail{} // <-- Use data.EmotionDetail{} for empty slice
	}
	// --- End Fetch ---

	// 6. Prepare template data
	templateData := NewTemplateData()
	templateData.Title = "Dashboard"
	templateData.SearchQuery = searchQuery
	templateData.FilterEmotion = filterEmotion
	templateData.FilterStartDate = filterStartDateStr
	templateData.FilterEndDate = filterEndDateStr
	templateData.Moods = moods
	templateData.HasMoodEntries = len(moods) > 0
	templateData.AvailableEmotions = availableEmotions // <-- Assign the []data.EmotionDetail slice

	// 7. Render the template
	renderErr := app.render(w, http.StatusOK, "dashboard.tmpl", templateData)
	if renderErr != nil {
		app.serverError(w, r, renderErr)
	}
}

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

// --- Home Handler (No Changes) ---
func (app *application) home(w http.ResponseWriter, r *http.Request) {
	templateData := NewTemplateData()
	templateData.Title = "Feel Flow"
	templateData.HeaderText = "Welcome To Feel Flow!"
	err := app.render(w, http.StatusOK, "home.tmpl", templateData)
	if err != nil {
		app.serverError(w, r, err)
	}
}

// --- Mood Handlers ---

// listMoods (Likely to be removed or refactored if dashboard is primary view)
func (app *application) listMoods(w http.ResponseWriter, r *http.Request) {
	// This handler likely becomes redundant if dashboard handles listing/filtering
	app.logger.Warn("Accessed deprecated /moods endpoint")
	http.Redirect(w, r, "/dashboard", http.StatusPermanentRedirect) // Redirect to dashboard

	/* -- Original Logic (kept for reference) --
	   moods, err := app.moods.GetAll()
	   if err != nil {
	       app.serverError(w, r, err)
	       return
	   }
	   templateData := NewTemplateData()
	   templateData.Title = "Your Mood Entries"
	   templateData.HeaderText = "Recent Moods"
	   templateData.Moods = moods // Pass the full list
	   err = app.render(w, http.StatusOK, "moods.tmpl", templateData)
	   if err != nil {
	       app.serverError(w, r, err)
	   }
	*/
}

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

// updateMood (Needs updating for custom emotions)
func (app *application) updateMood(w http.ResponseWriter, r *http.Request) {
	// TODO: Update this handler to read hidden emoji/color fields from the
	// (future) updated mood_edit_form.tmpl, similar to createMood.

	// --- Current Placeholder Logic (based on OLD form) ---
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

	err = r.ParseForm()
	if err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	// Read OLD form fields (will change)
	title := r.PostForm.Get("title")
	content := r.PostForm.Get("content")
	emotionName := r.PostForm.Get("emotion") // From OLD dropdown
	// Missing: emoji, color from new hidden fields in updated form

	// Temp: Fetch existing to potentially get emoji/color (will be replaced)
	existingMood, err := app.moods.Get(id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			app.notFound(w)
			return
		}
		app.serverError(w, r, err)
		return
	}
	tempEmoji := existingMood.Emoji                 // Fallback
	tempColor := existingMood.Color                 // Fallback
	if details, ok := EmotionMap[emotionName]; ok { // If default emotion selected
		tempEmoji = details.Emoji
		tempColor = GetEmotionDetails(emotionName).Color // Use helper
	}
	// --- End Temporary Logic ---

	// Populate mood struct (using temp emoji/color)
	mood := &data.Mood{
		ID:      id,
		Title:   title,
		Content: content,
		Emotion: emotionName,
		Emoji:   tempEmoji, // TODO: Replace with value from hidden form field
		Color:   tempColor, // TODO: Replace with value from hidden form field
	}

	v := validator.NewValidator()
	data.ValidateMood(v, mood) // Validate everything

	if !v.ValidData() {
		templateData := NewTemplateData()
		templateData.Title = fmt.Sprintf("Edit Mood Entry #%d (Error)", id)
		templateData.HeaderText = "Update Your Mood Entry"
		templateData.Mood = mood // Pass attempted mood back
		templateData.FormErrors = v.Errors
		templateData.FormData = map[string]string{ // Repopulate with attempted data
			"title":   title,
			"content": content,
			"emotion": emotionName,
			"emoji":   tempEmoji,
			"color":   tempColor,
		}
		app.logger.Warn("Validation failed for mood update", "id", id, "errors", v.Errors)
		// Render OLD edit form template
		renderErr := app.render(w, http.StatusUnprocessableEntity, "mood_edit_form.tmpl", templateData)
		if renderErr != nil {
			app.serverError(w, r, renderErr)
		}
		return
	}

	// --- Successful update path ---
	err = app.moods.Update(mood) // Update now includes emoji/color
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			app.notFound(w)
			return
		}
		app.serverError(w, r, err)
		return
	}

	app.logger.Info("Mood entry updated successfully", "id", mood.ID)
	http.Redirect(w, r, "/dashboard", http.StatusSeeOther) // Redirect to dashboard
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
