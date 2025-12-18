package repository

import (
	"augustberries/background-worker-service/internal/app/background-worker/entity"
	"context"

	"github.com/google/uuid"
)

type OrderRepository interface {
	GetByID(ctx context.Context, orderID uuid.UUID) (*entity.Order, error)

	Update(ctx context.Context, order *entity.Order) error

	UpdateDeliveryAndTotal(ctx context.Context, orderID uuid.UUID, deliveryPrice, totalPrice float64) error

	UpdateOrderWithCurrency(ctx context.Context, orderID uuid.UUID, deliveryPrice, totalPrice float64, currency string) error
}

type ExchangeRateRepository interface {
	Get(ctx context.Context, currency string) (*entity.ExchangeRate, error)

	Set(ctx context.Context, rate *entity.ExchangeRate) error

	SetMultiple(ctx context.Context, rates []*entity.ExchangeRate) error

	GetMultiple(ctx context.Context, currencies []string) (map[string]*entity.ExchangeRate, error)

	Exists(ctx context.Context, currency string) (bool, error)
}
