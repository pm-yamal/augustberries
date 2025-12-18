package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Redis    RedisConfig
	Kafka    KafkaConfig
	JWT      JWTConfig
}

type ServerConfig struct {
	Host string // Адрес хоста (по умолчанию 0.0.0.0)
	Port string // Порт сервера (по умолчанию 8081)
}

type DatabaseConfig struct {
	Host     string // Хост PostgreSQL
	Port     string // Порт PostgreSQL
	User     string // Имя пользователя БД
	Password string // Пароль БД
	DBName   string // Имя базы данных
	SSLMode  string // Режим SSL (disable/require/verify-full)
}

type RedisConfig struct {
	Host     string // Хост Redis
	Port     string // Порт Redis
	Password string // Пароль Redis (опционально)
	DB       int    // Номер БД Redis (0-15)
}

type KafkaConfig struct {
	Brokers []string // Список брокеров Kafka (формат: host:port)
	Topic   string   // Топик для событий PRODUCT_CREATED, PRODUCT_UPDATED, PRODUCT_DELETED
}

type JWTConfig struct {
	Secret string // Секретный ключ для проверки JWT токенов (должен совпадать с Auth Service)
}

func Load() (*Config, error) {
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
			User:     getEnv("DB_USER", "postgres"),
			Password: getEnv("DB_PASSWORD", "postgres"),
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
			Brokers: []string{getEnv("KAFKA_BROKERS", "localhost:9092")},
			Topic:   getEnv("KAFKA_TOPIC", "product_events"),
		},
		JWT: JWTConfig{
			Secret: getEnv("JWT_SECRET", "your-secret-key-change-this-in-production"),
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

func (c *RedisConfig) Address() string {
	return c.Host + ":" + c.Port
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
