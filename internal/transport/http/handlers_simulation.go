package http

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/felipinho/me-lab/internal/domain"
	"github.com/felipinho/me-lab/internal/repository/sqlite"
	"github.com/felipinho/me-lab/internal/simulation"
	"github.com/felipinho/me-lab/internal/simulation/crafting"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

const maxStepTicks = 500

type createSimulationRequest struct {
	Seed             int64                    `json:"seed"`
	AutoSteps        int                      `json:"auto_steps"`
	CraftingRequests []domain.CraftingRequest `json:"crafting_requests"`
}

type stepSimulationRequest struct {
	Ticks int `json:"ticks"`
}

type queueCraftingRequest struct {
	ItemID   string `json:"item_id"`
	Quantity int    `json:"quantity"`
	Source   string `json:"source"`
}

func (s *Server) buildProjectSnapshot(
	r *http.Request,
	project domain.Project,
	simID string,
	seed int64,
	startedAt time.Time,
	requests []domain.CraftingRequest,
) (simulation.Snapshot, error) {
	profile, ok := s.catalog.GetProfile(project.ProfileID)
	if !ok {
		return simulation.Snapshot{}, errors.New("project compatibility profile not found")
	}

	blocks, err := s.store.ListBlocks(r.Context(), project.ID)
	if err != nil {
		return simulation.Snapshot{}, err
	}

	patterns, err := s.store.ListPatterns(r.Context(), project.ID)
	if err != nil {
		return simulation.Snapshot{}, err
	}

	inv, err := s.store.ListInventory(r.Context(), project.ID)
	if err != nil {
		return simulation.Snapshot{}, err
	}

	catMap := simulation.BuildCatalogMap(s.catalog.BlocksForProfile(project.ProfileID))

	return simulation.Snapshot{
		SimulationID:     simID,
		ProjectID:        project.ID,
		Seed:             seed,
		StartedAt:        startedAt,
		Profile:          profile,
		Catalog:          catMap,
		Blocks:           blocks,
		Patterns:         patterns,
		Inventory:        inv,
		CraftingRequests: requests,
	}, nil
}

func (s *Server) handleValidateProject(w http.ResponseWriter, r *http.Request) {
	project, ok := s.loadOwnedProject(w, r, chi.URLParam(r, "projectId"))
	if !ok {
		return
	}

	snap, err := s.buildProjectSnapshot(r, project, "val-"+project.ID, 1, time.Now().UTC(), nil)
	if err != nil {
		s.logger.Error("build validation snapshot failed", "error", err, "project_id", project.ID)
		RespondError(w, r, http.StatusInternalServerError, ErrCodeInternal, "Failed to validate project")
		return
	}

	res := s.engine.Validate(snap)
	if res.Diagnostics == nil {
		res.Diagnostics = []domain.Diagnostic{}
	}
	RespondJSON(w, http.StatusOK, res)
}

func (s *Server) handleCreateSimulation(w http.ResponseWriter, r *http.Request) {
	project, ok := s.loadOwnedProject(w, r, chi.URLParam(r, "projectId"))
	if !ok {
		return
	}

	var req createSimulationRequest
	if r.Body != nil {
		dec := json.NewDecoder(r.Body)
		if err := dec.Decode(&req); err != nil && !errors.Is(err, io.EOF) {
			RespondError(w, r, http.StatusBadRequest, ErrCodeBadRequest, "Invalid JSON request body")
			return
		}
	}

	if req.AutoSteps < 0 || req.AutoSteps > maxStepTicks {
		RespondValidationError(w, r, map[string]any{"auto_steps": "must be between 0 and 500"})
		return
	}
	for i, cr := range req.CraftingRequests {
		if cr.ItemID == "" || cr.Quantity < 1 || cr.Quantity > 1_000_000 {
			RespondValidationError(w, r, map[string]any{"crafting_requests": "invalid item_id or quantity", "index": i})
			return
		}
	}

	if req.Seed == 0 {
		req.Seed = 1
	}

	simID := uuid.NewString()
	now := time.Now().UTC()

	snap, err := s.buildProjectSnapshot(r, project, simID, req.Seed, now, req.CraftingRequests)
	if err != nil {
		s.logger.Error("build simulation snapshot failed", "error", err, "project_id", project.ID)
		RespondError(w, r, http.StatusInternalServerError, ErrCodeInternal, "Failed to initialize simulation")
		return
	}

	state, events, diags, err := s.engine.Initialize(snap)
	if err != nil {
		s.logger.Error("engine initialize failed", "error", err, "project_id", project.ID)
		RespondError(w, r, http.StatusInternalServerError, ErrCodeInternal, "Simulation engine initialization failed")
		return
	}

	if req.AutoSteps > 0 {
		stepRes, err := s.engine.StepN(state, req.AutoSteps, snap.Profile, snap.Catalog)
		if err != nil {
			s.logger.Error("engine auto-step failed", "error", err)
			RespondError(w, r, http.StatusInternalServerError, ErrCodeInternal, "Simulation auto-step failed")
			return
		}
		events = append(events, stepRes.Events...)
		diags = stepRes.Diagnostics
	}

	snapBytes, err := json.Marshal(snap)
	if err != nil {
		RespondError(w, r, http.StatusInternalServerError, ErrCodeInternal, "Failed to serialize simulation snapshot")
		return
	}
	stateBytes, err := simulation.MarshalState(state)
	if err != nil {
		RespondError(w, r, http.StatusInternalServerError, ErrCodeInternal, "Failed to serialize simulation state")
		return
	}

	run := state.ToSimulationRun()
	if err := s.store.CreateSimulationRun(r.Context(), run, snapBytes, stateBytes, events, diags); err != nil {
		s.logger.Error("persist simulation run failed", "error", err)
		RespondError(w, r, http.StatusInternalServerError, ErrCodeInternal, "Failed to persist simulation run")
		return
	}

	RespondJSON(w, http.StatusCreated, run)
}

