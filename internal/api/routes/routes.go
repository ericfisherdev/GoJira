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
			r.Post("/oauth2/start", handlers.OAuth2Start)
			r.Get("/oauth2/callback", handlers.OAuth2Callback)
		})

		// Issue routes
		r.Route("/issues", func(r chi.Router) {
			r.Post("/", handlers.CreateIssue)
			r.Get("/{key}", handlers.GetIssue)
			r.Put("/{key}", handlers.UpdateIssue)
			r.Delete("/{key}", handlers.DeleteIssue)
			
			// Issue operations
			r.Get("/{key}/transitions", handlers.GetIssueTransitions)
			r.Post("/{key}/transitions", handlers.TransitionIssue)
			r.Get("/{key}/links", handlers.GetIssueLinks)
			r.Get("/{key}/customfields", handlers.GetCustomFields)
			
			// Issue linking
			r.Post("/link", handlers.CreateIssueLink)
			r.Delete("/link/{id}", handlers.DeleteIssueLink)
			r.Get("/linktypes", handlers.GetLinkTypes)
		})

		// Search routes
		r.Route("/search", func(r chi.Router) {
			r.Get("/", handlers.SearchIssues)
			r.Post("/", handlers.SearchIssues)
		})
	})
}