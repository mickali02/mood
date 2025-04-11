// mood/cmd/web/server.go
package main

import (
	"log/slog"
	"net/http"
	"time"
)

// serve configures and starts the application's HTTP server.
func (app *application) serve() error {
	srv := &http.Server{
		Addr:    app.addr,
		Handler: app.routes(),
		// Use the application's structured logger for server errors
		ErrorLog: slog.NewLogLogger(app.logger.Handler(), slog.LevelError),
		// Set timeouts to prevent Slowloris attacks and manage resources
		IdleTimeout:  time.Minute,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	// Log the server start address is handled in main.go before calling serve()

	// Start the HTTP server. ListenAndServe blocks until the server stops
	// or encounters an unrecoverable error.
	return srv.ListenAndServe()
}
