// mood/cmd/web/main.go
package main

import (
	"context"
	"database/sql"
	"flag"
	"html/template"
	"log/slog"
	"net/http"
	"os"
	"time"

	_ "github.com/lib/pq"

	"github.com/golangcollege/sessions"
	"github.com/mickali02/mood/internal/data"
)

// application struct holds application-wide dependencies.
type application struct {
	logger        *slog.Logger
	addr          string
	moods         *data.MoodModel // Existing MoodModel
	users         *data.UserModel // <-- UserModel field (already present in your provided code)
	templateCache map[string]*template.Template
	session       *sessions.Session // Existing session field
}

func main() {
	// --- Configuration ---
	addr := flag.String("addr", ":4000", "HTTP network address")
	dsn := flag.String("dsn", os.Getenv("MOODNOTES_DB_DSN"), "PostgreSQL DSN (reads MOODNOTES_DB_DSN env var)")
	secret := flag.String("secret", "Gm9zN!cRz&7$eL4qjV1@xPu!Zw5#Tb6K", "Secret key (must be 32 bytes)")
	flag.Parse()

	// --- Logging ---
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// --- Validate Secret Key Length ---
	// A quick but important security check: ensure the session secret key is the correct length (32 bytes).

	if len(*secret) != 32 {
		logger.Error("secret key must be exactly 32 bytes long", slog.Int("length", len(*secret)))
		os.Exit(1)
	}

	// --- Database Connection ---
	// Establish a connection to our PostgreSQL database using the DSN.
	// The `openDB` helper configures the connection pool for optimal performance.
	if *dsn == "" {
		logger.Error("database DSN must be provided via -dsn flag or MOODNOTES_DB_DSN environment variable")
		os.Exit(1)
	}
	db, err := openDB(*dsn) // Call helper to open and configure DB pool.
	if err != nil {
		logger.Error("failed to connect to database", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer db.Close() // Ensure database connection is closed when main exits.
	logger.Info("database connection pool established")

	// --- Template Cache ---
	// To improve performance, HTML templates are parsed once at startup
	// and stored in a cache. This avoids re-parsing on every request.
	templateCache, err := newTemplateCache() // `newTemplateCache` (in templates.go) loads and parses HTML files.
	if err != nil {
		logger.Error("failed to build template cache", slog.String("error", err.Error()))
		os.Exit(1)
	}
	logger.Info("template cache built successfully")

	// --- Session Manager Initialization ---
	// The session manager is configured here. We set a secret key for security,
	// define a session lifetime (12 hours), and set cookie attributes like Secure, HttpOnly, and SameSite
	// for better security and CSRF protection.
	sessionManager := sessions.New([]byte(*secret))
	sessionManager.Lifetime = 12 * time.Hour
	sessionManager.Secure = true
	sessionManager.HttpOnly = true
	sessionManager.SameSite = http.SameSiteLaxMode

	logger.Info("session manager initialized")

	// --- Application Dependencies Injection ---
	// All initialized components (logger, database models, template cache, session manager)
	// are then bundled into our `application` struct. This struct is passed to our HTTP handlers,
	// giving them access to these shared resources – this is a form of dependency injection.
	app := &application{
		logger:        logger,
		addr:          *addr,
		moods:         &data.MoodModel{DB: db}, // Initialize MoodModel
		users:         &data.UserModel{DB: db}, // <-- Initialize UserModel, passing db
		templateCache: templateCache,           // Initialize Template Cache
		session:       sessionManager,          // Initialize Session Manager
	}

	// --- Start Server ---
	// Start the HTTP server using the `app.serve()` method (defined in server.go),
	// which sets up routing and listens for incoming requests on the configured address.
	logger.Info("starting server", slog.String("addr", app.addr))
	err = app.serve() // `app.serve()` configures and starts the HTTPS server.
	if err != nil {   // If server fails to start.
		logger.Error("server failed to start", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

// openDB establishes and configures a database connection pool.
// This helper function connects to PostgreSQL and configures the connection pool settings
// like max open connections and idle timeouts, crucial for robust database interaction.
func openDB(dsn string) (*sql.DB, error) {
	// 1. Open Connection: `sql.Open` doesn't immediately create a connection, just prepares it.
	db, err := sql.Open("postgres", dsn) // "postgres" is the driver name.
	if err != nil {
		return nil, err
	}

	// 2. Configure Connection Pool:
	//    These settings help manage database resources efficiently.
	db.SetMaxOpenConns(25)                 // Max number of open connections to the database.
	db.SetMaxIdleConns(25)                 // Max number of connections in the idle connection pool.
	db.SetConnMaxIdleTime(5 * time.Minute) // Max amount of time a connection may be idle.
	db.SetConnMaxLifetime(2 * time.Hour)   // Max amount of time a connection may be reused.

	// 3. Verify Connection: `PingContext` attempts to connect to the database to ensure it's reachable.
	//    A timeout is used to prevent indefinite blocking.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel() // Release resources associated with the context.

	err = db.PingContext(ctx)
	if err != nil {
		db.Close() // If ping fails, close the db object before returning.
		return nil, err
	}

	return db, nil // Return the configured and verified database pool.

}
