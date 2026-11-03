package storage

import (
	"testing"

	"github.com/felipinho/me-lab/internal/domain"
)

func TestBusesTransferOnlyWhenPoweredAndChanneled(t *testing.T) {
	state := &State{
		Cells: []*CellState{
			{
				NetworkID:    "net-1",
				Online:       true,
				HasChannel:   true,
				TotalBytes:   4096,
				MaxTypes:     63,
				BytesPerType: 32,
				ItemsPerByte: 8,
				Items:        []domain.ItemStack{{ItemID: "minecraft:iron_ingot", Quantity: 16}},
			},
		},
		ExternalInventories: []*ExternalInventoryState{
			{
				Block:    domain.PlacedBlock{ID: "chest", BlockDefID: "minecraft:chest", X: 2},
				Capacity: 1728,
				Items:    []domain.ItemStack{{ItemID: "minecraft:oak_log", Quantity: 16}},
			},
		},
		Buses: []*BusState{
			{
				BlockID:       "import",
				NetworkID:     "net-1",
				Kind:          "ae2:import_bus",
				TargetBlockID: "chest",
				HasChannel:    true,
				Acceleration:  1,
			},
			{
				BlockID:       "export",
				NetworkID:     "net-1",
				Kind:          "ae2:export_bus",
				TargetBlockID: "chest",
				Filters:       []string{"minecraft:iron_ingot"},
				HasChannel:    true,
				Acceleration:  0,
			},
		},
	}

	transfers := state.StepBuses(map[string]bool{"net-1": true})
	if len(transfers) != 2 {
		t.Fatalf("expected import and export transfers, got %+v", transfers)
	}
	inv := state.ExternalInventories[0]
	if got := quantityOf(inv.Items, "minecraft:oak_log"); got != 8 {
		t.Fatalf("expected acceleration-1 import bus to move 8 oak logs, got %d remaining", got)
	}
	if got := quantityOf(inv.Items, "minecraft:iron_ingot"); got != 1 {
		t.Fatalf("expected export bus to move one iron ingot, got %d", got)
	}
	if got := quantityOf(state.Cells[0].Items, "minecraft:iron_ingot"); got != 15 {
		t.Fatalf("expected export bus to consume one ME iron ingot, got %d", got)
	}

	before := append([]domain.ItemStack(nil), inv.Items...)
	state.StepBuses(map[string]bool{"net-1": false})
	if got := quantityOf(inv.Items, "minecraft:iron_ingot"); got != quantityOf(before, "minecraft:iron_ingot") {
		t.Fatalf("offline bus changed external inventory: before=%+v after=%+v", before, inv.Items)
	}
}

func quantityOf(items []domain.ItemStack, itemID string) int {
	for _, item := range items {
		if item.ItemID == itemID {
			return item.Quantity
		}
	}
	return 0
}