func (s *Server) handleQueueCrafting(w http.ResponseWriter, r *http.Request) {
	run, project, _, stateBytes, ok := s.loadOwnedSimulation(w, r, chi.URLParam(r, "simulationId"))
	if !ok {
		return
	}

	var req queueCraftingRequest
	if r.Body == nil || json.NewDecoder(r.Body).Decode(&req) != nil {
		RespondError(w, r, http.StatusBadRequest, ErrCodeBadRequest, "Invalid crafting request")
		return
	}
	req.ItemID = strings.TrimSpace(req.ItemID)
	if req.Source == "" {
		req.Source = "crafting_terminal"
	}
	if req.ItemID == "" || req.Quantity < 1 || req.Quantity > 1_000_000 {
		RespondValidationError(w, r, map[string]any{"item_id": "required", "quantity": "must be between 1 and 1000000"})
		return
	}
	if run.Status != domain.SimStatusRunning {
		RespondError(w, r, http.StatusConflict, ErrCodeConflict, "Live simulation is not running")
		return
	}

	var state simulation.EngineState
	if err := json.Unmarshal(stateBytes, &state); err != nil {
		RespondError(w, r, http.StatusInternalServerError, ErrCodeInternal, "Corrupted simulation state")
		return
	}
	if state.Crafting == nil || state.Storage == nil {
		RespondError(w, r, http.StatusConflict, ErrCodeConflict, "Live crafting runtime is unavailable")
		return
	}
	profile, exists := s.catalog.GetProfile(project.ProfileID)
	if !exists {
		RespondError(w, r, http.StatusInternalServerError, ErrCodeInternal, "Project compatibility profile not found")
		return
	}
	emitter := crafting.EventEmitter{
		SimulationID: state.SimulationID,
		NextSequence: &state.NextSequence,
	}
	events, diags := state.Crafting.EnqueueRequest(
		state.SimulationID,
		domain.CraftingRequest{ItemID: req.ItemID, Quantity: req.Quantity, Source: req.Source},
		state.Storage,
		profile,
		emitter,
	)
	newStateBytes, err := simulation.MarshalState(&state)
	if err != nil {
		RespondError(w, r, http.StatusInternalServerError, ErrCodeInternal, "Failed to serialize crafting state")
		return
	}
	if err := s.store.UpdateSimulationRunState(r.Context(), state.ToSimulationRun(), newStateBytes, events, diags); err != nil {
		s.logger.Error("persist queued crafting request failed", "error", err, "simulation_id", state.SimulationID)
		RespondError(w, r, http.StatusInternalServerError, ErrCodeInternal, "Failed to queue crafting request")
		return
	}
	RespondJSON(w, http.StatusAccepted, state.ToSimulationRun())
}

