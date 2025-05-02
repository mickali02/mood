// mood/cmd/web/middleware.go
package main

import (
	"net/http"
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
