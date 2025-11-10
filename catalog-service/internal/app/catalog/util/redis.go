package util

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"augustberries/catalog-service/internal/app/catalog/entity"

	"github.com/redis/go-redis/v9"
)

const (
	// Используется для кеширования списка категорий на 10-30 минут
	categoriesCacheKey = "categories:all"
)

// RedisClient обертка над Redis клиентом для работы с кешем
// Используется для кеширования категорий и снижения нагрузки на PostgreSQL
// Реализует интерфейс RedisCache для dependency injection
type RedisClient struct {
	client *redis.Client
}

// NewRedisClient создает новый Redis клиент и проверяет соединение
// Возвращает ошибку, если не удается подключиться к Redis
func NewRedisClient(addr, password string, db int) (*RedisClient, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,     // Адрес Redis в формате host:port
		Password: password, // Пароль для аутентификации (пустая строка если не требуется)
		DB:       db,       // Номер БД (0-15)
	})

	// Проверяем соединение с таймаутом 5 секунд
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	return &RedisClient{client: client}, nil
}

// SetCategories сохраняет список категорий в кеш Redis
// TTL определяет время жизни кеша (обычно 10-30 минут)
// Использует JSON сериализацию для хранения
func (r *RedisClient) SetCategories(ctx context.Context, categories []entity.Category, ttl time.Duration) error {
	// Сериализуем массив категорий в JSON
	data, err := json.Marshal(categories)
	if err != nil {
		return fmt.Errorf("failed to marshal categories: %w", err)
	}

	// Сохраняем в Redis с указанным TTL
	if err := r.client.Set(ctx, categoriesCacheKey, data, ttl).Err(); err != nil {
		return fmt.Errorf("failed to set categories in cache: %w", err)
	}

	return nil
}

// GetCategories получает список категорий из кеша Redis
// Возвращает nil, nil если ключ не найден (cache miss)
func (r *RedisClient) GetCategories(ctx context.Context) ([]entity.Category, error) {
	// Получаем данные из Redis
	data, err := r.client.Get(ctx, categoriesCacheKey).Bytes()
	if err != nil {
		if err == redis.Nil {
			// Ключ не найден - это не ошибка, просто cache miss
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get categories from cache: %w", err)
	}

	// Десериализуем JSON в массив категорий
	var categories []entity.Category
	if err := json.Unmarshal(data, &categories); err != nil {
		return nil, fmt.Errorf("failed to unmarshal categories: %w", err)
	}

	return categories, nil
}

// DeleteCategories удаляет кеш категорий (инвалидация)
// Вызывается при создании/обновлении/удалении категорий
func (r *RedisClient) DeleteCategories(ctx context.Context) error {
	if err := r.client.Del(ctx, categoriesCacheKey).Err(); err != nil {
		return fmt.Errorf("failed to delete categories from cache: %w", err)
	}
	return nil
}

// Close закрывает соединение с Redis
// Должен вызываться при завершении работы приложения
func (r *RedisClient) Close() error {
	return r.client.Close()
}
