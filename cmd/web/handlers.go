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
	// "strings" // Removed unused import
	"encoding/json"
	"time"

	"github.com/mickali02/mood/internal/data"
	"github.com/mickali02/mood/internal/validator"
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
		PageSize:  4,
	}

	// --- 4. Fetch Filtered Mood Data ---
	app.logger.Info("Fetching filtered moods", "criteria", fmt.Sprintf("%+v", criteria))
	moods, metadata, err := app.moods.GetFiltered(criteria)
	if err != nil {
		app.logger.Error("Failed to fetch filtered moods", "error", err)
		moods = []*data.Mood{}
		metadata = data.Metadata{}
	}

	// --- 5. Prepare Moods for Display ---
	displayMoods := make([]displayMood, len(moods))
	for i, m := range moods {
		displayMoods[i] = displayMood{
			ID:         m.ID,
			CreatedAt:  m.CreatedAt,
			UpdatedAt:  m.UpdatedAt,
			Title:      m.Title,
			Content:    template.HTML(m.Content),
			RawContent: m.Content, // Ensure RawContent is populated
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
	templateData := NewTemplateData()
	templateData.Flash = flash // <-- Pass the flash message to the template data
	templateData.Title = "Dashboard"
	templateData.SearchQuery = searchQuery
	templateData.FilterEmotion = filterCombinedEmotion
	templateData.FilterStartDate = filterStartDateStr
	templateData.FilterEndDate = filterEndDateStr
	templateData.DisplayMoods = displayMoods
	templateData.HasMoodEntries = len(displayMoods) > 0
	templateData.AvailableEmotions = availableEmotions
	templateData.Metadata = metadata

	// --- 8. Render Response (HTMX or Full Page) ---
	if r.Header.Get("HX-Request") == "true" {
		app.logger.Info("Handling HTMX request for dashboard content area")
		ts, ok := app.templateCache["dashboard.tmpl"]
		if !ok {
			err := fmt.Errorf("template %q does not exist", "dashboard.tmpl")
			app.logger.Error("Template lookup failed for HTMX request", "template", "dashboard.tmpl", "error", err)
			app.serverError(w, r, err)
			return
		}
		// Render the specific block with the data (including flash message)
		err = ts.ExecuteTemplate(w, "dashboard-content", templateData)
		if err != nil {
			app.logger.Error("Failed to execute template block", "block", "dashboard-content", "error", err)
		}
	} else {
		app.logger.Info("Handling full page request for dashboard")
		// Render the full page with the data (including flash message)
		err = app.render(w, http.StatusOK, "dashboard.tmpl", templateData)
		if err != nil {
			app.logger.Error("Full page render failed", "template", "dashboard.tmpl", "error", err)
		}
	}
}

// --- Landing Page Handler ---
func (app *application) showLandingPage(w http.ResponseWriter, r *http.Request) {
	templateData := NewTemplateData()
	// Pop any potential flash messages even on unrelated pages to clear them
	_ = app.session.PopString(r, "flash")
	templateData.Title = "Feel Flow - Special Welcome"
	err := app.render(w, http.StatusOK, "landing.tmpl", templateData)
	if err != nil {
		app.serverError(w, r, err)
	}
}

// --- About Page Handler ---
func (app *application) showAboutPage(w http.ResponseWriter, r *http.Request) {
	templateData := NewTemplateData()
	// Pop any potential flash messages even on unrelated pages to clear them
	_ = app.session.PopString(r, "flash")
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
	// Pop any potential flash messages even on unrelated pages to clear them
	_ = app.session.PopString(r, "flash")
	templateData.Title = "New Mood Entry"
	templateData.HeaderText = "Log Your Mood"
	templateData.FormData = make(map[string]string)
	err := app.render(w, http.StatusOK, "mood_form.tmpl", templateData)
	if err != nil {
		app.serverError(w, r, err)
	}
}

// CreateMood handles the form submission for creating a new mood entry
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

		if r.Header.Get("HX-Request") == "true" {
			// For HTMX validation errors, render the partial form
			// It's debatable whether to pop flash here, but probably best not to
			// as the user hasn't successfully completed an action.
			app.render(w, http.StatusOK, "partials/mood_form_partial.tmpl", templateData)
		} else {
			// For full page validation errors, render the full form
			app.render(w, http.StatusUnprocessableEntity, "mood_form.tmpl", templateData)
		}
		return
	}

	err = app.moods.Insert(mood)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	// --- ADD FLASH MESSAGE BEFORE REDIRECT ---
	app.session.Put(r, "flash", "Mood entry successfully created!")
	app.logger.Info("Set flash message for create", "message", "Mood entry successfully created!")

	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}

