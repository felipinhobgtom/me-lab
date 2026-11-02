package http

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/felipinho/me-lab/internal/catalog"
	"github.com/felipinho/me-lab/internal/domain"
	"github.com/felipinho/me-lab/internal/export"
	"github.com/felipinho/me-lab/internal/repository/sqlite"
	"github.com/felipinho/me-lab/internal/simulation"
)

func newIntegrationServer(t *testing.T) (*httptest.Server, string) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "integration.db")
	return newIntegrationServerAtPath(t, dbPath), dbPath
}

func newIntegrationServerAtPath(t *testing.T, dbPath string) *httptest.Server {
	t.Helper()

	db, err := sqlite.NewDB(dbPath)
	if err != nil {
		t.Fatalf("NewDB: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := sqlite.Migrate(db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	reg, err := catalog.NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	if err := sqlite.SeedCatalog(context.Background(), db, reg); err != nil {
		t.Fatalf("SeedCatalog: %v", err)
	}

	router := NewRouter(Deps{
		Logger:    slog.New(slog.NewTextHandler(io.Discard, nil)),
		Store:     sqlite.NewStore(db),
		Catalog:   reg,
		Engine:    simulation.NewEngine(),
		JWTSecret: "integration-test-secret-32-bytes-key!!",
		JWTExpiry: time.Hour,
	})

	ts := httptest.NewServer(router)
	t.Cleanup(ts.Close)
	return ts
}

func doJSON(t *testing.T, client *http.Client, method, url, token string, body any) (*http.Response, []byte) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body: %v", err)
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("http %s %s: %v", method, url, err)
	}
	defer resp.Body.Close()
	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	return resp, respBytes
}

