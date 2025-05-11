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
	"unicode/utf8"

	"github.com/mickali02/mood/internal/data"
	"github.com/mickali02/mood/internal/validator"
	"github.com/microcosm-cc/bluemonday"
)

// getUserIDFromSession checks if a user is logged in by looking for their ID in the session
func (app *application) getUserIDFromSession(r *http.Request) int64 {
	if !app.session.Exists(r, "authenticatedUserID") {
		return 0
	}
	// Try to get the user ID and make sure it's the right type (int64)
	userID, ok := app.session.Get(r, "authenticatedUserID").(int64)
	if !ok {
		// Log an error if the ID isn't the expected type
		app.logger.Error("authenticatedUserID in session is not int64")
		return 0
	}
	// Return the valid user ID
	return userID
}

// Helper function to strip HTML and truncate text
func truncateTextWithEllipsis(htmlContent string, limit int) string {
	// 1. Sanitize HTML: Use bluemonday's strict policy to remove all HTML tags.
	p := bluemonday.StrictPolicy()
	plainText := p.Sanitize(htmlContent)

	// 2. Check Length: Count runes (Unicode characters) for accurate length.
	//    Using utf8.RuneCountInString handles multi-byte characters correctly
	if utf8.RuneCountInString(plainText) <= limit {
		return plainText
	}

	// 3. Truncate and Add Ellipsis: If over limit, convert to runes, slice, and append "...".
	runes := []rune(plainText)
	return string(runes[:limit]) + "..."
}

