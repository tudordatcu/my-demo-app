# SCENARIOS.md — VOIS Speechmark Demo Presenter Catalog

This is the **source of truth** mapping every planted scenario to its location, the Speechmark capability it demonstrates, whether it is a labeled **starter** or a **subtle** plant, and a suggested Speechmark prompt to run against the repo or PR.

- **Subtle** scenarios look like realistic mistakes and carry **no** in-code marker — find them with Speechmark.
- **Starter** scenarios carry an in-code `// SCENARIO[<ID>]: <desc>` comment for the simplest guided walkthroughs. There are exactly four: **SEC-02, SEC-17, AIA-06, TST-03**.

Module path: `github.com/vodafone/vois-speechmark-demo`. Paths below are relative to repo root.

> **Line numbers are informational** — code may drift; cite **file + symbol** as the authoritative locator.

---

## Track 1 — Security / Vulnerability

| ID | File(s) | Capability | Subtle/Starter | Suggested Speechmark prompt |
|---|---|---|---|---|
| SEC-01 | `internal/store/sqlite.go` (`SQLiteStore.ListSubscribers`) | SQL injection — search term concatenated into SQL via `fmt.Sprintf` | Subtle | "Scan `internal/store` for SQL injection. Show me the vulnerable query and rewrite it with parameterized placeholders." |
| SEC-02 | `internal/config/config.go` (`Load`, `apiKeyFallback` constant, line 24–25) | Hardcoded credential — default API key when env unset | **Starter** | "Find hardcoded secrets/credentials in `internal/config`. Explain the risk and propose a fail-closed default." |
| SEC-03 | `internal/auth/auth.go` (`GenerateToken`, line 22–26) | Weak randomness — tokens via `math/rand` | Subtle | "Audit `internal/auth` token generation for predictable randomness; migrate to `crypto/rand`." |
| SEC-04 | `internal/auth/auth.go` (`HashKey`, `APIKeyMiddleware`) | Weak hashing + non-constant-time compare (MD5) | Subtle | "Review API-key hashing and comparison in `internal/auth`; flag weak hash and timing-unsafe compare, then fix." |
| SEC-05 | `internal/httpapi/handlers_subscribers.go` (`handleGetSubscriber`) | IDOR — `GET /v1/subscribers/{id}` ignores caller ownership | Subtle | "Check `GET /v1/subscribers/{id}` for broken access control / IDOR and add an ownership check." |
| SEC-06 | `internal/httpapi/handlers_ops.go` (`handleAdminReset`) + `internal/httpapi/router.go` | Missing authentication — `POST /admin/reset` not behind auth middleware | Subtle | "Verify every state-changing route is authenticated. Which route is exposed and how do I gate it?" |
| SEC-07 | `internal/httpapi/handlers_misc.go` (`handleInvoiceExport`) | Path traversal — unsanitized user filename in export | Subtle | "Audit the invoice export handler for path traversal; sanitize the filename and constrain to a base dir." |
| SEC-08 | `internal/httpapi/handlers_misc.go` (`handlePing`) | Command injection — `/v1/diagnostics/ping` shells out with user input via `exec.Command("sh", "-c", ...)` | Subtle | "Review `/v1/diagnostics/ping` for command injection and replace the shell-out with a safe lookup." |
| SEC-09 | `internal/webhook/client.go` (`Client.Notify`) + `internal/httpapi/handlers_misc.go` (`handleRegisterWebhook`) | SSRF — webhook URL fetched without allow-list validation | Subtle | "Find the SSRF in webhook registration; add URL allow-listing and block internal/link-local targets." |
| SEC-10 | `internal/httpapi/middleware.go` (`logging`) + `internal/config` (`LogPII`) | PII in logs — MSISDN/IMSI logged at info when `LOG_PII=true` | Subtle | "Search logging middleware for PII leakage (MSISDN/IMSI) and redact it." |
| SEC-11 | `internal/httpapi/middleware.go` (`recover`) | Sensitive data in errors — internal panic detail returned to client | Subtle | "Check error responses for leaked internal details/stack traces and return a safe generic error." |
| SEC-12 | `internal/webhook/client.go` (`New`) | Insecure TLS — `InsecureSkipVerify: true` in the HTTP transport | Subtle | "Scan the webhook HTTP client for insecure TLS configuration and remove `InsecureSkipVerify`." |
| SEC-13 | `internal/httpapi/middleware.go` (`cors`) | Permissive CORS — `Access-Control-Allow-Origin: *` on authed routes | Subtle | "Review CORS configuration; the wildcard origin on authenticated routes — explain and tighten it." |
| SEC-14 | `internal/billing/billing.go` (`Rate`) | Integer overflow — data MB→bytes cast to `int32` wraps for large values | Subtle | "Audit `internal/billing` numeric conversions for overflow in the data-usage byte calculation." |
| SEC-15 | `internal/subscriber/service.go` (validation) + `internal/httpapi/handlers_subscribers.go` (`handleAddUsage`) | Missing input validation — negative usage / malformed MSISDN accepted | Subtle | "Add input validation for usage quantity and MSISDN format; show me what is currently accepted." |
| SEC-16 | `internal/httpapi/handlers_subscribers.go` (`handleAddUsage`) | No request size limit — unbounded JSON body (DoS) | Subtle | "Find request bodies decoded without a size limit and wrap them with `http.MaxBytesReader`." |
| SEC-17 | `internal/store/memory.go` (`MemoryStore` maps, line 16) | Race condition — maps accessed without a mutex (caught by `-race`) | **Starter** | "Run the race detector. `internal/store/memory.go` has unsynchronized map access — guard it with a mutex." |
| SEC-18 | `internal/webhook/client.go` (`Client.Notify`) | Resource leak — `resp.Body` not closed after the HTTP response | Subtle | "Find unclosed response bodies / leaked resources in `internal/webhook` and fix them." |
| SEC-19 | `internal/config/config.go` (`tokenSigningSecretFallback`, line 29) | Predictable/hardcoded token secret — signing secret constant in source | Subtle | "Locate the hardcoded token-signing secret and move it to required configuration." |
| SEC-20 | `internal/httpapi/handlers_misc.go` (`handleRedirect`) | Open redirect — `GET /v1/redirect?url=` honors any user-supplied target | Subtle | "Check `/v1/redirect` for open redirect; validate the target against an allow-list." |

