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

	"github.com/jackc/pgx/v5/pgxpool"

	"augustberries/catalog-service/internal/app/catalog/config"
	"augustberries/catalog-service/internal/app/catalog/handler"
	"augustberries/catalog-service/internal/app/catalog/repository"
	"augustberries/catalog-service/internal/app/catalog/service"
	"augustberries/catalog-service/internal/app/catalog/util"
)

func main() {
	// === ИНИЦИАЛИЗАЦИЯ КОНФИГУРАЦИИ ===
	// Загружаем конфигурацию из переменных окружения
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// === ПОДКЛЮЧЕНИЕ К POSTGRESQL ===
	// Используем connection pool для эффективного управления соединениями
	db, err := connectDB(context.Background(), cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()
	log.Println("Successfully connected to PostgreSQL")

	// === ПОДКЛЮЧЕНИЕ К REDIS ===
	// Redis используется для кеширования списка категорий
	redisClient, err := util.NewRedisClient(
		cfg.Redis.Address(),
		cfg.Redis.Password,
		cfg.Redis.DB,
	)
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer redisClient.Close()
	log.Println("Successfully connected to Redis")

	// === ИНИЦИАЛИЗАЦИЯ KAFKA PRODUCER ===
	// Kafka producer отправляет события PRODUCT_UPDATED в топик product_events
	// Background Worker подписан на этот топик для обработки событий
	kafkaProducer := util.NewKafkaProducer(cfg.Kafka.Brokers, cfg.Kafka.Topic)
	defer kafkaProducer.Close()
	log.Println("Successfully initialized Kafka producer")

	// === ИНИЦИАЛИЗАЦИЯ СЛОЯ РЕПОЗИТОРИЕВ ===
	// Репозитории отвечают за работу с PostgreSQL
	categoryRepo := repository.NewCategoryRepository(db)
	productRepo := repository.NewProductRepository(db)

	// === ИНИЦИАЛИЗАЦИЯ БИЗНЕС-ЛОГИКИ ===
	// Service layer координирует работу репозиториев, кеша и Kafka
	catalogService := service.NewCatalogService(
		categoryRepo,
		productRepo,
		redisClient,
		kafkaProducer,
	)

	// === ИНИЦИАЛИЗАЦИЯ HTTP HANDLERS ===
	// Handler обрабатывает HTTP запросы и вызывает методы service
	catalogHandler := handler.NewCatalogHandler(catalogService)

	// === НАСТРОЙКА МАРШРУТОВ ===
	// Настраиваем REST API endpoints согласно заданию
	router := handler.SetupRoutes(catalogHandler)

	// === НАСТРОЙКА HTTP СЕРВЕРА ===
	// Production-ready настройки с таймаутами
	server := &http.Server{
		Addr:         cfg.Server.Address(),
		Handler:      router,
		ReadTimeout:  15 * time.Second, // Таймаут чтения запроса
		WriteTimeout: 15 * time.Second, // Таймаут записи ответа
		IdleTimeout:  60 * time.Second, // Таймаут idle соединений
	}

	// === ЗАПУСК HTTP СЕРВЕРА ===
	// Запускаем сервер в отдельной горутине для graceful shutdown
	go func() {
		log.Printf("Starting Catalog Service on %s", cfg.Server.Address())
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// === GRACEFUL SHUTDOWN ===
	// Ожидаем сигнала завершения (SIGINT или SIGTERM)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down Catalog Service...")

	// Даем серверу 30 секунд на завершение текущих запросов
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Catalog Service stopped gracefully")
}

// connectDB устанавливает соединение с PostgreSQL используя pgx connection pool
// Использует retry logic с 10 попытками для устойчивости при запуске в Docker
func connectDB(ctx context.Context, cfg config.DatabaseConfig) (*pgxpool.Pool, error) {
	// Формируем connection string для pgx
	connString := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.DBName, cfg.SSLMode,
	)

	// Настройка пула соединений для production
	poolConfig, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("failed to parse pool config: %w", err)
	}

	// Оптимальные настройки пула для production
	poolConfig.MaxConns = 25                       // Максимум соединений в пуле
	poolConfig.MinConns = 5                        // Минимум соединений (держим открытыми)
	poolConfig.MaxConnLifetime = 5 * time.Minute   // Время жизни соединения
	poolConfig.MaxConnIdleTime = 1 * time.Minute   // Время простоя перед закрытием
	poolConfig.HealthCheckPeriod = 1 * time.Minute // Периодичность health checks

	// Пробуем подключиться с повторными попытками
	// При запуске в Docker когда PostgreSQL может быть еще не готов
	var pool *pgxpool.Pool
	for i := 0; i < 10; i++ {
		pool, err = pgxpool.NewWithConfig(ctx, poolConfig)
		if err == nil {
			// Проверяем соединение через ping
			if err = pool.Ping(ctx); err == nil {
				break
			}
			pool.Close()
		}
		log.Printf("Failed to connect to database (attempt %d/10): %v", i+1, err)
		time.Sleep(3 * time.Second)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to connect after 10 attempts: %w", err)
	}

	return pool, nil
}
