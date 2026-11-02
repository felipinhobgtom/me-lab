package energy

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/felipinho/me-lab/internal/domain"
	"github.com/felipinho/me-lab/internal/simulation/channels"
	"github.com/felipinho/me-lab/internal/simulation/diagnostics"
	"github.com/felipinho/me-lab/internal/simulation/network"
	"github.com/felipinho/me-lab/internal/simulation/world"
)

func testEnergyProfile() domain.CompatibilityProfile {
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
			MolecularAssemblerActiveE: 20,
		},
	}
}

func TestUnpoweredNetworkFailsWithInsufficientEnergy(t *testing.T) {
	blocks := []domain.PlacedBlock{
		{ID: "ctrl", BlockDefID: "ae2:controller", X: 0, Y: 0, Z: 0},
		{ID: "drive", BlockDefID: "ae2:drive", X: 1, Y: 0, Z: 0},
	}
	w := world.Build("", blocks, nil)
	nets, _ := network.Discover("", w, testEnergyProfile())
	alloc := channels.Allocate("", w, nets, testEnergyProfile())

	states, diags := Initialize("", nets, alloc, testEnergyProfile())
	if len(states) != 1 || states[0].Online {
		t.Fatalf("expected unpowered network to be offline, got %+v", states)
	}
	if len(diags) == 0 || diags[0].Code != diagnostics.CodeInsufficientEnergy {
		t.Fatalf("expected INSUFFICIENT_ENERGY diagnostic, got %+v", diags)
	}
}

func TestEnergyAcceptorAndCellDrainToBrownout(t *testing.T) {
	// Start with an Energy Cell at 0.01% charge (20 AE) and NO Energy Acceptor on a Controller (6 AE/t).
	// Tick 1: 20 - 6 = 14 AE (online)
	// Tick 2: 14 - 6 = 8 AE (online)
	// Tick 3: 8 - 6 = 2 AE (online)
	// Tick 4: 2 < 6 -> brownout! Online=false, INSUFFICIENT_ENERGY!
	blocks := []domain.PlacedBlock{
		{ID: "ctrl", BlockDefID: "ae2:controller", X: 0, Y: 0, Z: 0},
		{ID: "cell", BlockDefID: "ae2:energy_cell", X: 1, Y: 0, Z: 0, Config: map[string]any{"initial_charge_pct": 0.01}},
	}
	prof := testEnergyProfile()
	w := world.Build("", blocks, nil)
	nets, _ := network.Discover("", w, prof)
	alloc := channels.Allocate("", w, nets, prof)
	netsByID := map[string]*network.Network{"net-1": nets[0]}

	states, diags := Initialize("", nets, alloc, prof)
	if len(diags) != 0 || !states[0].Online {
		t.Fatalf("expected network with 20 AE stored to start online, got online=%v diags=%+v", states[0].Online, diags)
	}
	if math.Abs(states[0].Stored-20.0) > 1e-6 {
		t.Fatalf("expected initial stored 20 AE, got %f", states[0].Stored)
	}

	for tick := 1; tick <= 3; tick++ {
		stepDiags := StepEnergy("", states, netsByID, nil, alloc, prof)
		if len(stepDiags) != 0 || !states[0].Online {
			t.Fatalf("expected tick %d to remain online, got diags=%+v", tick, stepDiags)
		}
	}

	// Now add 1 active Molecular Assembler (+20 AE/t -> total 26 AE/t > 2 AE remaining) on tick 4!
	stepDiags := StepEnergy("", states, netsByID, map[string]int{"net-1": 1}, alloc, prof)
	if states[0].Online {
		t.Fatal("expected network to go offline when buffer exhausted")
	}
	if len(stepDiags) == 0 || stepDiags[0].Code != diagnostics.CodeInsufficientEnergy {
		t.Fatalf("expected INSUFFICIENT_ENERGY on brownout, got %+v", stepDiags)
	}
}

func TestCreativeEnergyCellAndAcceptorConversionJSONSafe(t *testing.T) {
	blocks := []domain.PlacedBlock{
		{ID: "ctrl", BlockDefID: "ae2:controller", X: 0, Y: 0, Z: 0},
		{ID: "ea", BlockDefID: "ae2:energy_acceptor", X: -1, Y: 0, Z: 0, Config: map[string]any{"fe_input_per_tick": 800}},
		{ID: "creative", BlockDefID: "ae2:creative_energy_cell", X: 1, Y: 0, Z: 0},
	}
	prof := testEnergyProfile()
	w := world.Build("", blocks, nil)
	nets, _ := network.Discover("", w, prof)
	alloc := channels.Allocate("", w, nets, prof)

	states, diags := Initialize("", nets, alloc, prof)
	if len(diags) != 0 || !states[0].Online {
		t.Fatalf("expected creative network online, got diags=%+v", diags)
	}
	// Verify JSON marshaling works cleanly (no +Inf).
	if _, err := json.Marshal(states); err != nil {
		t.Fatalf("expected NetworkEnergy to marshal cleanly to JSON: %v", err)
	}
}