## Track 2 — Code Quality / Smells

| ID | File(s) | Capability | Subtle/Starter | Suggested Speechmark prompt |
|---|---|---|---|---|
| QUA-01 | `internal/httpapi/handlers_subscribers.go` (`handleCreateSubscriber`) | God function — oversized create handler (parse + validate + business + persist all inline) | Subtle | "Review `handleCreateSubscriber`; it does too much. Suggest an extraction/refactor into focused functions." |
| QUA-02 | `internal/httpapi/handlers_subscribers.go` + `internal/httpapi/handlers_misc.go` | Duplication — copy-pasted validation logic across handlers | Subtle | "Find duplicated validation logic across handlers and extract a shared helper." |
| QUA-03 | `internal/billing/billing.go` (`Rate`) | Magic numbers — billing rates/divisors inline without named constants | Subtle | "Flag magic numbers in `internal/billing` and replace with named constants." |
| QUA-04 | `internal/billing/billing.go` (`defaultProrationDays`, `centsToMajor`) | Dead code — unused helpers/vars that are never called by `Rate` | Subtle | "Detect dead/unused code in `internal/billing` and remove it." |
| QUA-05 | `internal/billing/billing.go` (`Rate`) | High cyclomatic complexity — deeply nested rating branches | Subtle | "Measure cyclomatic complexity in `internal/billing.Rate` and flatten the nesting." |
| QUA-06 | `internal/billing/billing.go` (`Rate`) | Ignored errors — unchecked `err` on unknown-kind path enabling a possible nil deref | Subtle | "Find ignored errors in `internal/billing` that could lead to a nil dereference." |
| QUA-07 | `internal/httpapi/handlers_subscribers.go` (`handleAddUsage`) | Inconsistent error handling — plain text vs JSON envelope used by all other handlers | Subtle | "Identify the handler that returns plain-text errors instead of the JSON envelope and align it." |
| QUA-08 | `internal/subscriber/service.go` (`idCounter` package-level var) | Global mutable state — package-level mutable counter without synchronization | Subtle | "Find package-level mutable state in `internal/subscriber` and explain the concurrency hazard." |

## Track 3 — AI Coding Assistant (bugs / features / refactor / docs / tests)

