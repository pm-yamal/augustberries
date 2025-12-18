.PHONY: help build up down logs clean test coverage

help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

build: ## Build all Docker images
	docker-compose build

up: ## Start all services
	docker-compose up -d

down: ## Stop all services
	docker-compose down

restart: down up ## Restart all services

ps: ## Show services status
	docker-compose ps

clean: ## Stop and remove all containers, networks and volumes
	docker-compose down -v
	docker system prune -f

logs: ## Show all services logs
	docker-compose logs -f

logs-auth: ## Show Auth Service logs
	docker-compose logs -f auth-service

logs-catalog: ## Show Catalog Service logs
	docker-compose logs -f catalog-service

logs-orders: ## Show Orders Service logs
	docker-compose logs -f orders-service

logs-reviews: ## Show Reviews Service logs
	docker-compose logs -f reviews-service

logs-worker: ## Show Background Worker logs
	docker-compose logs -f background-worker-service

auth-shell: ## Connect to Auth Service container
	docker-compose exec auth-service sh

catalog-shell: ## Connect to Catalog Service container
	docker-compose exec catalog-service sh

orders-shell: ## Connect to Orders Service container
	docker-compose exec orders-service sh

auth-db-shell: ## Connect to Auth PostgreSQL
	docker-compose exec auth_db psql -U postgres -d auth_service

catalog-db-shell: ## Connect to Catalog PostgreSQL
	docker-compose exec catalog_db psql -U postgres -d catalog_service

orders-db-shell: ## Connect to Orders PostgreSQL
	docker-compose exec orders_db psql -U postgres -d orders_service

redis-cli: ## Connect to Redis CLI
	docker-compose exec redis redis-cli -a redis_password

kafka-topics: ## Show all Kafka topics
	docker-compose exec kafka kafka-topics --list --bootstrap-server localhost:9092

kafka-consume-orders: ## Read messages from order_events topic
	docker-compose exec kafka kafka-console-consumer --bootstrap-server localhost:9092 --topic order_events --from-beginning

kafka-consume-products: ## Read messages from product_events topic
	docker-compose exec kafka kafka-console-consumer --bootstrap-server localhost:9092 --topic product_events --from-beginning

test: ## Run all tests
	go test ./...

test-verbose: ## Run tests with verbose output
	go test -v ./...

test-short: ## Run tests in short mode
	go test -short ./...

coverage: ## Run tests with coverage report
	@echo "Running tests with coverage..."
	@go test -coverprofile=coverage.out -covermode=atomic ./...
	@echo ""
	@echo "=== Coverage Summary ==="
	@go tool cover -func=coverage.out | tail -1
	@echo ""
	@echo "Generating HTML report..."
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report saved to: coverage.html"

coverage-func: ## Show coverage by function
	@go test -coverprofile=coverage.out ./...
	@go tool cover -func=coverage.out

coverage-check: ## Check if coverage meets threshold (default 50%)
	@go test -coverprofile=coverage.out ./... 2>/dev/null
	@threshold=$${COVERAGE_THRESHOLD:-50}; \
	coverage=$$(go tool cover -func=coverage.out | tail -1 | awk '{print $$3}' | sed 's/%//'); \
	echo "Coverage: $$coverage%, Threshold: $$threshold%"; \
	result=$$(echo "$$coverage >= $$threshold" | bc -l); \
	if [ "$$result" -eq 1 ]; then echo "PASS"; else echo "FAIL"; exit 1; fi

fmt: ## Format code
	go fmt ./...

lint: ## Run linter
	golangci-lint run

deps: ## Download dependencies
	go mod download
	go mod verify

tidy: ## Cleanup dependencies
	go mod tidy

dev-auth: ## Run Auth Service locally
	go run auth-service/cmd/main.go

dev-catalog: ## Run Catalog Service locally
	go run catalog-service/cmd/main.go

dev-orders: ## Run Orders Service locally
	go run orders-service/cmd/main.go

dev-reviews: ## Run Reviews Service locally
	go run reviews-service/cmd/main.go

dev-worker: ## Run Background Worker locally
	go run background-worker-service/cmd/main.go

health: ## Check all services health
	@echo "=== Auth Service ===" && curl -sf http://localhost:8080/health || echo "Not responding"
	@echo "=== Catalog Service ===" && curl -sf http://localhost:8081/health || echo "Not responding"
	@echo "=== Orders Service ===" && curl -sf http://localhost:8082/health || echo "Not responding"
	@echo "=== Reviews Service ===" && curl -sf http://localhost:8083/health || echo "Not responding"

quick-start: build up health ## Quick start: build, run and check

rebuild: down build up ## Rebuild and restart all
