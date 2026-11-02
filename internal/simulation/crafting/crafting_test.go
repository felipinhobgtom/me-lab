package crafting

import (
	"testing"

	"github.com/felipinho/me-lab/internal/domain"
	"github.com/felipinho/me-lab/internal/simulation/channels"
	"github.com/felipinho/me-lab/internal/simulation/diagnostics"
	"github.com/felipinho/me-lab/internal/simulation/energy"
	"github.com/felipinho/me-lab/internal/simulation/network"
	"github.com/felipinho/me-lab/internal/simulation/storage"
	"github.com/felipinho/me-lab/internal/simulation/world"
)

func testCraftProfile() domain.CompatibilityProfile {
	return domain.CompatibilityProfile{
		ID: "ae2-1.21.1-default",
		Config: domain.ProfileConfig{
			NormalCableChannels:       8,
			DenseCableChannels:        32,
			ControllerFaceChannels:    32,
			AdHocChannelLimit:         8,
			ControllerMaxSpan:         7,
			ControllerEnergyPerBlock:  6,
			ControllerEnergyBuffer:    8000,
			NodeBaseEnergyBuffer:      25,
			ChannelHopEnergyCost:      0.0078125,
			FEPerAE:                   2,
			MolecularAssemblerTicks:   4,
			MolecularAssemblerActiveE: 20,
			ExternalMachineTicks:      8,
			ItemsPerByte:              8,
			MaxCellTypes:              63,
		},
	}
}

type craftEnv struct {
	prof   domain.CompatibilityProfile
	w      *world.World
	nets   []*network.Network
	alloc  *channels.Allocation
	energy []*energy.NetworkEnergy
	store  *storage.State
	craft  *State
	seq    int
	events []domain.SimulationEvent
	diags  []domain.Diagnostic
}

func setupCraftEnv(t *testing.T, blocks []domain.PlacedBlock, reqs []domain.CraftingRequest) *craftEnv {
	t.Helper()
	prof := testCraftProfile()
	seq := 1
	w := world.Build("sim-c", blocks, nil)
	nets, netDiags := network.Discover("sim-c", w, prof)
	alloc := channels.Allocate("sim-c", w, nets, prof)
	eStates, eDiags := energy.Initialize("sim-c", nets, alloc, prof)
	store, sDiags := storage.Initialize("sim-c", nets, alloc, eStates, nil, prof)
	cs, evs, cDiags := Initialize("sim-c", w, nets, alloc, eStates, store, nil, reqs, prof, EventEmitter{
		SimulationID: "sim-c",
		NextSequence: &seq,
	})

	allDiags := append([]domain.Diagnostic{}, w.Diagnostics...)
	allDiags = append(allDiags, netDiags...)
	allDiags = append(allDiags, alloc.Diagnostics...)
	allDiags = append(allDiags, eDiags...)
	allDiags = append(allDiags, sDiags...)
	allDiags = append(allDiags, cDiags...)

	return &craftEnv{
		prof:   prof,
		w:      w,
		nets:   nets,
		alloc:  alloc,
		energy: eStates,
		store:  store,
		craft:  cs,
		seq:    seq,
		events: evs,
		diags:  diagnostics.DeduplicateAndSort(allDiags),
	}
}

func (e *craftEnv) step(tick int) ([]domain.SimulationEvent, []domain.Diagnostic) {
	netsByID := make(map[string]*network.Network, len(e.nets))
	for _, n := range e.nets {
		netsByID[n.ID] = n
	}
	eDiags := energy.StepEnergy("sim-c", e.energy, netsByID, e.craft.ActiveAssemblersByNetwork(), e.alloc, e.prof)
	e.store.SyncOnlineStatus(e.energy, e.alloc.HasChannelByBlockID)
	e.craft.SyncOnlineStatus(e.energy, e.alloc.HasChannelByBlockID)
	evs, cDiags := e.craft.StepTick("sim-c", tick, e.energy, e.store, EventEmitter{
		SimulationID: "sim-c",
		NextSequence: &e.seq,
	})
	e.events = append(e.events, evs...)
	return evs, append(eDiags, cDiags...)
}

func hasDiagCode(diags []domain.Diagnostic, code string) bool {
	for _, d := range diags {
		if d.Code == code {
			return true
		}
	}
	return false
}

func hasEventType(events []domain.SimulationEvent, evType string) bool {
	for _, ev := range events {
		if ev.EventType == evType {
			return true
		}
	}
	return false
}

