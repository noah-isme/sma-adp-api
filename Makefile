DB_URL ?= postgresql://postgres:postgres@localhost:5432/admin_panel_sma?sslmode=disable

.PHONY: help setup dev build test test-coverage migrate-create migrate-up migrate-down docker-up docker-down swag validate-swagger-routes compatibility-smoke lint fmt contract-test shadow-compare toggle-go rollback seed seed-reset decommission-preflight migration-integrity backup-verify deploy-validate rollback-release

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
	GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod go vet ./...

contract-test: ## Run default Postman smoke contract via Newman (requires Docker and ACCESS_TOKEN)
	@BASE_URL=$${BASE_URL:-http://localhost:8080/api/v1}; \
	GATEWAY_URL=$${GATEWAY_URL:-$${BASE_URL%/api/v1}}; \
	ACCESS_TOKEN=$${ACCESS_TOKEN:-}; \
	if [ -z "$$ACCESS_TOKEN" ]; then echo "ACCESS_TOKEN is required for Core Protected Smoke"; exit 2; fi; \
	docker run --rm -e BASE_URL=$$BASE_URL -e GATEWAY_URL=$$GATEWAY_URL -e ACCESS_TOKEN=$$ACCESS_TOKEN \
	-v $(CURDIR)/tests/contract:/etc/newman postman/newman:alpine \
	run contract.postman_collection.json \
	--env-var baseUrl=$$BASE_URL \
	--env-var gatewayUrl=$$GATEWAY_URL \
	--env-var accessToken=$$ACCESS_TOKEN \
	--folder "Public Gateway" \
	--folder "Core Protected Smoke"

shadow-compare: ## Compare legacy vs Go API responses for critical endpoints
	GO_BASE_URL=$${GO_BASE_URL:-http://localhost:8080}; \
	LEGACY_BASE_URL=$${LEGACY_BASE_URL:-http://localhost:3000}; \
	go run ./scripts/shadow_compare --go-base $$GO_BASE_URL --legacy-base $$LEGACY_BASE_URL

toggle-go: ## Toggle ROUTE_TO_GO flag in .env (usage: make toggle-go value=true|false)
	@[ -n "$(value)" ] || (echo "Usage: make toggle-go value=true|false" && exit 1)
	@bash scripts/toggle_go.sh $(value)

rollback: ## Automated rollback: disable Go routing, flush Redis cache, and run compatibility smoke test
	@bash scripts/toggle_go.sh false
	redis-cli flushall 2>/dev/null || docker exec sma_redis redis-cli flushall 2>/dev/null || echo "Redis cache flush bypassed or failed"
	$(MAKE) compatibility-smoke

dev: ## Run dev server with Air (if installed) or plain go run
	@if command -v air >/dev/null 2>&1; then air; else go run ./cmd/api-gateway; fi

build: ## Build binary
	GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod go build -o bin/api-gateway ./cmd/api-gateway

test: ## Run tests
	GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod go test -v ./...

test-coverage: ## Coverage report
	GOCACHE=/tmp/go-build GOMODCACHE=/tmp/go-mod go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

migrate-create: ## Create new migration: make migrate-create name=init_schema
	migrate create -ext sql -dir migrations -seq $(name)

migrate-up: ## Run migrations up
	migrate -path migrations -database "$(DB_URL)" up

migrate-down: ## Run migrations down
	migrate -path migrations -database "$(DB_URL)" down

seed: ## Seed database with test data
	psql $(DB_URL) -f scripts/seed.sql

seed-reset: ## Reset database schema, run migrations, and seed data
	psql $(DB_URL) -c "DROP SCHEMA public CASCADE; CREATE SCHEMA public;"
	$(MAKE) migrate-up
	$(MAKE) seed

docker-up: ## Start Postgres & Redis
	docker compose -f docker/docker-compose.yml up -d

docker-down: ## Stop services
	docker compose -f docker/docker-compose.yml down

swag: ## Generate swagger docs
	swag init -g cmd/api-gateway/main.go -o api/swagger --parseDependency --parseInternal

validate-swagger-routes: ## Verify every API gateway route has a generated Swagger path
	python3 scripts/validate_swagger_routes.py

compatibility-smoke: ## Verify compatibility routes and optionally run seeded HTTP smoke
	python3 scripts/compatibility_smoke.py

decommission-preflight: ## Validate read-only cutover and legacy retirement prerequisites
	python3 scripts/decommission_preflight.py $(PREFLIGHT_ARGS)

migration-integrity: ## Run read-only migration and referential-integrity checks
	go run ./scripts/migration_integrity --dsn "$(DB_URL)" $(INTEGRITY_ARGS)

backup-verify: ## Create/check a PostgreSQL backup (use BACKUP_ARGS for restore drills)
	DATABASE_URL="$(DB_URL)" ./deploy/verify-backup.sh $(BACKUP_ARGS)

deploy-validate: ## Validate production Compose, Nginx, image, and monitoring artifacts
	python3 deploy/validate.py

rollback-release: ## Validate or execute a digest-pinned release rollback
	./deploy/rollback.sh $(RELEASE_ENV) $(ROLLBACK_ARGS)
