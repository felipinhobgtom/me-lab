package sqlite

import (
	"database/sql"
	"fmt"
	"time"

	// Pure-Go SQLite driver.
	_ "modernc.org/sqlite"
)

// NewDB opens a SQLite database at the given path and configures it for
// production use: WAL journal mode, foreign key enforcement, and a busy
// timeout so concurrent readers/writers don't immediately fail.
func NewDB(databaseURL string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("sqlite open: %w", err)
	}

	// SQLite only supports a single writer, so limit connections accordingly.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(time.Hour)

	// Enable WAL mode for better concurrent read performance.
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("sqlite pragma journal_mode: %w", err)
	}

	// Enforce foreign key constraints.
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("sqlite pragma foreign_keys: %w", err)
	}

	// Set busy timeout to 5 seconds to avoid SQLITE_BUSY errors under load.
	if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
		db.Close()
		return nil, fmt.Errorf("sqlite pragma busy_timeout: %w", err)
	}

	// Verify the connection is alive.
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("sqlite ping: %w", err)
	}

	return db, nil
}
