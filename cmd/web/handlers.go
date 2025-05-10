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

	"github.com/mickali02/mood/internal/data"
	"github.com/mickali02/mood/internal/validator"
)

// Helper function to get user ID from session
func (app *application) getUserIDFromSession(r *http.Request) int64 {
	if !app.session.Exists(r, "authenticatedUserID") {
		return 0
	}
	userID, ok := app.session.Get(r, "authenticatedUserID").(int64)
	if !ok {
		app.logger.Error("authenticatedUserID in session is not int64")
		return 0
	}
	return userID
}

/*
==========================================================================

	START: Dashboard Handler
	==========================================================================
*/
func (app *application) showDashboardPage(w http.ResponseWriter, r *http.Request) {
	userID := app.getUserIDFromSession(r)
	if userID == 0 {
		app.clientError(w, http.StatusUnauthorized)
		return
	}

	// --- FETCH USER DETAILS ---
	user, err := app.users.Get(userID)
	if err != nil {
		app.logger.Error("Failed to get user details for dashboard", "userID", userID, "error", err)
		user = &data.User{}
	}
	// --- END FETCH USER DETAILS ---

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

	var filterStartDate, filterEndDate time.Time
	if filterStartDateStr != "" {
		var parseErr error
		filterStartDate, parseErr = time.Parse("2006-01-02", filterStartDateStr)
		if parseErr != nil {
			app.logger.Warn("Invalid start date format", "date", filterStartDateStr, "error", parseErr)
			filterStartDate = time.Time{}
		}
	}
	if filterEndDateStr != "" {
		var parseErr error
		parsedEndDate, parseErr := time.Parse("2006-01-02", filterEndDateStr)
		if parseErr != nil {
			app.logger.Warn("Invalid end date format", "date", filterEndDateStr, "error", parseErr)
			filterEndDate = time.Time{}
		} else {
			filterEndDate = parsedEndDate.Add(24*time.Hour - 1*time.Nanosecond)
		}
		if !filterStartDate.IsZero() && !filterEndDate.IsZero() && filterEndDate.Before(filterStartDate) {
			app.logger.Warn("End date before start date, ignoring end date", "start", filterStartDateStr, "end", filterEndDateStr)
			filterEndDate = time.Time{}
		}
	}

	if !v.ValidData() {
		app.logger.Warn("Invalid page parameter", "page", pageStr, "errors", v.Errors)
		page = 1 // Default to page 1 on validation error
	}

	criteria := data.FilterCriteria{
		TextQuery: searchQuery, Emotion: filterCombinedEmotion,
		StartDate: filterStartDate, EndDate: filterEndDate,
		Page: page, PageSize: 4, UserID: userID,
	}

	app.logger.Info("Fetching filtered moods", "criteria", fmt.Sprintf("%+v", criteria))
	moods, metadata, err := app.moods.GetFiltered(criteria)
	if err != nil {
		// Revert to checking the error string, as returned by the data model
		if err.Error() == "invalid user ID provided for filtering moods" {
			app.logger.Error("Invalid UserID passed to GetFiltered", "userID", userID)
			app.serverError(w, r, errors.New("internal inconsistency: invalid user session"))
			return
		}
		// Handle other potential errors from GetFiltered
		app.logger.Error("Failed to fetch filtered moods", "error", err)
		// Default to empty slice and metadata on other errors
		moods = []*data.Mood{}
		metadata = data.Metadata{}
	}

	displayMoods := make([]displayMood, len(moods))
	for i, moodEntry := range moods {
		displayMoods[i] = displayMood{
			ID: moodEntry.ID, CreatedAt: moodEntry.CreatedAt, UpdatedAt: moodEntry.UpdatedAt,
			Title: moodEntry.Title, Content: template.HTML(moodEntry.Content), RawContent: moodEntry.Content,
			Emotion: moodEntry.Emotion, Emoji: moodEntry.Emoji, Color: moodEntry.Color,
		}
	}

	availableEmotions, err := app.moods.GetDistinctEmotionDetails(userID)
	if err != nil {
		app.logger.Error("Failed to fetch distinct emotions", "error", err, "userID", userID)
		availableEmotions = []data.EmotionDetail{} // Default to empty slice
	}

	templateData := app.newTemplateData(r)
	templateData.Title = "Dashboard"
	templateData.SearchQuery = searchQuery
	templateData.FilterEmotion = filterCombinedEmotion
	templateData.FilterStartDate = filterStartDateStr
	templateData.FilterEndDate = filterEndDateStr
	templateData.DisplayMoods = displayMoods
	templateData.HasMoodEntries = len(displayMoods) > 0
	templateData.AvailableEmotions = availableEmotions
	templateData.Metadata = metadata
	templateData.UserName = user.Name // User name from fetched user

	// Handle HTMX request or full page load
	if r.Header.Get("HX-Request") == "true" {
		app.logger.Info("Handling HTMX request for dashboard content area")
		ts, ok := app.templateCache["dashboard.tmpl"]
		if !ok {
			err := fmt.Errorf("template %q does not exist", "dashboard.tmpl")
			app.logger.Error("Template lookup failed", "template", "dashboard.tmpl", "error", err)
			// Send an HTMX error response if possible, or fallback
			http.Error(w, "Error loading dashboard content.", http.StatusInternalServerError)
			return
		}
		// Execute only the relevant block for HTMX swap
		err = ts.ExecuteTemplate(w, "dashboard-content", templateData)
		if err != nil {
			app.logger.Error("Failed to execute template block", "block", "dashboard-content", "error", err)
			// Don't write further if header already sent
		}
	} else {
		app.logger.Info("Handling full page request for dashboard")
		err = app.render(w, http.StatusOK, "dashboard.tmpl", templateData)
		if err != nil {
			// Render handles logging and error response
		}
	}
}

