BIN := mrlsp
PKG := ./cmd/mrlsp
PREFIX ?= $(HOME)/.local

VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X github.com/eunmann/martian-lsp/internal/server.Version=$(VERSION)

# Tools (using go run to avoid global installs)
GOLANGCI_LINT := go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2

.PHONY: all build test test-nvim vet lint lint-check install clean

all: build

build:
	go build -ldflags "$(LDFLAGS)" -o $(BIN) $(PKG)

test:
	go test ./...

# Headless Neovim integration checks (requires nvim 0.12+).
test-nvim:
	bash test/nvim/run.sh

vet:
	go vet ./...

lint: ## Run linter with auto-fix
	$(GOLANGCI_LINT) run --fix ./...

lint-check: ## Run linter without auto-fix (for CI)
	$(GOLANGCI_LINT) run ./...

install: build
	install -d $(PREFIX)/bin
	install -m 0755 $(BIN) $(PREFIX)/bin/$(BIN)

clean:
	rm -f $(BIN)