func (s *Server) handleListSimulations(w http.ResponseWriter, r *http.Request) {
	project, ok := s.loadOwnedProject(w, r, chi.URLParam(r, "projectId"))
	if !ok {
		return
	}
	pg := ParsePagination(r)
	runs, total, err := s.store.ListSimulationRuns(r.Context(), project.ID, pg.PageSize, pg.Offset())
	if err != nil {
		s.logger.Error("list simulation runs failed", "error", err)
		RespondError(w, r, http.StatusInternalServerError, ErrCodeInternal, "Failed to list simulations")
		return
	}
	if runs == nil {
		runs = []domain.SimulationRun{}
	}
	RespondJSON(w, http.StatusOK, NewPaginatedResponse(runs, pg.Page, pg.PageSize, total))
}

func (s *Server) loadOwnedSimulation(
	w http.ResponseWriter,
	r *http.Request,
	simID string,
) (domain.SimulationRun, domain.Project, []byte, []byte, bool) {
	run, snapBytes, stateBytes, err := s.store.GetSimulationRun(r.Context(), simID)
	if err != nil {
		if errors.Is(err, sqlite.ErrNotFound) {
			RespondError(w, r, http.StatusNotFound, ErrCodeNotFound, "Simulation run not found")
			return domain.SimulationRun{}, domain.Project{}, nil, nil, false
		}
		s.logger.Error("get simulation run failed", "error", err, "simulation_id", simID)
		RespondError(w, r, http.StatusInternalServerError, ErrCodeInternal, "Failed to load simulation run")
		return domain.SimulationRun{}, domain.Project{}, nil, nil, false
	}

	project, ok := s.loadOwnedProject(w, r, run.ProjectID)
	if !ok {
		return domain.SimulationRun{}, domain.Project{}, nil, nil, false
	}
	return run, project, snapBytes, stateBytes, true
}

func (s *Server) handleGetSimulation(w http.ResponseWriter, r *http.Request) {
	run, _, _, _, ok := s.loadOwnedSimulation(w, r, chi.URLParam(r, "simulationId"))
	if !ok {
		return
	}
	RespondJSON(w, http.StatusOK, run)
}

func (s *Server) handleStepSimulation(w http.ResponseWriter, r *http.Request) {
	_, project, _, stateBytes, ok := s.loadOwnedSimulation(w, r, chi.URLParam(r, "simulationId"))
	if !ok {
		return
	}

	ticks := 1
	if r.Body != nil {
		var req stepSimulationRequest
		dec := json.NewDecoder(r.Body)
		if err := dec.Decode(&req); err != nil && !errors.Is(err, io.EOF) {
			RespondError(w, r, http.StatusBadRequest, ErrCodeBadRequest, "Invalid JSON request body")
			return
		}
		if req.Ticks != 0 {
			ticks = req.Ticks
		}
	}
	if ticks < 1 || ticks > maxStepTicks {
		RespondValidationError(w, r, map[string]any{"ticks": "must be between 1 and 500"})
		return
	}

	var state simulation.EngineState
	if err := json.Unmarshal(stateBytes, &state); err != nil {
		s.logger.Error("unmarshal simulation state failed", "error", err)
		RespondError(w, r, http.StatusInternalServerError, ErrCodeInternal, "Corrupted simulation state")
		return
	}

	profile, _ := s.catalog.GetProfile(project.ProfileID)
	catMap := simulation.BuildCatalogMap(s.catalog.BlocksForProfile(project.ProfileID))

	stepRes, err := s.engine.StepN(&state, ticks, profile, catMap)
	if err != nil {
		s.logger.Error("step simulation failed", "error", err)
		RespondError(w, r, http.StatusInternalServerError, ErrCodeInternal, "Simulation step failed")
		return
	}

	newStateBytes, err := simulation.MarshalState(&state)
	if err != nil {
		RespondError(w, r, http.StatusInternalServerError, ErrCodeInternal, "Failed to serialize state")
		return
	}

	if err := s.store.UpdateSimulationRunState(r.Context(), stepRes.State, newStateBytes, stepRes.Events, stepRes.Diagnostics); err != nil {
		s.logger.Error("persist stepped simulation failed", "error", err)
		RespondError(w, r, http.StatusInternalServerError, ErrCodeInternal, "Failed to persist simulation step")
		return
	}

	RespondJSON(w, http.StatusOK, stepRes)
}

