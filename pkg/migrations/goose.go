package migrations

import (
	"database/sql"
	"embed"

	_ "github.com/lib/pq"
	"github.com/pressly/goose/v3"
)

// ApplyMigrations runs all pending Goose migrations from the given embedded FS.
// The FS must contain a "migrations" directory with .sql files.
func ApplyMigrations(connStr string, migrationFS embed.FS) error {
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return err
	}
	defer db.Close()

	goose.SetBaseFS(migrationFS)
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}

	if err := goose.Up(db, "migrations"); err != nil {
		return err
	}
	return nil
}
