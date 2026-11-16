BIN      := bin/apura
PKG      := ./cmd/apura
VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS  := -s -w -X main.version=$(VERSION)
DEV_DIR  := $(CURDIR)/.dev
MODULE_NOW := $(shell go list -m 2>/dev/null)

.PHONY: setup build build-cgo test test-race e2e lint fmt install dev-shell clean rename help

help: ## show targets
	@grep -E '^[a-z-]+:.*##' $(MAKEFILE_LIST) | awk -F':.*## ' '{printf "  %-12s %s\n", $$1, $$2}'

setup: ## resolve dependencies (first run)
	go mod tidy

build: ## build bin/apura (pure Go SQLite)
	go build -ldflags '$(LDFLAGS)' -o $(BIN) $(PKG)

build-cgo: ## build with the CGO SQLite driver
	go build -tags sqlite_cgo -ldflags '$(LDFLAGS)' -o $(BIN) $(PKG)

test: ## unit tests
	go test ./...

test-race: ## unit tests with the race detector
	go test -race ./...

e2e: build ## end-to-end test in a real interactive fish
	python3 scripts/e2e_fish.py $(BIN)

lint: ## golangci-lint if available, else go vet
	@if command -v golangci-lint >/dev/null; then golangci-lint run; else go vet ./...; fi

fmt: ## format code
	gofmt -w .

install: ## go install into $GOBIN
	go install -ldflags '$(LDFLAGS)' $(PKG)

dev-shell: build ## fish using the local binary and an isolated DB in .dev/
	@mkdir -p $(DEV_DIR)
	APURA_DB=$(DEV_DIR)/apura.db fish -C '$(CURDIR)/$(BIN) init fish | source; echo "apura dev shell (db: .dev/apura.db)"'

clean: ## remove build output and dev DB
	rm -rf bin $(DEV_DIR)

rename: ## change module path: make rename MODULE=github.com/you/apura
	@test -n "$(MODULE)" || (echo "usage: make rename MODULE=github.com/you/apura" && exit 1)
	go mod edit -module $(MODULE)
	grep -rl '$(MODULE_NOW)' --include='*.go' . | xargs sed -i.bak 's#$(MODULE_NOW)#$(MODULE)#g'
	find . -name '*.go.bak' -delete
	@echo "module renamed to $(MODULE)"
