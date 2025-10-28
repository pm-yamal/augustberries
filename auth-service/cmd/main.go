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
	"github.com/redis/go-redis/v9"

	"augustberries/auth-service/internal/app/auth/config"
	"augustberries/auth-service/internal/app/auth/handler"
	"augustberries/auth-service/internal/app/auth/repository"
	"augustberries/auth-service/internal/app/auth/service"
	"augustberries/auth-service/internal/app/auth/util"
)

func main() {
	// Загружаем конфигурацию
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Подключаемся к базе данных PostgreSQL
	db, err := connectDB(context.Background(), cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	log.Println("Successfully connected to PostgreSQL database")

	// Подключаемся к Redis
	redisClient := connectRedis(cfg.Redis)
	defer redisClient.Close()

	// Проверяем соединение с Redis
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}

	log.Println("Successfully connected to Redis")

	// Инициализируем JWT менеджер
	jwtManager := util.NewJWTManager(
		cfg.JWT.SecretKey,
		cfg.JWT.AccessTokenDuration,
		cfg.JWT.RefreshTokenDuration,
	)

	// Инициализируем репозитории
	userRepo := repository.NewUserRepository(db)
	roleRepo := repository.NewRoleRepository(db)

	// Используем Redis для хранения токенов вместо PostgreSQL
	tokenRepo := repository.NewRedisTokenRepository(redisClient)

	// Инициализируем сервисы
	authService := service.NewAuthService(userRepo, roleRepo, tokenRepo, jwtManager)

	// Инициализируем обработчики
	authHandler := handler.NewAuthHandler(authService)
	authMiddleware := handler.NewAuthMiddleware(authService)

	// Настраиваем маршруты с chi router
	router := handler.SetupRoutes(authHandler, authMiddleware)

	// Создаем HTTP сервер
	server := &http.Server{
		Addr:         cfg.Server.Address(),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Запускаем сервер в отдельной горутине
	go func() {
		log.Printf("Starting server on %s", cfg.Server.Address())
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Ожидаем сигнала завершения (graceful shutdown)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Даем серверу 30 секунд на завершение текущих запросов
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server stopped gracefully")
}

// connectDB устанавливает соединение с PostgreSQL используя pgx connection pool
func connectDB(ctx context.Context, cfg config.DatabaseConfig) (*pgxpool.Pool, error) {
	// Формируем connection string для pgx
	connString := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.DBName, cfg.SSLMode,
	)

	// Настройка пула соединений
	poolConfig, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("failed to parse pool config: %w", err)
	}

	// Оптимальные настройки пула для production
	poolConfig.MaxConns = 25                     // Максимум соединений
	poolConfig.MinConns = 5                      // Минимум соединений
	poolConfig.MaxConnLifetime = 5 * time.Minute // Время жизни соединения
	poolConfig.MaxConnIdleTime = 1 * time.Minute // Время бездействия до закрытия
	poolConfig.HealthCheckPeriod = 1 * time.Minute

	// Пробуем подключиться с повторными попытками
	var pool *pgxpool.Pool
	for i := 0; i < 10; i++ {
		pool, err = pgxpool.NewWithConfig(ctx, poolConfig)
		if err == nil {
			// Проверяем соединение
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

// connectRedis создает и настраивает Redis клиент
func connectRedis(cfg config.RedisConfig) *redis.Client {
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

	return client
}
