package http

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/felipinho/me-lab/internal/catalog"
	"github.com/felipinho/me-lab/internal/domain"
	"github.com/felipinho/me-lab/internal/repository/sqlite"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

const (
	maxGridCoord = 512
	maxBatchOps  = 2000
)

type createProjectRequest struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	ProfileID   string         `json:"profile_id"`
	Settings    map[string]any `json:"settings"`
}

type updateProjectRequest struct {
	Name        *string        `json:"name"`
	Description *string        `json:"description"`
	ProfileID   *string        `json:"profile_id"`
	Settings    map[string]any `json:"settings"`
}

type placeBlockRequest struct {
	BlockDefID string           `json:"block_def_id"`
	X          int              `json:"x"`
	Y          int              `json:"y"`
	Z          int              `json:"z"`
	Facing     domain.Direction `json:"facing"`
	Config     map[string]any   `json:"config"`
	Label      string           `json:"label"`
}

type updateBlockRequest struct {
	BlockDefID *string        `json:"block_def_id"`
	X          *int           `json:"x"`
	Y          *int           `json:"y"`
	Z          *int           `json:"z"`
	Facing     *string        `json:"facing"`
	Config     map[string]any `json:"config"`
	Label      *string        `json:"label"`
}

type batchBlockItemDTO struct {
	Operation string         `json:"operation"`
	Op        string         `json:"op"`
	BlockID   string         `json:"block_id"`
	ID        string         `json:"id"`
	Data      map[string]any `json:"data"`
	// Direct top-level fields also supported for flexibility
	BlockDefID string         `json:"block_def_id"`
	X          *int           `json:"x"`
	Y          *int           `json:"y"`
	Z          *int           `json:"z"`
	Facing     string         `json:"facing"`
	Config     map[string]any `json:"config"`
	Label      string         `json:"label"`
}

type batchBlocksRequest struct {
	ReplaceAll bool                `json:"replace_all"`
	Operations []batchBlockItemDTO `json:"operations"`
}

func (s *Server) loadOwnedProject(w http.ResponseWriter, r *http.Request, projectID string) (domain.Project, bool) {
	userID := UserIDFromContext(r.Context())
	project, err := s.store.GetProject(r.Context(), projectID)
	if err != nil {
		if errors.Is(err, sqlite.ErrNotFound) {
			RespondError(w, r, http.StatusNotFound, ErrCodeNotFound, "Project not found")
			return domain.Project{}, false
		}
		s.logger.Error("get project failed", "error", err, "project_id", projectID)
		RespondError(w, r, http.StatusInternalServerError, ErrCodeInternal, "Failed to load project")
		return domain.Project{}, false
	}
	if project.UserID != userID {
		RespondError(w, r, http.StatusForbidden, ErrCodeForbidden, "You do not have access to this project")
		return domain.Project{}, false
	}
	return project, true
}

func (s *Server) handleCreateProject(w http.ResponseWriter, r *http.Request) {
	var req createProjectRequest
	if err := decodeJSONBody(r, &req); err != nil {
		RespondError(w, r, http.StatusBadRequest, ErrCodeBadRequest, err.Error())
		return
	}

	name := strings.TrimSpace(req.Name)
	profileID := strings.TrimSpace(req.ProfileID)
	if profileID == "" {
		profileID = catalog.DefaultProfileID
	}

	valErrs := map[string]any{}
	if name == "" {
		valErrs["name"] = "project name is required"
	} else if len(name) > 120 {
		valErrs["name"] = "project name must not exceed 120 characters"
	}
	if len(req.Description) > 2000 {
		valErrs["description"] = "description must not exceed 2000 characters"
	}
	if _, ok := s.catalog.GetProfile(profileID); !ok {
		valErrs["profile_id"] = "unknown compatibility profile"
	}
	if len(valErrs) > 0 {
		RespondValidationError(w, r, valErrs)
		return
	}

	settings := req.Settings
	if settings == nil {
		settings = map[string]any{}
	}

	now := time.Now().UTC()
	created, err := s.store.CreateProject(r.Context(), domain.Project{
		ID:          uuid.NewString(),
		UserID:      UserIDFromContext(r.Context()),
		Name:        name,
		Description: strings.TrimSpace(req.Description),
		ProfileID:   profileID,
		Settings:    settings,
		CreatedAt:   now,
		UpdatedAt:   now,
	})
	if err != nil {
		s.logger.Error("create project failed", "error", err)
		RespondError(w, r, http.StatusInternalServerError, ErrCodeInternal, "Failed to create project")
		return
	}

	RespondJSON(w, http.StatusCreated, created)
}

