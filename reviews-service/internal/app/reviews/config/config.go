package config

import (
	"os"
)

// Config содержит все настройки приложения Reviews Service
// Включает конфигурацию для HTTP сервера, MongoDB, Kafka и JWT
type Config struct {
	Server  ServerConfig
	MongoDB MongoDBConfig
	Kafka   KafkaConfig
	JWT     JWTConfig
}

// ServerConfig - настройки HTTP сервера
type ServerConfig struct {
	Host string // Адрес хоста (по умолчанию 0.0.0.0)
	Port string // Порт сервера (по умолчанию 8083)
}

// MongoDBConfig - настройки подключения к MongoDB
// Используется для хранения отзывов с индексом по product_id
type MongoDBConfig struct {
	URI      string // URI подключения к MongoDB
	Database string // Имя базы данных
}

// KafkaConfig - настройки Kafka для отправки событий
// События отправляются при создании отзывов
type KafkaConfig struct {
	Brokers []string // Список брокеров Kafka (формат: host:port)
	Topic   string   // Топик для событий REVIEW_CREATED
}

// JWTConfig - настройки для проверки JWT токенов
// Используется для аутентификации запросов от пользователей
type JWTConfig struct {
	Secret string // Секретный ключ для проверки JWT токенов (должен совпадать с Auth Service)
}

// Load загружает конфигурацию из переменных окружения
func Load() (*Config, error) {
	return &Config{
		Server: ServerConfig{
			Host: getEnv("SERVER_HOST", "0.0.0.0"),
			Port: getEnv("SERVER_PORT", "8083"),
		},
		MongoDB: MongoDBConfig{
			URI:      getEnv("MONGODB_URI", "mongodb://localhost:27017"),
			Database: getEnv("MONGODB_DATABASE", "reviews_service"),
		},
		Kafka: KafkaConfig{
			Brokers: []string{getEnv("KAFKA_BROKERS", "localhost:9092")},
			Topic:   getEnv("KAFKA_TOPIC", "review_events"),
		},
		JWT: JWTConfig{
			// JWT Secret должен совпадать с Auth Service для валидации токенов
			Secret: getEnv("JWT_SECRET", "your-secret-key-change-this-in-production"),
		},
	}, nil
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
