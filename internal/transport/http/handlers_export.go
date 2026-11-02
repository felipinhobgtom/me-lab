package http

import (
	"net/http"
	"time"

	"github.com/felipinho/me-lab/internal/export"
	"github.com/felipinho/me-lab/internal/simulation"
	"github.com/go-chi/chi/v5"
)

func (s *Server) buildExportBlueprint(w http.ResponseWriter, r *http.Request) (export.Blueprint, map[string]string, bool) {
	project, ok := s.loadOwnedProject(w, r, chi.URLParam(r, "projectId"))
	if !ok {
		return export.Blueprint{}, nil, false
	}

	profile, exists := s.catalog.GetProfile(project.ProfileID)
	if !exists {
		RespondError(w, r, http.StatusInternalServerError, ErrCodeInternal, "Compatibility profile not found")
		return export.Blueprint{}, nil, false
	}

	blocks, err := s.store.ListBlocks(r.Context(), project.ID)
	if err != nil {
		s.logger.Error("export list blocks failed", "error", err)
		RespondError(w, r, http.StatusInternalServerError, ErrCodeInternal, "Failed to load project blocks")
		return export.Blueprint{}, nil, false
	}
	patterns, err := s.store.ListPatterns(r.Context(), project.ID)
	if err != nil {
		RespondError(w, r, http.StatusInternalServerError, ErrCodeInternal, "Failed to load project patterns")
		return export.Blueprint{}, nil, false
	}
	inv, err := s.store.ListInventory(r.Context(), project.ID)
	if err != nil {
		RespondError(w, r, http.StatusInternalServerError, ErrCodeInternal, "Failed to load project inventory")
		return export.Blueprint{}, nil, false
	}

	catMap := simulation.BuildCatalogMap(s.catalog.BlocksForProfile(project.ProfileID))
	valRes := s.engine.Validate(simulation.Snapshot{
		SimulationID: "export-" + project.ID,
		ProjectID:    project.ID,
		Seed:         1,
		StartedAt:    time.Now().UTC(),
		Profile:      profile,
		Catalog:      catMap,
		Blocks:       blocks,
		Patterns:     patterns,
		Inventory:    inv,
	})
	summaryCopy := valRes.Summary

	bp := export.BuildBlueprint(project, profile, blocks, patterns, inv, &summaryCopy)
	meta := map[string]string{
		"project_id":   project.ID,
		"project_name": project.Name,
		"profile_id":   project.ProfileID,
	}
	return bp, meta, true
}

func (s *Server) handleExportBlueprintJSON(w http.ResponseWriter, r *http.Request) {
	bp, _, ok := s.buildExportBlueprint(w, r)
	if !ok {
		return
	}
	RespondJSON(w, http.StatusOK, bp)
}

func (s *Server) handleExportBuildSheet(w http.ResponseWriter, r *http.Request) {
	bp, meta, ok := s.buildExportBlueprint(w, r)
	if !ok {
		return
	}
	catMap := simulation.BuildCatalogMap(s.catalog.BlocksForProfile(meta["profile_id"]))
	markdown := export.RenderBuildSheet(bp, catMap)

	RespondJSON(w, http.StatusOK, map[string]string{
		"project_id":   meta["project_id"],
		"project_name": meta["project_name"],
		"format":       "markdown",
		"content":      markdown,
	})
}
