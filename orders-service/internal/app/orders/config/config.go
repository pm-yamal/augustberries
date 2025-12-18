package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	Server         ServerConfig
	Database       DatabaseConfig
	Kafka          KafkaConfig
	JWT            JWTConfig
	CatalogService CatalogServiceConfig
}

type ServerConfig struct {
	Host string // Адрес хоста (по умолчанию 0.0.0.0)
	Port string // Порт сервера (по умолчанию 8082)
}

type DatabaseConfig struct {
	Host     string // Хост PostgreSQL
	Port     string // Порт PostgreSQL
	User     string // Имя пользователя БД
	Password string // Пароль БД
	DBName   string // Имя базы данных
	SSLMode  string // Режим SSL (disable/require/verify-full)
}

type KafkaConfig struct {
	Brokers []string // Список брокеров Kafka (формат: host:port)
	Topic   string   // Топик для событий ORDER_CREATED, ORDER_UPDATED
}

type JWTConfig struct {
	Secret string // Секретный ключ для проверки JWT токенов (должен совпадать с Auth Service)
}

type CatalogServiceConfig struct {
	URL string // URL Catalog Service для получения информации о товарах
}

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
			Secret: getEnv("JWT_SECRET", "your-secret-key-change-this-in-production"),
		},
		CatalogService: CatalogServiceConfig{
			URL: getEnv("CATALOG_SERVICE_URL", "http://localhost:8081"),
		},
	}, nil
}

func (c *DatabaseConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.DBName, c.SSLMode,
	)
}

func (c *ServerConfig) Address() string {
	return c.Host + ":" + c.Port
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}
