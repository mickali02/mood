// mood/cmd/web/routes.go
package main

import (
	"net/http"
)

// routes defines and returns the HTTP request multiplexer (router).
func (app *application) routes() http.Handler {
	mux := http.NewServeMux()

	// --- Static Files ---
	fileServer := http.FileServer(http.Dir("./ui/static/"))
	mux.Handle("GET /static/", http.StripPrefix("/static", fileServer))

	// --- Application Routes (Moods) ---
	mux.HandleFunc("GET /{$}", app.showLandingPage) // Or redirect to login/dashboard later
	mux.HandleFunc("GET /landing", app.showLandingPage)
	mux.HandleFunc("GET /about", app.showAboutPage)
	mux.HandleFunc("GET /dashboard", app.showDashboardPage)     // Will be protected later
	mux.HandleFunc("GET /mood/new", app.showMoodForm)           // Will be protected later
	mux.HandleFunc("POST /mood/new", app.createMood)            // Will be protected later
	mux.HandleFunc("GET /mood/edit/{id}", app.showEditMoodForm) // Will be protected later
	mux.HandleFunc("POST /mood/edit/{id}", app.updateMood)      // Will be protected later
	mux.HandleFunc("POST /mood/delete/{id}", app.deleteMood)    // Will be protected later
	mux.HandleFunc("GET /stats", app.showStatsPage)             // Will be protected later

	// --- NEW: User Authentication Routes ---
	mux.HandleFunc("GET /user/signup", app.signupUserForm) // Shows the signup form
	mux.HandleFunc("POST /user/signup", app.signupUser)    // Handles signup form submission
	mux.HandleFunc("GET /user/login", app.loginUserForm)   // Shows the login form
	mux.HandleFunc("POST /user/login", app.loginUser)      // Handles login form submission
	mux.HandleFunc("POST /user/logout", app.logoutUser)    // Handles logout action
	// --- END NEW ---

	// --- Middleware Chain ---
	// Apply middleware: log first, then manage session state for all mux routes.
	return app.sessionMiddleware(app.loggingMiddleware(mux))
}
