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
	// "github.com/lib/pq" // Keep commented unless specific pq errors are checked

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
	// Use local error variables within if blocks
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
		page = 1
	}

	criteria := data.FilterCriteria{
		TextQuery: searchQuery, Emotion: filterCombinedEmotion,
		StartDate: filterStartDate, EndDate: filterEndDate,
		Page: page, PageSize: 4, UserID: userID,
	}

	app.logger.Info("Fetching filtered moods", "criteria", fmt.Sprintf("%+v", criteria))
	moods, metadata, err := app.moods.GetFiltered(criteria)
	if err != nil {
		if err.Error() == "invalid user ID provided for filtering moods" {
			app.logger.Error("Invalid UserID passed to GetFiltered", "userID", userID)
			app.serverError(w, r, errors.New("internal inconsistency: invalid user session"))
			return
		}
		app.logger.Error("Failed to fetch filtered moods", "error", err)
		moods = []*data.Mood{}
		metadata = data.Metadata{}
	}

	displayMoods := make([]displayMood, len(moods))
	for i, moodEntry := range moods { // Renamed inner variable to avoid potential conflict if 'm' used elsewhere
		displayMoods[i] = displayMood{
			ID: moodEntry.ID, CreatedAt: moodEntry.CreatedAt, UpdatedAt: moodEntry.UpdatedAt,
			Title: moodEntry.Title, Content: template.HTML(moodEntry.Content), RawContent: moodEntry.Content,
			Emotion: moodEntry.Emotion, Emoji: moodEntry.Emoji, Color: moodEntry.Color,
		}
	}

	availableEmotions, err := app.moods.GetDistinctEmotionDetails(userID)
	if err != nil {
		app.logger.Error("Failed to fetch distinct emotions", "error", err, "userID", userID)
		availableEmotions = []data.EmotionDetail{}
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

	if r.Header.Get("HX-Request") == "true" {
		app.logger.Info("Handling HTMX request for dashboard content area")
		ts, ok := app.templateCache["dashboard.tmpl"]
		if !ok {
			err := fmt.Errorf("template %q does not exist", "dashboard.tmpl")
			app.logger.Error("Template lookup failed", "template", "dashboard.tmpl", "error", err)
			app.serverError(w, r, err)
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
			app.serverError(w, r, err)
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

	if r.Method != http.MethodPost { /* ... method check ... */
	}
	err := r.ParseForm()
	if err != nil { /* ... parse error ... */
	}
	title := r.PostForm.Get("title")
	content := r.PostForm.Get("content")
	emotionName := r.PostForm.Get("emotion")
	emoji := r.PostForm.Get("emoji")
	color := r.PostForm.Get("color")

	mood := &data.Mood{
		Title: title, Content: content, Emotion: emotionName,
		Emoji: emoji, Color: color, UserID: userID,
	}

	v := validator.NewValidator()
	data.ValidateMood(v, mood)

	if !v.ValidData() {
		templateData := app.newTemplateData(r)
		templateData.Title = "New Mood Entry (Error)"
		templateData.HeaderText = "Log Your Mood"
		templateData.FormErrors = v.Errors
		templateData.FormData = map[string]string{
			"title": title, "content": content, "emotion": emotionName,
			"emoji": emoji, "color": color, "emotion_choice": r.PostForm.Get("emotion_choice"),
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
	templateData.Mood = mood
	templateData.FormData = map[string]string{
		"title": mood.Title, "content": mood.Content, "emotion": mood.Emotion,
		"emoji": mood.Emoji, "color": mood.Color, "emotion_choice": mood.Emotion,
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

	originalMoodForCheck, err := app.moods.Get(id, userID)
	if err != nil {
		if errors.Is(err, data.ErrRecordNotFound) {
			app.notFound(w)
		} else {
			app.serverError(w, r, err)
		}
		return
	}

	if r.Method != http.MethodPost { /* ... method check ... */
	}
	err = r.ParseForm()
	if err != nil { /* ... parse error ... */
	}
	title := r.PostForm.Get("title")
	content := r.PostForm.Get("content")
	emotionName := r.PostForm.Get("emotion")
	emoji := r.PostForm.Get("emoji")
	color := r.PostForm.Get("color")

	mood := &data.Mood{
		ID: id, Title: title, Content: content, Emotion: emotionName,
		Emoji: emoji, Color: color, UserID: userID,
	}

	v := validator.NewValidator()
	data.ValidateMood(v, mood)

	if !v.ValidData() {
		templateData := app.newTemplateData(r)
		templateData.Title = fmt.Sprintf("Edit Mood Entry #%d (Error)", id)
		templateData.HeaderText = "Update Your Mood Entry"
		templateData.Mood = originalMoodForCheck
		templateData.FormErrors = v.Errors
		templateData.FormData = map[string]string{
			"title": title, "content": content, "emotion": emotionName,
			"emoji": emoji, "color": color, "emotion_choice": r.PostForm.Get("emotion_choice"),
		}
		errRender := app.render(w, http.StatusUnprocessableEntity, "mood_edit_form.tmpl", templateData)
		if errRender != nil {
			app.serverError(w, r, errRender)
		}
		return
	}

	err = app.moods.Update(mood)
	if err != nil {
		if errors.Is(err, data.ErrRecordNotFound) {
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
	if r.Method != http.MethodPost { /* ... method check ... */
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

	err = app.moods.Delete(id, userID) // Pass ID and UserID

	deleteErrOccurred := false
	flashMessage := ""

	if err != nil {
		if errors.Is(err, data.ErrRecordNotFound) {
			app.logger.Warn("Attempted delete non-existent/unauthorized mood", "id", id, "userID", userID)
			flashMessage = "" // No flash for not found on delete
		} else {
			app.serverError(w, r, err)
			deleteErrOccurred = true
		}
	} else {
		app.logger.Info("Mood entry deleted successfully", "id", id, "userID", userID)
		flashMessage = "Mood entry successfully deleted."
	}

	if flashMessage != "" && !deleteErrOccurred {
		app.session.Put(r, "flash", flashMessage)
		app.logger.Info("Set flash message for delete success", "message", flashMessage)
	}

	if r.Header.Get("HX-Request") == "true" && !deleteErrOccurred {
		currentFlash := app.session.PopString(r, "flash")
		app.logger.Info("Popped flash message for HTMX delete response", "message", currentFlash)

		currentPage := 1
		searchQuery := ""
		filterCombinedEmotion := ""
		filterStartDateStr := ""
		filterEndDateStr := ""

		// Parse the Referer URL - Use refererURL variable here
		refererURL, parseErr := url.Parse(r.Header.Get("Referer"))
		if parseErr == nil {
			refQuery := refererURL.Query() // Use refererURL
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

		// Declare and parse date variables - Use filterStartDate and filterEndDate
		var filterStartDate, filterEndDate time.Time
		if filterStartDateStr != "" {
			var parseErrStart error
			filterStartDate, parseErrStart = time.Parse("2006-01-02", filterStartDateStr)
			if parseErrStart != nil {
				app.logger.Warn("Invalid start date format from referer", "date", filterStartDateStr, "error", parseErrStart)
				filterStartDate = time.Time{}
			}
		}
		if filterEndDateStr != "" {
			var parseErrEnd error
			parsedEndDate, parseErrEnd := time.Parse("2006-01-02", filterEndDateStr)
			if parseErrEnd != nil {
				app.logger.Warn("Invalid end date format from referer", "date", filterEndDateStr, "error", parseErrEnd)
				filterEndDate = time.Time{}
			} else {
				filterEndDate = parsedEndDate.Add(24*time.Hour - 1*time.Nanosecond)
			}
			if !filterStartDate.IsZero() && !filterEndDate.IsZero() && filterEndDate.Before(filterStartDate) {
				app.logger.Warn("End date before start date from referer", "start", filterStartDateStr, "end", filterEndDateStr)
				filterEndDate = time.Time{}
			}
		}

		// Adjust page if last item deleted
		countCriteria := data.FilterCriteria{
			TextQuery: searchQuery, Emotion: filterCombinedEmotion,
			StartDate: filterStartDate, EndDate: filterEndDate, // Use date variables
			PageSize: 4, Page: 1, UserID: userID,
		}
		// Use tempMetadata
		_, tempMetadata, countErr := app.moods.GetFiltered(countCriteria)
		if countErr != nil {
			app.logger.Error("Failed to get count for page adjustment", "error", countErr)
		} else {
			lastPage := tempMetadata.LastPage // Use tempMetadata
			if lastPage == 0 {
				lastPage = 1
			}
			if currentPage > lastPage {
				app.logger.Info("Adjusting page after delete", "old_page", currentPage, "new_page", lastPage)
				currentPage = lastPage
			}
		}

		// Fetch moods for the potentially adjusted current page
		criteria := data.FilterCriteria{
			TextQuery: searchQuery, Emotion: filterCombinedEmotion,
			StartDate: filterStartDate, EndDate: filterEndDate, // Use date variables
			Page: currentPage, PageSize: 4, UserID: userID,
		}
		moods, metadata, fetchErr := app.moods.GetFiltered(criteria)
		if fetchErr != nil { /* log error, send HTMX error response */
			return
		}

		displayMoods := make([]displayMood, len(moods))
		for i, moodEntry := range moods { // Renamed inner var
			displayMoods[i] = displayMood{
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
		templateData.Flash = currentFlash
		templateData.SearchQuery = searchQuery
		templateData.FilterEmotion = filterCombinedEmotion
		templateData.FilterStartDate = filterStartDateStr
		templateData.FilterEndDate = filterEndDateStr
		templateData.DisplayMoods = displayMoods
		templateData.HasMoodEntries = len(displayMoods) > 0
		templateData.AvailableEmotions = availableEmotions
		templateData.Metadata = metadata

		ts, ok := app.templateCache["dashboard.tmpl"]
		if !ok { /* log error, send HTMX error response */
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		execErr := ts.ExecuteTemplate(w, "dashboard-content", templateData)
		if execErr != nil { /* log error */
		}
		return
	}

	if !deleteErrOccurred {
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
	}
}

/*
==========================================================================

	User Authentication Handlers (Unchanged)
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
	err := r.ParseForm()
	if err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}
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
		templateData.FormData = map[string]string{"name": name, "email": email}
		templateData.FormErrors = v.Errors
		errRender := app.render(w, http.StatusUnprocessableEntity, "signup.tmpl", templateData)
		if errRender != nil {
			app.serverError(w, r, errRender)
		}
		return
	}
	user := &data.User{Name: name, Email: email, Activated: true}
	err = user.Password.Set(passwordInput)
	if err != nil {
		app.serverError(w, r, err)
		return
	}
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
	err := r.ParseForm()
	if err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}
	email := r.PostForm.Get("email")
	passwordInput := r.PostForm.Get("password")
	v := validator.NewValidator()
	v.Check(validator.NotBlank(email), "generic", "Email must be provided")
	v.Check(validator.NotBlank(passwordInput), "generic", "Password must be provided")

	if !v.ValidData() {
		templateData := app.newTemplateData(r)
		templateData.Title = "Login (Error) - Feel Flow"
		templateData.FormData = map[string]string{"email": email}
		templateData.FormErrors = map[string]string{"generic": "Both email and password must be provided."}
		errRender := app.render(w, http.StatusUnprocessableEntity, "login.tmpl", templateData)
		if errRender != nil {
			app.serverError(w, r, errRender)
		}
		return
	}
	id, err := app.users.Authenticate(email, passwordInput)
	if err != nil {
		if errors.Is(err, data.ErrInvalidCredentials) {
			templateData := app.newTemplateData(r)
			templateData.Title = "Login (Error) - Feel Flow"
			templateData.FormData = map[string]string{"email": email}
			templateData.FormErrors = map[string]string{"generic": "Invalid email or password."}
			errRender := app.render(w, http.StatusUnprocessableEntity, "login.tmpl", templateData)
			if errRender != nil {
				app.serverError(w, r, errRender)
			}
		} else {
			app.serverError(w, r, err)
		}
		return
	}
	app.session.Put(r, "authenticatedUserID", id)
	app.session.Put(r, "flash", "You have been logged in successfully!")
	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}

func (app *application) logoutUser(w http.ResponseWriter, r *http.Request) {
	app.session.Remove(r, "authenticatedUserID")
	app.session.Put(r, "flash", "You have been logged out successfully.")
	http.Redirect(w, r, "/landing", http.StatusSeeOther)
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

	emotionCountsJSON, err := json.Marshal(stats.EmotionCounts)
	if err != nil {
		app.serverError(w, r, fmt.Errorf("marshal emotion counts: %w", err))
		return
	}
	monthlyCountsJSON, err := json.Marshal(stats.MonthlyCounts)
	if err != nil {
		app.serverError(w, r, fmt.Errorf("marshal monthly counts: %w", err))
		return
	}

	templateData := app.newTemplateData(r)
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

/*
==========================================================================

	Error Handlers (Unchanged)
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
