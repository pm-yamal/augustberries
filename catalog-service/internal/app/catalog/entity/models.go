package entity

import (
	"time"

	"github.com/google/uuid"
)

// Category представляет категорию товаров
type Category struct {
	ID        uuid.UUID `json:"id" db:"id"`
	Name      string    `json:"name" db:"name"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// Product представляет товар в каталоге
type Product struct {
	ID          uuid.UUID `json:"id" db:"id"`
	Name        string    `json:"name" db:"name"`
	Description string    `json:"description" db:"description"`
	Price       float64   `json:"price" db:"price"` // Цена в базовой валюте (USD)
	CategoryID  uuid.UUID `json:"category_id" db:"category_id"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
}

// ProductWithCategory содержит продукт с информацией о категории
type ProductWithCategory struct {
	Product
	Category Category `json:"category"`
}

// ProductEvent представляет событие изменения продукта для Kafka
type ProductEvent struct {
	EventType  string    `json:"event_type"` // PRODUCT_CREATED, PRODUCT_UPDATED, PRODUCT_DELETED
	ProductID  uuid.UUID `json:"product_id"`
	Name       string    `json:"name"`
	Price      float64   `json:"price"`
	CategoryID uuid.UUID `json:"category_id"`
	Timestamp  time.Time `json:"timestamp"`
}
