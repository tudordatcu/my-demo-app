# VOIS Speechmark demo — local tooling mirror of CI.
# Money/observability/security scenarios are documented in SCENARIOS.md.

MODULE      := github.com/vodafone/vois-speechmark-demo
BIN_DIR     := bin
SERVER_BIN  := $(BIN_DIR)/server
LOADGEN_BIN := $(BIN_DIR)/loadgen
IMAGE       := vois-speechmark-demo:latest
PKGS        := ./...

GO          ?= go
GOFLAGS     ?=

# Tool versions (installed on demand into $(GOBIN) if absent).
GOLANGCI_LINT_VERSION ?= v1.62.2
GOSEC_VERSION         ?= v2.21.4

GOBIN := $(shell $(GO) env GOPATH)/bin

.DEFAULT_GOAL := build

.PHONY: all build run test test-race vet cover lint sec docker loadgen \
        compose-up compose-down tidy tools clean help

all: build

## build: compile server + loadgen into ./bin
build:
	$(GO) build $(GOFLAGS) -o $(SERVER_BIN) ./cmd/server
	$(GO) build $(GOFLAGS) -o $(LOADGEN_BIN) ./cmd/loadgen

## run: run the HTTP server (in-memory store by default)
run:
	$(GO) run ./cmd/server

## test: run the unit/handler test suite
test:
	$(GO) test $(PKGS)

## test-race: run tests with the race detector (surfaces SEC-17)
test-race:
	$(GO) test -race -coverprofile=coverage.out -covermode=atomic $(PKGS)

## vet: run go vet
vet:
	$(GO) vet $(PKGS)

## cover: produce coverage.out and a human summary
cover:
	$(GO) test -coverprofile=coverage.out -covermode=atomic $(PKGS)
	$(GO) tool cover -func=coverage.out

## lint: run golangci-lint (installs it on demand)
lint: tools
	$(GOBIN)/golangci-lint run

## sec: run gosec security scanner (installs it on demand; may report planted SEC-* issues)
sec: tools
	$(GOBIN)/gosec -no-fail -fmt=text $(PKGS)

## docker: build the multi-stage container image
docker:
	docker build -t $(IMAGE) .

## loadgen: run the traffic generator against a running server
loadgen:
	$(GO) run ./cmd/loadgen

## compose-up: start app + prometheus via docker compose
compose-up:
	docker compose up --build

## compose-down: stop the docker compose stack
compose-down:
	docker compose down

## tidy: sync go.mod/go.sum
tidy:
	$(GO) mod tidy

## tools: install golangci-lint + gosec into GOPATH/bin if missing
tools:
	@command -v $(GOBIN)/golangci-lint >/dev/null 2>&1 || \
		$(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
	@command -v $(GOBIN)/gosec >/dev/null 2>&1 || \
		$(GO) install github.com/securego/gosec/v2/cmd/gosec@$(GOSEC_VERSION)

## clean: remove build artifacts
clean:
	rm -rf $(BIN_DIR) coverage.out

## help: list documented targets
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed -e 's/## //'