// showEditMoodForm handles displaying the form for editing a specific mood entry
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
	// Pop any potential flash messages even on unrelated pages to clear them
	_ = app.session.PopString(r, "flash")
	templateData.Title = fmt.Sprintf("Edit Mood Entry #%d", mood.ID)
	templateData.HeaderText = "Update Your Mood Entry"
	templateData.Mood = mood
	templateData.FormData = map[string]string{
		"title":          mood.Title,
		"content":        mood.Content,
		"emotion":        mood.Emotion,
		"emoji":          mood.Emoji,
		"color":          mood.Color,
		"emotion_choice": mood.Emotion, // Pre-select correct radio based on stored emotion
	}

	err = app.render(w, http.StatusOK, "mood_edit_form.tmpl", templateData)
	if err != nil {
		app.serverError(w, r, err)
	}
}

// updateMood handles POST requests to update a mood entry
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
		ID: id, Title: title, Content: content, Emotion: emotionName, Emoji: emoji, Color: color,
	}

	v := validator.NewValidator()
	data.ValidateMood(v, mood)

	if !v.ValidData() {
		templateName := "mood_edit_form.tmpl"

		templateData := NewTemplateData()
		templateData.Title = fmt.Sprintf("Edit Mood Entry #%d (Error)", id)
		templateData.HeaderText = "Update Your Mood Entry"
		templateData.Mood = originalMood
		templateData.FormErrors = v.Errors
		templateData.FormData = map[string]string{
			"title":          title,
			"content":        content,
			"emotion":        emotionName,
			"emoji":          emoji,
			"color":          color,
			"emotion_choice": r.PostForm.Get("emotion_choice"),
		}

		app.logger.Warn("Preparing validation error response (update)",
			"target_template", templateName,
			"form_errors", fmt.Sprintf("%#v", templateData.FormErrors))

		ts, ok := app.templateCache[templateName]
		if !ok {
			err := fmt.Errorf("template %q does not exist", templateName)
			app.logger.Error("Template lookup failed in error path", "template", templateName, "error", err)
			app.serverError(w, r, err)
			return
		}

		buf := new(bytes.Buffer)
		// IMPORTANT: Execute the *base* template name, not a block, when re-rendering the whole edit form page on validation error.
		// The template itself likely uses blocks, but we start the execution from the main template file.
		err = ts.ExecuteTemplate(buf, templateName, templateData)
		if err != nil {
			app.logger.Error("Failed to execute base template into buffer for validation error", "template", templateName, "error", err)
			app.serverError(w, r, fmt.Errorf("failed to execute template %q: %w", templateName, err))
			return
		}

		htmlFragment := buf.String()
		logFragment := htmlFragment
		if len(logFragment) > 500 {
			logFragment = logFragment[:500] + "...(truncated)"
		}
		app.logger.Debug("Generated HTML fragment for 422 response", "html_fragment", logFragment)

		// Send back the full form content with validation errors.
		// Use StatusUnprocessableEntity for validation failures.
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusUnprocessableEntity) // Indicate validation error
		_, writeErr := w.Write(buf.Bytes())
		if writeErr != nil {
			app.logger.Error("Failed to write template buffer to response writer", "error", writeErr)
		}
		return
	}

	// --- If validation passed, go ahead and update in DB ---
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

	// --- ADD FLASH MESSAGE BEFORE REDIRECT/HX-REDIRECT ---
	app.session.Put(r, "flash", "Mood entry successfully updated!")
	app.logger.Info("Set flash message for update", "message", "Mood entry successfully updated!")

	if r.Header.Get("HX-Request") == "true" {
		// For HTMX, send a header to trigger client-side redirect
		w.Header().Set("HX-Redirect", "/dashboard")
		w.WriteHeader(http.StatusOK) // OK status, redirect handled by HTMX
		return
	}
	// Otherwise, do a normal server-side redirect
	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}

