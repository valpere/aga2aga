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
	golangci-lint run

## validate: validate test fixtures with the aga CLI (no-op until Phase 1)
validate:
	@if ls tests/testdata/*.md 2>/dev/null | grep -q .; then \
		go run ./cmd/aga validate tests/testdata/*.md; \
	else \
		echo "No fixtures found in tests/testdata/ — skipping"; \
	fi

## docker: build the gateway container image
docker:
	docker build -t aga2aga .

## tidy: sync go.mod and go.sum
tidy:
	go mod tidy