/*
==========================================================================

	START: Dashboard Handler
==========================================================================
*/
// showDashboardPage handles GET requests to the /dashboard endpoint.
// Primary role is to display a user's mood entries, allowing for filtering and pagination.
// It also handles both full page loads and partial updates via HTMX.
func (app *application) showDashboardPage(w http.ResponseWriter, r *http.Request) {
	// --- 1. AUTHENTICATION & USER IDENTIFICATION ---
	// First, we need to know *who* is requesting the page.
	// We retrieve the user's ID from the session.
	// If userID is 0, it means the user is not authenticated or the session is invalid.
	userID := app.getUserIDFromSession(r)
	if userID == 0 {
		app.clientError(w, http.StatusUnauthorized)
		return
	}

	// --- 2. FETCHING USER DETAILS (for personalization) ---
	// Once authenticated, we fetch the user's details (like their name) from the database.
	// This is used to personalize the dashboard (e.g., "Hi, [UserName]!").
	user, err := app.users.Get(userID)
	if err != nil {
		// If fetching fails (e.g., database error), log it.
		// We still proceed but with an empty user struct, so the page doesn't crash.
		app.logger.Error("Failed to get user details for dashboard", "userID", userID, "error", err)
		user = &data.User{}
	}

	// --- 3. PROCESSING URL QUERY PARAMETERS (for filtering and pagination) ---
	// The dashboard can be filtered (by text, emotion, date range) and paginated.
	// These parameters are passed in the URL (e.g., /dashboard?query=happy&page=2).

	// Initialize a new validator instance for validating query parameters.
	v := validator.NewValidator()

	// r.URL.Query() parses the query string from the URL into a map-like structure.
	query := r.URL.Query()

	// Get filter values from the query parameters.
	// query.Get("param_name") retrieves the value for "param_name". If not present, it returns an empty string.
	searchQuery := query.Get("query")             // For text search in title/content
	filterCombinedEmotion := query.Get("emotion") // For filtering by a specific emotion (e.g., "Happy::😊")
	filterStartDateStr := query.Get("start_date") // Start of date range filter
	filterEndDateStr := query.Get("end_date")     // End of date range filter
	pageStr := query.Get("page")                  // Requested page number for pagination

	// --- 3a. PAGE NUMBER PARSING & VALIDATION ---
	// Convert the page string to an integer.
	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 { // If conversion fails or page is not positive
		page = 1 // Default to page 1
	}

	// Validate the page number using our validator.
	v.Check(page > 0, "page", "must be a positive integer")
	v.Check(page <= 10_000_000, "page", "must be less than 10 million")

	// --- 3b. DATE FILTER PARSING & VALIDATION ---
	var filterStartDate, filterEndDate time.Time // Initialize as zero-value time.Time

	// Parse the start date string if provided.
	if filterStartDateStr != "" {
		var parseErr error
		// time.Parse expects a layout string ("2006-01-02") and the date string to parse.
		filterStartDate, parseErr = time.Parse("2006-01-02", filterStartDateStr)
		if parseErr != nil {
			// If parsing fails, log a warning and keep filterStartDate as zero (effectively no start date filter).
			app.logger.Warn("Invalid start date format", "date", filterStartDateStr, "error", parseErr)
			filterStartDate = time.Time{} // reset to zero value if parsing fails
		}
	}

	// Parse the end date string if provided.
	if filterEndDateStr != "" {
		var parseErr error
		parsedEndDate, parseErr := time.Parse("2006-01-02", filterEndDateStr)
		if parseErr != nil {
			app.logger.Warn("Invalid end date format", "date", filterEndDateStr, "error", parseErr)
			filterEndDate = time.Time{} // Reset to zero value
		} else {
			// To make the end date inclusive for the entire day, set it to the end of that day.
			filterEndDate = parsedEndDate.Add(24*time.Hour - 1*time.Nanosecond)
		}
		// Basic validation: if both dates are set, end date should not be before start date.
		if !filterStartDate.IsZero() && !filterEndDate.IsZero() && filterEndDate.Before(filterStartDate) {
			app.logger.Warn("End date before start date, ignoring end date", "start", filterStartDateStr, "end", filterEndDateStr)
			filterEndDate = time.Time{} // Ignore the end date if it's invalid relative to start date
		}
	}

	// --- 3c. APPLYING VALIDATION RESULTS ---
	// If any validation checks (e.g., for the page number) failed:
	if !v.ValidData() {
		app.logger.Warn("Invalid page parameter", "page", pageStr, "errors", v.Errors)
		page = 1 // Default to page 1 on any validation error for query parameters
	}

	// --- 4. PREPARING FILTER CRITERIA FOR DATABASE QUERY ---
	// Consolidate all filter parameters into a FilterCriteria struct.
	// This struct is passed to the data model (app.moods.GetFiltered) to fetch relevant mood entries.
	// PageSize is hardcoded to 4 entries per page for this dashboard.
	criteria := data.FilterCriteria{
		TextQuery: searchQuery,
		Emotion:   filterCombinedEmotion,
		StartDate: filterStartDate,
		EndDate:   filterEndDate,
		Page:      page, PageSize: 4, // Defines how many mood entries to show per page
		UserID: userID, // Crucial: ensures we only fetch moods for the logged-in user
	}

	// --- 5. FETCHING MOOD ENTRIES & METADATA FROM DATABASE ---
	// Call the GetFiltered method on our mood model, passing the criteria.
	// This method returns:
	//   - `moods`: A slice of *data.Mood pointers matching the filters.
	//   - `metadata`: Pagination information (total records, current page, last page, etc.).
	//   - `err`: Any error encountered during the database query.
	moods, metadata, err := app.moods.GetFiltered(criteria)
	if err != nil {
		// Specific error handling for a case where an invalid UserID might be passed.
		// This is more of a consistency check; userID should be valid from the session.
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

	// Define the character limit for the short content on dashboard cards
	const shortContentCharacterLimit = 35 // Adjust as needed

	// --- 6. TRANSFORMING MOOD DATA FOR DISPLAY ---
	// The `data.Mood` struct might contain raw data (e.g., HTML content as a string).
	// We transform it into a `displayMood` struct, which is tailored for the template.
	// For example, `Content` is converted to `template.HTML` to prevent XSS vulnerabilities
	// when rendering user-generated HTML content.
	displayMoods := make([]displayMood, len(moods))
	for i, moodEntry := range moods {
		displayMoods[i] = displayMood{
			ID:           moodEntry.ID,
			CreatedAt:    moodEntry.CreatedAt,
			UpdatedAt:    moodEntry.UpdatedAt,
			Title:        moodEntry.Title,
			Content:      template.HTML(moodEntry.Content),                                                       // Mark content as safe HTML for template
			ShortContent: template.HTML(truncateTextWithEllipsis(moodEntry.Content, shortContentCharacterLimit)), // Truncated plain text
			RawContent:   moodEntry.Content,                                                                      // Keep raw content for "View More" modal
			Emotion:      moodEntry.Emotion,
			Emoji:        moodEntry.Emoji,
			Color:        moodEntry.Color,
		}
	}

	// --- 7. FETCHING DISTINCT EMOTIONS (for filter dropdown) ---
	// To populate the "Filter by Emotion" dropdown, we fetch all unique emotion/emoji/color
	// combinations that the current user has logged.
	availableEmotions, err := app.moods.GetDistinctEmotionDetails(userID)
	if err != nil {
		app.logger.Error("Failed to fetch distinct emotions", "error", err, "userID", userID)
		availableEmotions = []data.EmotionDetail{} // Default to empty slice
	}

	// --- 8. PREPARING TEMPLATE DATA ---
	// Consolidate all data needed by the HTML template into a `TemplateData` struct.
	// `app.newTemplateData(r)` initializes common fields like CSRF token, authentication status, flash messages.
	templateData := app.newTemplateData(r)
	templateData.Title = "Dashboard"
	// Pass back filter values so the form fields can be re-populated with current selections.
	templateData.SearchQuery = searchQuery
	templateData.FilterEmotion = filterCombinedEmotion
	templateData.FilterStartDate = filterStartDateStr
	templateData.FilterEndDate = filterEndDateStr
	// Data to display.
	templateData.DisplayMoods = displayMoods
	templateData.HasMoodEntries = len(displayMoods) > 0 // For conditional rendering in template
	templateData.AvailableEmotions = availableEmotions  // For the filter dropdown
	templateData.Metadata = metadata                    // For pagination controls
	templateData.UserName = user.Name                   // User's name for personalization

	// --- 9. RENDERING THE PAGE (Full Page Load vs. HTMX Partial Update) ---
	// Check if the request is an HTMX request by looking for the "HX-Request" header.
	// HTMX uses this header to indicate that it's an AJAX request expecting a partial HTML response.
	if r.Header.Get("HX-Request") == "true" {
		// --- 9a. HTMX PARTIAL UPDATE ---
		// If it's an HTMX request (e.g., user changed a filter, clicked pagination).
		app.logger.Info("Handling HTMX request for dashboard content area")
		// Retrieve the "dashboard.tmpl" template from our pre-compiled cache.
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
		// --- 9b. FULL PAGE LOAD ---
		// If not an HTMX request, it's a standard browser request for the full page.
		app.logger.Info("Handling full page request for dashboard")
		// Render the entire "dashboard.tmpl" page with all its layout.
		// `app.render` is a helper function that handles template execution and writing to the response.
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

// showLandingPage handles GET requests to the root ("/") or "/landing" endpoint.
// Its purpose is to display the main welcome/entry page of the application.
func (app *application) showLandingPage(w http.ResponseWriter, r *http.Request) {
	// --- 1. PREPARE TEMPLATE DATA ---
	// `app.newTemplateData(r)` is a helper method that initializes a TemplateData struct.
	// This struct holds data that will be passed to the HTML template.
	// The helper typically populates:
	//   - Authentication status (IsAuthenticated)
	//   - Flash messages (if any, popped from the session)
	//   - CSRF token (for any forms that might be on the page, though landing usually doesn't have POST forms)
	//   - Default values for common fields (like default emotions list, if used globally)
	templateData := app.newTemplateData(r)
	// Set the specific title for the landing page. This will be used in the <title> tag of the HTML.
	templateData.Title = "Feel Flow"

	// --- 2. RENDER THE HTML TEMPLATE ---
	// `app.render()` is another helper method responsible for:
	//   1. Looking up the specified template file (e.g., "landing.tmpl") in the template cache.
	//   2. Executing the template with the provided `templateData`.
	//   3. Writing the resulting HTML to the `http.ResponseWriter` (`w`) with the given HTTP status code (http.StatusOK, which is 200).
	err := app.render(w, http.StatusOK, "landing.tmpl", templateData)

	// --- 3. ERROR HANDLING ---
	// If the `app.render()` method encounters an error (e.g., template not found, error during template execution),
	// it will return an error.
	if err != nil {
		app.serverError(w, r, err)
	}
}

// showAboutPage handles GET requests to the "/about" endpoint.
// It displays the "About" page, providing information about the application.
func (app *application) showAboutPage(w http.ResponseWriter, r *http.Request) {
	// --- 1. PREPARE TEMPLATE DATA ---
	// Similar to showLandingPage, we initialize the base template data.
	// This ensures consistency in how common data (auth status, CSRF token) is available to all pages.
	templateData := app.newTemplateData(r)
	templateData.Title = "About Feel Flow"
	// --- 2. RENDER THE HTML TEMPLATE ---
	// Render the "about.tmpl" HTML template with the prepared data.
	err := app.render(w, http.StatusOK, "about.tmpl", templateData)
	// --- 3. ERROR HANDLING ---
	// If rendering fails, log the error and send a 500 response.
	if err != nil {
		app.serverError(w, r, err)
	}
}

/*
==========================================================================

	Mood Handlers
==========================================================================
*/

// showMoodForm displays the HTML form for creating a new mood entry.
// This is the 'C' in CRUD - Create. It serves the page where users input new mood data.
func (app *application) showMoodForm(w http.ResponseWriter, r *http.Request) {
	// 1. Prepare Base Template Data: Initializes common data like CSRF token, auth status.
	templateData := app.newTemplateData(r)
	// 2. Set Page-Specific Data: Title for the HTML head, HeaderText for the main heading on the form.
	templateData.Title = "New Mood Entry"
	templateData.HeaderText = "Log Your Mood"
	// 3. Render Form: Uses the "mood_form.tmpl" template.
	//    `app.render` is a helper to execute the template with data and send to the browser.
	err := app.render(w, http.StatusOK, "mood_form.tmpl", templateData)
	if err != nil {
		// 4. Handle Errors: If rendering fails, log it and show a server error page.
		app.serverError(w, r, err)
	}
}

// createMood handles the submission (POST request) of the new mood form.
// After submitting the form, this handler processes the data, validates it, and saves it to the database.
func (app *application) createMood(w http.ResponseWriter, r *http.Request) {
	// 1. Authentication: Ensure the user is logged in before creating a mood.
	userID := app.getUserIDFromSession(r)
	if userID == 0 {
		app.clientError(w, http.StatusUnauthorized) // User not logged in.
		return
	}

	// 2. Method Check: This handler expects a POST request.
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)        // Inform client only POST is allowed.
		app.clientError(w, http.StatusMethodNotAllowed) // Send 405 error.
		return
	}

	// 3. Parse Form Data: `r.ParseForm()` populates `r.PostForm` with submitted values.
	err := r.ParseForm()
	if err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	// 4. Extract Data: Get individual field values from the parsed form.
	title := r.PostForm.Get("title")
	content := r.PostForm.Get("content")              // Content from Quill editor (HTML).
	emotionName := r.PostForm.Get("emotion")          // Final selected/custom emotion name.
	emoji := r.PostForm.Get("emoji")                  // Final selected/custom emoji.
	color := r.PostForm.Get("color")                  // Final selected/custom color.
	emotionChoice := r.PostForm.Get("emotion_choice") // Keep track of radio button selection

	// 5. Populate Mood Struct: Create a `data.Mood` struct with the extracted data.
	mood := &data.Mood{
		Title:   title,
		Content: content, // Storing raw HTML from Quill.
		Emotion: emotionName,
		Emoji:   emoji,
		Color:   color,
		UserID:  userID, // Associate mood with the logged-in user.
	}

	// 6. Validation: Validate the mood data using our custom validator.
	//    `data.ValidateMood` checks for blank fields, length limits, valid formats, etc.
	v := validator.NewValidator()
	data.ValidateMood(v, mood)

	// 7. Handle Validation Errors: If data is invalid...
	if !v.ValidData() {
		templateData := app.newTemplateData(r)
		templateData.Title = "New Mood Entry (Error)"
		templateData.HeaderText = "Log Your Mood"
		templateData.FormErrors = v.Errors // Pass validation errors to the template.
		// Repopulate form data for user convenience
		templateData.FormData = map[string]string{
			"title":          title,
			"content":        content,
			"emotion":        emotionName,
			"emoji":          emoji,
			"color":          color,
			"emotion_choice": emotionChoice, // Repopulate selected radio
		}
		// Re-render the form with a 422 Unprocessable Entity status.
		errRender := app.render(w, http.StatusUnprocessableEntity, "mood_form.tmpl", templateData)
		if errRender != nil {
			app.serverError(w, r, errRender)
		}
		return
	}

	// 8. Database Insert: If data is valid, insert the new mood into the database.
	//    `app.moods.Insert` is a method on our MoodModel.
	err = app.moods.Insert(mood)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	// 9. Success & Redirect: On successful creation...
	//    Set a flash message to inform the user.
	app.session.Put(r, "flash", "Mood entry successfully created!")
	//    Redirect the user to the dashboard to see their new entry.
	//    `http.StatusSeeOther` (303) is used for POST-redirect-GET pattern.
	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}

// showEditMoodForm displays the form for editing an existing mood entry.
// This is part of 'U' in CRUD - Update. It first reads existing data to pre-fill the form.
func (app *application) showEditMoodForm(w http.ResponseWriter, r *http.Request) {
	// 1. Get Mood ID: Extract the mood ID from the URL path (e.g., /mood/edit/{id}).
	//    `r.PathValue("id")` is used with Go's new router (if applicable) or similar for other routers.
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 { // Validate ID.
		app.notFound(w)
		return
	}

	// 2. Authentication: Ensure user is logged in.
	userID := app.getUserIDFromSession(r)
	if userID == 0 {
		app.clientError(w, http.StatusUnauthorized)
		return
	}

	// 3. Fetch Existing Mood: Get the mood entry from the database using its ID and the UserID.
	//    This also acts as an ownership check: user can only edit their own moods.
	mood, err := app.moods.Get(id, userID)
	if err != nil {
		if errors.Is(err, data.ErrRecordNotFound) { // Mood not found or not owned by user.
			app.notFound(w)
		} else {
			app.serverError(w, r, err)
		}
		return
	}

	// 4. Prepare Template Data:
	templateData := app.newTemplateData(r)
	templateData.Title = fmt.Sprintf("Edit Mood Entry #%d", mood.ID)
	templateData.HeaderText = "Update Your Mood Entry"
	templateData.Mood = mood // Pass existing mood data
	// Populate FormData with existing mood data for the form fields
	// This ensures the form shows the current values of the mood entry.
	templateData.FormData = map[string]string{
		"title":          mood.Title,
		"content":        mood.Content,
		"emotion":        mood.Emotion,
		"emoji":          mood.Emoji,
		"color":          mood.Color,
		"emotion_choice": mood.Emotion, // Pre-select the correct radio button
	}

	// 5. Render Form: Use the "mood_edit_form.tmpl" template.
	err = app.render(w, http.StatusOK, "mood_edit_form.tmpl", templateData)
	if err != nil {
		app.serverError(w, r, err)
	}
}

// updateMood handles the submission (POST request) of the edit mood form.
// This completes the 'U' in CRUD. It validates submitted changes and updates the database record.
func (app *application) updateMood(w http.ResponseWriter, r *http.Request) {
	// 1. Get Mood ID: Extract ID from URL.
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		app.notFound(w)
		return
	}
	// 2. Authentication: Ensure user is logged in.
	userID := app.getUserIDFromSession(r)
	if userID == 0 {
		app.clientError(w, http.StatusUnauthorized)
		return
	}

	// 3. Fetch Original Mood (for ownership check & context on error):
	//    It's good practice to re-fetch or verify ownership before an update.
	originalMoodForCheck, err := app.moods.Get(id, userID)
	if err != nil {
		if errors.Is(err, data.ErrRecordNotFound) {
			app.notFound(w)
		} else {
			app.serverError(w, r, err)
		}
		return
	}

	// 4. Method Check: Expect POST.
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		app.clientError(w, http.StatusMethodNotAllowed)
		return
	}
	// 5. Parse Form Data.
	err = r.ParseForm()
	if err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	// 6. Extract Submitted Data.
	title := r.PostForm.Get("title")
	content := r.PostForm.Get("content")
	emotionName := r.PostForm.Get("emotion")
	emoji := r.PostForm.Get("emoji")
	color := r.PostForm.Get("color")
	emotionChoice := r.PostForm.Get("emotion_choice")

	// 7. Populate Mood Struct with Updated Values:
	//    Crucially, include the ID for the `UPDATE` SQL query and UserID for the `WHERE` clause.
	mood := &data.Mood{
		ID:      id, // Set the ID for update
		Title:   title,
		Content: content,
		Emotion: emotionName,
		Emoji:   emoji,
		Color:   color,
		UserID:  userID, // Include UserID for ownership check in model
	}

	// 8. Validation: Validate the *updated* mood data.
	v := validator.NewValidator()
	data.ValidateMood(v, mood)

	// 9. Handle Validation Errors: If updated data is invalid...
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

	// 10. Database Update: If valid, perform the update in the database.
	//     `app.moods.Update` will internally ensure `id` and `UserID` match.
	err = app.moods.Update(mood)
	if err != nil {
		if errors.Is(err, data.ErrRecordNotFound) { // Handle case where mood was deleted between GET and POST
			app.notFound(w)
		} else {
			app.serverError(w, r, err)
		}
		return
	}

	// 11. Success & Redirect:
	app.session.Put(r, "flash", "Mood entry successfully updated!")

	// Handle HTMX redirect or standard redirect
	if r.Header.Get("HX-Request") == "true" {
		// Tell HTMX to redirect the browser after successful swap/update
		w.Header().Set("HX-Redirect", "/dashboard") // Instruct HTMX to navigate to /dashboard.
		w.WriteHeader(http.StatusOK)                // HTMX needs a 200 OK for HX-Redirect to work.
	} else {
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther) // Standard redirect.
	}
}

