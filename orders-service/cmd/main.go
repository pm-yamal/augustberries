package main

import (
	http2 "augustberries/orders-service/internal/app/orders/infrastructure/http"
	"augustberries/orders-service/internal/app/orders/infrastructure/messaging"
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

	"augustberries/orders-service/internal/app/orders/config"
	"augustberries/orders-service/internal/app/orders/handler"
	"augustberries/orders-service/internal/app/orders/repository"
	"augustberries/orders-service/internal/app/orders/service"
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

	// === ИНИЦИАЛИЗАЦИЯ KAFKA PRODUCER ===
	// Kafka producer отправляет события ORDER_CREATED, ORDER_UPDATED в топик order_events
	kafkaProducer := messaging.NewKafkaProducer(cfg.Kafka.Brokers, cfg.Kafka.Topic)
	defer kafkaProducer.Close()
	log.Println("Successfully initialized Kafka producer")

	// === ИНИЦИАЛИЗАЦИЯ CATALOG CLIENT ===
	// HTTP клиент для взаимодействия с Catalog Service
	catalogClient := http2.NewCatalogClient(cfg.CatalogService.URL)
	log.Println("Initialized Catalog Service client")

	// === ИНИЦИАЛИЗАЦИЯ СЛОЯ РЕПОЗИТОРИЕВ ===
	// Репозитории отвечают за работу с PostgreSQL
	orderRepo := repository.NewOrderRepository(db)
	orderItemRepo := repository.NewOrderItemRepository(db)

	// === ИНИЦИАЛИЗАЦИЯ БИЗНЕС-ЛОГИКИ ===
	// Service layer координирует работу репозиториев, Catalog Service и Kafka
	orderService := service.NewOrderService(
		orderRepo,
		orderItemRepo,
		catalogClient,
		kafkaProducer,
	)

	// === ИНИЦИАЛИЗАЦИЯ AUTH MIDDLEWARE ===
	// Middleware проверяет JWT токены для защиты API эндпоинтов
	// JWT Secret должен совпадать с Auth Service
	authMiddleware := handler.NewAuthMiddleware(cfg.JWT.Secret)
	log.Println("Initialized Auth middleware")

	// === ИНИЦИАЛИЗАЦИЯ HTTP HANDLERS ===
	// Handler обрабатывает HTTP запросы и вызывает методы service
	orderHandler := handler.NewOrderHandler(orderService)

	// === НАСТРОЙКА МАРШРУТОВ ===
	// Настраиваем REST API endpoints согласно заданию с использованием Gin
	// Применяем Auth middleware для защиты эндпоинтов
	router := handler.SetupRoutes(orderHandler, authMiddleware)

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
		log.Printf("Starting Orders Service on %s", cfg.Server.Address())
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// === GRACEFUL SHUTDOWN ===
	// Ожидаем сигнала завершения (SIGINT или SIGTERM)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down Orders Service...")

	// Даем серверу 30 секунд на завершение текущих запросов
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Orders Service stopped gracefully")
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
