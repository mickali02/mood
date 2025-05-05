// mood/cmd/web/handlers.go
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/mickali02/mood/internal/data"      // <-- Ensure internal/data is imported
	"github.com/mickali02/mood/internal/validator" // <-- Ensure internal/validator is imported
)

/* ==========================================================================
   START: Dashboard Handler
   ========================================================================== */
// (Keep your existing showDashboardPage handler as it is)
func (app *application) showDashboardPage(w http.ResponseWriter, r *http.Request) {
	// ... your existing dashboard logic ...
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
	templateData := app.newTemplateData() // <-- Use your helper
	templateData.Flash = flash            // <-- Pass the flash message
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
			// app.render logs, serverError responds if possible
			app.serverError(w, r, err)
		}
	}
}

/* ==========================================================================
   Static Pages Handlers
   ========================================================================== */
// (Keep your existing showLandingPage and showAboutPage handlers)
func (app *application) showLandingPage(w http.ResponseWriter, r *http.Request) {
	templateData := app.newTemplateData()
	_ = app.session.PopString(r, "flash")
	templateData.Title = "Feel Flow - Special Welcome"
	err := app.render(w, http.StatusOK, "landing.tmpl", templateData)
	if err != nil {
		app.serverError(w, r, err)
	}
}

func (app *application) showAboutPage(w http.ResponseWriter, r *http.Request) {
	templateData := app.newTemplateData()
	_ = app.session.PopString(r, "flash")
	templateData.Title = "About Feel Flow"
	err := app.render(w, http.StatusOK, "about.tmpl", templateData)
	if err != nil {
		app.serverError(w, r, err)
	}
}

/* ==========================================================================
   Mood Handlers
   ========================================================================== */
// (Keep your existing mood handlers: showMoodForm, createMood, showEditMoodForm, updateMood, deleteMood)
func (app *application) showMoodForm(w http.ResponseWriter, r *http.Request) {
	templateData := app.newTemplateData()
	_ = app.session.PopString(r, "flash")
	templateData.Title = "New Mood Entry"
	templateData.HeaderText = "Log Your Mood"
	templateData.FormData = make(map[string]string) // Or handled in newTemplateData
	err := app.render(w, http.StatusOK, "mood_form.tmpl", templateData)
	if err != nil {
		app.serverError(w, r, err)
	}
}

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
	data.ValidateMood(v, mood) // Assuming this function exists in data/mood.go

	if !v.ValidData() {
		templateData := app.newTemplateData()
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

		// Simplified re-render for validation error
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

	app.session.Put(r, "flash", "Mood entry successfully created!")
	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}

func (app *application) showEditMoodForm(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		app.notFound(w)
		return
	}
	mood, err := app.moods.Get(id)
	if err != nil {
		if errors.Is(err, data.ErrRecordNotFound) { // Use data error
			app.notFound(w)
		} else {
			app.serverError(w, r, err)
		}
		return
	}

	templateData := app.newTemplateData()
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
		"emotion_choice": mood.Emotion,
	}

	err = app.render(w, http.StatusOK, "mood_edit_form.tmpl", templateData)
	if err != nil {
		app.serverError(w, r, err)
	}
}

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
	data.ValidateMood(v, mood) // Assuming this function exists

	if !v.ValidData() {
		// Fetch original mood needed for context if re-rendering
		originalMood, fetchErr := app.moods.Get(id)
		if fetchErr != nil {
			if errors.Is(fetchErr, data.ErrRecordNotFound) {
				app.notFound(w)
			} else {
				app.serverError(w, r, fetchErr)
			}
			return
		}

		templateData := app.newTemplateData()
		templateData.Title = fmt.Sprintf("Edit Mood Entry #%d (Error)", id)
		templateData.HeaderText = "Update Your Mood Entry"
		templateData.Mood = originalMood // Pass original mood
		templateData.FormErrors = v.Errors
		templateData.FormData = map[string]string{
			"title":          title, // Use submitted data for repopulation
			"content":        content,
			"emotion":        emotionName,
			"emoji":          emoji,
			"color":          color,
			"emotion_choice": r.PostForm.Get("emotion_choice"),
		}

		err := app.render(w, http.StatusUnprocessableEntity, "mood_edit_form.tmpl", templateData)
		if err != nil {
			app.serverError(w, r, err)
		}
		return
	}

	err = app.moods.Update(mood)
	if err != nil {
		if errors.Is(err, data.ErrRecordNotFound) { // Use data error
			app.notFound(w)
		} else {
			app.serverError(w, r, err)
		}
		return
	}

	app.session.Put(r, "flash", "Mood entry successfully updated!")

	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", "/dashboard")
		w.WriteHeader(http.StatusOK)
	} else {
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
	}
}

