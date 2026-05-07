# Rinha de Backend 2023 Q3 - Go Implementation
# Usage: make help

# --- Configuration ---
APP_NAME    := api
BIN_DIR     := bin
BINARY      := $(BIN_DIR)/$(APP_NAME)
GO          := go
GOFLAGS     := -race
DOCKER_COMP := docker compose
PORT        := 9999
BASE_URL    := http://localhost:$(PORT)

# Build flags
LDFLAGS := -s -w
BUILD_FLAGS := -ldflags "$(LDFLAGS)"

# --- Default target ---
.DEFAULT_GOAL := help

# --- Targets ---

.PHONY: help
help: ## Show this help message
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

.PHONY: build
build: $(BINARY) ## Build the application binary

$(BINARY): $(shell find . -name '*.go' -not -path './tests/*') go.mod go.sum
	@mkdir -p $(BIN_DIR)
	$(GO) build $(BUILD_FLAGS) -o $@ ./cmd/api/

.PHONY: run
run: build ## Build and run locally
	$(BINARY)

.PHONY: test
test: test-unit test-integration ## Run all tests

.PHONY: test-unit
test-unit: ## Run unit tests with race detection
	$(GO) test $(GOFLAGS) ./internal/... -v

.PHONY: test-integration
test-integration: ## Run integration tests (requires Docker)
	$(GO) test $(GOFLAGS) ./tests/ -v -count=1

.PHONY: test-short
test-short: ## Run only unit tests (skip integration)
	$(GO) test $(GOFLAGS) ./... -short -v

.PHONY: lint
lint: ## Run static analysis
	$(GO) vet ./...

.PHONY: fmt
fmt: ## Format all Go source files
	$(GO) fmt ./...

.PHONY: tidy
tidy: ## Tidy and verify module dependencies
	$(GO) mod tidy
	$(GO) mod verify

.PHONY: clean
clean: ## Remove build artifacts and stop containers
	rm -rf $(BIN_DIR)
	$(DOCKER_COMP) down -v --remove-orphans 2>/dev/null || true

.PHONY: up
up: ## Start full Docker stack (build + detach)
	$(DOCKER_COMP) up -d --build

.PHONY: down
down: ## Stop Docker stack
	$(DOCKER_COMP) down

.PHONY: restart
restart: down up ## Restart Docker stack

.PHONY: logs
logs: ## Tail all container logs
	$(DOCKER_COMP) logs -f

.PHONY: ps
ps: ## Show container status
	$(DOCKER_COMP) ps

.PHONY: smoke
smoke: ## Run smoke test against running stack
	@echo "=== POST /pessoas ==="
	@curl -sf -w "\nHTTP %{http_code}\n" -X POST $(BASE_URL)/pessoas \
		-H "Content-Type: application/json" \
		-d '{"apelido":"smoke-$(shell date +%s)","nome":"Smoke Test","nascimento":"2000-01-01","stack":["Go"]}' || \
		(echo "FAILED - is the stack running? (make up)" && exit 1)
	@echo "\n=== GET /contagem-pessoas ==="
	@curl -sf $(BASE_URL)/contagem-pessoas
	@echo "\n"