// deleteMood (HTMX Enhanced - Now handles flash message before re-render)
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
	deleteErrOccurred := false
	flashMessage := "" // Store potential flash message

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			app.logger.Warn("Attempted to delete non-existent mood entry", "id", id)
			// Decide if you want a "not found" flash message
			// flashMessage = "Mood entry not found."
		} else {
			app.serverError(w, r, err) // Use helper for actual server errors
			deleteErrOccurred = true
			// Optionally set an error flash if serverError doesn't handle it fully
			// flashMessage = "Error deleting mood entry."
		}
	} else {
		app.logger.Info("Mood entry deleted successfully", "id", id)
		flashMessage = "Mood entry successfully deleted." // Set success flash
	}

	// --- Set Flash Message in Session (if generated and no critical error) ---
	if flashMessage != "" && !deleteErrOccurred {
		app.session.Put(r, "flash", flashMessage)
		app.logger.Info("Set flash message for delete", "message", flashMessage)
	}

	// --- HTMX Response: Re-render dashboard content ---
	if r.Header.Get("HX-Request") == "true" && !deleteErrOccurred {

		// --- POP FLASH MESSAGE AGAIN (before rendering fragment) ---
		// We need the flash message *now* to include it in the re-rendered fragment.
		currentFlash := app.session.PopString(r, "flash")
		app.logger.Info("Popped flash message for HTMX delete response", "message", currentFlash)

		// --- Re-fetch and re-render logic ---
		currentPage := 1
		searchQuery := ""
		filterCombinedEmotion := ""
		filterStartDateStr := ""
		filterEndDateStr := ""

		// Attempt to extract filter state from the Referer URL
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

		// --- Parse filter date range ---
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

		// Check page validity after delete (adjust if last item on page deleted)
		countCriteria := data.FilterCriteria{TextQuery: searchQuery, Emotion: filterCombinedEmotion, StartDate: filterStartDate, EndDate: filterEndDate, PageSize: 4, Page: 1} // Use Page=1 to get total count
		_, tempMetadata, countErr := app.moods.GetFiltered(countCriteria)
		if countErr != nil {
			app.logger.Error("Failed to get count for page adjustment after delete", "error", countErr)
		} else {
			lastPage := tempMetadata.LastPage
			if lastPage == 0 {
				lastPage = 1
			} // Ensure lastPage is at least 1
			if currentPage > lastPage {
				app.logger.Info("Adjusting page after delete", "old_page", currentPage, "new_page", lastPage)
				currentPage = lastPage
			}
		}

		// --- Fetch filtered moods for the (potentially adjusted) current page ---
		criteria := data.FilterCriteria{TextQuery: searchQuery, Emotion: filterCombinedEmotion, StartDate: filterStartDate, EndDate: filterEndDate, Page: currentPage, PageSize: 4}

		moods, metadata, fetchErr := app.moods.GetFiltered(criteria)
		if fetchErr != nil {
			app.logger.Error("Failed to fetch filtered moods after delete", "error", fetchErr)
			// Send back a simple error message within the target area for HTMX
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK) // Still OK status, but content indicates error
			// You might want a specific error partial template later
			fmt.Fprint(w, `<p class="error-message flash-message error" style="text-align:center; padding: 20px;">Error refreshing list after delete.</p>`)
			return
		}

		// --- Convert moods for display ---
		displayMoods := make([]displayMood, len(moods))
		for i, m := range moods {
			displayMoods[i] = displayMood{
				ID: m.ID, CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
				Title: m.Title, Content: template.HTML(m.Content),
				RawContent: m.Content, // Ensure RawContent is populated
				Emotion:    m.Emotion, Emoji: m.Emoji, Color: m.Color,
			}
		}
		// --- Load list of all distinct emotions for filter options ---
		availableEmotions, emotionErr := app.moods.GetDistinctEmotionDetails()
		if emotionErr != nil {
			app.logger.Error("Failed to fetch distinct emotions for delete refresh", "error", emotionErr)
			availableEmotions = []data.EmotionDetail{}
		}
		// --- Populate template data INCLUDING the FLASH message ---
		templateData := NewTemplateData()
		templateData.Flash = currentFlash // <-- Pass the popped flash message
		templateData.SearchQuery = searchQuery
		templateData.FilterEmotion = filterCombinedEmotion
		templateData.FilterStartDate = filterStartDateStr
		templateData.FilterEndDate = filterEndDateStr
		templateData.DisplayMoods = displayMoods
		templateData.HasMoodEntries = len(displayMoods) > 0
		templateData.AvailableEmotions = availableEmotions
		templateData.Metadata = metadata

		// --- Lookup and render the dashboard content block only ---
		ts, ok := app.templateCache["dashboard.tmpl"]
		if !ok {
			err := fmt.Errorf("template %q does not exist", "dashboard.tmpl")
			app.logger.Error("Template lookup failed for delete refresh", "template", "dashboard.tmpl", "error", err)
			app.serverError(w, r, err)
			return
		}
		// --- Render only the 'dashboard-content' block for HTMX ---
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		execErr := ts.ExecuteTemplate(w, "dashboard-content", templateData)
		if execErr != nil {
			app.logger.Error("Failed to execute template block for delete refresh", "block", "dashboard-content", "error", execErr)
			// Don't call serverError here as headers might already be sent
		}
		return
	}

	// --- Redirect for non-HTMX (full page load) ---
	if !deleteErrOccurred {
		// The flash message was already put into the session earlier
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
	}
	// If deleteErrOccurred, the serverError handler was already called and handled the response.
}