/*
==========================================================================

	Static Pages Handlers
	==========================================================================
*/
func (app *application) showLandingPage(w http.ResponseWriter, r *http.Request) {
	templateData := app.newTemplateData(r)
	templateData.Title = "Feel Flow - Special Welcome"
	err := app.render(w, http.StatusOK, "landing.tmpl", templateData)
	if err != nil {
		app.serverError(w, r, err)
	}
}

func (app *application) showAboutPage(w http.ResponseWriter, r *http.Request) {
	templateData := app.newTemplateData(r)
	templateData.Title = "About Feel Flow"
	err := app.render(w, http.StatusOK, "about.tmpl", templateData)
	if err != nil {
		app.serverError(w, r, err)
	}
}

/*
==========================================================================

	Mood Handlers
	==========================================================================
*/
func (app *application) showMoodForm(w http.ResponseWriter, r *http.Request) {
	templateData := app.newTemplateData(r)
	templateData.Title = "New Mood Entry"
	templateData.HeaderText = "Log Your Mood"
	err := app.render(w, http.StatusOK, "mood_form.tmpl", templateData)
	if err != nil {
		app.serverError(w, r, err)
	}
}

func (app *application) createMood(w http.ResponseWriter, r *http.Request) {
	userID := app.getUserIDFromSession(r)
	if userID == 0 {
		app.clientError(w, http.StatusUnauthorized)
		return
	}

	// Basic method check (can be middleware too)
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

	// Extract form data
	title := r.PostForm.Get("title")
	content := r.PostForm.Get("content")
	emotionName := r.PostForm.Get("emotion")
	emoji := r.PostForm.Get("emoji")
	color := r.PostForm.Get("color")
	emotionChoice := r.PostForm.Get("emotion_choice") // Keep track of radio button selection

	mood := &data.Mood{
		Title:   title,
		Content: content,
		Emotion: emotionName,
		Emoji:   emoji,
		Color:   color,
		UserID:  userID,
	}

	v := validator.NewValidator()
	data.ValidateMood(v, mood)

	if !v.ValidData() {
		templateData := app.newTemplateData(r)
		templateData.Title = "New Mood Entry (Error)"
		templateData.HeaderText = "Log Your Mood"
		templateData.FormErrors = v.Errors
		// Repopulate form data for user convenience
		templateData.FormData = map[string]string{
			"title":          title,
			"content":        content,
			"emotion":        emotionName,
			"emoji":          emoji,
			"color":          color,
			"emotion_choice": emotionChoice, // Repopulate selected radio
		}
		errRender := app.render(w, http.StatusUnprocessableEntity, "mood_form.tmpl", templateData)
		if errRender != nil {
			app.serverError(w, r, errRender)
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
	userID := app.getUserIDFromSession(r)
	if userID == 0 {
		app.clientError(w, http.StatusUnauthorized)
		return
	}

	mood, err := app.moods.Get(id, userID)
	if err != nil {
		if errors.Is(err, data.ErrRecordNotFound) {
			app.notFound(w)
		} else {
			app.serverError(w, r, err)
		}
		return
	}

	templateData := app.newTemplateData(r)
	templateData.Title = fmt.Sprintf("Edit Mood Entry #%d", mood.ID)
	templateData.HeaderText = "Update Your Mood Entry"
	templateData.Mood = mood // Pass existing mood data
	// Populate FormData with existing mood data for the form fields
	templateData.FormData = map[string]string{
		"title":          mood.Title,
		"content":        mood.Content,
		"emotion":        mood.Emotion,
		"emoji":          mood.Emoji,
		"color":          mood.Color,
		"emotion_choice": mood.Emotion, // Pre-select the correct radio button
	}

	err = app.render(w, http.StatusOK, "mood_edit_form.tmpl", templateData)
	if err != nil {
		app.serverError(w, r, err)
	}
}

func (app *application) updateMood(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		app.notFound(w)
		return
	}
	userID := app.getUserIDFromSession(r)
	if userID == 0 {
		app.clientError(w, http.StatusUnauthorized)
		return
	}

	// Get original mood to check ownership and potentially display on error
	originalMoodForCheck, err := app.moods.Get(id, userID)
	if err != nil {
		if errors.Is(err, data.ErrRecordNotFound) {
			app.notFound(w)
		} else {
			app.serverError(w, r, err)
		}
		return
	}

	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		app.clientError(w, http.StatusMethodNotAllowed)
		return
	}
	err = r.ParseForm()
	if err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	// Extract form data
	title := r.PostForm.Get("title")
	content := r.PostForm.Get("content")
	emotionName := r.PostForm.Get("emotion")
	emoji := r.PostForm.Get("emoji")
	color := r.PostForm.Get("color")
	emotionChoice := r.PostForm.Get("emotion_choice")

	// Populate mood struct with updated values
	mood := &data.Mood{
		ID:      id, // Set the ID for update
		Title:   title,
		Content: content,
		Emotion: emotionName,
		Emoji:   emoji,
		Color:   color,
		UserID:  userID, // Include UserID for ownership check in model
	}

	v := validator.NewValidator()
	data.ValidateMood(v, mood)

	if !v.ValidData() {
		templateData := app.newTemplateData(r)
		templateData.Title = fmt.Sprintf("Edit Mood Entry #%d (Error)", id)
		templateData.HeaderText = "Update Your Mood Entry"
		templateData.Mood = originalMoodForCheck // Pass original mood for context
		templateData.FormErrors = v.Errors
		// Repopulate form with submitted (invalid) data
		templateData.FormData = map[string]string{
			"title":          title,
			"content":        content,
			"emotion":        emotionName,
			"emoji":          emoji,
			"color":          color,
			"emotion_choice": emotionChoice,
		}
		errRender := app.render(w, http.StatusUnprocessableEntity, "mood_edit_form.tmpl", templateData)
		if errRender != nil {
			app.serverError(w, r, errRender)
		}
		return
	}

	// Perform the update
	err = app.moods.Update(mood)
	if err != nil {
		if errors.Is(err, data.ErrRecordNotFound) { // Handle case where mood was deleted between GET and POST
			app.notFound(w)
		} else {
			app.serverError(w, r, err)
		}
		return
	}

	app.session.Put(r, "flash", "Mood entry successfully updated!")

	// Handle HTMX redirect or standard redirect
	if r.Header.Get("HX-Request") == "true" {
		// Tell HTMX to redirect the browser after successful swap/update
		w.Header().Set("HX-Redirect", "/dashboard")
		w.WriteHeader(http.StatusOK) // OK status needed for HX-Redirect
	} else {
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
	}
}

