package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"augustberries/orders-service/internal/app/orders/entity"
	"augustberries/orders-service/internal/app/orders/infrastructure"
	"augustberries/orders-service/internal/app/orders/repository"

	"github.com/google/uuid"
)

var (
	// Ошибки бизнес-логики для обработки в handlers
	ErrOrderNotFound      = errors.New("order not found")
	ErrProductNotFound    = errors.New("product not found in catalog")
	ErrInvalidOrderStatus = errors.New("invalid order status")
	ErrUnauthorized       = errors.New("unauthorized access to order")
)

// OrderService обрабатывает бизнес-логику заказов
// Координирует работу репозиториев, Catalog Service и Kafka
type OrderService struct {
	orderRepo     repository.OrderRepository
	orderItemRepo repository.OrderItemRepository
	catalogClient infrastructure.CatalogServiceClient
	kafkaProducer infrastructure.MessagePublisher
}

// NewOrderService создает новый сервис заказов с внедрением зависимостей
func NewOrderService(
	orderRepo repository.OrderRepository,
	orderItemRepo repository.OrderItemRepository,
	catalogClient infrastructure.CatalogServiceClient,
	kafkaProducer infrastructure.MessagePublisher,
) *OrderService {
	return &OrderService{
		orderRepo:     orderRepo,
		orderItemRepo: orderItemRepo,
		catalogClient: catalogClient,
		kafkaProducer: kafkaProducer,
	}
}

// CreateOrder создает новый заказ
// 1. Проверяет цены товаров в Catalog Service
// 2. Рассчитывает итоговую сумму в рублях (конвертация валют)
// 3. Сохраняет заказ и позиции в БД
// 4. Отправляет событие ORDER_CREATED в Kafka
func (s *OrderService) CreateOrder(ctx context.Context, userID uuid.UUID, req *entity.CreateOrderRequest, authToken string) (*entity.OrderWithItems, error) {
	// Устанавливаем auth токен для запросов к Catalog Service
	s.catalogClient.SetAuthToken(authToken)

	// Собираем список ProductID для проверки в Catalog Service
	productIDs := make([]uuid.UUID, len(req.Items))
	for i, item := range req.Items {
		productIDs[i] = item.ProductID
	}

	// Получаем информацию о товарах из Catalog Service
	products, err := s.catalogClient.GetProducts(ctx, productIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to get products from catalog: %w", err)
	}

	// Проверяем что все товары существуют
	for _, productID := range productIDs {
		if _, exists := products[productID]; !exists {
			return nil, ErrProductNotFound
		}
	}

	// Создаем заказ
	order := &entity.Order{
		ID:            uuid.New(),
		UserID:        userID,
		DeliveryPrice: req.DeliveryPrice,
		Currency:      req.Currency,
		Status:        entity.OrderStatusPending,
		CreatedAt:     time.Now(),
	}

	// Создаем позиции заказа и рассчитываем итоговую сумму
	var totalPrice float64
	orderItems := make([]entity.OrderItem, 0, len(req.Items))

	for _, itemReq := range req.Items {
		product := products[itemReq.ProductID]

		// Цена за единицу берется из Catalog Service (в USD)
		// В реальном проекте здесь нужна конвертация валют
		unitPrice := product.Price

		item := entity.OrderItem{
			ID:        uuid.New(),
			OrderID:   order.ID,
			ProductID: itemReq.ProductID,
			Quantity:  itemReq.Quantity,
			UnitPrice: unitPrice,
		}

		orderItems = append(orderItems, item)
		totalPrice += unitPrice * float64(itemReq.Quantity)
	}

	// Добавляем стоимость доставки
	totalPrice += req.DeliveryPrice

	// Согласно ТЗ, цены товаров в валюте, а итоговая сумма в рублях
	// Здесь должна быть конвертация валют, но для простоты используем коэффициенты
	if req.Currency == "USD" {
		totalPrice = totalPrice * 95 // 1 USD = 95 RUB (примерный курс)
		order.Currency = "RUB"
	} else if req.Currency == "EUR" {
		totalPrice = totalPrice * 105 // 1 EUR = 105 RUB (примерный курс)
		order.Currency = "RUB"
	}
	// Если валюта уже RUB, оставляем как есть

	order.TotalPrice = totalPrice

	// Сохраняем заказ в БД
	if err := s.orderRepo.Create(ctx, order); err != nil {
		return nil, fmt.Errorf("failed to create order: %w", err)
	}

	// Сохраняем позиции заказа
	for _, item := range orderItems {
		if err := s.orderItemRepo.Create(ctx, &item); err != nil {
			return nil, fmt.Errorf("failed to create order item: %w", err)
		}
	}

	// Отправляем событие ORDER_CREATED в Kafka
	event := entity.OrderEvent{
		EventType:  "ORDER_CREATED",
		OrderID:    order.ID,
		UserID:     order.UserID,
		TotalPrice: order.TotalPrice,
		Currency:   order.Currency,
		Status:     order.Status,
		ItemsCount: len(orderItems),
		Timestamp:  time.Now(),
	}

	if err := s.publishOrderEvent(ctx, event); err != nil {
		// Логируем ошибку, но не прерываем выполнение
		// Заказ уже создан, проблемы с Kafka не критичны
		fmt.Printf("failed to publish order created event: %v\n", err)
	}

	return &entity.OrderWithItems{
		Order: *order,
		Items: orderItems,
	}, nil
}