// deleteMood handles the deletion of a mood entry.
// This is the 'D' in CRUD - Delete. It removes a mood entry based on its ID and user ownership.
func (app *application) deleteMood(w http.ResponseWriter, r *http.Request) {
	// 1. Method Check: Expect POST (often forms for delete use POST for CSRF protection).
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		app.clientError(w, http.StatusMethodNotAllowed)
		return
	}

	// 2. Get Mood ID: Extract from URL.
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		app.notFound(w)
		return
	}

	// 3. Authentication: Ensure user is logged in.
	userID := app.getUserIDFromSession(r)
	if userID == 0 {
		app.clientError(w, http.StatusUnauthorized)
		return
	}

	// 4. Database Delete: Call the model's Delete method.
	//    The model's `Delete` method should handle the ownership check
	err = app.moods.Delete(id, userID) // Model handles ownership check

	// 5. Handle Deletion Result:
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

	// 6. Set Flash Message (only on actual success):
	if flashMessage != "" {
		app.session.Put(r, "flash", flashMessage)
		app.logger.Info("Set flash message for delete success", "message", flashMessage)
	}

	// 7. Handle HTMX vs. Standard Redirect/Response:
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

// signupUserForm displays the user registration form.
// Serves the HTML form for new users to create an account.
func (app *application) signupUserForm(w http.ResponseWriter, r *http.Request) {
	// 1. Prepare Base Template Data: Initializes common data (CSRF, auth status, etc.).
	templateData := app.newTemplateData(r)
	// 2. Set Page Title: For the HTML `<title>` tag.
	templateData.Title = "Sign Up - Feel Flow"
	// 3. Render Template: Displays "signup.tmpl".
	err := app.render(w, http.StatusOK, "signup.tmpl", templateData)
	if err != nil {
		app.serverError(w, r, err) // Handle rendering errors.
	}
}

