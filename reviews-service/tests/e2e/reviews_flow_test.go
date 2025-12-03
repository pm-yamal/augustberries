//go:build e2e

package e2e

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"augustberries/reviews-service/internal/app/reviews/entity"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const BaseURL = "http://localhost:8083"

var AuthToken = "test-jwt-token"

func getAuthHeaders() http.Header {
	headers := make(http.Header)
	headers.Set("Content-Type", "application/json")
	headers.Set("Authorization", "Bearer "+AuthToken)
	return headers
}

func TestFullReviewFlow(t *testing.T) {
	client := &http.Client{Timeout: 10 * time.Second}
	productID := "test-product-" + primitive.NewObjectID().Hex()

	// Create
	createReq := entity.CreateReviewRequest{ProductID: productID, Rating: 4, Text: "Good product here."}
	body, _ := json.Marshal(createReq)

	req, _ := http.NewRequest(http.MethodPost, BaseURL+"/reviews", bytes.NewBuffer(body))
	req.Header = getAuthHeaders()

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)

	var created entity.Review
	json.NewDecoder(resp.Body).Decode(&created)
	reviewID := created.ID.Hex()

	defer func() {
		req, _ := http.NewRequest(http.MethodDelete, BaseURL+"/reviews/"+reviewID, nil)
		req.Header = getAuthHeaders()
		resp, _ := client.Do(req)
		if resp != nil {
			resp.Body.Close()
		}
	}()

	// Get
	req, _ = http.NewRequest(http.MethodGet, BaseURL+"/reviews/"+productID, nil)
	resp, err = client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var listResp entity.ReviewListResponse
	json.NewDecoder(resp.Body).Decode(&listResp)
	assert.Equal(t, 1, listResp.Total)

	// Update
	updateReq := entity.UpdateReviewRequest{Rating: 5, Text: "Updated: excellent!"}
	body, _ = json.Marshal(updateReq)

	req, _ = http.NewRequest(http.MethodPatch, BaseURL+"/reviews/"+reviewID, bytes.NewBuffer(body))
	req.Header = getAuthHeaders()

	resp, err = client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestGetNonExistentProductReviews(t *testing.T) {
	client := &http.Client{Timeout: 10 * time.Second}

	req, _ := http.NewRequest(http.MethodGet, BaseURL+"/reviews/nonexistent-product", nil)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var listResp entity.ReviewListResponse
	json.NewDecoder(resp.Body).Decode(&listResp)
	assert.Equal(t, 0, listResp.Total)
}

