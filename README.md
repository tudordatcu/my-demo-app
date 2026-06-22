# VOIS Speechmark Demo — Telecom Subscriber Service

A standalone, runnable **Go HTTP service** modeling a telecom **subscriber / usage / billing** domain. It is *functionally unrelated* to VOIS Speechmark — its job is to be a realistic sample codebase that the **Speechmark** platform is run **against** to showcase its capabilities.

The repo ships with deliberately **planted scenarios** (bugs, vulnerabilities, smells, observability and testing situations). Surrounding code stays clean and idiomatic so the plants read as realistic mistakes. The full presenter mapping lives in [`SCENARIOS.md`](./SCENARIOS.md).

- **Module:** `github.com/vodafone/vois-speechmark-demo`
- **Go:** 1.26 (no cgo)
- **Stack:** stdlib `net/http` (Go 1.22+ method+path routing), `log/slog`, Prometheus `client_golang`, OpenTelemetry, `modernc.org/sqlite` (pure-Go SQLite).

> Not production-hardened. The planted issues are intentional and documented in `SCENARIOS.md`.

## Quickstart

```bash
# Run locally (in-memory store, stdout traces)
make run

# Or the full observability stack (app + Prometheus)
docker-compose up
```

Then smoke-test:

```bash
curl localhost:8080/healthz
curl localhost:8080/readyz
curl localhost:8080/metrics
# Authenticated routes need: -H "Authorization: Bearer <API_KEY>"
```

## Run modes (environment variables)

Configuration is env-based (see `internal/config`). Key toggles:

| Env var | Default | Purpose |
|---|---|---|
| `ADDR` | `:8080` | Listen address. |
| `STORE` | `memory` | Backend store: `memory` or `sqlite`. **`STORE=sqlite`** enables the SQL-injection demo (SEC-01). |
| `SQLITE_DSN` | `file:vois-demo.db?cache=shared` | SQLite DSN when `STORE=sqlite`. |
| `API_KEY` | _(hardcoded fallback — SEC-02)_ | Bearer key for `/v1` routes. |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | _(empty)_ | Empty = **stdout** trace exporter; set to an **OTLP** collector endpoint to ship traces (OBS-03). |
| `LOG_LEVEL` | `info` | slog level. |
| `LOG_PII` | `false` | When `true`, request logs include MSISDN/IMSI (SEC-10 privacy scenario). |
| `SLOW_INVOICE` | `false` | When `true`, the invoice endpoint adds latency (OBS-04). |
| `CARRIER_FAILURE_RATE` | `0.1` | Simulated downstream carrier-lookup failure rate (OBS-05). |

Common run shapes:

```bash
# SQLite store (unlocks SEC-01 SQL-injection demo)
STORE=sqlite make run

# Ship traces to an OTLP collector instead of stdout
OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4317 make run

# Turn on the PII-logging privacy scenario
LOG_PII=true make run
```

## The four demo tracks

Each track has a dedicated table in [`SCENARIOS.md`](./SCENARIOS.md) covering every scenario ID, its file(s), the capability it shows, whether it is a labeled **starter** or **subtle** plant, and a suggested Speechmark prompt.

1. **AI coding assistant** (`AIA-01..10`) — fix planted bugs, add missing features (`DELETE`/plan-change return 501), refactor the god function, generate docs and tests.
2. **Code quality / security scanning** (`SEC-01..20`, `QUA-01..08`) — detect planted vulnerabilities (SQLi, IDOR, SSRF, command injection, weak crypto, …) and smells (god function, magic numbers, global state, …).
3. **Observability / APM** (`OBS-01..08`) — structured logs, metrics, traces, plus runtime scenarios: invoice latency (N+1 + sleep), flaky carrier error rate, unbounded saturation, high-cardinality metric, broken trace propagation.
4. **Testing / CI-CD** (`TST-01..05`, `CI-01..04`) — a table-driven suite, handler `httptest`, a time-dependent flaky test, a `-race` test, deliberate coverage gaps, and a CI pipeline whose gates run on the PR.

**Starters** (carry an in-code `// SCENARIO[<ID>]` comment for the simplest walkthroughs): **SEC-02, SEC-17, AIA-06, TST-03**. All other scenarios are subtle (catalog-only).

## Architecture

