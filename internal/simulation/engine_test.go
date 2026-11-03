package simulation

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/felipinho/me-lab/internal/domain"
	"github.com/felipinho/me-lab/internal/simulation/diagnostics"
)

func acceptanceProfile() domain.CompatibilityProfile {
	return domain.CompatibilityProfile{
		ID:               "ae2-1.21.1-default",
		Name:             "AE2 1.21.1 Default",
		MinecraftVersion: "1.21.1",
		AE2Version:       "19.0.x",
		ChannelMode:      "default",
		Config: domain.ProfileConfig{
			NormalCableChannels:       8,
			DenseCableChannels:        32,
			ControllerFaceChannels:    32,
			AdHocChannelLimit:         8,
			InfiniteChannels:          false,
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
			CellTiers: map[string]domain.CellTierSpec{
				"1k":   {TotalBytes: 1024, MaxTypes: 63, BytesPerType: 8},
				"4k":   {TotalBytes: 4096, MaxTypes: 63, BytesPerType: 32},
				"16k":  {TotalBytes: 16384, MaxTypes: 63, BytesPerType: 128},
				"64k":  {TotalBytes: 65536, MaxTypes: 63, BytesPerType: 512},
				"256k": {TotalBytes: 262144, MaxTypes: 63, BytesPerType: 2048},
			},
		},
	}
}

