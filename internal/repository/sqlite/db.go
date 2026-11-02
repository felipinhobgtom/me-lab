package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/felipinho/me-lab/internal/catalog"
	"github.com/felipinho/me-lab/migrations"
	"github.com/pressly/goose/v3"
	// Pure-Go SQLite driver.
	_ "modernc.org/sqlite"
)

var migrateMu sync.Mutex

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

// Migrate applies all embedded SQL migrations to db using goose.
func Migrate(db *sql.DB) error {
	migrateMu.Lock()
	defer migrateMu.Unlock()

	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("goose set dialect: %w", err)
	}
	if err := goose.Up(db, "."); err != nil {
		return fmt.Errorf("goose up: %w", err)
	}
	return nil
}

// SeedCatalog upserts all compatibility profiles and block definitions from reg
// into SQLite inside a single transaction.
func SeedCatalog(ctx context.Context, db *sql.DB, reg *catalog.Registry) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin catalog seed tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	const upsertProfileSQL = `
		INSERT INTO compatibility_profiles (id, name, minecraft_version, ae2_version, channel_mode, config_json)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			minecraft_version = excluded.minecraft_version,
			ae2_version = excluded.ae2_version,
			channel_mode = excluded.channel_mode,
			config_json = excluded.config_json`

	profileStmt, err := tx.PrepareContext(ctx, upsertProfileSQL)
	if err != nil {
		return fmt.Errorf("prepare profile upsert: %w", err)
	}
	defer profileStmt.Close()

	for _, p := range reg.Profiles() {
		cfgJSON, err := json.Marshal(p.Config)
		if err != nil {
			return fmt.Errorf("marshal profile %s config: %w", p.ID, err)
		}
		if _, err := profileStmt.ExecContext(ctx, p.ID, p.Name, p.MinecraftVersion, p.AE2Version, p.ChannelMode, string(cfgJSON)); err != nil {
			return fmt.Errorf("upsert profile %s: %w", p.ID, err)
		}
	}

	const upsertBlockSQL = `
		INSERT INTO block_definitions (id, profile_id, name, category, metadata_json, behavior_key, fidelity_label, doc_reference)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id, profile_id) DO UPDATE SET
			name = excluded.name,
			category = excluded.category,
			metadata_json = excluded.metadata_json,
			behavior_key = excluded.behavior_key,
			fidelity_label = excluded.fidelity_label,
			doc_reference = excluded.doc_reference`

	blockStmt, err := tx.PrepareContext(ctx, upsertBlockSQL)
	if err != nil {
		return fmt.Errorf("prepare block upsert: %w", err)
	}
	defer blockStmt.Close()

	for _, b := range reg.AllBlocks() {
		metaJSON, err := json.Marshal(b.Metadata)
		if err != nil {
			return fmt.Errorf("marshal block %s metadata: %w", b.ID, err)
		}
		if _, err := blockStmt.ExecContext(
			ctx,
			b.ID,
			b.ProfileID,
			b.Name,
			b.Category,
			string(metaJSON),
			b.BehaviorKey,
			string(b.Fidelity),
			b.DocReference,
		); err != nil {
			return fmt.Errorf("upsert block (%s, %s): %w", b.ProfileID, b.ID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit catalog seed: %w", err)
	}
	return nil
}
