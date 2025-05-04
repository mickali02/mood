// mood/cmd/web/handlers.go
package main

import (
	"encoding/json"
	"errors"
	"fmt" // <-- Ensure fmt is imported
	"html/template"
	"net/http" // <-- Ensure net/http is imported
	"net/url"
	"strconv"
	"time"

	"github.com/mickali02/mood/internal/data"
	"github.com/mickali02/mood/internal/validator" // <-- Ensure validator is imported
)

/* ==========================================================================
   START: Dashboard Handler
   ========================================================================== */
// showDashboardPage handles requests for the main dashboard page.
// It displays a list of mood entries, potentially filtered and paginated,
// handles both full page loads and HTMX partial updates, and shows flash messages.
func (app *application) showDashboardPage(w http.ResponseWriter, r *http.Request) {
	// --- POP FLASH MESSAGE FIRST ---
	flash := app.session.PopString(r, "flash") // Get and remove flash message from session

	// --- 1. Extract Filter and Pagination Parameters ---
	v := validator.NewValidator()
	query := r.URL.Query()
	searchQuery := query.Get("query")
	filterCombinedEmotion := query.Get("emotion")
	filterStartDateStr := query.Get("start_date")
	filterEndDateStr := query.Get("end_date")
	pageStr := query.Get("page")
	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}
	v.Check(page > 0, "page", "must be a positive integer")
	v.Check(page <= 10_000_000, "page", "must be less than 10 million")

	// --- 2. Parse Date Filter Strings ---
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
			// Include the whole end day
			filterEndDate = parsedEndDate.Add(24*time.Hour - 1*time.Nanosecond)
		}
		if !filterStartDate.IsZero() && !filterEndDate.IsZero() && filterEndDate.Before(filterStartDate) {
			app.logger.Warn("End date before start date, ignoring end date", "start", filterStartDateStr, "end", filterEndDateStr)
			filterEndDate = time.Time{}
		}
	}

	// --- Re-check Validator and Finalize Page Number ---
	if !v.ValidData() {
		app.logger.Warn("Invalid page parameter", "page", pageStr, "errors", v.Errors)
		page = 1
	}

	// --- 3. Assemble Filter Criteria ---
	criteria := data.FilterCriteria{
		TextQuery: searchQuery,
		Emotion:   filterCombinedEmotion,
		StartDate: filterStartDate,
		EndDate:   filterEndDate,
		Page:      page,
		PageSize:  4, // Or your desired page size
	}

	// --- 4. Fetch Filtered Mood Data ---
	app.logger.Info("Fetching filtered moods", "criteria", fmt.Sprintf("%+v", criteria))
	moods, metadata, err := app.moods.GetFiltered(criteria)
	if err != nil {
		app.logger.Error("Failed to fetch filtered moods", "error", err)
		// Gracefully handle error, maybe show empty list + flash message
		moods = []*data.Mood{}
		metadata = data.Metadata{}
		// Optionally set an error flash message here
		// app.session.Put(r, "flash", "Error retrieving mood entries.")
		// flash = app.session.PopString(r, "flash") // Re-pop if set
	}

	// --- 5. Prepare Moods for Display ---
	displayMoods := make([]displayMood, len(moods))
	for i, m := range moods {
		displayMoods[i] = displayMood{
			ID:         m.ID,
			CreatedAt:  m.CreatedAt,
			UpdatedAt:  m.UpdatedAt,
			Title:      m.Title,
			Content:    template.HTML(m.Content), // Assume content is safe HTML (e.g., from Quill/sanitizer)
			RawContent: m.Content,                // Keep raw content for modals etc.
			Emotion:    m.Emotion,
			Emoji:      m.Emoji,
			Color:      m.Color,
		}
	}

	// --- 6. Fetch Available Emotions for Filter Dropdown ---
	availableEmotions, err := app.moods.GetDistinctEmotionDetails()
	if err != nil {
		app.logger.Error("Failed to fetch distinct emotions", "error", err)
		availableEmotions = []data.EmotionDetail{}
	}

	// --- 7. Prepare Data for the Template ---
	templateData := app.newTemplateData() // Use your helper
	templateData.Flash = flash            // Pass the flash message
	templateData.Title = "Dashboard - Feel Flow"
	templateData.SearchQuery = searchQuery
	templateData.FilterEmotion = filterCombinedEmotion
	templateData.FilterStartDate = filterStartDateStr
	templateData.FilterEndDate = filterEndDateStr
	templateData.DisplayMoods = displayMoods
	templateData.HasMoodEntries = len(displayMoods) > 0
	templateData.AvailableEmotions = availableEmotions
	templateData.Metadata = metadata
	// Add any other necessary fields like IsAuthenticated later

	// --- 8. Render Response (HTMX or Full Page) ---
	if r.Header.Get("HX-Request") == "true" {
		app.logger.Info("Handling HTMX request for dashboard content area")
		ts, ok := app.templateCache["dashboard.tmpl"]
		if !ok {
			err := fmt.Errorf("template %q does not exist", "dashboard.tmpl")
			app.logger.Error("Template lookup failed for HTMX request", "template", "dashboard.tmpl", "error", err)
			// Avoid double error response if possible
			// Check if headers already written before calling serverError
			if w.Header().Get("Content-Type") == "" {
				app.serverError(w, r, err)
			}
			return
		}
		// Render the specific block
		err = ts.ExecuteTemplate(w, "dashboard-content", templateData)
		if err != nil {
			app.logger.Error("Failed to execute template block", "block", "dashboard-content", "error", err)
			// Headers likely already sent, can't send 500 status easily
		}
	} else {
		app.logger.Info("Handling full page request for dashboard")
		// Render the full page
		err = app.render(w, http.StatusOK, "dashboard.tmpl", templateData)
		if err != nil {
			// app.render logs, app.serverError sends response if possible
			app.serverError(w, r, err)
		}
	}
}

