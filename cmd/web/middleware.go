// mood/cmd/web/middleware.go
package main

import (
	"net/http"
)

// loggingMiddleware logs details about incoming HTTP requests.
func (app *application) loggingMiddleware(next http.Handler) http.Handler {
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

	}

	// Return the HandlerFunc
	return http.HandlerFunc(fn)
}