func (s *Server) handlePauseSimulation(w http.ResponseWriter, r *http.Request) {
	_, _, _, stateBytes, ok := s.loadOwnedSimulation(w, r, chi.URLParam(r, "simulationId"))
	if !ok {
		return
	}

	var state simulation.EngineState
	if err := json.Unmarshal(stateBytes, &state); err != nil {
		RespondError(w, r, http.StatusInternalServerError, ErrCodeInternal, "Corrupted simulation state")
		return
	}

	simulation.Pause(&state)
	newStateBytes, err := simulation.MarshalState(&state)
	if err != nil {
		RespondError(w, r, http.StatusInternalServerError, ErrCodeInternal, "Failed to serialize state")
		return
	}
	run := state.ToSimulationRun()
	if err := s.store.UpdateSimulationRunState(r.Context(), run, newStateBytes, nil, state.Diagnostics); err != nil {
		RespondError(w, r, http.StatusInternalServerError, ErrCodeInternal, "Failed to pause simulation")
		return
	}
	RespondJSON(w, http.StatusOK, run)
}

func (s *Server) handleResumeSimulation(w http.ResponseWriter, r *http.Request) {
	_, _, _, stateBytes, ok := s.loadOwnedSimulation(w, r, chi.URLParam(r, "simulationId"))
	if !ok {
		return
	}

	var state simulation.EngineState
	if err := json.Unmarshal(stateBytes, &state); err != nil {
		RespondError(w, r, http.StatusInternalServerError, ErrCodeInternal, "Corrupted simulation state")
		return
	}

	simulation.Resume(&state)
	newStateBytes, err := simulation.MarshalState(&state)
	if err != nil {
		RespondError(w, r, http.StatusInternalServerError, ErrCodeInternal, "Failed to serialize state")
		return
	}
	run := state.ToSimulationRun()
	if err := s.store.UpdateSimulationRunState(r.Context(), run, newStateBytes, nil, state.Diagnostics); err != nil {
		RespondError(w, r, http.StatusInternalServerError, ErrCodeInternal, "Failed to resume simulation")
		return
	}
	RespondJSON(w, http.StatusOK, run)
}

func (s *Server) handleResetSimulation(w http.ResponseWriter, r *http.Request) {
	_, _, snapBytes, _, ok := s.loadOwnedSimulation(w, r, chi.URLParam(r, "simulationId"))
	if !ok {
		return
	}

	var snap simulation.Snapshot
	if err := json.Unmarshal(snapBytes, &snap); err != nil {
		RespondError(w, r, http.StatusInternalServerError, ErrCodeInternal, "Corrupted simulation snapshot")
		return
	}

	state, initEvents, diags, err := s.engine.Initialize(snap)
	if err != nil {
		RespondError(w, r, http.StatusInternalServerError, ErrCodeInternal, "Failed to re-initialize simulation")
		return
	}

	newStateBytes, err := simulation.MarshalState(state)
	if err != nil {
		RespondError(w, r, http.StatusInternalServerError, ErrCodeInternal, "Failed to serialize state")
		return
	}

	run := state.ToSimulationRun()
	if err := s.store.ResetSimulationRunState(r.Context(), run, newStateBytes, initEvents, diags); err != nil {
		s.logger.Error("reset simulation failed", "error", err)
		RespondError(w, r, http.StatusInternalServerError, ErrCodeInternal, "Failed to reset simulation")
		return
	}
	RespondJSON(w, http.StatusOK, run)
}

func (s *Server) handleGetSimulationEvents(w http.ResponseWriter, r *http.Request) {
	run, _, _, _, ok := s.loadOwnedSimulation(w, r, chi.URLParam(r, "simulationId"))
	if !ok {
		return
	}
	pg := ParsePagination(r)
	events, total, err := s.store.ListSimulationEvents(r.Context(), run.ID, pg.PageSize, pg.Offset())
	if err != nil {
		s.logger.Error("list simulation events failed", "error", err)
		RespondError(w, r, http.StatusInternalServerError, ErrCodeInternal, "Failed to list simulation events")
		return
	}
	if events == nil {
		events = []domain.SimulationEvent{}
	}
	RespondJSON(w, http.StatusOK, NewPaginatedResponse(events, pg.Page, pg.PageSize, total))
}

func (s *Server) handleGetSimulationDiagnostics(w http.ResponseWriter, r *http.Request) {
	run, _, _, _, ok := s.loadOwnedSimulation(w, r, chi.URLParam(r, "simulationId"))
	if !ok {
		return
	}
	diags, err := s.store.ListSimulationDiagnostics(r.Context(), run.ID)
	if err != nil {
		s.logger.Error("list simulation diagnostics failed", "error", err)
		RespondError(w, r, http.StatusInternalServerError, ErrCodeInternal, "Failed to list simulation diagnostics")
		return
	}
	if diags == nil {
		diags = []domain.Diagnostic{}
	}
	RespondJSON(w, http.StatusOK, diags)
}