/* ==========================================================================
   Static Pages Handlers
   ========================================================================== */

// showLandingPage handler
func (app *application) showLandingPage(w http.ResponseWriter, r *http.Request) {
	templateData := app.newTemplateData()
	_ = app.session.PopString(r, "flash") // Clear flash
	templateData.Title = "Feel Flow - Welcome"
	err := app.render(w, http.StatusOK, "landing.tmpl", templateData)
	if err != nil {
		app.serverError(w, r, err)
	}
}

// showAboutPage handler
func (app *application) showAboutPage(w http.ResponseWriter, r *http.Request) {
	templateData := app.newTemplateData()
	_ = app.session.PopString(r, "flash") // Clear flash
	templateData.Title = "About - Feel Flow"
	err := app.render(w, http.StatusOK, "about.tmpl", templateData)
	if err != nil {
		app.serverError(w, r, err)
	}
}

/* ==========================================================================
   Mood Handlers
   ========================================================================== */

// showMoodForm handler
func (app *application) showMoodForm(w http.ResponseWriter, r *http.Request) {
	templateData := app.newTemplateData()
	_ = app.session.PopString(r, "flash") // Clear flash
	templateData.Title = "New Mood Entry - Feel Flow"
	templateData.HeaderText = "Log Your Mood"
	// Ensure maps are initialized (should be handled by newTemplateData)
	// templateData.FormData = make(map[string]string)
	// templateData.FormErrors = make(map[string]string)

	err := app.render(w, http.StatusOK, "mood_form.tmpl", templateData)
	if err != nil {
		app.serverError(w, r, err)
	}
}