| ID | File(s) | Capability | Subtle/Starter | Suggested Speechmark prompt |
|---|---|---|---|---|
| AIA-01 | `internal/billing/billing.go` (`Rate`, proration path) | Money as `float64` — precision bug (fix across codebase) | Subtle | "Money should be int64 cents everywhere; find and fix the `float64` money path in `internal/billing`." |
| AIA-02 | `internal/billing/billing.go` (`Rate`, `spanDays` calculation) | Proration off-by-one — wrong day count at period boundary | Subtle | "Verify proration day-count at the period boundary in `internal/billing`; there is an off-by-one." |
| AIA-03 | `internal/subscriber/service.go` (`ListPlans`, `ListSubscribers`) | Pagination off-by-one — list service methods subtract 1 from offset before passing to store | Subtle | "Check pagination (limit/offset) on the list endpoints for an off-by-one and fix it." |
| AIA-04 | `internal/billing/billing.go` (`Rate`, `spanDays` via `.Local()`) | Timezone bug — period boundary uses local vs UTC inconsistently | Subtle | "Audit `internal/billing` period boundaries for local-vs-UTC inconsistency; normalize to UTC." |
| AIA-05 | `internal/subscriber/service.go` (`allowedTransitions`, `ChangeStatus`) | Invalid status transition — terminated subscriber can be reactivated | Subtle | "Review the status state machine in `internal/subscriber`; a terminated subscriber should not reactivate." |
| AIA-06 | `internal/httpapi/handlers_misc.go` (`handleDeleteSubscriber`, line 101) | Feature gap — `DELETE /v1/subscribers/{id}` returns 501 + TODO | **Starter** | "Implement `DELETE /v1/subscribers/{id}`; it currently returns 501. Add store + service support and a test." |
| AIA-07 | `internal/httpapi/handlers_misc.go` (`handleChangePlan`) | Feature gap — plan change with proration returns 501 + TODO | Subtle | "Implement `PATCH /v1/subscribers/{id}/plan` with prorated billing; it currently returns 501." |
| AIA-08 | `internal/httpapi/handlers_subscribers.go` (`handleCreateSubscriber`) + `internal/httpapi/handlers_misc.go` | Refactor target — the god function + duplicated validation | Subtle | "Refactor the subscriber-create handler and de-duplicate validation across `internal/httpapi`." |
| AIA-09 | `internal/billing/billing.go` (exported symbols — `Rate`, `daysInMonth`, `centsToMajor`) | Docs gap — exported billing types/functions lack doc comments | Subtle | "Generate Go doc comments for all exported symbols in `internal/billing`." |
| AIA-10 | `internal/store/memory.go`, `internal/store/sqlite.go`, `internal/auth/auth.go` | Test gap — `store` and `auth` packages have no `_test.go` files | Subtle | "Write table-driven tests for `internal/store` and `internal/auth`; both packages are currently untested." |

## Track 4 — Observability / APM

| ID | File(s) | Capability | Subtle/Starter | Suggested Speechmark prompt |
|---|---|---|---|---|
| OBS-01 | `internal/observability/logging.go` (`InitLogger`) + `internal/httpapi/middleware.go` (`logging`) | Structured logs — slog JSON with `request_id` + latency (good baseline) | Subtle (baseline) | "Show the structured-logging setup; confirm request_id and latency are emitted per request." |
| OBS-02 | `internal/observability/metrics.go` (`NewMetrics`) | Metrics suite — request/latency/in-flight + business metrics (good baseline) | Subtle (baseline) | "Enumerate the Prometheus metrics this service exposes and what each measures." |
| OBS-03 | `internal/observability/tracing.go` (`InitTracer`) | Tracing — server + child spans, stdout/OTLP (good baseline) | Subtle (baseline) | "Explain the OTel tracing setup and how to switch from stdout to an OTLP collector." |
| OBS-04 | `internal/httpapi/invoice.go` (`handleGetInvoice`) | Latency scenario — invoice N+1 store lookups per usage kind + configurable `SlowInvoice` sleep | Subtle | "Trace `GET /v1/subscribers/{id}/invoice`; identify the N+1 lookups and the sleep, then optimize." |
| OBS-05 | `internal/httpapi/carrier.go` (`CarrierClient.Lookup`) + `internal/httpapi/invoice.go` | Error-rate scenario — flaky carrier lookup controlled by `CARRIER_FAILURE_RATE` | Subtle | "Why does the invoice endpoint intermittently fail? Find the simulated downstream and surface its error rate." |
| OBS-06 | `internal/store/memory.go` (`MemoryStore`) | Saturation scenario — unbounded in-memory store growth (no eviction, no cap) | Subtle | "Under load, memory climbs without bound. Locate the unbounded growth in `internal/store`." |
| OBS-07 | `internal/observability/metrics.go` (`NewMetrics`, `vois_billing_amount_cents`) + `internal/httpapi/invoice.go` | High-cardinality metric — `vois_billing_amount_cents{msisdn}` labelled per subscriber | Subtle | "Review metric labels for cardinality risk; `vois_billing_amount_cents` labels by `msisdn` — explain and fix." |
| OBS-08 | `internal/webhook/client.go` (`Client.Notify`) | Broken trace propagation — `http.NewRequest` used instead of `http.NewRequestWithContext`, dropping the span context | Subtle | "The webhook calls do not appear in traces. Find where context propagation is dropped in `internal/webhook`." |

