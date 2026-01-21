package database

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

type Migrator struct {
	db *DB
}

func NewMigrator(db *DB) *Migrator {
	return &Migrator{db: db}
}

func (m *Migrator) Run(ctx context.Context) error {
	// Get all migration files
	files, err := m.getMigrationFiles()
	if err != nil {
		return fmt.Errorf("failed to get migration files: %w", err)
	}

	// Execute each migration
	for _, file := range files {
		if err := m.executeMigration(ctx, file); err != nil {
			return fmt.Errorf("failed to execute migration %s: %w", file, err)
		}
	}

	return nil
}

func (m *Migrator) getMigrationFiles() ([]string, error) {
	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		return nil, err
	}

	var files []string
	for _, entry := range entries {
		if ! entry.IsDir() && strings.HasSuffix(entry.Name(), ".sql") {
			files = append(files, entry.Name())
		}
	}

	sort.Strings(files)
	return files, nil
}

func (m *Migrator) executeMigration(ctx context.Context, filename string) error {
	content, err := fs.ReadFile(migrationsFS, "migrations/"+filename)
	if err != nil {
		return fmt.Errorf("failed to read migration file: %w", err)
	}

	fmt.Printf("Executing migration: %s\n", filename)

	_, err = m.db.ExecContext(ctx, string(content))
	if err != nil {
		return fmt.Errorf("failed to execute SQL:  %w", err)
	}

	return nil
}

func (m *Migrator) RunFile(ctx context.Context, filename string) error {
	return m.executeMigration(ctx, filename)
}