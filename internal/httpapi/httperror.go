package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/vodafone/vois-speechmark-demo/internal/subscriber"
)

const (
	defaultPageLimit = 50
	maxPageLimit     = 200
)

// writeJSON encodes v as JSON with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError writes a JSON error envelope with the given status code.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// httpStatusForError maps domain sentinel errors to HTTP status codes.
func httpStatusForError(err error) int {
	switch {
	case err == nil:
		return http.StatusOK
	case errors.Is(err, subscriber.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, subscriber.ErrValidation):
		return http.StatusBadRequest
	case errors.Is(err, subscriber.ErrInvalidStatusTransition):
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}

// parsePagination reads ?limit and ?offset, clamping to sane defaults/bounds.
func parsePagination(r *http.Request) (limit, offset int) {
	limit = defaultPageLimit
	offset = 0
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	if limit > maxPageLimit {
		limit = maxPageLimit
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}
	return limit, offset
}