func (app *application) deleteMood(w http.ResponseWriter, r *http.Request) {
	// Basic method check
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
	userID := app.getUserIDFromSession(r)
	if userID == 0 {
		app.clientError(w, http.StatusUnauthorized)
		return
	}

	err = app.moods.Delete(id, userID) // Model handles ownership check

	deleteErrOccurred := false
	flashMessage := ""

	if err != nil {
		if errors.Is(err, data.ErrRecordNotFound) {
			// Log warning but don't flash user for trying to delete non-existent/unauthorized
			app.logger.Warn("Attempted delete non-existent/unauthorized mood", "id", id, "userID", userID)
			// For HTMX, maybe return a specific status or empty content?
			// For non-HTMX, just redirect without flash.
			flashMessage = ""
		} else {
			// Real error during delete
			app.serverError(w, r, err)
			deleteErrOccurred = true // Prevent redirect/HTMX success response
		}
	} else {
		// Successful delete
		app.logger.Info("Mood entry deleted successfully", "id", id, "userID", userID)
		flashMessage = "Mood entry successfully deleted."
	}

	// Set flash only on actual success
	if flashMessage != "" {
		app.session.Put(r, "flash", flashMessage)
		app.logger.Info("Set flash message for delete success", "message", flashMessage)
	}

	// Handle HTMX response for successful delete
	if r.Header.Get("HX-Request") == "true" && !deleteErrOccurred {
		currentFlash := app.session.PopString(r, "flash") // Get the flash message for HTMX response
		app.logger.Info("Popped flash message for HTMX delete response", "message", currentFlash)

		// --- Logic to re-render dashboard content after delete ---
		// Determine the correct page to show after deletion (handle deleting last item on a page)
		currentPage := 1 // Default
		searchQuery := ""
		filterCombinedEmotion := ""
		filterStartDateStr := ""
		filterEndDateStr := ""

		// Parse Referer URL to maintain filters/page
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

		// Parse dates from referer strings
		var filterStartDate, filterEndDate time.Time
		if filterStartDateStr != "" { /* ... date parsing logic ... */
			var parseErrStart error
			filterStartDate, parseErrStart = time.Parse("2006-01-02", filterStartDateStr)
			if parseErrStart != nil {
				filterStartDate = time.Time{}
			}
		}
		if filterEndDateStr != "" { /* ... date parsing logic ... */
			var parseErrEnd error
			parsedEndDate, parseErrEnd := time.Parse("2006-01-02", filterEndDateStr)
			if parseErrEnd == nil {
				filterEndDate = parsedEndDate.Add(24*time.Hour - 1*time.Nanosecond)
			} else {
				filterEndDate = time.Time{}
			}
			if !filterStartDate.IsZero() && !filterEndDate.IsZero() && filterEndDate.Before(filterStartDate) {
				filterEndDate = time.Time{}
			}
		}

		// Check current total count with same filters to adjust page number if needed
		countCriteria := data.FilterCriteria{
			TextQuery: searchQuery, Emotion: filterCombinedEmotion,
			StartDate: filterStartDate, EndDate: filterEndDate,
			PageSize: 4, Page: 1, UserID: userID, // PageSize matters, Page 1 to get total
		}
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
				currentPage = lastPage // Go to the new last page
			}
		}

		// Fetch moods for the potentially adjusted current page
		criteria := data.FilterCriteria{
			TextQuery: searchQuery, Emotion: filterCombinedEmotion,
			StartDate: filterStartDate, EndDate: filterEndDate,
			Page: currentPage, PageSize: 4, UserID: userID,
		}
		moods, metadata, fetchErr := app.moods.GetFiltered(criteria)
		if fetchErr != nil {
			app.logger.Error("Failed to fetch filtered moods after delete", "error", fetchErr)
			// Send HTMX error response or fallback
			http.Error(w, "Error reloading dashboard content.", http.StatusInternalServerError)
			return
		}

		// Prepare data for re-rendering the dashboard fragment
		displayMoods := make([]displayMood, len(moods))
		for i, moodEntry := range moods {
			displayMoods[i] = displayMood{ /* ... populate displayMood ... */
				ID: moodEntry.ID, CreatedAt: moodEntry.CreatedAt, UpdatedAt: moodEntry.UpdatedAt,
				Title: moodEntry.Title, Content: template.HTML(moodEntry.Content), RawContent: moodEntry.Content,
				Emotion: moodEntry.Emotion, Emoji: moodEntry.Emoji, Color: moodEntry.Color,
			}
		}
		availableEmotions, emotionErr := app.moods.GetDistinctEmotionDetails(userID)
		if emotionErr != nil {
			availableEmotions = []data.EmotionDetail{}
		}

		templateData := app.newTemplateData(r)
		templateData.Flash = currentFlash // Pass the popped flash message
		templateData.SearchQuery = searchQuery
		templateData.FilterEmotion = filterCombinedEmotion
		templateData.FilterStartDate = filterStartDateStr
		templateData.FilterEndDate = filterEndDateStr
		templateData.DisplayMoods = displayMoods
		templateData.HasMoodEntries = len(displayMoods) > 0
		templateData.AvailableEmotions = availableEmotions
		templateData.Metadata = metadata
		// Don't need to fetch User again here, newTemplateData handles it if authenticated

		// Render just the dashboard content block for HTMX swap
		ts, ok := app.templateCache["dashboard.tmpl"]
		if !ok {
			err := fmt.Errorf("template %q does not exist", "dashboard.tmpl")
			app.logger.Error("Template lookup failed", "template", "dashboard.tmpl", "error", err)
			http.Error(w, "Error loading dashboard content.", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK) // Important: OK status for successful HTMX swap
		execErr := ts.ExecuteTemplate(w, "dashboard-content", templateData)
		if execErr != nil {
			app.logger.Error("Failed to execute template block for delete refresh", "block", "dashboard-content", "error", execErr)
		}
		return // Stop execution after HTMX response
	}

	// Standard redirect for non-HTMX requests if no error occurred
	if !deleteErrOccurred {
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
	}
	// If deleteErrOccurred, the serverError handler already wrote the response.
}

