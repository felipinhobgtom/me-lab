package channels

import (
	"fmt"
	"testing"

	"github.com/felipinho/me-lab/internal/domain"
	"github.com/felipinho/me-lab/internal/simulation/diagnostics"
	"github.com/felipinho/me-lab/internal/simulation/network"
	"github.com/felipinho/me-lab/internal/simulation/world"
)

func stdProfile() domain.CompatibilityProfile {
	return domain.CompatibilityProfile{
		ID: "ae2-1.21.1-default",
		Config: domain.ProfileConfig{
			NormalCableChannels:    8,
			DenseCableChannels:     32,
			ControllerFaceChannels: 32,
			AdHocChannelLimit:      8,
			ControllerMaxSpan:      7,
		},
	}
}

func countDiagCode(diags []domain.Diagnostic, code string) int {
	n := 0
	for _, d := range diags {
		if d.Code == code {
			n++
		}
	}
	return n
}

func TestAdHocNetworkWithinAndOverLimit(t *testing.T) {
	// 8 devices on a smart cable -> all 8 get channels, smart cable reports 8/8.
	blocks8 := []domain.PlacedBlock{
		{ID: "sc-0", BlockDefID: "ae2:smart_cable", X: 0, Y: 0, Z: 0},
		{ID: "sc-1", BlockDefID: "ae2:smart_cable", X: 1, Y: 0, Z: 0},
	}
	coords := []domain.Coord3D{
		{X: 0, Y: 1, Z: 0}, {X: 0, Y: -1, Z: 0}, {X: 0, Y: 0, Z: 1}, {X: 0, Y: 0, Z: -1},
		{X: 1, Y: 1, Z: 0}, {X: 1, Y: -1, Z: 0}, {X: 1, Y: 0, Z: 1}, {X: 1, Y: 0, Z: -1},
	}
	for i, c := range coords {
		blocks8 = append(blocks8, domain.PlacedBlock{
			ID: fmt.Sprintf("dev-%d", i), BlockDefID: "ae2:drive", X: c.X, Y: c.Y, Z: c.Z,
		})
	}

	w8 := world.Build("", blocks8, nil)
	nets8, _ := network.Discover("", w8, stdProfile())
	alloc8 := Allocate("", w8, nets8, stdProfile())

	if len(alloc8.Diagnostics) != 0 {
		t.Fatalf("expected 0 diagnostics for 8-device ad-hoc network, got %+v", alloc8.Diagnostics)
	}
	if alloc8.CableOverlays[0].ChannelsUsed != 8 || alloc8.CableOverlays[0].Overloaded {
		t.Fatalf("expected smart cable to report 8/8 non-overloaded, got %+v", alloc8.CableOverlays[0])
	}

	// Add a 9th device -> exceeds 8-channel ad-hoc limit -> ALL 9 devices lose channels!
	blocks9 := append(blocks8,
		domain.PlacedBlock{ID: "sc-2", BlockDefID: "ae2:smart_cable", X: 2, Y: 0, Z: 0},
		domain.PlacedBlock{ID: "dev-8", BlockDefID: "ae2:terminal", X: 2, Y: 1, Z: 0},
	)
	w9 := world.Build("", blocks9, nil)
	nets9, _ := network.Discover("", w9, stdProfile())
	alloc9 := Allocate("", w9, nets9, stdProfile())

	if countDiagCode(alloc9.Diagnostics, diagnostics.CodeNetworkNoController) != 1 {
		t.Fatalf("expected NETWORK_NO_CONTROLLER diagnostic, got %+v", alloc9.Diagnostics)
	}
	if countDiagCode(alloc9.Diagnostics, diagnostics.CodeDeviceNoChannel) != 9 {
		t.Fatalf("expected all 9 devices to lose channels on overloaded ad-hoc net, got %d", countDiagCode(alloc9.Diagnostics, diagnostics.CodeDeviceNoChannel))
	}
	for _, dr := range alloc9.DeviceRoutes {
		if dr.HasChannel {
			t.Fatalf("expected HasChannel=false on overloaded ad-hoc device %s", dr.BlockID)
		}
	}
}

