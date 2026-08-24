package testutil

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/jackc/pgx/v5/stdlib"
	testcontainerspostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

// StartPostgresContainer starts a Postgres container and returns the connection string
func StartPostgresContainer(ctx context.Context, t *testing.T) string {
	pgContainer, err := testcontainerspostgres.Run(ctx,
		"postgres:15-alpine",
		testcontainerspostgres.WithDatabase("testdb"),
		testcontainerspostgres.WithUsername("postgres"),
		testcontainerspostgres.WithPassword("postgres"),
		testcontainerspostgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Fatalf("failed to start postgres container: %v", err)
	}

	t.Cleanup(func() {
		if err := pgContainer.Terminate(context.Background()); err != nil {
			t.Fatalf("failed to terminate postgres container: %v", err)
		}
	})

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("failed to get connection string: %v", err)
	}

	return connStr
}

// RunMigrations runs all migrations up on the given database
func RunMigrations(t *testing.T, db *sql.DB) {
	_, b, _, _ := runtime.Caller(0)
	basepath := filepath.Dir(b)
	migrationsPath := filepath.Join(basepath, "..", "..", "db", "migrations")

	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		t.Fatalf("failed to create migrate driver: %v", err)
	}

	m, err := migrate.NewWithDatabaseInstance(
		fmt.Sprintf("file://%s", migrationsPath),
		"postgres", driver)
	if err != nil {
		t.Fatalf("failed to create migrate instance: %v", err)
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		t.Fatalf("failed to run migrations: %v", err)
	}
}

// SetupTestDB starts postgres, runs migrations, and returns an active *sql.DB
func SetupTestDB(t *testing.T) *sql.DB {
	ctx := context.Background()
	connStr := StartPostgresContainer(ctx, t)

	db, err := sql.Open("pgx", connStr)
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}

	t.Cleanup(func() {
		db.Close()
	})

	// Wait for db to be actually ready
	for i := 0; i < 10; i++ {
		err = db.Ping()
		if err == nil {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("failed to ping database: %v", err)
	}

	RunMigrations(t, db)

	return db
}
