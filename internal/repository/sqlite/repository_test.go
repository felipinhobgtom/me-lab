package sqlite

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/felipinho/me-lab/internal/catalog"
	"github.com/felipinho/me-lab/internal/domain"
)

func setupTestStore(t *testing.T) (*Store, *catalog.Registry) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "melab_test.db")
	db, err := NewDB(dbPath)
	if err != nil {
		t.Fatalf("NewDB failed: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	reg, err := catalog.NewRegistry()
	if err != nil {
		t.Fatalf("catalog.NewRegistry failed: %v", err)
	}

	if err := SeedCatalog(context.Background(), db, reg); err != nil {
		t.Fatalf("SeedCatalog failed: %v", err)
	}

	return NewStore(db), reg
}

func TestMigrateAndIdempotentSeedCatalog(t *testing.T) {
	t.Parallel()

	store, reg := setupTestStore(t)
	ctx := context.Background()

	// Re-running SeedCatalog must upsert cleanly without errors or duplicates.
	if err := SeedCatalog(ctx, store.DB(), reg); err != nil {
		t.Fatalf("second SeedCatalog failed: %v", err)
	}

	var profileCount, blockDefCount int
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM compatibility_profiles`).Scan(&profileCount); err != nil {
		t.Fatalf("count compatibility_profiles: %v", err)
	}
	if err := store.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM block_definitions`).Scan(&blockDefCount); err != nil {
		t.Fatalf("count block_definitions: %v", err)
	}

	if profileCount != len(reg.Profiles()) {
		t.Errorf("compatibility_profiles count = %d, want %d", profileCount, len(reg.Profiles()))
	}
	if blockDefCount != len(reg.AllBlocks()) {
		t.Errorf("block_definitions count = %d, want %d", blockDefCount, len(reg.AllBlocks()))
	}
}

func TestUserCRUDAndConflicts(t *testing.T) {
	t.Parallel()

	store, _ := setupTestStore(t)
	ctx := context.Background()

	created, err := store.CreateUser(ctx, domain.User{
		Email:        "alice@melab.dev",
		PasswordHash: "$argon2id$v=19$m=65536,t=3,p=2$c2FsdA$aGFzaA",
	})
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}
	if created.ID == "" {
		t.Fatal("expected generated User ID")
	}

	if _, err := store.CreateUser(ctx, domain.User{
		Email:        "alice@melab.dev",
		PasswordHash: "duplicate",
	}); !errors.Is(err, ErrConflict) {
		t.Fatalf("expected ErrConflict on duplicate email, got %v", err)
	}

	byEmail, err := store.GetUserByEmail(ctx, "alice@melab.dev")
	if err != nil {
		t.Fatalf("GetUserByEmail failed: %v", err)
	}
	if byEmail.ID != created.ID || byEmail.PasswordHash != created.PasswordHash {
		t.Fatalf("GetUserByEmail mismatch: %+v vs %+v", byEmail, created)
	}

	byID, err := store.GetUserByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetUserByID failed: %v", err)
	}
	if byID.Email != "alice@melab.dev" {
		t.Fatalf("GetUserByID Email = %q, want alice@melab.dev", byID.Email)
	}

	if _, err := store.GetUserByEmail(ctx, "missing@melab.dev"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound for missing email, got %v", err)
	}
	if _, err := store.GetUserByID(ctx, "missing-id"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound for missing ID, got %v", err)
	}
}

