package entity

import (
	"time"

	"github.com/google/uuid"
)

type Category struct {
	ID        uuid.UUID `json:"id" gorm:"type:uuid;primaryKey"`
	Name      string    `json:"name" gorm:"type:varchar(255);unique;not null"`
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
}

func (Category) TableName() string {
	return "categories"
}

type Product struct {
	ID          uuid.UUID `json:"id" gorm:"type:uuid;primaryKey"`
	Name        string    `json:"name" gorm:"type:varchar(255);not null"`
	Description string    `json:"description" gorm:"type:text"`
	Price       float64   `json:"price" gorm:"type:decimal(10,2);not null"` // Цена в базовой валюте (USD)
	CategoryID  uuid.UUID `json:"category_id" gorm:"type:uuid;not null"`
	Category    *Category `json:"category,omitempty" gorm:"foreignKey:CategoryID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:RESTRICT"`
	CreatedAt   time.Time `json:"created_at" gorm:"autoCreateTime"`
}

func (Product) TableName() string {
	return "products"
}

type ProductWithCategory struct {
	Product
	Category Category `json:"category"`
}

type ProductEvent struct {
	EventType  string    `json:"event_type"` // PRODUCT_CREATED, PRODUCT_UPDATED, PRODUCT_DELETED
	ProductID  uuid.UUID `json:"product_id"`
	Name       string    `json:"name"`
	Price      float64   `json:"price"`
	CategoryID uuid.UUID `json:"category_id"`
	Timestamp  time.Time `json:"timestamp"`
}
