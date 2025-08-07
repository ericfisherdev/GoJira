package routes

import (
	"github.com/ericfisherdev/GoJira/internal/api/handlers"
	"github.com/go-chi/chi/v5"
)

func SetupRoutes(r *chi.Mux) {
	// Health check routes
	r.Get("/health", handlers.HealthCheck)
	r.Get("/ready", handlers.ReadinessCheck)
	
	// Metrics and monitoring routes
	r.Get("/metrics", handlers.GetMetrics)
	r.Post("/metrics/reset", handlers.ResetMetrics)
	r.Get("/health/detailed", handlers.GetHealthWithMetrics)

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
			r.Post("/advanced", handlers.AdvancedSearchIssues)
			r.Post("/paginated", handlers.SearchWithPaginationHandler)
			r.Post("/export", handlers.ExportSearchResults)
			r.Post("/page", handlers.GetSearchPage)
			r.Post("/all-pages", handlers.GetAllSearchPages)
			r.Get("/validate", handlers.ValidateJQL)
			r.Get("/suggestions", handlers.GetJQLSuggestions)
			r.Get("/fields", handlers.GetJQLFields)
			r.Get("/functions", handlers.GetJQLFunctions)
		})

		// Filter routes
		r.Route("/filters", func(r chi.Router) {
			r.Get("/", handlers.GetAllFilters)
			r.Get("/{id}", handlers.GetFilter)
			r.Get("/{id}/search", handlers.SearchWithFilter)
		})

		// Sprint routes
		r.Route("/sprints", func(r chi.Router) {
			r.Get("/", handlers.GetSprints)
			r.Post("/", handlers.CreateSprint)
			r.Get("/active", handlers.GetActiveSprints)
			r.Get("/upcoming", handlers.GetUpcomingSprints)
			r.Get("/health", handlers.GetSprintHealthCheck)
			r.Post("/validate", handlers.ValidateSprintRequest)
			r.Get("/{id}", handlers.GetSprint)
			r.Put("/{id}", handlers.UpdateSprint)
			r.Post("/{id}/start", handlers.StartSprint)
			r.Post("/{id}/auto-start", handlers.AutoStartSprint)
			r.Post("/{id}/close", handlers.CloseSprint)
			r.Post("/{id}/complete", handlers.CompleteSprintWithReport)
			r.Get("/{id}/issues", handlers.GetSprintIssues)
			r.Post("/{id}/issues", handlers.MoveIssuesToSprint)
			r.Get("/{id}/report", handlers.GetSprintReport)
			r.Get("/{id}/metrics", handlers.GetSprintMetrics)
			r.Get("/{id}/predict", handlers.PredictSprintSuccess)
			r.Post("/{id}/clone", handlers.CloneSprint)
		})

		// Board routes
		r.Route("/boards", func(r chi.Router) {
			r.Get("/", handlers.GetBoards)
			r.Get("/{id}", handlers.GetBoard)
			r.Get("/{id}/configuration", handlers.GetBoardConfiguration)
			r.Get("/{id}/issues", handlers.GetBoardIssues)
			r.Get("/{id}/backlog", handlers.GetBoardBacklog)
			r.Get("/{id}/sprints", handlers.GetBoardSprints)
		})

		// Workflow routes
		r.Route("/workflows", func(r chi.Router) {
			r.Get("/", handlers.GetWorkflows)
			r.Get("/{name}", handlers.GetWorkflow)
			r.Get("/{name}/statemachine", handlers.GetWorkflowStateMachine)
			r.Get("/{name}/analytics", handlers.GetWorkflowAnalytics)
			
			// Workflow scheme operations
			r.Get("/schemes", handlers.GetWorkflowSchemes)
			r.Get("/schemes/project/{projectKey}", handlers.GetProjectWorkflowScheme)
		})

		// Issue workflow operations
		r.Route("/issues/{issueKey}/workflow", func(r chi.Router) {
			r.Get("/", handlers.GetIssueWorkflow)
			r.Get("/transitions", handlers.GetAvailableTransitions)
			r.Get("/transitions/{transitionId}/validate", handlers.ValidateTransition)
			r.Post("/transitions/{transitionId}/execute", handlers.ExecuteTransition)
		})

		// Claude-optimized routes
		r.Route("/claude", func(r chi.Router) {
			// Claude-formatted issue operations
			r.Get("/issues/{key}", handlers.ClaudeGetIssue)
			r.Post("/issues", handlers.ClaudeCreateIssue)
			r.Post("/search", handlers.ClaudeSearchIssues)
			
			// Natural language processing
			r.Post("/command", handlers.ProcessNaturalLanguageCommand)
			r.Post("/jql", handlers.GenerateJQLFromNaturalLanguage)
			r.Get("/suggestions", handlers.GetCommandSuggestions)
		})
	})
}