func TestEndToEndMultiTierCraftWithCoProcessorParallelism(t *testing.T) {
	// Acceptance fixture: oak_log -> oak_planks (1->4) -> chest (8->1), request 2 chests!
	// Needs 16 oak_planks = 4 batches of planks, then 2 batches of chests = 6 batches total.
	// With 1 Co-Processor (2 dispatches/tick) and 2 Molecular Assemblers (4 ticks/craft):
	// - Tick 1: dispatches 2 planks batches in parallel
	// - Tick 5: 2 planks batches finish -> dispatches remaining 2 planks batches in parallel
	// - Tick 9: remaining 2 planks batches finish (16 planks ready!) -> dispatches 2 chest batches in parallel
	// - Tick 13: 2 chest batches finish -> 2 chests inserted into ME Storage -> job_completed!
	blocks := []domain.PlacedBlock{
		{ID: "ea", BlockDefID: "ae2:energy_acceptor", X: -1, Y: 0, Z: 0, Config: map[string]any{"fe_input_per_tick": 800}},
		{ID: "ec", BlockDefID: "ae2:energy_cell", X: -1, Y: 1, Z: 0, Config: map[string]any{"initial_charge_pct": 100}},
		{ID: "ctrl", BlockDefID: "ae2:controller", X: 0, Y: 0, Z: 0},
		{ID: "dc1", BlockDefID: "ae2:dense_smart_cable", X: 1, Y: 0, Z: 0},
		{ID: "dc2", BlockDefID: "ae2:dense_smart_cable", X: 2, Y: 0, Z: 0},
		{ID: "sc1", BlockDefID: "ae2:smart_cable", X: 3, Y: 0, Z: 0},
		{ID: "sc2", BlockDefID: "ae2:smart_cable", X: 3, Y: 0, Z: 1},
		{
			ID: "drive", BlockDefID: "ae2:drive", X: 2, Y: 0, Z: 1,
			Config: map[string]any{
				"priority": 10,
				"cells": []map[string]any{
					{
						"tier": "4k",
						"items": []map[string]any{
							{"item_id": "minecraft:oak_log", "quantity": 64},
						},
					},
				},
			},
		},
		{ID: "ct", BlockDefID: "ae2:crafting_terminal", X: 3, Y: 1, Z: 0},
		{ID: "cpu-s", BlockDefID: "ae2:crafting_storage_4k", X: 1, Y: 0, Z: -1},
		{ID: "cpu-c", BlockDefID: "ae2:crafting_co_processor", X: 1, Y: 1, Z: -1},
		{
			ID: "pp", BlockDefID: "ae2:pattern_provider", X: 4, Y: 0, Z: 0,
			Config: map[string]any{
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
			},
		},
		{ID: "ma1", BlockDefID: "ae2:molecular_assembler", X: 5, Y: 0, Z: 0},
		{ID: "ma2", BlockDefID: "ae2:molecular_assembler", X: 4, Y: 1, Z: 0},
	}

	env := setupCraftEnv(t, blocks, []domain.CraftingRequest{
		{ItemID: "minecraft:chest", Quantity: 2},
	})
	if len(env.diags) != 0 {
		t.Fatalf("expected 0 init diagnostics, got %+v", env.diags)
	}
	if !hasEventType(env.events, "job_created") || !hasEventType(env.events, "ingredient_reserved") {
		t.Fatalf("expected job_created and ingredient_reserved on init, got %+v", env.events)
	}
	// 4 oak_log reserved -> 60 left in storage immediately.
	if got := env.store.AvailableMap("net-1")["minecraft:oak_log"]; got != 60 {
		t.Fatalf("expected 60 oak_log remaining after reserving 4, got %d", got)
	}

	// Tick 1: Co-processor dispatches 2 parallel batches across ma1 and ma2!
	t1Evs, _ := env.step(1)
	dispatchedT1 := 0
	for _, ev := range t1Evs {
		if ev.EventType == "provider_dispatched" {
			dispatchedT1++
		}
	}
	if dispatchedT1 != 2 {
		t.Fatalf("expected 2 parallel batch dispatches on tick 1 via co-processor, got %d", dispatchedT1)
	}

	for tick := 2; tick <= 13; tick++ {
		env.step(tick)
	}

	if env.craft.Jobs[0].Status != "completed" {
		t.Fatalf("expected job completed by tick 13, got status=%s completed=%d/%d",
			env.craft.Jobs[0].Status, env.craft.Jobs[0].CompletedCrafts, env.craft.Jobs[0].TotalBatches)
	}
	if got := env.store.AvailableMap("net-1")["minecraft:chest"]; got != 2 {
		t.Fatalf("expected 2 minecraft:chest in online ME Storage, got %d", got)
	}
	for _, expectedEv := range []string{
		"job_created", "ingredient_reserved", "provider_dispatched",
		"assembler_started", "craft_completed", "output_returned", "job_completed",
	} {
		if !hasEventType(env.events, expectedEv) {
			t.Fatalf("missing expected event %s", expectedEv)
		}
	}
}