func (app *application) deleteMood(w http.ResponseWriter, r *http.Request) {
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
	data := app.newTemplateData()
	data.Title = "Sign Up - Feel Flow"
	// Ensure maps are initialized here if not done in newTemplateData
	// data.FormData = make(map[string]string)
	// data.FormErrors = make(map[string]string)

	err := app.render(w, http.StatusOK, "signup.tmpl", data)
	if err != nil {
		app.serverError(w, r, err)
	}
}

// signupUser handles the submission of the signup form.
func (app *application) signupUser(w http.ResponseWriter, r *http.Request) {
	// Parse form data
	err := r.ParseForm()
	if err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	// Extract values
	name := r.PostForm.Get("name")
	email := r.PostForm.Get("email")
	passwordInput := r.PostForm.Get("password")

	// Validate input
	v := validator.NewValidator()
	v.Check(validator.NotBlank(name), "name", "Name must be provided")
	v.Check(validator.MaxLength(name, 100), "name", "Must not be more than 100 characters")

	v.Check(validator.NotBlank(email), "email", "Email must be provided")
	v.Check(validator.MaxLength(email, 254), "email", "Must not be more than 254 characters")
	v.Check(validator.Matches(email, validator.EmailRX), "email", "Must be a valid email address")

	v.Check(validator.NotBlank(passwordInput), "password", "Password must be provided")
	v.Check(validator.MinLength(passwordInput, 8), "password", "Must be at least 8 characters long")
	v.Check(validator.MaxLength(passwordInput, 72), "password", "Must not be more than 72 characters")

	if !v.ValidData() {
		// Re-render form with errors
		data := app.newTemplateData()
		data.Title = "Sign Up (Error) - Feel Flow"
		data.FormData = map[string]string{ // Repopulate form data
			"name":  name,
			"email": email,
			// Exclude password
		}
		data.FormErrors = v.Errors // Pass errors
		err := app.render(w, http.StatusUnprocessableEntity, "signup.tmpl", data)
		if err != nil {
			app.serverError(w, r, err)
		}
		return
	}

	// Create user struct
	user := &data.User{
		Name:      name,
		Email:     email,
		Activated: true, // Or true if not implementing activation step
	}

	// Hash password
	err = user.Password.Set(passwordInput)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	// Insert user into DB
	err = app.users.Insert(user) // Uses the method in internal/data/users.go
	if err != nil {
		if errors.Is(err, data.ErrDuplicateEmail) {
			v.AddError("email", "Email address is already in use") // Add specific error

			// Re-render form with this new error
			data := app.newTemplateData()
			data.Title = "Sign Up (Error) - Feel Flow"
			data.FormData = map[string]string{"name": name, "email": email}
			data.FormErrors = v.Errors
			err := app.render(w, http.StatusUnprocessableEntity, "signup.tmpl", data)
			if err != nil {
				app.serverError(w, r, err)
			}
		} else {
			// Other insert errors
			app.serverError(w, r, err)
		}
		return
	}

	// Success: Add flash message and redirect to login
	app.session.Put(r, "flash", "Your signup was successful! Please log in.")
	http.Redirect(w, r, "/user/login", http.StatusSeeOther)
}

// loginUserForm displays the user login page.
func (app *application) loginUserForm(w http.ResponseWriter, r *http.Request) {
	data := app.newTemplateData()
	data.Title = "Login - Feel Flow"
	// Get flash message from session (e.g., after signup redirect)
	data.Flash = app.session.PopString(r, "flash")
	// Initialize maps (should be handled by newTemplateData)
	// data.FormData = make(map[string]string)
	// data.FormErrors = make(map[string]string)

	// Render the login template
	err := app.render(w, http.StatusOK, "login.tmpl", data)
	if err != nil {
		app.serverError(w, r, err)
	}
}

