.DEFAULT_GOAL := help

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
