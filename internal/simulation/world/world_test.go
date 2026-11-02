package world

import (
	"testing"

	"github.com/felipinho/me-lab/internal/domain"
	"github.com/felipinho/me-lab/internal/simulation/diagnostics"
)

func hasDiagnosticCode(diags []domain.Diagnostic, code string) bool {
	for _, d := range diags {
		if d.Code == code {
			return true
		}
	}
	return false
}

func TestColoredCableAndDeviceConnectivity(t *testing.T) {
	blocks := []domain.PlacedBlock{
		{ID: "c-blue", BlockDefID: "ae2:fluix_cable", X: 0, Y: 0, Z: 0, Config: map[string]any{"color": "blue"}},
		{ID: "c-red", BlockDefID: "ae2:fluix_cable", X: 1, Y: 0, Z: 0, Config: map[string]any{"color": "red"}},
		{ID: "c-fluix", BlockDefID: "ae2:smart_cable", X: 2, Y: 0, Z: 0, Config: map[string]any{"color": "fluix"}},
		{ID: "drive", BlockDefID: "ae2:drive", X: 0, Y: 1, Z: 0},
	}

	w := Build("sim-w1", blocks, nil)

	// Blue (0,0,0) and Red (1,0,0) must NOT connect.
	if w.Connected(domain.Coord3D{X: 0, Y: 0, Z: 0}, domain.Coord3D{X: 1, Y: 0, Z: 0}) {
		t.Fatal("expected blue and red cables NOT to connect")
	}

	// Red (1,0,0) and Fluix (2,0,0) MUST connect.
	if !w.Connected(domain.Coord3D{X: 1, Y: 0, Z: 0}, domain.Coord3D{X: 2, Y: 0, Z: 0}) {
		t.Fatal("expected red cable to connect to fluix cable")
	}

	// Blue (0,0,0) and non-cable Drive (0,1,0) MUST connect regardless of cable color.
	if !w.Connected(domain.Coord3D{X: 0, Y: 0, Z: 0}, domain.Coord3D{X: 0, Y: 1, Z: 0}) {
		t.Fatal("expected colored cable to connect to adjacent ME Drive")
	}
}

func TestCableAnchorsAndAnchoredFaces(t *testing.T) {
	blocks := []domain.PlacedBlock{
		{ID: "c1", BlockDefID: "ae2:fluix_cable", X: 0, Y: 0, Z: 0, Config: map[string]any{"anchored_faces": []any{"east"}}},
		{ID: "c2", BlockDefID: "ae2:fluix_cable", X: 1, Y: 0, Z: 0},
		{ID: "anchor", BlockDefID: "ae2:cable_anchor", X: 1, Y: 1, Z: 0},
	}

	w := Build("sim-w2", blocks, nil)

	if w.Connected(domain.Coord3D{X: 0, Y: 0, Z: 0}, domain.Coord3D{X: 1, Y: 0, Z: 0}) {
		t.Fatal("expected anchored east face to block ME connection")
	}
	if anchorNode := w.ByCoord[domain.Coord3D{X: 1, Y: 1, Z: 0}]; anchorNode == nil || anchorNode.IsMENode {
		t.Fatal("expected ae2:cable_anchor not to join ME network")
	}
	if !hasDiagnosticCode(w.Diagnostics, diagnostics.CodeBlockDisconnected) {
		t.Fatal("expected isolated cables separated by anchor to emit BLOCK_DISCONNECTED warning")
	}
}

func TestDirectionalPatternProviderAndAssemblerAndExternalMachine(t *testing.T) {
	blocks := []domain.PlacedBlock{
		{ID: "cable", BlockDefID: "ae2:smart_cable", X: 0, Y: 0, Z: 0},
		{
			ID:         "pp",
			BlockDefID: "ae2:pattern_provider",
			X:          1, Y: 0, Z: 0,
			Facing: domain.East, // Push face points toward (2, 0, 0)
			Config: map[string]any{"variant": "directional"},
		},
		{ID: "ma-push", BlockDefID: "ae2:molecular_assembler", X: 2, Y: 0, Z: 0},
		{ID: "ma-side", BlockDefID: "ae2:molecular_assembler", X: 1, Y: 1, Z: 0},
		{ID: "ext", BlockDefID: "melab:external_machine", X: 1, Y: -1, Z: 0},
	}

	w := Build("sim-w3", blocks, nil)

	// Cable on West side of directional pattern provider connects.
	if !w.Connected(domain.Coord3D{X: 0, Y: 0, Z: 0}, domain.Coord3D{X: 1, Y: 0, Z: 0}) {
		t.Fatal("expected non-push face of directional pattern provider to connect to cable")
	}
	// Push face (East -> 2,0,0) does NOT form an ME network edge!
	if w.Connected(domain.Coord3D{X: 1, Y: 0, Z: 0}, domain.Coord3D{X: 2, Y: 0, Z: 0}) {
		t.Fatal("expected directional pattern provider NOT to connect ME edge on its Facing push side")
	}
	// Up face (1,1,0) connects to Molecular Assembler, and Molecular Assembler transmits channels with 0 usage.
	if !w.Connected(domain.Coord3D{X: 1, Y: 0, Z: 0}, domain.Coord3D{X: 1, Y: 1, Z: 0}) {
		t.Fatal("expected non-push face of directional pattern provider to connect to Molecular Assembler")
	}
	maSide := w.ByID["ma-side"]
	if !maSide.TransmitsChannels() || maSide.EffectiveChannelUsage() != 0 {
		t.Fatalf("expected molecular assembler to transmit channels with 0 usage, got transmit=%v usage=%d", maSide.TransmitsChannels(), maSide.EffectiveChannelUsage())
	}
	// External machine does not join ME networks.
	extNode := w.ByID["ext"]
	if extNode.IsMENode {
		t.Fatal("expected melab:external_machine not to be an ME node")
	}
}

func TestUnsupportedBlocksAndCreativeSelfSufficient(t *testing.T) {
	blocks := []domain.PlacedBlock{
		{ID: "p2p", BlockDefID: "ae2:p2p_tunnel", X: 0, Y: 0, Z: 0},
		{ID: "creative", BlockDefID: "ae2:creative_energy_cell", X: 5, Y: 0, Z: 0},
	}
	catalog := map[string]domain.BlockDefinition{
		"ae2:p2p_tunnel": {
			ID:       "ae2:p2p_tunnel",
			Name:     "ME P2P Tunnel",
			Fidelity: domain.FidelityUnsupported,
		},
	}

	w := Build("sim-w4", blocks, catalog)
	if !hasDiagnosticCode(w.Diagnostics, diagnostics.CodeUnsupportedMechanic) {
		t.Fatal("expected UNSUPPORTED_MECHANIC diagnostic for ae2:p2p_tunnel")
	}
	if hasDiagnosticCode(w.Diagnostics, diagnostics.CodeBlockDisconnected) {
		t.Fatal("expected standalone ae2:creative_energy_cell not to emit BLOCK_DISCONNECTED")
	}
}