func TestBlockingModeAndExternalProcessingMachine(t *testing.T) {
	blocks := []domain.PlacedBlock{
		{ID: "creative", BlockDefID: "ae2:creative_energy_cell", X: -1, Y: 0, Z: 0},
		{ID: "ctrl", BlockDefID: "ae2:controller", X: 0, Y: 0, Z: 0},
		{
			ID: "drive", BlockDefID: "ae2:drive", X: 1, Y: 0, Z: 0,
			Config: map[string]any{
				"cells": []map[string]any{
					{"tier": "4k", "items": []map[string]any{{"item_id": "minecraft:raw_iron", "quantity": 10}}},
				},
			},
		},
		{ID: "cpu-s", BlockDefID: "ae2:crafting_storage_4k", X: 0, Y: 1, Z: 0},
		{ID: "cpu-c", BlockDefID: "ae2:crafting_co_processor", X: 0, Y: 2, Z: 0},
		{
			ID: "pp", BlockDefID: "ae2:pattern_provider", X: 0, Y: 0, Z: 1,
			Facing: domain.South,
			Config: map[string]any{
				"variant":       "directional",
				"blocking_mode": true,
				"patterns": []map[string]any{
					{
						"id":           "pat-smelt",
						"pattern_type": "processing",
						"inputs":       []map[string]any{{"item_id": "minecraft:raw_iron", "quantity": 1}},
						"outputs":      []map[string]any{{"item_id": "minecraft:iron_ingot", "quantity": 1}},
					},
				},
			},
		},
		// Placed on South face (0, 0, 2) matching directional provider's Facing!
		{ID: "furnace", BlockDefID: "melab:external_machine", X: 0, Y: 0, Z: 2, Config: map[string]any{"process_ticks": 3, "auto_return": true}},
	}

	env := setupCraftEnv(t, blocks, []domain.CraftingRequest{
		{ItemID: "minecraft:iron_ingot", Quantity: 2},
	})
	if len(env.diags) != 0 {
		t.Fatalf("unexpected init diags: %+v", env.diags)
	}

	// Tick 1: CPU has 1 co-processor (wants to dispatch 2 batches), furnace accepts batch 1 and blocks batch 2 -> PROVIDER_BLOCKED!
	_, t1Diags := env.step(1)
	if !hasDiagCode(t1Diags, diagnostics.CodeProviderBlocked) {
		t.Fatalf("expected PROVIDER_BLOCKED warning when blocking_mode=true and furnace is busy, got %+v", t1Diags)
	}

	for tick := 2; tick <= 8; tick++ {
		env.step(tick)
	}
	if env.craft.Jobs[0].Status != "completed" {
		t.Fatalf("expected processing job to complete after sequential batches, got %s", env.craft.Jobs[0].Status)
	}
	if got := env.store.AvailableMap("net-1")["minecraft:iron_ingot"]; got != 2 {
		t.Fatalf("expected 2 iron_ingot in storage, got %d", got)
	}
}

