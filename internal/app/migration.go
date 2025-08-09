package app

import (
	"database/sql"
	"embed"

	_ "github.com/lib/pq"
	"github.com/pressly/goose/v3"
)

func ApplyMigrations(connStr string, migrationFS embed.FS) error {
	var db *sql.DB
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return err
	}

	goose.SetBaseFS(migrationFS)
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}

	if err := goose.Up(db, "migrations"); err != nil {
		return err
	}
	return nil
}
