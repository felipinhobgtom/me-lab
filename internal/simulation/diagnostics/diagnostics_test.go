package diagnostics

import (
	"testing"

	"github.com/felipinho/me-lab/internal/domain"
)

func TestDeterministicDiagnosticConstructors(t *testing.T) {
	b1 := domain.PlacedBlock{ID: "blk-2", BlockDefID: "ae2:fluix_cable", X: 2, Y: 0, Z: 0}
	b2 := domain.PlacedBlock{ID: "blk-1", BlockDefID: "ae2:fluix_cable", X: 1, Y: 0, Z: 0}

	d1 := NewChannelCapacityExceeded("sim-1", []domain.PlacedBlock{b1, b2}, 8, 8)
	d2 := NewChannelCapacityExceeded("sim-1", []domain.PlacedBlock{b2, b1}, 8, 8)

	if d1.ID != d2.ID {
		t.Fatalf("expected deterministic ID regardless of input block order, got %q vs %q", d1.ID, d2.ID)
	}
	if d1.Code != CodeChannelCapacityExceeded {
		t.Fatalf("unexpected code: %s", d1.Code)
	}
	if len(d1.AffectedBlocks) != 2 || d1.AffectedBlocks[0].ID != "blk-1" {
		t.Fatalf("expected sorted affected blocks, got %+v", d1.AffectedBlocks)
	}
	if d1.Remediation == "" {
		t.Fatal("expected non-empty remediation")
	}

	dOtherSim := NewChannelCapacityExceeded("sim-2", []domain.PlacedBlock{b2, b1}, 8, 8)
	if d1.ID == dOtherSim.ID {
		t.Fatal("expected distinct IDs across different simulation IDs")
	}
}

func TestAllDiagnosticCodesAndSorting(t *testing.T) {
	b := domain.PlacedBlock{ID: "b1", BlockDefID: "ae2:drive", X: 0, Y: 0, Z: 0}
	all := []domain.Diagnostic{
		NewBlockDisconnected("", b),
		NewProviderBlocked("", b, "minecraft:chest"),
		NewUnsupportedMechanic("", b, "P2P Tunnel"),
		NewChannelCapacityExceeded("", []domain.PlacedBlock{b}, 8, 8),
		NewDeviceNoChannel("", b, "saturated"),
		NewNetworkNoController("", "net-1", []domain.PlacedBlock{b}, 9, 8),
		NewMultipleControllersDisconnected("", []domain.PlacedBlock{b}),
		NewInvalidControllerMultiblock("", []domain.PlacedBlock{b}, "cross"),
		NewInvalidCraftingCPUMultiblock("", []domain.PlacedBlock{b}, "L-shape"),
		NewInsufficientEnergy("", "net-1", []domain.PlacedBlock{b}, 0, 10),
		NewStorageFull("", []domain.PlacedBlock{b}, "minecraft:cobblestone", 64),
		NewMissingCraftingPattern("", []domain.PlacedBlock{b}, "minecraft:chest"),
		NewRecursiveCraftingPattern("", []domain.PlacedBlock{b}, []string{"a", "b", "a"}),
		NewMissingIngredients("", []domain.PlacedBlock{b}, "minecraft:oak_log", 4),
		NewInsufficientCPUStorage("", []domain.PlacedBlock{b}, 2048, 1024),
		NewNoValidCraftingCPU("", []domain.PlacedBlock{b}, domain.SeverityError, "none"),
		NewNoAdjacentCraftingInventory("", b, "crafting"),
	}

	if len(all) != 17 {
		t.Fatalf("expected 17 diagnostics, got %d", len(all))
	}

	// Duplicate entry should be collapsed by DeduplicateAndSort.
	withDup := append(all, NewBlockDisconnected("", b))
	sorted := DeduplicateAndSort(withDup)
	if len(sorted) != 17 {
		t.Fatalf("expected 17 deduplicated diagnostics, got %d", len(sorted))
	}
	if sorted[0].Severity != domain.SeverityError {
		t.Fatalf("expected errors sorted before warnings, got %s first", sorted[0].Severity)
	}
	if sorted[len(sorted)-1].Severity != domain.SeverityWarning {
		t.Fatalf("expected warnings sorted after errors, got %s last", sorted[len(sorted)-1].Severity)
	}
}
