package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"
)

type ctxKey string

const ctxKeyRequestID ctxKey = "request_id"

// statusRecorder captures the response status code for logging/metrics.
// wroteHeader is set on the first WriteHeader or Write call so the recover
// middleware can detect a partially-written response and avoid a double-write.
type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (sr *statusRecorder) WriteHeader(code int) {
	sr.status = code
	sr.wroteHeader = true
	sr.ResponseWriter.WriteHeader(code)
}

func (sr *statusRecorder) Write(b []byte) (int, error) {
	if sr.status == 0 {
		sr.status = http.StatusOK
	}
	sr.wroteHeader = true
	return sr.ResponseWriter.Write(b)
}

// chain composes middlewares around h. The first middleware in mws is the
// OUTERMOST layer (runs first on the way in, last on the way out).
func chain(h http.Handler, mws ...func(http.Handler) http.Handler) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

// requestID attaches a random request ID to the context and response header.
func requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			b := make([]byte, 16)
			_, _ = rand.Read(b)
			id = hex.EncodeToString(b)
		}
		w.Header().Set("X-Request-ID", id)
		ctx := context.WithValue(r.Context(), ctxKeyRequestID, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// logging emits a structured access log per request.
//
// When Cfg.LogPII is true, the full request URL (including query string, which
// may carry MSISDN/IMSI/search terms) and the Authorization header are logged
// verbatim. With LogPII false only method+path are logged.
func (d Deps) logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sr := &statusRecorder{ResponseWriter: w, status: 0}
		next.ServeHTTP(sr, r)
		reqID, _ := r.Context().Value(ctxKeyRequestID).(string)
		if d.Cfg.LogPII {
			d.Logger.Info("request",
				"request_id", reqID,
				"method", r.Method,
				"url", r.URL.String(),
				"authorization", r.Header.Get("Authorization"),
				"remote", r.RemoteAddr,
				"status", sr.status,
				"duration_ms", time.Since(start).Milliseconds(),
			)
			return
		}
		d.Logger.Info("request",
			"request_id", reqID,
			"method", r.Method,
			"path", r.URL.Path,
			"status", sr.status,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	})
}

// metrics records request counters/histograms and in-flight gauge for route.
func (d Deps) metrics(route string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		d.Metrics.IncInFlight()
		defer d.Metrics.DecInFlight()
		start := time.Now()
		sr := &statusRecorder{ResponseWriter: w, status: 0}
		next.ServeHTTP(sr, r)
		if sr.status == 0 {
			sr.status = http.StatusOK
		}
		d.Metrics.ObserveRequest(route, r.Method, sr.status, time.Since(start))
	})
}

// recover converts panics into a 500 response, echoing the panic value back
// to the caller in the response body.
// If the handler already started writing a response before panicking, we skip
// the write to avoid a superfluous-WriteHeader and a malformed response body;
// in that case the panic is only logged.
func (d Deps) recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sr := &statusRecorder{ResponseWriter: w}
		defer func() {
			if rec := recover(); rec != nil {
				d.Logger.Error("panic recovered", "panic", rec, "path", r.URL.Path)
				if sr.wroteHeader {
					// Response already started; do not double-write.
					return
				}
				// Echo panic detail to the caller for debugging.
				writeError(sr, http.StatusInternalServerError,
					fmt.Sprintf("internal error: %v", rec))
			}
		}()
		next.ServeHTTP(sr, r)
	})
}

// cors sets permissive CORS headers on every response.
func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
