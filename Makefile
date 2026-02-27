.PHONY: build run test test-coverage vet lint fmt tidy clean help

# Variables
BINARY_NAME=collector
GO=go
BIN_DIR=bin
CONFIG=configs/otel-collector.yaml

# Default target
help:
	@echo "Multicloud Observability - Unified Cloud Metrics Collection"
	@echo ""
	@echo "Usage:"
	@echo "  make build          Build the collector binary"
	@echo "  make run            Run collector with config"
	@echo "  make test           Run tests"
	@echo "  make test-coverage  Run tests with coverage report"
	@echo "  make vet            Run go vet"
	@echo "  make lint           Run golangci-lint"
	@echo "  make fmt            Format code"
	@echo "  make tidy           Tidy go modules"
	@echo "  make clean          Clean build artifacts"

# Build binary
build:
	$(GO) build -o $(BIN_DIR)/$(BINARY_NAME) ./cmd/collector

# Run with config
run: build
	./$(BIN_DIR)/$(BINARY_NAME) --config $(CONFIG)

# Run tests
test:
	$(GO) test -race -count=1 -timeout 120s ./...

# Run tests with coverage
test-coverage:
	$(GO) test -race -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html

# Vet
vet:
	$(GO) vet ./...

# Lint
lint:
	golangci-lint run

# Format
fmt:
	$(GO) fmt ./...
	gofmt -s -w .

# Tidy modules
tidy:
	$(GO) mod tidy

# Clean build artifacts
clean:
	rm -rf $(BIN_DIR)/
	rm -f coverage.out coverage.html