// === NEW: User Authentication Handlers (Placeholders) ===

// signupUserForm displays the user signup page.
func (app *application) signupUserForm(w http.ResponseWriter, r *http.Request) {
	// Create empty template data initially
	data := app.newTemplateData()             // Use your helper to get default data
	data.Title = "Sign Up - Feel Flow"        // Set a specific title
	data.FormErrors = make(map[string]string) // Ensure maps are initialized
	data.FormData = make(map[string]string)

	// Render the signup template
	// Use "signup.tmpl" which is the base name of the template file
	err := app.render(w, http.StatusOK, "signup.tmpl", data)
	if err != nil {
		// The render helper should already log the error
		app.serverError(w, r, err) // Send generic server error response
	}
}

// signupUser handles the submission of the signup form.
func (app *application) signupUser(w http.ResponseWriter, r *http.Request) {
	// TODO (Later):
	// 1. Parse form data (name, email, password).
	// 2. Validate the data (non-blank fields, valid email, password length).
	// 3. If validation fails, re-render signup form with errors.
	// 4. Hash the password securely using bcrypt (via password.Set method).
	// 5. Create a data.User struct.
	// 6. Insert the user into the database using app.users.Insert().
	// 7. Handle potential errors (like duplicate email).
	// 8. Add a flash message ("Signup successful!").
	// 9. Redirect the user (e.g., to the login page or dashboard).

	// Placeholder:
	fmt.Fprintln(w, "Process User Signup Form Placeholder")
}

