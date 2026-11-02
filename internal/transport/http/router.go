package http

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

// Deps holds the dependencies required to set up routes.
type Deps struct {
	Logger    *slog.Logger
	JWTSecret string
}

// NewRouter creates a chi.Mux with all middleware and route groups mounted.
func NewRouter(deps Deps) *chi.Mux {
	r := chi.NewRouter()

	// Core middleware stack
	r.Use(RequestID)
	r.Use(chimw.RealIP)
	r.Use(StructuredLogger(deps.Logger))
	r.Use(chimw.Recoverer)

	// CORS configured for development
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:*", "http://127.0.0.1:*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Request-ID"},
		ExposedHeaders:   []string{"X-Request-ID"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Health check — unauthenticated
	r.Get("/health", healthHandler())

	// API v1 routes
	r.Route("/api/v1", func(api chi.Router) {
		// Auth routes — unauthenticated
		api.Route("/auth", func(auth chi.Router) {
			auth.HandleFunc("/*", placeholderHandler("auth"))
		})

		// Project routes — authenticated
		api.Route("/projects", func(projects chi.Router) {
			projects.HandleFunc("/*", placeholderHandler("projects"))
		})

		// Catalog routes — authenticated
		api.Route("/catalog", func(catalog chi.Router) {
			catalog.HandleFunc("/*", placeholderHandler("catalog"))
		})
	})

	return r
}

// healthHandler returns a handler that serves the /health endpoint.
func healthHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]string{
			"status":    "ok",
			"version":   "0.1.0",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

// placeholderHandler returns a handler that responds with a "not implemented" message.
// This will be replaced as each route group is implemented.
func placeholderHandler(group string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		RespondJSON(w, http.StatusNotImplemented, map[string]string{
			"message": group + " routes not yet implemented",
		})
	}
}
