package entity

import (
	"time"

	"github.com/google/uuid"
)

// Order представляет заказ из Orders Service
// Структура должна совпадать с orders-service/entity/Order
type Order struct {
	ID            uuid.UUID   `json:"id" gorm:"type:uuid;primaryKey"`
	UserID        uuid.UUID   `json:"user_id" gorm:"type:uuid;not null"`
	TotalPrice    float64     `json:"total_price" gorm:"type:decimal(10,2);not null"`
	DeliveryPrice float64     `json:"delivery_price" gorm:"type:decimal(10,2);not null"`
	Currency      string      `json:"currency" gorm:"type:varchar(10);not null;default:'RUB'"`
	Status        OrderStatus `json:"status" gorm:"type:varchar(50);not null;default:'pending'"`
	CreatedAt     time.Time   `json:"created_at" gorm:"autoCreateTime"`
}

// TableName указывает имя таблицы для GORM
func (Order) TableName() string {
	return "orders"
}

// OrderStatus представляет статусы заказа
type OrderStatus string

const (
	OrderStatusPending   OrderStatus = "pending"
	OrderStatusConfirmed OrderStatus = "confirmed"
	OrderStatusShipped   OrderStatus = "shipped"
	OrderStatusDelivered OrderStatus = "delivered"
	OrderStatusCancelled OrderStatus = "cancelled"
)

// OrderEvent представляет событие из Kafka топика order_events
// Структура должна совпадать с orders-service/entity/OrderEvent
type OrderEvent struct {
	EventType  string      `json:"event_type"` // ORDER_CREATED, ORDER_UPDATED
	OrderID    uuid.UUID   `json:"order_id"`
	UserID     uuid.UUID   `json:"user_id"`
	TotalPrice float64     `json:"total_price"`
	Currency   string      `json:"currency"`
	Status     OrderStatus `json:"status"`
	ItemsCount int         `json:"items_count"`
	Timestamp  time.Time   `json:"timestamp"`
}

// ExchangeRate представляет курс валюты
// Хранится в Redis с префиксом "rates:"
type ExchangeRate struct {
	Currency  string    `json:"currency"`   // Код валюты (USD, EUR, RUB и т.д.)
	Rate      float64   `json:"rate"`       // Курс относительно базовой валюты (USD)
	UpdatedAt time.Time `json:"updated_at"` // Время последнего обновления
}

// ExchangeRatesResponse представляет ответ от API exchangerate-api.com
// Используется для парсинга JSON ответа от внешнего API
type ExchangeRatesResponse struct {
	Base  string             `json:"base"`  // Базовая валюта (обычно USD)
	Date  string             `json:"date"`  // Дата курсов
	Rates map[string]float64 `json:"rates"` // Курсы валют: {"EUR": 0.93, "RUB": 91.23, ...}
}

// DeliveryCalculation представляет результат расчета доставки
type DeliveryCalculation struct {
	OrderID           uuid.UUID // ID заказа
	OriginalDelivery  float64   // Исходная цена доставки
	OriginalCurrency  string    // Исходная валюта
	ConvertedDelivery float64   // Сконвертированная цена доставки
	ConvertedCurrency string    // Целевая валюта (обычно USD или RUB)
	ExchangeRate      float64   // Использованный курс
	NewTotalPrice     float64   // Новая итоговая сумма заказа
	CalculatedAt      time.Time // Время расчета
}

// Константы для типов событий
const (
	EventTypeOrderCreated = "ORDER_CREATED"
	EventTypeOrderUpdated = "ORDER_UPDATED"
)

// Константы для префиксов Redis ключей
const (
	RedisKeyPrefixRate = "rates:" // Префикс для хранения курсов валют: rates:USD, rates:EUR
)

// Поддерживаемые валюты для конвертации
var SupportedCurrencies = []string{"USD", "EUR", "RUB", "GBP", "JPY", "CNY"}

// GetRedisKeyForRate возвращает Redis ключ для курса валюты
func GetRedisKeyForRate(currency string) string {
	return RedisKeyPrefixRate + currency
}
