.DEFAULT_GOAL := help

HOMEDNS_PI_HOST ?= homedns
HOMEDNS_PI_USER ?= joyteaser

.PHONY: help install lint format format-check test build ci pre-commit clean

help: ## Display available commands
	@awk 'BEGIN {FS = ":.*##"; printf "\nHomeDNS Analytics commands:\n\n"} /^[a-zA-Z_-]+:.*?##/ {printf "  %-16s %s\n", $$1, $$2} END {printf "\n"}' $(MAKEFILE_LIST)

install: ## Install frontend dependencies
	npm --prefix frontend ci

lint: ## Run project linters
	npm --prefix frontend run lint

format: ## Format project files
	npm --prefix frontend run format

format-check: ## Check project formatting without modifying files
	npm --prefix frontend run format:check

test: ## Run project tests
	npm --prefix frontend run test:run

build: ## Build project components
	npm --prefix frontend run build

pre-commit: ## Run all pre-commit hooks
	pre-commit run --all-files

ci: format-check lint test build ## Run all continuous-integration checks

clean: ## Remove generated frontend files
	rm -rf frontend/dist frontend/coverage

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