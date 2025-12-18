package entity

import (
	"time"

	"github.com/google/uuid"
)

type Order struct {
	ID            uuid.UUID   `json:"id" gorm:"type:uuid;primaryKey"`
	UserID        uuid.UUID   `json:"user_id" gorm:"type:uuid;not null"`                       // ID пользователя из Auth Service
	TotalPrice    float64     `json:"total_price" gorm:"type:decimal(10,2);not null"`          // Итоговая стоимость в валюте клиента
	DeliveryPrice float64     `json:"delivery_price" gorm:"type:decimal(10,2);not null"`       // Цена доставки
	Currency      string      `json:"currency" gorm:"type:varchar(10);not null;default:'RUB'"` // Валюта (USD, EUR, RUB и т.п.)
	Status        OrderStatus `json:"status" gorm:"type:varchar(50);not null;default:'pending'"`
	CreatedAt     time.Time   `json:"created_at" gorm:"autoCreateTime"`
	Items         []OrderItem `json:"items,omitempty" gorm:"foreignKey:OrderID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}

func (Order) TableName() string {
	return "orders"
}

type OrderStatus string

const (
	OrderStatusPending   OrderStatus = "pending"   // Ожидает обработки
	OrderStatusConfirmed OrderStatus = "confirmed" // Подтвержден
	OrderStatusShipped   OrderStatus = "shipped"   // Отправлен
	OrderStatusDelivered OrderStatus = "delivered" // Доставлен
	OrderStatusCancelled OrderStatus = "cancelled" // Отменен
)

type OrderItem struct {
	ID        uuid.UUID `json:"id" gorm:"type:uuid;primaryKey"`
	OrderID   uuid.UUID `json:"order_id" gorm:"type:uuid;not null"` // Ссылка на заказ
	ProductID uuid.UUID `json:"product_id" gorm:"type:uuid;not null"`
	Quantity  int       `json:"quantity" gorm:"not null;check:quantity > 0"`
	UnitPrice float64   `json:"unit_price" gorm:"type:decimal(10,2);not null"` // Цена за единицу на момент покупки
}

func (OrderItem) TableName() string {
	return "order_items"
}

type OrderWithItems struct {
	Order
	Items []OrderItem `json:"items"`
}

type OrderEvent struct {
	EventType    string      `json:"event_type"` // ORDER_CREATED, ORDER_UPDATED
	OrderID      uuid.UUID   `json:"order_id"`
	UserID       uuid.UUID   `json:"user_id"`
	TotalPrice   float64     `json:"total_price"`
	Currency     string      `json:"currency"`
	Status       OrderStatus `json:"status"`
	ItemsCount   int         `json:"items_count"`
	Timestamp    time.Time   `json:"timestamp"`
}

type Product struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Price       float64   `json:"price"`
	CategoryID  uuid.UUID `json:"category_id"`
}

type ProductWithCategory struct {
	Product
	Category Category `json:"category"`
}

type Category struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}
