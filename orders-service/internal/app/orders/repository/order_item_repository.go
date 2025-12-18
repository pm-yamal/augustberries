package repository

import (
	"context"

	"augustberries/orders-service/internal/app/orders/entity"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type orderItemRepository struct {
	db *gorm.DB
}

func NewOrderItemRepository(db *gorm.DB) OrderItemRepository {
	return &orderItemRepository{db: db}
}

func (r *orderItemRepository) Create(ctx context.Context, item *entity.OrderItem) error {
	result := r.db.WithContext(ctx).Create(item)
	return result.Error
}

func (r *orderItemRepository) GetByOrderID(ctx context.Context, orderID uuid.UUID) ([]entity.OrderItem, error) {
	var items []entity.OrderItem
	result := r.db.WithContext(ctx).
		Where("order_id = ?", orderID).
		Find(&items)

	if result.Error != nil {
		return nil, result.Error
	}

	return items, nil
}

func (r *orderItemRepository) DeleteByOrderID(ctx context.Context, orderID uuid.UUID) error {
	result := r.db.WithContext(ctx).
		Where("order_id = ?", orderID).
		Delete(&entity.OrderItem{})

	return result.Error
}
