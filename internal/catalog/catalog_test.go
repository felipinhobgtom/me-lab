package catalog

import (
	"strings"
	"testing"

	"github.com/felipinho/me-lab/internal/domain"
)

func TestRegistryProfilesAndCellTiers(t *testing.T) {
	t.Parallel()

	reg, err := NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry failed: %v", err)
	}

	profiles := reg.Profiles()
	if len(profiles) != 3 {
		t.Fatalf("expected 3 compatibility profiles, got %d", len(profiles))
	}

	defProf, ok := reg.GetProfile("ae2-1.21.1-default")
	if !ok {
		t.Fatal("missing ae2-1.21.1-default profile")
	}
	if defProf.Config.NormalCableChannels != 8 || defProf.Config.DenseCableChannels != 32 || defProf.Config.AdHocChannelLimit != 8 || defProf.Config.InfiniteChannels {
		t.Fatalf("unexpected default profile channels: %+v", defProf.Config)
	}
	if defProf.Config.ControllerEnergyPerBlock != 6 ||
		defProf.Config.ControllerEnergyBuffer != 8000 ||
		defProf.Config.NodeBaseEnergyBuffer != 25 ||
		defProf.Config.ChannelHopEnergyCost != 0.0078125 ||
		defProf.Config.FEPerAE != 2 ||
		defProf.Config.MolecularAssemblerTicks != 4 ||
		defProf.Config.MolecularAssemblerActiveE != 20 ||
		defProf.Config.ExternalMachineTicks != 8 ||
		defProf.Config.ItemsPerByte != 8 ||
		defProf.Config.MaxCellTypes != 63 {
		t.Fatalf("unexpected default profile energy/crafting constants: %+v", defProf.Config)
	}

	x2Prof, ok := reg.GetProfile("ae2-1.21.1-x2")
	if !ok {
		t.Fatal("missing ae2-1.21.1-x2 profile")
	}
	if x2Prof.Config.NormalCableChannels != 16 || x2Prof.Config.DenseCableChannels != 64 || x2Prof.Config.AdHocChannelLimit != 16 || x2Prof.Config.InfiniteChannels {
		t.Fatalf("unexpected x2 profile channels: %+v", x2Prof.Config)
	}

	infProf, ok := reg.GetProfile("ae2-1.21.1-infinite")
	if !ok {
		t.Fatal("missing ae2-1.21.1-infinite profile")
	}
	if !infProf.Config.InfiniteChannels {
		t.Fatal("expected infinite_channels=true on ae2-1.21.1-infinite")
	}

	expectedTiers := map[string]domain.CellTierSpec{
		"1k":   {TotalBytes: 1024, MaxTypes: 63, BytesPerType: 8},
		"4k":   {TotalBytes: 4096, MaxTypes: 63, BytesPerType: 32},
		"16k":  {TotalBytes: 16384, MaxTypes: 63, BytesPerType: 128},
		"64k":  {TotalBytes: 65536, MaxTypes: 63, BytesPerType: 512},
		"256k": {TotalBytes: 262144, MaxTypes: 63, BytesPerType: 2048},
	}

	for _, p := range profiles {
		if len(p.Config.References) == 0 {
			t.Errorf("profile %s missing official AE2 references", p.ID)
		}
		for _, ref := range p.Config.References {
			if !strings.HasPrefix(ref, "https://guide.appliedenergistics.org/1.21.1/") {
				t.Errorf("profile %s has unexpected reference %q", p.ID, ref)
			}
		}
		for tier, want := range expectedTiers {
			got, exists := p.Config.CellTiers[tier]
			if !exists {
				t.Errorf("profile %s missing cell tier %s", p.ID, tier)
				continue
			}
			if got != want {
				t.Errorf("profile %s cell tier %s = %+v, want %+v", p.ID, tier, got, want)
			}
		}
	}
}

