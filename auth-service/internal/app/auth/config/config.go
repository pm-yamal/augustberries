package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config содержит все настройки приложения
type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Redis    RedisConfig
	JWT      JWTConfig
}

// ServerConfig - настройки HTTP сервера
type ServerConfig struct {
	Host string
	Port string
}

// DatabaseConfig - настройки подключения к PostgreSQL
type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
}

// RedisConfig - настройки подключения к Redis
type RedisConfig struct {
	Host     string
	Port     string
	Password string
	DB       int
}

// JWTConfig - настройки для JWT токенов
type JWTConfig struct {
	SecretKey            string
	AccessTokenDuration  time.Duration
	RefreshTokenDuration time.Duration
}

// Load загружает конфигурацию из переменных окружения
func Load() (*Config, error) {
	// JWT настройки
	accessDuration, err := time.ParseDuration(getEnv("JWT_ACCESS_DURATION", "15m"))
	if err != nil {
		return nil, fmt.Errorf("invalid JWT_ACCESS_DURATION: %w", err)
	}

	refreshDuration, err := time.ParseDuration(getEnv("JWT_REFRESH_DURATION", "168h")) // 7 дней
	if err != nil {
		return nil, fmt.Errorf("invalid JWT_REFRESH_DURATION: %w", err)
	}

	return &Config{
		Server: ServerConfig{
			Host: getEnv("SERVER_HOST", "0.0.0.0"),
			Port: getEnv("SERVER_PORT", "8080"),
		},
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnv("DB_PORT", "5432"),
			User:     getEnv("DB_USER", "mongodb"),
			Password: getEnv("DB_PASSWORD", "mongodb"),
			DBName:   getEnv("DB_NAME", "auth_service"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
		},
		Redis: RedisConfig{
			Host:     getEnv("REDIS_HOST", "localhost"),
			Port:     getEnv("REDIS_PORT", "6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getEnvInt("REDIS_DB", 0),
		},
		JWT: JWTConfig{
			SecretKey:            getEnv("JWT_SECRET", "your-secret-key-change-in-production"),
			AccessTokenDuration:  accessDuration,
			RefreshTokenDuration: refreshDuration,
		},
	}, nil
}

// DSN возвращает строку подключения к PostgreSQL
func (c *DatabaseConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.DBName, c.SSLMode,
	)
}

// Address возвращает адрес Redis в формате host:port
func (c *RedisConfig) Address() string {
	return c.Host + ":" + c.Port
}

// Address возвращает адрес сервера в формате host:port
func (c *ServerConfig) Address() string {
	return c.Host + ":" + c.Port
}

// getEnv получает значение переменной окружения или возвращает значение по умолчанию
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvInt получает значение переменной окружения как int
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}
