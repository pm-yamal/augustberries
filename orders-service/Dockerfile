# Этап сборки
FROM golang:1.24.9-alpine AS builder

# Устанавливаем необходимые инструменты
RUN apk add --no-cache git

# Устанавливаем рабочую директорию
WORKDIR /app

# Копируем go.mod и go.sum для кеширования зависимостей
COPY go.mod go.sum ./

# Загружаем зависимости
RUN go mod download

# Копируем исходный код сервиса
COPY orders-service/ ./orders-service/

# Собираем приложение
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /orders-service ./orders-service/cmd/main.go

# Этап запуска
FROM alpine:latest

# Устанавливаем CA сертификаты для HTTPS и timezone data
RUN apk --no-cache add ca-certificates tzdata

# Создаем непривилегированного пользователя
RUN addgroup -g 1000 appuser && \
    adduser -D -u 1000 -G appuser appuser

WORKDIR /app

# Копируем скомпилированное приложение из builder
COPY --from=builder /orders-service /app/orders-service

# Копируем миграции
COPY orders-service/migration/ /app/migration/

# Меняем владельца файлов
RUN chown -R appuser:appuser /app

# Переключаемся на непривилегированного пользователя
USER appuser

# Открываем порт
EXPOSE 8082

# Запускаем приложение
CMD ["/app/orders-service"]
