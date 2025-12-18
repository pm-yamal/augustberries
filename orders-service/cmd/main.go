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

	"augustberries/orders-service/internal/app/orders/config"
	"augustberries/orders-service/internal/app/orders/handler"
	http2 "augustberries/orders-service/internal/app/orders/infrastructure/http"
	"augustberries/orders-service/internal/app/orders/infrastructure/messaging"
	"augustberries/orders-service/internal/app/orders/repository"
	"augustberries/orders-service/internal/app/orders/service"
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
	logger.Init("orders-service", logLevel)

	logstashAddr := os.Getenv("LOGSTASH_ADDR")
	if logstashAddr != "" {
		if err := logger.InitLogstash(logstashAddr, "orders-service", logLevel); err != nil {
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

	kafkaProducer := messaging.NewKafkaProducer(cfg.Kafka.Brokers, cfg.Kafka.Topic)
	defer kafkaProducer.Close()
	logger.Info().
		Str("topic", cfg.Kafka.Topic).
		Msg("Initialized Kafka producer")

	catalogClient := http2.NewCatalogClient(cfg.CatalogService.URL)
	logger.Info().
		Str("url", cfg.CatalogService.URL).
		Msg("Initialized Catalog Service client")

	orderRepo := repository.NewOrderRepository(db)
	orderItemRepo := repository.NewOrderItemRepository(db)

	orderService := service.NewOrderService(
		orderRepo,
		orderItemRepo,
		catalogClient,
		kafkaProducer,
	)

	authMiddleware := handler.NewAuthMiddleware(cfg.JWT.Secret)
	orderHandler := handler.NewOrderHandler(orderService)
	router := handler.SetupRoutes(orderHandler, authMiddleware)

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
			Msg("Starting Orders Service")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal().Err(err).Msg("Failed to start server")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info().Msg("Shutting down Orders Service...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Fatal().Err(err).Msg("Server forced to shutdown")
	}

	logger.Info().Msg("Orders Service stopped gracefully")
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
