.PHONY: help build test lint fmt gen check
.DEFAULT_GOAL := help

# golangci-lint v2 is required for lint and fmt (config: .golangci.yml).
# Install: https://golangci-lint.run/welcome/install/
GOLANGCI := golangci-lint

help: ## list targets
	@grep -E '^[a-z]+:.*##' $(MAKEFILE_LIST) | awk -F':.*## ' '{printf "  %-8s %s\n", $$1, $$2}'

build: ## build ./bin/vantage
	go build -o bin/vantage ./cmd/vantage

test: ## run tests (uses isolated tmux sockets only)
	go test ./...

lint: ## run linters and check formatting (gofumpt, goimports)
	@command -v $(GOLANGCI) >/dev/null || (echo "golangci-lint v2 not found: see Makefile header"; exit 1)
	$(GOLANGCI) run ./...
	@test -z "$$($(GOLANGCI) fmt --diff ./... 2>&1)" || ($(GOLANGCI) fmt --diff ./...; echo "formatting needed: run make fmt"; exit 1)

fmt: ## format code with gofumpt and goimports
	@command -v $(GOLANGCI) >/dev/null || (echo "golangci-lint v2 not found: see Makefile header"; exit 1)
	$(GOLANGCI) fmt ./...

gen: ## regenerate sqlc code (internal/store/gen)
	go tool sqlc generate

check: lint ## lint + format check + vet + tests; must pass before committing
	go vet ./...
	go test ./...
	@go tool sqlc generate && git diff --quiet -- internal/store/gen || (echo "sqlc output is stale: run make gen and commit"; exit 1)
