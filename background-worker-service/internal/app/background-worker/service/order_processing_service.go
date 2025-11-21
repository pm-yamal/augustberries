package service

import (
	"context"
	"fmt"
	"log"

	"augustberries/background-worker-service/internal/app/background-worker/entity"
	"augustberries/background-worker-service/internal/app/background-worker/repository"

	"github.com/google/uuid"
)

// ExchangeRateServiceInterface определяет интерфейс для работы с курсами валют
type ExchangeRateServiceInterface interface {
	// GetRate получает курс валюты из Redis
	GetRate(ctx context.Context, currency string) (*entity.ExchangeRate, error)
	// GetRates получает курсы нескольких валют из Redis
	GetRates(ctx context.Context, currencies []string) (map[string]*entity.ExchangeRate, error)
	// ConvertCurrency конвертирует сумму из одной валюты в другую
	ConvertCurrency(ctx context.Context, amount float64, fromCurrency, toCurrency string) (float64, float64, error)
	// FetchAndStoreRates получает курсы валют из внешнего API и сохраняет в Redis
	FetchAndStoreRates(ctx context.Context) error
}

// OrderProcessingService обрабатывает заказы и рассчитывает доставку
type OrderProcessingService struct {
	orderRepo   repository.OrderRepository
	exchangeSvc ExchangeRateServiceInterface
}

// NewOrderProcessingService создает новый сервис обработки заказов
func NewOrderProcessingService(
	orderRepo repository.OrderRepository,
	exchangeSvc ExchangeRateServiceInterface,
) *OrderProcessingService {
	return &OrderProcessingService{
		orderRepo:   orderRepo,
		exchangeSvc: exchangeSvc,
	}
}

// ProcessOrderCreated обрабатывает событие ORDER_CREATED
// ЛОГИКА:
// 1. Получить цены товаров из заказа (они в USD согласно каталогу)
// 2. Конвертировать в RUB
// 3. Рассчитать доставку в RUB
// 4. Сохранить заказ с currency = "RUB"
func (s *OrderProcessingService) ProcessOrderCreated(ctx context.Context, event *entity.OrderEvent) error {
	log.Printf("Processing ORDER_CREATED for order %s (currency: %s)", event.OrderID, event.Currency)

	// Получаем заказ из БД
	order, err := s.orderRepo.GetByID(ctx, event.OrderID)
	if err != nil {
		return fmt.Errorf("failed to get order: %w", err)
	}

	// Проверяем что у заказа есть стоимость доставки для обработки
	if order.DeliveryPrice == 0 {
		log.Printf("Order %s has zero delivery price, skipping processing", order.ID)
		return nil
	}

	// Рассчитываем доставку с учетом курса валюты
	calculation, err := s.calculateDeliveryWithExchange(ctx, order)
	if err != nil {
		return fmt.Errorf("failed to calculate delivery: %w", err)
	}

	// Обновляем заказ в БД: доставка, итоговая цена и валюта RUB
	if err := s.orderRepo.UpdateOrderWithCurrency(
		ctx,
		order.ID,
		calculation.ConvertedDelivery,
		calculation.NewTotalPrice,
		"RUB", // Сохраняем заказ с currency = "RUB"
	); err != nil {
		return fmt.Errorf("failed to update order: %w", err)
	}

	log.Printf("Successfully processed order %s: delivery %.2f %s -> %.2f %s (rate: %.4f), total: %.2f RUB",
		order.ID,
		calculation.OriginalDelivery,
		calculation.OriginalCurrency,
		calculation.ConvertedDelivery,
		calculation.ConvertedCurrency,
		calculation.ExchangeRate,
		calculation.NewTotalPrice,
	)

	return nil
}

// calculateDeliveryWithExchange рассчитывает стоимость доставки с учетом курса валюты
// ЛОГИКА:
// 1. Получить цены товаров из заказа (они в USD согласно каталогу)
// 2. Конвертировать товары и доставку из USD в RUB
// 3. Сохранить заказ с currency = "RUB"
func (s *OrderProcessingService) calculateDeliveryWithExchange(
	ctx context.Context,
	order *entity.Order,
) (*entity.DeliveryCalculation, error) {
	// Целевая валюта всегда RUB
	targetCurrency := "RUB"

	// Исходная валюта заказа (по умолчанию USD согласно каталогу)
	sourceCurrency := order.Currency
	if sourceCurrency == "" {
		sourceCurrency = "USD"
	}

	// Конвертируем стоимость доставки из USD в RUB
	convertedDelivery, exchangeRate, err := s.exchangeSvc.ConvertCurrency(
		ctx,
		order.DeliveryPrice,
		sourceCurrency,
		targetCurrency,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to convert delivery price from %s to %s: %w", sourceCurrency, targetCurrency, err)
	}

	// Конвертируем цену товаров (без доставки) из USD в RUB
	priceWithoutDelivery := order.TotalPrice - order.DeliveryPrice

	convertedPrice, _, err := s.exchangeSvc.ConvertCurrency(
		ctx,
		priceWithoutDelivery,
		sourceCurrency,
		targetCurrency,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to convert total price from %s to %s: %w", sourceCurrency, targetCurrency, err)
	}

	// Новая итоговая сумма в RUB = конвертированная цена товаров + конвертированная доставка
	newTotal := convertedPrice + convertedDelivery

	return &entity.DeliveryCalculation{
		OrderID:           order.ID,
		OriginalDelivery:  order.DeliveryPrice,
		OriginalCurrency:  sourceCurrency,
		ConvertedDelivery: convertedDelivery,
		ConvertedCurrency: targetCurrency, // Всегда RUB
		ExchangeRate:      exchangeRate,
		NewTotalPrice:     newTotal,
		CalculatedAt:      order.CreatedAt,
	}, nil
}

// ProcessOrderEvent обрабатывает событие заказа из Kafka
func (s *OrderProcessingService) ProcessOrderEvent(ctx context.Context, event *entity.OrderEvent) error {
	switch event.EventType {
	case entity.EventTypeOrderCreated:
		return s.ProcessOrderCreated(ctx, event)
	case entity.EventTypeOrderUpdated:
		// Для ORDER_UPDATED пока не требуется обработка согласно ТЗ
		log.Printf("Received ORDER_UPDATED event for order %s, skipping processing", event.OrderID)
		return nil
	default:
		log.Printf("Unknown event type: %s for order %s", event.EventType, event.OrderID)
		return nil
	}
}

// ValidateOrder проверяет корректность данных заказа
func (s *OrderProcessingService) ValidateOrder(order *entity.Order) error {
	if order.ID == uuid.Nil {
		return fmt.Errorf("invalid order ID")
	}

	if order.UserID == uuid.Nil {
		return fmt.Errorf("invalid user ID")
	}

	if order.Currency == "" {
		return fmt.Errorf("currency not specified")
	}

	if order.DeliveryPrice < 0 {
		return fmt.Errorf("delivery price cannot be negative")
	}

	if order.TotalPrice < 0 {
		return fmt.Errorf("total price cannot be negative")
	}

	return nil
}
