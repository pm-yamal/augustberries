package repository

import (
	"context"
	"errors"
	"fmt"

	"augustberries/background-worker-service/internal/app/background-worker/entity"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// orderRepository реализует OrderRepository для работы с PostgreSQL через GORM
type orderRepository struct {
	db *gorm.DB
}

// NewOrderRepository создает новый репозиторий заказов
func NewOrderRepository(db *gorm.DB) OrderRepository {
	return &orderRepository{db: db}
}

// GetByID получает заказ по ID
func (r *orderRepository) GetByID(ctx context.Context, orderID uuid.UUID) (*entity.Order, error) {
	var order entity.Order

	// Выполняем запрос к БД с учетом контекста
	result := r.db.WithContext(ctx).Where("id = ?", orderID).First(&order)

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("order not found: %w", result.Error)
		}
		return nil, fmt.Errorf("failed to get order: %w", result.Error)
	}

	return &order, nil
}

// Update обновляет заказ
func (r *orderRepository) Update(ctx context.Context, order *entity.Order) error {
	// Обновляем все поля заказа
	result := r.db.WithContext(ctx).Save(order)

	if result.Error != nil {
		return fmt.Errorf("failed to update order: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("order not found or no changes made")
	}

	return nil
}

// UpdateDeliveryAndTotal обновляет цену доставки и общую сумму заказа
// Используется после расчета стоимости доставки с учетом курсов валют
func (r *orderRepository) UpdateDeliveryAndTotal(ctx context.Context, orderID uuid.UUID, deliveryPrice, totalPrice float64) error {
	// Выполняем точечное обновление двух полей
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

// UpdateOrderWithCurrency обновляет цену доставки, общую сумму и валюту заказа
// Используется после расчета стоимости доставки с конвертацией в RUB
func (r *orderRepository) UpdateOrderWithCurrency(ctx context.Context, orderID uuid.UUID, deliveryPrice, totalPrice float64, currency string) error {
	// Выполняем точечное обновление трех полей
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
