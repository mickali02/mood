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

	// === KEEP Existing Home Page Route ===
	// This remains the default page when visiting "/"
	mux.HandleFunc("GET /{$}", app.home) // Root path matches exactly

	// === ADD Route for the NEW Separate Landing Page ===
	mux.HandleFunc("GET /landing", app.showLandingPage) // New page at /landing

	// === ADD Route for the NEW About Page ===
	mux.HandleFunc("GET /about", app.showAboutPage)

	// === EXISTING MOOD ROUTES (Unchanged) ===
	mux.HandleFunc("GET /moods", app.listMoods)
	mux.HandleFunc("GET /mood/new", app.showMoodForm)
	mux.HandleFunc("POST /mood/new", app.createMood)
	mux.HandleFunc("GET /mood/edit/{id}", app.showEditMoodForm)
	mux.HandleFunc("POST /mood/edit/{id}", app.updateMood)
	mux.HandleFunc("POST /mood/delete/{id}", app.deleteMood)

	// --- Middleware ---
	return app.loggingMiddleware(mux)
}
