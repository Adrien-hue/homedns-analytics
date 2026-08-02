.DEFAULT_GOAL := help

DNS_DIR := dns
DNS_BINARY := $(DNS_DIR)/bin/homedns-dns
DNS_MAIN := ./cmd/homedns-dns
DNS_VERSION ?= dev
DNS_COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DNS_BUILD_TIME ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
DNS_MODULE := github.com/Adrien-hue/homedns-analytics/dns
DNS_LDFLAGS := \
	-X '$(DNS_MODULE)/internal/version.Version=$(DNS_VERSION)' \
	-X '$(DNS_MODULE)/internal/version.Commit=$(DNS_COMMIT)' \
	-X '$(DNS_MODULE)/internal/version.BuildTime=$(DNS_BUILD_TIME)'

HOMEDNS_PI_HOST ?= homedns
HOMEDNS_PI_USER ?= joyteaser

.PHONY: help install lint format format-check test build ci pre-commit clean

help: ## Display available commands
	@awk 'BEGIN {FS = ":.*##"; printf "\nHomeDNS Analytics commands:\n\n"} /^[a-zA-Z_-]+:.*?##/ {printf "  %-16s %s\n", $$1, $$2} END {printf "\n"}' $(MAKEFILE_LIST)

install: ## Install project dependencies
	npm --prefix frontend ci
	cd $(DNS_DIR) && go mod download

lint: dns-vet ## Run project linters
	npm --prefix frontend run lint

format: dns-format ## Format project files
	npm --prefix frontend run format

format-check: dns-format-check ## Check project formatting without modifying files
	npm --prefix frontend run format:check

test: dns-test ## Run project tests
	npm --prefix frontend run test:run

build: dns-build ## Build project components
	npm --prefix frontend run build

pre-commit: ## Run all pre-commit hooks
	pre-commit run --all-files

ci: format-check lint test build ## Run all continuous-integration checks

clean: dns-clean ## Remove generated project files
	rm -rf frontend/dist frontend/coverage

# ==============================================================================
# DNS Service
# ==============================================================================

.PHONY: dns-format dns-format-check dns-vet dns-test dns-build dns-run dns-clean

dns-format: ## Format DNS Go source files
	cd $(DNS_DIR) && gofmt -w .

dns-format-check: ## Check DNS Go source formatting
	@files="$$(cd $(DNS_DIR) && gofmt -l .)"; \
	if [ -n "$$files" ]; then \
		echo "The following Go files need formatting:"; \
		echo "$$files"; \
		exit 1; \
	fi

dns-vet: ## Run static analysis on the DNS service
	cd $(DNS_DIR) && go vet ./...

dns-test: ## Run DNS service tests
	cd $(DNS_DIR) && go test ./...

dns-build: ## Build the DNS service
	cd "$(DNS_DIR)" && \
		go build \
			-ldflags "$(DNS_LDFLAGS)" \
			-o "bin/homedns-dns" \
			"$(DNS_MAIN)"

dns-version:
	cd "$(DNS_DIR)" && go run "$(DNS_MAIN)" --version

dns-run: ## Run the DNS service locally
	cd $(DNS_DIR) && go run $(DNS_MAIN)

dns-clean: ## Remove generated DNS files
	@test -n "$(DNS_DIR)"
	@test "$(DNS_DIR)" = "dns"
	rm -rf "$(DNS_DIR)/bin"

# ==============================================================================
# Benchmarks
# ==============================================================================

.PHONY: benchmark benchmark-full benchmark-baseline benchmark-help

benchmark: benchmark-full

benchmark-full:
	./scripts/benchmarks/benchmark.sh full

benchmark-baseline:
	./scripts/benchmarks/benchmark.sh baseline

benchmark-help:
	./scripts/benchmarks/benchmark.sh help

# ==============================================================================
# Release Package
# ==============================================================================

.PHONY: release-package

release-package:
	@./deploy/package-release.sh

# ==============================================================================
# Deploy Package
# ==============================================================================

.PHONY: deploy deploy-info

deploy:
	HOMEDNS_PI_HOST="$(HOMEDNS_PI_HOST)" \
	HOMEDNS_PI_USER="$(HOMEDNS_PI_USER)" \
	./deploy/deploy-release.sh

deploy-info:
	@echo "Target: $(HOMEDNS_PI_USER)@$(HOMEDNS_PI_HOST)"
	@ssh "$(HOMEDNS_PI_USER)@$(HOMEDNS_PI_HOST)" \
		'echo "Current release:" && \
		readlink -f /opt/homedns/current && \
		echo && \
		cat /opt/homedns/current/RELEASE'
