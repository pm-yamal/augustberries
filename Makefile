.PHONY: help build up down logs clean test migrate

help: ## Показать справку
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

# ==================== DOCKER COMPOSE ====================

build: ## Собрать все Docker образы
	docker-compose build

up: ## Запустить все сервисы
	docker-compose up -d

down: ## Остановить все сервисы
	docker-compose down

restart: down up ## Перезапустить все сервисы

ps: ## Показать статус сервисов
	docker-compose ps

clean: ## Остановить и удалить все контейнеры, сети и volumes
	docker-compose down -v
	docker system prune -f

# ==================== LOGS ====================

logs: ## Показать логи всех сервисов
	docker-compose logs -f

logs-auth: ## Показать логи Auth Service
	docker-compose logs -f auth-service

logs-catalog: ## Показать логи Catalog Service
	docker-compose logs -f catalog-service

logs-orders: ## Показать логи Orders Service
	docker-compose logs -f orders-service

# ==================== SHELLS ====================

auth-shell: ## Подключиться к контейнеру Auth Service
	docker-compose exec auth-service sh

catalog-shell: ## Подключиться к контейнеру Catalog Service
	docker-compose exec catalog-service sh

orders-shell: ## Подключиться к контейнеру Orders Service
	docker-compose exec orders-service sh

# ==================== DATABASE ====================

postgres-auth-shell: ## Подключиться к PostgreSQL Auth
	docker-compose exec postgres-auth psql -U postgres -d auth_service

postgres-catalog-shell: ## Подключиться к PostgreSQL Catalog
	docker-compose exec postgres-catalog psql -U postgres -d catalog_service

postgres-orders-shell: ## Подключиться к PostgreSQL Orders
	docker-compose exec postgres-orders psql -U postgres -d orders_service

redis-cli: ## Подключиться к Redis CLI
	docker-compose exec redis redis-cli -a redis_password

# ==================== KAFKA ====================

kafka-topics: ## Показать все топики Kafka
	docker-compose exec kafka kafka-topics --list --bootstrap-server localhost:9092

kafka-create-topic: ## Создать топик (использовать: make kafka-create-topic TOPIC=my_topic)
	docker-compose exec kafka kafka-topics --create --bootstrap-server localhost:9092 --topic $(TOPIC) --partitions 3 --replication-factor 1

kafka-describe-topic: ## Описать топик (использовать: make kafka-describe-topic TOPIC=order_events)
	docker-compose exec kafka kafka-topics --describe --bootstrap-server localhost:9092 --topic $(TOPIC)

kafka-consume-orders: ## Читать сообщения из топика order_events
	docker-compose exec kafka kafka-console-consumer --bootstrap-server localhost:9092 --topic order_events --from-beginning

kafka-consume-products: ## Читать сообщения из топика product_events
	docker-compose exec kafka kafka-console-consumer --bootstrap-server localhost:9092 --topic product_events --from-beginning

# ==================== GO COMMANDS ====================

test: ## Запустить тесты
	go test -v ./...

test-coverage: ## Запустить тесты с покрытием
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

fmt: ## Форматировать код
	go fmt ./...

lint: ## Проверить код линтером
	golangci-lint run

deps: ## Установить зависимости
	go mod download
	go mod verify

tidy: ## Очистить зависимости
	go mod tidy

# ==================== LOCAL DEVELOPMENT ====================

dev-auth: ## Запустить Auth Service локально (нужны PostgreSQL и Redis)
	go run auth-service/cmd/main.go

dev-catalog: ## Запустить Catalog Service локально (нужны PostgreSQL, Redis и Kafka)
	go run catalog-service/cmd/main.go

dev-orders: ## Запустить Orders Service локально (нужны PostgreSQL и Kafka)
	go run orders-service/cmd/main.go

# ==================== MIGRATIONS ====================

migrate-auth: ## Применить миграции для Auth Service
	@echo "Миграции применяются автоматически при старте контейнера"
	@echo "Или используйте: docker-compose exec postgres-auth psql -U postgres -d auth_service -f /docker-entrypoint-initdb.d/001_init.sql"

migrate-catalog: ## Применить миграции для Catalog Service
	@echo "Миграции применяются автоматически при старте контейнера"
	@echo "Или используйте: docker-compose exec postgres-catalog psql -U postgres -d catalog_service -f /docker-entrypoint-initdb.d/001_init.sql"

migrate-orders: ## Применить миграции для Orders Service
	@echo "Миграции применяются автоматически при старте контейнера"
	@echo "Или используйте: docker-compose exec postgres-orders psql -U postgres -d orders_service -f /docker-entrypoint-initdb.d/001_init.sql"

# ==================== HEALTH CHECKS ====================

health: ## Проверить здоровье всех сервисов
	@echo "=== Auth Service ==="
	@curl -s http://localhost:8080/auth/health || echo "Auth Service не отвечает"
	@echo "\n=== Catalog Service ==="
	@curl -s http://localhost:8081/health || echo "Catalog Service не отвечает"
	@echo "\n=== Orders Service ==="
	@curl -s http://localhost:8082/health || echo "Orders Service не отвечает"

# ==================== QUICK ACTIONS ====================

quick-start: build up health ## Быстрый старт: собрать, запустить и проверить

rebuild: down build up ## Пересобрать и перезапустить все

rebuild-orders: ## Пересобрать только Orders Service
	docker-compose build orders-service
	docker-compose up -d orders-service
	docker-compose logs -f orders-service

rebuild-catalog: ## Пересобрать только Catalog Service
	docker-compose build catalog-service
	docker-compose up -d catalog-service
	docker-compose logs -f catalog-service

rebuild-auth: ## Пересобрать только Auth Service
	docker-compose build auth-service
	docker-compose up -d auth-service
	docker-compose logs -f auth-service
