package repository

import (
	"context"

	"augustberries/orders-service/internal/app/orders/entity"

	"github.com/google/uuid"
)

// OrderRepository определяет методы для работы с заказами
type OrderRepository interface {
	Create(ctx context.Context, order *entity.Order) error
	GetByID(ctx context.Context, id uuid.UUID) (*entity.Order, error)
	GetByUserID(ctx context.Context, userID uuid.UUID) ([]entity.Order, error)
	Update(ctx context.Context, order *entity.Order) error
	Delete(ctx context.Context, id uuid.UUID) error
	GetWithItems(ctx context.Context, id uuid.UUID) (*entity.OrderWithItems, error)
}

// OrderItemRepository определяет методы для работы с позициями заказов
type OrderItemRepository interface {
	Create(ctx context.Context, item *entity.OrderItem) error
	GetByOrderID(ctx context.Context, orderID uuid.UUID) ([]entity.OrderItem, error)
	DeleteByOrderID(ctx context.Context, orderID uuid.UUID) error
}
