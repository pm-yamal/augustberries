package main

import (
	"augustberries/reviews-service/internal/app/reviews/config"
	"augustberries/reviews-service/internal/app/reviews/handler"
	"augustberries/reviews-service/internal/app/reviews/infrastructure/messaging"
	"augustberries/reviews-service/internal/app/reviews/repository"
	"augustberries/reviews-service/internal/app/reviews/service"
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {
	// === ИНИЦИАЛИЗАЦИЯ КОНФИГУРАЦИИ ===
	// Загружаем конфигурацию из переменных окружения
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// === ПОДКЛЮЧЕНИЕ К MONGODB ===
	// Используем официальный MongoDB driver для Go
	mongoClient, err := connectMongoDB(cfg.MongoDB)
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := mongoClient.Disconnect(ctx); err != nil {
			log.Printf("Error disconnecting from MongoDB: %v", err)
		}
	}()
	log.Println("Successfully connected to MongoDB")

	// Получаем базу данных
	db := mongoClient.Database(cfg.MongoDB.Database)

	// === ИНИЦИАЛИЗАЦИЯ KAFKA PRODUCER ===
	// Kafka producer отправляет события REVIEW_CREATED в топик review_events
	kafkaProducer := messaging.NewKafkaProducer(cfg.Kafka.Brokers, cfg.Kafka.Topic)
	defer kafkaProducer.Close()
	log.Println("Successfully initialized Kafka producer")

	// === ИНИЦИАЛИЗАЦИЯ СЛОЯ РЕПОЗИТОРИЕВ ===
	// Репозиторий отвечает за работу с MongoDB (с индексом по product_id)
	reviewRepo := repository.NewReviewRepository(db)

	// === ИНИЦИАЛИЗАЦИЯ БИЗНЕС-ЛОГИКИ ===
	// Service layer координирует работу репозитория и Kafka
	reviewService := service.NewReviewService(reviewRepo, kafkaProducer)

	// === ИНИЦИАЛИЗАЦИЯ AUTH MIDDLEWARE ===
	// Middleware проверяет JWT токены для защиты API эндпоинтов
	// JWT Secret должен совпадать с Auth Service
	authMiddleware := handler.NewAuthMiddleware(cfg.JWT.Secret)
	log.Println("Initialized Auth middleware")

	// === ИНИЦИАЛИЗАЦИЯ HTTP HANDLERS ===
	// Handler обрабатывает HTTP запросы и вызывает методы service
	reviewHandler := handler.NewReviewHandler(reviewService)

	// === НАСТРОЙКА МАРШРУТОВ ===
	// Настраиваем REST API endpoints согласно заданию с использованием Gin
	// Применяем Auth middleware для защиты эндпоинтов
	router := handler.SetupRoutes(reviewHandler, authMiddleware)

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
		log.Printf("Starting Reviews Service on %s", cfg.Server.Address())
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// === GRACEFUL SHUTDOWN ===
	// Ожидаем сигнала завершения (SIGINT или SIGTERM)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down Reviews Service...")

	// Даем серверу 30 секунд на завершение текущих запросов
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Reviews Service stopped gracefully")
}

// connectMongoDB устанавливает соединение с MongoDB
// Использует retry logic с 10 попытками для устойчивости при запуске в Docker
func connectMongoDB(cfg config.MongoDBConfig) (*mongo.Client, error) {
	// Настройка MongoDB клиента
	clientOptions := options.Client().ApplyURI(cfg.URI)

	// Пробуем подключиться с повторными попытками
	// При запуске в Docker MongoDB может быть еще не готов
	var client *mongo.Client
	var err error

	for i := 0; i < 10; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		client, err = mongo.Connect(ctx, clientOptions)
		if err == nil {
			// Проверяем соединение
			pingCtx, pingCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer pingCancel()

			if err = client.Ping(pingCtx, nil); err == nil {
				return client, nil
			}
		}

		log.Printf("Failed to connect to MongoDB (attempt %d/10): %v", i+1, err)
		time.Sleep(3 * time.Second)
	}

	return nil, err
}