func TestCraftingFailureDiagnostics(t *testing.T) {
	baseBlocks := func(patterns []map[string]any, items []map[string]any, includeAssembler bool) []domain.PlacedBlock {
		b := []domain.PlacedBlock{
			{ID: "creative", BlockDefID: "ae2:creative_energy_cell", X: -1, Y: 0, Z: 0},
			{ID: "ctrl", BlockDefID: "ae2:controller", X: 0, Y: 0, Z: 0},
			{
				ID: "drive", BlockDefID: "ae2:drive", X: 1, Y: 0, Z: 0,
				Config: map[string]any{"cells": []map[string]any{{"tier": "16k", "items": items}}},
			},
			{ID: "cpu-1k", BlockDefID: "ae2:crafting_storage_1k", X: 0, Y: 1, Z: 0},
			{
				ID: "pp", BlockDefID: "ae2:pattern_provider", X: 0, Y: 0, Z: 1,
				Config: map[string]any{"patterns": patterns},
			},
		}
		if includeAssembler {
			b = append(b, domain.PlacedBlock{ID: "ma", BlockDefID: "ae2:molecular_assembler", X: 0, Y: 0, Z: 2})
		}
		return b
	}

	t.Run("missing crafting pattern", func(t *testing.T) {
		env := setupCraftEnv(t, baseBlocks(nil, nil, true), []domain.CraftingRequest{
			{ItemID: "minecraft:beacon", Quantity: 1},
		})
		if !hasDiagCode(env.diags, diagnostics.CodeMissingCraftingPattern) || env.craft.Jobs[0].Status != "failed" {
			t.Fatalf("expected MISSING_CRAFTING_PATTERN failure, got diags=%+v job=%+v", env.diags, env.craft.Jobs[0])
		}
	})

	t.Run("recursive crafting pattern cycle", func(t *testing.T) {
		pats := []map[string]any{
			{
				"id": "p1", "pattern_type": "crafting",
				"inputs":  []map[string]any{{"item_id": "minecraft:iron_block", "quantity": 1}},
				"outputs": []map[string]any{{"item_id": "minecraft:iron_ingot", "quantity": 9}},
			},
			{
				"id": "p2", "pattern_type": "crafting",
				"inputs":  []map[string]any{{"item_id": "minecraft:iron_ingot", "quantity": 9}},
				"outputs": []map[string]any{{"item_id": "minecraft:iron_block", "quantity": 1}},
			},
		}
		env := setupCraftEnv(t, baseBlocks(pats, nil, true), []domain.CraftingRequest{
			{ItemID: "minecraft:iron_ingot", Quantity: 9},
		})
		if !hasDiagCode(env.diags, diagnostics.CodeRecursiveCraftingPattern) || env.craft.Jobs[0].Status != "failed" {
			t.Fatalf("expected RECURSIVE_CRAFTING_PATTERN, got diags=%+v", env.diags)
		}
	})

	t.Run("missing raw ingredients", func(t *testing.T) {
		pats := []map[string]any{
			{
				"id": "p-stick", "pattern_type": "crafting",
				"inputs":  []map[string]any{{"item_id": "minecraft:oak_planks", "quantity": 2}},
				"outputs": []map[string]any{{"item_id": "minecraft:stick", "quantity": 4}},
			},
		}
		env := setupCraftEnv(t, baseBlocks(pats, nil, true), []domain.CraftingRequest{
			{ItemID: "minecraft:stick", Quantity: 4},
		})
		if !hasDiagCode(env.diags, diagnostics.CodeMissingIngredients) || env.craft.Jobs[0].Status != "failed" {
			t.Fatalf("expected MISSING_INGREDIENTS, got diags=%+v", env.diags)
		}
	})

	t.Run("insufficient CPU storage", func(t *testing.T) {
		pats := []map[string]any{
			{
				"id": "p-planks", "pattern_type": "crafting",
				"inputs":  []map[string]any{{"item_id": "minecraft:oak_log", "quantity": 1}},
				"outputs": []map[string]any{{"item_id": "minecraft:oak_planks", "quantity": 4}},
			},
		}
		items := []map[string]any{{"item_id": "minecraft:oak_log", "quantity": 5000}}
		// Request 8000 planks -> 2000 batches > 1024-byte 1k CPU!
		env := setupCraftEnv(t, baseBlocks(pats, items, true), []domain.CraftingRequest{
			{ItemID: "minecraft:oak_planks", Quantity: 8000},
		})
		if !hasDiagCode(env.diags, diagnostics.CodeInsufficientCPUStorage) || env.craft.Jobs[0].Status != "failed" {
			t.Fatalf("expected INSUFFICIENT_CPU_STORAGE, got diags=%+v", env.diags)
		}
	})

	t.Run("missing adjacent molecular assembler", func(t *testing.T) {
		pats := []map[string]any{
			{
				"id": "p-planks", "pattern_type": "crafting",
				"inputs":  []map[string]any{{"item_id": "minecraft:oak_log", "quantity": 1}},
				"outputs": []map[string]any{{"item_id": "minecraft:oak_planks", "quantity": 4}},
			},
		}
		items := []map[string]any{{"item_id": "minecraft:oak_log", "quantity": 10}}
		env := setupCraftEnv(t, baseBlocks(pats, items, false), []domain.CraftingRequest{
			{ItemID: "minecraft:oak_planks", Quantity: 4},
		})
		// Fails on tick 1 dispatch check -> NO_ADJACENT_CRAFTING_INVENTORY, refunds reserved oak_log!
		_, stepDiags := env.step(1)
		if !hasDiagCode(stepDiags, diagnostics.CodeNoAdjacentCraftingInventory) || env.craft.Jobs[0].Status != "failed" {
			t.Fatalf("expected NO_ADJACENT_CRAFTING_INVENTORY on tick 1, got diags=%+v job=%+v", stepDiags, env.craft.Jobs[0])
		}
		if got := env.store.AvailableMap("net-1")["minecraft:oak_log"]; got != 10 {
			t.Fatalf("expected reserved ingredients refunded to storage on job failure, got %d", got)
		}
	})
}
