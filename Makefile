# relaymesh-edge — rmesh agent

SHELL := /bin/bash

GO ?= go
BIN_DIR := bin
BINARY := $(BIN_DIR)/rmesh
MAIN := ./cmd/rmesh
APP_VERSION ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(APP_VERSION)

# Optional override for agent targets: make doctor CONFIG=/path/to/config.yaml
CONFIG ?=

.PHONY: help
help:
	@echo "relaymesh-edge (rmesh) — common targets:"
	@echo ""
	@echo "  make build          compile rmesh to $(BINARY)"
	@echo "  make install        go install $(MAIN) (into \$$(go env GOPATH)/bin)"
	@echo "  make test           run unit tests"
	@echo "  make test-race      run tests with -race"
	@echo "  make coverage       test coverage report (coverage.out)"
	@echo "  make coverage-web   coverage + open HTML report"
	@echo "  make tidy           go mod tidy"
	@echo "  make fmt            gofmt all Go sources"
	@echo "  make vet            go vet ./..."
	@echo "  make lint           golangci-lint run (if installed)"
	@echo "  make clean          remove build artifacts"
	@echo ""
	@echo "  make doctor         rmesh agent doctor  (platform default config)"
	@echo "  make observe        rmesh agent observe (dry-run JSONL)"
	@echo "  make run            rmesh agent run     (publish to MQTT)"
	@echo "                      override: make doctor CONFIG=path/to/config.yaml"
	@echo ""
	@echo "  make ci             tidy + vet + test + build (local CI parity)"

# rmesh_config_args expands to --config when CONFIG is set; otherwise the binary default applies.
rmesh_config_args = $(if $(CONFIG),--config "$(CONFIG)",)

.PHONY: build
build:
	@mkdir -p $(BIN_DIR)
	$(GO) build -ldflags="$(LDFLAGS)" -o $(BINARY) $(MAIN)
	@echo "built $(BINARY)"

.PHONY: install
install:
	$(GO) install -ldflags="$(LDFLAGS)" $(MAIN)

.PHONY: test
test:
	$(GO) test ./...

.PHONY: test-race
test-race:
	$(GO) test -race ./...

.PHONY: coverage
coverage:
	$(GO) test -coverprofile=coverage.out ./...
	$(GO) tool cover -func=coverage.out

.PHONY: coverage-web
coverage-web: coverage
	$(GO) tool cover -html=coverage.out

.PHONY: tidy
tidy:
	$(GO) mod tidy

.PHONY: fmt
fmt:
	@find . -name '*.go' -not -path './vendor/*' -print0 | xargs -0 gofmt -w

.PHONY: vet
vet:
	$(GO) vet ./...

.PHONY: lint
lint:
	@command -v golangci-lint >/dev/null 2>&1 || { echo "golangci-lint not installed"; exit 1; }
	golangci-lint run ./...

.PHONY: clean
clean:
	rm -rf $(BIN_DIR) coverage.out rmesh dist

.PHONY: doctor observe run
doctor: build
	$(BINARY) $(rmesh_config_args) agent doctor

observe: build
	$(BINARY) $(rmesh_config_args) agent observe

run: build
	$(BINARY) $(rmesh_config_args) agent run

.PHONY: ci
ci: tidy vet test build
