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

.PHONY: help fmt fmt-check tidy vet test check build run

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
