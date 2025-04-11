// mood/cmd/web/middleware.go
package main

import (
	"net/http"
)

// loggingMiddleware logs details about incoming HTTP requests.
func (app *application) loggingMiddleware(next http.Handler) http.Handler {
	// Use http.HandlerFunc to convert a function into an http.Handler
	fn := func(w http.ResponseWriter, r *http.Request) {
		// Log request details before passing to the next handler
		app.logger.Info("received request",
			"remote_ip", r.RemoteAddr,
			"proto", r.Proto,
			"method", r.Method,
			"uri", r.URL.RequestURI(), // Includes path and query string
		)

		// Call the next handler in the chain
		next.ServeHTTP(w, r)

		// You could add logging after the request is handled too, e.g., status code
		// Need to wrap ResponseWriter to capture status, which adds complexity.
		// app.logger.Info("finished request", "uri", r.URL.RequestURI())
	}

	// Return the HandlerFunc
	return http.HandlerFunc(fn)
}

// Add other middleware here if needed (e.g., authentication, CORS)
// func (app *application) authenticate(next http.Handler) http.Handler { ... }
