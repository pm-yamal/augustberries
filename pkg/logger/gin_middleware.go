package logger

import (
	"time"

	"github.com/gin-gonic/gin"
)

func GinLoggerMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = generateRequestID()
		}
		c.Header("X-Request-ID", requestID)

		c.Next()

		duration := time.Since(start)
		status := c.Writer.Status()
		size := c.Writer.Size()

		var event = Info()
		if status >= 500 {
			event = Error()
		} else if status >= 400 {
			event = Warn()
		}

		logEvent := event.
			Str("request_id", requestID).
			Str("method", c.Request.Method).
			Str("path", path).
			Str("query", query).
			Str("remote_addr", c.ClientIP()).
			Str("user_agent", c.Request.UserAgent()).
			Int("status", status).
			Int("size", size).
			Float64("duration_ms", float64(duration.Milliseconds()))

		if len(c.Errors) > 0 {
			logEvent.Str("error", c.Errors.String())
		}

		logEvent.Msg("HTTP request")
	}
}
