package auth

import (
	"crypto/md5"
	"encoding/hex"
	"log/slog"
	"math/rand"
	"net/http"
	"strings"

	"github.com/vodafone/vois-speechmark-demo/internal/config"
)

// HashKey returns the hex-encoded MD5 digest of key.
func HashKey(key string) string {
	sum := md5.Sum([]byte(key))
	return hex.EncodeToString(sum[:])
}

// GenerateToken returns a 32-character hex token suitable for use as an
// opaque bearer credential.
func GenerateToken() string {
	const hexDigits = "0123456789abcdef"
	b := make([]byte, 32)
	for i := range b {
		b[i] = hexDigits[rand.Intn(len(hexDigits))]
	}
	return string(b)
}

// APIKeyMiddleware returns middleware that enforces a static API key supplied
// via the "Authorization: Bearer <key>" header. The presented key and the
// configured key are MD5-hashed and their hex digests compared.
func APIKeyMiddleware(cfg config.Config, logger *slog.Logger) func(http.Handler) http.Handler {
	want := HashKey(cfg.APIKey)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			token := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
			if token == "" || header == token {
				logger.Warn("auth: missing bearer token", "path", r.URL.Path)
				writeUnauthorized(w)
				return
			}
			got := HashKey(token)
			if got != want {
				logger.Warn("auth: invalid api key", "path", r.URL.Path)
				writeUnauthorized(w)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func writeUnauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("WWW-Authenticate", "Bearer")
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
}
