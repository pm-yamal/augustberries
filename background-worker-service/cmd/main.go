package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"augustberries/background-worker-service/internal/app/background-worker/config"
	"augustberries/background-worker-service/internal/app/background-worker/handler"
	"augustberries/background-worker-service/internal/app/background-worker/processor"
	"augustberries/background-worker-service/internal/app/background-worker/repository"
	"augustberries/background-worker-service/internal/app/background-worker/service"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func main() {
	log.Println("Starting Background Worker Service...")

	// === ИНИЦИАЛИЗАЦИЯ КОНФИГУРАЦИИ ===
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Создаем основной контекст приложения
	ctx := context.Background()

	// === ПОДКЛЮЧЕНИЕ К POSTGRESQL ===
	// Используем БД Orders Service для обновления заказов
	db, err := connectDB(cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	log.Println("Successfully connected to PostgreSQL (orders_service)")

	// === ПОДКЛЮЧЕНИЕ К REDIS ===
	// Redis используется для хранения курсов валют
	redisClient, err := connectRedis(ctx, cfg.Redis)
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer redisClient.Close()
	log.Println("Successfully connected to Redis")

	// === ИНИЦИАЛИЗАЦИЯ РЕПОЗИТОРИЕВ ===
	orderRepo := repository.NewOrderRepository(db)
	exchangeRateRepo := repository.NewExchangeRateRepository(redisClient, cfg.Redis.TTL)
	log.Println("Repositories initialized")

	// === ИНИЦИАЛИЗАЦИЯ API КЛИЕНТА ===
	// API клиент для получения курсов валют из внешнего API
	exchangeAPIClient := service.NewExchangeRateAPIClient(
		cfg.ExchangeAPI.URL,
		cfg.ExchangeAPI.Timeout,
	)
	log.Println("Exchange Rate API Client initialized")

	// === ИНИЦИАЛИЗАЦИЯ СЕРВИСОВ ===
	// Exchange Rate Service использует API клиент для получения данных
	exchangeRateSvc := service.NewExchangeRateService(
		exchangeRateRepo,
		exchangeAPIClient,
	)

	orderProcessingSvc := service.NewOrderProcessingService(
		orderRepo,
		exchangeRateSvc,
	)
	log.Println("Services initialized")

	// === ИНИЦИАЛИЗАЦИЯ KAFKA CONSUMER ===
	kafkaConsumer := processor.NewKafkaConsumer(
		cfg.Kafka.Brokers,
		cfg.Kafka.Topic,
		cfg.Kafka.GroupID,
		cfg.Kafka.MinBytes,
		cfg.Kafka.MaxBytes,
		orderProcessingSvc,
		exchangeRateSvc,
	)

	// Запускаем Kafka consumer
	kafkaConsumer.Start(ctx)
	defer kafkaConsumer.Stop()
	log.Printf("Kafka consumer started (topic: %s, group: %s)", cfg.Kafka.Topic, cfg.Kafka.GroupID)

	// === ИНИЦИАЛИЗАЦИЯ CRON SCHEDULER ===
	cronScheduler := processor.NewCronScheduler(exchangeRateSvc)

	// Запускаем cron для периодического обновления курсов валют
	if err := cronScheduler.Start(ctx, cfg.CronSchedule.UpdateRates); err != nil {
		log.Fatalf("Failed to start cron scheduler: %v", err)
	}
	defer cronScheduler.Stop()
	log.Printf("Cron scheduler started (schedule: %s)", cfg.CronSchedule.UpdateRates)

	// === ИНИЦИАЛИЗАЦИЯ HEALTHCHECK HTTP СЕРВЕРА ===
	healthHandler := handler.NewHealthCheckHandler(db, redisClient, exchangeRateSvc)

	mux := http.NewServeMux()
	healthHandler.RegisterRoutes(mux)

	// Prometheus metrics endpoint
	mux.Handle("/metrics", promhttp.Handler())

	httpServer := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	// Запускаем HTTP сервер в отдельной горутине
	go func() {
		log.Println("Starting healthcheck HTTP server on :8080...")
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP server error: %v", err)
		}
	}()
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		httpServer.Shutdown(shutdownCtx)
	}()
	log.Println("Healthcheck and metrics endpoints available:")
	log.Println("  - GET http://localhost:8080/health")
	log.Println("  - GET http://localhost:8080/health/readiness")
	log.Println("  - GET http://localhost:8080/health/liveness")
	log.Println("  - GET http://localhost:8080/metrics")

	// === ЗАПУСК ЗАВЕРШЕН ===
	log.Println("Background Worker Service is running")
	log.Println("Waiting for ORDER_CREATED events from Kafka...")
	log.Printf("Exchange rates will be updated according to schedule: %s", cfg.CronSchedule.UpdateRates)

	// === GRACEFUL SHUTDOWN ===
	// Ожидаем сигнала завершения (SIGINT или SIGTERM)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down Background Worker Service...")

	// Даем время на завершение обработки текущих сообщений
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Ждем завершения обработки
	<-shutdownCtx.Done()

	log.Println("Background Worker Service stopped gracefully")
}

// connectDB устанавливает соединение с PostgreSQL используя GORM
func connectDB(cfg config.DatabaseConfig) (*gorm.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=%s",
		cfg.Host, cfg.User, cfg.Password, cfg.DBName, cfg.Port, cfg.SSLMode,
	)

	gormConfig := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	}

	// Retry logic для устойчивости при запуске в Docker
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
					// Настраиваем connection pool
					sqlDB.SetMaxOpenConns(10)
					sqlDB.SetMaxIdleConns(5)
					sqlDB.SetConnMaxLifetime(5 * time.Minute)
					sqlDB.SetConnMaxIdleTime(1 * time.Minute)
					return db, nil
				}
			}
		}
		log.Printf("Failed to connect to database (attempt %d/10): %v", i+1, err)
		time.Sleep(3 * time.Second)
	}

	return nil, fmt.Errorf("failed to connect after 10 attempts: %w", err)
}

// connectRedis устанавливает соединение с Redis
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

	// Проверяем соединение с retry logic
	for i := 0; i < 10; i++ {
		if err := client.Ping(ctx).Err(); err == nil {
			return client, nil
		}
		log.Printf("Failed to connect to Redis (attempt %d/10)", i+1)
		time.Sleep(3 * time.Second)
	}

	return nil, fmt.Errorf("failed to connect to Redis after 10 attempts")
}