/*
==========================================================================

	User Authentication Handlers
	==========================================================================
*/
func (app *application) signupUserForm(w http.ResponseWriter, r *http.Request) {
	templateData := app.newTemplateData(r)
	templateData.Title = "Sign Up - Feel Flow"
	err := app.render(w, http.StatusOK, "signup.tmpl", templateData)
	if err != nil {
		app.serverError(w, r, err)
	}
}

func (app *application) signupUser(w http.ResponseWriter, r *http.Request) {
	// Check method
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

	// Extract and validate form data
	name := r.PostForm.Get("name")
	email := r.PostForm.Get("email")
	passwordInput := r.PostForm.Get("password")

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
		templateData := app.newTemplateData(r)
		templateData.Title = "Sign Up (Error) - Feel Flow"
		templateData.FormData = map[string]string{"name": name, "email": email} // Don't repopulate password
		templateData.FormErrors = v.Errors
		errRender := app.render(w, http.StatusUnprocessableEntity, "signup.tmpl", templateData)
		if errRender != nil {
			app.serverError(w, r, errRender)
		}
		return
	}

	// Create user struct and set password
	user := &data.User{Name: name, Email: email, Activated: true} // Activate immediately for simplicity
	err = user.Password.Set(passwordInput)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	// Insert user into database
	err = app.users.Insert(user)
	if err != nil {
		if errors.Is(err, data.ErrDuplicateEmail) {
			v.AddError("email", "Email address is already in use")
			templateData := app.newTemplateData(r)
			templateData.Title = "Sign Up (Error) - Feel Flow"
			templateData.FormData = map[string]string{"name": name, "email": email}
			templateData.FormErrors = v.Errors
			errRender := app.render(w, http.StatusUnprocessableEntity, "signup.tmpl", templateData)
			if errRender != nil {
				app.serverError(w, r, errRender)
			}
		} else {
			app.serverError(w, r, err)
		}
		return
	}

	// Success
	app.session.Put(r, "flash", "Your signup was successful! Please log in.")
	http.Redirect(w, r, "/user/login", http.StatusSeeOther)
}

