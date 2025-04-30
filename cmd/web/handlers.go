// mood/cmd/web/handlers.go
package main

import (
	"bytes" 
	"database/sql"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"strings" 
	"time"

	"github.com/mickali02/mood/internal/data"
	"github.com/mickali02/mood/internal/validator"
)

/* ==========================================================================
   START: Dashboard Handler
   ========================================================================== */
// showDashboardPage handles requests for the main dashboard page.
// It displays a list of mood entries, potentially filtered and paginated,
// and handles both full page loads and HTMX partial updates for the mood list area.
func (app *application) showDashboardPage(w http.ResponseWriter, r *http.Request) {
	// --- 1. Extract Filter and Pagination Parameters ---
	// Retrieve filter values directly from the URL query parameters.
	// We use a validator primarily for the 'page' parameter to ensure it's a valid positive integer.
	v := validator.NewValidator() // Use validator for page number check
	query := r.URL.Query()

	// Read search text query.
	searchQuery := query.Get("query") 
	// Read selected emotion for filtering.
	filterCombinedEmotion := query.Get("emotion")
	// Read start date string for filtering.
	filterStartDateStr := query.Get("start_date")
	// Read end date string for filtering.
	filterEndDateStr := query.Get("end_date")

	// Read and validate the 'page' parameter for pagination.
	pageStr := query.Get("page")
	page, err := strconv.Atoi(pageStr)
	// If 'page' is missing, not a number, or less than 1, default to page 1.
	if err != nil || page < 1 {
		page = 1 // Default to page 1 if invalid or missing
	}
	// Use the validator to enforce rules on the page number.
	v.Check(page > 0, "page", "must be a positive integer")
	v.Check(page <= 10_000_000, "page", "must be less than 10 million")

	// --- 2. Parse Date Filter Strings ---
	// Convert the string representations of start and end dates into time.Time objects.
	var filterStartDate, filterEndDate time.Time // Initialize as zero time.
	var dateParseError error  // Variable to capture parsing errors.

	// Parse the start date if provided.
	if filterStartDateStr != "" {
		// Attempt to parse using the standard YYYY-MM-DD format.
		filterStartDate, dateParseError = time.Parse("2006-01-02", filterStartDateStr)
		// If parsing fails, log a warning and keep the start date as zero (effectively ignoring it).
		if dateParseError != nil {
			app.logger.Warn("Invalid start date format", "date", filterStartDateStr, "error", dateParseError)
			filterStartDate = time.Time{}
		}
	}

	// Parse the end date if provided.
	if filterEndDateStr != "" {
		parsedEndDate, dateParseError := time.Parse("2006-01-02", filterEndDateStr)
		// If parsing fails, log a warning and keep the end date as zero.
		if dateParseError != nil {
			app.logger.Warn("Invalid end date format", "date", filterEndDateStr, "error", dateParseError)
			filterEndDate = time.Time{}
		} else {
			// Include the whole end day up to the last nanosecond
			filterEndDate = parsedEndDate.Add(24*time.Hour - 1*time.Nanosecond)
		}
		// Check if end date is before start date after parsing
		if !filterStartDate.IsZero() && !filterEndDate.IsZero() && filterEndDate.Before(filterStartDate) {
			app.logger.Warn("End date before start date, ignoring end date", "start", filterStartDateStr, "end", filterEndDateStr)
			// Reset end date if it's invalid relative to start date
			filterEndDate = time.Time{}
		}
	}

	// --- Re-check Validator and Finalize Page Number ---
	// If the validator found issues with the page number (or potentially other fields later),
	// log the errors and reset the page to the default (1).
	if !v.ValidData() {
		app.logger.Warn("Invalid page parameter", "page", pageStr, "errors", v.Errors)
		page = 1 // Reset to default if validator fails
	}

	// --- 3. Assemble Filter Criteria ---
	// Create a FilterCriteria struct to pass all filtering and pagination parameters
	criteria := data.FilterCriteria{
		TextQuery: searchQuery,         // Text to search for in title/content.
		Emotion:   filterCombinedEmotion, // Specific emotion to filter by.
		StartDate: filterStartDate,       // Start date for filtering (time.Time object).
		EndDate:   filterEndDate,         // End date for filtering (time.Time object, adjusted to end of day).
		Page:      page,                // Current page number for pagination.
		PageSize:  4,                   // Number of items per page (hardcoded here).
	}

	// --- 4. Fetch Filtered Mood Data ---
	// Call the data layer function to retrieve mood entries matching the criteria,
	// along with pagination metadata (total records, current page, page size, etc.)
	app.logger.Info("Fetching filtered moods", "criteria", fmt.Sprintf("%+v", criteria)) // Log the criteria being used.
	// GetFiltered now returns moods, metadata, and an error.
	moods, metadata, err := app.moods.GetFiltered(criteria) 
	if err != nil {
		// If there's an error fetching moods (e.g., database connection issue), log the error.
		app.logger.Error("Failed to fetch filtered moods", "error", err)
		// Don't nil out moods, return empty slice instead
		moods = []*data.Mood{}
		metadata = data.Metadata{} // Return empty metadata
		// Optionally, set an error message in templateData to display to the user
	}

	// --- 5. Prepare Moods for Display ---
	// Convert the raw `data.Mood` structs into `displayMood` structs suitable for the template.
	// This is particularly important for converting the `Content` string into `template.HTML`
	displayMoods := make([]displayMood, len(moods)) // Pre-allocate slice for efficiency.
	for i, m := range moods {
		displayMoods[i] = displayMood{
			ID:         m.ID,
			CreatedAt:  m.CreatedAt, // Pass through timestamps.
			UpdatedAt:  m.UpdatedAt,
			Title:      m.Title,
			Content:    template.HTML(m.Content), // Cast content to template.HTML
			RawContent: m.Content,
			Emotion:    m.Emotion,
			Emoji:      m.Emoji,
			Color:      m.Color,
		}
	}

	// --- 6. Fetch Available Emotions for Filter Dropdown ---
	// Get a list of distinct emotions (and their associated emoji/color) currently present
	// in the database. This is used to populate the emotion filter dropdown in the UI.
	availableEmotions, err := app.moods.GetDistinctEmotionDetails()
	if err != nil {
		// If fetching distinct emotions fails, log the error and provide an empty slice
		// to the template, so the dropdown will simply be empty rather than causing a crash.
		app.logger.Error("Failed to fetch distinct emotions", "error", err)
		availableEmotions = []data.EmotionDetail{}
	}

	// --- 7. Prepare Data for the Template ---
	// Create the main data structure (`templateData`) that will be passed to the HTML template.
	// This includes page metadata, fetched data, form values (for repopulating filters)
	templateData := NewTemplateData()            // Initialize with common data (like flash messages, default emotions).
	templateData.Title = "Dashboard"             // Set the HTML title for the page.
	templateData.SearchQuery = searchQuery       // Pass the current search query back to repopulate the input field.
	templateData.FilterEmotion = filterCombinedEmotion // Pass the selected emotion back.
	templateData.FilterStartDate = filterStartDateStr // Pass the start date string back.
	templateData.FilterEndDate = filterEndDateStr     // Pass the end date string back.
	templateData.DisplayMoods = displayMoods       // The list of moods prepared for display.
	templateData.HasMoodEntries = len(displayMoods) > 0 // A boolean flag for easily checking if there are moods to show.
	templateData.AvailableEmotions = availableEmotions // The list of distinct emotions for the filter dropdown.
	templateData.Metadata = metadata               // Pass the pagination metadata (current page, total pages, etc.).

	// --- 8. Render Response (HTMX or Full Page) ---
	// Check if the request came from HTMX (indicated by the 'HX-Request' header).
	// This allows us to send back only the updated part of the page (the mood list)
	// instead of reloading the entire page, providing a smoother user experience.
	if r.Header.Get("HX-Request") == "true" {
		app.logger.Info("Handling HTMX request for dashboard content area")
		// Look up the specific template file containing the dashboard layout.
		ts, ok := app.templateCache["dashboard.tmpl"]
		if !ok {
			// If the template isn't found in the cache, this is a critical server error.
			err := fmt.Errorf("template %q does not exist", "dashboard.tmpl")
			app.logger.Error("Template lookup failed for HTMX request", "template", "dashboard.tmpl", "error", err)
			app.serverError(w, r, err) // Use serverError helper
			return
		}
		// Execute the specific block for HTMX requests
		err = ts.ExecuteTemplate(w, "dashboard-content", templateData)
		if err != nil {
			// Log error but don't necessarily call serverError as headers might be sent
			app.logger.Error("Failed to execute template block", "block", "dashboard-content", "error", err)
		}
	} else {
		app.logger.Info("Handling full page request for dashboard")
		// Render the full page
		err = app.render(w, http.StatusOK, "dashboard.tmpl", templateData)
		if err != nil {
			// render() already logs, but we might want different handling here
			app.logger.Error("Full page render failed", "template", "dashboard.tmpl", "error", err)
			// serverError might have already been called by render or its helpers
		}
	}
}

