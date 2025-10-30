package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

// SetupRoutes настраивает все маршруты Catalog Service
func SetupRoutes(catalogHandler *CatalogHandler) *chi.Mux {
	router := chi.NewRouter()

	// Глобальные middleware
	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(middleware.Logger)
	router.Use(middleware.Recoverer)
	router.Use(middleware.Compress(5))

	// CORS настройки
	router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"https://*", "http://*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Health check endpoint
	router.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok","service":"catalog-service"}`))
	})

	// Products endpoints
	router.Route("/products", func(r chi.Router) {
		r.Get("/", catalogHandler.GetAllProducts)       // Список всех товаров
		r.Post("/", catalogHandler.CreateProduct)       // Создать товар
		r.Get("/{id}", catalogHandler.GetProduct)       // Товар по ID
		r.Put("/{id}", catalogHandler.UpdateProduct)    // Обновить товар (отправляет в Kafka при изменении цены)
		r.Delete("/{id}", catalogHandler.DeleteProduct) // Удалить товар
	})

	// Categories endpoints
	router.Route("/categories", func(r chi.Router) {
		r.Get("/", catalogHandler.GetAllCategories)      // Список категорий (кеш Redis)
		r.Post("/", catalogHandler.CreateCategory)       // Создать категорию
		r.Get("/{id}", catalogHandler.GetCategory)       // Категория по ID
		r.Put("/{id}", catalogHandler.UpdateCategory)    // Обновить категорию
		r.Delete("/{id}", catalogHandler.DeleteCategory) // Удалить категорию
	})

	return router
}