func (s *Server) handleListProjects(w http.ResponseWriter, r *http.Request) {
	pg := ParsePagination(r)
	projects, total, err := s.store.ListProjectsByOwner(r.Context(), UserIDFromContext(r.Context()), pg.PageSize, pg.Offset())
	if err != nil {
		s.logger.Error("list projects failed", "error", err)
		RespondError(w, r, http.StatusInternalServerError, ErrCodeInternal, "Failed to list projects")
		return
	}
	if projects == nil {
		projects = []domain.Project{}
	}
	RespondJSON(w, http.StatusOK, NewPaginatedResponse(projects, pg.Page, pg.PageSize, total))
}

func (s *Server) handleGetProject(w http.ResponseWriter, r *http.Request) {
	project, ok := s.loadOwnedProject(w, r, chi.URLParam(r, "projectId"))
	if !ok {
		return
	}
	RespondJSON(w, http.StatusOK, project)
}

func (s *Server) handleUpdateProject(w http.ResponseWriter, r *http.Request) {
	project, ok := s.loadOwnedProject(w, r, chi.URLParam(r, "projectId"))
	if !ok {
		return
	}

	var req updateProjectRequest
	if err := decodeJSONBody(r, &req); err != nil {
		RespondError(w, r, http.StatusBadRequest, ErrCodeBadRequest, err.Error())
		return
	}

	valErrs := map[string]any{}
	if req.Name != nil {
		n := strings.TrimSpace(*req.Name)
		if n == "" || len(n) > 120 {
			valErrs["name"] = "name must be 1..120 characters"
		} else {
			project.Name = n
		}
	}
	if req.Description != nil {
		if len(*req.Description) > 2000 {
			valErrs["description"] = "description must not exceed 2000 characters"
		} else {
			project.Description = strings.TrimSpace(*req.Description)
		}
	}
	if req.ProfileID != nil {
		pid := strings.TrimSpace(*req.ProfileID)
		if _, exists := s.catalog.GetProfile(pid); !exists {
			valErrs["profile_id"] = "unknown compatibility profile"
		} else {
			project.ProfileID = pid
		}
	}
	if req.Settings != nil {
		project.Settings = req.Settings
	}
	if len(valErrs) > 0 {
		RespondValidationError(w, r, valErrs)
		return
	}

	project.UpdatedAt = time.Now().UTC()
	updated, err := s.store.UpdateProject(r.Context(), project)
	if err != nil {
		s.logger.Error("update project failed", "error", err)
		RespondError(w, r, http.StatusInternalServerError, ErrCodeInternal, "Failed to update project")
		return
	}
	RespondJSON(w, http.StatusOK, updated)
}

