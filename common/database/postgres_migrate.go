package database

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func newMigrate(connectionString, migrationsPath string) (*migrate.Migrate, error) {
	m, err := migrate.New("file://"+migrationsPath, connectionString)
	if err != nil {
		return nil, fmt.Errorf("init migrate: %w", err)
	}
	return m, nil
}

func closeMigrate(m *migrate.Migrate) {
	if srcErr, dbErr := m.Close(); srcErr != nil || dbErr != nil {
		slog.Warn("close migrate", "source_err", srcErr, "db_err", dbErr)
	}
}

func RunMigrations(connectionString, migrationsPath string) error {
	m, err := newMigrate(connectionString, migrationsPath)
	if err != nil {
		return err
	}
	defer closeMigrate(m)

	if err := m.Up(); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			slog.Info("migrations: no changes to apply")
			return nil
		}
		return fmt.Errorf("apply migrations: %w", err)
	}

	v, dirty, _ := m.Version()
	slog.Info("migrations applied", "version", v, "dirty", dirty)
	return nil
}

func MigrationVersion(connectionString, migrationsPath string) (version uint, dirty bool, err error) {
	m, err := newMigrate(connectionString, migrationsPath)
	if err != nil {
		return 0, false, err
	}
	defer closeMigrate(m)

	version, dirty, err = m.Version()
	if errors.Is(err, migrate.ErrNilVersion) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("read version: %w", err)
	}
	return version, dirty, nil
}
