.PHONY: dev dev-frontend build build-frontend test test-frontend test-all \
       lint lint-frontend fmt migrate migrate-create \
       docker-build docker-up docker-down clean help

# ──────────────────────────────────────────────
# Variables
# ──────────────────────────────────────────────
BINARY_NAME := me-lab
GO_FILES     := $(shell find cmd internal migrations -name '*.go')
GO_PKGS      := ./cmd/... ./internal/... ./migrations/...
COVERAGE     := coverage.out

# ──────────────────────────────────────────────
# Development
# ──────────────────────────────────────────────

## dev: Run API server with hot reload
dev:
	@echo "▸ Starting API server..."
	go run ./cmd/api

## dev-frontend: Run frontend dev server
dev-frontend:
	@echo "▸ Starting frontend dev server..."
	cd web && npm run dev

# ──────────────────────────────────────────────
# Build
# ──────────────────────────────────────────────

## build: Build Go binary
build:
	@echo "▸ Building $(BINARY_NAME)..."
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o $(BINARY_NAME) ./cmd/api

## build-frontend: Build frontend production bundle
build-frontend:
	@echo "▸ Building frontend..."
	cd web && npm run build

# ──────────────────────────────────────────────
# Testing
# ──────────────────────────────────────────────

## test: Run Go tests with coverage
test:
	@echo "▸ Running Go tests..."
	go test -race -coverprofile=$(COVERAGE) -covermode=atomic $(GO_PKGS)
	@go tool cover -func=$(COVERAGE) | tail -1

## test-frontend: Run frontend tests
test-frontend:
	@echo "▸ Running frontend tests..."
	cd web && npm run test

## test-all: Run all tests (backend + frontend)
test-all: test test-frontend

# ──────────────────────────────────────────────
# Linting & Formatting
# ──────────────────────────────────────────────

## lint: Run go vet and staticcheck
lint:
	@echo "▸ Running go vet..."
	go vet $(GO_PKGS)
	@echo "▸ Running staticcheck..."
	staticcheck $(GO_PKGS)

## lint-frontend: Run eslint and TypeScript type-check
lint-frontend:
	@echo "▸ Running ESLint..."
	cd web && npm run lint
	@echo "▸ Running TypeScript type-check..."
	cd web && npm run type-check

## fmt: Format Go code with gofmt
fmt:
	@echo "▸ Formatting Go code..."
	gofmt -s -w $(GO_FILES)

# ──────────────────────────────────────────────
# Database
# ──────────────────────────────────────────────

## migrate: Run database migrations
migrate:
	@echo "▸ Running migrations..."
	go run ./cmd/migrate up

## migrate-create: Create a new migration (usage: make migrate-create NAME=create_users)
migrate-create:
	@test -n "$(NAME)" || (echo "Error: NAME is required. Usage: make migrate-create NAME=create_users" && exit 1)
	@echo "▸ Creating migration: $(NAME)"
	go run ./cmd/migrate create $(NAME) sql

# ──────────────────────────────────────────────
# Docker
# ──────────────────────────────────────────────

## docker-build: Build Docker images
docker-build:
	@echo "▸ Building Docker images..."
	docker compose build

## docker-up: Start Docker Compose services
docker-up:
	@echo "▸ Starting Docker Compose..."
	docker compose up -d

## docker-down: Stop Docker Compose services
docker-down:
	@echo "▸ Stopping Docker Compose..."
	docker compose down

# ──────────────────────────────────────────────
# Cleanup
# ──────────────────────────────────────────────

## clean: Remove build artifacts
clean:
	@echo "▸ Cleaning..."
	rm -f $(BINARY_NAME)
	rm -f $(COVERAGE) coverage.html
	rm -rf bin/
	cd web && rm -rf dist/ .vite/

# ──────────────────────────────────────────────
# Help
# ──────────────────────────────────────────────

## help: Show available targets
help:
	@echo "ME Lab — Available targets:"
	@echo ""
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/^## /  /'
