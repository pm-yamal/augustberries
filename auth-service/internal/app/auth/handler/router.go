package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

// SetupRoutes настраивает все маршруты приложения с использованием chi router
func SetupRoutes(authHandler *AuthHandler, authMiddleware *AuthMiddleware) *chi.Mux {
	router := chi.NewRouter()

	// Глобальные middleware от chi
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
		w.Write([]byte(`{"status":"ok","service":"auth-service"}`))
	})

	// Публичные эндпоинты (без аутентификации)
	router.Route("/auth", func(r chi.Router) {
		r.Post("/register", authHandler.Register)
		r.Post("/login", authHandler.Login)
		r.Post("/refresh", authHandler.RefreshToken)
		r.Post("/validate", authHandler.ValidateToken)

		// Защищенные эндпоинты (требуют аутентификации)
		r.Group(func(r chi.Router) {
			r.Use(authMiddleware.Authenticate)
			r.Get("/me", authHandler.GetMe)
			r.Post("/logout", authHandler.Logout)
		})
	})

	// Пример использования middleware для проверки ролей
	router.Route("/admin", func(r chi.Router) {
		r.Use(authMiddleware.Authenticate)
		r.Use(authMiddleware.RequireRole("admin"))

		r.Get("/users", func(w http.ResponseWriter, r *http.Request) {
			respondJSON(w, http.StatusOK, map[string]string{
				"message": "Admin only endpoint - list users",
			})
		})
	})

	// Пример использования проверки разрешений
	router.Route("/api/products", func(r chi.Router) {
		r.Use(authMiddleware.Authenticate)

		// Любой авторизованный пользователь может читать
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			respondJSON(w, http.StatusOK, map[string]string{
				"message": "List products",
			})
		})

		// Только с разрешением product.create
		r.With(authMiddleware.RequirePermission("product.create")).Post("/", func(w http.ResponseWriter, r *http.Request) {
			respondJSON(w, http.StatusCreated, map[string]string{
				"message": "Product created",
			})
		})
	})

	return router
}
