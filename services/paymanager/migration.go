package api

import (
	"embed"

	"TestTaskJustPay/pkg/migrations"
)

func ApplyMigrations(connStr string, migrationFS embed.FS) error {
	return migrations.ApplyMigrations(connStr, migrationFS)
}
