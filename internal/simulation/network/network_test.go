package network

import (
	"fmt"
	"testing"

	"github.com/felipinho/me-lab/internal/domain"
	"github.com/felipinho/me-lab/internal/simulation/diagnostics"
	"github.com/felipinho/me-lab/internal/simulation/world"
)

func defaultProfile() domain.CompatibilityProfile {
	return domain.CompatibilityProfile{
		ID: "ae2-1.21.1-default",
		Config: domain.ProfileConfig{
			ControllerMaxSpan: 7,
		},
	}
}

func hasDiag(diags []domain.Diagnostic, code string) bool {
	for _, d := range diags {
		if d.Code == code {
			return true
		}
	}
	return false
}

func TestControllerValidationRules(t *testing.T) {
	t.Run("ad-hoc network when 0 controllers", func(t *testing.T) {
		w := world.Build("", []domain.PlacedBlock{
			{ID: "c1", BlockDefID: "ae2:fluix_cable", X: 0, Y: 0, Z: 0},
			{ID: "d1", BlockDefID: "ae2:drive", X: 1, Y: 0, Z: 0},
		}, nil)
		nets, diags := Discover("", w, defaultProfile())
		if len(diags) != 0 || len(nets) != 1 || !nets[0].IsAdHoc || nets[0].HasController {
			t.Fatalf("expected 1 valid ad-hoc network, got nets=%d diags=%+v", len(nets), diags)
		}
	})

	t.Run("disconnected controllers on same network fails", func(t *testing.T) {
		w := world.Build("", []domain.PlacedBlock{
			{ID: "ctrl1", BlockDefID: "ae2:controller", X: 0, Y: 0, Z: 0},
			{ID: "cable", BlockDefID: "ae2:fluix_cable", X: 1, Y: 0, Z: 0},
			{ID: "ctrl2", BlockDefID: "ae2:controller", X: 2, Y: 0, Z: 0},
		}, nil)
		nets, diags := Discover("", w, defaultProfile())
		if len(nets) != 1 || nets[0].ControllerValid {
			t.Fatal("expected single network with invalid controller")
		}
		if !hasDiag(diags, diagnostics.CodeInvalidControllerMultiblock) {
			t.Fatal("expected INVALID_CONTROLLER_MULTIBLOCK")
		}
		if !hasDiag(diags, diagnostics.CodeMultipleControllersDisconnected) {
			t.Fatal("expected MULTIPLE_CONTROLLERS_DISCONNECTED")
		}
	})

	t.Run("controller span exceeding 7 blocks fails", func(t *testing.T) {
		var blocks []domain.PlacedBlock
		for x := range 8 {
			blocks = append(blocks, domain.PlacedBlock{
				ID: fmt.Sprintf("ctrl-%d", x), BlockDefID: "ae2:controller", X: x, Y: 0, Z: 0,
			})
		}
		w := world.Build("", blocks, nil)
		nets, diags := Discover("", w, defaultProfile())
		if nets[0].ControllerValid || !hasDiag(diags, diagnostics.CodeInvalidControllerMultiblock) {
			t.Fatalf("expected 8-long controller line to fail maxSpan=7, got valid=%v diags=%+v", nets[0].ControllerValid, diags)
		}
	})

	t.Run("controller cross shape (>1 opposite axes) fails while hollow 3x3 frame succeeds", func(t *testing.T) {
		// Plus-cross in XZ plane around (0,0,0) -> (0,0,0) has both N&S and E&W -> 2 opposite axes -> invalid!
		crossBlocks := []domain.PlacedBlock{
			{ID: "center", BlockDefID: "ae2:controller", X: 0, Y: 0, Z: 0},
			{ID: "n", BlockDefID: "ae2:controller", X: 0, Y: 0, Z: -1},
			{ID: "s", BlockDefID: "ae2:controller", X: 0, Y: 0, Z: 1},
			{ID: "e", BlockDefID: "ae2:controller", X: 1, Y: 0, Z: 0},
			{ID: "w", BlockDefID: "ae2:controller", X: -1, Y: 0, Z: 0},
		}
		wCross := world.Build("", crossBlocks, nil)
		netsCross, diagsCross := Discover("", wCross, defaultProfile())
		if netsCross[0].ControllerValid || !hasDiag(diagsCross, diagnostics.CodeInvalidControllerMultiblock) {
			t.Fatal("expected plus-cross controller shape to fail axis rule")
		}

		// Hollow 3x3 ring in XZ plane (8 perimeter blocks, empty center (0,0,0)) -> every block has <= 1 opposite axis!
		var ringBlocks []domain.PlacedBlock
		for x := -1; x <= 1; x++ {
			for z := -1; z <= 1; z++ {
				if x == 0 && z == 0 {
					continue
				}
				ringBlocks = append(ringBlocks, domain.PlacedBlock{
					ID: fmt.Sprintf("r-%d-%d", x, z), BlockDefID: "ae2:controller", X: x, Y: 0, Z: z,
				})
			}
		}
		wRing := world.Build("", ringBlocks, nil)
		netsRing, diagsRing := Discover("", wRing, defaultProfile())
		if !netsRing[0].ControllerValid || len(diagsRing) != 0 {
			t.Fatalf("expected hollow 3x3 controller ring to be valid, got valid=%v diags=%+v", netsRing[0].ControllerValid, diagsRing)
		}
	})
}