func TestUpdateNonExistentReview(t *testing.T) {
	client := &http.Client{Timeout: 10 * time.Second}

	updateReq := entity.UpdateReviewRequest{Rating: 5}
	body, _ := json.Marshal(updateReq)

	req, _ := http.NewRequest(http.MethodPatch, BaseURL+"/reviews/"+primitive.NewObjectID().Hex(), bytes.NewBuffer(body))
	req.Header = getAuthHeaders()

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestDeleteNonExistentReview(t *testing.T) {
	client := &http.Client{Timeout: 10 * time.Second}

	req, _ := http.NewRequest(http.MethodDelete, BaseURL+"/reviews/"+primitive.NewObjectID().Hex(), nil)
	req.Header = getAuthHeaders()

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestUnauthorizedAccess(t *testing.T) {
	client := &http.Client{Timeout: 10 * time.Second}

	createReq := entity.CreateReviewRequest{ProductID: "test", Rating: 5, Text: "Test review."}
	body, _ := json.Marshal(createReq)

	req, _ := http.NewRequest(http.MethodPost, BaseURL+"/reviews", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestHealthCheck(t *testing.T) {
	client := &http.Client{Timeout: 10 * time.Second}

	resp, err := client.Get(BaseURL + "/health")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestCreateReview_ValidationErrors тестирует валидацию
func TestCreateReview_ValidationErrors(t *testing.T) {
	client := &http.Client{Timeout: 10 * time.Second}

	testCases := []struct {
		name    string
		request map[string]interface{}
	}{
		{
			name: "Rating too low",
			request: map[string]interface{}{
				"product_id": "test-product",
				"rating":     0,
				"text":       "Достаточно длинный текст отзыва.",
			},
		},
		{
			name: "Rating too high",
			request: map[string]interface{}{
				"product_id": "test-product",
				"rating":     6,
				"text":       "Достаточно длинный текст отзыва.",
			},
		},
		{
			name: "Text too short",
			request: map[string]interface{}{
				"product_id": "test-product",
				"rating":     5,
				"text":       "Short",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			body, _ := json.Marshal(tc.request)

			req, _ := http.NewRequest(http.MethodPost, BaseURL+"/reviews", bytes.NewBuffer(body))
			req.Header = getAuthHeaders()

			resp, err := client.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
		})
	}
}

// TestInvalidObjectID тестирует невалидный MongoDB ObjectID
func TestInvalidObjectID(t *testing.T) {
	client := &http.Client{Timeout: 10 * time.Second}

	invalidIDs := []string{"invalid-id", "123", "not-an-objectid"}

	for _, invalidID := range invalidIDs {
		t.Run("Update_"+invalidID, func(t *testing.T) {
			updateReq := entity.UpdateReviewRequest{Rating: 5}
			body, _ := json.Marshal(updateReq)

			req, _ := http.NewRequest(http.MethodPatch, BaseURL+"/reviews/"+invalidID, bytes.NewBuffer(body))
			req.Header = getAuthHeaders()

			resp, err := client.Do(req)
			require.NoError(t, err)
			resp.Body.Close()

			// MongoDB ObjectID валидация
			assert.True(t, resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusNotFound)
		})
	}
}

// TestMultipleReviewsForProduct тестирует множественные отзывы
func TestMultipleReviewsForProduct(t *testing.T) {
	client := &http.Client{Timeout: 10 * time.Second}

	productID := "multi-review-" + primitive.NewObjectID().Hex()
	var createdIDs []string

	// Создаём несколько отзывов
	for i := 1; i <= 3; i++ {
		createReq := entity.CreateReviewRequest{
			ProductID: productID,
			Rating:    i + 2,
			Text:      "Отзыв номер " + string(rune('0'+i)) + ". Достаточно длинный текст.",
		}
		body, _ := json.Marshal(createReq)

		req, _ := http.NewRequest(http.MethodPost, BaseURL+"/reviews", bytes.NewBuffer(body))
		req.Header = getAuthHeaders()

		resp, err := client.Do(req)
		require.NoError(t, err)

		if resp.StatusCode == http.StatusCreated {
			var review entity.Review
			json.NewDecoder(resp.Body).Decode(&review)
			createdIDs = append(createdIDs, review.ID.Hex())
		}
		resp.Body.Close()
	}

	// Cleanup
	defer func() {
		for _, id := range createdIDs {
			req, _ := http.NewRequest(http.MethodDelete, BaseURL+"/reviews/"+id, nil)
			req.Header = getAuthHeaders()
			resp, _ := client.Do(req)
			if resp != nil {
				resp.Body.Close()
			}
		}
	}()

	// Получаем все отзывы
	resp, err := client.Get(BaseURL + "/reviews/product/" + productID)
	require.NoError(t, err)
	defer resp.Body.Close()

	var listResp entity.ReviewListResponse
	json.NewDecoder(resp.Body).Decode(&listResp)

	assert.Equal(t, 3, listResp.Total)
}

// TestAllRatings тестирует все допустимые значения рейтинга
func TestAllRatings(t *testing.T) {
	client := &http.Client{Timeout: 10 * time.Second}

	for rating := 1; rating <= 5; rating++ {
		t.Run("rating_"+string(rune('0'+rating)), func(t *testing.T) {
			productID := "rating-test-" + primitive.NewObjectID().Hex()

			createReq := entity.CreateReviewRequest{
				ProductID: productID,
				Rating:    rating,
				Text:      "Тестовый отзыв с рейтингом. Длинный текст.",
			}
			body, _ := json.Marshal(createReq)

			req, _ := http.NewRequest(http.MethodPost, BaseURL+"/reviews", bytes.NewBuffer(body))
			req.Header = getAuthHeaders()

			resp, err := client.Do(req)
			require.NoError(t, err)

			assert.Equal(t, http.StatusCreated, resp.StatusCode)

			var review entity.Review
			json.NewDecoder(resp.Body).Decode(&review)
			resp.Body.Close()

			assert.Equal(t, rating, review.Rating)

			// Cleanup
			if review.ID != primitive.NilObjectID {
				req, _ := http.NewRequest(http.MethodDelete, BaseURL+"/reviews/"+review.ID.Hex(), nil)
				req.Header = getAuthHeaders()
				resp, _ := client.Do(req)
				if resp != nil {
					resp.Body.Close()
				}
			}
		})
	}
}
