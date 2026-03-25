.DEFAULT_GOAL := help

.PHONY: help build test lint validate docker tidy

## help: print this help message
help:
	@echo "Usage: make <target>"
	@echo ""
	@sed -n 's/^## //p' $(MAKEFILE_LIST) | column -t -s ':' | sed -e 's/^/  /'

## build: compile all packages
build:
	go build ./...

## test: run all tests
test:
	go test ./...

## lint: run go vet and golangci-lint
lint:
	go vet ./...
	@command -v golangci-lint >/dev/null 2>&1 || \
		{ echo "golangci-lint not installed — see https://golangci-lint.run/usage/install/"; exit 1; }
	golangci-lint run

## validate: validate test fixtures with the aga CLI
validate:
	@files=$$(find tests/testdata -maxdepth 1 -name '*.md' 2>/dev/null); \
	if [ -n "$$files" ]; then \
		echo "$$files" | xargs go run ./cmd/aga2aga validate; \
	else \
		echo "No fixtures found in tests/testdata/ — skipping"; \
	fi

VERSION ?= $(shell git rev-parse --short HEAD)

## docker: build the gateway container image (tagged with git SHA and :latest)
docker:
	docker build -t aga2aga:$(VERSION) -t aga2aga:latest .

## tidy: sync go.mod and go.sum
tidy:
	go mod tidy