// loginUserForm displays the user login page.
func (app *application) loginUserForm(w http.ResponseWriter, r *http.Request) {
	// TODO (Later):
	// 1. Check if user is already logged in, redirect if so.
	// 2. Create template data (e.g., flash message, empty form struct).
	// 3. Render an HTML login form template (e.g., "login.tmpl").

	// Placeholder:
	fmt.Fprintln(w, "Display User Login Form Placeholder")
}

// loginUser handles the submission of the login form.
func (app *application) loginUser(w http.ResponseWriter, r *http.Request) {
	// TODO (Later):
	// 1. Parse form data (email, password).
	// 2. Authenticate the user using app.users.Authenticate(email, password).
	// 3. Handle authentication errors (ErrInvalidCredentials, ErrRecordNotFound).
	//    - If error, re-render login form with flash message.
	// 4. If authentication successful:
	//    - Regenerate the session ID (security best practice).
	//    - Store the user ID in the session (e.g., app.session.Put(r, "authenticatedUserID", userID)).
	//    - Add a flash message ("Login successful!").
	//    - Redirect the user to the dashboard ("/dashboard").

	// Placeholder:
	fmt.Fprintln(w, "Process User Login Form Placeholder")
}

// logoutUser handles logging the user out.
func (app *application) logoutUser(w http.ResponseWriter, r *http.Request) {
	// TODO (Later):
	// 1. Remove the authenticatedUserID key from the session (app.session.Remove(r, "authenticatedUserID")).
	// 2. Add a flash message ("You have been logged out.").
	// 3. Redirect the user to the landing or login page.
	// Optional: Destroy the session completely if needed (app.session.Destroy(r))

	// Placeholder:
	fmt.Fprintln(w, "Process User Logout Placeholder")
}

// --- Error Helpers ---
// serverError logs an internal server error and sends a 500 response.
func (app *application) serverError(w http.ResponseWriter, r *http.Request, err error) {
	var (
		method = r.Method
		uri    = r.URL.RequestURI()
	)
	app.logger.Error("server error encountered", "error", err.Error(), "method", method, "uri", uri)
	// Removed the incorrect Hijacked check. If headers are already sent, this might cause
	// a "superfluous response.WriteHeader call" log, but it's generally safe for 500.
	http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
}

// clientError sends an HTTP response with the given client-side error status.
func (app *application) clientError(w http.ResponseWriter, status int) {
	http.Error(w, http.StatusText(status), status)
}

// notFound sends a 404 Not Found client error.
func (app *application) notFound(w http.ResponseWriter) {
	app.clientError(w, http.StatusNotFound)
}

// showStatsPage handles requests for the mood statistics page.
func (app *application) showStatsPage(w http.ResponseWriter, r *http.Request) {
	// Fetch all stats using the new aggregate function
	stats, err := app.moods.GetAllStats()
	if err != nil {
		app.logger.Error("Failed to fetch mood stats", "error", err)
		// Optionally, render the page with an error message or just serverError
		app.serverError(w, r, err)
		return
	}

	// Prepare data for Chart.js (Convert slices to JSON strings)
	// Chart.js can easily consume JSON data.
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
	templateData := NewTemplateData() // Use your existing helper
	templateData.Title = "Mood Statistics"
	templateData.Stats = stats                                 // Pass the whole stats struct
	templateData.EmotionCountsJSON = string(emotionCountsJSON) // Pass JSON strings
	templateData.MonthlyCountsJSON = string(monthlyCountsJSON)
	// Add a sample quote (you could make this dynamic later)
	templateData.Quote = "Every mood matters. Thanks for checking in 💖"

	// Render the new stats template
	renderErr := app.render(w, http.StatusOK, "stats.tmpl", templateData)
	if renderErr != nil {
		// app.render already logs errors, but serverError handles the response
		app.serverError(w, r, renderErr)
	}
}
