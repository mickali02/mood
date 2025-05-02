// mood/cmd/web/server.go
package main

import (
	"crypto/tls" // Ensure this import is present
	"log/slog"
	"net/http"
	"time"
)

// serve configures and starts the application's HTTP server.
func (app *application) serve() error {

	// --- Define Advanced TLS Configuration ---
	// This struct allows customizing TLS settings like cipher suites and minimum version.
	tlsConfig := &tls.Config{
		// Prioritize modern elliptic curves for key exchange (good for performance and security)
		CurvePreferences: []tls.CurveID{tls.X25519, tls.CurveP256},
		// Set the minimum acceptable TLS version (TLS 1.2 is a common baseline)
		MinVersion: tls.VersionTLS12,
		// Define a preferred list of strong cipher suites. The server will try to negotiate
		// one of these, preferring the ones listed earlier.
		CipherSuites: []uint16{
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305, // Recommended for performance/security
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,   // Recommended for performance/security
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			// Add older suites here if compatibility is needed, but prioritize the above.
		},
		// PreferServerCipherSuites: true, // Go >= 1.17 generally prefers server suites by default when CipherSuites is set. Explicitly setting can ensure behavior.
	}
	// --- End TLS Configuration ---

	// Configure the http.Server with address, handler, error logger, timeouts, and the TLS config.
	srv := &http.Server{
		Addr:         app.addr,                                                 // The address to listen on (e.g., ":4000")
		Handler:      app.routes(),                                             // The main application router/handler
		ErrorLog:     slog.NewLogLogger(app.logger.Handler(), slog.LevelError), // Use structured logger for server errors
		IdleTimeout:  time.Minute,                                              // Max time for connections to stay idle
		ReadTimeout:  5 * time.Second,                                          // Max time to read the entire request
		WriteTimeout: 10 * time.Second,                                         // Max time to write the entire response
		TLSConfig:    tlsConfig,                                                // <-- Assign the custom TLS configuration
	}

	// Log the server start address (message now indicates HTTPS)
	app.logger.Info("starting HTTPS server with advanced TLS config", slog.String("addr", srv.Addr))

	// Start the HTTPS server using ListenAndServeTLS.
	// This method uses the Addr and TLSConfig defined in the srv struct.
	// It requires the paths to the certificate and key files.
	err := srv.ListenAndServeTLS("./tls/cert.pem", "./tls/key.pem")
	// We don't need to log the Fatal error here as main.go handles it if serve() returns an error.
	return err // Return the error to main.go
}
