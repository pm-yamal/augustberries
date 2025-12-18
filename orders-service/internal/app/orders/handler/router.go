package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"augustberries/pkg/logger"
	"augustberries/pkg/metrics"
)

func SetupRoutes(orderHandler *OrderHandler, authMiddleware *AuthMiddleware) *gin.Engine {
	router := gin.New()

	router.Use(gin.Recovery())

	router.Use(logger.GinLoggerMiddleware())

	router.Use(metrics.GinPrometheusMiddleware("orders-service"))

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"service": "orders-service",
		})
	})

	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	orders := router.Group("/orders")
	orders.Use(authMiddleware.Authenticate())
	{
		orders.POST("/", orderHandler.CreateOrder)
		orders.GET("/", orderHandler.GetUserOrders)
		orders.GET("/:id", orderHandler.GetOrder)
		orders.PATCH("/:id", orderHandler.UpdateOrderStatus)
		orders.DELETE("/:id", orderHandler.DeleteOrder)
	}

	return router
}
