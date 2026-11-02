package storage

import (
	"fmt"
	"testing"

	"github.com/felipinho/me-lab/internal/domain"
	"github.com/felipinho/me-lab/internal/simulation/channels"
	"github.com/felipinho/me-lab/internal/simulation/diagnostics"
	"github.com/felipinho/me-lab/internal/simulation/energy"
	"github.com/felipinho/me-lab/internal/simulation/network"
	"github.com/felipinho/me-lab/internal/simulation/world"
)

func testStorageProfile() domain.CompatibilityProfile {
	return domain.CompatibilityProfile{
		ID: "ae2-1.21.1-default",
		Config: domain.ProfileConfig{
			NormalCableChannels:    8,
			DenseCableChannels:     32,
			ControllerFaceChannels: 32,
			AdHocChannelLimit:      8,
			ItemsPerByte:           8,
			MaxCellTypes:           63,
			CellTiers:              defaultCellTiers(),
		},
	}
}

func buildOnlineStorage(t *testing.T, drives []domain.PlacedBlock, inv []domain.ProjectInventoryEntry) *State {
	t.Helper()
	blocks := []domain.PlacedBlock{
		{ID: "ctrl", BlockDefID: "ae2:controller", X: 0, Y: 0, Z: 0},
		{ID: "creative", BlockDefID: "ae2:creative_energy_cell", X: -1, Y: 0, Z: 0},
	}
	blocks = append(blocks, drives...)

	prof := testStorageProfile()
	w := world.Build("", blocks, nil)
	nets, _ := network.Discover("", w, prof)
	alloc := channels.Allocate("", w, nets, prof)
	eStates, _ := energy.Initialize("", nets, alloc, prof)
	st, diags := Initialize("", nets, alloc, eStates, inv, prof)
	if len(diags) != 0 {
		t.Fatalf("unexpected init diagnostics: %+v", diags)
	}
	return st
}

func TestExactAE2CellByteAndTypeCapacities(t *testing.T) {
	// 1k cell: 1024 B, 8 B/type, 8 items/B -> max 8,128 of 1 item type; 63 types max -> 4,160 items.
	// 4k cell: 4096 B, 32 B/type -> max 32,512 of 1 item type.
	// 64k cell: 65536 B, 512 B/type -> max 520,192 of 1 item type.
	cases := []struct {
		tier         string
		expectedMax1 int
	}{
		{"1k", 8128},
		{"4k", 32512},
		{"64k", 520192},
	}

	for _, tc := range cases {
		st := buildOnlineStorage(t, []domain.PlacedBlock{
			{
				ID: "drive-1", BlockDefID: "ae2:drive", X: 1, Y: 0, Z: 0,
				Config: map[string]any{
					"cells": []map[string]any{{"tier": tc.tier}},
				},
			},
		}, nil)

		rem, diags := st.Insert("", "minecraft:cobblestone", tc.expectedMax1+10)
		if rem != 10 {
			t.Fatalf("tier %s: expected remainder 10 beyond %d capacity, got rem=%d", tc.tier, tc.expectedMax1, rem)
		}
		if len(diags) != 1 || diags[0].Code != diagnostics.CodeStorageFull {
			t.Fatalf("tier %s: expected STORAGE_FULL diagnostic, got %+v", tc.tier, diags)
		}
		if st.Cells[0].BytesUsed() != st.Cells[0].TotalBytes {
			t.Fatalf("tier %s: expected full bytes %d, got %d", tc.tier, st.Cells[0].TotalBytes, st.Cells[0].BytesUsed())
		}
	}

	// Test 63-type limit on a 1k cell: 64th distinct type must be rejected even with plenty of bytes free!
	st63 := buildOnlineStorage(t, []domain.PlacedBlock{
		{
			ID: "drive-63", BlockDefID: "ae2:drive", X: 1, Y: 0, Z: 0,
			Config: map[string]any{"cells": []map[string]any{{"tier": "1k"}}},
		},
	}, nil)
	for i := range 63 {
		rem, diags := st63.Insert("", fmt.Sprintf("minecraft:item_%02d", i), 1)
		if rem != 0 || len(diags) != 0 {
			t.Fatalf("expected type %d to fit, got rem=%d diags=%+v", i, rem, diags)
		}
	}
	rem64, diags64 := st63.Insert("", "minecraft:item_64", 1)
	if rem64 != 1 || len(diags64) != 1 || diags64[0].Code != diagnostics.CodeStorageFull {
		t.Fatalf("expected 64th distinct type to be rejected by 63-type limit, got rem=%d diags=%+v", rem64, diags64)
	}
}

