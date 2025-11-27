package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// SetupRoutes настраивает все маршруты Catalog Service с использованием Gin
// Применяет Auth middleware для защиты эндпоинтов
func SetupRoutes(catalogHandler *CatalogHandler, authMiddleware *AuthMiddleware) *gin.Engine {
	router := gin.Default()

	// Health check endpoint - публичный, без аутентификации
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"service": "catalog-service",
		})
	})

	// Products endpoints - все требуют аутентификации
	products := router.Group("/products")
	products.Use(authMiddleware.Authenticate()) // Все маршруты требуют JWT токен
	{
		// GET эндпоинты доступны всем аутентифицированным пользователям
		products.GET("", catalogHandler.GetAllProducts) // Список всех товаров
		products.GET("/:id", catalogHandler.GetProduct) // Товар по ID

		// POST, PUT, DELETE только для manager и admin
		products.POST("", authMiddleware.RequireRole("manager", "admin"), catalogHandler.CreateProduct)    // Создать товар
		products.PUT("/:id", authMiddleware.RequireRole("manager", "admin"), catalogHandler.UpdateProduct) // Обновить товар (отправляет в Kafka при изменении цены)
		products.DELETE("/:id", authMiddleware.RequireRole("admin"), catalogHandler.DeleteProduct)         // Удалить товар (только admin)
	}

	// Categories endpoints - все требуют аутентификации
	categories := router.Group("/categories")
	categories.Use(authMiddleware.Authenticate()) // Все маршруты требуют JWT токен
	{
		// GET эндпоинты доступны всем аутентифицированным пользователям
		categories.GET("", catalogHandler.GetAllCategories) // Список категорий (кеш Redis)
		categories.GET("/:id", catalogHandler.GetCategory)  // Категория по ID

		// POST, PUT, DELETE только для manager и admin
		categories.POST("", authMiddleware.RequireRole("manager", "admin"), catalogHandler.CreateCategory)    // Создать категорию
		categories.PUT("/:id", authMiddleware.RequireRole("manager", "admin"), catalogHandler.UpdateCategory) // Обновить категорию
		categories.DELETE("/:id", authMiddleware.RequireRole("admin"), catalogHandler.DeleteCategory)         // Удалить категорию (только admin)
	}

	return router
}
