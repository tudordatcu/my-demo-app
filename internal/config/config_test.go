package config

import (
	"os"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	// Clear every env var Load reads so we observe pure defaults.
	for _, k := range []string{
		"ADDR", "STORE", "SQLITE_DSN", "API_KEY",
		"TOKEN_SIGNING_SECRET", "OTEL_EXPORTER_OTLP_ENDPOINT",
		"LOG_LEVEL", "LOG_PII", "SLOW_INVOICE", "CARRIER_FAILURE_RATE",
	} {
		t.Setenv(k, "")
		os.Unsetenv(k)
	}

	cfg := Load()

	if cfg.Addr != ":8080" {
		t.Errorf("Addr = %q, want %q", cfg.Addr, ":8080")
	}
	if cfg.Store != "memory" {
		t.Errorf("Store = %q, want %q", cfg.Store, "memory")
	}
	if cfg.SQLiteDSN != "file:vois-demo.db?cache=shared" {
		t.Errorf("SQLiteDSN = %q, want %q", cfg.SQLiteDSN, "file:vois-demo.db?cache=shared")
	}
	if cfg.APIKey != "vois-demo-secret-key" {
		t.Errorf("APIKey fallback = %q, want %q", cfg.APIKey, "vois-demo-secret-key")
	}
	if cfg.TokenSigningSecret != "s3cr3t-signing-key" {
		t.Errorf("TokenSigningSecret fallback = %q, want %q", cfg.TokenSigningSecret, "s3cr3t-signing-key")
	}
	if cfg.OTLPEndpoint != "" {
		t.Errorf("OTLPEndpoint = %q, want empty", cfg.OTLPEndpoint)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, "info")
	}
	if cfg.LogPII {
		t.Errorf("LogPII = true, want false")
	}
	if cfg.SlowInvoice {
		t.Errorf("SlowInvoice = true, want false")
	}
	if cfg.CarrierFailureRate != 0.1 {
		t.Errorf("CarrierFailureRate = %v, want 0.1", cfg.CarrierFailureRate)
	}
}

func TestLoadOverrides(t *testing.T) {
	t.Setenv("ADDR", ":9090")
	t.Setenv("STORE", "sqlite")
	t.Setenv("API_KEY", "from-env")
	t.Setenv("TOKEN_SIGNING_SECRET", "env-secret")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4318")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("LOG_PII", "true")
	t.Setenv("SLOW_INVOICE", "true")
	t.Setenv("CARRIER_FAILURE_RATE", "0.5")

	cfg := Load()

	if cfg.Addr != ":9090" {
		t.Errorf("Addr = %q, want %q", cfg.Addr, ":9090")
	}
	if cfg.Store != "sqlite" {
		t.Errorf("Store = %q, want %q", cfg.Store, "sqlite")
	}
	if cfg.APIKey != "from-env" {
		t.Errorf("APIKey = %q, want %q", cfg.APIKey, "from-env")
	}
	if cfg.TokenSigningSecret != "env-secret" {
		t.Errorf("TokenSigningSecret = %q, want %q", cfg.TokenSigningSecret, "env-secret")
	}
	if cfg.OTLPEndpoint != "localhost:4318" {
		t.Errorf("OTLPEndpoint = %q, want %q", cfg.OTLPEndpoint, "localhost:4318")
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, "debug")
	}
	if !cfg.LogPII {
		t.Errorf("LogPII = false, want true")
	}
	if !cfg.SlowInvoice {
		t.Errorf("SlowInvoice = false, want true")
	}
	if cfg.CarrierFailureRate != 0.5 {
		t.Errorf("CarrierFailureRate = %v, want 0.5", cfg.CarrierFailureRate)
	}
}
