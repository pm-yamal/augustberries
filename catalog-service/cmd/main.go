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

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

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
	// Используем GORM для работы с PostgreSQL
	db, err := connectDB(cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
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

	// === ИНИЦИАЛИЗАЦИЯ AUTH MIDDLEWARE ===
	// Middleware проверяет JWT токены для защиты API эндпоинтов
	// JWT Secret должен совпадать с Auth Service
	authMiddleware := handler.NewAuthMiddleware(cfg.JWT.Secret)
	log.Println("Initialized Auth middleware")

	// === ИНИЦИАЛИЗАЦИЯ HTTP HANDLERS ===
	// Handler обрабатывает HTTP запросы и вызывает методы service
	catalogHandler := handler.NewCatalogHandler(catalogService)

	// === НАСТРОЙКА МАРШРУТОВ ===
	// Настраиваем REST API endpoints согласно заданию с использованием Gin
	// Применяем Auth middleware для защиты эндпоинтов
	router := handler.SetupRoutes(catalogHandler, authMiddleware)

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

// connectDB устанавливает соединение с PostgreSQL используя GORM
// Использует retry logic с 10 попытками для устойчивости при запуске в Docker
func connectDB(cfg config.DatabaseConfig) (*gorm.DB, error) {
	// Формируем connection string для PostgreSQL
	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=%s",
		cfg.Host, cfg.User, cfg.Password, cfg.DBName, cfg.Port, cfg.SSLMode,
	)

	// Настройка GORM конфигурации
	gormConfig := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info), // Логирование SQL запросов
	}

	// Пробуем подключиться с повторными попытками
	// При запуске в Docker PostgreSQL может быть еще не готов
	var db *gorm.DB
	var err error

	for i := 0; i < 10; i++ {
		db, err = gorm.Open(postgres.Open(dsn), gormConfig)
		if err == nil {
			// Проверяем соединение через SQL DB
			sqlDB, sqlErr := db.DB()
			if sqlErr != nil {
				err = sqlErr
			} else {
				// Проверяем что соединение работает
				pingErr := sqlDB.Ping()
				if pingErr != nil {
					err = pingErr
				} else {
					// Успешное подключение - настраиваем connection pool
					sqlDB.SetMaxOpenConns(25)                 // Максимум открытых соединений
					sqlDB.SetMaxIdleConns(5)                  // Максимум idle соединений
					sqlDB.SetConnMaxLifetime(5 * time.Minute) // Время жизни соединения
					sqlDB.SetConnMaxIdleTime(1 * time.Minute) // Время простоя перед закрытием
					return db, nil
				}
			}
		}
		log.Printf("Failed to connect to database (attempt %d/10): %v", i+1, err)
		time.Sleep(3 * time.Second)
	}

	return nil, fmt.Errorf("failed to connect after 10 attempts: %w", err)
}
