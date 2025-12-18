package entity

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Review struct {
	ID        primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	ProductID string             `json:"product_id" bson:"product_id"` // UUID товара из Catalog Service
	UserID    string             `json:"user_id" bson:"user_id"`       // UUID пользователя из Auth Service
	Rating    int                `json:"rating" bson:"rating"`         // Оценка от 1 до 5
	Text      string             `json:"text" bson:"text"`             // Текст отзыва
	CreatedAt time.Time          `json:"created_at" bson:"created_at"`
	UpdatedAt time.Time          `json:"updated_at" bson:"updated_at"`
}

type ReviewEvent struct {
	EventType string    `json:"event_type"` // REVIEW_CREATED
	ReviewID  string    `json:"review_id"`
	ProductID string    `json:"product_id"`
	UserID    string    `json:"user_id"`
	Rating    int       `json:"rating"`
	Timestamp time.Time `json:"timestamp"`
}