// createMood handler
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

	// Extract data
	title := r.PostForm.Get("title")
	content := r.PostForm.Get("content")
	emotionName := r.PostForm.Get("emotion")
	emoji := r.PostForm.Get("emoji")
	color := r.PostForm.Get("color")

	// Create mood struct
	mood := &data.Mood{
		Title:   title,
		Content: content, // Assume content needs sanitizing or comes from trusted source (like Quill)
		Emotion: emotionName,
		Emoji:   emoji,
		Color:   color,
	}

	// Validate
	v := validator.NewValidator()
	// Assuming ValidateMood exists and works correctly
	data.ValidateMood(v, mood) // Use your existing validation function

	if !v.ValidData() {
		// Prepare data for re-rendering
		templateData := app.newTemplateData()
		templateData.Title = "New Mood Entry (Error) - Feel Flow"
		templateData.HeaderText = "Log Your Mood"
		templateData.FormErrors = v.Errors
		templateData.FormData = map[string]string{
			"title":          title,
			"content":        content,
			"emotion":        emotionName,
			"emoji":          emoji,
			"color":          color,
			"emotion_choice": r.PostForm.Get("emotion_choice"), // Repopulate radio selection
		}

		// Render based on request type (HTMX or full page)
		if r.Header.Get("HX-Request") == "true" {
			// Render the partial form block for HTMX
			// Note: Ensure 'partials/mood_form_partial.tmpl' exists and defines a block
			// that can be targeted and swapped correctly. This often requires a specific
			// template structure for HTMX validation feedback.
			// For simplicity, we might re-render the full form even for HTMX here,
			// unless a dedicated partial exists. Let's assume re-rendering full for now.
			// err = app.render(w, http.StatusOK, "partials/mood_form_partial.tmpl", templateData)
			err = app.render(w, http.StatusUnprocessableEntity, "mood_form.tmpl", templateData)
		} else {
			// Render the full form for standard POST
			err = app.render(w, http.StatusUnprocessableEntity, "mood_form.tmpl", templateData)
		}
		if err != nil {
			app.serverError(w, r, err)
		}
		return
	}

	// Insert into database
	err = app.moods.Insert(mood)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	// Success: Add flash message and redirect
	app.session.Put(r, "flash", "Mood entry successfully created!")
	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}

// showEditMoodForm handler
func (app *application) showEditMoodForm(w http.ResponseWriter, r *http.Request) {
	// Extract ID
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		app.notFound(w)
		return
	}

	// Get mood entry
	mood, err := app.moods.Get(id)
	if err != nil {
		if errors.Is(err, data.ErrRecordNotFound) { // Use the error from data package
			app.notFound(w)
		} else {
			app.serverError(w, r, err)
		}
		return
	}

	// Prepare template data
	templateData := app.newTemplateData()
	_ = app.session.PopString(r, "flash") // Clear flash
	templateData.Title = fmt.Sprintf("Edit Mood #%d - Feel Flow", mood.ID)
	templateData.HeaderText = "Update Your Mood Entry"
	templateData.Mood = mood // Pass the mood object itself
	// Pre-populate FormData from the fetched mood data
	templateData.FormData = map[string]string{
		"title":          mood.Title,
		"content":        mood.Content,
		"emotion":        mood.Emotion,
		"emoji":          mood.Emoji,
		"color":          mood.Color,
		"emotion_choice": mood.Emotion, // Pre-select radio based on stored emotion
	}

	// Render edit form
	err = app.render(w, http.StatusOK, "mood_edit_form.tmpl", templateData)
	if err != nil {
		app.serverError(w, r, err)
	}
}

