package service

import (
	"context"
	"fmt"
	"log"

	"augustberries/background-worker-service/internal/app/background-worker/entity"
	"augustberries/background-worker-service/internal/app/background-worker/repository"

	"github.com/google/uuid"
)

type OrderProcessingService struct {
	orderRepo   repository.OrderRepository
	exchangeSvc ExchangeRateServiceInterface
}

func NewOrderProcessingService(
	orderRepo repository.OrderRepository,
	exchangeSvc ExchangeRateServiceInterface,
) *OrderProcessingService {
	return &OrderProcessingService{
		orderRepo:   orderRepo,
		exchangeSvc: exchangeSvc,
	}
}

func (s *OrderProcessingService) ProcessOrderCreated(ctx context.Context, event *entity.OrderEvent) error {
	log.Printf("Processing ORDER_CREATED for order %s (currency: %s)", event.OrderID, event.Currency)

	order, err := s.orderRepo.GetByID(ctx, event.OrderID)
	if err != nil {
		return fmt.Errorf("failed to get order: %w", err)
	}

	if err := s.ValidateOrder(order); err != nil {
		return fmt.Errorf("order validation failed: %w", err)
	}

	if order.DeliveryPrice == 0 {
		log.Printf("Order %s has zero delivery price, skipping processing", order.ID)
		return nil
	}

	calculation, err := s.calculateDeliveryWithExchange(ctx, order)
	if err != nil {
		return fmt.Errorf("failed to calculate delivery: %w", err)
	}

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

func (s *OrderProcessingService) calculateDeliveryWithExchange(
	ctx context.Context,
	order *entity.Order,
) (*entity.DeliveryCalculation, error) {
	targetCurrency := "RUB"

	sourceCurrency := order.Currency
	if sourceCurrency == "" {
		sourceCurrency = "USD"
	}

	convertedDelivery, exchangeRate, err := s.exchangeSvc.ConvertCurrency(
		ctx,
		order.DeliveryPrice,
		sourceCurrency,
		targetCurrency,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to convert delivery price from %s to %s: %w", sourceCurrency, targetCurrency, err)
	}

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

func (s *OrderProcessingService) ProcessOrderEvent(ctx context.Context, event *entity.OrderEvent) error {
	switch event.EventType {
	case entity.EventTypeOrderCreated:
		return s.ProcessOrderCreated(ctx, event)
	case entity.EventTypeOrderUpdated:
		log.Printf("Received ORDER_UPDATED event for order %s, skipping processing", event.OrderID)
		return nil
	default:
		log.Printf("Unknown event type: %s for order %s", event.EventType, event.OrderID)
		return nil
	}
}

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