## Track 5 — Testing / CI-CD

| ID | File(s) | Capability | Subtle/Starter | Suggested Speechmark prompt |
|---|---|---|---|---|
| TST-01 | `internal/billing/billing_test.go` (`TestRate`) | Table-driven tests — billing rating (good example) | Subtle (example) | "Review the billing tests as a table-driven style reference; extend them to cover proration edge cases." |
| TST-02 | `internal/httpapi/router_test.go`, `internal/httpapi/handlers_misc_test.go` | Handler tests — `httptest` against `NewRouter` for a range of endpoints | Subtle (example) | "Show the handler tests; add `httptest` coverage for an endpoint that lacks it." |
| TST-03 | `internal/httpapi/invoice_test.go` (`seedInvoiceFixture`) | Flaky test — usage record seeded with `time.Now().UTC()` makes the invoice time-window dependent | **Starter** | "There is a time-dependent test in `internal/httpapi/invoice_test.go`. Diagnose the flakiness and make it deterministic." |
| TST-04 | `internal/httpapi/race_test.go` (`TestConcurrentCreateSubscriber`, build tag `race`) | Race test — concurrent `POST /v1/subscribers` under `-race` (exercises SEC-17) | Subtle | "Run the race test with `-race`; explain the data race it reveals and how to fix the underlying store." |
| TST-05 | `internal/store/` (no `_test.go`), `internal/auth/` (no `_test.go`) | Coverage gaps — store and auth packages have zero test files (ties to AIA-10) | Subtle | "Report packages with zero test coverage and generate a test plan for `internal/store` and `internal/auth`." |

## Track 6 — CI / Tooling

| ID | File(s) | Capability | Subtle/Starter | Suggested Speechmark prompt |
|---|---|---|---|---|
| CI-01 | `.github/workflows/ci.yml` | CI workflow — build, vet, golangci-lint, gosec, `go test -race -cover`, docker build | Subtle | "Review the CI workflow; confirm security (gosec) and race gates run on PRs and explain each step." |
| CI-02 | `Makefile` | Makefile — run/build/test/test-race/lint/sec/docker/loadgen/compose-up targets | Subtle | "Walk through the Makefile targets and map each to its CI equivalent." |
| CI-03 | `.golangci.yml`, `prometheus.yml` | Config files — linter set + Prometheus scrape config | Subtle | "Audit `.golangci.yml` linter selection and `prometheus.yml` scrape targets." |
| CI-04 | `Dockerfile`, `docker-compose.yml` | Container — multi-stage Dockerfile, compose stack (app + Prometheus, optional Grafana) | Subtle | "Review the multi-stage Dockerfile and compose stack for the observability demo." |

---

## Starter index (in-code `// SCENARIO[<ID>]` labeled)

| ID | Where | One-line |
|---|---|---|
| SEC-02 | `internal/config/config.go` (line 24) | Hardcoded API-key fallback (`apiKeyFallback` constant). |
| SEC-17 | `internal/store/memory.go` (line 16) | Unsynchronized map access (data race). |
| AIA-06 | `internal/httpapi/handlers_misc.go` (line 101) | `DELETE /v1/subscribers/{id}` returns 501. |
| TST-03 | `internal/httpapi/invoice_test.go` (`seedInvoiceFixture`) | Time-dependent flaky test seeding usage with `time.Now()`. |

> **Note on TST-03 marker:** The `// SCENARIO[TST-03]` in-code comment is absent from the current source — the flaky test is identified by the `time.Now().UTC()` call in `seedInvoiceFixture`. The other three starters (SEC-02, SEC-17, AIA-06) carry the marker exactly as specified.

All other IDs are **subtle** — no in-code marker; discover them with Speechmark.

> Coverage note (from design §8): the catalog is intentionally broad ("maximum coverage"). If surrounding code cannot host a scenario realistically, the scenario is dropped and noted here rather than forced — no silent omissions.