func acceptanceAutocraftingBlocks() []domain.PlacedBlock {
	return []domain.PlacedBlock{
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
							{"item_id": "minecraft:iron_ingot", "quantity": 32},
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
}

func TestEngineDeterminismAndJSONRoundTrip(t *testing.T) {
	prof := acceptanceProfile()
	blocksA := acceptanceAutocraftingBlocks()
	// Reverse block slice order in run B to prove input-order independence & full determinism!
	blocksB := make([]domain.PlacedBlock, len(blocksA))
	for i := range blocksA {
		blocksB[i] = blocksA[len(blocksA)-1-i]
	}

	snapA := Snapshot{
		SimulationID:     "sim-det-1",
		ProjectID:        "proj-1",
		Seed:             42,
		Profile:          prof,
		Blocks:           blocksA,
		CraftingRequests: []domain.CraftingRequest{{ItemID: "minecraft:chest", Quantity: 2}},
	}
	snapB := Snapshot{
		SimulationID:     "sim-det-1",
		ProjectID:        "proj-1",
		Seed:             42,
		Profile:          prof,
		Blocks:           blocksB,
		CraftingRequests: []domain.CraftingRequest{{ItemID: "minecraft:chest", Quantity: 2}},
	}

	// Run A continuously in memory.
	stateA, evInitA, diagInitA, err := Initialize(snapA)
	if err != nil {
		t.Fatalf("Initialize A failed: %v", err)
	}
	if len(diagInitA) != 0 {
		t.Fatalf("unexpected diagnostics in A: %+v", diagInitA)
	}
	resA, err := StepN(stateA, 15, prof, nil)
	if err != nil {
		t.Fatalf("StepN A failed: %v", err)
	}

	// Run B while marshaling and unmarshaling EngineState to/from JSON on EVERY SINGLE TICK!
	stateB, evInitB, _, err := Initialize(snapB)
	if err != nil {
		t.Fatalf("Initialize B failed: %v", err)
	}

	var allStepEventsB []domain.SimulationEvent
	for range 15 {
		raw, err := MarshalState(stateB)
		if err != nil {
			t.Fatalf("MarshalState failed: %v", err)
		}
		stateB, err = UnmarshalState(raw)
		if err != nil {
			t.Fatalf("UnmarshalState failed: %v", err)
		}
		stepRes, err := Step(stateB, prof, nil)
		if err != nil {
			t.Fatalf("Step B failed: %v", err)
		}
		allStepEventsB = append(allStepEventsB, stepRes.Events...)
	}

	if stateA.Status != domain.SimStatusCompleted || stateB.Status != domain.SimStatusCompleted {
		t.Fatalf("expected both runs completed, got A=%s B=%s", stateA.Status, stateB.Status)
	}

	jsonStateA, err := MarshalState(stateA)
	if err != nil {
		t.Fatalf("MarshalState A failed: %v", err)
	}
	jsonStateB, err := MarshalState(stateB)
	if err != nil {
		t.Fatalf("MarshalState B failed: %v", err)
	}
	if !bytes.Equal(jsonStateA, jsonStateB) {
		t.Fatalf("expected 100%% byte-identical EngineState JSON across runs!\nA: %s\nB: %s", string(jsonStateA), string(jsonStateB))
	}

	evBytesA, _ := json.Marshal(append(evInitA, resA.Events...))
	evBytesB, _ := json.Marshal(append(evInitB, allStepEventsB...))
	if !bytes.Equal(evBytesA, evBytesB) {
		t.Fatalf("expected 100%% byte-identical SimulationEvents across runs!\nA: %s\nB: %s", string(evBytesA), string(evBytesB))
	}
}

func TestValidateValidVsBottleneckFaultFixtures(t *testing.T) {
	prof := acceptanceProfile()
	eng := NewEngine()

	// 1. Valid fixture -> Valid == true, 4 channels used, online!
	valOK := eng.Validate(Snapshot{
		ProjectID: "proj-valid",
		Profile:   prof,
		Blocks:    acceptanceAutocraftingBlocks(),
	})
	if !valOK.Valid || len(valOK.Diagnostics) != 0 {
		t.Fatalf("expected valid fixture to pass validation, got valid=%v diags=%+v", valOK.Valid, valOK.Diagnostics)
	}
	if valOK.Summary.OnlineNetworks != 1 || valOK.Summary.ChannelsUsed != 4 {
		t.Fatalf("expected 1 online network using 4 channels, got %+v", valOK.Summary)
	}

	// 2. Fault fixture: 10 channel-consuming devices on an ad-hoc network with NO controller -> Valid == false!
	faultBlocks := []domain.PlacedBlock{
		{ID: "creative", BlockDefID: "ae2:creative_energy_cell", X: -1, Y: 0, Z: 0},
	}
	for i := range 10 {
		faultBlocks = append(faultBlocks,
			domain.PlacedBlock{ID: fmt.Sprintf("sc-%d", i), BlockDefID: "ae2:smart_cable", X: i, Y: 0, Z: 0},
			domain.PlacedBlock{ID: fmt.Sprintf("drv-%d", i), BlockDefID: "ae2:drive", X: i, Y: 1, Z: 0},
		)
	}

	valFault := eng.Validate(Snapshot{
		ProjectID: "proj-fault",
		Profile:   prof,
		Blocks:    faultBlocks,
	})
	if valFault.Valid {
		t.Fatal("expected 10-device controllerless fixture to fail validation")
	}

	hasNoCtrl := false
	hasNoChan := false
	for _, d := range valFault.Diagnostics {
		if d.Code == diagnostics.CodeNetworkNoController {
			hasNoCtrl = true
		}
		if d.Code == diagnostics.CodeDeviceNoChannel {
			hasNoChan = true
		}
	}
	if !hasNoCtrl || !hasNoChan {
		t.Fatalf("expected NETWORK_NO_CONTROLLER and DEVICE_NO_CHANNEL diagnostics, got %+v", valFault.Diagnostics)
	}
}

func TestPauseAndResumeLifecycle(t *testing.T) {
	prof := acceptanceProfile()
	state, _, _, err := Initialize(Snapshot{
		SimulationID:     "sim-pause",
		Profile:          prof,
		Blocks:           acceptanceAutocraftingBlocks(),
		CraftingRequests: []domain.CraftingRequest{{ItemID: "minecraft:chest", Quantity: 1}},
	})
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	Pause(state)
	if state.Status != domain.SimStatusPaused {
		t.Fatalf("expected paused status, got %s", state.Status)
	}
	resPaused, err := Step(state, prof, nil)
	if err != nil {
		t.Fatalf("Step while paused failed: %v", err)
	}
	if resPaused.Tick != 0 || len(resPaused.Events) != 0 {
		t.Fatalf("expected paused simulation not to advance tick, got tick=%d", resPaused.Tick)
	}

	Resume(state)
	if state.Status != domain.SimStatusRunning {
		t.Fatalf("expected resumed status running, got %s", state.Status)
	}
	resStep, err := Step(state, prof, nil)
	if err != nil || resStep.Tick != 1 {
		t.Fatalf("expected tick=1 after resume, got tick=%d err=%v", resStep.Tick, err)
	}
}

func TestPoweredImportBusMovesItemsIntoMEStorage(t *testing.T) {
	prof := acceptanceProfile()
	state, _, diags, err := Initialize(Snapshot{
		SimulationID: "sim-import-bus",
		Profile:      prof,
		Blocks: []domain.PlacedBlock{
			{ID: "energy-cell", BlockDefID: "ae2:energy_cell", X: -1, Y: 0, Z: 0, Config: map[string]any{"initial_charge_pct": 100}},
			{ID: "controller", BlockDefID: "ae2:controller", X: 0, Y: 0, Z: 0},
			{ID: "cable", BlockDefID: "ae2:fluix_cable", X: 1, Y: 0, Z: 0, Config: map[string]any{"subparts": map[string]any{"east": "ae2:import_bus"}}},
			{ID: "drive", BlockDefID: "ae2:drive", X: 0, Y: 1, Z: 0, Config: map[string]any{"cells": []map[string]any{{"tier": "4k"}}}},
			{ID: "chest", BlockDefID: "minecraft:chest", X: 2, Y: 0, Z: 0, Config: map[string]any{"inventory": []map[string]any{{"item_id": "minecraft:oak_log", "quantity": 4}}}},
		},
	})
	if err != nil || len(diags) != 0 {
		t.Fatalf("expected powered bus fixture to initialize cleanly, err=%v diags=%+v", err, diags)
	}

	result, err := Step(state, prof, nil)
	if err != nil {
		t.Fatalf("Step failed: %v", err)
	}
	if len(result.Events) != 1 || result.Events[0].EventType != "bus_transfer" {
		t.Fatalf("expected one bus_transfer event, got %+v", result.Events)
	}
	if got := state.Storage.AvailableMap("net-1")["minecraft:oak_log"]; got != 1 {
		t.Fatalf("expected one oak log imported per tick, got %d", got)
	}
	if got := state.Storage.ExternalInventories[0].Items[0].Quantity; got != 3 {
		t.Fatalf("expected three oak logs remaining in chest, got %d", got)
	}
}
