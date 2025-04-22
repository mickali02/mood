// mood/cmd/web/handlers.go
package main

import (
	"database/sql"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"net/url" // <-- Import net/url
	"strconv"
	"time"

	"github.com/mickali02/mood/internal/data"
	"github.com/mickali02/mood/internal/validator"
)

// --- Handler for the Dashboard Page ---
func (app *application) showDashboardPage(w http.ResponseWriter, r *http.Request) {
	// 1. Read filter parameters directly
	v := validator.NewValidator() // Use validator for page number check
	query := r.URL.Query()

	searchQuery := query.Get("query") // Use direct reading
	filterCombinedEmotion := query.Get("emotion")
	filterStartDateStr := query.Get("start_date")
	filterEndDateStr := query.Get("end_date")

	// Read page parameter with validation
	pageStr := query.Get("page")
	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1 // Default to page 1 if invalid or missing
	}
	v.Check(page > 0, "page", "must be a positive integer")
	v.Check(page <= 10_000_000, "page", "must be less than 10 million")

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
		parsedEndDate, dateParseError := time.Parse("2006-01-02", filterEndDateStr)
		if dateParseError != nil {
			app.logger.Warn("Invalid end date format", "date", filterEndDateStr, "error", dateParseError)
			filterEndDate = time.Time{}
		} else {
			filterEndDate = parsedEndDate.Add(24*time.Hour - 1*time.Nanosecond)
		}
		if !filterStartDate.IsZero() && !filterEndDate.IsZero() && filterEndDate.Before(filterStartDate) {
			app.logger.Warn("End date before start date", "start", filterStartDateStr, "end", filterEndDateStr)
			filterEndDate = time.Time{}
		}
	}

	if !v.ValidData() {
		app.logger.Warn("Invalid page parameter", "page", pageStr, "errors", v.Errors)
		page = 1 // Reset to default if validator fails
	}

	// 3. Create FilterCriteria struct including pagination
	criteria := data.FilterCriteria{
		TextQuery: searchQuery,
		Emotion:   filterCombinedEmotion,
		StartDate: filterStartDate,
		EndDate:   filterEndDate,
		Page:      page,
		PageSize:  4, // Set page size
	}

	// 4. Fetch filtered moods AND metadata
	app.logger.Info("Fetching filtered moods", "criteria", fmt.Sprintf("%+v", criteria))
	moods, metadata, err := app.moods.GetFiltered(criteria) // Expect 3 return values
	if err != nil {
		app.logger.Error("Failed to fetch filtered moods", "error", err)
		moods = []*data.Mood{}
		metadata = data.Metadata{}
	}

	// --- Convert moods for display ---
	displayMoods := make([]displayMood, len(moods))
	for i, m := range moods {
		displayMoods[i] = displayMood{
			ID:        m.ID,
			CreatedAt: m.CreatedAt,
			UpdatedAt: m.UpdatedAt,
			Title:     m.Title,
			Content:   template.HTML(m.Content), // Cast content to template.HTML
			Emotion:   m.Emotion,
			Emoji:     m.Emoji,
			Color:     m.Color,
		}
	}

	// 5. Fetch distinct emotions
	availableEmotions, err := app.moods.GetDistinctEmotionDetails()
	if err != nil {
		app.logger.Error("Failed to fetch distinct emotions", "error", err)
		availableEmotions = []data.EmotionDetail{}
	}

	// 6. Prepare template data, including metadata
	templateData := NewTemplateData()
	templateData.Title = "Dashboard"
	templateData.SearchQuery = searchQuery
	templateData.FilterEmotion = filterCombinedEmotion
	templateData.FilterStartDate = filterStartDateStr
	templateData.FilterEndDate = filterEndDateStr
	templateData.DisplayMoods = displayMoods
	templateData.HasMoodEntries = len(displayMoods) > 0
	templateData.AvailableEmotions = availableEmotions
	templateData.Metadata = metadata // Pass metadata

	// 7. Handle HTMX request or full page load
	if r.Header.Get("HX-Request") == "true" {
		app.logger.Info("Handling HTMX request for dashboard content area")
		ts, ok := app.templateCache["dashboard.tmpl"]
		if !ok {
			app.logger.Error("Template lookup failed for HTMX request", "template", "dashboard.tmpl")
			app.serverError(w, r, fmt.Errorf("template %q does not exist", "dashboard.tmpl"))
			return
		}
		err = ts.ExecuteTemplate(w, "dashboard-content", templateData)
		if err != nil {
			app.logger.Error("Failed to execute template block", "block", "dashboard-content", "error", err)
		}
	} else {
		app.logger.Info("Handling full page request for dashboard")
		err = app.render(w, http.StatusOK, "dashboard.tmpl", templateData)
		if err != nil {
			app.logger.Error("Full page render failed", "template", "dashboard.tmpl", "error", err)
		}
	}
}