func (app *application) loginUserForm(w http.ResponseWriter, r *http.Request) {
	templateData := app.newTemplateData(r)
	templateData.Title = "Login - Feel Flow"
	err := app.render(w, http.StatusOK, "login.tmpl", templateData)
	if err != nil {
		app.serverError(w, r, err)
	}
}

func (app *application) loginUser(w http.ResponseWriter, r *http.Request) {
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

	email := r.PostForm.Get("email")
	passwordInput := r.PostForm.Get("password")

	// Basic validation for presence
	v := validator.NewValidator()
	v.Check(validator.NotBlank(email), "generic", "Email must be provided")            // Using generic key
	v.Check(validator.NotBlank(passwordInput), "generic", "Password must be provided") // Using generic key

	// Use a generic error message on login failure
	genericError := func() {
		templateData := app.newTemplateData(r)
		templateData.Title = "Login (Error) - Feel Flow"
		templateData.FormData = map[string]string{"email": email} // Repopulate email
		templateData.FormErrors = map[string]string{"generic": "Invalid email or password."}
		errRender := app.render(w, http.StatusUnprocessableEntity, "login.tmpl", templateData)
		if errRender != nil {
			app.serverError(w, r, errRender)
		}
	}

	if !v.ValidData() {
		genericError()
		return
	}

	// Authenticate user
	id, err := app.users.Authenticate(email, passwordInput)
	if err != nil {
		if errors.Is(err, data.ErrInvalidCredentials) {
			genericError()
		} else { // Handle other potential errors (e.g., database connection)
			app.serverError(w, r, err)
		}
		return
	}

	// Authentication successful
	app.session.Put(r, "authenticatedUserID", id) // Store user ID in session
	app.session.Put(r, "flash", "You have been logged in successfully!")
	http.Redirect(w, r, "/dashboard", http.StatusSeeOther) // Redirect to dashboard
}

