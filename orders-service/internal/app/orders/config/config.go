package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config содержит все настройки приложения Orders Service
// Включает конфигурацию для HTTP сервера, PostgreSQL, Kafka и JWT
type Config struct {
	Server         ServerConfig
	Database       DatabaseConfig
	Kafka          KafkaConfig
	JWT            JWTConfig
	CatalogService CatalogServiceConfig
}

// ServerConfig - настройки HTTP сервера
type ServerConfig struct {
	Host string // Адрес хоста (по умолчанию 0.0.0.0)
	Port string // Порт сервера (по умолчанию 8082)
}

// DatabaseConfig - настройки подключения к PostgreSQL
// Используется для хранения заказов и позиций заказов
type DatabaseConfig struct {
	Host     string // Хост PostgreSQL
	Port     string // Порт PostgreSQL
	User     string // Имя пользователя БД
	Password string // Пароль БД
	DBName   string // Имя базы данных
	SSLMode  string // Режим SSL (disable/require/verify-full)
}

// KafkaConfig - настройки Kafka для отправки событий
// События отправляются при создании/обновлении заказов
type KafkaConfig struct {
	Brokers []string // Список брокеров Kafka (формат: host:port)
	Topic   string   // Топик для событий ORDER_CREATED, ORDER_UPDATED
}

// JWTConfig - настройки для проверки JWT токенов
// Используется для аутентификации запросов от пользователей
type JWTConfig struct {
	Secret string // Секретный ключ для проверки JWT токенов (должен совпадать с Auth Service)
}

// CatalogServiceConfig - настройки для обращения к Catalog Service
// Используется для проверки цен товаров
type CatalogServiceConfig struct {
	URL string // URL Catalog Service для получения информации о товарах
}

// Load загружает конфигурацию из переменных окружения
// Возвращает ошибку, если не удалось распарсить значения
func Load() (*Config, error) {
	return &Config{
		Server: ServerConfig{
			Host: getEnv("SERVER_HOST", "0.0.0.0"),
			Port: getEnv("SERVER_PORT", "8082"),
		},
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnv("DB_PORT", "5432"),
			User:     getEnv("DB_USER", "postgres"),
			Password: getEnv("DB_PASSWORD", "postgres"),
			DBName:   getEnv("DB_NAME", "orders_service"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
		},
		Kafka: KafkaConfig{
			Brokers: []string{getEnv("KAFKA_BROKERS", "localhost:9092")},
			Topic:   getEnv("KAFKA_TOPIC", "order_events"),
		},
		JWT: JWTConfig{
			// JWT Secret должен совпадать с Auth Service для валидации токенов
			Secret: getEnv("JWT_SECRET", "your-secret-key-change-this-in-production"),
		},
		CatalogService: CatalogServiceConfig{
			URL: getEnv("CATALOG_SERVICE_URL", "http://localhost:8081"),
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

// getEnv получает значение переменной окружения или возвращает значение по умолчанию
// Используется для гибкой конфигурации через environment variables
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
