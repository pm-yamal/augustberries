package main

import (
	"context"
	"fmt"
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
	logger.Init("auth-service", logLevel)

	logstashAddr := os.Getenv("LOGSTASH_ADDR")
	if logstashAddr != "" {
		if err := logger.InitLogstash(logstashAddr, "auth-service", logLevel); err != nil {
			logger.Warn().Err(err).Msg("Failed to connect to Logstash, using stdout only")
		} else {
			logger.Info().Str("logstash_addr", logstashAddr).Msg("Connected to Logstash")
		}
	}

	db, err := connectDB(context.Background(), cfg.Database)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to connect to database")
	}
	defer db.Close()

	logger.Info().
		Str("host", cfg.Database.Host).
		Str("database", cfg.Database.DBName).
		Msg("Connected to PostgreSQL")

	redisClient := connectRedis(cfg.Redis)
	defer redisClient.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := redisClient.Ping(ctx).Err(); err != nil {
		logger.Fatal().Err(err).Msg("Failed to connect to Redis")
	}

	logger.Info().
		Str("host", cfg.Redis.Host).
		Int("db", cfg.Redis.DB).
		Msg("Connected to Redis")

	jwtManager := util.NewJWTManager(
		cfg.JWT.SecretKey,
		cfg.JWT.AccessTokenDuration,
		cfg.JWT.RefreshTokenDuration,
	)

	userRepo := repository.NewUserRepository(db)
	roleRepo := repository.NewRoleRepository(db)
	tokenRepo := repository.NewRedisTokenRepository(redisClient)

	authService := service.NewAuthService(userRepo, roleRepo, tokenRepo, jwtManager)
	roleService := service.NewRoleService(roleRepo)
	permissionService := service.NewPermissionService(roleRepo)

	authHandler := handler.NewAuthHandler(authService)
	roleHandler := handler.NewRoleHandler(roleService, permissionService)
	authMiddleware := handler.NewAuthMiddleware(authService)

	router := handler.SetupRoutes(authHandler, roleHandler, authMiddleware)

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
			Msg("Starting Auth Service")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal().Err(err).Msg("Failed to start server")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info().Msg("Shutting down server...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Fatal().Err(err).Msg("Server forced to shutdown")
	}

	logger.Info().Msg("Server stopped gracefully")
}

func connectDB(ctx context.Context, cfg config.DatabaseConfig) (*pgxpool.Pool, error) {
	connString := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.DBName, cfg.SSLMode,
	)

	poolConfig, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("failed to parse pool config: %w", err)
	}

	poolConfig.MaxConns = 25
	poolConfig.MinConns = 5
	poolConfig.MaxConnLifetime = 5 * time.Minute
	poolConfig.MaxConnIdleTime = 1 * time.Minute
	poolConfig.HealthCheckPeriod = 1 * time.Minute

	var pool *pgxpool.Pool
	for i := 0; i < 10; i++ {
		pool, err = pgxpool.NewWithConfig(ctx, poolConfig)
		if err == nil {
			if err = pool.Ping(ctx); err == nil {
				break
			}
			pool.Close()
		}
		logger.Warn().
			Int("attempt", i+1).
			Err(err).
			Msg("Failed to connect to database, retrying...")
		time.Sleep(3 * time.Second)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to connect after 10 attempts: %w", err)
	}

	return pool, nil
}

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