func (app *application) logoutUser(w http.ResponseWriter, r *http.Request) {
	// Check method
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		app.clientError(w, http.StatusMethodNotAllowed)
		return
	}

	// Remove user ID from session
	app.session.Remove(r, "authenticatedUserID")
	app.session.Put(r, "flash", "You have been logged out successfully.")
	http.Redirect(w, r, "/landing", http.StatusSeeOther) // Redirect to landing page
}

/*
==========================================================================

	Stats Page Handler
	==========================================================================
*/
func (app *application) showStatsPage(w http.ResponseWriter, r *http.Request) {
	userID := app.getUserIDFromSession(r)
	if userID == 0 {
		app.clientError(w, http.StatusUnauthorized)
		return
	}

	stats, err := app.moods.GetAllStats(userID)
	if err != nil {
		app.logger.Error("Failed to fetch mood stats", "error", err, "userID", userID)
		app.serverError(w, r, err)
		return
	}

	// *** ADDED: Defensive check in case GetAllStats could return nil without error ***
	if stats == nil {
		// This case should ideally not happen if GetAllStats is implemented correctly,
		// but good to handle defensively.
		app.logger.Error("GetAllStats returned nil stats object unexpectedly", "userID", userID)
		// Render with empty stats or return an error
		stats = &data.MoodStats{} // Proceed with empty stats for the template
		// Alternatively, treat as server error:
		// app.serverError(w, r, errors.New("failed to retrieve stats data"))
		// return
	}

	// *** ADDED: Detailed logging BEFORE marshalling ***
	app.logger.Info("Preparing stats data for template",
		"userID", userID,
		"totalEntries", stats.TotalEntries,
		"hasMostCommon", stats.MostCommonEmotion != nil,
		"emotionCountsLength", len(stats.EmotionCounts),
		"weeklyCountsLength", len(stats.WeeklyCounts),
		"hasLatestMood", stats.LatestMood != nil,
		"avgEntries", stats.AvgEntriesPerWeek,
	)
	// *** END ADDED LOGGING ***

	emotionCountsJSON, err := json.Marshal(stats.EmotionCounts)
	if err != nil {
		app.serverError(w, r, fmt.Errorf("marshal emotion counts: %w", err))
		return
	}

	templateData := app.newTemplateData(r)
	templateData.Title = "Mood Statistics"
	templateData.Stats = stats
	templateData.EmotionCountsJSON = string(emotionCountsJSON)
	templateData.Quote = "Every mood matters. Thanks for checking in 💖"

	// *** ADDED: Log final JSON being sent ***
	app.logger.Debug("Final JSON for template", "emotions", templateData.EmotionCountsJSON)
	// *** END ADDED LOGGING ***

	renderErr := app.render(w, http.StatusOK, "stats.tmpl", templateData)
	if renderErr != nil {
		app.serverError(w, r, renderErr)
	}
}

/*
==========================================================================
	User Profile Handlers
==========================================================================
*/