// updateMood handler
func (app *application) updateMood(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		app.clientError(w, http.StatusMethodNotAllowed)
		return
	}

	// Extract ID
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		app.notFound(w)
		return
	}

	// Fetch original mood (optional but good for context on error)
	// We primarily need the ID for the update query.
	// Let's skip fetching again unless needed for complex logic.

	// Parse form
	err = r.ParseForm()
	if err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	// Extract data
	title := r.PostForm.Get("title")
	content := r.PostForm.Get("content")
	emotionName := r.PostForm.Get("emotion")
	emoji := r.PostForm.Get("emoji")
	color := r.PostForm.Get("color")

	// Create mood struct with ID for update
	mood := &data.Mood{
		ID:      id,
		Title:   title,
		Content: content,
		Emotion: emotionName,
		Emoji:   emoji,
		Color:   color,
	}

	// Validate
	v := validator.NewValidator()
	data.ValidateMood(v, mood) // Use your existing validation function

	if !v.ValidData() {
		// Fetch original mood needed for re-rendering template
		originalMood, fetchErr := app.moods.Get(id)
		if fetchErr != nil {
			// Handle case where mood was deleted between GET and POST
			if errors.Is(fetchErr, data.ErrRecordNotFound) {
				app.notFound(w)
			} else {
				app.serverError(w, r, fetchErr)
			}
			return
		}

		// Prepare data for re-rendering
		templateData := app.newTemplateData()
		templateData.Title = fmt.Sprintf("Edit Mood #%d (Error) - Feel Flow", id)
		templateData.HeaderText = "Update Your Mood Entry"
		templateData.Mood = originalMood // Pass original mood for context if needed by template
		templateData.FormErrors = v.Errors
		templateData.FormData = map[string]string{
			"title":          title, // Use submitted (invalid) data for repopulation
			"content":        content,
			"emotion":        emotionName,
			"emoji":          emoji,
			"color":          color,
			"emotion_choice": r.PostForm.Get("emotion_choice"), // Repopulate radio
		}

		// Render based on request type
		templateName := "mood_edit_form.tmpl"
		// Re-rendering the full edit form is generally simpler for validation errors
		err = app.render(w, http.StatusUnprocessableEntity, templateName, templateData)
		if err != nil {
			app.serverError(w, r, err)
		}
		return
	}

	// Update in DB
	err = app.moods.Update(mood)
	if err != nil {
		if errors.Is(err, data.ErrRecordNotFound) { // Use error from data package
			app.notFound(w)
		} else {
			app.serverError(w, r, err)
		}
		return
	}

	// Success: Add flash and redirect (or handle HTMX)
	app.session.Put(r, "flash", "Mood entry successfully updated!")

	if r.Header.Get("HX-Request") == "true" {
		// For HTMX, typically redirect via header
		w.Header().Set("HX-Redirect", "/dashboard")
		w.WriteHeader(http.StatusOK) // OK status, client handles redirect
	} else {
		// Standard redirect
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther) // Redirect to dashboard
	}
}

// deleteMood handler
func (app *application) deleteMood(w http.ResponseWriter, r *http.Request) {
	// (Keep your existing deleteMood logic, it seems reasonable with HTMX handling)
	// ... (your existing deleteMood code) ...

	// Ensure it uses data.ErrRecordNotFound for consistency
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
	deleteErrOccurred := false
	flashMessage := ""

	if err != nil {
		if errors.Is(err, data.ErrRecordNotFound) { // <-- Use data package error
			app.logger.Warn("Attempted to delete non-existent mood entry", "id", id)
			flashMessage = "Mood entry not found." // Set flash for user feedback
		} else {
			app.serverError(w, r, err)
			deleteErrOccurred = true
			// flashMessage = "Error deleting mood entry." // Optional error flash
		}
	} else {
		app.logger.Info("Mood entry deleted successfully", "id", id)
		flashMessage = "Mood entry successfully deleted." // Set success flash
	}

	// Set Flash Message if appropriate
	if flashMessage != "" && !deleteErrOccurred {
		app.session.Put(r, "flash", flashMessage)
		app.logger.Info("Set flash message for delete", "message", flashMessage)
	}

	// HTMX Response: Re-render dashboard content block
	if r.Header.Get("HX-Request") == "true" && !deleteErrOccurred {

		// Pop flash for immediate display in the fragment
		currentFlash := app.session.PopString(r, "flash")
		app.logger.Info("Popped flash message for HTMX delete response", "message", currentFlash)

		// --- Re-fetch and re-render logic (copied from your version) ---
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

		// Adjust page if last item deleted
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
				app.logger.Info("Adjusting page after delete", "old_page", currentPage, "new_page", lastPage)
				currentPage = lastPage
			}
		}

		// Fetch moods for the potentially adjusted current page
		criteria := data.FilterCriteria{TextQuery: searchQuery, Emotion: filterCombinedEmotion, StartDate: filterStartDate, EndDate: filterEndDate, Page: currentPage, PageSize: 4}
		moods, metadata, fetchErr := app.moods.GetFiltered(criteria)
		if fetchErr != nil {
			app.logger.Error("Failed to fetch filtered moods after delete", "error", fetchErr)
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK) // OK status, but content indicates error
			fmt.Fprint(w, `<p class="error-message flash-message error" style="text-align:center; padding: 20px;">Error refreshing list after delete.</p>`)
			return
		}

		// Prepare data for display
		displayMoods := make([]displayMood, len(moods))
		for i, m := range moods {
			displayMoods[i] = displayMood{
				ID: m.ID, CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
				Title: m.Title, Content: template.HTML(m.Content),
				RawContent: m.Content, Emotion: m.Emotion, Emoji: m.Emoji, Color: m.Color,
			}
		}
		availableEmotions, emotionErr := app.moods.GetDistinctEmotionDetails()
		if emotionErr != nil {
			availableEmotions = []data.EmotionDetail{}
		}

		templateData := app.newTemplateData()
		templateData.Flash = currentFlash // <-- Use popped flash
		templateData.SearchQuery = searchQuery
		templateData.FilterEmotion = filterCombinedEmotion
		templateData.FilterStartDate = filterStartDateStr
		templateData.FilterEndDate = filterEndDateStr
		templateData.DisplayMoods = displayMoods
		templateData.HasMoodEntries = len(displayMoods) > 0
		templateData.AvailableEmotions = availableEmotions
		templateData.Metadata = metadata

		// Lookup and render the dashboard content block
		ts, ok := app.templateCache["dashboard.tmpl"]
		if !ok {
			err := fmt.Errorf("template %q does not exist", "dashboard.tmpl")
			app.logger.Error("Template lookup failed for delete refresh", "template", "dashboard.tmpl", "error", err)
			app.serverError(w, r, err) // Use serverError helper
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		execErr := ts.ExecuteTemplate(w, "dashboard-content", templateData)
		if execErr != nil {
			app.logger.Error("Failed to execute template block for delete refresh", "block", "dashboard-content", "error", execErr)
			// Don't call serverError here as headers might already be sent
		}
		return
	}

	// Standard Redirect for non-HTMX
	if !deleteErrOccurred {
		// Flash was already set
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
	}
	// If deleteErrOccurred, serverError handled the response
}

