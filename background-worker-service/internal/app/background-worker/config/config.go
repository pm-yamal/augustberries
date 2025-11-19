package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config содержит все настройки приложения Background Worker Service
// Включает конфигурацию для PostgreSQL, Redis, Kafka и внешнего API валют
type Config struct {
	Database     DatabaseConfig
	Redis        RedisConfig
	Kafka        KafkaConfig
	ExchangeAPI  ExchangeAPIConfig
	CronSchedule CronScheduleConfig
}

// DatabaseConfig - настройки подключения к PostgreSQL Orders Service
// Используется для обновления заказов после расчета доставки
type DatabaseConfig struct {
	Host     string // Хост PostgreSQL
	Port     string // Порт PostgreSQL
	User     string // Имя пользователя БД
	Password string // Пароль БД
	DBName   string // Имя базы данных (orders_service)
	SSLMode  string // Режим SSL (disable/require/verify-full)
}

// RedisConfig - настройки подключения к Redis
// Используется для хранения курсов валют с TTL
type RedisConfig struct {
	Host     string        // Хост Redis
	Port     string        // Порт Redis
	Password string        // Пароль Redis
	DB       int           // Номер БД Redis (обычно 0)
	TTL      time.Duration // TTL для курсов валют (30-60 минут)
}

// KafkaConfig - настройки Kafka для подписки на события
// Слушает топик order_events для обработки ORDER_CREATED
type KafkaConfig struct {
	Brokers  []string // Список брокеров Kafka (формат: host:port)
	Topic    string   // Топик для прослушивания (order_events)
	GroupID  string   // ID группы потребителей для распределения нагрузки
	MinBytes int      // Минимум байт для fetch запроса
	MaxBytes int      // Максимум байт для fetch запроса
}

// ExchangeAPIConfig - настройки для внешнего API валют
// Используется для получения актуальных курсов валют
type ExchangeAPIConfig struct {
	URL     string // URL API для получения курсов (например, exchangerate-api.com)
	APIKey  string // API ключ для аутентификации (если требуется)
	Timeout int    // Таймаут запроса в секундах
}

// CronScheduleConfig - настройки расписания cron задач
type CronScheduleConfig struct {
	UpdateRates string // Расписание обновления курсов валют (например, "0 */30 * * * *" каждые 30 минут)
}

// Load загружает конфигурацию из переменных окружения
// Возвращает ошибку, если не удалось распарсить значения
func Load() (*Config, error) {
	// TTL для курсов валют (по умолчанию 30 минут)
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
			// Используем бесплатный API exchangerate-api.com
			URL:     getEnv("EXCHANGE_API_URL", "https://api.exchangerate-api.com/v4/latest/USD"),
			APIKey:  getEnv("EXCHANGE_API_KEY", ""), // Для бесплатной версии ключ не нужен
			Timeout: getEnvInt("EXCHANGE_API_TIMEOUT", 10),
		},
		CronSchedule: CronScheduleConfig{
			// По умолчанию обновляем курсы каждые 30 минут
			UpdateRates: getEnv("CRON_UPDATE_RATES", "0 */30 * * * *"),
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

// Address возвращает адрес Redis в формате host:port
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

// getEnvInt получает значение переменной окружения как int
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}