func (app *application) showUserProfilePage(w http.ResponseWriter, r *http.Request) {
	userID := app.getUserIDFromSession(r)
	if userID == 0 {
		app.clientError(w, http.StatusUnauthorized)
		return
	}

	user, err := app.users.Get(userID)
	if err != nil {
		if errors.Is(err, data.ErrRecordNotFound) {
			app.notFound(w)
		} else {
			app.serverError(w, r, err)
		}
		return
	}

	// --- Pagination Logic for Profile Page ---
	pageStr := r.URL.Query().Get("page")
	currentPage, err := strconv.Atoi(pageStr) // Renamed to currentPage for clarity
	if err != nil || currentPage < 1 {
		currentPage = 1
	}
	// Define total pages for profile.
	profileTotalPages := 2 // Page 1: Info/Password, Page 2: Reset/Delete
	if currentPage > profileTotalPages {
		currentPage = profileTotalPages // Cap at max pages
	}
	// --- End Pagination Logic ---

	templateData := app.newTemplateData(r)
	templateData.Title = "User Profile"
	templateData.User = user
	templateData.ProfileCurrentPage = currentPage // Use the processed currentPage
	templateData.ProfileTotalPages = profileTotalPages

	if templateData.FormData == nil {
		templateData.FormData = make(map[string]string)
	}
	if _, ok := templateData.FormData["name"]; !ok {
		templateData.FormData["name"] = user.Name
	}
	if _, ok := templateData.FormData["email"]; !ok {
		templateData.FormData["email"] = user.Email
	}

	err = app.render(w, http.StatusOK, "profile.tmpl", templateData)
	if err != nil {
		app.serverError(w, r, err)
	}
}

func (app *application) updateUserProfile(w http.ResponseWriter, r *http.Request) {
	userID := app.getUserIDFromSession(r)
	if userID == 0 {
		app.clientError(w, http.StatusUnauthorized)
		return
	}

	user, err := app.users.Get(userID)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		app.clientError(w, http.StatusMethodNotAllowed)
		return
	}

	err = r.ParseForm()
	if err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	originalEmail := user.Email

	// Update user fields from form
	user.Name = r.PostForm.Get("name")
	user.Email = r.PostForm.Get("email")

	v := validator.NewValidator()
	// Re-validate the updated fields
	v.Check(validator.NotBlank(user.Name), "name", "Name must be provided")
	v.Check(validator.MaxLength(user.Name, 100), "name", "Name must not be more than 100 characters")
	v.Check(validator.NotBlank(user.Email), "email", "Email must be provided")
	v.Check(validator.MaxLength(user.Email, 254), "email", "Must not be more than 254 characters")
	v.Check(validator.Matches(user.Email, validator.EmailRX), "email", "Must be a valid email address")

	if !v.ValidData() {
		templateData := app.newTemplateData(r)
		templateData.Title = "User Profile (Error)"
		templateData.User = user // Pass user with attempted changes but validation errors
		templateData.FormErrors = v.Errors
		templateData.FormData = map[string]string{ // Repopulate form with submitted data
			"name":  user.Name,
			"email": user.Email,
		}
		templateData.ProfileCurrentPage = 1 // Explicitly set page for error re-render
		errRender := app.render(w, http.StatusUnprocessableEntity, "profile.tmpl", templateData)
		if errRender != nil {
			app.serverError(w, r, errRender)
		}
		return
	}

	// Call the UserModel's Update method
	err = app.users.Update(user)
	if err != nil {
		if errors.Is(err, data.ErrDuplicateEmail) {
			v.AddError("email", "Email address is already in use")
			templateData := app.newTemplateData(r)
			templateData.Title = "User Profile (Error)"
			user.Email = originalEmail // Revert email in struct before passing to template
			templateData.User = user
			templateData.FormErrors = v.Errors
			templateData.FormData = map[string]string{
				"name":  r.PostForm.Get("name"),  // Show what user typed
				"email": r.PostForm.Get("email"), // Show problematic email
			}
			templateData.ProfileCurrentPage = 1 // Explicitly set page
			errRender := app.render(w, http.StatusUnprocessableEntity, "profile.tmpl", templateData)
			if errRender != nil {
				app.serverError(w, r, errRender)
			}
		} else {
			app.serverError(w, r, err)
		}
		return
	}

	app.session.Put(r, "flash", "Profile updated successfully.")
	http.Redirect(w, r, "/user/profile", http.StatusSeeOther)
}

