.DEFAULT_GOAL := help

.PHONY: help build clean test test-integration lint validate docker docker-admin docker-images up-infra up down ps logs tidy e2e-setup e2e

## help: print this help message
help:
	@echo "Usage: make <target>"
	@echo ""
	@sed -n 's/^## //p' $(MAKEFILE_LIST) | column -t -s ':' | sed -e 's/^/  /'

## build: compile all packages and produce binaries in bin/
build:
	@mkdir -p bin
	go build -o bin/aga2aga-enveloper ./cmd/enveloper/
	go build -o bin/aga2aga-admin   ./cmd/admin/
	go build -o bin/aga2aga-gateway ./cmd/gateway/

## clean: remove compiled binaries
clean:
	rm -rf bin/

## test: run all tests
test:
	go test ./...

## test-integration: run integration tests (requires Docker)
test-integration:
	go test -tags integration -timeout 120s ./tests/integration/...

## lint: run go vet and golangci-lint
lint:
	go vet ./...
	@command -v golangci-lint >/dev/null 2>&1 || \
		{ echo "golangci-lint not installed — see https://golangci-lint.run/usage/install/"; exit 1; }
	golangci-lint run

## validate: validate test fixtures with the aga2aga-enveloper CLI
validate:
	@files=$$(find tests/testdata -maxdepth 1 -name '*.md' 2>/dev/null); \
	if [ -n "$$files" ]; then \
		echo "$$files" | xargs go run ./cmd/enveloper validate; \
	else \
		echo "No fixtures found in tests/testdata/ — skipping"; \
	fi

VERSION ?= $(shell git rev-parse --short HEAD)

# Load ADMIN_API_KEY and other secrets from .env.local automatically.
# All docker compose targets use docker/docker-compose.local.yml (ports 8087/3001/6380).
COMPOSE := docker compose -f docker/docker-compose.local.yml --env-file .env.local

## docker: build the gateway image tagged with git SHA and latest
docker:
	docker build -f docker/Dockerfile -t aga2aga-gateway:$(VERSION) -t aga2aga-gateway:latest .

## docker-admin: build the admin image tagged with git SHA and latest
docker-admin:
	docker build -f docker/Dockerfile.admin -t aga2aga-admin:$(VERSION) -t aga2aga-admin:latest .

## docker-images: build both gateway and admin images
docker-images: docker docker-admin

## up-infra: start Redis and Admin UI only (first-time setup: create ADMIN_API_KEY)
up-infra:
	$(COMPOSE) up -d redis admin

## up: start all services (requires ADMIN_API_KEY in .env.local)
up:
	$(COMPOSE) up -d

## down: stop all services
down:
	$(COMPOSE) down

## ps: show service status
ps:
	$(COMPOSE) ps

## logs: tail logs from all services
logs:
	$(COMPOSE) logs --tail=50 --follow

## tidy: sync go.mod and go.sum
tidy:
	go mod tidy

## e2e-setup: install Playwright and Chromium browser (one-time)
e2e-setup:
	cd e2e && npm ci && npx playwright install chromium

## e2e: run Playwright E2E tests against Docker Compose stack (resets DB volume)
e2e: docker-images
	$(COMPOSE) down -v
	$(COMPOSE) up -d --wait
	cd e2e && npx playwright test; status=$$?; \
	cd .. && $(COMPOSE) down -v; exit $$status
