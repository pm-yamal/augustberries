# Stage 1: Build
FROM golang:1.24-alpine AS builder

# Устанавливаем необходимые пакеты для сборки
RUN apk add --no-cache git

# Устанавливаем рабочую директорию
WORKDIR /app

# Копируем go.mod и go.sum для кеширования зависимостей
COPY go.mod go.sum ./

# Загружаем зависимости
RUN go mod download

# Копируем весь проект
COPY . .

# Собираем бинарный файл
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /app/bin/worker ./background-worker-service/cmd/main.go

# Stage 2: Runtime
FROM alpine:latest

# Устанавливаем ca-certificates для HTTPS запросов к API
RUN apk --no-cache add ca-certificates tzdata

# Создаем непривилегированного пользователя
RUN addgroup -g 1000 appgroup && \
    adduser -D -u 1000 -G appgroup appuser

WORKDIR /app

# Копируем бинарный файл из builder stage
COPY --from=builder /app/bin/worker /app/worker

# Меняем владельца файлов
RUN chown -R appuser:appgroup /app

# Переключаемся на непривилегированного пользователя
USER appuser

# Запускаем worker
CMD ["/app/worker"]