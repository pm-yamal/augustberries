package infrastructure

import (
	"context"

	"augustberries/orders-service/internal/app/orders/entity"

	"github.com/google/uuid"
)

// MessagePublisher интерфейс для отправки сообщений в очередь (Kafka)
// Используется для dependency injection и упрощения тестирования
type MessagePublisher interface {
	PublishMessage(ctx context.Context, key string, value []byte) error
	Close() error
}

// CatalogServiceClient интерфейс для взаимодействия с Catalog Service
// Используется для dependency injection и упрощения тестирования
type CatalogServiceClient interface {
	SetAuthToken(token string)
	GetProduct(ctx context.Context, productID uuid.UUID) (*entity.ProductWithCategory, error)
	GetProducts(ctx context.Context, productIDs []uuid.UUID) (map[uuid.UUID]*entity.ProductWithCategory, error)
}
