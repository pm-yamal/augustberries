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

type HealthCheckHandler struct {
	db          *gorm.DB
	redisClient *redis.Client
	exchangeSvc service.ExchangeRateServiceInterface
}

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

type HealthResponse struct {
	Status    string            `json:"status"`
	Checks    map[string]string `json:"checks"`
	Timestamp time.Time         `json:"timestamp"`
}

func (h *HealthCheckHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	checks := make(map[string]string)
	overallStatus := "healthy"

	if err := h.checkDatabase(ctx); err != nil {
		checks["database"] = "unhealthy: " + err.Error()
		overallStatus = "unhealthy"
	} else {
		checks["database"] = "healthy"
	}

	if err := h.checkRedis(ctx); err != nil {
		checks["redis"] = "unhealthy: " + err.Error()
		overallStatus = "unhealthy"
	} else {
		checks["redis"] = "healthy"
	}

	if err := h.checkExchangeRates(ctx); err != nil {
		checks["exchange_rates"] = "warning: " + err.Error()
	} else {
		checks["exchange_rates"] = "healthy"
	}

	response := HealthResponse{
		Status:    overallStatus,
		Checks:    checks,
		Timestamp: time.Now(),
	}

	w.Header().Set("Content-Type", "application/json")

	if overallStatus != "healthy" {
		w.WriteHeader(http.StatusServiceUnavailable)
	} else {
		w.WriteHeader(http.StatusOK)
	}

	json.NewEncoder(w).Encode(response)
}

func (h *HealthCheckHandler) Readiness(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

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

func (h *HealthCheckHandler) Liveness(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("alive"))
}

func (h *HealthCheckHandler) checkDatabase(ctx context.Context) error {
	sqlDB, err := h.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.PingContext(ctx)
}

func (h *HealthCheckHandler) checkRedis(ctx context.Context) error {
	return h.redisClient.Ping(ctx).Err()
}

func (h *HealthCheckHandler) checkExchangeRates(ctx context.Context) error {
	rate, err := h.exchangeSvc.GetRate(ctx, "USD")
	if err != nil {
		return err
	}

	age := time.Since(rate.UpdatedAt)
	if age > 2*time.Hour {
		log.Printf("WARNING: Exchange rate for USD is outdated (age: %v)", age)
	}

	return nil
}

func (h *HealthCheckHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/health", h.HealthCheck)
	mux.HandleFunc("/health/readiness", h.Readiness)
	mux.HandleFunc("/health/liveness", h.Liveness)
}