// signupUser handles the submission of the user registration form.
// Processes new user details, validates them, hashes the password, and saves the user to the database.
func (app *application) signupUser(w http.ResponseWriter, r *http.Request) {
	// 1. Method Check: Ensure request is POST.
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		app.clientError(w, http.StatusMethodNotAllowed)
		return
	}

	// 2. Parse Form Data: Extract submitted values.
	err := r.ParseForm()
	if err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	// 3. Extract User Inputs: Get name, email, and password from the form.
	name := r.PostForm.Get("name")
	email := r.PostForm.Get("email")
	passwordInput := r.PostForm.Get("password")

	// 4. Validate Inputs: Use the validator to check field requirements (not blank, length, format).
	v := validator.NewValidator()
	v.Check(validator.NotBlank(name), "name", "Name must be provided")
	v.Check(validator.MaxLength(name, 100), "name", "Must not be more than 100 characters")
	v.Check(validator.NotBlank(email), "email", "Email must be provided")
	v.Check(validator.MaxLength(email, 254), "email", "Must not be more than 254 characters")
	v.Check(validator.Matches(email, validator.EmailRX), "email", "Must be a valid email address")
	v.Check(validator.NotBlank(passwordInput), "password", "Password must be provided")
	v.Check(validator.MinLength(passwordInput, 8), "password", "Must be at least 8 characters long")
	v.Check(validator.MaxLength(passwordInput, 72), "password", "Must not be more than 72 characters")

	// 5. Handle Validation Errors: If any checks fail...
	if !v.ValidData() {
		templateData := app.newTemplateData(r)
		templateData.Title = "Sign Up (Error) - Feel Flow"
		templateData.FormData = map[string]string{"name": name, "email": email} // Don't repopulate password
		templateData.FormErrors = v.Errors
		// Re-render signup form with errors and a 422 status.
		errRender := app.render(w, http.StatusUnprocessableEntity, "signup.tmpl", templateData)
		if errRender != nil {
			app.serverError(w, r, errRender)
		}
		return
	}

	// 6. Create User Object & Hash Password:
	//    - Initialize a `data.User` struct. Users are activated immediately for simplicity.
	//    - Securely hash the password using `user.Password.Set()`, which uses bcrypt.
	user := &data.User{Name: name, Email: email, Activated: true} // Activate immediately for simplicity
	err = user.Password.Set(passwordInput)                        // Hashes and stores the password.
	if err != nil {
		app.serverError(w, r, err) // Error during password hashing.
		return
	}

	// 7. Insert User into Database:
	err = app.users.Insert(user) // `app.users` is our UserModel instance.
	if err != nil {
		// Handle specific database errors, like a duplicate email.
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

	// 8. Success: User created.
	//    Set a flash message and redirect to the login page.
	app.session.Put(r, "flash", "Your signup was successful! Please log in.")
	http.Redirect(w, r, "/user/login", http.StatusSeeOther)
}

