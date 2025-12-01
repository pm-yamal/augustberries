//go:build e2e

package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"augustberries/orders-service/internal/app/orders/entity"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	// BaseURL - адрес запущенного orders-service
	BaseURL = "http://localhost:8082"
)

// AuthToken - тестовый JWT токен
var AuthToken = "test-jwt-token"

// getAuthHeaders возвращает заголовки с авторизацией
func getAuthHeaders() http.Header {
	headers := make(http.Header)
	headers.Set("Content-Type", "application/json")
	headers.Set("Authorization", "Bearer "+AuthToken)
	return headers
}

// TestFullOrderFlow тестирует полный цикл заказа:
// 1. Создание заказа
// 2. Получение заказа
// 3. Обновление статуса (pending -> confirmed -> shipped -> delivered)
// 4. Получение списка заказов
// 5. Удаление заказа
func TestFullOrderFlow(t *testing.T) {
	client := &http.Client{Timeout: 10 * time.Second}

	// Используем фиксированный productID который mock catalog service знает
	productID := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	// ==================== Step 1: Create Order ====================
	t.Log("Step 1: Creating order")

	createReq := entity.CreateOrderRequest{
		Items: []entity.OrderItemRequest{
			{ProductID: productID, Quantity: 2},
		},
		DeliveryPrice: 15.0,
		Currency:      "USD",
	}
	body, _ := json.Marshal(createReq)

	req, _ := http.NewRequest(http.MethodPost, BaseURL+"/orders", bytes.NewBuffer(body))
	req.Header = getAuthHeaders()

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode, "Order creation should succeed")

	var orderResp entity.OrderResponse
	err = json.NewDecoder(resp.Body).Decode(&orderResp)
	require.NoError(t, err)

	assert.NotEqual(t, uuid.Nil, orderResp.ID)
	assert.Equal(t, entity.OrderStatusPending, orderResp.Status)
	assert.Equal(t, "USD", orderResp.Currency)

	orderID := orderResp.ID
	t.Logf("Created order: %s", orderID)

	// Cleanup: удаляем заказ после теста
	defer func() {
		req, _ := http.NewRequest(http.MethodDelete, BaseURL+"/orders/"+orderID.String(), nil)
		req.Header = getAuthHeaders()
		resp, err := client.Do(req)
		if err == nil {
			resp.Body.Close()
		}
	}()

	// ==================== Step 2: Get Order ====================
	t.Log("Step 2: Getting order")

	req, _ = http.NewRequest(http.MethodGet, BaseURL+"/orders/"+orderID.String(), nil)
	req.Header = getAuthHeaders()

	resp, err = client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var getResp entity.OrderResponse
	err = json.NewDecoder(resp.Body).Decode(&getResp)
	require.NoError(t, err)

	assert.Equal(t, orderID, getResp.ID)
	t.Logf("Order status: %s", getResp.Status)

	// ==================== Step 3: Update Status Flow ====================
	statusFlow := []entity.OrderStatus{
		entity.OrderStatusConfirmed,
		entity.OrderStatusShipped,
		entity.OrderStatusDelivered,
	}

	for _, newStatus := range statusFlow {
		t.Logf("Step 3: Updating status to %s", newStatus)

		updateReq := entity.UpdateOrderStatusRequest{Status: newStatus}
		body, _ = json.Marshal(updateReq)

		req, _ = http.NewRequest(http.MethodPatch, BaseURL+"/orders/"+orderID.String(), bytes.NewBuffer(body))
		req.Header = getAuthHeaders()

		resp, err = client.Do(req)
		require.NoError(t, err)
		resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode, "Status update to %s should succeed", newStatus)
	}

	// Проверяем финальный статус
	req, _ = http.NewRequest(http.MethodGet, BaseURL+"/orders/"+orderID.String(), nil)
	req.Header = getAuthHeaders()

	resp, err = client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	var finalResp entity.OrderResponse
	json.NewDecoder(resp.Body).Decode(&finalResp)

	assert.Equal(t, entity.OrderStatusDelivered, finalResp.Status)
	t.Log("Order delivered successfully!")

	// ==================== Step 4: Get User Orders ====================
	t.Log("Step 4: Getting user orders")

	req, _ = http.NewRequest(http.MethodGet, BaseURL+"/orders", nil)
	req.Header = getAuthHeaders()

	resp, err = client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var ordersResp map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&ordersResp)

	assert.GreaterOrEqual(t, ordersResp["total"].(float64), float64(1))
	t.Logf("Total orders: %.0f", ordersResp["total"])

	t.Log("Full order flow completed!")
}

