// mood/cmd/web/routes.go
package main

import (
	"net/http"
)

// routes defines and returns the HTTP request multiplexer (router).
func (app *application) routes() http.Handler {
	mux := http.NewServeMux()

	// --- Static Files ---
	// Serves files from the ./ui/static directory.
	fileServer := http.FileServer(http.Dir("./ui/static/"))
	mux.Handle("GET /static/", http.StripPrefix("/static", fileServer)) // Use GET explicitly

	// --- Unprotected Application Routes ---
	// These routes are accessible to anyone, logged in or not.
	mux.HandleFunc("GET /{$}", app.showLandingPage)        // Root path
	mux.HandleFunc("GET /landing", app.showLandingPage)    // Landing page
	mux.HandleFunc("GET /about", app.showAboutPage)        // About page
	mux.HandleFunc("GET /user/signup", app.signupUserForm) // Display signup form
	mux.HandleFunc("POST /user/signup", app.signupUser)    // Process signup form
	mux.HandleFunc("GET /user/login", app.loginUserForm)   // Display login form
	mux.HandleFunc("POST /user/login", app.loginUser)      // Process login form

	// --- Protected Application Routes ---
	// These routes require the user to be authenticated.
	// We wrap the final handler with the requireAuthentication middleware.

	// Dashboard
	mux.HandleFunc("GET /dashboard", app.requireAuthentication(http.HandlerFunc(app.showDashboardPage)).ServeHTTP)

	// Mood Entry Management
	mux.HandleFunc("GET /mood/new", app.requireAuthentication(http.HandlerFunc(app.showMoodForm)).ServeHTTP)
	mux.HandleFunc("POST /mood/new", app.requireAuthentication(http.HandlerFunc(app.createMood)).ServeHTTP)
	mux.HandleFunc("GET /mood/edit/{id}", app.requireAuthentication(http.HandlerFunc(app.showEditMoodForm)).ServeHTTP)
	mux.HandleFunc("POST /mood/edit/{id}", app.requireAuthentication(http.HandlerFunc(app.updateMood)).ServeHTTP)
	mux.HandleFunc("POST /mood/delete/{id}", app.requireAuthentication(http.HandlerFunc(app.deleteMood)).ServeHTTP)

	// Statistics Page
	mux.HandleFunc("GET /stats", app.requireAuthentication(http.HandlerFunc(app.showStatsPage)).ServeHTTP)

	// Logout Action (Requires being logged in to perform the action)
	mux.HandleFunc("POST /user/logout", app.requireAuthentication(http.HandlerFunc(app.logoutUser)).ServeHTTP)

	// --- Base Middleware Chain ---
	// Apply session management first, then logging to ALL routes defined above.
	// The requireAuthentication middleware has already been applied specifically to protected routes.
	standardMiddleware := app.sessionMiddleware(app.loggingMiddleware(mux))

	// Return the fully wrapped handler chain.
	return standardMiddleware
}