func TestAE2BottleneckNormalCableBetweenDenseAndController(t *testing.T) {
	// Classic AE2 bottleneck: Controller(0,0,0) -> FluixCable(1,0,0) [cap 8!] -> DenseCable spine -> 9 Drives.
	blocks := []domain.PlacedBlock{
		{ID: "ctrl", BlockDefID: "ae2:controller", X: 0, Y: 0, Z: 0},
		{ID: "bottleneck", BlockDefID: "ae2:fluix_cable", X: 1, Y: 0, Z: 0},
	}
	for i := range 9 {
		x := 2 + i
		blocks = append(blocks,
			domain.PlacedBlock{ID: fmt.Sprintf("dense-%d", i), BlockDefID: "ae2:dense_smart_cable", X: x, Y: 0, Z: 0},
			domain.PlacedBlock{ID: fmt.Sprintf("drive-%d", i), BlockDefID: "ae2:drive", X: x, Y: 1, Z: 0},
		)
	}

	w := world.Build("", blocks, nil)
	nets, _ := network.Discover("", w, stdProfile())
	alloc := Allocate("", w, nets, stdProfile())

	// Closest 8 drives (drive-0 .. drive-7) get channels; drive-8 fails!
	for i := range 8 {
		id := fmt.Sprintf("drive-%d", i)
		if !alloc.HasChannelByBlockID[id] {
			t.Fatalf("expected %s to acquire channel", id)
		}
	}
	if alloc.HasChannelByBlockID["drive-8"] {
		t.Fatal("expected furthest drive-8 to fail due to 8-channel bottleneck")
	}

	if countDiagCode(alloc.Diagnostics, diagnostics.CodeChannelCapacityExceeded) != 1 {
		t.Fatalf("expected 1 CHANNEL_CAPACITY_EXCEEDED on bottleneck cable, got %+v", alloc.Diagnostics)
	}
	if countDiagCode(alloc.Diagnostics, diagnostics.CodeDeviceNoChannel) != 1 {
		t.Fatalf("expected 1 DEVICE_NO_CHANNEL for drive-8, got %+v", alloc.Diagnostics)
	}

	// Verify bottleneck cable overlay is marked Overloaded=true with 8/8.
	foundBottleneck := false
	for _, co := range alloc.CableOverlays {
		if co.BlockID == "bottleneck" {
			foundBottleneck = true
			if co.ChannelsUsed != 8 || co.Capacity != 8 || !co.Overloaded {
				t.Fatalf("unexpected bottleneck overlay: %+v", co)
			}
		}
	}
	if !foundBottleneck {
		t.Fatal("missing bottleneck cable overlay")
	}
}

func TestThreeTierPreferenceDenseTrunkOverNormalShortcut(t *testing.T) {
	// Controller at (0,0,0)
	// Path A (Dense trunk): (0,1,0) -> (0,2,0) -> (1,2,0) -> (2,2,0) -> (2,1,0) [Dense cables] -> SmartCable(2,0,0) -> Drive(3,0,0)
	// Path B (Shorter normal-cable bridge): (1,0,0) [Fluix] -> SmartCable(2,0,0) -> Drive(3,0,0)
	// AE2 3-tier rule: SmartCable(2,0,0) touches Dense(2,1,0) (1 normal hop + dense hops) vs Fluix(1,0,0) (2 normal hops),
	// so Drive MUST route via Dense trunk first, leaving Fluix(1,0,0) at 0 channels!
	blocks := []domain.PlacedBlock{
		{ID: "ctrl", BlockDefID: "ae2:controller", X: 0, Y: 0, Z: 0},
		{ID: "fluix-shortcut", BlockDefID: "ae2:fluix_cable", X: 1, Y: 0, Z: 0},
		{ID: "smart-branch", BlockDefID: "ae2:smart_cable", X: 2, Y: 0, Z: 0},
		{ID: "drive", BlockDefID: "ae2:drive", X: 3, Y: 0, Z: 0},
		{ID: "d1", BlockDefID: "ae2:dense_cable", X: 0, Y: 1, Z: 0},
		{ID: "d2", BlockDefID: "ae2:dense_cable", X: 0, Y: 2, Z: 0},
		{ID: "d3", BlockDefID: "ae2:dense_cable", X: 1, Y: 2, Z: 0},
		{ID: "d4", BlockDefID: "ae2:dense_cable", X: 2, Y: 2, Z: 0},
		{ID: "d5", BlockDefID: "ae2:dense_cable", X: 2, Y: 1, Z: 0},
	}

	w := world.Build("", blocks, nil)
	nets, _ := network.Discover("", w, stdProfile())
	alloc := Allocate("", w, nets, stdProfile())

	for _, co := range alloc.CableOverlays {
		if co.BlockID == "fluix-shortcut" && co.ChannelsUsed != 0 {
			t.Fatalf("expected 3-tier routing to prefer Dense trunk over Fluix shortcut, got shortcut used=%d", co.ChannelsUsed)
		}
		if co.BlockID == "d1" && co.ChannelsUsed != 1 {
			t.Fatalf("expected Dense trunk d1 to carry 1 channel, got %d", co.ChannelsUsed)
		}
	}
}

