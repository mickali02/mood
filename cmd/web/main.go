// mood/cmd/web/main.go
package main

import (
	"context"
	"database/sql"
	"flag"
	"html/template"
	"log/slog"
	"os"
	"time"

	_ "github.com/lib/pq"

	"github.com/mickali02/mood/internal/data"
)

// application struct holds application-wide dependencies.
type application struct {
	logger        *slog.Logger
	addr          string
	moods         *data.MoodModel
	templateCache map[string]*template.Template
}

func main() {
	// --- Configuration ---
	addr := flag.String("addr", ":4000", "HTTP network address")
	// Read DSN from environment variable (defined in .envrc)
	dsn := flag.String("dsn", os.Getenv("MOODNOTES_DB_DSN"), "PostgreSQL DSN (reads MOODNOTES_DB_DSN env var)")
	flag.Parse()

	// --- Logging ---
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))

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
	templateCache, err := newTemplateCache() // Defined in templates.go
	if err != nil {
		logger.Error("failed to build template cache", slog.String("error", err.Error()))
		os.Exit(1)
	}
	logger.Info("template cache built successfully")

	// --- Application Dependencies ---
	app := &application{
		logger:        logger,
		addr:          *addr,
		moods:         &data.MoodModel{DB: db}, // Inject DB into MoodModel
		templateCache: templateCache,
	}

	// --- Start Server ---
	logger.Info("starting server", slog.String("addr", app.addr))
	err = app.serve() // Defined in server.go
	if err != nil {
		logger.Error("server failed to start", slog.String("error", err.Error()))
		os.Exit(1)
	}
}

// openDB connects to the database and verifies the connection.
func openDB(dsn string) (*sql.DB, error) {
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}

	// Configure connection pool settings (optional but recommended)
	db.SetMaxOpenConns(25)                 // Max number of open connections
	db.SetMaxIdleConns(25)                 // Max number of idle connections
	db.SetConnMaxIdleTime(5 * time.Minute) // Max time an idle connection is kept
	db.SetConnMaxLifetime(2 * time.Hour)   // Max time a connection can be reused

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = db.PingContext(ctx)
	if err != nil {
		db.Close() // Close pool if ping fails
		return nil, err
	}

	return db, nil
}
