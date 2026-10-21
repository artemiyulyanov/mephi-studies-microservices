package cmd

import (
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"

	commondb "common/database"
)

func main() {
	if err := run(); err != nil {
		slog.Error("migrate failed", "err", err)
		os.Exit(1)
	}
}

func run() error {
	dir := flag.String("dir", "migrations", "directory with *.sql migrations")
	cmd := flag.String("cmd", "up", "up | version")
	flag.Parse()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return errors.New("DATABASE_URL is required")
	}

	switch *cmd {
	case "up":
		return commondb.RunMigrations(dsn, *dir)
	case "version":
		v, dirty, err := commondb.MigrationVersion(dsn, *dir)
		if err != nil {
			return err
		}
		slog.Info("current version", "version", v, "dirty", dirty)
		return nil
	default:
		return fmt.Errorf("unknown -cmd %q", *cmd)
	}
}
