package util

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"augustberries/catalog-service/internal/app/catalog/entity"

	"github.com/redis/go-redis/v9"
)

const categoriesCacheKey = "categories:all"

type RedisClient struct {
	client *redis.Client
}

func NewRedisClient(addr, password string, db int) (*RedisClient, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	return &RedisClient{client: client}, nil
}

func (r *RedisClient) SetCategories(ctx context.Context, categories []entity.Category, ttl time.Duration) error {
	data, err := json.Marshal(categories)
	if err != nil {
		return fmt.Errorf("failed to marshal categories: %w", err)
	}

	if err := r.client.Set(ctx, categoriesCacheKey, data, ttl).Err(); err != nil {
		return fmt.Errorf("failed to set categories in cache: %w", err)
	}

	return nil
}

func (r *RedisClient) GetCategories(ctx context.Context) ([]entity.Category, error) {
	data, err := r.client.Get(ctx, categoriesCacheKey).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get categories from cache: %w", err)
	}

	var categories []entity.Category
	if err := json.Unmarshal(data, &categories); err != nil {
		return nil, fmt.Errorf("failed to unmarshal categories: %w", err)
	}

	return categories, nil
}

func (r *RedisClient) DeleteCategories(ctx context.Context) error {
	if err := r.client.Del(ctx, categoriesCacheKey).Err(); err != nil {
		return fmt.Errorf("failed to delete categories from cache: %w", err)
	}
	return nil
}

func (r *RedisClient) Close() error {
	return r.client.Close()
}
