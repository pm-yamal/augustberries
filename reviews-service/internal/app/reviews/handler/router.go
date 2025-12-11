package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"augustberries/pkg/logger"
	"augustberries/pkg/metrics"
)

func SetupRoutes(reviewHandler *ReviewHandler, authMiddleware *AuthMiddleware) *gin.Engine {
	router := gin.New()

	router.Use(gin.Recovery())

	router.Use(logger.GinLoggerMiddleware())

	router.Use(metrics.GinPrometheusMiddleware("reviews-service"))

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"service": "reviews-service",
		})
	})

	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	reviews := router.Group("/reviews")
	reviews.Use(authMiddleware.Authenticate())
	{

		reviews.POST("/", reviewHandler.CreateReview)
		reviews.GET("/product/:product_id", reviewHandler.GetReviewsByProduct)
		reviews.PATCH("/:review_id", reviewHandler.UpdateReview)
		reviews.DELETE("/:review_id", reviewHandler.DeleteReview)
	}

	return router
}