func TestRegistryBlockLookupAndFidelity(t *testing.T) {
	t.Parallel()

	reg, err := NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry failed: %v", err)
	}

	requiredBlocks := map[string]domain.Fidelity{
		"ae2:fluix_cable":               domain.FidelityExact,
		"ae2:covered_cable":             domain.FidelityExact,
		"ae2:smart_cable":               domain.FidelityExact,
		"ae2:dense_cable":               domain.FidelityExact,
		"ae2:dense_smart_cable":         domain.FidelityExact,
		"ae2:quartz_fiber":              domain.FidelityExact,
		"ae2:cable_anchor":              domain.FidelityExact,
		"ae2:controller":                domain.FidelityExact,
		"ae2:energy_acceptor":           domain.FidelityExact,
		"ae2:energy_cell":               domain.FidelityExact,
		"ae2:dense_energy_cell":         domain.FidelityExact,
		"ae2:creative_energy_cell":      domain.FidelityExact,
		"ae2:drive":                     domain.FidelityExact,
		"ae2:chest":                     domain.FidelityExact,
		"ae2:interface":                 domain.FidelityApproximated,
		"ae2:terminal":                  domain.FidelityExact,
		"ae2:crafting_terminal":         domain.FidelityExact,
		"ae2:pattern_encoding_terminal": domain.FidelityExact,
		"ae2:pattern_access_terminal":   domain.FidelityExact,
		"ae2:import_bus":                domain.FidelityApproximated,
		"ae2:export_bus":                domain.FidelityApproximated,
		"ae2:storage_bus":               domain.FidelityApproximated,
		"ae2:pattern_provider":          domain.FidelityExact,
		"ae2:molecular_assembler":       domain.FidelityExact,
		"ae2:crafting_unit":             domain.FidelityExact,
		"ae2:crafting_storage_1k":       domain.FidelityExact,
		"ae2:crafting_storage_4k":       domain.FidelityExact,
		"ae2:crafting_storage_16k":      domain.FidelityExact,
		"ae2:crafting_storage_64k":      domain.FidelityExact,
		"ae2:crafting_storage_256k":     domain.FidelityExact,
		"ae2:crafting_co_processor":     domain.FidelityExact,
		"ae2:crafting_monitor":          domain.FidelityExact,
		"ae2:inscriber":                 domain.FidelityApproximated,
		"ae2:charger":                   domain.FidelityApproximated,
		"minecraft:furnace":             domain.FidelityApproximated,
		"minecraft:blast_furnace":       domain.FidelityApproximated,
		"minecraft:hopper":              domain.FidelityApproximated,
		"minecraft:barrel":              domain.FidelityApproximated,
		"ae2:p2p_tunnel":                domain.FidelityUnsupported,
		"ae2:quantum_ring":              domain.FidelityUnsupported,
	}

	for _, profile := range reg.Profiles() {
		blocks := reg.BlocksForProfile(profile.ID)
		if len(blocks) != len(requiredBlocks) {
			t.Fatalf("profile %s has %d blocks, want %d", profile.ID, len(blocks), len(requiredBlocks))
		}
		for blockID, wantFidelity := range requiredBlocks {
			b, ok := reg.GetBlock(profile.ID, blockID)
			if !ok {
				t.Fatalf("profile %s missing block %s", profile.ID, blockID)
			}
			if b.ProfileID != profile.ID {
				t.Errorf("block %s ProfileID = %q, want %q", blockID, b.ProfileID, profile.ID)
			}
			if b.Fidelity != wantFidelity {
				t.Errorf("block %s Fidelity = %q, want %q", blockID, b.Fidelity, wantFidelity)
			}
			if !strings.HasPrefix(b.DocReference, "https://guide.appliedenergistics.org/1.21.1/") {
				t.Errorf("block %s missing 1.21.1 guide doc_reference, got %q", blockID, b.DocReference)
			}
			if b.Metadata.Visual.Shape == "" || b.Metadata.Visual.Color == "" {
				t.Errorf("block %s missing visual metadata: %+v", blockID, b.Metadata.Visual)
			}
		}
	}

	// Verify profile-scaled channel capacities.
	defCable, _ := reg.GetBlock("ae2-1.21.1-default", "ae2:fluix_cable")
	x2Cable, _ := reg.GetBlock("ae2-1.21.1-x2", "ae2:fluix_cable")
	if defCable.Metadata.ChannelCapacity != 8 || x2Cable.Metadata.ChannelCapacity != 16 {
		t.Fatalf("expected fluix cable capacity 8 (default) / 16 (x2), got %d / %d",
			defCable.Metadata.ChannelCapacity, x2Cable.Metadata.ChannelCapacity)
	}

	defDense, _ := reg.GetBlock("ae2-1.21.1-default", "ae2:dense_smart_cable")
	x2Dense, _ := reg.GetBlock("ae2-1.21.1-x2", "ae2:dense_smart_cable")
	if defDense.Metadata.ChannelCapacity != 32 || x2Dense.Metadata.ChannelCapacity != 64 {
		t.Fatalf("expected dense smart cable capacity 32 (default) / 64 (x2), got %d / %d",
			defDense.Metadata.ChannelCapacity, x2Dense.Metadata.ChannelCapacity)
	}

	all := reg.AllBlocks()
	if len(all) != 3*len(requiredBlocks) {
		t.Fatalf("AllBlocks() len = %d, want %d", len(all), 3*len(requiredBlocks))
	}

	if _, ok := reg.GetProfile("nonexistent"); ok {
		t.Fatal("expected false for unknown profile")
	}
	if _, ok := reg.GetBlock(DefaultProfileID, "ae2:nonexistent"); ok {
		t.Fatal("expected false for unknown block")
	}
}
