package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/felipinho/me-lab/internal/repository/sqlite"
	"github.com/felipinho/me-lab/migrations"
	"github.com/pressly/goose/v3"
)

func main() {
	defaultDB := os.Getenv("DATABASE_URL")
	if defaultDB == "" {
		defaultDB = "me-lab.db"
	}

	dbURL := flag.String("db", defaultDB, "SQLite database path or DSN")
	migrationsDir := flag.String("dir", "migrations", "Filesystem migrations directory (used by create)")
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		usage()
		os.Exit(1)
	}

	cmd := args[0]
	if err := runCommand(cmd, *dbURL, *migrationsDir, args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "migrate %s failed: %v\n", cmd, err)
		os.Exit(1)
	}
}

func runCommand(cmd, dbURL, migrationsDir string, args []string) error {
	db, err := sqlite.NewDB(dbURL)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("set goose dialect: %w", err)
	}

	switch cmd {
	case "up":
		goose.SetBaseFS(migrations.FS)
		return goose.Up(db, ".")

	case "down":
		goose.SetBaseFS(migrations.FS)
		return goose.Down(db, ".")

	case "status":
		goose.SetBaseFS(migrations.FS)
		return goose.Status(db, ".")

	case "create":
		if len(args) < 1 || args[0] == "" {
			return fmt.Errorf("create requires a migration name: migrate create <name> [sql|go]")
		}
		migType := "sql"
		if len(args) >= 2 && args[1] != "" {
			migType = args[1]
		}
		goose.SetBaseFS(nil)
		goose.SetSequential(true)
		return goose.Create(db, migrationsDir, args[0], migType)

	default:
		usage()
		return fmt.Errorf("unknown command %q (supported: up, down, status, create)", cmd)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "Usage: migrate [-db <path>] [-dir <migrations_dir>] <up|down|status|create> [args...]")
}
