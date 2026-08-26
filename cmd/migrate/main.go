// Command migrate applies or rolls back database migrations. It wraps
// golang-migrate so the project doesn't depend on a separately installed
// CLI: `go run ./cmd/migrate up` is enough in any environment that can
// already build the rest of the module.
package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/stan-ley-tech/medqueue/internal/config"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "migrate:", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) < 2 {
		return fmt.Errorf("usage: migrate [up|down|version|force <version>]")
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	m, err := migrate.New("file://migrations", cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("open migrator: %w", err)
	}
	defer m.Close()

	switch os.Args[1] {
	case "up":
		err = m.Up()
	case "down":
		err = m.Down()
	case "version":
		version, dirty, verr := m.Version()
		if verr != nil {
			return verr
		}
		fmt.Printf("version=%d dirty=%v\n", version, dirty)
		return nil
	case "force":
		if len(os.Args) < 3 {
			return fmt.Errorf("usage: migrate force <version>")
		}
		var version int
		if _, err := fmt.Sscanf(os.Args[2], "%d", &version); err != nil {
			return fmt.Errorf("invalid version: %w", err)
		}
		err = m.Force(version)
	default:
		return fmt.Errorf("unknown command %q", os.Args[1])
	}

	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}
	fmt.Println("migrate:", os.Args[1], "completed")
	return nil
}
