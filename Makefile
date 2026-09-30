BINARY     := ecsctl
VERSION    ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT     ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
BUILD_DIR  := ./bin
LDFLAGS    := -ldflags "-X github.com/roslaan001/ecsctl/cmd.Version=$(VERSION) -X github.com/roslaan001/ecsctl/cmd.Commit=$(COMMIT) -X github.com/roslaan001/ecsctl/cmd.BuildDate=$(BUILD_DATE)"

.PHONY: all build clean install tidy lint test test-floci docs-serve docs-build

all: build

## build: compile the binary into ./bin/ecsctl
build:
	@mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY) .

## install: install ecsctl to $GOPATH/bin
install:
	go install $(LDFLAGS) .

## tidy: tidy and vendor go modules
tidy:
	go mod tidy

## lint: run golangci-lint
lint:
	golangci-lint run ./...

## test: run all tests
test:
	go test ./... -v -race

## test-floci: run ECS integration test against local Floci (requires Docker Compose or Podman)
test-floci:
	@set -eu; \
		if command -v docker >/dev/null 2>&1 && docker compose version >/dev/null 2>&1; then \
			cleanup() { docker compose -f docker-compose.floci.yml down -v; }; \
			trap cleanup EXIT; \
			docker compose -f docker-compose.floci.yml up -d; \
		elif command -v podman >/dev/null 2>&1; then \
			container_name=ecsctl-floci-test-$$$$; \
			cleanup() { podman rm -f "$$container_name" >/dev/null 2>&1 || true; }; \
			trap cleanup EXIT; \
			podman run -d --name "$$container_name" -p 4566:4566 -e FLOCI_SERVICES_ECS_MOCK=true docker.io/floci/floci:2.1.0 >/dev/null; \
		else \
			echo 'Floci tests require Docker Compose or Podman' >&2; exit 1; \
		fi; \
		AWS_ENDPOINT_URL=http://127.0.0.1:4566 AWS_DEFAULT_REGION=us-east-1 AWS_ACCESS_KEY_ID=test AWS_SECRET_ACCESS_KEY=test AWS_EC2_METADATA_DISABLED=true go test -tags=integration ./pkg/aws -run '^TestFlociECSServiceLifecycle$$' -count=1 -v

## clean: remove build artifacts
clean:
	rm -rf $(BUILD_DIR)
	rm -rf site

## docs-serve: run mkdocs development server locally
docs-serve:
	@if [ ! -d ".venv" ]; then \
		echo "Creating virtual environment .venv..."; \
		python3 -m venv .venv; \
		.venv/bin/pip install --upgrade pip; \
		.venv/bin/pip install mkdocs-material; \
	fi
	@echo "Starting MkDocs development server..."
	@.venv/bin/mkdocs serve

## docs-build: build mkdocs static site
docs-build:
	@if [ ! -d ".venv" ]; then \
		echo "Creating virtual environment .venv..."; \
		python3 -m venv .venv; \
		.venv/bin/pip install --upgrade pip; \
		.venv/bin/pip install mkdocs-material; \
	fi
	@echo "Building MkDocs static site..."
	@.venv/bin/mkdocs build

## help: print this help
help:
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | column -t -s ':' | sed -e 's/^/ /'