// --- Landing Page Handler ---
func (app *application) showLandingPage(w http.ResponseWriter, r *http.Request) {
	templateData := NewTemplateData()
	templateData.Title = "Feel Flow - Special Welcome"
	err := app.render(w, http.StatusOK, "landing.tmpl", templateData)
	if err != nil {
		app.serverError(w, r, err) // Use helper
	}
}

// --- About Page Handler ---
func (app *application) showAboutPage(w http.ResponseWriter, r *http.Request) {
	templateData := NewTemplateData()
	templateData.Title = "About Feel Flow"
	err := app.render(w, http.StatusOK, "about.tmpl", templateData)
	if err != nil {
		app.serverError(w, r, err) // Use helper
	}
}

// --- Mood Handlers ---

// showMoodForm
func (app *application) showMoodForm(w http.ResponseWriter, r *http.Request) {
	templateData := NewTemplateData()
	templateData.Title = "New Mood Entry"
	templateData.HeaderText = "Log Your Mood"
	templateData.FormData = make(map[string]string) // Ensure FormData is initialized
	err := app.render(w, http.StatusOK, "mood_form.tmpl", templateData)
	if err != nil {
		app.serverError(w, r, err)
	}
}

// CreateMood
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
		templateData := app.newTemplateData()
		templateData.Title = "New Mood Entry"
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
		// DefaultEmotions already pre-filled by app.newTemplateData()

		if r.Header.Get("HX-Request") == "true" {
			app.render(w, http.StatusOK, "partials/mood_form_partial.tmpl", templateData)
		} else {
			app.render(w, http.StatusUnprocessableEntity, "mood_form.tmpl", templateData)
		}
		return
	}

	// Save the mood entry
	err = app.moods.Insert(mood)
	if err != nil {
		app.serverError(w, r, err) // Correct: Added 'r'
		return
	}

	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
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
			app.serverError(w, r, err) // Use helper
		}
		return
	}
	templateData := NewTemplateData()
	templateData.Title = fmt.Sprintf("Edit Mood Entry #%d", mood.ID)
	templateData.HeaderText = "Update Your Mood Entry"
	templateData.Mood = mood // Pass the fetched mood to the template
	templateData.FormData = map[string]string{
		"title":          mood.Title,
		"content":        mood.Content,
		"emotion":        mood.Emotion,
		"emoji":          mood.Emoji,
		"color":          mood.Color,
		"emotion_choice": mood.Emotion,
	}
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
	originalMood, err := app.moods.Get(id) // Fetch original first
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
		ID: id, Title: title, Content: content, Emotion: emotionName, Emoji: emoji, Color: color,
	}

	v := validator.NewValidator()
	data.ValidateMood(v, mood)

	if !v.ValidData() {
		// --- Determine which template ---
		templateName := "mood_edit_form.tmpl" // Only one for update

		// --- Prepare Template Data ---
		templateData := NewTemplateData()
		templateData.Title = fmt.Sprintf("Edit Mood Entry #%d (Error)", id)
		templateData.HeaderText = "Update Your Mood Entry"
		templateData.Mood = originalMood // Pass original mood
		templateData.FormErrors = v.Errors
		templateData.FormData = map[string]string{
			"title":          title,
			"content":        content,
			"emotion":        emotionName,
			"emoji":          emoji,
			"color":          color,
			"emotion_choice": r.PostForm.Get("emotion_choice"),
		}

		// --- Log Prepared Data ---
		app.logger.Warn("Preparing validation error response (update)",
			"target_template", templateName,
			"target_block", "mood-form-content", // Target block still relevant for HTMX context
			"form_errors", fmt.Sprintf("%#v", templateData.FormErrors),
			"form_data_title", templateData.FormData["title"],
			"form_data_content_len", len(templateData.FormData["content"]),
			"form_data_emotion_choice", templateData.FormData["emotion_choice"])

		// --- Template Lookup ---
		ts, ok := app.templateCache[templateName]
		if !ok {
			err := fmt.Errorf("template %q does not exist", templateName)
			app.logger.Error("Template lookup failed in error path", "template", templateName, "error", err)
			app.serverError(w, r, err)
			return
		}

		// --- Execute Template into Buffer ---
		buf := new(bytes.Buffer)
		// *** CHANGE: Execute the BASE template name (diagnostic) ***
		err = ts.ExecuteTemplate(buf, templateName, templateData)
		// *** END CHANGE ***
		if err != nil {
			app.logger.Error("Failed to execute base template into buffer", "template", templateName, "error", err)
			app.serverError(w, r, fmt.Errorf("failed to execute template %q: %w", templateName, err))
			return
		}

		// --- Log Generated HTML ---
		htmlFragment := buf.String()
		logFragment := htmlFragment
		if len(logFragment) > 500 {
			logFragment = logFragment[:500] + "...(truncated)"
		}
		app.logger.Debug("Generated HTML fragment for 422 response", "html_fragment", logFragment)
		if strings.Contains(htmlFragment, `class="error-message"`) {
			app.logger.Debug(">>> HTML fragment CONTAINS 'class=\"error-message\"")
		} else {
			app.logger.Warn(">>> HTML fragment DOES NOT contain 'class=\"error-message\"")
		}
		if strings.Contains(htmlFragment, `class="invalid"`) || strings.Contains(htmlFragment, `invalid-editor`) {
			app.logger.Debug(">>> HTML fragment CONTAINS 'invalid' class")
		} else {
			app.logger.Warn(">>> HTML fragment DOES NOT contain 'invalid' class")
		}
		// --- End Log Generated HTML ---

		// --- Write Response ---
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, writeErr := w.Write(buf.Bytes())
		if writeErr != nil {
			app.logger.Error("Failed to write template buffer to response writer", "error", writeErr)
		}
		return // Return after handling the error
	}

	// --- Success Path ---
	err = app.moods.Update(mood)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			app.notFound(w)
		} else {
			app.serverError(w, r, err) // Use helper
		}
		return
	}
	app.logger.Info("Mood entry updated successfully", "id", mood.ID)

	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", "/dashboard")
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}