func TestAPIIntegration_AuthOwnershipPersistenceAndOpenAPI(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "persist.db")
	srv := newIntegrationServerAtPath(t, dbPath)
	client := srv.Client()

	// 1. OpenAPI & Swagger docs endpoints
	resp, rawSpec := doJSON(t, client, http.MethodGet, srv.URL+"/openapi.json", "", nil)
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(rawSpec), `"openapi": "3.1.0"`) {
		t.Fatalf("expected OpenAPI 3.1 spec, got status=%d", resp.StatusCode)
	}
	resp, rawDocs := doJSON(t, client, http.MethodGet, srv.URL+"/docs", "", nil)
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(rawDocs), "swagger-ui") {
		t.Fatalf("expected Swagger UI HTML, got status=%d", resp.StatusCode)
	}
	resp, _ = doJSON(t, client, http.MethodGet, srv.URL+"/ready", "", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected /ready 200, got %d", resp.StatusCode)
	}

	// 2. Register User A & User B
	resp, bodyA := doJSON(t, client, http.MethodPost, srv.URL+"/api/v1/auth/register", "", map[string]string{
		"email":    "alice@melab.test",
		"password": "correct-horse-battery",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("register alice failed (%d): %s", resp.StatusCode, string(bodyA))
	}
	var authA authResponse
	_ = json.Unmarshal(bodyA, &authA)
	if authA.Token == "" {
		t.Fatal("expected JWT token for alice")
	}

	// Duplicate registration returns 409 Conflict
	resp, _ = doJSON(t, client, http.MethodPost, srv.URL+"/api/v1/auth/register", "", map[string]string{
		"email":    "alice@melab.test",
		"password": "correct-horse-battery",
	})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409 duplicate email, got %d", resp.StatusCode)
	}

	// Login works and /auth/me returns user
	resp, loginBytes := doJSON(t, client, http.MethodPost, srv.URL+"/api/v1/auth/login", "", map[string]string{
		"email":    "alice@melab.test",
		"password": "correct-horse-battery",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login failed (%d): %s", resp.StatusCode, string(loginBytes))
	}
	resp, _ = doJSON(t, client, http.MethodGet, srv.URL+"/api/v1/auth/me", authA.Token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("/auth/me failed: %d", resp.StatusCode)
	}

	// Register Bob
	resp, bodyB := doJSON(t, client, http.MethodPost, srv.URL+"/api/v1/auth/register", "", map[string]string{
		"email":    "bob@melab.test",
		"password": "bob-secure-password-123",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("register bob failed: %d", resp.StatusCode)
	}
	var authB authResponse
	_ = json.Unmarshal(bodyB, &authB)

	// 3. Catalog endpoints (profiles + URL-encoded block ID)
	resp, profBytes := doJSON(t, client, http.MethodGet, srv.URL+"/api/v1/catalog/profiles", "", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list profiles failed: %d", resp.StatusCode)
	}
	var profiles []domain.CompatibilityProfile
	_ = json.Unmarshal(profBytes, &profiles)
	if len(profiles) < 3 {
		t.Fatalf("expected >= 3 profiles, got %d", len(profiles))
	}

	resp, _ = doJSON(t, client, http.MethodGet, srv.URL+"/api/v1/catalog/blocks/ae2%3Acontroller", "", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for encoded ae2%%3Acontroller, got %d", resp.StatusCode)
	}

	// 4. Create Project under Alice & verify strict ownership isolation against Bob
	resp, projBytes := doJSON(t, client, http.MethodPost, srv.URL+"/api/v1/projects", authA.Token, map[string]any{
		"name":        "Alice Storage Grid",
		"description": "Main base AE2 1.21.1 layout",
		"profile_id":  "ae2-1.21.1-default",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create project failed (%d): %s", resp.StatusCode, string(projBytes))
	}
	var proj domain.Project
	_ = json.Unmarshal(projBytes, &proj)

	// Bob cannot view or mutate Alice's project
	resp, _ = doJSON(t, client, http.MethodGet, srv.URL+"/api/v1/projects/"+proj.ID, authB.Token, nil)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for non-owner Bob, got %d", resp.StatusCode)
	}

	// 5. Place block, rotate/update config, batch update
	resp, blkBytes := doJSON(t, client, http.MethodPost, srv.URL+"/api/v1/projects/"+proj.ID+"/blocks", authA.Token, map[string]any{
		"block_def_id": "ae2:drive",
		"x":            2,
		"y":            0,
		"z":            1,
		"facing":       "north",
		"config":       map[string]any{"priority": 25},
		"label":        "Ore Drive",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("place block failed (%d): %s", resp.StatusCode, string(blkBytes))
	}
	var placed domain.PlacedBlock
	_ = json.Unmarshal(blkBytes, &placed)

	// Rotate & update config
	resp, _ = doJSON(t, client, http.MethodPatch, srv.URL+"/api/v1/projects/"+proj.ID+"/blocks/"+placed.ID, authA.Token, map[string]any{
		"facing": "east",
		"config": map[string]any{"priority": 50},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("patch block failed: %d", resp.StatusCode)
	}

	// Close server and re-open against SAME SQLite file to verify persistence across restarts
	srv.Close()
	srv2 := newIntegrationServerAtPath(t, dbPath)
	resp, reloadBytes := doJSON(t, srv2.Client(), http.MethodGet, srv2.URL+"/api/v1/projects/"+proj.ID+"/blocks", authA.Token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("reload blocks after restart failed: %d", resp.StatusCode)
	}
	var reloaded []domain.PlacedBlock
	_ = json.Unmarshal(reloadBytes, &reloaded)
	if len(reloaded) != 1 || reloaded[0].Facing != domain.East || reloaded[0].X != 2 || reloaded[0].Z != 1 {
		t.Fatalf("unexpected reloaded block after restart: %+v", reloaded)
	}
	if prio, _ := reloaded[0].Config["priority"].(float64); int(prio) != 50 {
		t.Fatalf("expected persisted priority=50, got %v", reloaded[0].Config["priority"])
	}
}

func TestAPIIntegration_EndToEndAutocraftingAndFaultFixtures(t *testing.T) {
	srv, _ := newIntegrationServer(t)
	client := srv.Client()

	_, regBody := doJSON(t, client, http.MethodPost, srv.URL+"/api/v1/auth/register", "", map[string]string{
		"email":    "crafter@melab.test",
		"password": "autocrafting-pass-123",
	})
	var authRes authResponse
	_ = json.Unmarshal(regBody, &authRes)

	// =========================================================================
	// Fixture 1: Valid Controller-Based Autocrafting Network
	// =========================================================================
	_, projBody := doJSON(t, client, http.MethodPost, srv.URL+"/api/v1/projects", authRes.Token, map[string]any{
		"name":       "Valid Autocrafting Acceptance Fixture",
		"profile_id": "ae2-1.21.1-default",
	})
	var validProj domain.Project
	_ = json.Unmarshal(projBody, &validProj)

	fixtureOps := []map[string]any{
		{"operation": "create", "data": map[string]any{"block_def_id": "ae2:energy_acceptor", "x": -1, "y": 0, "z": 0, "config": map[string]any{"fe_input_per_tick": 800}}},
		{"operation": "create", "data": map[string]any{"block_def_id": "ae2:energy_cell", "x": -1, "y": 1, "z": 0, "config": map[string]any{"initial_charge_pct": 100}}},
		{"operation": "create", "data": map[string]any{"block_def_id": "ae2:controller", "x": 0, "y": 0, "z": 0}},
		{"operation": "create", "data": map[string]any{"block_def_id": "ae2:dense_smart_cable", "x": 1, "y": 0, "z": 0}},
		{"operation": "create", "data": map[string]any{"block_def_id": "ae2:dense_smart_cable", "x": 2, "y": 0, "z": 0}},
		{"operation": "create", "data": map[string]any{"block_def_id": "ae2:smart_cable", "x": 3, "y": 0, "z": 0}},
		{"operation": "create", "data": map[string]any{"block_def_id": "ae2:drive", "x": 2, "y": 0, "z": 1, "config": map[string]any{
			"priority": 10,
			"cells": []map[string]any{
				{
					"tier":  "4k",
					"items": []map[string]any{{"item_id": "minecraft:oak_log", "quantity": 64}},
				},
			},
		}}},
		{"operation": "create", "data": map[string]any{"block_def_id": "ae2:crafting_terminal", "x": 3, "y": 1, "z": 0}},
		{"operation": "create", "data": map[string]any{"block_def_id": "ae2:crafting_storage_4k", "x": 1, "y": 0, "z": -1}},
		{"operation": "create", "data": map[string]any{"block_def_id": "ae2:crafting_co_processor", "x": 1, "y": 1, "z": -1}},
		{"operation": "create", "data": map[string]any{"block_def_id": "ae2:pattern_provider", "x": 4, "y": 0, "z": 0, "config": map[string]any{
			"priority": 5,
			"variant":  "normal",
			"patterns": []map[string]any{
				{
					"id":           "pat-planks",
					"pattern_type": "crafting",
					"inputs":       []map[string]any{{"item_id": "minecraft:oak_log", "quantity": 1}},
					"outputs":      []map[string]any{{"item_id": "minecraft:oak_planks", "quantity": 4}},
				},
				{
					"id":           "pat-chest",
					"pattern_type": "crafting",
					"inputs":       []map[string]any{{"item_id": "minecraft:oak_planks", "quantity": 8}},
					"outputs":      []map[string]any{{"item_id": "minecraft:chest", "quantity": 1}},
				},
			},
		}}},
		{"operation": "create", "data": map[string]any{"block_def_id": "ae2:molecular_assembler", "x": 5, "y": 0, "z": 0}},
		{"operation": "create", "data": map[string]any{"block_def_id": "ae2:molecular_assembler", "x": 4, "y": 1, "z": 0}},
	}

	resp, batchBody := doJSON(t, client, http.MethodPost, srv.URL+"/api/v1/projects/"+validProj.ID+"/blocks/batch", authRes.Token, map[string]any{
		"replace_all": true,
		"operations":  fixtureOps,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("batch fixture failed (%d): %s", resp.StatusCode, string(batchBody))
	}

	// Validate valid fixture
	resp, valBytes := doJSON(t, client, http.MethodPost, srv.URL+"/api/v1/projects/"+validProj.ID+"/validate", authRes.Token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("validate failed (%d): %s", resp.StatusCode, string(valBytes))
	}
	var valRes domain.ValidationResult
	_ = json.Unmarshal(valBytes, &valRes)
	if !valRes.Valid {
		t.Fatalf("expected valid=true on acceptance fixture, got diagnostics: %+v", valRes.Diagnostics)
	}
	if valRes.Summary.ChannelsUsed != 4 {
		// Drive (1) + Crafting Terminal (1) + Crafting CPU (1) + Pattern Provider (1) = 4 channels
		t.Fatalf("expected 4 channels used, got %d", valRes.Summary.ChannelsUsed)
	}

	// Start Simulation requesting 2x minecraft:chest (requires 16 oak_planks -> 4 oak_log crafts + 2 chest crafts)
	resp, simBytes := doJSON(t, client, http.MethodPost, srv.URL+"/api/v1/projects/"+validProj.ID+"/simulations", authRes.Token, map[string]any{
		"seed": 42,
		"crafting_requests": []map[string]any{
			{"item_id": "minecraft:chest", "quantity": 2, "source": "terminal"},
		},
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create simulation failed (%d): %s", resp.StatusCode, string(simBytes))
	}
	var simRun domain.SimulationRun
	_ = json.Unmarshal(simBytes, &simRun)

	// Test pause & resume
	resp, pauseBytes := doJSON(t, client, http.MethodPost, srv.URL+"/api/v1/simulations/"+simRun.ID+"/pause", authRes.Token, nil)
	var pausedRun domain.SimulationRun
	_ = json.Unmarshal(pauseBytes, &pausedRun)
	if resp.StatusCode != http.StatusOK || pausedRun.Status != domain.SimStatusPaused {
		t.Fatalf("expected paused status, got %d / %s", resp.StatusCode, pausedRun.Status)
	}

	resp, resumeBytes := doJSON(t, client, http.MethodPost, srv.URL+"/api/v1/simulations/"+simRun.ID+"/resume", authRes.Token, nil)
	var resumedRun domain.SimulationRun
	_ = json.Unmarshal(resumeBytes, &resumedRun)
	if resp.StatusCode != http.StatusOK || resumedRun.Status != domain.SimStatusRunning {
		t.Fatalf("expected running status on resume, got %d / %s", resp.StatusCode, resumedRun.Status)
	}

	// Step simulation until completion (max 30 ticks)
	var stepRes domain.SimulationStepResult
	resp, stepBytes := doJSON(t, client, http.MethodPost, srv.URL+"/api/v1/simulations/"+simRun.ID+"/step", authRes.Token, map[string]any{
		"ticks": 25,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("step simulation failed (%d): %s", resp.StatusCode, string(stepBytes))
	}
	_ = json.Unmarshal(stepBytes, &stepRes)
	if stepRes.State.Status != domain.SimStatusCompleted {
		t.Fatalf("expected simulation to complete within 25 ticks, got status=%s summary=%+v", stepRes.State.Status, stepRes.State.Summary)
	}

	// Retrieve persisted events and verify full crafting lifecycle event types
	resp, evBytes := doJSON(t, client, http.MethodGet, srv.URL+"/api/v1/simulations/"+simRun.ID+"/events?page=1&page_size=100", authRes.Token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list events failed: %d", resp.StatusCode)
	}
	var evPage struct {
		Data []domain.SimulationEvent `json:"data"`
	}
	_ = json.Unmarshal(evBytes, &evPage)
	seenTypes := map[string]bool{}
	for _, ev := range evPage.Data {
		seenTypes[ev.EventType] = true
	}
	for _, reqType := range []string{"job_created", "ingredient_reserved", "provider_dispatched", "assembler_started", "craft_completed", "output_returned", "job_completed"} {
		if !seenTypes[reqType] {
			t.Fatalf("missing expected simulation event %q in timeline (got %v)", reqType, seenTypes)
		}
	}

	// Export JSON blueprint & Markdown build sheet
	resp, bpBytes := doJSON(t, client, http.MethodGet, srv.URL+"/api/v1/projects/"+validProj.ID+"/export/json", authRes.Token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("export json failed: %d", resp.StatusCode)
	}
	var bp export.Blueprint
	if err := json.Unmarshal(bpBytes, &bp); err != nil || bp.SchemaVersion != "1.0" || len(bp.Blocks) != len(fixtureOps) {
		t.Fatalf("unexpected exported blueprint: err=%v version=%s blocks=%d", err, bp.SchemaVersion, len(bp.Blocks))
	}

	resp, sheetBytes := doJSON(t, client, http.MethodGet, srv.URL+"/api/v1/projects/"+validProj.ID+"/export/build-sheet", authRes.Token, nil)
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(sheetBytes), "Bill of Materials") {
		t.Fatalf("export build sheet failed (%d): %s", resp.StatusCode, string(sheetBytes))
	}

	// Reset simulation back to Tick 0
	resp, resetBytes := doJSON(t, client, http.MethodPost, srv.URL+"/api/v1/simulations/"+simRun.ID+"/reset", authRes.Token, nil)
	var resetRun domain.SimulationRun
	_ = json.Unmarshal(resetBytes, &resetRun)
	if resp.StatusCode != http.StatusOK || resetRun.CurrentTick != 0 {
		t.Fatalf("expected reset to tick 0, got status=%d tick=%d", resp.StatusCode, resetRun.CurrentTick)
	}

	// =========================================================================
	// Fixture 2: Deliberate Fault Network (Ad-Hoc Channel Overload + Unsupported Block)
	// =========================================================================
	_, faultProjBody := doJSON(t, client, http.MethodPost, srv.URL+"/api/v1/projects", authRes.Token, map[string]any{
		"name":       "Faulty Ad-Hoc Overload Network",
		"profile_id": "ae2-1.21.1-default",
	})
	var faultProj domain.Project
	_ = json.Unmarshal(faultProjBody, &faultProj)

	faultOps := []map[string]any{
		{"operation": "create", "data": map[string]any{"block_def_id": "ae2:energy_acceptor", "x": -1, "y": 0, "z": 0, "config": map[string]any{"fe_input_per_tick": 800}}},
		{"operation": "create", "data": map[string]any{"block_def_id": "ae2:p2p_tunnel", "x": -1, "y": 0, "z": 1}},
	}
	for i := range 10 {
		faultOps = append(faultOps,
			map[string]any{"operation": "create", "data": map[string]any{"block_def_id": "ae2:smart_cable", "x": i, "y": 0, "z": 0}},
			map[string]any{"operation": "create", "data": map[string]any{"block_def_id": "ae2:drive", "x": i, "y": 1, "z": 0}},
		)
	}
	_, _ = doJSON(t, client, http.MethodPost, srv.URL+"/api/v1/projects/"+faultProj.ID+"/blocks/batch", authRes.Token, map[string]any{
		"replace_all": true,
		"operations":  faultOps,
	})

	resp, faultValBytes := doJSON(t, client, http.MethodPost, srv.URL+"/api/v1/projects/"+faultProj.ID+"/validate", authRes.Token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("fault validate failed: %d", resp.StatusCode)
	}
	var faultVal domain.ValidationResult
	_ = json.Unmarshal(faultValBytes, &faultVal)
	if faultVal.Valid {
		t.Fatal("expected faulty network to fail validation")
	}

	codes := map[string]int{}
	for _, d := range faultVal.Diagnostics {
		codes[d.Code]++
		if len(d.AffectedBlocks) == 0 {
			t.Fatalf("expected affected_blocks populated on diagnostic %s", d.Code)
		}
	}
	if codes["NETWORK_NO_CONTROLLER"] == 0 || codes["DEVICE_NO_CHANNEL"] == 0 || codes["UNSUPPORTED_MECHANIC"] == 0 {
		t.Fatalf("expected NETWORK_NO_CONTROLLER, DEVICE_NO_CHANNEL, and UNSUPPORTED_MECHANIC diagnostics, got %v", codes)
	}
}
