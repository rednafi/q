# Go environment
GOPATH := $(shell go env GOPATH)
GOBIN := $(shell go env GOBIN)
ifeq ($(GOBIN),)
GOBIN := $(GOPATH)/bin
endif

.PHONY: format lint

# Format code: installs formatters if needed, runs them
format:
	@echo "Ensuring formatters are installed..."
	@command -v gofumpt >/dev/null 2>&1 || go install mvdan.cc/gofumpt@latest
	@echo "Running formatters..."
	@go fmt ./...
	@gofumpt -w .

# Lint: uses local config and makes sure it's installed
lint:
	@echo "Ensuring golangci-lint is installed..."
	@command -v golangci-lint >/dev/null 2>&1 || go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@echo "Running golangci-lint..."
	@golangci-lint run