// deleteMood (HTMX Enhanced - Now re-renders dashboard content)
func (app *application) deleteMood(w http.ResponseWriter, r *http.Request) {
	// --- Ensure it's a POST request ---
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		app.clientError(w, http.StatusMethodNotAllowed)
		return
	}

	// --- Get ID ---
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		app.notFound(w)
		return
	}

	// --- Attempt Deletion ---
	err = app.moods.Delete(id)
	deleteErrOccurred := false // Flag to track if a real DB error happened (not just 'not found')
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			app.logger.Warn("Attempted to delete non-existent mood entry", "id", id)
		} else {
			app.serverError(w, r, err)
			deleteErrOccurred = true
		}
	} else {
		app.logger.Info("Mood entry deleted successfully", "id", id)
	}

	// --- HTMX Response: Re-render dashboard content ---
	if r.Header.Get("HX-Request") == "true" && !deleteErrOccurred {

		// Re-fetch and re-render logic
		currentPage := 1
		searchQuery := ""
		filterCombinedEmotion := ""
		filterStartDateStr := ""
		filterEndDateStr := ""

		refererURL, parseErr := url.Parse(r.Header.Get("Referer"))
		if parseErr == nil {
			refQuery := refererURL.Query()
			searchQuery = refQuery.Get("query")
			filterCombinedEmotion = refQuery.Get("emotion")
			filterStartDateStr = refQuery.Get("start_date")
			filterEndDateStr = refQuery.Get("end_date")
			pageStr := refQuery.Get("page")
			parsedPage, convErr := strconv.Atoi(pageStr)
			if convErr == nil && parsedPage > 0 {
				currentPage = parsedPage
			}
		} else {
			app.logger.Warn("Could not parse Referer URL for delete refresh", "referer", r.Header.Get("Referer"), "error", parseErr)
		}

		var filterStartDate, filterEndDate time.Time
		var dateParseError error
		if filterStartDateStr != "" {
			filterStartDate, dateParseError = time.Parse("2006-01-02", filterStartDateStr)
			if dateParseError != nil {
				filterStartDate = time.Time{}
			}
		}
		if filterEndDateStr != "" {
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

		// Check page validity after delete
		countCriteria := data.FilterCriteria{TextQuery: searchQuery, Emotion: filterCombinedEmotion, StartDate: filterStartDate, EndDate: filterEndDate, PageSize: 4, Page: 1}
		_, tempMetadata, countErr := app.moods.GetFiltered(countCriteria)
		if countErr != nil {
			app.logger.Error("Failed to get count for page adjustment after delete", "error", countErr)
		} else {
			lastPage := tempMetadata.LastPage
			if lastPage == 0 {
				lastPage = 1
			}
			if currentPage > lastPage {
				currentPage = lastPage
			}
		}

		criteria := data.FilterCriteria{TextQuery: searchQuery, Emotion: filterCombinedEmotion, StartDate: filterStartDate, EndDate: filterEndDate, Page: currentPage, PageSize: 4}

		moods, metadata, fetchErr := app.moods.GetFiltered(criteria)
		if fetchErr != nil {
			app.logger.Error("Failed to fetch filtered moods after delete", "error", fetchErr)
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, `<p class="error-message" style="text-align:center; padding: 20px;">Error refreshing list after delete.</p>`)
			return
		}

		// --- Convert moods for display ---
		displayMoods := make([]displayMood, len(moods))
		for i, m := range moods {
			displayMoods[i] = displayMood{ // <--- Look inside this assignment
				ID:         m.ID,
				CreatedAt:  m.CreatedAt,
				UpdatedAt:  m.UpdatedAt,
				Title:      m.Title,
				Content:    template.HTML(m.Content), // This assigns the HTML-safe version
				RawContent: m.Content,                // <<< THIS LINE IS MISSING
				Emotion:    m.Emotion,
				Emoji:      m.Emoji,
				Color:      m.Color,
			}
		}

		availableEmotions, emotionErr := app.moods.GetDistinctEmotionDetails()
		if emotionErr != nil {
			app.logger.Error("Failed to fetch distinct emotions for delete refresh", "error", emotionErr)
			availableEmotions = []data.EmotionDetail{}
		}

		templateData := NewTemplateData()
		templateData.SearchQuery = searchQuery
		templateData.FilterEmotion = filterCombinedEmotion
		templateData.FilterStartDate = filterStartDateStr
		templateData.FilterEndDate = filterEndDateStr
		templateData.DisplayMoods = displayMoods
		templateData.HasMoodEntries = len(displayMoods) > 0
		templateData.AvailableEmotions = availableEmotions
		templateData.Metadata = metadata

		ts, ok := app.templateCache["dashboard.tmpl"]
		if !ok {
			err := fmt.Errorf("template %q does not exist", "dashboard.tmpl")
			app.logger.Error("Template lookup failed for delete refresh", "template", "dashboard.tmpl", "error", err)
			app.serverError(w, r, err)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		execErr := ts.ExecuteTemplate(w, "dashboard-content", templateData) // Render the dashboard content block
		if execErr != nil {
			app.logger.Error("Failed to execute template block for delete refresh", "block", "dashboard-content", "error", execErr)
		}
		return
	}

	if !deleteErrOccurred {
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
	}
}

// --- Error Helpers ---
func (app *application) serverError(w http.ResponseWriter, r *http.Request, err error) {
	var (
		method = r.Method
		uri    = r.URL.RequestURI()
	)
	app.logger.Error("server error encountered", "error", err.Error(), "method", method, "uri", uri)
	http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
}
func (app *application) clientError(w http.ResponseWriter, status int) {
	http.Error(w, http.StatusText(status), status)
}
func (app *application) notFound(w http.ResponseWriter) { app.clientError(w, http.StatusNotFound) }