// --- Landing Page Handler ---
func (app *application) showLandingPage(w http.ResponseWriter, r *http.Request) {
	templateData := NewTemplateData()
	templateData.Title = "Feel Flow - Special Welcome"
	err := app.render(w, http.StatusOK, "landing.tmpl", templateData)
	if err != nil {
		app.serverError(w, r, err)
	}
}

// --- About Page Handler ---
func (app *application) showAboutPage(w http.ResponseWriter, r *http.Request) {
	templateData := NewTemplateData()
	templateData.Title = "About Feel Flow"
	err := app.render(w, http.StatusOK, "about.tmpl", templateData)
	if err != nil {
		app.serverError(w, r, err)
	}
}

// --- Mood Handlers ---

// showMoodForm
func (app *application) showMoodForm(w http.ResponseWriter, r *http.Request) {
	templateData := NewTemplateData()
	templateData.Title = "New Mood Entry"
	templateData.HeaderText = "Log Your Mood"
	templateData.FormData = make(map[string]string)
	err := app.render(w, http.StatusOK, "mood_form.tmpl", templateData)
	if err != nil {
		app.serverError(w, r, err)
	}
}

// createMood (HTMX Enhanced)
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
	emotionName := r.PostForm.Get("emotion")
	emoji := r.PostForm.Get("emoji")
	color := r.PostForm.Get("color")

	mood := &data.Mood{
		Title:   title,
		Content: content,
		Emotion: emotionName,
		Emoji:   emoji,
		Color:   color,
	}

	v := validator.NewValidator()
	data.ValidateMood(v, mood)

	if !v.ValidData() {
		templateData := NewTemplateData()
		templateData.Title = "New Mood Entry (Error)"
		templateData.HeaderText = "Log Your Mood"
		templateData.FormErrors = v.Errors
		templateData.FormData = map[string]string{
			"title":          title,
			"content":        content,
			"emotion":        emotionName,
			"emoji":          emoji,
			"color":          color,
			"emotion_choice": r.PostForm.Get("emotion_choice"),
		}

		app.logger.Warn("Validation failed for new mood entry", "errors", v.Errors)
		ts, ok := app.templateCache["mood_form.tmpl"]
		if !ok {
			app.serverError(w, r, fmt.Errorf("template %q does not exist", "mood_form.tmpl"))
			return
		}
		w.WriteHeader(http.StatusUnprocessableEntity)                  // 422 for validation errors
		err = ts.ExecuteTemplate(w, "mood-form-content", templateData) // Render block
		if err != nil {
			app.logger.Error("Failed to execute mood-form-content block on validation error", "error", err)
		}
		return
	}

	err = app.moods.Insert(mood)
	if err != nil {
		app.serverError(w, r, err)
		return
	}
	app.logger.Info("Mood entry created successfully", "id", mood.ID)

	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", "/dashboard") // Client-side redirect via HTMX
		w.WriteHeader(http.StatusOK)                // Or 201 Created
		return
	}
	http.Redirect(w, r, "/dashboard", http.StatusSeeOther) // Standard redirect
}

// showEditMoodForm
func (app *application) showEditMoodForm(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		app.notFound(w)
		return
	}
	mood, err := app.moods.Get(id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
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
	templateData.FormData = map[string]string{
		"title":   mood.Title,
		"content": mood.Content, // Pass existing HTML content
		"emotion": mood.Emotion,
		"emoji":   mood.Emoji,
		"color":   mood.Color,
	}

	// Render the full edit page initially
	err = app.render(w, http.StatusOK, "mood_edit_form.tmpl", templateData)
	if err != nil {
		app.serverError(w, r, err)
	}
}

