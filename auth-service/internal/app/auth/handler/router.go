package handler

import (
	"net/http"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"augustberries/pkg/logger"
	"augustberries/pkg/metrics"
)

// SetupRoutes настраивает все маршруты приложения с использованием Gin
func SetupRoutes(authHandler *AuthHandler, authMiddleware *AuthMiddleware) *gin.Engine {
	router := gin.New()

	// Recovery middleware для обработки panic
	router.Use(gin.Recovery())

	// JSON logging middleware для HTTP-запросов (ELK Stack)
	router.Use(logger.GinLoggerMiddleware())

	// Prometheus metrics middleware
	router.Use(metrics.GinPrometheusMiddleware("auth-service"))

	// CORS настройки
	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"https://*", "http://*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposeHeaders:    []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"service": "auth-service",
		})
	})

	// Prometheus metrics endpoint
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Публичные эндпоинты (без аутентификации)
	auth := router.Group("/auth")
	{
		auth.POST("/register", authHandler.Register)
		auth.POST("/login", authHandler.Login)
		auth.POST("/refresh", authHandler.RefreshToken)
		auth.POST("/validate", authHandler.ValidateToken)

		// Защищенные эндпоинты (требуют аутентификации)
		protected := auth.Group("")
		protected.Use(authMiddleware.Authenticate())
		{
			protected.GET("/me", authHandler.GetMe)
			protected.POST("/logout", authHandler.Logout)
		}
	}

	// Admin эндпоинты - только для администраторов
	admin := router.Group("/admin")
	admin.Use(authMiddleware.Authenticate())
	admin.Use(authMiddleware.RequireRole("admin"))
	{
		admin.GET("/users", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"message": "Admin only endpoint - list users",
			})
		})
	}

	// API эндпоинты с проверкой разрешений
	api := router.Group("/api/products")
	api.Use(authMiddleware.Authenticate())
	{
		// Любой авторизованный пользователь может читать
		api.GET("", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"message": "List products",
			})
		})

		// Только с разрешением product.create
		api.POST("", authMiddleware.RequirePermission("product.create"), func(c *gin.Context) {
			c.JSON(http.StatusCreated, gin.H{
				"message": "Product created",
			})
		})
	}

	return router
}
