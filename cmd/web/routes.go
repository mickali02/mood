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

	// --- Application Routes ---
	// (Existing routes remain the same)
	mux.HandleFunc("GET /{$}", app.showLandingPage)
	mux.HandleFunc("GET /landing", app.showLandingPage)
	mux.HandleFunc("GET /about", app.showAboutPage)
	mux.HandleFunc("GET /dashboard", app.showDashboardPage)
	mux.HandleFunc("GET /mood/new", app.showMoodForm)
	mux.HandleFunc("POST /mood/new", app.createMood)
	mux.HandleFunc("GET /mood/edit/{id}", app.showEditMoodForm)
	mux.HandleFunc("POST /mood/edit/{id}", app.updateMood)
	mux.HandleFunc("POST /mood/delete/{id}", app.deleteMood)

	// --- Middleware Chain ---
	// Apply middleware: log first, then manage session state for all mux routes.
	// Static files bypass this chain as they are handled separately above.
	return app.sessionMiddleware(app.loggingMiddleware(mux)) // <-- Apply session middleware
}