// loginUser handles the submission of the login form.
func (app *application) loginUser(w http.ResponseWriter, r *http.Request) {
	// 1. Parse form data
	err := r.ParseForm()
	if err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	// 2. Extract email and password
	email := r.PostForm.Get("email")
	passwordInput := r.PostForm.Get("password") // Use different name from package

	// 3. Validate the input briefly (non-blank)
	// More complex validation isn't usually needed here like on signup,
	// as the Authenticate method handles the core check.
	v := validator.NewValidator()
	v.Check(validator.NotBlank(email), "generic", "Email must be provided")            // Using generic key
	v.Check(validator.NotBlank(passwordInput), "generic", "Password must be provided") // Using generic key

	// If basic validation fails, re-render with a generic message
	if !v.ValidData() {
		data := app.newTemplateData()
		data.Title = "Login (Error) - Feel Flow"
		data.FormErrors = v.Errors // Pass the errors map
		// Repopulate email, but not password
		data.FormData = map[string]string{"email": email}
		// Use the generic error message from validator for display
		data.FormErrors["generic"] = "Both email and password must be provided."

		err := app.render(w, http.StatusUnprocessableEntity, "login.tmpl", data)
		if err != nil {
			app.serverError(w, r, err)
		}
		return
	}

	// 4. Authenticate the user
	id, err := app.users.Authenticate(email, passwordInput)
	if err != nil {
		// Check if the error is specifically ErrInvalidCredentials
		if errors.Is(err, data.ErrInvalidCredentials) {
			// Authentication failed, re-render login form with error flash/message
			data := app.newTemplateData()
			data.Title = "Login (Error) - Feel Flow"
			// Repopulate email, but not password
			data.FormData = map[string]string{"email": email}
			// Add a generic error message for the form
			data.FormErrors = map[string]string{"generic": "Invalid email or password."}

			// Optionally, add a flash message too for more prominent display
			// app.session.Put(r, "flash", "Invalid email or password.")
			// data.Flash = app.session.PopString(r, "flash") // Need to pop if set

			err := app.render(w, http.StatusUnprocessableEntity, "login.tmpl", data)
			if err != nil {
				app.serverError(w, r, err)
			}
		} else {
			// Any other error is a server error
			app.serverError(w, r, err)
		}
		return // Stop processing on error
	}

	// --- Authentication Successful ---

	// 6. Store authenticated user ID in the session
	app.session.Put(r, "authenticatedUserID", id) // Use the ID returned by Authenticate

	// 7. (Optional) Add success flash message
	app.session.Put(r, "flash", "You have been logged in successfully!")

	// 8. Redirect to the dashboard
	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}

// logoutUser handles logging the user out.
func (app *application) logoutUser(w http.ResponseWriter, r *http.Request) {
	// Remove the authentication token from the session data.
	app.session.Remove(r, "authenticatedUserID") // Use the same key as in loginUser

	// Add a flash message to inform the user.
	app.session.Put(r, "flash", "You have been logged out successfully.")

	// Redirect the user to the application's landing page.
	// Redirecting to "/" often makes sense, or "/landing" if that's your main entry.
	http.Redirect(w, r, "/landing", http.StatusSeeOther) // Redirect to landing page
}

/* ==========================================================================
   Stats Page Handler
   ========================================================================== */
// (Keep your existing showStatsPage handler)
func (app *application) showStatsPage(w http.ResponseWriter, r *http.Request) {
	// ... your existing stats logic ...
	stats, err := app.moods.GetAllStats()
	if err != nil {
		app.logger.Error("Failed to fetch mood stats", "error", err)
		app.serverError(w, r, err)
		return
	}

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

	templateData := app.newTemplateData()
	templateData.Title = "Mood Statistics"
	templateData.Stats = stats
	templateData.EmotionCountsJSON = string(emotionCountsJSON)
	templateData.MonthlyCountsJSON = string(monthlyCountsJSON)
	templateData.Quote = "Every mood matters. Thanks for checking in 💖"

	renderErr := app.render(w, http.StatusOK, "stats.tmpl", templateData)
	if renderErr != nil {
		app.serverError(w, r, renderErr)
	}
}

/* ==========================================================================
   Error Handlers
   ========================================================================== */
// (Keep your existing error handlers: serverError, clientError, notFound)
func (app *application) serverError(w http.ResponseWriter, r *http.Request, err error) {
	var (
		method = r.Method
		uri    = r.URL.RequestURI()
	)
	app.logger.Error("server error encountered", "error", err.Error(), "method", method, "uri", uri)

	if w.Header().Get("Content-Type") != "" {
		app.logger.Warn("headers already written, cannot send error response")
		return
	}
	http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
}

func (app *application) clientError(w http.ResponseWriter, status int) {
	http.Error(w, http.StatusText(status), status)
}

func (app *application) notFound(w http.ResponseWriter) {
	app.clientError(w, http.StatusNotFound)
}
