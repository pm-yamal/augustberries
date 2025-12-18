package infrastructure

import (
	"context"

	"augustberries/orders-service/internal/app/orders/entity"

	"github.com/google/uuid"
)

type MessagePublisher interface {
	PublishMessage(ctx context.Context, key string, value []byte) error
	Close() error
}

type CatalogServiceClient interface {
	SetAuthToken(token string)
	GetProduct(ctx context.Context, productID uuid.UUID) (*entity.ProductWithCategory, error)
	GetProducts(ctx context.Context, productIDs []uuid.UUID) (map[uuid.UUID]*entity.ProductWithCategory, error)
}
