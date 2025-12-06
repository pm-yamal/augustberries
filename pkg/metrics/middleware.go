package metrics

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

func GinPrometheusMiddleware(serviceName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.URL.Path == "/metrics" || c.Request.URL.Path == "/health" {
			c.Next()
			return
		}

		start := time.Now()

		HttpRequestsInFlight.WithLabelValues(serviceName).Inc()
		defer HttpRequestsInFlight.WithLabelValues(serviceName).Dec()

		c.Next()

		duration := time.Since(start).Seconds()
		status := strconv.Itoa(c.Writer.Status())
		path := normalizePath(c.Request.URL.Path)

		HttpRequestsTotal.WithLabelValues(serviceName, c.Request.Method, path, status).Inc()
		HttpRequestDuration.WithLabelValues(serviceName, c.Request.Method, path).Observe(duration)
	}
}

func normalizePath(path string) string {
	if len(path) > 100 {
		path = path[:100]
	}
	return path
}
