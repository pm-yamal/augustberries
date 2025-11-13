package entity

import "github.com/google/uuid"

// CreateOrderRequest - запрос на создание заказа
type CreateOrderRequest struct {
	Items         []OrderItemRequest `json:"items" validate:"required,min=1,dive"`
	DeliveryPrice float64            `json:"delivery_price" validate:"gte=0"`
	Currency      string             `json:"currency" validate:"required,oneof=USD EUR RUB"`
}

// OrderItemRequest - позиция заказа в запросе
type OrderItemRequest struct {
	ProductID uuid.UUID `json:"product_id" validate:"required"`
	Quantity  int       `json:"quantity" validate:"required,gt=0"`
}

// UpdateOrderStatusRequest - запрос на обновление статуса заказа
type UpdateOrderStatusRequest struct {
	Status OrderStatus `json:"status" validate:"required,oneof=pending confirmed shipped delivered cancelled"`
}

// ErrorResponse - стандартный ответ об ошибке
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

// SuccessResponse - стандартный ответ об успехе
type SuccessResponse struct {
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// OrderResponse - полный ответ с заказом
type OrderResponse struct {
	ID            uuid.UUID      `json:"id"`
	UserID        uuid.UUID      `json:"user_id"`
	TotalPrice    float64        `json:"total_price"`
	DeliveryPrice float64        `json:"delivery_price"`
	Currency      string         `json:"currency"`
	Status        OrderStatus    `json:"status"`
	CreatedAt     string         `json:"created_at"`
	Items         []ItemResponse `json:"items"`
}

// ItemResponse - позиция заказа в ответе
type ItemResponse struct {
	ID          uuid.UUID `json:"id"`
	ProductID   uuid.UUID `json:"product_id"`
	ProductName string    `json:"product_name,omitempty"`
	Quantity    int       `json:"quantity"`
	UnitPrice   float64   `json:"unit_price"`
	TotalPrice  float64   `json:"total_price"`
}
