package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"augustberries/pkg/logger"
	"augustberries/pkg/metrics"
)

func SetupRoutes(catalogHandler *CatalogHandler, authMiddleware *AuthMiddleware) *gin.Engine {
	router := gin.New()

	router.Use(gin.Recovery())

	router.Use(logger.GinLoggerMiddleware())

	router.Use(metrics.GinPrometheusMiddleware("catalog-service"))

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"service": "catalog-service",
		})
	})

	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	products := router.Group("/products")
	products.Use(authMiddleware.Authenticate())
	{
		products.GET("", catalogHandler.GetAllProducts)
		products.GET("/:id", catalogHandler.GetProduct)

		products.POST("", authMiddleware.RequireRole("manager", "admin"), catalogHandler.CreateProduct)
		products.PUT("/:id", authMiddleware.RequireRole("manager", "admin"), catalogHandler.UpdateProduct)
		products.DELETE("/:id", authMiddleware.RequireRole("admin"), catalogHandler.DeleteProduct)
	}

	categories := router.Group("/categories")
	categories.Use(authMiddleware.Authenticate())
	{
		categories.GET("", catalogHandler.GetAllCategories)
		categories.GET("/:id", catalogHandler.GetCategory)

		categories.POST("", authMiddleware.RequireRole("manager", "admin"), catalogHandler.CreateCategory)
		categories.PUT("/:id", authMiddleware.RequireRole("manager", "admin"), catalogHandler.UpdateCategory)
		categories.DELETE("/:id", authMiddleware.RequireRole("admin"), catalogHandler.DeleteCategory)
	}

	return router
}