/* ==========================================================================
   User Authentication Handlers
   ========================================================================== */

// signupUserForm displays the user signup page.
func (app *application) signupUserForm(w http.ResponseWriter, r *http.Request) {
	// Create empty template data initially
	data := app.newTemplateData()      // Use your helper to get default data
	data.Title = "Sign Up - Feel Flow" // Set a specific title
	// Ensure maps are initialized (should be handled by newTemplateData)
	// data.FormErrors = make(map[string]string)
	// data.FormData = make(map[string]string)

	// Render the signup template
	err := app.render(w, http.StatusOK, "signup.tmpl", data)
	if err != nil {
		app.serverError(w, r, err)
	}
}

// signupUser handles the submission of the signup form.
func (app *application) signupUser(w http.ResponseWriter, r *http.Request) {
	// Parse the form data
	err := r.ParseForm()
	if err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	// Get the form values
	name := r.PostForm.Get("name")
	email := r.PostForm.Get("email")
	passwordInput := r.PostForm.Get("password") // Renamed to avoid conflict

	// Initialize a new validator instance
	v := validator.NewValidator()

	// Perform validation checks
	v.Check(validator.NotBlank(name), "name", "Name must be provided")
	v.Check(validator.MaxLength(name, 100), "name", "Name must not be more than 100 characters long")

	v.Check(validator.NotBlank(email), "email", "Email must be provided")
	v.Check(validator.MaxLength(email, 254), "email", "Email must not be more than 254 characters long")
	v.Check(validator.Matches(email, validator.EmailRX), "email", "Must be a valid email address")

	v.Check(validator.NotBlank(passwordInput), "password", "Password must be provided")
	v.Check(validator.MinLength(passwordInput, 8), "password", "Password must be at least 8 characters long")
	v.Check(validator.MaxLength(passwordInput, 72), "password", "Password must not be more than 72 characters long") // bcrypt limit

	// Check if validation failed
	if !v.ValidData() {
		data := app.newTemplateData()
		data.Title = "Sign Up (Error) - Feel Flow"
		data.FormData = map[string]string{
			"name":  name,
			"email": email,
			// DO NOT REPOPULATE PASSWORD
		}
		data.FormErrors = v.Errors
		err := app.render(w, http.StatusUnprocessableEntity, "signup.tmpl", data)
		if err != nil {
			app.serverError(w, r, err)
		}
		return // Stop processing
	}

	// --- Validation Passed ---

	// Placeholder: If validation succeeds, print message (as per video 2 end state)
	// TODO (Next Steps): Create user struct, hash password, insert user via app.users.Insert()
	fmt.Fprintln(w, "Validation successful! (Placeholder for adding user)")

}

