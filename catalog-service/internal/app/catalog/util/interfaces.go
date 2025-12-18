package util

import (
	"context"
	"time"

	"augustberries/catalog-service/internal/app/catalog/entity"
)

type RedisCache interface {
	SetCategories(ctx context.Context, categories []entity.Category, ttl time.Duration) error
	GetCategories(ctx context.Context) ([]entity.Category, error)
	DeleteCategories(ctx context.Context) error
	Close() error
}

type MessagePublisher interface {
	PublishMessage(ctx context.Context, key string, value []byte) error
	Close() error
}
