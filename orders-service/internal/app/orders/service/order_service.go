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
	"augustberries/pkg/metrics"

	"github.com/google/uuid"
)

var (
	ErrOrderNotFound      = errors.New("order not found")
	ErrProductNotFound    = errors.New("product not found")
	ErrInvalidOrderStatus = errors.New("invalid order status")
	ErrUnauthorized       = errors.New("unauthorized access to order")
)

type OrderService struct {
	orderRepo     repository.OrderRepository
	orderItemRepo repository.OrderItemRepository
	catalogClient infrastructure.CatalogServiceClient
	kafkaProducer infrastructure.MessagePublisher
}

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

func (s *OrderService) CreateOrder(ctx context.Context, userID uuid.UUID, req *entity.CreateOrderRequest, authToken string) (*entity.OrderWithItems, error) {
	s.catalogClient.SetAuthToken(authToken)

	productIDs := make([]uuid.UUID, len(req.Items))
	for i, item := range req.Items {
		productIDs[i] = item.ProductID
	}

	products, err := s.catalogClient.GetProducts(ctx, productIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to get products from catalog: %w", err)
	}

	for _, productID := range productIDs {
		if _, exists := products[productID]; !exists {
			return nil, ErrProductNotFound
		}
	}

	order := &entity.Order{
		ID:            uuid.New(),
		UserID:        userID,
		DeliveryPrice: req.DeliveryPrice,
		Currency:      req.Currency,
		Status:        entity.OrderStatusPending,
		CreatedAt:     time.Now(),
	}

	var totalPrice float64
	orderItems := make([]entity.OrderItem, 0, len(req.Items))

	for _, itemReq := range req.Items {
		product := products[itemReq.ProductID]
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

	totalPrice += req.DeliveryPrice
	order.TotalPrice = totalPrice

	if err := s.orderRepo.Create(ctx, order); err != nil {
		return nil, fmt.Errorf("failed to create order: %w", err)
	}

	for _, item := range orderItems {
		if err := s.orderItemRepo.Create(ctx, &item); err != nil {
			return nil, fmt.Errorf("failed to create order item: %w", err)
		}
	}

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
		fmt.Printf("failed to publish order created event: %v\n", err)
	}

	metrics.OrdersCreated.Inc()
	metrics.OrdersTotal.Add(order.TotalPrice)
	metrics.OrdersByStatus.WithLabelValues(string(order.Status)).Inc()

	return &entity.OrderWithItems{
		Order: *order,
		Items: orderItems,
	}, nil
}

func (s *OrderService) GetOrder(ctx context.Context, orderID uuid.UUID, userID uuid.UUID) (*entity.OrderWithItems, error) {
	order, err := s.orderRepo.GetWithItems(ctx, orderID)
	if err != nil {
		if errors.Is(err, repository.ErrOrderNotFound) {
			return nil, ErrOrderNotFound
		}
		return nil, fmt.Errorf("failed to get order: %w", err)
	}

	if order.UserID != userID {
		return nil, ErrUnauthorized
	}

	return order, nil
}

func (s *OrderService) UpdateOrderStatus(ctx context.Context, orderID uuid.UUID, userID uuid.UUID, newStatus entity.OrderStatus) (*entity.Order, error) {
	order, err := s.orderRepo.GetByID(ctx, orderID)
	if err != nil {
		if errors.Is(err, repository.ErrOrderNotFound) {
			return nil, ErrOrderNotFound
		}
		return nil, fmt.Errorf("failed to get order: %w", err)
	}

	if order.UserID != userID {
		return nil, ErrUnauthorized
	}

	if !isValidStatusTransition(order.Status, newStatus) {
		return nil, ErrInvalidOrderStatus
	}

	order.Status = newStatus

	if err := s.orderRepo.Update(ctx, order); err != nil {
		return nil, fmt.Errorf("failed to update order: %w", err)
	}

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

	metrics.OrdersByStatus.WithLabelValues(string(order.Status)).Inc()

	return order, nil
}

func (s *OrderService) DeleteOrder(ctx context.Context, orderID uuid.UUID, userID uuid.UUID) error {
	order, err := s.orderRepo.GetByID(ctx, orderID)
	if err != nil {
		if errors.Is(err, repository.ErrOrderNotFound) {
			return ErrOrderNotFound
		}
		return fmt.Errorf("failed to get order: %w", err)
	}

	if order.UserID != userID {
		return ErrUnauthorized
	}

	if err := s.orderRepo.Delete(ctx, orderID); err != nil {
		return fmt.Errorf("failed to delete order: %w", err)
	}

	return nil
}

func (s *OrderService) GetUserOrders(ctx context.Context, userID uuid.UUID) ([]entity.Order, error) {
	orders, err := s.orderRepo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user orders: %w", err)
	}
	return orders, nil
}

func (s *OrderService) publishOrderEvent(ctx context.Context, event entity.OrderEvent) error {
	eventData, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal order event: %w", err)
	}

	if err := s.kafkaProducer.PublishMessage(ctx, event.OrderID.String(), eventData); err != nil {
		return fmt.Errorf("failed to publish to kafka: %w", err)
	}

	return nil
}

func isValidStatusTransition(from, to entity.OrderStatus) bool {
	validTransitions := map[entity.OrderStatus][]entity.OrderStatus{
		entity.OrderStatusPending:   {entity.OrderStatusConfirmed, entity.OrderStatusCancelled},
		entity.OrderStatusConfirmed: {entity.OrderStatusShipped, entity.OrderStatusCancelled},
		entity.OrderStatusShipped:   {entity.OrderStatusDelivered},
		entity.OrderStatusDelivered: {},
		entity.OrderStatusCancelled: {},
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
