package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config содержит все настройки приложения Catalog Service
// Включает конфигурацию для HTTP сервера, PostgreSQL, Redis, Kafka и JWT
type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Redis    RedisConfig
	Kafka    KafkaConfig
	JWT      JWTConfig
}

// ServerConfig - настройки HTTP сервера
type ServerConfig struct {
	Host string // Адрес хоста (по умолчанию 0.0.0.0)
	Port string // Порт сервера (по умолчанию 8081)
}

// DatabaseConfig - настройки подключения к PostgreSQL
// Используется для хранения категорий и товаров
type DatabaseConfig struct {
	Host     string // Хост PostgreSQL
	Port     string // Порт PostgreSQL
	User     string // Имя пользователя БД
	Password string // Пароль БД
	DBName   string // Имя базы данных
	SSLMode  string // Режим SSL (disable/require/verify-full)
}

// RedisConfig - настройки подключения к Redis для кеширования
// Используется для кеширования списка категорий
type RedisConfig struct {
	Host     string // Хост Redis
	Port     string // Порт Redis
	Password string // Пароль Redis (опционально)
	DB       int    // Номер БД Redis (0-15)
}

// KafkaConfig - настройки Kafka для отправки событий
// События отправляются при изменении товаров (создание/обновление/удаление)
type KafkaConfig struct {
	Brokers []string // Список брокеров Kafka (формат: host:port)
	Topic   string   // Топик для событий PRODUCT_CREATED, PRODUCT_UPDATED, PRODUCT_DELETED
}

// JWTConfig - настройки для проверки JWT токенов
// Используется для аутентификации запросов от других сервисов
type JWTConfig struct {
	Secret string // Секретный ключ для проверки JWT токенов (должен совпадать с Auth Service)
}

// Load загружает конфигурацию из переменных окружения
// Возвращает ошибку, если не удалось распарсить значения
func Load() (*Config, error) {
	// Парсим Redis DB как число
	redisDB, err := strconv.Atoi(getEnv("REDIS_DB", "0"))
	if err != nil {
		return nil, fmt.Errorf("invalid REDIS_DB value: %w", err)
	}

	return &Config{
		Server: ServerConfig{
			Host: getEnv("SERVER_HOST", "0.0.0.0"),
			Port: getEnv("SERVER_PORT", "8081"),
		},
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnv("DB_PORT", "5432"),
			User:     getEnv("DB_USER", "mongodb"),
			Password: getEnv("DB_PASSWORD", "mongodb"),
			DBName:   getEnv("DB_NAME", "catalog_service"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
		},
		Redis: RedisConfig{
			Host:     getEnv("REDIS_HOST", "localhost"),
			Port:     getEnv("REDIS_PORT", "6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       redisDB,
		},
		Kafka: KafkaConfig{
			// ИСПРАВЛЕНО: Топик должен быть product_events согласно заданию
			Brokers: []string{getEnv("KAFKA_BROKERS", "localhost:9092")},
			Topic:   getEnv("KAFKA_TOPIC", "product_events"),
		},
		JWT: JWTConfig{
			// JWT Secret должен совпадать с Auth Service для валидации токенов
			Secret: getEnv("JWT_SECRET", "your-secret-key-change-this-in-production"),
		},
	}, nil
}

// DSN возвращает строку подключения к PostgreSQL в формате libpq
func (c *DatabaseConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.DBName, c.SSLMode,
	)
}

// Address возвращает адрес сервера в формате host:port для HTTP сервера
func (c *ServerConfig) Address() string {
	return c.Host + ":" + c.Port
}

// Address возвращает адрес Redis в формате host:port для подключения
func (c *RedisConfig) Address() string {
	return c.Host + ":" + c.Port
}

// getEnv получает значение переменной окружения или возвращает значение по умолчанию
// Используется для гибкой конфигурации через environment variables
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