// TestInvalidStatusTransition тестирует невалидные переходы статусов
func TestInvalidStatusTransition(t *testing.T) {
	client := &http.Client{Timeout: 10 * time.Second}

	productID := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	// Создаём заказ
	createReq := entity.CreateOrderRequest{
		Items: []entity.OrderItemRequest{
			{ProductID: productID, Quantity: 1},
		},
		DeliveryPrice: 10.0,
		Currency:      "USD",
	}
	body, _ := json.Marshal(createReq)

	req, _ := http.NewRequest(http.MethodPost, BaseURL+"/orders", bytes.NewBuffer(body))
	req.Header = getAuthHeaders()

	resp, err := client.Do(req)
	require.NoError(t, err)

	var orderResp entity.OrderResponse
	json.NewDecoder(resp.Body).Decode(&orderResp)
	resp.Body.Close()

	orderID := orderResp.ID

	// Cleanup
	defer func() {
		req, _ := http.NewRequest(http.MethodDelete, BaseURL+"/orders/"+orderID.String(), nil)
		req.Header = getAuthHeaders()
		resp, _ := client.Do(req)
		if resp != nil {
			resp.Body.Close()
		}
	}()

	// Пытаемся перейти напрямую в shipped (минуя confirmed)
	updateReq := entity.UpdateOrderStatusRequest{Status: entity.OrderStatusShipped}
	body, _ = json.Marshal(updateReq)

	req, _ = http.NewRequest(http.MethodPatch, BaseURL+"/orders/"+orderID.String(), bytes.NewBuffer(body))
	req.Header = getAuthHeaders()

	resp, err = client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "Invalid status transition should fail")
}

// TestGetNonExistentOrder тестирует получение несуществующего заказа
func TestGetNonExistentOrder(t *testing.T) {
	client := &http.Client{Timeout: 10 * time.Second}

	nonExistentID := uuid.New()

	req, _ := http.NewRequest(http.MethodGet, BaseURL+"/orders/"+nonExistentID.String(), nil)
	req.Header = getAuthHeaders()

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// TestInvalidUUID тестирует обработку невалидного UUID
func TestInvalidUUID(t *testing.T) {
	client := &http.Client{Timeout: 10 * time.Second}

	invalidIDs := []string{"invalid-uuid", "123", "not-a-uuid"}

	for _, invalidID := range invalidIDs {
		t.Run(fmt.Sprintf("ID=%s", invalidID), func(t *testing.T) {
			req, _ := http.NewRequest(http.MethodGet, BaseURL+"/orders/"+invalidID, nil)
			req.Header = getAuthHeaders()

			resp, err := client.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
		})
	}
}

// TestUnauthorizedAccess тестирует доступ без авторизации
func TestUnauthorizedAccess(t *testing.T) {
	client := &http.Client{Timeout: 10 * time.Second}

	endpoints := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/orders"},
		{http.MethodPost, "/orders"},
		{http.MethodGet, "/orders/" + uuid.New().String()},
	}

	for _, ep := range endpoints {
		t.Run(fmt.Sprintf("%s %s", ep.method, ep.path), func(t *testing.T) {
			req, _ := http.NewRequest(ep.method, BaseURL+ep.path, nil)
			// НЕ устанавливаем Authorization header

			resp, err := client.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		})
	}
}

