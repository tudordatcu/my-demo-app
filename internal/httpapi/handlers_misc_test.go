package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// miscRouter returns a router built from testDeps (defined in router_test.go).
func miscRouter(t *testing.T) http.Handler {
	t.Helper()
	return NewRouter(testDeps(t))
}

// authReqStr builds an authenticated request with a string body.
// authReq (defined in router_test.go) takes []byte; this variant accepts string.
func authReqStr(t *testing.T, method, target, body string) *http.Request {
	t.Helper()
	if body == "" {
		return authReq(t, method, target, nil)
	}
	return authReq(t, method, target, []byte(body))
}

func TestDeleteSubscriberReturns501(t *testing.T) {
	router := miscRouter(t)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, authReqStr(t, http.MethodDelete, "/v1/subscribers/sub-1", ""))

	if rr.Code != http.StatusNotImplemented {
		t.Fatalf("expected 501, got %d (body=%s)", rr.Code, rr.Body.String())
	}
}

func TestChangePlanReturns501(t *testing.T) {
	router := miscRouter(t)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, authReqStr(t, http.MethodPatch, "/v1/subscribers/sub-1/plan", `{"plan_id":"plan-2"}`))

	if rr.Code != http.StatusNotImplemented {
		t.Fatalf("expected 501, got %d (body=%s)", rr.Code, rr.Body.String())
	}
}

func TestRegisterWebhookCallsNotify(t *testing.T) {
	var hits int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	router := miscRouter(t)
	rr := httptest.NewRecorder()
	body := `{"url":"` + upstream.URL + `"}`
	router.ServeHTTP(rr, authReqStr(t, http.MethodPost, "/v1/webhooks", body))

	if rr.Code != http.StatusOK && rr.Code != http.StatusCreated {
		t.Fatalf("expected 200/201, got %d (body=%s)", rr.Code, rr.Body.String())
	}
	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Fatalf("expected Notify to hit upstream exactly once, got %d hits", got)
	}
}

func TestRegisterWebhookRejectsMissingURL(t *testing.T) {
	router := miscRouter(t)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, authReqStr(t, http.MethodPost, "/v1/webhooks", `{}`))

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing url, got %d (body=%s)", rr.Code, rr.Body.String())
	}
}

func TestPingReturnsOutput(t *testing.T) {
	router := miscRouter(t)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, authReqStr(t, http.MethodGet, "/v1/diagnostics/ping?host=127.0.0.1", ""))

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body=%s)", rr.Code, rr.Body.String())
	}
	var out struct {
		Host   string `json:"host"`
		Output string `json:"output"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode ping response: %v", err)
	}
	if out.Host != "127.0.0.1" {
		t.Fatalf("expected host echoed as 127.0.0.1, got %q", out.Host)
	}
}

func TestPingRejectsMissingHost(t *testing.T) {
	router := miscRouter(t)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, authReqStr(t, http.MethodGet, "/v1/diagnostics/ping", ""))

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing host, got %d (body=%s)", rr.Code, rr.Body.String())
	}
}

func TestInvoiceExportWritesFile(t *testing.T) {
	router := miscRouter(t)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, authReqStr(t, http.MethodGet, "/v1/subscribers/sub-1/invoice/export?filename=invoice.json", ""))

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body=%s)", rr.Code, rr.Body.String())
	}
	var out struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode export response: %v", err)
	}
	if !strings.HasSuffix(out.Path, "invoice.json") {
		t.Fatalf("expected path to end with the requested filename, got %q", out.Path)
	}
}

func TestInvoiceExportRejectsMissingFilename(t *testing.T) {
	router := miscRouter(t)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, authReqStr(t, http.MethodGet, "/v1/subscribers/sub-1/invoice/export", ""))

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing filename, got %d (body=%s)", rr.Code, rr.Body.String())
	}
}

func TestRedirectHonorsURL(t *testing.T) {
	router := miscRouter(t)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, authReqStr(t, http.MethodGet, "/v1/redirect?url=https://example.com/next", ""))

	if rr.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d (body=%s)", rr.Code, rr.Body.String())
	}
	if loc := rr.Header().Get("Location"); loc != "https://example.com/next" {
		t.Fatalf("expected Location to echo user URL, got %q", loc)
	}
}

func TestRedirectRejectsMissingURL(t *testing.T) {
	router := miscRouter(t)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, authReqStr(t, http.MethodGet, "/v1/redirect", ""))

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing url, got %d (body=%s)", rr.Code, rr.Body.String())
	}
}