// updateMood (HTMX Enhanced)
func (app *application) updateMood(w http.ResponseWriter, r *http.Request) {
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
	// Get original mood to pass back on validation error
	originalMood, err := app.moods.Get(id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			app.notFound(w)
		} else {
			app.serverError(w, r, err)
		}
		return
	}

	err = r.ParseForm()
	if err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	title := r.PostForm.Get("title")
	content := r.PostForm.Get("content")
	emotionName := r.PostForm.Get("emotion")
	emoji := r.PostForm.Get("emoji")
	color := r.PostForm.Get("color")

	mood := &data.Mood{
		ID:      id,
		Title:   title,
		Content: content,
		Emotion: emotionName,
		Emoji:   emoji,
		Color:   color,
	}

	v := validator.NewValidator()
	data.ValidateMood(v, mood)

	if !v.ValidData() {
		templateData := NewTemplateData()
		templateData.Title = fmt.Sprintf("Edit Mood Entry #%d (Error)", id)
		templateData.HeaderText = "Update Your Mood Entry"
		templateData.Mood = originalMood // Pass original mood for reference
		templateData.FormErrors = v.Errors
		templateData.FormData = map[string]string{ // Pass submitted data back
			"title":          title,
			"content":        content,
			"emotion":        emotionName,
			"emoji":          emoji,
			"color":          color,
			"emotion_choice": r.PostForm.Get("emotion_choice"),
		}

		app.logger.Warn("Validation failed for mood update", "id", id, "errors", v.Errors)
		ts, ok := app.templateCache["mood_edit_form.tmpl"]
		if !ok {
			app.serverError(w, r, fmt.Errorf("template %q does not exist", "mood_edit_form.tmpl"))
			return
		}
		w.WriteHeader(http.StatusUnprocessableEntity)                  // 422
		err = ts.ExecuteTemplate(w, "mood-form-content", templateData) // Render block
		if err != nil {
			app.logger.Error("Failed to execute mood-form-content block on validation error (update)", "error", err)
		}
		return
	}

	err = app.moods.Update(mood)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			app.notFound(w)
		} else {
			app.serverError(w, r, err)
		}
		return
	}
	app.logger.Info("Mood entry updated successfully", "id", mood.ID)

	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", "/dashboard") // Client-side redirect via HTMX
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/dashboard", http.StatusSeeOther) // Standard redirect
}