func TestProjectIsolationAndCRUD(t *testing.T) {
	t.Parallel()

	store, _ := setupTestStore(t)
	ctx := context.Background()

	alice, err := store.CreateUser(ctx, domain.User{Email: "alice@melab.dev", PasswordHash: "hash1"})
	if err != nil {
		t.Fatalf("create alice: %v", err)
	}
	bob, err := store.CreateUser(ctx, domain.User{Email: "bob@melab.dev", PasswordHash: "hash2"})
	if err != nil {
		t.Fatalf("create bob: %v", err)
	}

	p1, err := store.CreateProject(ctx, domain.Project{
		UserID:      alice.ID,
		Name:        "Main Base Controller",
		Description: "32-channel core",
		ProfileID:   catalog.DefaultProfileID,
		Settings:    map[string]any{"grid_size": float64(16)},
	})
	if err != nil {
		t.Fatalf("CreateProject p1: %v", err)
	}
	_, err = store.CreateProject(ctx, domain.Project{
		UserID:    alice.ID,
		Name:      "Autocrafting Tower",
		ProfileID: "ae2-1.21.1-x2",
	})
	if err != nil {
		t.Fatalf("CreateProject p2: %v", err)
	}
	_, err = store.CreateProject(ctx, domain.Project{
		UserID:    bob.ID,
		Name:      "Bob Sandbox",
		ProfileID: "ae2-1.21.1-infinite",
	})
	if err != nil {
		t.Fatalf("CreateProject bob: %v", err)
	}

	aliceProjects, aliceTotal, err := store.ListProjectsByOwner(ctx, alice.ID, 10, 0)
	if err != nil {
		t.Fatalf("ListProjectsByOwner alice: %v", err)
	}
	if aliceTotal != 2 || len(aliceProjects) != 2 {
		t.Fatalf("expected 2 alice projects, got total=%d len=%d", aliceTotal, len(aliceProjects))
	}

	bobProjects, bobTotal, err := store.ListProjectsByOwner(ctx, bob.ID, 10, 0)
	if err != nil {
		t.Fatalf("ListProjectsByOwner bob: %v", err)
	}
	if bobTotal != 1 || len(bobProjects) != 1 || bobProjects[0].Name != "Bob Sandbox" {
		t.Fatalf("unexpected bob projects: total=%d %+v", bobTotal, bobProjects)
	}

	updated, err := store.UpdateProject(ctx, domain.Project{
		ID:          p1.ID,
		Name:        "Main Base Controller v2",
		Description: "Expanded 7x7x7 controller",
		ProfileID:   "ae2-1.21.1-x2",
		Settings:    map[string]any{"grid_size": float64(32)},
	})
	if err != nil {
		t.Fatalf("UpdateProject: %v", err)
	}
	if updated.Name != "Main Base Controller v2" || updated.ProfileID != "ae2-1.21.1-x2" {
		t.Fatalf("unexpected updated project: %+v", updated)
	}

	if err := store.DeleteProject(ctx, p1.ID); err != nil {
		t.Fatalf("DeleteProject: %v", err)
	}
	if _, err := store.GetProject(ctx, p1.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestBlockCRUDPatternsInventoryAndBatchRollback(t *testing.T) {
	t.Parallel()

	store, _ := setupTestStore(t)
	ctx := context.Background()

	u, err := store.CreateUser(ctx, domain.User{Email: "builder@melab.dev", PasswordHash: "hash"})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	proj, err := store.CreateProject(ctx, domain.Project{
		UserID:    u.ID,
		Name:      "Voxel Test",
		ProfileID: catalog.DefaultProfileID,
	})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	ctrl, err := store.CreateBlock(ctx, domain.PlacedBlock{
		ProjectID:  proj.ID,
		BlockDefID: "ae2:controller",
		X:          0,
		Y:          0,
		Z:          0,
		Facing:     domain.North,
		Label:      "Core",
	})
	if err != nil {
		t.Fatalf("CreateBlock controller: %v", err)
	}

	// Coordinate collision must return ErrConflict.
	if _, err := store.CreateBlock(ctx, domain.PlacedBlock{
		ProjectID:  proj.ID,
		BlockDefID: "ae2:fluix_cable",
		X:          0,
		Y:          0,
		Z:          0,
	}); !errors.Is(err, ErrConflict) {
		t.Fatalf("expected ErrConflict on occupied coordinate, got %v", err)
	}

	// Atomic batch that succeeds.
	blocks, err := store.ApplyBlockBatch(ctx, proj.ID, []BatchOperation{
		{
			Op:         BatchOpCreate,
			BlockDefID: "ae2:fluix_cable",
			X:          1,
			Y:          0,
			Z:          0,
			Config:     map[string]any{"color": "blue"},
		},
		{
			Op:         BatchOpCreate,
			BlockDefID: "ae2:drive",
			X:          2,
			Y:          0,
			Z:          0,
			Facing:     domain.East,
			Config:     map[string]any{"priority": float64(10)},
		},
	}, false)
	if err != nil {
		t.Fatalf("ApplyBlockBatch valid failed: %v", err)
	}
	if len(blocks) != 3 {
		t.Fatalf("expected 3 blocks after batch, got %d", len(blocks))
	}

	// Atomic batch with a second op colliding on (0,0,0) must roll back the first op too!
	_, err = store.ApplyBlockBatch(ctx, proj.ID, []BatchOperation{
		{
			Op:         BatchOpCreate,
			BlockDefID: "ae2:energy_acceptor",
			X:          0,
			Y:          1,
			Z:          0,
		},
		{
			Op:         BatchOpCreate,
			BlockDefID: "ae2:terminal",
			X:          0,
			Y:          0,
			Z:          0, // already occupied by controller!
		},
	}, false)
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("expected ErrConflict on batch collision, got %v", err)
	}

	afterRollback, err := store.ListBlocks(ctx, proj.ID)
	if err != nil {
		t.Fatalf("ListBlocks after rollback: %v", err)
	}
	if len(afterRollback) != 3 {
		t.Fatalf("expected rollback to preserve 3 blocks, got %d", len(afterRollback))
	}

	// Single update & delete.
	ctrl.Label = "Primary Controller"
	updatedCtrl, err := store.UpdateBlock(ctx, ctrl)
	if err != nil || updatedCtrl.Label != "Primary Controller" {
		t.Fatalf("UpdateBlock failed: %+v, err=%v", updatedCtrl, err)
	}
	if err := store.DeleteBlock(ctx, proj.ID, blocks[2].ID); err != nil {
		t.Fatalf("DeleteBlock failed: %v", err)
	}

	// ReplacePatterns & ReplaceInventory round-trip.
	savedPats, err := store.ReplacePatterns(ctx, proj.ID, []domain.ProjectPattern{
		{
			PatternType: "crafting",
			Inputs:      []domain.ItemStack{{ItemID: "minecraft:oak_log", Quantity: 1}},
			Outputs:     []domain.ItemStack{{ItemID: "minecraft:oak_planks", Quantity: 4}},
		},
	})
	if err != nil || len(savedPats) != 1 {
		t.Fatalf("ReplacePatterns failed: %+v, err=%v", savedPats, err)
	}
	listedPats, err := store.ListPatterns(ctx, proj.ID)
	if err != nil || len(listedPats) != 1 || listedPats[0].Outputs[0].Quantity != 4 {
		t.Fatalf("ListPatterns mismatch: %+v, err=%v", listedPats, err)
	}

	savedInv, err := store.ReplaceInventory(ctx, proj.ID, []domain.ProjectInventoryEntry{
		{
			ItemID:          "minecraft:oak_log",
			Quantity:        128,
			StorageLocation: map[string]any{"drive_slot": float64(0)},
		},
	})
	if err != nil || len(savedInv) != 1 {
		t.Fatalf("ReplaceInventory failed: %+v, err=%v", savedInv, err)
	}
	listedInv, err := store.ListInventory(ctx, proj.ID)
	if err != nil || len(listedInv) != 1 || listedInv[0].Quantity != 128 {
		t.Fatalf("ListInventory mismatch: %+v, err=%v", listedInv, err)
	}
}

func TestSimulationRunLifecyclePersistence(t *testing.T) {
	t.Parallel()

	store, _ := setupTestStore(t)
	ctx := context.Background()

	u, err := store.CreateUser(ctx, domain.User{Email: "sim@melab.dev", PasswordHash: "hash"})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	proj, err := store.CreateProject(ctx, domain.Project{
		UserID:    u.ID,
		Name:      "Sim Project",
		ProfileID: catalog.DefaultProfileID,
	})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	run := domain.SimulationRun{
		ID:          "sim-run-1",
		ProjectID:   proj.ID,
		Status:      domain.SimStatusRunning,
		Seed:        42,
		CurrentTick: 0,
		Summary: domain.SimulationSummary{
			Networks:          1,
			OnlineNetworks:    1,
			ChannelsUsed:      2,
			ChannelsAvailable: 32,
			EnergyStored:      8000,
			EnergyCapacity:    8000,
			DiagnosticsCount:  1,
		},
		Overlay: &domain.TopologyOverlay{
			Networks: []domain.NetworkSummaryOverlay{{ID: "net-1", Online: true}},
		},
		StartedAt: time.Now().UTC().Truncate(time.Second),
	}

	initialEvents := []domain.SimulationEvent{
		{
			Sequence:  1,
			Tick:      0,
			EventType: "NETWORK_BOOT",
			Payload:   map[string]any{"network_id": "net-1"},
		},
	}
	initialDiags := []domain.Diagnostic{
		{
			Code:             "UNSUPPORTED_MECHANIC",
			Severity:         domain.SeverityWarning,
			Message:          "P2P tunnel is unsupported in MVP",
			AffectedBlocks:   []domain.AffectedBlock{{ID: "blk-1", X: 1, Y: 0, Z: 0}},
			Remediation:      "Remove P2P tunnel or replace with direct dense cable.",
			BlocksSimulation: false,
		},
	}

	if err := store.CreateSimulationRun(ctx, run, []byte(`{"blocks":1}`), []byte(`{"tick":0}`), initialEvents, initialDiags); err != nil {
		t.Fatalf("CreateSimulationRun failed: %v", err)
	}

	gotRun, snapBytes, stateBytes, err := store.GetSimulationRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("GetSimulationRun failed: %v", err)
	}
	if gotRun.Status != domain.SimStatusRunning || gotRun.Seed != 42 || string(snapBytes) != `{"blocks":1}` || string(stateBytes) != `{"tick":0}` {
		t.Fatalf("unexpected GetSimulationRun result: run=%+v snap=%s state=%s", gotRun, snapBytes, stateBytes)
	}
	if gotRun.Overlay == nil || len(gotRun.Overlay.Networks) != 1 {
		t.Fatalf("expected persisted overlay, got %+v", gotRun.Overlay)
	}

	// Step simulation state forward: append event 2 and replace diagnostics.
	finished := time.Now().UTC().Truncate(time.Second)
	run.CurrentTick = 4
	run.Status = domain.SimStatusCompleted
	run.FinishedAt = &finished
	run.Summary.CompletedJobs = 1
	run.Summary.DiagnosticsCount = 0

	stepEvents := []domain.SimulationEvent{
		{
			Sequence:  2,
			Tick:      4,
			EventType: "CRAFT_COMPLETED",
			Payload:   map[string]any{"item_id": "minecraft:oak_planks", "quantity": float64(4)},
		},
	}
	if err := store.UpdateSimulationRunState(ctx, run, []byte(`{"tick":4}`), stepEvents, nil); err != nil {
		t.Fatalf("UpdateSimulationRunState failed: %v", err)
	}

	events, totalEvents, err := store.ListSimulationEvents(ctx, run.ID, 10, 0)
	if err != nil {
		t.Fatalf("ListSimulationEvents failed: %v", err)
	}
	if totalEvents != 2 || len(events) != 2 || events[1].EventType != "CRAFT_COMPLETED" {
		t.Fatalf("unexpected events after step: total=%d %+v", totalEvents, events)
	}

	diags, err := store.ListSimulationDiagnostics(ctx, run.ID)
	if err != nil {
		t.Fatalf("ListSimulationDiagnostics failed: %v", err)
	}
	if len(diags) != 0 {
		t.Fatalf("expected replaced diagnostics to be empty, got %+v", diags)
	}

	// Reset simulation back to tick 0.
	run.CurrentTick = 0
	run.Status = domain.SimStatusPaused
	run.FinishedAt = nil
	if err := store.ResetSimulationRunState(ctx, run, []byte(`{"tick":0}`), initialEvents, initialDiags); err != nil {
		t.Fatalf("ResetSimulationRunState failed: %v", err)
	}

	eventsAfterReset, totalAfterReset, err := store.ListSimulationEvents(ctx, run.ID, 10, 0)
	if err != nil || totalAfterReset != 1 || len(eventsAfterReset) != 1 {
		t.Fatalf("expected 1 event after reset, got total=%d len=%d err=%v", totalAfterReset, len(eventsAfterReset), err)
	}
	diagsAfterReset, err := store.ListSimulationDiagnostics(ctx, run.ID)
	if err != nil || len(diagsAfterReset) != 1 || diagsAfterReset[0].Code != "UNSUPPORTED_MECHANIC" {
		t.Fatalf("expected 1 diagnostic after reset, got %+v err=%v", diagsAfterReset, err)
	}

	runs, runTotal, err := store.ListSimulationRuns(ctx, proj.ID, 10, 0)
	if err != nil || runTotal != 1 || len(runs) != 1 {
		t.Fatalf("ListSimulationRuns mismatch: total=%d len=%d err=%v", runTotal, len(runs), err)
	}
}
