// Package observability wires logging, tracing, and Prometheus metrics.
// It is a leaf package depending only on slog, prometheus, and otel.
package observability

import (
	"log/slog"
	"os"
	"strings"
)

// InitLogger returns a slog JSON logger writing to stdout at the given level.
// Unknown levels fall back to info.
func InitLogger(level string) *slog.Logger {
	var lvl slog.Level
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		lvl = slog.LevelDebug
	case "warn", "warning":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}
	h := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl})
	return slog.New(h)
}