func (s *Server) handleDeleteProject(w http.ResponseWriter, r *http.Request) {
	project, ok := s.loadOwnedProject(w, r, chi.URLParam(r, "projectId"))
	if !ok {
		return
	}
	if err := s.store.DeleteProject(r.Context(), project.ID); err != nil {
		s.logger.Error("delete project failed", "error", err)
		RespondError(w, r, http.StatusInternalServerError, ErrCodeInternal, "Failed to delete project")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListBlocks(w http.ResponseWriter, r *http.Request) {
	project, ok := s.loadOwnedProject(w, r, chi.URLParam(r, "projectId"))
	if !ok {
		return
	}
	blocks, err := s.store.ListBlocks(r.Context(), project.ID)
	if err != nil {
		s.logger.Error("list blocks failed", "error", err)
		RespondError(w, r, http.StatusInternalServerError, ErrCodeInternal, "Failed to list blocks")
		return
	}
	if blocks == nil {
		blocks = []domain.PlacedBlock{}
	}
	RespondJSON(w, http.StatusOK, blocks)
}

func validateCoords(x, y, z int) bool {
	return x >= -maxGridCoord && x <= maxGridCoord &&
		y >= -maxGridCoord && y <= maxGridCoord &&
		z >= -maxGridCoord && z <= maxGridCoord
}

func validateStorageConfig(blockDefID string, cfg map[string]any) string {
	if blockDefID != "ae2:drive" && blockDefID != "ae2:chest" {
		return ""
	}
	rawCells, present := cfg["cells"]
	if !present {
		return ""
	}
	cells, ok := rawCells.([]any)
	if !ok {
		return "config.cells must be an array"
	}
	maxCells := 10
	if blockDefID == "ae2:chest" {
		maxCells = 1
	}
	if len(cells) > maxCells {
		return fmt.Sprintf("config.cells cannot contain more than %d storage cells", maxCells)
	}
	validTiers := map[string]bool{
		"1k": true, "4k": true, "16k": true, "64k": true, "256k": true,
	}
	for cellIndex, rawCell := range cells {
		cell, ok := rawCell.(map[string]any)
		if !ok {
			return fmt.Sprintf("config.cells[%d] must be an object", cellIndex)
		}
		if tier, exists := cell["tier"]; exists {
			tierName, ok := tier.(string)
			if !ok || !validTiers[tierName] {
				return fmt.Sprintf("config.cells[%d].tier is invalid", cellIndex)
			}
		}
		rawItems, exists := cell["items"]
		if !exists {
			continue
		}
		items, ok := rawItems.([]any)
		if !ok {
			return fmt.Sprintf("config.cells[%d].items must be an array", cellIndex)
		}
		seenTypes := map[string]struct{}{}
		for itemIndex, rawItem := range items {
			item, ok := rawItem.(map[string]any)
			if !ok {
				return fmt.Sprintf("config.cells[%d].items[%d] must be an object", cellIndex, itemIndex)
			}
			itemID, ok := item["item_id"].(string)
			if !ok || strings.TrimSpace(itemID) == "" {
				return fmt.Sprintf("config.cells[%d].items[%d].item_id is required", cellIndex, itemIndex)
			}
			itemID = strings.TrimSpace(itemID)
			if _, exists := seenTypes[itemID]; !exists {
				if len(seenTypes) >= 63 {
					return fmt.Sprintf("config.cells[%d].items cannot contain more than 63 types", cellIndex)
				}
				seenTypes[itemID] = struct{}{}
			}
			if _, ok := positiveStorageQuantity(item["quantity"]); !ok {
				return fmt.Sprintf("config.cells[%d].items[%d].quantity must be a positive integer", cellIndex, itemIndex)
			}
		}
	}
	return ""
}

func positiveStorageQuantity(value any) (int, bool) {
	quantity, ok := asInt(value)
	if !ok || quantity < 1 {
		return 0, false
	}
	if raw, isFloat := value.(float64); isFloat && raw != float64(quantity) {
		return 0, false
	}
	return quantity, true
}

func (s *Server) handlePlaceBlock(w http.ResponseWriter, r *http.Request) {
	project, ok := s.loadOwnedProject(w, r, chi.URLParam(r, "projectId"))
	if !ok {
		return
	}

	var req placeBlockRequest
	if err := decodeJSONBody(r, &req); err != nil {
		RespondError(w, r, http.StatusBadRequest, ErrCodeBadRequest, err.Error())
		return
	}

	facing := domain.North
	if req.Facing != "" {
		parsed, valid := domain.ParseDirection(string(req.Facing))
		if !valid {
			RespondValidationError(w, r, map[string]any{"facing": "must be north, south, east, west, up, or down"})
			return
		}
		facing = parsed
	}

	valErrs := map[string]any{}
	if _, exists := s.catalog.GetBlock(project.ProfileID, strings.TrimSpace(req.BlockDefID)); !exists {
		valErrs["block_def_id"] = "unknown block definition for project profile"
	}
	if !validateCoords(req.X, req.Y, req.Z) {
		valErrs["coordinates"] = "coordinates must be within [-512, 512]"
	}
	if len(req.Label) > 120 {
		valErrs["label"] = "label must not exceed 120 characters"
	}
	if len(valErrs) > 0 {
		RespondValidationError(w, r, valErrs)
		return
	}

	cfg := req.Config
	if cfg == nil {
		cfg = map[string]any{}
	}
	blockDefID := strings.TrimSpace(req.BlockDefID)
	if configErr := validateStorageConfig(blockDefID, cfg); configErr != "" {
		RespondValidationError(w, r, map[string]any{"config": configErr})
		return
	}

	now := time.Now().UTC()
	placed, err := s.store.CreateBlock(r.Context(), domain.PlacedBlock{
		ID:         uuid.NewString(),
		ProjectID:  project.ID,
		BlockDefID: strings.TrimSpace(req.BlockDefID),
		X:          req.X,
		Y:          req.Y,
		Z:          req.Z,
		Facing:     facing,
		Config:     cfg,
		Label:      strings.TrimSpace(req.Label),
		CreatedAt:  now,
		UpdatedAt:  now,
	})
	if err != nil {
		if errors.Is(err, sqlite.ErrConflict) {
			RespondError(w, r, http.StatusConflict, ErrCodeConflict, "Another block already occupies these coordinates")
			return
		}
		s.logger.Error("place block failed", "error", err)
		RespondError(w, r, http.StatusInternalServerError, ErrCodeInternal, "Failed to place block")
		return
	}

	RespondJSON(w, http.StatusCreated, placed)
}

func (s *Server) handleUpdateBlock(w http.ResponseWriter, r *http.Request) {
	project, ok := s.loadOwnedProject(w, r, chi.URLParam(r, "projectId"))
	if !ok {
		return
	}
	blockID := chi.URLParam(r, "blockId")

	existing, err := s.store.GetBlock(r.Context(), project.ID, blockID)
	if err != nil {
		if errors.Is(err, sqlite.ErrNotFound) {
			RespondError(w, r, http.StatusNotFound, ErrCodeNotFound, "Block not found")
			return
		}
		s.logger.Error("get block failed", "error", err)
		RespondError(w, r, http.StatusInternalServerError, ErrCodeInternal, "Failed to load block")
		return
	}

	var req updateBlockRequest
	if err := decodeJSONBody(r, &req); err != nil {
		RespondError(w, r, http.StatusBadRequest, ErrCodeBadRequest, err.Error())
		return
	}

	if req.BlockDefID != nil {
		defID := strings.TrimSpace(*req.BlockDefID)
		if _, exists := s.catalog.GetBlock(project.ProfileID, defID); !exists {
			RespondValidationError(w, r, map[string]any{"block_def_id": "unknown block definition"})
			return
		}
		existing.BlockDefID = defID
	}
	if req.X != nil {
		existing.X = *req.X
	}
	if req.Y != nil {
		existing.Y = *req.Y
	}
	if req.Z != nil {
		existing.Z = *req.Z
	}
	if !validateCoords(existing.X, existing.Y, existing.Z) {
		RespondValidationError(w, r, map[string]any{"coordinates": "coordinates must be within [-512, 512]"})
		return
	}
	if req.Facing != nil {
		dir, valid := domain.ParseDirection(*req.Facing)
		if !valid {
			RespondValidationError(w, r, map[string]any{"facing": "invalid direction"})
			return
		}
		existing.Facing = dir
	}
	if req.Config != nil {
		existing.Config = req.Config
	}
	if configErr := validateStorageConfig(existing.BlockDefID, existing.Config); configErr != "" {
		RespondValidationError(w, r, map[string]any{"config": configErr})
		return
	}
	if req.Label != nil {
		if len(*req.Label) > 120 {
			RespondValidationError(w, r, map[string]any{"label": "label must not exceed 120 characters"})
			return
		}
		existing.Label = strings.TrimSpace(*req.Label)
	}
	existing.UpdatedAt = time.Now().UTC()

	updated, err := s.store.UpdateBlock(r.Context(), existing)
	if err != nil {
		if errors.Is(err, sqlite.ErrConflict) {
			RespondError(w, r, http.StatusConflict, ErrCodeConflict, "Target coordinate is already occupied")
			return
		}
		s.logger.Error("update block failed", "error", err)
		RespondError(w, r, http.StatusInternalServerError, ErrCodeInternal, "Failed to update block")
		return
	}
	RespondJSON(w, http.StatusOK, updated)
}

func (s *Server) handleDeleteBlock(w http.ResponseWriter, r *http.Request) {
	project, ok := s.loadOwnedProject(w, r, chi.URLParam(r, "projectId"))
	if !ok {
		return
	}
	if err := s.store.DeleteBlock(r.Context(), project.ID, chi.URLParam(r, "blockId")); err != nil {
		if errors.Is(err, sqlite.ErrNotFound) {
			RespondError(w, r, http.StatusNotFound, ErrCodeNotFound, "Block not found")
			return
		}
		s.logger.Error("delete block failed", "error", err)
		RespondError(w, r, http.StatusInternalServerError, ErrCodeInternal, "Failed to delete block")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleBatchBlocks(w http.ResponseWriter, r *http.Request) {
	project, ok := s.loadOwnedProject(w, r, chi.URLParam(r, "projectId"))
	if !ok {
		return
	}

	var req batchBlocksRequest
	if err := decodeJSONBody(r, &req); err != nil {
		RespondError(w, r, http.StatusBadRequest, ErrCodeBadRequest, err.Error())
		return
	}
	if len(req.Operations) > maxBatchOps {
		RespondValidationError(w, r, map[string]any{"operations": "batch cannot exceed 2000 operations"})
		return
	}

	ops := make([]sqlite.BatchOperation, 0, len(req.Operations))
	for i, item := range req.Operations {
		opName := strings.ToLower(strings.TrimSpace(item.Operation))
		if opName == "" {
			opName = strings.ToLower(strings.TrimSpace(item.Op))
		}
		blockID := strings.TrimSpace(item.BlockID)
		if blockID == "" {
			blockID = strings.TrimSpace(item.ID)
		}

		// Merge nested `data` object with top-level fields
		blockDefID := strings.TrimSpace(item.BlockDefID)
		x, y, z := 0, 0, 0
		hasX, hasY, hasZ := false, false, false
		if item.X != nil {
			x, hasX = *item.X, true
		}
		if item.Y != nil {
			y, hasY = *item.Y, true
		}
		if item.Z != nil {
			z, hasZ = *item.Z, true
		}
		facingStr := item.Facing
		cfg := item.Config
		label := item.Label

		if item.Data != nil {
			if v, ok := item.Data["id"].(string); ok && blockID == "" {
				blockID = strings.TrimSpace(v)
			}
			if v, ok := item.Data["block_def_id"].(string); ok && blockDefID == "" {
				blockDefID = strings.TrimSpace(v)
			}
			if v, ok := asInt(item.Data["x"]); ok {
				x, hasX = v, true
			}
			if v, ok := asInt(item.Data["y"]); ok {
				y, hasY = v, true
			}
			if v, ok := asInt(item.Data["z"]); ok {
				z, hasZ = v, true
			}
			if v, ok := item.Data["facing"].(string); ok && facingStr == "" {
				facingStr = v
			}
			if v, ok := item.Data["config"].(map[string]any); ok && cfg == nil {
				cfg = v
			}
			if v, ok := item.Data["label"].(string); ok && label == "" {
				label = v
			}
		}

		switch opName {
		case "create", "place":
			if _, exists := s.catalog.GetBlock(project.ProfileID, blockDefID); !exists {
				RespondValidationError(w, r, map[string]any{"operations": "unknown block_def_id at index", "index": i, "block_def_id": blockDefID})
				return
			}
			if !hasX || !hasY || !hasZ || !validateCoords(x, y, z) {
				RespondValidationError(w, r, map[string]any{"operations": "invalid or missing coordinates at index", "index": i})
				return
			}
			facing := domain.North
			if facingStr != "" {
				d, valid := domain.ParseDirection(facingStr)
				if !valid {
					RespondValidationError(w, r, map[string]any{"operations": "invalid facing at index", "index": i})
					return
				}
				facing = d
			}
			if blockID == "" {
				blockID = uuid.NewString()
			}
			if cfg == nil {
				cfg = map[string]any{}
			}
			if configErr := validateStorageConfig(blockDefID, cfg); configErr != "" {
				RespondValidationError(w, r, map[string]any{
					"operations": configErr,
					"index":      i,
				})
				return
			}
			ops = append(ops, sqlite.BatchOperation{
				Op:         sqlite.BatchOpCreate,
				ID:         blockID,
				BlockDefID: blockDefID,
				X:          x,
				Y:          y,
				Z:          z,
				Coord:      &domain.Coord3D{X: x, Y: y, Z: z},
				Facing:     facing,
				Config:     cfg,
				Label:      strings.TrimSpace(label),
			})

		case "update":
			if blockID == "" {
				RespondValidationError(w, r, map[string]any{"operations": "block_id required for update", "index": i})
				return
			}
			existing, err := s.store.GetBlock(r.Context(), project.ID, blockID)
			if err != nil {
				RespondError(w, r, http.StatusNotFound, ErrCodeNotFound, "Block in batch update not found")
				return
			}
			if blockDefID == "" {
				blockDefID = existing.BlockDefID
			} else if _, exists := s.catalog.GetBlock(project.ProfileID, blockDefID); !exists {
				RespondValidationError(w, r, map[string]any{"operations": "unknown block_def_id", "index": i})
				return
			}
			if !hasX {
				x = existing.X
			}
			if !hasY {
				y = existing.Y
			}
			if !hasZ {
				z = existing.Z
			}
			if !validateCoords(x, y, z) {
				RespondValidationError(w, r, map[string]any{"operations": "coordinates out of bounds", "index": i})
				return
			}
			facing := existing.Facing
			if facingStr != "" {
				d, valid := domain.ParseDirection(facingStr)
				if !valid {
					RespondValidationError(w, r, map[string]any{"operations": "invalid facing", "index": i})
					return
				}
				facing = d
			}
			if cfg == nil {
				cfg = existing.Config
			}
			if configErr := validateStorageConfig(blockDefID, cfg); configErr != "" {
				RespondValidationError(w, r, map[string]any{
					"operations": configErr,
					"index":      i,
				})
				return
			}
			if label == "" && item.Data != nil {
				if _, hasLbl := item.Data["label"]; !hasLbl {
					label = existing.Label
				}
			} else if label == "" {
				label = existing.Label
			}
			ops = append(ops, sqlite.BatchOperation{
				Op:         sqlite.BatchOpUpdate,
				ID:         blockID,
				BlockDefID: blockDefID,
				X:          x,
				Y:          y,
				Z:          z,
				Coord:      &domain.Coord3D{X: x, Y: y, Z: z},
				Facing:     facing,
				Config:     cfg,
				Label:      strings.TrimSpace(label),
			})

		case "delete":
			if blockID == "" {
				RespondValidationError(w, r, map[string]any{"operations": "block_id required for delete", "index": i})
				return
			}
			ops = append(ops, sqlite.BatchOperation{
				Op: sqlite.BatchOpDelete,
				ID: blockID,
			})

		default:
			RespondValidationError(w, r, map[string]any{"operations": "unsupported batch operation", "index": i})
			return
		}
	}

	results, err := s.store.ApplyBlockBatch(r.Context(), project.ID, ops, req.ReplaceAll)
	if err != nil {
		if errors.Is(err, sqlite.ErrConflict) {
			RespondError(w, r, http.StatusConflict, ErrCodeConflict, "Batch operation caused a coordinate conflict")
			return
		}
		if errors.Is(err, sqlite.ErrNotFound) {
			RespondError(w, r, http.StatusNotFound, ErrCodeNotFound, "Referenced block not found during batch")
			return
		}
		s.logger.Error("batch blocks failed", "error", err)
		RespondError(w, r, http.StatusInternalServerError, ErrCodeInternal, "Failed to execute block batch")
		return
	}
	if results == nil {
		results = []domain.PlacedBlock{}
	}

	// Touch project updated_at timestamp
	project.UpdatedAt = time.Now().UTC()
	_, _ = s.store.UpdateProject(r.Context(), project)

	RespondJSON(w, http.StatusOK, map[string]any{"results": results})
}

func asInt(v any) (int, bool) {
	switch n := v.(type) {
	case float64:
		return int(n), true
	case int:
		return n, true
	case int64:
		return int(n), true
	default:
		return 0, false
	}
}
