# Project variables
BINARY_NAME = q
BUILD_DIR = _build
VERSION = $(shell git describe --tags --always --dirty)
COMMIT = $(shell git rev-parse --short HEAD)
DATE = $(shell date -u '+%Y-%m-%d_%H:%M:%S')

.PHONY: build install uninstall lint format test

# Build the application
build:
	go build -ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)" -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/q

# Install the binary
install: build
	cp $(BUILD_DIR)/$(BINARY_NAME) /usr/local/bin/

# Uninstall the binary
uninstall:
	rm -f /usr/local/bin/$(BINARY_NAME)

# Format code: installs formatters if needed, runs them
format:
	@echo "Ensuring formatters are installed..."
	@command -v gofumpt >/dev/null 2>&1 || go install mvdan.cc/gofumpt@latest
	@command -v goimports >/dev/null 2>&1 || go install golang.org/x/tools/cmd/goimports@latest
	@command -v gci >/dev/null 2>&1 || go install github.com/daixiang0/gci@latest

	@echo "Running formatters..."
	go vet ./...
	go fmt ./...
	gofumpt -extra -l -w .
	goimports -l -w .
	gci write --skip-generated .
	go mod tidy

# Lint: uses local config and makes sure it’s installed
lint:
	@command -v golangci-lint >/dev/null 2>&1 || go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	golangci-lint run

# Run tests
test:
	go test -v ./...
