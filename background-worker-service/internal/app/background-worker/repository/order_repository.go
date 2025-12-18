package repository

import (
	"context"
	"errors"
	"fmt"

	"augustberries/background-worker-service/internal/app/background-worker/entity"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type orderRepository struct {
	db *gorm.DB
}

func NewOrderRepository(db *gorm.DB) OrderRepository {
	return &orderRepository{db: db}
}

func (r *orderRepository) GetByID(ctx context.Context, orderID uuid.UUID) (*entity.Order, error) {
	var order entity.Order

	result := r.db.WithContext(ctx).Where("id = ?", orderID).First(&order)

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("order not found: %w", result.Error)
		}
		return nil, fmt.Errorf("failed to get order: %w", result.Error)
	}

	return &order, nil
}

func (r *orderRepository) Update(ctx context.Context, order *entity.Order) error {
	result := r.db.WithContext(ctx).Save(order)

	if result.Error != nil {
		return fmt.Errorf("failed to update order: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("order not found or no changes made")
	}

	return nil
}

func (r *orderRepository) UpdateDeliveryAndTotal(ctx context.Context, orderID uuid.UUID, deliveryPrice, totalPrice float64) error {
	result := r.db.WithContext(ctx).
		Model(&entity.Order{}).
		Where("id = ?", orderID).
		Updates(map[string]interface{}{
			"delivery_price": deliveryPrice,
			"total_price":    totalPrice,
		})

	if result.Error != nil {
		return fmt.Errorf("failed to update delivery and total price: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("order %s not found", orderID)
	}

	return nil
}

func (r *orderRepository) UpdateOrderWithCurrency(ctx context.Context, orderID uuid.UUID, deliveryPrice, totalPrice float64, currency string) error {
	result := r.db.WithContext(ctx).
		Model(&entity.Order{}).
		Where("id = ?", orderID).
		Updates(map[string]interface{}{
			"delivery_price": deliveryPrice,
			"total_price":    totalPrice,
			"currency":       currency,
		})

	if result.Error != nil {
		return fmt.Errorf("failed to update delivery, total price and currency: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("order %s not found", orderID)
	}

	return nil
}
