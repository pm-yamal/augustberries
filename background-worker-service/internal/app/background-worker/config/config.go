package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Database     DatabaseConfig
	Redis        RedisConfig
	Kafka        KafkaConfig
	ExchangeAPI  ExchangeAPIConfig
	CronSchedule CronScheduleConfig
}

type DatabaseConfig struct {
	Host     string // Хост PostgreSQL
	Port     string // Порт PostgreSQL
	User     string // Имя пользователя БД
	Password string // Пароль БД
	DBName   string // Имя базы данных (orders_service)
	SSLMode  string // Режим SSL (disable/require/verify-full)
}

type RedisConfig struct {
	Host     string        // Хост Redis
	Port     string        // Порт Redis
	Password string        // Пароль Redis
	DB       int           // Номер БД Redis (обычно 0)
	TTL      time.Duration // TTL для курсов валют (30-60 минут)
}

type KafkaConfig struct {
	Brokers  []string // Список брокеров Kafka (формат: host:port)
	Topic    string   // Топик для прослушивания (order_events)
	GroupID  string   // ID группы потребителей для распределения нагрузки
	MinBytes int      // Минимум байт для fetch запроса
	MaxBytes int      // Максимум байт для fetch запроса
}

type ExchangeAPIConfig struct {
	URL     string // URL API для получения курсов (например, exchangerate-api.com)
	APIKey  string // API ключ для аутентификации (если требуется)
	Timeout int    // Таймаут запроса в секундах
}

type CronScheduleConfig struct {
	UpdateRates string // Расписание обновления курсов валют (например, "0 */30 * * * *" каждые 30 минут)
}

func Load() (*Config, error) {
	ttlMinutes := getEnvInt("REDIS_RATES_TTL_MINUTES", 30)

	return &Config{
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnv("DB_PORT", "5433"), // Порт PostgreSQL для Orders Service
			User:     getEnv("DB_USER", "postgres"),
			Password: getEnv("DB_PASSWORD", "postgres"),
			DBName:   getEnv("DB_NAME", "orders_service"), // БД заказов
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
		},
		Redis: RedisConfig{
			Host:     getEnv("REDIS_HOST", "localhost"),
			Port:     getEnv("REDIS_PORT", "6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getEnvInt("REDIS_DB", 2), // Отдельная БД для курсов валют
			TTL:      time.Duration(ttlMinutes) * time.Minute,
		},
		Kafka: KafkaConfig{
			Brokers:  []string{getEnv("KAFKA_BROKERS", "localhost:9092")},
			Topic:    getEnv("KAFKA_TOPIC", "order_events"),
			GroupID:  getEnv("KAFKA_GROUP_ID", "background-worker-group"),
			MinBytes: getEnvInt("KAFKA_MIN_BYTES", 1),    // 1 byte minimum
			MaxBytes: getEnvInt("KAFKA_MAX_BYTES", 10e6), // 10MB maximum
		},
		ExchangeAPI: ExchangeAPIConfig{
			URL:     getEnv("EXCHANGE_API_URL", "https://api.exchangerate-api.com/v4/latest/USD"),
			APIKey:  getEnv("EXCHANGE_API_KEY", ""), // Для бесплатной версии ключ не нужен
			Timeout: getEnvInt("EXCHANGE_API_TIMEOUT", 10),
		},
		CronSchedule: CronScheduleConfig{
			UpdateRates: getEnv("CRON_UPDATE_RATES", "0 */30 * * * *"),
		},
	}, nil
}

func (c *DatabaseConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.DBName, c.SSLMode,
	)
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

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}
