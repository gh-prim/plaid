# PLAID — single entry point for dev, tests and builds.
# `make test` runs everything CI checks and must pass before any PR.

PKG     := github.com/gh-prim/plaid/server
VERSION ?= 0.1.0-dev
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
LDFLAGS := -s -w -X $(PKG)/internal/buildinfo.Version=$(VERSION) -X $(PKG)/internal/buildinfo.Commit=$(COMMIT)
WEBUI   := server/internal/webui/dist

.PHONY: help setup dev dev-server dev-web generate check-codegen web-build build \
        test test-server test-web check-i18n lint lint-server lint-web lint-shell \
        docker deb test-deb clean

help: ## List targets
	@grep -E '^[a-z-]+:.*## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*## "}; {printf "  %-14s %s\n", $$1, $$2}'

setup: ## Install web dependencies and Go modules/tools
	pnpm -C web install --frozen-lockfile
	go -C server mod download

dev: ## Run the Go server (:32500, auto-restart) and the Vite dev server (:5173)
	$(MAKE) -j2 dev-server dev-web

dev-server:
	cd server && go tool air

dev-web:
	pnpm -C web dev

generate: ## Regenerate Go server and TS client from api/openapi.yaml
	go -C server generate ./internal/api/
	pnpm -C web generate

check-codegen: ## Fail if generated code is out of date with api/openapi.yaml
	./scripts/check-codegen.sh

web-build: ## Build the web app and copy it into the server embed dir
	pnpm -C web build
	find $(WEBUI) -mindepth 1 ! -name .keep -delete
	cp -R web/dist/. $(WEBUI)/

build: web-build ## Build server/bin/plaid with the web app embedded
	CGO_ENABLED=0 go -C server build -trimpath -ldflags "$(LDFLAGS)" -o bin/plaid ./cmd/plaid

test: lint test-server test-web check-i18n check-codegen ## Everything CI checks
	@echo "✅ all checks passed"

test-server:
	go -C server test -race ./...

test-web:
	pnpm -C web test

check-i18n: ## Missing/unused translation keys
	pnpm -C web i18n:check

lint: lint-server lint-web lint-shell ## All linters

lint-server:
	cd server && golangci-lint run ./...

lint-web:
	pnpm -C web typecheck
	pnpm -C web lint
	pnpm -C web exec prettier --check .

lint-shell:
	shellcheck scripts/*.sh $(wildcard packaging/*.sh packaging/deb/*.sh) $(wildcard .claude/hooks/*.sh)

docker: ## Build the Docker image plaid:dev
	docker build --build-arg VERSION=$(VERSION) --build-arg COMMIT=$(COMMIT) -t plaid:dev .

deb: ## Build snapshot .deb packages and binaries into dist/
	go run github.com/goreleaser/goreleaser/v2@v2.19.0 release --snapshot --clean

test-deb: ## Test the arm64/amd64 .deb in a systemd container (needs `make deb` + Docker)
	./scripts/test-deb.sh $(or $(IMAGE),ubuntu:24.04) $(ARCH)

clean:
	rm -rf server/bin server/tmp web/dist dist
	find $(WEBUI) -mindepth 1 ! -name .keep -delete
