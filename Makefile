.DEFAULT_GOAL := help

APP_NAME := ticket-service
BIN_DIR := bin
BINARY := $(BIN_DIR)/$(APP_NAME)
MODULE := $(shell go list -m)

VERSION ?= dev
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || printf "none")
BUILD_TIME ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

LDFLAGS := \
	-X '$(MODULE)/internal/platform/buildinfo.Version=$(VERSION)' \
	-X '$(MODULE)/internal/platform/buildinfo.Commit=$(COMMIT)' \
	-X '$(MODULE)/internal/platform/buildinfo.BuildTime=$(BUILD_TIME)'

.PHONY: help fmt fmt-check tidy vet test check build run \
	migrate-create migrate-up migrate-down migrate-version

help:
	@echo "Available targets:"
	@echo "  fmt        format Go files"
	@echo "  fmt-check  verify Go formatting"
	@echo "  tidy       synchronize module dependencies"
	@echo "  vet        run static checks"
	@echo "  test       run tests with race detector"
	@echo "  check      run all non-mutating checks"
	@echo "  build      build ticket-service"
	@echo "  run        run ticket-service"
	@echo "  migrate-create  create migration pair; pass name=..."
	@echo "  migrate-up      apply all migrations"
	@echo "  migrate-down    roll back one migration"
	@echo "  migrate-version show current migration version"

fmt:
	gofmt -w .

fmt-check:
	@files="$$(gofmt -l .)"; \
	if [ -n "$$files" ]; then \
		echo "The following files are not formatted:"; \
		echo "$$files"; \
		exit 1; \
	fi

tidy:
	go mod tidy

vet:
	go vet ./...

test:
	go test -race -count=1 ./...

check: fmt-check vet test

build:
	mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 go build \
		-trimpath \
		-ldflags "$(LDFLAGS)" \
		-o $(BINARY) \
		./cmd/ticket-service

run:
	go run ./cmd/ticket-service

MIGRATIONS_DIR := migrations/ticket

migrate-create:
	@test -n "$(name)" || \
		(echo "usage: make migrate-create name=create_example"; exit 1)
	migrate create \
		-ext sql \
		-dir $(MIGRATIONS_DIR) \
		-seq \
		$(name)

migrate-up:
	@test -n "$(DATABASE_URL)" || \
		(echo "DATABASE_URL is required"; exit 1)
	migrate \
		-path $(MIGRATIONS_DIR) \
		-database "$(DATABASE_URL)" \
		up

migrate-down:
	@test -n "$(DATABASE_URL)" || \
		(echo "DATABASE_URL is required"; exit 1)
	migrate \
		-path $(MIGRATIONS_DIR) \
		-database "$(DATABASE_URL)" \
		down 1

migrate-version:
	@test -n "$(DATABASE_URL)" || \
		(echo "DATABASE_URL is required"; exit 1)
	migrate \
		-path $(MIGRATIONS_DIR) \
		-database "$(DATABASE_URL)" \
		version