// TestHealthCheck проверяет endpoint /health
func TestHealthCheck(t *testing.T) {
	client := &http.Client{Timeout: 10 * time.Second}

	resp, err := client.Get(BaseURL + "/health")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestCancelOrder тестирует отмену заказа
func TestCancelOrder(t *testing.T) {
	client := &http.Client{Timeout: 10 * time.Second}

	productID := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	// Создаём заказ
	createReq := entity.CreateOrderRequest{
		Items: []entity.OrderItemRequest{
			{ProductID: productID, Quantity: 1},
		},
		DeliveryPrice: 5.0,
		Currency:      "RUB",
	}
	body, _ := json.Marshal(createReq)

	req, _ := http.NewRequest(http.MethodPost, BaseURL+"/orders", bytes.NewBuffer(body))
	req.Header = getAuthHeaders()

	resp, err := client.Do(req)
	require.NoError(t, err)

	var orderResp entity.OrderResponse
	json.NewDecoder(resp.Body).Decode(&orderResp)
	resp.Body.Close()

	orderID := orderResp.ID

	// Cleanup
	defer func() {
		req, _ := http.NewRequest(http.MethodDelete, BaseURL+"/orders/"+orderID.String(), nil)
		req.Header = getAuthHeaders()
		resp, _ := client.Do(req)
		if resp != nil {
			resp.Body.Close()
		}
	}()

	// Отменяем заказ (pending -> cancelled)
	updateReq := entity.UpdateOrderStatusRequest{Status: entity.OrderStatusCancelled}
	body, _ = json.Marshal(updateReq)

	req, _ = http.NewRequest(http.MethodPatch, BaseURL+"/orders/"+orderID.String(), bytes.NewBuffer(body))
	req.Header = getAuthHeaders()

	resp, err = client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Проверяем статус
	req, _ = http.NewRequest(http.MethodGet, BaseURL+"/orders/"+orderID.String(), nil)
	req.Header = getAuthHeaders()

	resp, err = client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	var finalResp entity.OrderResponse
	json.NewDecoder(resp.Body).Decode(&finalResp)

	assert.Equal(t, entity.OrderStatusCancelled, finalResp.Status)
}

// TestConcurrentOrderCreation тестирует параллельное создание заказов
func TestConcurrentOrderCreation(t *testing.T) {
	client := &http.Client{Timeout: 30 * time.Second}

	productID := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	done := make(chan uuid.UUID, 5)

	for i := 0; i < 5; i++ {
		go func(idx int) {
			createReq := entity.CreateOrderRequest{
				Items: []entity.OrderItemRequest{
					{ProductID: productID, Quantity: idx + 1},
				},
				DeliveryPrice: float64(idx * 5),
				Currency:      "USD",
			}
			body, _ := json.Marshal(createReq)

			req, _ := http.NewRequest(http.MethodPost, BaseURL+"/orders", bytes.NewBuffer(body))
			req.Header = getAuthHeaders()

			resp, err := client.Do(req)
			if err != nil || resp.StatusCode != http.StatusCreated {
				done <- uuid.Nil
				if resp != nil {
					resp.Body.Close()
				}
				return
			}

			var orderResp entity.OrderResponse
			json.NewDecoder(resp.Body).Decode(&orderResp)
			resp.Body.Close()

			done <- orderResp.ID
		}(i)
	}

	// Собираем результаты и cleanup
	var createdIDs []uuid.UUID
	for i := 0; i < 5; i++ {
		id := <-done
		if id != uuid.Nil {
			createdIDs = append(createdIDs, id)
		}
	}

	// Cleanup
	for _, id := range createdIDs {
		req, _ := http.NewRequest(http.MethodDelete, BaseURL+"/orders/"+id.String(), nil)
		req.Header = getAuthHeaders()
		resp, _ := client.Do(req)
		if resp != nil {
			resp.Body.Close()
		}
	}

	assert.Equal(t, 5, len(createdIDs), "All concurrent order creations should succeed")
}