func TestDrivePriorityPartitionsInverterAndOfflineExclusion(t *testing.T) {
	st := buildOnlineStorage(t, []domain.PlacedBlock{
		{
			ID: "drive-low", BlockDefID: "ae2:drive", X: 1, Y: 0, Z: 0,
			Config: map[string]any{
				"priority": 0,
				"cells": []map[string]any{
					{"tier": "4k"},
				},
			},
		},
		{
			ID: "drive-high", BlockDefID: "ae2:drive", X: 0, Y: 1, Z: 0,
			Config: map[string]any{
				"priority": 10,
				"cells": []map[string]any{
					// Slot 0: whitelist only iron_ingot
					{"tier": "4k", "partition": []string{"minecraft:iron_ingot"}, "inverter": false},
					// Slot 1: blacklist cobblestone (inverter = true)
					{"tier": "4k", "partition": []string{"minecraft:cobblestone"}, "inverter": true},
				},
			},
		},
	}, nil)

	// Insert iron_ingot -> goes into drive-high slot 0 (priority 10, partitioned for iron_ingot).
	if rem, _ := st.Insert("", "minecraft:iron_ingot", 32); rem != 0 {
		t.Fatalf("unexpected remainder: %d", rem)
	}
	// Insert gold_ingot -> rejected by drive-high slot 0 (whitelist iron), accepted by drive-high slot 1 (blacklist cobble).
	if rem, _ := st.Insert("", "minecraft:gold_ingot", 16); rem != 0 {
		t.Fatalf("unexpected remainder: %d", rem)
	}
	// Insert cobblestone -> rejected by both cells on drive-high, falls through to drive-low (priority 0)!
	if rem, _ := st.Insert("", "minecraft:cobblestone", 64); rem != 0 {
		t.Fatalf("unexpected remainder: %d", rem)
	}

	for _, c := range st.Cells {
		switch {
		case c.DriveBlock.ID == "drive-high" && c.Slot == 0:
			if c.QuantityOf("minecraft:iron_ingot") != 32 || c.TypesUsed() != 1 {
				t.Fatalf("unexpected drive-high slot 0 items: %+v", c.Items)
			}
		case c.DriveBlock.ID == "drive-high" && c.Slot == 1:
			if c.QuantityOf("minecraft:gold_ingot") != 16 || c.TypesUsed() != 1 {
				t.Fatalf("unexpected drive-high slot 1 items: %+v", c.Items)
			}
		case c.DriveBlock.ID == "drive-low" && c.Slot == 0:
			if c.QuantityOf("minecraft:cobblestone") != 64 || c.TypesUsed() != 1 {
				t.Fatalf("unexpected drive-low slot 0 items: %+v", c.Items)
			}
		}
	}

	// Verify Extract and offline exclusion: if network goes offline, AvailableTotals() is empty and Extract returns 0.
	extracted := st.Extract("minecraft:iron_ingot", 12)
	if extracted != 12 {
		t.Fatalf("expected to extract 12 iron_ingot, got %d", extracted)
	}

	st.SyncOnlineStatus([]*energy.NetworkEnergy{{NetworkID: "net-1", Online: false}}, nil)
	if len(st.AvailableTotals()) != 0 {
		t.Fatalf("expected 0 available items when network offline, got %+v", st.AvailableTotals())
	}
	if got := st.Extract("minecraft:iron_ingot", 1); got != 0 {
		t.Fatalf("expected 0 extracted when offline, got %d", got)
	}
}