// deleteMood (HTMX Enhanced - Now re-renders dashboard content)
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
	deleteErrOccurred := false // Flag to track if a real delete error happened
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			app.logger.Warn("Attempted to delete non-existent mood entry", "id", id)
			// Continue as if successful for HTMX refresh
		} else {
			app.serverError(w, r, err) // Real DB error
			deleteErrOccurred = true
		}
	} else {
		app.logger.Info("Mood entry deleted successfully", "id", id)
	}

	// --- HTMX Response: Re-render dashboard content ---
	if r.Header.Get("HX-Request") == "true" {
		// If a critical DB error happened during delete, don't proceed
		if deleteErrOccurred {
			return // serverError likely already wrote response
		}

		// Re-fetch and re-render logic (simplified using Referer)
		currentPage := 1
		searchQuery := ""
		filterCombinedEmotion := ""
		filterStartDateStr := ""
		filterEndDateStr := ""

		refererURL, err := url.Parse(r.Header.Get("Referer"))
		if err == nil {
			refQuery := refererURL.Query()
			searchQuery = refQuery.Get("query")
			filterCombinedEmotion = refQuery.Get("emotion")
			filterStartDateStr = refQuery.Get("start_date")
			filterEndDateStr = refQuery.Get("end_date")
			pageStr := refQuery.Get("page")
			parsedPage, parseErr := strconv.Atoi(pageStr)
			if parseErr == nil && parsedPage > 0 {
				currentPage = parsedPage
			}
		} else {
			app.logger.Warn("Could not parse Referer URL for delete refresh", "referer", r.Header.Get("Referer"), "error", err)
		}

		// Parse dates (same logic as showDashboardPage)
		var filterStartDate, filterEndDate time.Time
		var dateParseError error
		if filterStartDateStr != "" { /* ... date parsing ... */
			filterStartDate, dateParseError = time.Parse("2006-01-02", filterStartDateStr)
			if dateParseError != nil {
				filterStartDate = time.Time{}
			}
		}
		if filterEndDateStr != "" { /* ... date parsing ... */
			parsedEndDate, dateParseError := time.Parse("2006-01-02", filterEndDateStr)
			if dateParseError != nil {
				filterEndDate = time.Time{}
			} else {
				filterEndDate = parsedEndDate.Add(24*time.Hour - 1*time.Nanosecond)
			}
			if !filterStartDate.IsZero() && !filterEndDate.IsZero() && filterEndDate.Before(filterStartDate) {
				filterEndDate = time.Time{}
			}
		}

		// Re-create criteria for the current view state
		criteria := data.FilterCriteria{
			TextQuery: searchQuery,
			Emotion:   filterCombinedEmotion,
			StartDate: filterStartDate,
			EndDate:   filterEndDate,
			Page:      currentPage,
			PageSize:  4,
		}

		// Fetch the updated data
		moods, metadata, err := app.moods.GetFiltered(criteria)
		if err != nil {
			app.logger.Error("Failed to fetch filtered moods after delete", "error", err)
			// Send back an error message within the target area
			w.WriteHeader(http.StatusOK) // Still OK, but send error HTML
			// Consider a dedicated error fragment/template
			fmt.Fprint(w, `<p class="error-message" style="text-align:center; padding: 20px;">Error refreshing list after delete.</p>`)
			return
		}

		// Convert for display
		displayMoods := make([]displayMood, len(moods))
		for i, m := range moods {
			displayMoods[i] = displayMood{ID: m.ID, CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt, Title: m.Title, Content: template.HTML(m.Content), Emotion: m.Emotion, Emoji: m.Emoji, Color: m.Color}
		}
		availableEmotions, err := app.moods.GetDistinctEmotionDetails()
		if err != nil {
			availableEmotions = []data.EmotionDetail{}
		}

		// Prepare data for the fragment
		templateData := NewTemplateData()
		templateData.SearchQuery = searchQuery
		templateData.FilterEmotion = filterCombinedEmotion
		templateData.FilterStartDate = filterStartDateStr
		templateData.FilterEndDate = filterEndDateStr
		templateData.DisplayMoods = displayMoods
		templateData.HasMoodEntries = len(displayMoods) > 0
		templateData.AvailableEmotions = availableEmotions
		templateData.Metadata = metadata

		// Render the dashboard content block
		ts, ok := app.templateCache["dashboard.tmpl"]
		if !ok {
			app.logger.Error("Template lookup failed for delete refresh", "template", "dashboard.tmpl")
			app.serverError(w, r, fmt.Errorf("template %q does not exist", "dashboard.tmpl"))
			return
		}
		w.WriteHeader(http.StatusOK) // Set OK status before writing body
		err = ts.ExecuteTemplate(w, "dashboard-content", templateData)
		if err != nil {
			app.logger.Error("Failed to execute template block for delete refresh", "block", "dashboard-content", "error", err)
		}
		return // Stop processing after sending HTMX response
	}

	// Fallback for non-HTMX requests
	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}

// --- Error Helpers ---
func (app *application) serverError(w http.ResponseWriter, r *http.Request, err error) {
	var (
		method = r.Method
		uri    = r.URL.RequestURI()
	)
	app.logger.Error("Internal server error", "error", err.Error(), "method", method, "uri", uri)
	if w.Header().Get("Content-Type") == "" {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	} else {
		app.logger.Warn("Headers already sent, cannot write server error to response body")
	}
}

func (app *application) clientError(w http.ResponseWriter, status int) {
	http.Error(w, http.StatusText(status), status)
}

func (app *application) notFound(w http.ResponseWriter) {
	app.clientError(w, http.StatusNotFound)
}
