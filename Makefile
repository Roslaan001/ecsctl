BINARY     := ecsctl
VERSION    ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT     ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
BUILD_DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
BUILD_DIR  := ./bin
LDFLAGS    := -ldflags "-X github.com/roslaan001/ecsctl/cmd.Version=$(VERSION) -X github.com/roslaan001/ecsctl/cmd.Commit=$(COMMIT) -X github.com/roslaan001/ecsctl/cmd.BuildDate=$(BUILD_DATE)"

.PHONY: all build clean install tidy lint test

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

## clean: remove build artifacts
clean:
	rm -rf $(BUILD_DIR)

## help: print this help
help:
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | column -t -s ':' | sed -e 's/^/ /'
