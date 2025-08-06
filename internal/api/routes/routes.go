package routes

import (
	"github.com/ericfisherdev/GoJira/internal/api/handlers"
	"github.com/go-chi/chi/v5"
)

func SetupRoutes(r *chi.Mux) {
	// Health check routes
	r.Get("/health", handlers.HealthCheck)
	r.Get("/ready", handlers.ReadinessCheck)

	// API v1 routes
	r.Route("/api/v1", func(r chi.Router) {
		// Authentication routes
		r.Route("/auth", func(r chi.Router) {
			r.Post("/connect", handlers.Connect)
			r.Post("/disconnect", handlers.Disconnect)
			r.Get("/status", handlers.Status)
		})

		// Issue routes
		r.Route("/issues", func(r chi.Router) {
			r.Post("/", handlers.CreateIssue)
			r.Get("/{key}", handlers.GetIssue)
			r.Put("/{key}", handlers.UpdateIssue)
			r.Delete("/{key}", handlers.DeleteIssue)
		})
	})
}