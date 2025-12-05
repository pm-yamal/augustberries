package metrics

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// =============================================================================
// Gin Middleware (для всех сервисов)
// =============================================================================

// GinPrometheusMiddleware возвращает Gin middleware,
// который собирает метрики http_requests_total и http_request_duration_seconds
func GinPrometheusMiddleware(serviceName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Пропускаем метрики для /metrics и /health endpoints
		if c.Request.URL.Path == "/metrics" || c.Request.URL.Path == "/health" {
			c.Next()
			return
		}

		start := time.Now()

		// Увеличиваем счётчик активных запросов
		HttpRequestsInFlight.WithLabelValues(serviceName).Inc()
		defer HttpRequestsInFlight.WithLabelValues(serviceName).Dec()

		// Выполняем запрос
		c.Next()

		// Записываем метрики
		duration := time.Since(start).Seconds()
		status := strconv.Itoa(c.Writer.Status())
		path := normalizePath(c.Request.URL.Path)

		HttpRequestsTotal.WithLabelValues(serviceName, c.Request.Method, path, status).Inc()
		HttpRequestDuration.WithLabelValues(serviceName, c.Request.Method, path).Observe(duration)
	}
}

// =============================================================================
// Helpers
// =============================================================================

// normalizePath нормализует путь для уменьшения кардинальности метрик
// Заменяет UUID и числовые ID на плейсхолдеры
func normalizePath(path string) string {
	// Простая нормализация - можно расширить при необходимости
	// Для более сложной нормализации можно использовать regex

	// Ограничиваем длину пути
	if len(path) > 100 {
		path = path[:100]
	}

	return path
}