// loginUserForm displays the user login page.
// Serves the HTML form for existing users to log in.
func (app *application) loginUserForm(w http.ResponseWriter, r *http.Request) {
	templateData := app.newTemplateData(r)
	templateData.Title = "Login - Feel Flow"
	err := app.render(w, http.StatusOK, "login.tmpl", templateData)
	if err != nil {
		app.serverError(w, r, err)
	}
}

// loginUser handles the submission of the user login form.
// Authenticates the user by checking credentials against the database and manages the session.
func (app *application) loginUser(w http.ResponseWriter, r *http.Request) {
	// 1. Method Check: Expect POST.
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		app.clientError(w, http.StatusMethodNotAllowed)
		return
	}

	// 2. Parse Form Data.
	err := r.ParseForm()
	if err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	// 3. Extract Credentials.
	email := r.PostForm.Get("email")
	passwordInput := r.PostForm.Get("password")

	// 4. Basic Validation (Presence): Check if email/password were provided.
	//    A generic error message is used for login failures to avoid revealing which field was incorrect.
	v := validator.NewValidator()
	v.Check(validator.NotBlank(email), "generic", "Email must be provided")            // Using generic key
	v.Check(validator.NotBlank(passwordInput), "generic", "Password must be provided") // Using generic key

	// Helper function to render login form with a generic error.
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

	// 5. Handle Basic Validation Failure.
	if !v.ValidData() {
		genericError() // Show generic error on the form.
		return
	}

	// 6. Authenticate User: Call UserModel's Authenticate method.
	//    This checks email, password hash, and if the user is activated.
	id, err := app.users.Authenticate(email, passwordInput)
	if err != nil {
		if errors.Is(err, data.ErrInvalidCredentials) {
			genericError()
		} else { // Handle other potential errors (e.g., database connection)
			app.serverError(w, r, err)
		}
		return
	}

	// 7. Authentication Successful:
	//    - Store the user's ID in the session to mark them as logged in.
	//    - Set a success flash message.
	//    - Redirect to the dashboard.
	app.session.Put(r, "authenticatedUserID", id) // Store user ID in session
	app.session.Put(r, "flash", "You have been logged in successfully!")
	http.Redirect(w, r, "/dashboard", http.StatusSeeOther) // Redirect to dashboard
}

// logoutUser handles the user logout process.
// Clears user authentication from the session and redirects to a public page.
func (app *application) logoutUser(w http.ResponseWriter, r *http.Request) {
	// 1. Method Check: Expect POST
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		app.clientError(w, http.StatusMethodNotAllowed)
		return
	}

	// 2. Clear Session: Remove the "authenticatedUserID" from the session.
	app.session.Remove(r, "authenticatedUserID")
	// 3. Notify User & Redirect: Set flash message and redirect to the landing page.
	app.session.Put(r, "flash", "You have been logged out successfully.")
	http.Redirect(w, r, "/landing", http.StatusSeeOther) // Redirect to landing page
}

