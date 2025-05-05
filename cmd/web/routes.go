// mood/cmd/web/routes.go
package main

import (
	"net/http"
)

func (app *application) routes() http.Handler {
	mux := http.NewServeMux()

	// --- Static Files ---
	fileServer := http.FileServer(http.Dir("./ui/static/"))
	mux.Handle("GET /static/", http.StripPrefix("/static", fileServer))

	// --- Unprotected Application Routes ---
	mux.HandleFunc("GET /{$}", app.showLandingPage)
	mux.HandleFunc("GET /landing", app.showLandingPage)
	mux.HandleFunc("GET /about", app.showAboutPage)
	mux.HandleFunc("GET /user/signup", app.signupUserForm)
	mux.HandleFunc("POST /user/signup", app.signupUser)
	mux.HandleFunc("GET /user/login", app.loginUserForm)
	mux.HandleFunc("POST /user/login", app.loginUser)

	// --- Protected Application Routes ---
	// Apply requireAuthentication middleware
	mux.HandleFunc("GET /dashboard", app.requireAuthentication(http.HandlerFunc(app.showDashboardPage)).ServeHTTP)
	mux.HandleFunc("GET /mood/new", app.requireAuthentication(http.HandlerFunc(app.showMoodForm)).ServeHTTP)
	mux.HandleFunc("POST /mood/new", app.requireAuthentication(http.HandlerFunc(app.createMood)).ServeHTTP)
	mux.HandleFunc("GET /mood/edit/{id}", app.requireAuthentication(http.HandlerFunc(app.showEditMoodForm)).ServeHTTP)
	mux.HandleFunc("POST /mood/edit/{id}", app.requireAuthentication(http.HandlerFunc(app.updateMood)).ServeHTTP)
	mux.HandleFunc("POST /mood/delete/{id}", app.requireAuthentication(http.HandlerFunc(app.deleteMood)).ServeHTTP)
	mux.HandleFunc("GET /stats", app.requireAuthentication(http.HandlerFunc(app.showStatsPage)).ServeHTTP)
	mux.HandleFunc("POST /user/logout", app.requireAuthentication(http.HandlerFunc(app.logoutUser)).ServeHTTP)

	// --- Base Middleware Chain ---
	// Order: Log -> Session -> CSRF -> Mux (with its own per-route middleware)
	// **MODIFIED:** Added noSurf middleware
	standardMiddleware := app.sessionMiddleware(app.loggingMiddleware(mux))
	csrfProtectedMiddleware := noSurf(standardMiddleware) // Apply noSurf globally

	// Return the fully wrapped handler chain.
	return csrfProtectedMiddleware // <-- Return the CSRF protected chain
}
