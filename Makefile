.PHONY: help setup dev build test test-coverage migrate-create migrate-up migrate-down docker-up docker-down swag lint fmt

help:
@grep -E '^[a-zA-Z_-]+:.*?## .*$$' \
$(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

setup: ## Install tools and prepare env
go mod tidy
go install github.com/swaggo/swag/cmd/swag@latest
go install github.com/golang-migrate/migrate/v4/cmd/migrate@latest
@[ -f .env ] || cp .env.example .env

fmt: ## Format Go files
gofmt -w $(shell find . -name '*.go' -not -path './vendor/*')

lint: ## Run vet
go vet ./...

dev: ## Run dev server with Air (if installed) or plain go run
@if command -v air >/dev/null 2>&1; then air; else go run ./cmd/api-gateway; fi

build: ## Build binary
go build -o bin/api-gateway ./cmd/api-gateway

test: ## Run tests
go test -v ./...

test-coverage: ## Coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html

migrate-create: ## Create new migration: make migrate-create name=init_schema
migrate create -ext sql -dir migrations -seq $(name)

migrate-up: ## Run migrations up
migrate -path migrations -database "postgresql://postgres:postgres@localhost:5432/admin_panel_sma?sslmode=disable" up

migrate-down: ## Run migrations down
migrate -path migrations -database "postgresql://postgres:postgres@localhost:5432/admin_panel_sma?sslmode=disable" down

docker-up: ## Start Postgres & Redis
docker compose -f docker/docker-compose.yml up -d

docker-down: ## Stop services
docker compose -f docker/docker-compose.yml down

swag: ## Generate swagger docs
swag init -g cmd/api-gateway/main.go -o api/swagger
