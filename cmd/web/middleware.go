// mood/cmd/web/middleware.go
package main

import (
	"net/http"

	"github.com/justinas/nosurf"
)

// loggingMiddleware logs details about incoming HTTP requests.
func (app *application) loggingMiddleware(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		app.logger.Info("received request",
			"remote_ip", r.RemoteAddr,
			"proto", r.Proto,
			"method", r.Method,
			"uri", r.URL.RequestURI(),
		)
		next.ServeHTTP(w, r)
	}
	return http.HandlerFunc(fn)
}

// sessionMiddleware loads and saves session data for the current request.
func (app *application) sessionMiddleware(next http.Handler) http.Handler {
	// Use the Enable() middleware provided by golangcollege/sessions
	// This automatically loads and saves session data for the request.
	return app.session.Enable(next)
}

// **** ADD THIS MIDDLEWARE ****
// requireAuthentication checks if a user is authenticated. If not, it redirects
// them to the login page and returns. Otherwise, it calls the next handler.
func (app *application) requireAuthentication(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		// Use the isAuthenticated helper we created earlier.
		if !app.isAuthenticated(r) {
			app.logger.Warn("Authentication required", "uri", r.URL.RequestURI()) // Log attempt

			// Add a flash message to be shown on the login page.
			app.session.Put(r, "flash", "You must be logged in to view this page.")

			// Redirect the user to the login page.
			http.Redirect(w, r, "/user/login", http.StatusFound) // 302 Found
			return                                               // Important: Stop processing the request here.
		}

		// If the user *is* authenticated, call the next handler in the chain.
		// Crucially, this also prevents caching of protected pages by browsers/proxies,
		// as different responses (login page vs actual page) might be served for the same URL.
		w.Header().Add("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	}
	// Wrap the handler function so it satisfies the http.Handler interface.
	return http.HandlerFunc(fn)
}

// noSurf middleware adds CSRF protection to all non-safe methods (POST, PUT, DELETE, etc.)
func noSurf(next http.Handler) http.Handler {
	// Create a new CSRF handler
	csrfHandler := nosurf.New(next)

	// Configure the base cookie settings
	// Ensure Secure is true if using HTTPS (which you are)
	csrfHandler.SetBaseCookie(http.Cookie{
		HttpOnly: true,
		Path:     "/",                  // Available across the entire site
		Secure:   true,                 // Requires HTTPS
		SameSite: http.SameSiteLaxMode, // Standard SameSite setting
		// MaxAge and Domain can be set if needed, but defaults are often fine
	})

	// You can add custom error handling here if desired using csrfHandler.SetFailureHandler()
	// For now, it will return a 403 Forbidden by default on failure.

	return csrfHandler
}

// **** END ADDED MIDDLEWARE ****
