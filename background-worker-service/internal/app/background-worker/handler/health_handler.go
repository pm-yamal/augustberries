package handler

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"augustberries/background-worker-service/internal/app/background-worker/service"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// HealthCheckHandler управляет healthcheck endpoint'ами
type HealthCheckHandler struct {
	db          *gorm.DB
	redisClient *redis.Client
	exchangeSvc service.ExchangeRateServiceInterface
}

// NewHealthCheckHandler создает новый healthcheck handler
func NewHealthCheckHandler(
	db *gorm.DB,
	redisClient *redis.Client,
	exchangeSvc service.ExchangeRateServiceInterface,
) *HealthCheckHandler {
	return &HealthCheckHandler{
		db:          db,
		redisClient: redisClient,
		exchangeSvc: exchangeSvc,
	}
}

// HealthResponse структура ответа healthcheck
type HealthResponse struct {
	Status    string            `json:"status"`
	Checks    map[string]string `json:"checks"`
	Timestamp time.Time         `json:"timestamp"`
}

// HealthCheck основной healthcheck endpoint
func (h *HealthCheckHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	checks := make(map[string]string)
	overallStatus := "healthy"

	// Проверяем PostgreSQL
	if err := h.checkDatabase(ctx); err != nil {
		checks["database"] = "unhealthy: " + err.Error()
		overallStatus = "unhealthy"
	} else {
		checks["database"] = "healthy"
	}

	// Проверяем Redis
	if err := h.checkRedis(ctx); err != nil {
		checks["redis"] = "unhealthy: " + err.Error()
		overallStatus = "unhealthy"
	} else {
		checks["redis"] = "healthy"
	}

	// Проверяем наличие курсов валют в Redis
	if err := h.checkExchangeRates(ctx); err != nil {
		checks["exchange_rates"] = "warning: " + err.Error()
		// Не делаем статус unhealthy, т.к. это warning
	} else {
		checks["exchange_rates"] = "healthy"
	}

	response := HealthResponse{
		Status:    overallStatus,
		Checks:    checks,
		Timestamp: time.Now(),
	}

	w.Header().Set("Content-Type", "application/json")

	// Устанавливаем HTTP статус код
	if overallStatus != "healthy" {
		w.WriteHeader(http.StatusServiceUnavailable)
	} else {
		w.WriteHeader(http.StatusOK)
	}

	json.NewEncoder(w).Encode(response)
}

// Readiness проверяет готовность сервиса к обработке запросов
func (h *HealthCheckHandler) Readiness(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	// Проверяем что все критические компоненты работают
	if err := h.checkDatabase(ctx); err != nil {
		http.Error(w, "database not ready", http.StatusServiceUnavailable)
		return
	}

	if err := h.checkRedis(ctx); err != nil {
		http.Error(w, "redis not ready", http.StatusServiceUnavailable)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ready"))
}

// Liveness простая проверка что приложение живо
func (h *HealthCheckHandler) Liveness(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("alive"))
}

// checkDatabase проверяет подключение к PostgreSQL
func (h *HealthCheckHandler) checkDatabase(ctx context.Context) error {
	sqlDB, err := h.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.PingContext(ctx)
}

// checkRedis проверяет подключение к Redis
func (h *HealthCheckHandler) checkRedis(ctx context.Context) error {
	return h.redisClient.Ping(ctx).Err()
}

// checkExchangeRates проверяет наличие актуальных курсов валют
func (h *HealthCheckHandler) checkExchangeRates(ctx context.Context) error {
	// Проверяем наличие курса USD как индикатор
	rate, err := h.exchangeSvc.GetRate(ctx, "USD")
	if err != nil {
		return err
	}

	// Проверяем возраст курса
	age := time.Since(rate.UpdatedAt)
	if age > 2*time.Hour {
		log.Printf("WARNING: Exchange rate for USD is outdated (age: %v)", age)
	}

	return nil
}

// RegisterRoutes регистрирует healthcheck маршруты
func (h *HealthCheckHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/health", h.HealthCheck)
	mux.HandleFunc("/health/readiness", h.Readiness)
	mux.HandleFunc("/health/liveness", h.Liveness)
}
