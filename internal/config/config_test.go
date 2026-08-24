package config_test

import (
	"os"
	"testing"
	"time"

	"github.com/sentinelmesh/sentinelmesh/internal/config"
)

func TestLoadConfig(t *testing.T) {
	os.Clearenv()

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.HTTPAddr != "127.0.0.1:8787" {
		t.Errorf("expected default HTTPAddr 127.0.0.1:8787, got %s", cfg.HTTPAddr)
	}
	if cfg.DatabaseURL != "" {
		t.Errorf("expected empty DatabaseURL by default, got %s", cfg.DatabaseURL)
	}
	if cfg.ShutdownTimeout != 10*time.Second {
		t.Errorf("expected 10s default shutdown timeout, got %v", cfg.ShutdownTimeout)
	}
}

func TestLoadConfig_EnvVars(t *testing.T) {
	os.Clearenv()
	os.Setenv("SENTINEL_HTTP_ADDR", ":8081")
	os.Setenv("SENTINEL_GRPC_ADDR", ":9091")
	os.Setenv("SENTINEL_DB_URL", "postgres://user:pass@localhost:5432/db")
	os.Setenv("SENTINEL_ENVIRONMENT", "production")
	os.Setenv("SENTINEL_SHUTDOWN_TIMEOUT", "30s")

	cfg, _ := config.Load()

	if cfg.HTTPAddr != ":8081" {
		t.Errorf("expected :8081, got %s", cfg.HTTPAddr)
	}
	if cfg.GRPCAddr != ":9091" {
		t.Errorf("expected :9091, got %s", cfg.GRPCAddr)
	}
	if cfg.DatabaseURL != "postgres://user:pass@localhost:5432/db" {
		t.Errorf("expected postgres URL, got %s", cfg.DatabaseURL)
	}
	if cfg.Environment != "production" {
		t.Errorf("expected production environment, got %s", cfg.Environment)
	}
	if cfg.ShutdownTimeout != 30*time.Second {
		t.Errorf("expected 30s timeout, got %v", cfg.ShutdownTimeout)
	}
}

func TestLoggableDatabaseURL(t *testing.T) {
	cfg := &config.Config{DatabaseURL: "postgres://admin:secret123@db.internal:5432/sentinel"}
	loggable := cfg.LoggableDatabaseURL()
	if loggable != "postgres://admin:xxx@db.internal:5432/sentinel" {
		t.Errorf("expected masked URL, got %s", loggable)
	}

	cfgNoAuth := &config.Config{DatabaseURL: "postgres://db.internal:5432/sentinel"}
	if cfgNoAuth.LoggableDatabaseURL() != "postgres://db.internal:5432/sentinel" {
		t.Errorf("expected unmodified URL, got %s", cfgNoAuth.LoggableDatabaseURL())
	}
}
