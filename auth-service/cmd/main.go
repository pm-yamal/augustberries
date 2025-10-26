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

	// Подключаемся к базе данных
	db, err := connectDB(context.Background(), cfg.Database)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	log.Println("Successfully connected to database")

	// Инициализируем JWT менеджер
	jwtManager := util.NewJWTManager(
		cfg.JWT.SecretKey,
		cfg.JWT.AccessTokenDuration,
		cfg.JWT.RefreshTokenDuration,
	)

	// Инициализируем репозитории
	userRepo := repository.NewUserRepository(db)
	roleRepo := repository.NewRoleRepository(db)
	tokenRepo := repository.NewTokenRepository(db)

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

	// Запускаем фоновую задачу очистки истекших токенов
	go cleanupExpiredTokens(tokenRepo)

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
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
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

// cleanupExpiredTokens периодически удаляет истекшие токены из БД
func cleanupExpiredTokens(tokenRepo repository.TokenRepository) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)

		if err := tokenRepo.CleanupExpiredTokens(ctx); err != nil {
			log.Printf("Failed to cleanup expired tokens: %v", err)
		} else {
			log.Println("Successfully cleaned up expired tokens")
		}

		cancel()
	}
}