/*
==========================================================================

	Stats Page Handler
==========================================================================
*/
// showStatsPage displays various mood statistics for the logged-in user.
// Fetches and aggregates user's mood data to display charts and summaries.
func (app *application) showStatsPage(w http.ResponseWriter, r *http.Request) {
	// 1. Authentication: Ensure user is logged in.
	userID := app.getUserIDFromSession(r)
	if userID == 0 {
		app.clientError(w, http.StatusUnauthorized)
		return
	}

	// 2. Fetch Stats Data: Call MoodModel's GetAllStats method for the current user.
	stats, err := app.moods.GetAllStats(userID)
	if err != nil {
		app.logger.Error("Failed to fetch mood stats", "error", err, "userID", userID)
		app.serverError(w, r, err)
		return
	}

	// 3. Defensive Check for Nil Stats
	if stats == nil {
		app.logger.Error("GetAllStats returned nil stats object unexpectedly", "userID", userID)
		stats = &data.MoodStats{} // Proceed with empty stats for template rendering.
	}

	// 4. Log Prepared Stats
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

	// 5. Marshal Emotion Counts to JSON: For use by JavaScript charting libraries.
	emotionCountsJSON, err := json.Marshal(stats.EmotionCounts)
	if err != nil {
		app.serverError(w, r, fmt.Errorf("marshal emotion counts: %w", err))
		return
	}

	// 6. Prepare Template Data:
	templateData := app.newTemplateData(r)
	templateData.Title = "Mood Statistics"
	templateData.Stats = stats                                          // Pass the aggregated stats.
	templateData.EmotionCountsJSON = string(emotionCountsJSON)          // Pass JSON string for charts.
	templateData.Quote = "Every mood matters. Thanks for checking in 💖" // Inspirational quote.

	// 7. Render Stats Page: Use "stats.tmpl".
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

// showUserProfilePage displays the user's profile settings page.
// Presentation Point: "Allows users to view and access forms to update their profile information and manage their account."
func (app *application) showUserProfilePage(w http.ResponseWriter, r *http.Request) {
	// 1. Authentication.
	userID := app.getUserIDFromSession(r)
	if userID == 0 {
		app.clientError(w, http.StatusUnauthorized)
		return
	}

	// 2. Fetch User Data: Get current user details to display and pre-fill forms.
	user, err := app.users.Get(userID)
	if err != nil {
		if errors.Is(err, data.ErrRecordNotFound) {
			app.notFound(w)
		} else {
			app.serverError(w, r, err)
		}
		return
	}

	// 3. Handle Profile Page Pagination (if profile settings are split across multiple views/tabs).
	pageStr := r.URL.Query().Get("page")
	currentPage, err := strconv.Atoi(pageStr) // Renamed to currentPage for clarity
	if err != nil || currentPage < 1 {
		currentPage = 1 // Default to first page/section of profile.
	}
	// Define total pages for profile.
	profileTotalPages := 2 // Page 1: Info/Password, Page 2: Reset/Delete
	if currentPage > profileTotalPages {
		currentPage = profileTotalPages // Cap at max pages
	}
	// --- End Pagination Logic ---

	// 4. Prepare Template Data:
	templateData := app.newTemplateData(r)
	templateData.Title = "User Profile"
	templateData.User = user                      // Pass user object for display.
	templateData.ProfileCurrentPage = currentPage // Use the processed currentPage
	templateData.ProfileTotalPages = profileTotalPages

	// Pre-fill form data for name/email fields if not already set by a previous error.
	if templateData.FormData == nil {
		templateData.FormData = make(map[string]string)
	}
	if _, ok := templateData.FormData["name"]; !ok {
		templateData.FormData["name"] = user.Name
	}
	if _, ok := templateData.FormData["email"]; !ok {
		templateData.FormData["email"] = user.Email
	}

	// 5. Render Profile Page: Use "profile.tmpl".
	// --- MODIFIED: Check for HTMX request ---
	if r.Header.Get("HX-Request") == "true" {
		app.logger.Info("HTMX: Rendering profile content fragment", "page", currentPage)
		ts, ok := app.templateCache["profile.tmpl"]
		if !ok {
			err := fmt.Errorf("template %q does not exist", "profile.tmpl")
			app.logger.Error("Template lookup failed for profile", "template", "profile.tmpl", "error", err)
			http.Error(w, "Error loading profile content.", http.StatusInternalServerError)
			return
		}
		// Execute only the "profile-content" block for HTMX swap
		err = ts.ExecuteTemplate(w, "profile-content", templateData)
		if err != nil {
			app.logger.Error("Failed to execute profile template block", "block", "profile-content", "error", err)
		}
	} else {
		app.logger.Info("Full page request for profile", "page", currentPage)
		err = app.render(w, http.StatusOK, "profile.tmpl", templateData)
		if err != nil {
			app.serverError(w, r, err)
		}
	}
}

// updateUserProfile handles submission for updating user's name and email.
// Processes changes to user's name/email, validates, and updates the database.
func (app *application) updateUserProfile(w http.ResponseWriter, r *http.Request) {
	// 1. Authentication.
	userID := app.getUserIDFromSession(r)
	if userID == 0 {
		app.clientError(w, http.StatusUnauthorized)
		return
	}

	// 2. Fetch Current User Data (needed if validation fails, to show original state or for context).
	user, err := app.users.Get(userID)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	// 3. Method Check & Parse Form.
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

	// 4. Store Original Email (in case of update conflict or error).
	originalEmail := user.Email

	// 5. Update User Struct Fields from Form Data.
	user.Name = r.PostForm.Get("name")
	user.Email = r.PostForm.Get("email")

	// 6. Validate Updated Fields.
	v := validator.NewValidator()
	// Re-validate the updated fields
	v.Check(validator.NotBlank(user.Name), "name", "Name must be provided")
	v.Check(validator.MaxLength(user.Name, 100), "name", "Name must not be more than 100 characters")
	v.Check(validator.NotBlank(user.Email), "email", "Email must be provided")
	v.Check(validator.MaxLength(user.Email, 254), "email", "Must not be more than 254 characters")
	v.Check(validator.Matches(user.Email, validator.EmailRX), "email", "Must be a valid email address")

	// 7. Handle Validation Errors.
	if !v.ValidData() {
		templateData := app.newTemplateData(r)
		templateData.Title = "User Profile (Error)"
		// For template consistency, pass user object (even if it has pending invalid changes).
		// The FormData map will hold the actual submitted values.
		templateData.User = &data.User{ID: user.ID, Name: user.Name, Email: user.Email, CreatedAt: user.CreatedAt}
		templateData.FormErrors = v.Errors
		templateData.FormData = map[string]string{
			"name":  user.Name,  // This is the attempted new name
			"email": user.Email, // This is the attempted new email
		}
		templateData.ProfileCurrentPage = 1 // Name/Email form is on page 1.

		// --- MODIFIED: Render fragment for HTMX on validation error ---
		if r.Header.Get("HX-Request") == "true" {
			app.logger.Info("HTMX: Re-rendering profile content due to name/email update validation errors")
			ts, ok := app.templateCache["profile.tmpl"]
			if !ok {
				app.serverError(w, r, fmt.Errorf("template profile.tmpl not found"))
				return
			}
			w.WriteHeader(http.StatusUnprocessableEntity)
			errRender := ts.ExecuteTemplate(w, "profile-content", templateData)
			if errRender != nil {
				app.serverError(w, r, errRender)
			}
		} else {
			errRender := app.render(w, http.StatusUnprocessableEntity, "profile.tmpl", templateData)
			if errRender != nil {
				app.serverError(w, r, errRender)
			}
		}
		return
	}

	// 8. Database Update (User Profile).
	err = app.users.Update(user)
	if err != nil {
		if errors.Is(err, data.ErrDuplicateEmail) {
			v.AddError("email", "Email address is already in use")
			templateData := app.newTemplateData(r)
			templateData.Title = "User Profile (Error)"
			// User object here should ideally reflect state before problematic update for display consistency,
			// but FormData will hold the submitted (problematic) values.
			tempUserForDisplay := &data.User{ID: user.ID, Name: r.PostForm.Get("name"), Email: originalEmail, CreatedAt: user.CreatedAt}
			templateData.User = tempUserForDisplay
			templateData.FormErrors = v.Errors
			templateData.FormData = map[string]string{
				"name":  r.PostForm.Get("name"),
				"email": r.PostForm.Get("email"),
			}
			templateData.ProfileCurrentPage = 1

			// --- MODIFIED: Render fragment for HTMX on duplicate email error ---
			if r.Header.Get("HX-Request") == "true" {
				app.logger.Info("HTMX: Re-rendering profile content due to duplicate email on update")
				ts, ok := app.templateCache["profile.tmpl"]
				if !ok {
					app.serverError(w, r, fmt.Errorf("template profile.tmpl not found"))
					return
				}
				w.WriteHeader(http.StatusUnprocessableEntity)
				errRender := ts.ExecuteTemplate(w, "profile-content", templateData)
				if errRender != nil {
					app.serverError(w, r, errRender)
				}
			} else {
				errRender := app.render(w, http.StatusUnprocessableEntity, "profile.tmpl", templateData)
				if errRender != nil {
					app.serverError(w, r, errRender)
				}
			}
		} else {
			app.serverError(w, r, err)
		}
		return
	}

	// 9. Success.
	app.session.Put(r, "flash", "Profile updated successfully.")
	// --- MODIFIED: Send HX-Redirect for HTMX success ---
	if r.Header.Get("HX-Request") == "true" {
		app.logger.Info("HTMX: Sending HX-Redirect to /user/profile after profile update")
		w.Header().Set("HX-Redirect", "/user/profile") // Redirect to the profile page (page 1 by default)
		w.WriteHeader(http.StatusOK)
	} else {
		http.Redirect(w, r, "/user/profile", http.StatusSeeOther)
	}
}

// changeUserPassword handles submission for changing the user's password.
// Securely updates a user's password after verifying their current password.
func (app *application) changeUserPassword(w http.ResponseWriter, r *http.Request) {
	// 1. Authentication.
	userID := app.getUserIDFromSession(r)
	if userID == 0 {
		app.clientError(w, http.StatusUnauthorized)
		return
	}

	// 2. Fetch User (needed for current password check and context).
	user, err := app.users.Get(userID)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	// 3. Method Check & Parse Form.
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

	// 4. Extract Passwords from Form.
	currentPassword := r.PostForm.Get("current_password")
	newPassword := r.PostForm.Get("new_password")
	confirmPassword := r.PostForm.Get("confirm_password")

	// 5. Validate Password Fields (strength, match, presence).
	v := validator.NewValidator()
	data.ValidatePasswordUpdate(v, currentPassword, newPassword, confirmPassword)

	// Helper to render password form with errors.
	renderPasswordError := func(formErrors map[string]string) {
		templateData := app.newTemplateData(r)
		templateData.Title = "User Profile (Password Error)"
		templateData.User = user
		templateData.FormErrors = formErrors
		if templateData.FormData == nil {
			templateData.FormData = make(map[string]string)
		}
		templateData.FormData["name"] = user.Name   // Keep user info for context
		templateData.FormData["email"] = user.Email // Keep user info for context
		templateData.ProfileCurrentPage = 1         // Password form is on page 1.

		// --- MODIFIED: Render fragment for HTMX on validation error ---
		if r.Header.Get("HX-Request") == "true" {
			app.logger.Info("HTMX: Re-rendering profile content due to password change validation errors")
			ts, ok := app.templateCache["profile.tmpl"]
			if !ok {
				app.serverError(w, r, fmt.Errorf("template profile.tmpl not found"))
				return
			}
			w.WriteHeader(http.StatusUnprocessableEntity)
			errRender := ts.ExecuteTemplate(w, "profile-content", templateData)
			if errRender != nil {
				app.serverError(w, r, errRender)
			}
		} else {
			errRender := app.render(w, http.StatusUnprocessableEntity, "profile.tmpl", templateData)
			if errRender != nil {
				app.serverError(w, r, errRender)
			}
		}
	}

	// 6. Handle Basic Validation Errors.
	if !v.ValidData() {
		renderPasswordError(v.Errors)
		return
	}

	// 7. Verify Current Password.
	match, err := user.Password.Matches(currentPassword) // Uses bcrypt.CompareHashAndPassword.
	if err != nil {
		app.serverError(w, r, err)
		return
	}
	if !match {
		v.AddError("current_password", "Current password incorrect")
		renderPasswordError(v.Errors)
		return
	}

	// 8. Set New Password Hash on User Struct.
	err = user.Password.Set(newPassword) // Hashes the new password.
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	// 9. Update Password in Database.
	err = app.users.UpdatePassword(user.ID, user.Password.Hash())
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	// 10. Success.
	app.session.Put(r, "flash", "Password updated successfully.")
	// --- MODIFIED: Send HX-Redirect for HTMX success ---
	if r.Header.Get("HX-Request") == "true" {
		app.logger.Info("HTMX: Sending HX-Redirect to /user/profile after password update")
		w.Header().Set("HX-Redirect", "/user/profile")
		w.WriteHeader(http.StatusOK)
	} else {
		http.Redirect(w, r, "/user/profile", http.StatusSeeOther)
	}
}

// resetUserEntries handles the request to delete all mood entries for the current user.
// Data management feature allowing users to clear their mood history.
func (app *application) resetUserEntries(w http.ResponseWriter, r *http.Request) {
	// 1. Authentication.
	userID := app.getUserIDFromSession(r)
	if userID == 0 {
		app.clientError(w, http.StatusUnauthorized)
		return
	}

	// 2. Method Check (should be POST, often triggered by a confirmation button).
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		app.clientError(w, http.StatusMethodNotAllowed)
		return
	}

	// 3. Delete All Moods for UserID: Call MoodModel method.
	err := app.moods.DeleteAllByUserID(userID)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	// 4. Success.
	app.session.Put(r, "flash", "All your mood entries have been reset.")
	// --- MODIFIED: Send HX-Redirect for HTMX success ---
	if r.Header.Get("HX-Request") == "true" {
		app.logger.Info("HTMX: Sending HX-Redirect to /user/profile?page=2 after resetting entries")
		w.Header().Set("HX-Redirect", "/user/profile?page=2")
		w.WriteHeader(http.StatusOK)
	} else {
		// Redirect to the second page of the profile settings.
		http.Redirect(w, r, "/user/profile?page=2", http.StatusSeeOther)
	}
}

