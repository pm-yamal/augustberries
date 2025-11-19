package service

import (
	"context"
	"fmt"
	"log"

	"augustberries/background-worker-service/internal/app/background-worker/entity"
	"augustberries/background-worker-service/internal/app/background-worker/repository"
	"github.com/google/uuid"
)

// OrderProcessingService обрабатывает заказы и рассчитывает доставку
type OrderProcessingService struct {
	orderRepo   repository.OrderRepository
	exchangeSvc *ExchangeRateService
}

// NewOrderProcessingService создает новый сервис обработки заказов
func NewOrderProcessingService(
	orderRepo repository.OrderRepository,
	exchangeSvc *ExchangeRateService,
) *OrderProcessingService {
	return &OrderProcessingService{
		orderRepo:   orderRepo,
		exchangeSvc: exchangeSvc,
	}
}

// ProcessOrderCreated обрабатывает событие ORDER_CREATED
// 1. Получает заказ из БД
// 2. Получает курс валюты из Redis
// 3. Рассчитывает стоимость доставки в нужной валюте
// 4. Обновляет заказ в PostgreSQL
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

	// Обновляем заказ в БД
	if err := s.orderRepo.UpdateDeliveryAndTotal(
		ctx,
		order.ID,
		calculation.ConvertedDelivery,
		calculation.NewTotalPrice,
	); err != nil {
		return fmt.Errorf("failed to update order: %w", err)
	}

	log.Printf("Successfully processed order %s: delivery %.2f %s -> %.2f %s (rate: %.4f), total: %.2f",
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
func (s *OrderProcessingService) calculateDeliveryWithExchange(
	ctx context.Context,
	order *entity.Order,
) (*entity.DeliveryCalculation, error) {
	// Определяем базовую валюту для расчетов (например, USD)
	baseCurrency := "USD"

	// Конвертируем стоимость доставки из валюты заказа в базовую валюту
	convertedDelivery, exchangeRate, err := s.exchangeSvc.ConvertCurrency(
		ctx,
		order.DeliveryPrice,
		order.Currency,
		baseCurrency,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to convert delivery price: %w", err)
	}

	// Рассчитываем новую итоговую сумму
	// Конвертируем текущую сумму без доставки
	priceWithoutDelivery := order.TotalPrice - order.DeliveryPrice

	convertedPrice, _, err := s.exchangeSvc.ConvertCurrency(
		ctx,
		priceWithoutDelivery,
		order.Currency,
		baseCurrency,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to convert total price: %w", err)
	}

	// Новая итоговая сумма = конвертированная цена товаров + конвертированная доставка
	newTotal := convertedPrice + convertedDelivery

	return &entity.DeliveryCalculation{
		OrderID:           order.ID,
		OriginalDelivery:  order.DeliveryPrice,
		OriginalCurrency:  order.Currency,
		ConvertedDelivery: convertedDelivery,
		ConvertedCurrency: baseCurrency,
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
