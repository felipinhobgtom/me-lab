package http

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/felipinho/me-lab/internal/catalog"
	"github.com/felipinho/me-lab/internal/repository/sqlite"
	"github.com/felipinho/me-lab/internal/simulation"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

// Deps holds dependencies required to construct the HTTP router.
type Deps struct {
	Logger             *slog.Logger
	Store              *sqlite.Store
	Catalog            *catalog.Registry
	Engine             *simulation.Engine
	JWTSecret          string
	JWTExpiry          time.Duration
	CORSAllowedOrigins []string
}

// Server bundles HTTP handlers with domain services and persistence.
type Server struct {
	logger    *slog.Logger
	store     *sqlite.Store
	catalog   *catalog.Registry
	engine    *simulation.Engine
	jwtSecret string
	jwtExpiry time.Duration
}

// NewRouter constructs the Chi router with middleware, operational endpoints, OpenAPI docs, and API v1 routes.
func NewRouter(deps Deps) *chi.Mux {
	logger := deps.Logger
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	expiry := deps.JWTExpiry
	if expiry <= 0 {
		expiry = 24 * time.Hour
	}
	cat := deps.Catalog
	if cat == nil {
		cat, _ = catalog.NewRegistry()
	}
	eng := deps.Engine
	if eng == nil {
		eng = simulation.NewEngine()
	}
	origins := deps.CORSAllowedOrigins
	if len(origins) == 0 {
		origins = []string{"http://localhost:*", "http://127.0.0.1:*"}
	}

	srv := &Server{
		logger:    logger,
		store:     deps.Store,
		catalog:   cat,
		engine:    eng,
		jwtSecret: deps.JWTSecret,
		jwtExpiry: expiry,
	}

	r := chi.NewRouter()

	// Core middleware stack
	r.Use(RequestID)
	r.Use(StructuredLogger(logger))
	r.Use(chimw.Recoverer)
	r.Use(MaxBodySize(maxBodyBytes))

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   origins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Request-ID"},
		ExposedHeaders:   []string{"X-Request-ID"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Operational & API documentation endpoints
	r.Get("/health", healthHandler())
	r.Get("/ready", srv.readyHandler())
	r.Get("/openapi.json", openAPIHandler())
	r.Get("/docs", swaggerDocsHandler())

	// Versioned REST API v1
	r.Route("/api/v1", func(api chi.Router) {
		// Auth endpoints
		api.Route("/auth", func(authGroup chi.Router) {
			authGroup.Post("/register", srv.handleRegister)
			authGroup.Post("/login", srv.handleLogin)
			authGroup.With(AuthMiddleware(srv.jwtSecret)).Get("/me", srv.handleMe)
		})

		// Block & compatibility profile catalog (public read so docs/explorers can inspect profiles)
		api.Route("/catalog", func(catGroup chi.Router) {
			catGroup.Get("/profiles", srv.handleListProfiles)
			catGroup.Get("/blocks", srv.handleListBlocksCatalog)
			catGroup.Get("/blocks/{blockDefinitionId}", srv.handleGetBlockDefinition)
		})

		// Authenticated project & simulation endpoints
		api.Group(func(authed chi.Router) {
			authed.Use(SandboxAuthMiddleware(srv.jwtSecret, srv.store))

			authed.Route("/projects", func(proj chi.Router) {
				proj.Post("/", srv.handleCreateProject)
				proj.Get("/", srv.handleListProjects)

				proj.Route("/{projectId}", func(p chi.Router) {
					p.Get("/", srv.handleGetProject)
					p.Patch("/", srv.handleUpdateProject)
					p.Delete("/", srv.handleDeleteProject)

					// Project 3D layout
					p.Get("/blocks", srv.handleListBlocks)
					p.Post("/blocks", srv.handlePlaceBlock)
					p.Post("/blocks/batch", srv.handleBatchBlocks)
					p.Patch("/blocks/{blockId}", srv.handleUpdateBlock)
					p.Delete("/blocks/{blockId}", srv.handleDeleteBlock)

					// Validation & Simulation creation/listing
					p.Post("/validate", srv.handleValidateProject)
					p.Post("/simulations", srv.handleCreateSimulation)
					p.Get("/simulations", srv.handleListSimulations)

					// Blueprint export
					p.Get("/export/json", srv.handleExportBlueprintJSON)
					p.Get("/export/build-sheet", srv.handleExportBuildSheet)
				})
			})

			authed.Route("/simulations/{simulationId}", func(sim chi.Router) {
				sim.Get("/", srv.handleGetSimulation)
				sim.Post("/craft", srv.handleQueueCrafting)
				sim.Post("/step", srv.handleStepSimulation)
				sim.Post("/pause", srv.handlePauseSimulation)
				sim.Post("/resume", srv.handleResumeSimulation)
				sim.Post("/reset", srv.handleResetSimulation)
				sim.Get("/events", srv.handleGetSimulationEvents)
				sim.Get("/diagnostics", srv.handleGetSimulationDiagnostics)
			})
		})
	})

	return r
}

func healthHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]string{
			"status":    "ok",
			"version":   "1.0.0",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}
}

func (s *Server) readyHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.store != nil && s.store.DB() != nil {
			ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
			defer cancel()
			if err := s.store.DB().PingContext(ctx); err != nil {
				RespondError(w, r, http.StatusServiceUnavailable, ErrCodeInternal, "Database not ready")
				return
			}
		}
		RespondJSON(w, http.StatusOK, map[string]string{
			"status": "ok",
			"db":     "connected",
		})
	}
}
