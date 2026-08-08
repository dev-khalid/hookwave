package routers

import (
	"net/http"
	"time"

	"github.com/dev-khalid/hookwave/internal/subscriber/handlers"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func SubscriberRouter() *chi.Mux {
	router := chi.NewRouter()

	router.Get("/long-running-work", func(w http.ResponseWriter, r *http.Request) {
		// Simulate long-running work
		time.Sleep(5 * time.Second)
		w.WriteHeader(200)
		w.Write([]byte(`{"message": "Long running work completed"}`))
	})

	router.Post("/webhook-listener", handlers.WebhookHandler)

	apiRouter := chi.NewRouter()
	apiRouter.Use(middleware.Logger)
	apiRouter.Get("/health", handlers.HealthHandler)

	apiRouter.Mount("/api/v1", router)
	return apiRouter
}
