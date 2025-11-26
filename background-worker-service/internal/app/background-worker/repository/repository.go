package repository

import (
	"augustberries/background-worker-service/internal/app/background-worker/entity"
	"context"

	"github.com/google/uuid"
)

// OrderRepository интерфейс для работы с заказами в PostgreSQL
type OrderRepository interface {
	// GetByID получает заказ по ID
	GetByID(ctx context.Context, orderID uuid.UUID) (*entity.Order, error)

	// Update обновляет заказ
	Update(ctx context.Context, order *entity.Order) error

	// UpdateDeliveryAndTotal обновляет цену доставки и общую сумму заказа
	UpdateDeliveryAndTotal(ctx context.Context, orderID uuid.UUID, deliveryPrice, totalPrice float64) error

	// UpdateOrderWithCurrency обновляет цену доставки, общую сумму и валюту заказа
	UpdateOrderWithCurrency(ctx context.Context, orderID uuid.UUID, deliveryPrice, totalPrice float64, currency string) error
}

// ExchangeRateRepository интерфейс для работы с курсами валют в Redis
type ExchangeRateRepository interface {
	// Get получает курс валюты из Redis
	Get(ctx context.Context, currency string) (*entity.ExchangeRate, error)

	// Set сохраняет курс валюты в Redis с TTL
	Set(ctx context.Context, rate *entity.ExchangeRate) error

	// SetMultiple сохраняет несколько курсов валют батчем
	SetMultiple(ctx context.Context, rates []*entity.ExchangeRate) error

	// GetMultiple получает несколько курсов валют
	GetMultiple(ctx context.Context, currencies []string) (map[string]*entity.ExchangeRate, error)

	// Exists проверяет существование курса в Redis
	Exists(ctx context.Context, currency string) (bool, error)
}