// deleteUserAccount handles the permanent deletion of a user's account and all their data.
// Critical data deletion feature. Removes user and associated mood entries.
func (app *application) deleteUserAccount(w http.ResponseWriter, r *http.Request) {
	// 1. Authentication.
	userID := app.getUserIDFromSession(r)
	if userID == 0 {
		app.clientError(w, http.StatusUnauthorized)
		return
	}

	// 2. Method Check.
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		app.clientError(w, http.StatusMethodNotAllowed)
		return
	}

	// 3. Delete User from Database: UserModel's Delete method.
	//    (Database constraints like ON DELETE CASCADE should handle deleting associated moods).
	err := app.users.Delete(userID)
	if err != nil {
		if errors.Is(err, data.ErrRecordNotFound) {
			// User might have already been deleted. Log, but proceed with logout.
			app.logger.Warn("Attempt to delete non-existent user account", "userID", userID)
		} else {
			app.serverError(w, r, err)
			return
		}
	}

	// 4. Log User Out: Clear their session.
	app.session.Remove(r, "authenticatedUserID")
	// 5. Notify and Redirect to Public Page.
	app.session.Put(r, "flash", "Your account has been successfully deleted.")
	if r.Header.Get("HX-Request") == "true" {
		app.logger.Info("HTMX: Sending HX-Redirect to /landing after account deletion")
		w.Header().Set("HX-Redirect", "/landing")
		w.WriteHeader(http.StatusOK)
	} else {
		http.Redirect(w, r, "/landing", http.StatusSeeOther)
	}
}

