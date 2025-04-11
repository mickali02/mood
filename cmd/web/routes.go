// mood/cmd/web/routes.go
package main

import (
	"net/http"
)

// routes defines and returns the HTTP request multiplexer (router).
func (app *application) routes() http.Handler {
	mux := http.NewServeMux() // Using standard library mux

	// --- Static Files ---
	// Serve files from the "./ui/static/" directory.
	fileServer := http.FileServer(http.Dir("./ui/static/"))
	// Route requests for /static/ paths, stripping the /static prefix.
	mux.Handle("GET /static/", http.StripPrefix("/static", fileServer))

	// --- Application Routes ---

	// Optional Home Page
	mux.HandleFunc("GET /{$}", app.home) // Root path matches exactly

	// Mood Routes (CRUD)
	mux.HandleFunc("GET /moods", app.listMoods)                 // List all moods
	mux.HandleFunc("GET /mood/new", app.showMoodForm)           // Show form to create new mood (Changed from /mood)
	mux.HandleFunc("POST /mood/new", app.createMood)            // Handle creation of new mood (Changed from /mood)
	mux.HandleFunc("GET /mood/edit/{id}", app.showEditMoodForm) // Show form to edit mood
	mux.HandleFunc("POST /mood/edit/{id}", app.updateMood)      // Handle update of existing mood
	mux.HandleFunc("POST /mood/delete/{id}", app.deleteMood)    // Handle deletion of mood

	// --- Middleware ---
	// Wrap the mux with middleware. Logging first, then others if needed.
	return app.loggingMiddleware(mux) // Defined in middleware.go
}
