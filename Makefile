.PHONY: help build up down logs clean test migrate

help: ## Показать справку
@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

build: ## Собрать все Docker образы
docker-compose build

up: ## Запустить все сервисы
docker-compose up -d

down: ## Остановить все сервисы
docker-compose down

logs: ## Показать логи всех сервисов
docker-compose logs -f

logs-auth: ## Показать логи Auth Service
docker-compose logs -f auth-service

clean: ## Остановить и удалить все контейнеры, сети и volumes
docker-compose down -v
docker system prune -f

restart: down up ## Перезапустить все сервисы

ps: ## Показать статус сервисов
docker-compose ps

auth-shell: ## Подключиться к контейнеру Auth Service
docker-compose exec auth-service sh

postgres-shell: ## Подключиться к PostgreSQL
docker-compose exec postgres-auth psql -U postgres -d auth_service

redis-cli: ## Подключиться к Redis CLI
docker-compose exec redis redis-cli -a redis_password

test: ## Запустить тесты
go test -v ./...

fmt: ## Форматировать код
go fmt ./...

lint: ## Проверить код линтером
golangci-lint run

deps: ## Установить зависимости
go mod download
go mod verify

tidy: ## Очистить зависимости
go mod tidy

dev-auth: ## Запустить Auth Service локально (нужны PostgreSQL и Redis)
go run auth-service/cmd/main.go
