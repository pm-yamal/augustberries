package service

import (
	"context"

	"augustberries/orders-service/internal/app/orders/entity"

	"github.com/google/uuid"
)

type OrderServiceInterface interface {
	CreateOrder(ctx context.Context, userID uuid.UUID, req *entity.CreateOrderRequest, authToken string) (*entity.OrderWithItems, error)
	GetOrder(ctx context.Context, orderID uuid.UUID, userID uuid.UUID) (*entity.OrderWithItems, error)
	UpdateOrderStatus(ctx context.Context, orderID uuid.UUID, userID uuid.UUID, newStatus entity.OrderStatus) (*entity.Order, error)
	DeleteOrder(ctx context.Context, orderID uuid.UUID, userID uuid.UUID) error
	GetUserOrders(ctx context.Context, userID uuid.UUID) ([]entity.Order, error)
}
