// Package testutil provides shared setup for integration, e2e, and load
// tests: a migrated PostgreSQL connection and Redis client pointed at
// whatever TEST_DATABASE_URL / TEST_REDIS_ADDR describe (docker-compose
// locally, service containers in CI).
package testutil

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/stan-ley-tech/medqueue/internal/cache"
	"github.com/stan-ley-tech/medqueue/internal/db"
)

const (
	defaultTestDatabaseURL = "postgres://medqueue:medqueue@localhost:5432/medqueue_test?sslmode=disable"
	defaultTestRedisAddr   = "localhost:6379"
)

// RequireDB connects to the test database, applies all migrations, and
// returns a pool. Tests calling this must be run with a live Postgres
// reachable at TEST_DATABASE_URL (or the local default); use `go test
// -tags=integration` via `make test-integration` which assumes docker
// compose's postgres service is up.
func RequireDB(t *testing.T) *db.Pool {
	t.Helper()

	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		url = defaultTestDatabaseURL
	}

	if err := applyMigrations(url); err != nil {
		t.Fatalf("testutil: apply migrations: %v", err)
	}

	pool, err := db.Connect(context.Background(), db.Options{URL: url})
	if err != nil {
		t.Fatalf("testutil: connect to test database: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// RequireRedis connects to the test Redis instance.
func RequireRedis(t *testing.T) *cache.Client {
	t.Helper()

	addr := os.Getenv("TEST_REDIS_ADDR")
	if addr == "" {
		addr = defaultTestRedisAddr
	}

	client, err := cache.Connect(context.Background(), cache.Options{Addr: addr})
	if err != nil {
		t.Fatalf("testutil: connect to test redis: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func applyMigrations(databaseURL string) error {
	m, err := migrate.New(migrationsSourceURL(), databaseURL)
	if err != nil {
		return err
	}
	defer m.Close()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return err
	}
	return nil
}

// migrationsSourceURL resolves the migrations directory relative to the
// repository root regardless of which test package's directory `go test`
// happens to run from, and regardless of platform.
//
// Deliberately uses "file:" with a single colon, not "file://" with a
// double slash: golang-migrate's file source driver reconstructs the
// path from url.Host+url.Path, and net/url parses anything after "file:"
// that starts with "/" as a path (so a POSIX absolute path like
// "/home/x/migrations" round-trips correctly) but parses a Windows drive
// path like "E:/MedQueue/migrations" — which doesn't start with "/" — as
// Opaque instead, since "E:" would otherwise be misread as a URL
// host:port. golang-migrate falls back to Opaque when Host+Path is
// empty, so this single form handles both platforms.
func migrationsSourceURL() string {
	slashed := filepath.ToSlash(filepath.Join(findRepoRoot(), "migrations"))
	return "file:" + slashed
}

func findRepoRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "."
		}
		dir = parent
	}
}