func TestColoredBranchesAndInfiniteChannelsMode(t *testing.T) {
	// Two adjacent colored smart cable branches (blue and red) carry 8 devices each = 16 devices
	// without cross-connecting, feeding into a fluix Dense Smart Cable into Controller!
	blocks := []domain.PlacedBlock{
		{ID: "ctrl", BlockDefID: "ae2:controller", X: 0, Y: 0, Z: 0},
		{ID: "trunk-0", BlockDefID: "ae2:dense_smart_cable", X: 1, Y: 0, Z: 0},
		{ID: "trunk-1", BlockDefID: "ae2:dense_smart_cable", X: 1, Y: 0, Z: 1},
	}
	for i := range 8 {
		x := 2 + i
		blocks = append(blocks,
			domain.PlacedBlock{ID: fmt.Sprintf("blue-%d", i), BlockDefID: "ae2:smart_cable", X: x, Y: 0, Z: 0, Config: map[string]any{"color": "blue"}},
			domain.PlacedBlock{ID: fmt.Sprintf("red-%d", i), BlockDefID: "ae2:smart_cable", X: x, Y: 0, Z: 1, Config: map[string]any{"color": "red"}},
			domain.PlacedBlock{ID: fmt.Sprintf("db-%d", i), BlockDefID: "ae2:terminal", X: x, Y: 1, Z: 0},
			domain.PlacedBlock{ID: fmt.Sprintf("dr-%d", i), BlockDefID: "ae2:terminal", X: x, Y: 1, Z: 1},
		)
	}

	w := world.Build("", blocks, nil)
	nets, _ := network.Discover("", w, stdProfile())
	alloc := Allocate("", w, nets, stdProfile())

	if len(alloc.Diagnostics) != 0 {
		t.Fatalf("expected all 16 devices on parallel colored branches to succeed, got %+v", alloc.Diagnostics)
	}
	if alloc.ByNetwork["net-1"].ChannelsUsed != 16 {
		t.Fatalf("expected 16 channels used on net-1, got %d", alloc.ByNetwork["net-1"].ChannelsUsed)
	}

	// Also verify InfiniteChannels profile allows > 8 devices even without a controller!
	infProf := stdProfile()
	infProf.Config.InfiniteChannels = true
	var adHocMany []domain.PlacedBlock
	for i := range 12 {
		adHocMany = append(adHocMany,
			domain.PlacedBlock{ID: fmt.Sprintf("c-%d", i), BlockDefID: "ae2:fluix_cable", X: i, Y: 0, Z: 0},
			domain.PlacedBlock{ID: fmt.Sprintf("t-%d", i), BlockDefID: "ae2:terminal", X: i, Y: 1, Z: 0},
		)
	}
	wInf := world.Build("", adHocMany, nil)
	netsInf, _ := network.Discover("", wInf, infProf)
	allocInf := Allocate("", wInf, netsInf, infProf)
	if len(allocInf.Diagnostics) != 0 {
		t.Fatalf("expected 0 diagnostics in infinite channels mode, got %+v", allocInf.Diagnostics)
	}
}