func (app *application) changeUserPassword(w http.ResponseWriter, r *http.Request) {
	userID := app.getUserIDFromSession(r)
	if userID == 0 {
		app.clientError(w, http.StatusUnauthorized)
		return
	}

	user, err := app.users.Get(userID)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		app.clientError(w, http.StatusMethodNotAllowed)
		return
	}

	err = r.ParseForm()
	if err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	currentPassword := r.PostForm.Get("current_password")
	newPassword := r.PostForm.Get("new_password")
	confirmPassword := r.PostForm.Get("confirm_password")

	v := validator.NewValidator()
	data.ValidatePasswordUpdate(v, currentPassword, newPassword, confirmPassword)

	// Render error page helper
	renderPasswordError := func(errors map[string]string) {
		templateData := app.newTemplateData(r)
		templateData.Title = "User Profile (Password Error)"
		templateData.User = user // Pass existing user info
		templateData.FormErrors = errors
		// Don't repopulate password fields, but keep name/email for context
		templateData.FormData = make(map[string]string)
		templateData.FormData["name"] = user.Name
		templateData.FormData["email"] = user.Email
		templateData.ProfileCurrentPage = 1 // Password form is on page 1

		errRender := app.render(w, http.StatusUnprocessableEntity, "profile.tmpl", templateData)
		if errRender != nil {
			app.serverError(w, r, errRender)
		}
	}

	if !v.ValidData() {
		renderPasswordError(v.Errors)
		return
	}

	// Check if current password matches
	match, err := user.Password.Matches(currentPassword)
	if err != nil {
		app.serverError(w, r, err)
		return
	}
	if !match {
		v.AddError("current_password", "Current password incorrect")
		renderPasswordError(v.Errors)
		return
	}

	// Set the new password (updates hash in the user struct)
	err = user.Password.Set(newPassword)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	// Update the password in the database
	err = app.users.UpdatePassword(user.ID, user.Password.Hash())
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	app.session.Put(r, "flash", "Password updated successfully.")
	http.Redirect(w, r, "/user/profile", http.StatusSeeOther)
}

func (app *application) resetUserEntries(w http.ResponseWriter, r *http.Request) {
	userID := app.getUserIDFromSession(r)
	if userID == 0 {
		app.clientError(w, http.StatusUnauthorized)
		return
	}

	if r.Method != http.MethodPost {
		app.clientError(w, http.StatusMethodNotAllowed)
		return
	}

	err := app.moods.DeleteAllByUserID(userID)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	app.session.Put(r, "flash", "All your mood entries have been reset.")
	http.Redirect(w, r, "/user/profile", http.StatusSeeOther)
}

func (app *application) deleteUserAccount(w http.ResponseWriter, r *http.Request) {
	userID := app.getUserIDFromSession(r)
	if userID == 0 {
		app.clientError(w, http.StatusUnauthorized)
		return
	}

	if r.Method != http.MethodPost {
		app.clientError(w, http.StatusMethodNotAllowed)
		return
	}

	// Delete the user (moods should cascade delete)
	err := app.users.Delete(userID)
	if err != nil {
		if errors.Is(err, data.ErrRecordNotFound) {
			app.logger.Warn("Attempt to delete non-existent user account", "userID", userID)
		} else {
			app.serverError(w, r, err)
			return
		}
	}

	// Log the user out
	app.session.Remove(r, "authenticatedUserID")
	app.session.Put(r, "flash", "Your account has been successfully deleted.")
	http.Redirect(w, r, "/landing", http.StatusSeeOther)
}

/*
==========================================================================

	Error Handlers
	==========================================================================
*/
func (app *application) serverError(w http.ResponseWriter, r *http.Request, err error) {
	var (
		method = r.Method
		uri    = r.URL.RequestURI()
	)
	app.logger.Error("server error encountered", "error", err.Error(), "method", method, "uri", uri)
	if headersSent := w.Header().Get("Content-Type"); headersSent != "" {
		app.logger.Warn("headers already written, cannot send error response", "sent_content_type", headersSent)
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
