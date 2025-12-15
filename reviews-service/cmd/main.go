package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"augustberries/pkg/logger"
	"augustberries/reviews-service/internal/app/reviews/config"
	"augustberries/reviews-service/internal/app/reviews/handler"
	"augustberries/reviews-service/internal/app/reviews/infrastructure/messaging"
	"augustberries/reviews-service/internal/app/reviews/repository"
	"augustberries/reviews-service/internal/app/reviews/service"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		os.Exit(1)
	}

	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "info"
	}
	logger.Init("reviews-service", logLevel)

	logstashAddr := os.Getenv("LOGSTASH_ADDR")
	if logstashAddr != "" {
		if err := logger.InitLogstash(logstashAddr, "reviews-service", logLevel); err != nil {
			logger.Warn().Err(err).Msg("Failed to connect to Logstash, using stdout only")
		} else {
			logger.Info().Str("logstash_addr", logstashAddr).Msg("Connected to Logstash")
		}
	}

	mongoClient, err := connectMongoDB(cfg.MongoDB)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to connect to MongoDB")
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := mongoClient.Disconnect(ctx); err != nil {
			logger.Error().Err(err).Msg("Error disconnecting from MongoDB")
		}
	}()
	logger.Info().
		Str("database", cfg.MongoDB.Database).
		Msg("Connected to MongoDB")

	db := mongoClient.Database(cfg.MongoDB.Database)

	kafkaProducer := messaging.NewKafkaProducer(cfg.Kafka.Brokers, cfg.Kafka.Topic)
	defer kafkaProducer.Close()
	logger.Info().
		Str("topic", cfg.Kafka.Topic).
		Msg("Initialized Kafka producer")

	reviewRepo := repository.NewReviewRepository(db)
	reviewService := service.NewReviewService(reviewRepo, kafkaProducer)

	authMiddleware := handler.NewAuthMiddleware(cfg.JWT.Secret)
	reviewHandler := handler.NewReviewHandler(reviewService)
	router := handler.SetupRoutes(reviewHandler, authMiddleware)

	server := &http.Server{
		Addr:         cfg.Server.Address(),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		logger.Info().
			Str("address", cfg.Server.Address()).
			Msg("Starting Reviews Service")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal().Err(err).Msg("Failed to start server")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info().Msg("Shutting down Reviews Service...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Fatal().Err(err).Msg("Server forced to shutdown")
	}

	logger.Info().Msg("Reviews Service stopped gracefully")
}

func connectMongoDB(cfg config.MongoDBConfig) (*mongo.Client, error) {
	clientOptions := options.Client().ApplyURI(cfg.URI)

	var client *mongo.Client
	var err error

	for i := 0; i < 10; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		client, err = mongo.Connect(ctx, clientOptions)
		if err == nil {
			pingCtx, pingCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer pingCancel()

			if err = client.Ping(pingCtx, nil); err == nil {
				return client, nil
			}
		}

		logger.Warn().
			Int("attempt", i+1).
			Err(err).
			Msg("Failed to connect to MongoDB, retrying...")
		time.Sleep(3 * time.Second)
	}

	return nil, err
}