func TestCraftingCPUMultiblockDiscovery(t *testing.T) {
	t.Run("valid 1x2x1 CPU with 4k storage and co-processor", func(t *testing.T) {
		w := world.Build("", []domain.PlacedBlock{
			{ID: "cable", BlockDefID: "ae2:fluix_cable", X: 0, Y: 0, Z: 0},
			{ID: "s4k", BlockDefID: "ae2:crafting_storage_4k", X: 1, Y: 0, Z: 0},
			{ID: "coproc", BlockDefID: "ae2:crafting_co_processor", X: 1, Y: 1, Z: 0},
		}, nil)
		nets, diags := Discover("", w, defaultProfile())
		if len(diags) != 0 {
			t.Fatalf("unexpected diagnostics: %+v", diags)
		}
		if len(nets[0].CraftingCPUs) != 1 {
			t.Fatalf("expected 1 valid CPU, got %d", len(nets[0].CraftingCPUs))
		}
		cpu := nets[0].CraftingCPUs[0]
		if cpu.ID != "cpu-1" || cpu.TotalBytes != 4096 || cpu.CoProcessors != 1 {
			t.Fatalf("unexpected CPU properties: %+v", cpu)
		}
	})

	t.Run("L-shaped CPU fails rectangular prism rule", func(t *testing.T) {
		w := world.Build("", []domain.PlacedBlock{
			{ID: "u1", BlockDefID: "ae2:crafting_storage_1k", X: 0, Y: 0, Z: 0},
			{ID: "u2", BlockDefID: "ae2:crafting_unit", X: 1, Y: 0, Z: 0},
			{ID: "u3", BlockDefID: "ae2:crafting_unit", X: 0, Y: 1, Z: 0},
		}, nil)
		nets, diags := Discover("", w, defaultProfile())
		if len(nets[0].CraftingCPUs) != 0 || !hasDiag(diags, diagnostics.CodeInvalidCraftingCPUMultiblock) {
			t.Fatalf("expected L-shaped CPU to fail with INVALID_CRAFTING_CPU_MULTIBLOCK, got cpus=%d diags=%+v", len(nets[0].CraftingCPUs), diags)
		}
	})

	t.Run("CPU with 0 storage bytes emits NO_VALID_CRAFTING_CPU warning", func(t *testing.T) {
		w := world.Build("", []domain.PlacedBlock{
			{ID: "u1", BlockDefID: "ae2:crafting_unit", X: 0, Y: 0, Z: 0},
			{ID: "u2", BlockDefID: "ae2:crafting_co_processor", X: 0, Y: 1, Z: 0},
		}, nil)
		nets, diags := Discover("", w, defaultProfile())
		if len(nets[0].CraftingCPUs) != 0 || !hasDiag(diags, diagnostics.CodeNoValidCraftingCPU) {
			t.Fatalf("expected storage-less CPU to emit NO_VALID_CRAFTING_CPU warning, got cpus=%d diags=%+v", len(nets[0].CraftingCPUs), diags)
		}
	})
}
