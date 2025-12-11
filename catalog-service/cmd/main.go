package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"augustberries/catalog-service/internal/app/catalog/config"
	"augustberries/catalog-service/internal/app/catalog/handler"
	"augustberries/catalog-service/internal/app/catalog/repository"
	"augustberries/catalog-service/internal/app/catalog/service"
	"augustberries/catalog-service/internal/app/catalog/util"
	"augustberries/pkg/logger"
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
	logger.Init("catalog-service", logLevel)

	logstashAddr := os.Getenv("LOGSTASH_ADDR")
	if logstashAddr != "" {
		if err := logger.InitLogstash(logstashAddr, "catalog-service", logLevel); err != nil {
			logger.Warn().Err(err).Msg("Failed to connect to Logstash, using stdout only")
		} else {
			logger.Info().Str("logstash_addr", logstashAddr).Msg("Connected to Logstash")
		}
	}

	db, err := connectDB(cfg.Database)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to connect to database")
	}
	logger.Info().
		Str("host", cfg.Database.Host).
		Str("database", cfg.Database.DBName).
		Msg("Connected to PostgreSQL")

	redisClient, err := util.NewRedisClient(
		cfg.Redis.Address(),
		cfg.Redis.Password,
		cfg.Redis.DB,
	)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to connect to Redis")
	}
	defer redisClient.Close()
	logger.Info().
		Str("host", cfg.Redis.Host).
		Int("db", cfg.Redis.DB).
		Msg("Connected to Redis")

	kafkaProducer := util.NewKafkaProducer(cfg.Kafka.Brokers, cfg.Kafka.Topic)
	defer kafkaProducer.Close()
	logger.Info().
		Str("topic", cfg.Kafka.Topic).
		Msg("Initialized Kafka producer")

	categoryRepo := repository.NewCategoryRepository(db)
	productRepo := repository.NewProductRepository(db)

	catalogService := service.NewCatalogService(
		categoryRepo,
		productRepo,
		redisClient,
		kafkaProducer,
	)

	authMiddleware := handler.NewAuthMiddleware(cfg.JWT.Secret)
	catalogHandler := handler.NewCatalogHandler(catalogService)
	router := handler.SetupRoutes(catalogHandler, authMiddleware)

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
			Msg("Starting Catalog Service")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal().Err(err).Msg("Failed to start server")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info().Msg("Shutting down Catalog Service...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Fatal().Err(err).Msg("Server forced to shutdown")
	}

	logger.Info().Msg("Catalog Service stopped gracefully")
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
				pingErr := sqlDB.Ping()
				if pingErr != nil {
					err = pingErr
				} else {
					sqlDB.SetMaxOpenConns(25)
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
