// mood/cmd/web/main.go
package main

import (
	"context"
	"database/sql"
	"flag"
	"html/template"
	"log/slog"
	"net/http" // Import net/http for http.Server options potentially (though not strictly needed here yet)
	"os"
	"time" // Import time package

	_ "github.com/lib/pq"

	"github.com/golangcollege/sessions" // <-- Import the sessions package
	"github.com/mickali02/mood/internal/data"
)

// application struct holds application-wide dependencies.
type application struct {
	logger        *slog.Logger
	addr          string
	moods         *data.MoodModel
	templateCache map[string]*template.Template
	session       *sessions.Session // <-- Add session field
}

func main() {
	// --- Configuration ---
	addr := flag.String("addr", ":4000", "HTTP network address")
	dsn := flag.String("dsn", os.Getenv("MOODNOTES_DB_DSN"), "PostgreSQL DSN (reads MOODNOTES_DB_DSN env var)")
	// Define a flag for the secret key
	secret := flag.String("secret", "Gm9zN!cRz&7$eL4qjV1@xPu!Zw5#Tb6K", "Secret key (must be 32 bytes)") // <-- Add secret flag
	flag.Parse()

	// --- Logging ---
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// --- Validate Secret Key Length ---
	if len(*secret) != 32 {
		logger.Error("secret key must be exactly 32 bytes long", slog.Int("length", len(*secret)))
		os.Exit(1)
	}

	// --- Database Connection ---
	if *dsn == "" {
		logger.Error("database DSN must be provided via -dsn flag or MOODNOTES_DB_DSN environment variable")
		os.Exit(1)
	}
	db, err := openDB(*dsn)
	if err != nil {
		logger.Error("failed to connect to database", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer db.Close()
	logger.Info("database connection pool established")

	// --- Template Cache ---
	templateCache, err := newTemplateCache()
	if err != nil {
		logger.Error("failed to build template cache", slog.String("error", err.Error()))
		os.Exit(1)
	}
	logger.Info("template cache built successfully")

	// --- Session Manager Initialization ---
	sessionManager := sessions.New([]byte(*secret))
	sessionManager.Lifetime = 12 * time.Hour
	sessionManager.Secure = true // <--- Make sure this line is present and set to true
	sessionManager.HttpOnly = true
	sessionManager.SameSite = http.SameSiteLaxMode

	logger.Info("session manager initialized")

	// --- Application Dependencies ---
	app := &application{
		logger:        logger,
		addr:          *addr,
		moods:         &data.MoodModel{DB: db},
		templateCache: templateCache,
		session:       sessionManager, // <-- Inject session manager
	}

	// --- Start Server ---
	logger.Info("starting server", slog.String("addr", app.addr))
	err = app.serve()
	if err != nil {
		logger.Error("server failed to start", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

// openDB function remains unchanged...
func openDB(dsn string) (*sql.DB, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxIdleTime(5 * time.Minute)
	db.SetConnMaxLifetime(2 * time.Hour)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = db.PingContext(ctx)
	if err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}
