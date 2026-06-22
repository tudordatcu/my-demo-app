// Package config loads service configuration from environment variables,
// applying defaults suitable for the in-memory demo run mode.
package config

import (
	"os"
	"strconv"
)

// Config holds all runtime configuration for the service.
type Config struct {
	Addr               string
	Store              string
	SQLiteDSN          string
	APIKey             string
	TokenSigningSecret string
	OTLPEndpoint       string
	LogLevel           string
	LogPII             bool
	SlowInvoice        bool
	CarrierFailureRate float64
}

// SEC-02: hardcoded default API key used when API_KEY is unset. // SCENARIO[SEC-02]:
const apiKeyFallback = "vois-demo-secret-key"

// SEC-19: predictable token signing secret baked into the binary,
// used when TOKEN_SIGNING_SECRET is unset.
const tokenSigningSecretFallback = "s3cr3t-signing-key"

// Load reads configuration from the environment, falling back to defaults.
func Load() Config {
	return Config{
		Addr:               getString("ADDR", ":8080"),
		Store:              getString("STORE", "memory"),
		SQLiteDSN:          getString("SQLITE_DSN", "file:vois-demo.db?cache=shared"),
		APIKey:             getString("API_KEY", apiKeyFallback),
		TokenSigningSecret: getString("TOKEN_SIGNING_SECRET", tokenSigningSecretFallback),
		OTLPEndpoint:       getString("OTEL_EXPORTER_OTLP_ENDPOINT", ""),
		LogLevel:           getString("LOG_LEVEL", "info"),
		LogPII:             getBool("LOG_PII", false),
		SlowInvoice:        getBool("SLOW_INVOICE", false),
		CarrierFailureRate: getFloat("CARRIER_FAILURE_RATE", 0.1),
	}
}

func getString(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func getBool(key string, fallback bool) bool {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}

func getFloat(key string, fallback float64) float64 {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return fallback
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return fallback
	}
	return f
}
