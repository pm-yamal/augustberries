package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"augustberries/background-worker-service/internal/app/background-worker/config"
	"augustberries/background-worker-service/internal/app/background-worker/handler"
	"augustberries/background-worker-service/internal/app/background-worker/processor"
	"augustberries/background-worker-service/internal/app/background-worker/repository"
	"augustberries/background-worker-service/internal/app/background-worker/service"
	"augustberries/pkg/logger"
)

func main() {
	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "info"
	}
	logger.Init("background-worker-service", logLevel)

	logstashAddr := os.Getenv("LOGSTASH_ADDR")
	if logstashAddr != "" {
		if err := logger.InitLogstash(logstashAddr, "background-worker-service", logLevel); err != nil {
			logger.Warn().Err(err).Msg("Failed to connect to Logstash, using stdout only")
		} else {
			logger.Info().Str("logstash_addr", logstashAddr).Msg("Connected to Logstash")
		}
	}

	logger.Info().Msg("Starting Background Worker Service...")

	cfg, err := config.Load()
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to load config")
	}

	ctx := context.Background()

	db, err := connectDB(cfg.Database)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to connect to database")
	}
	logger.Info().
		Str("host", cfg.Database.Host).
		Str("database", cfg.Database.DBName).
		Msg("Connected to PostgreSQL")

	redisClient, err := connectRedis(ctx, cfg.Redis)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to connect to Redis")
	}
	defer redisClient.Close()
	logger.Info().
		Str("host", cfg.Redis.Host).
		Int("db", cfg.Redis.DB).
		Msg("Connected to Redis")

	orderRepo := repository.NewOrderRepository(db)
	exchangeRateRepo := repository.NewExchangeRateRepository(redisClient, cfg.Redis.TTL)

	exchangeAPIClient := service.NewExchangeRateAPIClient(
		cfg.ExchangeAPI.URL,
		cfg.ExchangeAPI.Timeout,
	)

	exchangeRateSvc := service.NewExchangeRateService(
		exchangeRateRepo,
		exchangeAPIClient,
	)

	orderProcessingSvc := service.NewOrderProcessingService(
		orderRepo,
		exchangeRateSvc,
	)

	kafkaConsumer := processor.NewKafkaConsumer(
		cfg.Kafka.Brokers,
		cfg.Kafka.Topic,
		cfg.Kafka.GroupID,
		cfg.Kafka.MinBytes,
		cfg.Kafka.MaxBytes,
		orderProcessingSvc,
		exchangeRateSvc,
	)

	kafkaConsumer.Start(ctx)
	defer kafkaConsumer.Stop()
	logger.Info().
		Str("topic", cfg.Kafka.Topic).
		Str("group_id", cfg.Kafka.GroupID).
		Msg("Kafka consumer started")

	cronScheduler := processor.NewCronScheduler(exchangeRateSvc)

	if err := cronScheduler.Start(ctx, cfg.CronSchedule.UpdateRates); err != nil {
		logger.Fatal().Err(err).Msg("Failed to start cron scheduler")
	}
	defer cronScheduler.Stop()
	logger.Info().
		Str("schedule", cfg.CronSchedule.UpdateRates).
		Msg("Cron scheduler started")

	healthHandler := handler.NewHealthCheckHandler(db, redisClient, exchangeRateSvc)

	mux := http.NewServeMux()
	healthHandler.RegisterRoutes(mux)
	mux.Handle("/metrics", promhttp.Handler())

	httpServer := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	go func() {
		logger.Info().
			Str("address", ":8080").
			Msg("Starting healthcheck HTTP server")
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error().Err(err).Msg("HTTP server error")
		}
	}()
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		httpServer.Shutdown(shutdownCtx)
	}()

	logger.Info().Msg("Background Worker Service is running")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info().Msg("Shutting down Background Worker Service...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	<-shutdownCtx.Done()

	logger.Info().Msg("Background Worker Service stopped gracefully")
}

func connectDB(cfg config.DatabaseConfig) (*gorm.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=%s",
		cfg.Host, cfg.User, cfg.Password, cfg.DBName, cfg.Port, cfg.SSLMode,
	)

	gormConfig := &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Info),
	}

	var db *gorm.DB
	var err error

	for i := 0; i < 10; i++ {
		db, err = gorm.Open(postgres.Open(dsn), gormConfig)
		if err == nil {
			sqlDB, sqlErr := db.DB()
			if sqlErr != nil {
				err = sqlErr
			} else {
				if pingErr := sqlDB.Ping(); pingErr != nil {
					err = pingErr
				} else {
					sqlDB.SetMaxOpenConns(10)
					sqlDB.SetMaxIdleConns(5)
					sqlDB.SetConnMaxLifetime(5 * time.Minute)
					sqlDB.SetConnMaxIdleTime(1 * time.Minute)
					return db, nil
				}
			}
		}
		logger.Warn().
			Int("attempt", i+1).
			Err(err).
			Msg("Failed to connect to database, retrying...")
		time.Sleep(3 * time.Second)
	}

	return nil, fmt.Errorf("failed to connect after 10 attempts: %w", err)
}

func connectRedis(ctx context.Context, cfg config.RedisConfig) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         cfg.Address(),
		Password:     cfg.Password,
		DB:           cfg.DB,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolSize:     10,
		MinIdleConns: 5,
	})

	for i := 0; i < 10; i++ {
		if err := client.Ping(ctx).Err(); err == nil {
			return client, nil
		}
		logger.Warn().
			Int("attempt", i+1).
			Msg("Failed to connect to Redis, retrying...")
		time.Sleep(3 * time.Second)
	}

	return nil, fmt.Errorf("failed to connect to Redis after 10 attempts")
}
