package handler

import (
	"net/http"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"augustberries/pkg/logger"
	"augustberries/pkg/metrics"
)

func SetupRoutes(authHandler *AuthHandler, roleHandler *RoleHandler, authMiddleware *AuthMiddleware) *gin.Engine {
	router := gin.New()

	router.Use(gin.Recovery())
	router.Use(logger.GinLoggerMiddleware())
	router.Use(metrics.GinPrometheusMiddleware("auth-service"))

	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"https://*", "http://*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		AllowHeaders:     []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposeHeaders:    []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"service": "auth-service",
		})
	})

	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	auth := router.Group("/auth")
	{
		auth.POST("/register", authHandler.Register)
		auth.POST("/login", authHandler.Login)
		auth.POST("/refresh", authHandler.RefreshToken)
		auth.POST("/validate", authHandler.ValidateToken)

		protected := auth.Group("")
		protected.Use(authMiddleware.Authenticate())
		{
			protected.GET("/me", authHandler.GetMe)
			protected.POST("/logout", authHandler.Logout)
		}
	}

	admin := router.Group("/admin")
	admin.Use(authMiddleware.Authenticate())
	admin.Use(authMiddleware.RequireRole("admin"))
	{
		admin.GET("/users", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"message": "Admin only endpoint - list users",
			})
		})

		roles := admin.Group("/roles")
		{
			roles.GET("", roleHandler.ListRoles)
			roles.GET("/:id", roleHandler.GetRole)
			roles.POST("", roleHandler.CreateRole)
			roles.PUT("/:id", roleHandler.UpdateRole)
			roles.DELETE("/:id", roleHandler.DeleteRole)
			roles.GET("/:id/permissions", roleHandler.GetRolePermissions)
			roles.POST("/:id/permissions", roleHandler.AssignPermissions)
			roles.DELETE("/:id/permissions", roleHandler.RemovePermissions)
		}

		permissions := admin.Group("/permissions")
		{
			permissions.GET("", roleHandler.ListPermissions)
			permissions.POST("", roleHandler.CreatePermission)
			permissions.DELETE("/:id", roleHandler.DeletePermission)
		}
	}

	api := router.Group("/api/products")
	api.Use(authMiddleware.Authenticate())
	{
		api.GET("", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"message": "List products",
			})
		})

		api.POST("", authMiddleware.RequirePermission("product.create"), func(c *gin.Context) {
			c.JSON(http.StatusCreated, gin.H{
				"message": "Product created",
			})
		})
	}

	return router
}