Layered monolith — dependencies point inward; the `Store` interface is the seam for swapping memory/sqlite:

```
httpapi (handlers/middleware) → service (subscriber, billing) → store
```

```
cmd/server/main.go        – config load, observability init, store init, router, graceful shutdown
cmd/loadgen/main.go       – traffic generator driving dashboards/traces
internal/config           – env-based config; hardcoded-secret fallback (SEC-02/SEC-19)
internal/httpapi          – ServeMux router, handlers, middleware (requestID, slog, metrics, recover, auth, CORS)
internal/subscriber       – Subscriber + Plan models, validation, service logic, status state machine
internal/billing          – rating/invoice engine (proration, overage) — money-precision & off-by-one bugs
internal/store            – Store interface; memory impl (race) + sqlite impl (SQLi)
internal/auth             – API-key middleware; token generation (weak-random/weak-hash)
internal/observability    – slog, Prometheus metrics, OTel tracer provider
internal/webhook          – outbound webhook client (SSRF, InsecureSkipVerify, no trace propagation, leak)
```

See [API surface](#api-surface) below or `SCENARIOS.md` for the per-route scenario mapping.

## Load generator

`cmd/loadgen` is a standalone HTTP client that drives a steady mix of valid and scenario-triggering traffic so dashboards and traces populate immediately:

```bash
make loadgen
```

It targets the running service (`ADDR`, default `:8080`) and exercises create/get/list/usage/invoice paths — point your Prometheus/Grafana or trace viewer at the service while it runs.

## API surface

All JSON over HTTP. `/v1` routes require `Authorization: Bearer <API_KEY>`.

| Method | Path | Notes |
|---|---|---|
| GET | `/healthz` | Liveness (unauth) |
| GET | `/readyz` | Readiness — store ping (unauth) |
| GET | `/metrics` | Prometheus (unauth) |
| POST | `/v1/plans` | Create plan |
| GET | `/v1/plans` | List plans |
| GET | `/v1/plans/{id}` | Get plan |
| POST | `/v1/subscribers` | Create subscriber |
| GET | `/v1/subscribers` | List/search (SQLi in sqlite store) |
| GET | `/v1/subscribers/{id}` | Get subscriber (IDOR) |
| PATCH | `/v1/subscribers/{id}/status` | Status transition |
| PATCH | `/v1/subscribers/{id}/plan` | Plan change — **501 TODO** |
| DELETE | `/v1/subscribers/{id}` | **501 TODO** (starter) |
| POST | `/v1/subscribers/{id}/usage` | Ingest usage / CDR |
| GET | `/v1/subscribers/{id}/invoice` | Compute invoice (latency + billing bugs) |
| GET | `/v1/subscribers/{id}/invoice/export` | Export to file (path traversal) |
| POST | `/v1/webhooks` | Register webhook URL (SSRF) |
| GET | `/v1/diagnostics/ping` | Network diagnostics (command injection) |
| GET | `/v1/redirect` | Redirect (open redirect, `?url=`) |
| POST | `/admin/reset` | Reset store — **missing auth** scenario |

## Testing

```bash
make test        # go test ./...
make test-race   # go test ./... -race -cover  (canonical; reveals SEC-17)
```

Coverage gaps in `internal/store` and `internal/auth` are **expected and catalogued** (AIA-10 / TST-05), not accidental.

## CI / PR integration gate

`.github/workflows/ci.yml` runs on every PR:

```
checkout → setup-go 1.26 → go build ./... → go vet ./... → golangci-lint → gosec → go test ./... -race -coverprofile → docker build
```

The gates deliberately include **security (gosec)** and **race** steps, so opening a PR surfaces the planted issues. **This PR run is the Speechmark integration-gate trigger point** — for the CI/security tracks, open the PR and let the gates run, then drive Speechmark against the diff and the gate output.

## Demo workflow

1. `make run` (or `docker-compose up`) starts the service (+ Prometheus).
2. `make loadgen` drives traffic → metrics/traces populate.
3. Pick a track, open [`SCENARIOS.md`](./SCENARIOS.md), choose a scenario ID, and run its suggested Speechmark prompt against the repo/PR.
4. For CI/security tracks, open the PR — the integration gates run build/vet/lint/gosec/race and surface planted issues.