/*
==========================================================================

	Error Handlers
==========================================================================
*/

// serverError logs detailed error information and sends a generic 500 Internal Server Error response to the client.
// Centralized error handling for unexpected server issues.
func (app *application) serverError(w http.ResponseWriter, r *http.Request, err error) {
	// 1. Log Error Details: Include method, URI, and the actual error message for server-side diagnosis.
	var (
		method = r.Method
		uri    = r.URL.RequestURI()
	)
	app.logger.Error("server error encountered", "error", err.Error(), "method", method, "uri", uri)
	// 2. Check if Headers Already Sent: If response headers have been written, we can't send a new error page.
	//    This prevents "http: superfluous response.WriteHeader call" errors.
	if headersSent := w.Header().Get("Content-Type"); headersSent != "" {
		app.logger.Warn("headers already written, cannot send error response", "sent_content_type", headersSent)
		return
	}
	// 3. Send Generic 500 Response: `http.Error` is a convenient way to do this.
	http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
}

// clientError sends a specific HTTP status code and corresponding text to the client.
// Used for errors caused by the client (e.g., 400 Bad Request, 401 Unauthorized).
// Presentation Point: "Handles errors caused by client actions, like invalid input or unauthorized access."
func (app *application) clientError(w http.ResponseWriter, status int) {
	http.Error(w, http.StatusText(status), status)
}

// notFound is a convenience wrapper around clientError to send a 404 Not Found response.
// Presentation Point: "Specific helper for 404 errors when a resource isn't found."
func (app *application) notFound(w http.ResponseWriter) {
	app.clientError(w, http.StatusNotFound)
}
