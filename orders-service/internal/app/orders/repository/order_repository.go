package repository

import (
	"context"
	"errors"

	"augustberries/orders-service/internal/app/orders/entity"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	// Стандартные ошибки репозитория для обработки в service layer
	ErrOrderNotFound = errors.New("order not found")
)

type orderRepository struct {
	db *gorm.DB // GORM DB для работы с PostgreSQL
}

// NewOrderRepository создает новый репозиторий заказов
func NewOrderRepository(db *gorm.DB) OrderRepository {
	return &orderRepository{db: db}
}

// Create создает новый заказ в PostgreSQL
func (r *orderRepository) Create(ctx context.Context, order *entity.Order) error {
	result := r.db.WithContext(ctx).Create(order)
	return result.Error
}

// GetByID получает заказ по ID из PostgreSQL
func (r *orderRepository) GetByID(ctx context.Context, id uuid.UUID) (*entity.Order, error) {
	var order entity.Order
	result := r.db.WithContext(ctx).First(&order, "id = ?", id)

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, ErrOrderNotFound
		}
		return nil, result.Error
	}

	return &order, nil
}

// GetByUserID получает все заказы пользователя
func (r *orderRepository) GetByUserID(ctx context.Context, userID uuid.UUID) ([]entity.Order, error) {
	var orders []entity.Order
	result := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&orders)

	if result.Error != nil {
		return nil, result.Error
	}

	return orders, nil
}

// Update обновляет заказ в PostgreSQL
func (r *orderRepository) Update(ctx context.Context, order *entity.Order) error {
	result := r.db.WithContext(ctx).Model(order).
		Where("id = ?", order.ID).
		Updates(map[string]interface{}{
			"status":         order.Status,
			"total_price":    order.TotalPrice,
			"delivery_price": order.DeliveryPrice,
			"currency":       order.Currency,
		})

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return ErrOrderNotFound
	}

	return nil
}

// Delete удаляет заказ из PostgreSQL
// Позиции заказа удаляются автоматически через CASCADE
func (r *orderRepository) Delete(ctx context.Context, id uuid.UUID) error {
	result := r.db.WithContext(ctx).Delete(&entity.Order{}, "id = ?", id)

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return ErrOrderNotFound
	}

	return nil
}

// GetWithItems получает заказ с полным списком позиций
func (r *orderRepository) GetWithItems(ctx context.Context, id uuid.UUID) (*entity.OrderWithItems, error) {
	var order entity.Order
	result := r.db.WithContext(ctx).
		Preload("Items").
		First(&order, "id = ?", id)

	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, ErrOrderNotFound
		}
		return nil, result.Error
	}

	return &entity.OrderWithItems{
		Order: order,
		Items: order.Items,
	}, nil
}