// loginUserForm displays the user login page.
func (app *application) loginUserForm(w http.ResponseWriter, r *http.Request) {
	// TODO (Later):
	// 1. Check if user is already logged in, redirect if so.
	// 2. Create template data (flash, empty form struct).
	// 3. Render an HTML login form template (e.g., "login.tmpl").
	data := app.newTemplateData()
	data.Title = "Login - Feel Flow"
	// data.FormData = make(map[string]string) // Initialize if needed
	// data.FormErrors = make(map[string]string) // Initialize if needed

	// For now, placeholder or basic template render:
	// err := app.render(w, http.StatusOK, "login.tmpl", data) // If login.tmpl exists
	// if err != nil { app.serverError(w, r, err) }
	fmt.Fprintln(w, "Display User Login Form Placeholder") // Keep placeholder until template exists
}

// loginUser handles the submission of the login form.
func (app *application) loginUser(w http.ResponseWriter, r *http.Request) {
	// TODO (Later): Parse form, authenticate via app.users.Authenticate(), manage session
	fmt.Fprintln(w, "Process User Login Form Placeholder")
}

// logoutUser handles logging the user out.
func (app *application) logoutUser(w http.ResponseWriter, r *http.Request) {
	// TODO (Later): Destroy session data, redirect
	fmt.Fprintln(w, "Process User Logout Placeholder")
}

/* ==========================================================================
   Stats Page Handler
   ========================================================================== */

// showStatsPage handler
func (app *application) showStatsPage(w http.ResponseWriter, r *http.Request) {
	// Fetch all stats
	stats, err := app.moods.GetAllStats()
	if err != nil {
		app.logger.Error("Failed to fetch mood stats", "error", err)
		app.serverError(w, r, err)
		return
	}

	// Prepare data for Chart.js
	emotionCountsJSON, err := json.Marshal(stats.EmotionCounts)
	if err != nil {
		app.logger.Error("Failed to marshal emotion counts to JSON", "error", err)
		app.serverError(w, r, err)
		return
	}
	monthlyCountsJSON, err := json.Marshal(stats.MonthlyCounts)
	if err != nil {
		app.logger.Error("Failed to marshal monthly counts to JSON", "error", err)
		app.serverError(w, r, err)
		return
	}

	// Prepare template data
	templateData := app.newTemplateData()
	templateData.Title = "Mood Statistics - Feel Flow"
	templateData.Stats = stats
	templateData.EmotionCountsJSON = string(emotionCountsJSON)
	templateData.MonthlyCountsJSON = string(monthlyCountsJSON)
	templateData.Quote = "Every mood matters. Thanks for checking in 💖" // Example quote

	// Render the stats template
	renderErr := app.render(w, http.StatusOK, "stats.tmpl", templateData)
	if renderErr != nil {
		app.serverError(w, r, renderErr)
	}
}

/* ==========================================================================
   Error Handlers
   ========================================================================== */

// serverError logs detailed error and sends generic 500 response.
func (app *application) serverError(w http.ResponseWriter, r *http.Request, err error) {
	var (
		method = r.Method
		uri    = r.URL.RequestURI()
		// Add trace later if needed
	)
	app.logger.Error("server error encountered", "error", err.Error(), "method", method, "uri", uri)

	// Prevent writing error response if headers already sent
	if w.Header().Get("Content-Type") != "" {
		app.logger.Warn("headers already written, cannot send error response")
		return
	}
	http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
}

// clientError sends specific client error status and text.
func (app *application) clientError(w http.ResponseWriter, status int) {
	http.Error(w, http.StatusText(status), status)
}

// notFound sends 404 Not Found response.
func (app *application) notFound(w http.ResponseWriter) {
	app.clientError(w, http.StatusNotFound)
}
