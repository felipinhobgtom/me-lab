package storage

import (
	"testing"

	"github.com/felipinho/me-lab/internal/domain"
	"github.com/felipinho/me-lab/internal/simulation/world"
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

func TestFurnaceProcessesAfter200Ticks(t *testing.T) {
	state := &State{ExternalInventories: []*ExternalInventoryState{{
		Block: domain.PlacedBlock{ID: "furnace", BlockDefID: "minecraft:furnace"},
		Input: []domain.ItemStack{{ItemID: "minecraft:raw_iron", Quantity: 1}},
		Fuel:  []domain.ItemStack{{ItemID: "minecraft:coal", Quantity: 1}},
	}}}

	for i := 0; i < 200; i++ {
		state.StepExternalMachines()
	}
	inv := state.ExternalInventories[0]
	if got := quantityOf(inv.Output, "minecraft:iron_ingot"); got != 1 {
		t.Fatalf("expected furnace output after 200 ticks, got %+v", inv.Output)
	}
	if got := quantityOf(inv.Fuel, "minecraft:coal"); got != 0 {
		t.Fatalf("expected coal consumed when burning, got %d", got)
	}
}

func TestBlastFurnaceProcessesAfter100Ticks(t *testing.T) {
	state := &State{ExternalInventories: []*ExternalInventoryState{{
		Block: domain.PlacedBlock{ID: "blast", BlockDefID: "minecraft:blast_furnace"},
		Input: []domain.ItemStack{{ItemID: "minecraft:raw_gold", Quantity: 1}},
		Fuel:  []domain.ItemStack{{ItemID: "minecraft:coal", Quantity: 1}},
	}}}

	for i := 0; i < 100; i++ {
		state.StepExternalMachines()
	}
	inv := state.ExternalInventories[0]
	if got := quantityOf(inv.Output, "minecraft:gold_ingot"); got != 1 {
		t.Fatalf("expected blast furnace output after 100 ticks, got %+v", inv.Output)
	}
	if inv.BurnTime != 1400 || !inv.Lit {
		t.Fatalf("expected blast furnace to consume two burn ticks per tick, burn=%d lit=%v", inv.BurnTime, inv.Lit)
	}
}

func TestFurnaceOutputCapacityAndCompatibilityBlockBurning(t *testing.T) {
	tests := []domain.ItemStack{
		{ItemID: "minecraft:iron_ingot", Quantity: 64},
		{ItemID: "minecraft:gold_ingot", Quantity: 1},
	}
	for _, output := range tests {
		state := &State{ExternalInventories: []*ExternalInventoryState{{
			Block:  domain.PlacedBlock{ID: "furnace", BlockDefID: "minecraft:furnace"},
			Input:  []domain.ItemStack{{ItemID: "minecraft:raw_iron", Quantity: 1}},
			Fuel:   []domain.ItemStack{{ItemID: "minecraft:coal", Quantity: 1}},
			Output: []domain.ItemStack{output},
		}}}
		state.StepExternalMachines()
		inv := state.ExternalInventories[0]
		if inv.BurnTime != 0 || inv.Lit || quantityOf(inv.Fuel, "minecraft:coal") != 1 {
			t.Fatalf("blocked output consumed fuel: output=%+v fuel=%+v burn=%d lit=%v", output, inv.Fuel, inv.BurnTime, inv.Lit)
		}
	}
}

func TestFurnaceWithoutFuelDoesNotLight(t *testing.T) {
	state := &State{ExternalInventories: []*ExternalInventoryState{{
		Block: domain.PlacedBlock{ID: "furnace", BlockDefID: "minecraft:furnace"},
		Input: []domain.ItemStack{{ItemID: "minecraft:sand", Quantity: 1}},
	}}}
	state.StepExternalMachines()
	inv := state.ExternalInventories[0]
	if inv.Lit || inv.BurnTime != 0 || inv.CookTime != 0 {
		t.Fatalf("furnace without fuel started: %+v", inv)
	}
}

func TestContainersPreserveLegacyItems(t *testing.T) {
	state := &State{ExternalInventories: []*ExternalInventoryState{{
		Block: domain.PlacedBlock{ID: "chest", BlockDefID: "minecraft:chest"},
		Items: []domain.ItemStack{{ItemID: "minecraft:oak_log", Quantity: 12}},
	}}}
	state.StepExternalMachines()
	inv := state.ExternalInventories[0]
	if quantityOf(inv.Items, "minecraft:oak_log") != 12 {
		t.Fatalf("container legacy items changed: %+v", inv.Items)
	}
	if len(inv.Input) != 0 || len(inv.Fuel) != 0 || len(inv.Output) != 0 {
		t.Fatalf("container unexpectedly gained machine slots: %+v", inv)
	}
}

func TestHopperIsAFiveSlotExternalInventory(t *testing.T) {
	if got := inventoryCapacity(domain.PlacedBlock{BlockDefID: "minecraft:hopper"}); got != 5*64 {
		t.Fatalf("expected hopper capacity for five 64-item slots, got %d", got)
	}
	if !isExternalInventory(&world.Node{Block: domain.PlacedBlock{BlockDefID: "minecraft:hopper"}}) {
		t.Fatal("hopper was not registered as an external inventory")
	}
}

func TestBusesUseFurnaceOutputAndInputSlots(t *testing.T) {
	importState := &State{
		Cells: []*CellState{{
			NetworkID: "net-1", Online: true, HasChannel: true,
			TotalBytes: 4096, MaxTypes: 63, BytesPerType: 32, ItemsPerByte: 8,
		}},
		ExternalInventories: []*ExternalInventoryState{{
			Block:    domain.PlacedBlock{ID: "furnace", BlockDefID: "minecraft:furnace"},
			Capacity: 3 * 64,
			Output:   []domain.ItemStack{{ItemID: "minecraft:iron_ingot", Quantity: 2}},
		}},
		Buses: []*BusState{{
			BlockID: "import", NetworkID: "net-1", Kind: "ae2:import_bus",
			TargetBlockID: "furnace", HasChannel: true, Acceleration: 0,
		}},
	}
	importState.StepBuses(map[string]bool{"net-1": true})
	if got := quantityOf(importState.ExternalInventories[0].Output, "minecraft:iron_ingot"); got != 1 {
		t.Fatalf("expected import bus to remove one output item, got %d", got)
	}

	exportState := &State{
		Cells: []*CellState{{
			NetworkID: "net-1", Online: true, HasChannel: true,
			TotalBytes: 4096, MaxTypes: 63, BytesPerType: 32, ItemsPerByte: 8,
			Items: []domain.ItemStack{{ItemID: "minecraft:raw_iron", Quantity: 2}},
		}},
		ExternalInventories: []*ExternalInventoryState{{
			Block:    domain.PlacedBlock{ID: "furnace", BlockDefID: "minecraft:furnace"},
			Capacity: 3 * 64,
		}},
		Buses: []*BusState{{
			BlockID: "export", NetworkID: "net-1", Kind: "ae2:export_bus",
			TargetBlockID: "furnace", Filters: []string{"minecraft:raw_iron"},
			HasChannel: true, Acceleration: 0,
		}},
	}
	exportState.StepBuses(map[string]bool{"net-1": true})
	if got := quantityOf(exportState.ExternalInventories[0].Input, "minecraft:raw_iron"); got != 1 {
		t.Fatalf("expected export bus to fill furnace input, got %d", got)
	}
}