// GetOrder получает заказ по ID с проверкой доступа
func (s *OrderService) GetOrder(ctx context.Context, orderID uuid.UUID, userID uuid.UUID) (*entity.OrderWithItems, error) {
	order, err := s.orderRepo.GetWithItems(ctx, orderID)
	if err != nil {
		if errors.Is(err, repository.ErrOrderNotFound) {
			return nil, ErrOrderNotFound
		}
		return nil, fmt.Errorf("failed to get order: %w", err)
	}

	// Проверяем что пользователь имеет доступ к заказу
	if order.UserID != userID {
		return nil, ErrUnauthorized
	}

	return order, nil
}

// UpdateOrderStatus обновляет статус заказа и отправляет событие ORDER_UPDATED в Kafka
func (s *OrderService) UpdateOrderStatus(ctx context.Context, orderID uuid.UUID, userID uuid.UUID, newStatus entity.OrderStatus) (*entity.Order, error) {
	// Получаем заказ
	order, err := s.orderRepo.GetByID(ctx, orderID)
	if err != nil {
		if errors.Is(err, repository.ErrOrderNotFound) {
			return nil, ErrOrderNotFound
		}
		return nil, fmt.Errorf("failed to get order: %w", err)
	}

	// Проверяем доступ
	if order.UserID != userID {
		return nil, ErrUnauthorized
	}

	// Проверяем допустимость смены статуса
	if !isValidStatusTransition(order.Status, newStatus) {
		return nil, ErrInvalidOrderStatus
	}

	// Обновляем статус
	order.Status = newStatus

	if err := s.orderRepo.Update(ctx, order); err != nil {
		return nil, fmt.Errorf("failed to update order: %w", err)
	}

	// Отправляем событие ORDER_UPDATED в Kafka
	items, _ := s.orderItemRepo.GetByOrderID(ctx, orderID)
	event := entity.OrderEvent{
		EventType:  "ORDER_UPDATED",
		OrderID:    order.ID,
		UserID:     order.UserID,
		TotalPrice: order.TotalPrice,
		Currency:   order.Currency,
		Status:     order.Status,
		ItemsCount: len(items),
		Timestamp:  time.Now(),
	}

	if err := s.publishOrderEvent(ctx, event); err != nil {
		fmt.Printf("failed to publish order updated event: %v\n", err)
	}

	return order, nil
}

// DeleteOrder удаляет заказ с проверкой доступа
func (s *OrderService) DeleteOrder(ctx context.Context, orderID uuid.UUID, userID uuid.UUID) error {
	// Получаем заказ для проверки доступа
	order, err := s.orderRepo.GetByID(ctx, orderID)
	if err != nil {
		if errors.Is(err, repository.ErrOrderNotFound) {
			return ErrOrderNotFound
		}
		return fmt.Errorf("failed to get order: %w", err)
	}

	// Проверяем доступ
	if order.UserID != userID {
		return ErrUnauthorized
	}

	// Удаляем заказ (позиции удаляются автоматически через CASCADE)
	if err := s.orderRepo.Delete(ctx, orderID); err != nil {
		return fmt.Errorf("failed to delete order: %w", err)
	}

	return nil
}

// GetUserOrders получает все заказы пользователя
func (s *OrderService) GetUserOrders(ctx context.Context, userID uuid.UUID) ([]entity.Order, error) {
	orders, err := s.orderRepo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user orders: %w", err)
	}

	return orders, nil
}

// publishOrderEvent отправляет событие о заказе в Kafka
func (s *OrderService) publishOrderEvent(ctx context.Context, event entity.OrderEvent) error {
	// Сериализуем событие в JSON
	eventData, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal order event: %w", err)
	}

	// Отправляем в Kafka с ключом = OrderID для партиционирования
	if err := s.kafkaProducer.PublishMessage(ctx, event.OrderID.String(), eventData); err != nil {
		return fmt.Errorf("failed to publish to kafka: %w", err)
	}

	return nil
}

// isValidStatusTransition проверяет допустимость смены статуса заказа
func isValidStatusTransition(from, to entity.OrderStatus) bool {
	// Определяем допустимые переходы статусов
	validTransitions := map[entity.OrderStatus][]entity.OrderStatus{
		entity.OrderStatusPending: {
			entity.OrderStatusConfirmed,
			entity.OrderStatusCancelled,
		},
		entity.OrderStatusConfirmed: {
			entity.OrderStatusShipped,
			entity.OrderStatusCancelled,
		},
		entity.OrderStatusShipped: {
			entity.OrderStatusDelivered,
		},
		entity.OrderStatusDelivered: {}, // Финальный статус
		entity.OrderStatusCancelled: {}, // Финальный статус
	}

	allowedStatuses, exists := validTransitions[from]
	if !exists {
		return false
	}

	for _, status := range allowedStatuses {
		if status == to {
			return true
		}
	}

	return false
}
