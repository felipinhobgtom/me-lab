package export

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/felipinho/me-lab/internal/domain"
)

func mkBlock(id, defID string, x, y, z int, facing domain.Direction, label string, cfg map[string]any) domain.PlacedBlock {
	return domain.PlacedBlock{
		ID:         id,
		ProjectID:  "proj-001",
		BlockDefID: defID,
		X:          x,
		Y:          y,
		Z:          z,
		Facing:     facing,
		Config:     cfg,
		Label:      label,
	}
}

func mkDef(id, name, category string, fidelity domain.Fidelity) domain.BlockDefinition {
	return domain.BlockDefinition{
		ID:        id,
		ProfileID: "ae2-1.21.1-default",
		Name:      name,
		Category:  category,
		Fidelity:  fidelity,
	}
}

func TestBuildBlueprint_DeterministicYZXOrdering(t *testing.T) {
	project := domain.Project{
		ID:        "proj-001",
		UserID:    "user-001",
		Name:      "Deterministic Sort Check",
		ProfileID: "ae2-1.21.1-default",
	}
	profile := domain.CompatibilityProfile{
		ID:               "ae2-1.21.1-default",
		Name:             "AE2 1.21.1 Default",
		MinecraftVersion: "1.21.1",
		AE2Version:       "19.2.1",
		ChannelMode:      "default",
	}
	summary := &domain.SimulationSummary{
		Networks:          1,
		OnlineNetworks:    1,
		ChannelsUsed:      3,
		ChannelsAvailable: 32,
	}

	inputA := []domain.PlacedBlock{
		mkBlock("b-top", "ae2:molecular_assembler", 0, 2, 0, domain.North, "", nil),
		mkBlock("b-mid-z2", "ae2:smart_cable", 1, 0, 2, domain.North, "", nil),
		mkBlock("b-neg-y", "ae2:energy_acceptor", 5, -1, 5, domain.Up, "", nil),
		mkBlock("b-mid-x0", "ae2:controller", 0, 0, 0, domain.North, "", nil),
		mkBlock("b-mid-x-neg", "ae2:fluix_cable", -2, 0, 2, domain.North, "", nil),
		mkBlock("b-layer1", "ae2:pattern_provider", 0, 1, 0, domain.Up, "", nil),
	}
	origFirstID := inputA[0].ID

	inputB := []domain.PlacedBlock{
		inputA[4],
		inputA[2],
		inputA[5],
		inputA[0],
		inputA[3],
		inputA[1],
	}

	bpA := BuildBlueprint(project, profile, inputA, nil, nil, summary)
	bpB := BuildBlueprint(project, profile, inputB, nil, nil, summary)

	if bpA.SchemaVersion != "1.0" {
		t.Fatalf("expected schema_version 1.0, got %q", bpA.SchemaVersion)
	}
	if bpA.ExportedAt.IsZero() {
		t.Fatal("expected non-zero ExportedAt timestamp")
	}
	if inputA[0].ID != origFirstID {
		t.Fatalf("BuildBlueprint mutated input slice: got %q, want %q", inputA[0].ID, origFirstID)
	}

	// Mutating original summary must not mutate the Blueprint's snapshot.
	summary.ChannelsUsed = 999
	if bpA.ValidationSummary == nil || bpA.ValidationSummary.ChannelsUsed != 3 {
		t.Fatalf("expected ValidationSummary deep copy with ChannelsUsed=3, got %+v", bpA.ValidationSummary)
	}

	wantOrder := []string{
		"b-neg-y",     // (5, -1, 5)
		"b-mid-x0",    // (0, 0, 0)
		"b-mid-x-neg", // (-2, 0, 2)
		"b-mid-z2",    // (1, 0, 2)
		"b-layer1",    // (0, 1, 0)
		"b-top",       // (0, 2, 0)
	}
	if len(bpA.Blocks) != len(wantOrder) {
		t.Fatalf("got %d blocks, want %d", len(bpA.Blocks), len(wantOrder))
	}
	for i, wantID := range wantOrder {
		if bpA.Blocks[i].ID != wantID {
			t.Errorf("index %d: got block %q at %s, want %q", i, bpA.Blocks[i].ID, bpA.Blocks[i].Coord(), wantID)
		}
	}

	// Normalize timestamps and verify identical JSON output across shuffled inputs and empty-array serialization.
	fixedTime := time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)
	bpA.ExportedAt = fixedTime
	bpB.ExportedAt = fixedTime

	jsonA, err := json.Marshal(bpA)
	if err != nil {
		t.Fatalf("marshal bpA: %v", err)
	}
	jsonB, err := json.Marshal(bpB)
	if err != nil {
		t.Fatalf("marshal bpB: %v", err)
	}
	if string(jsonA) != string(jsonB) {
		t.Fatalf("blueprints from shuffled inputs differed:\nA: %s\nB: %s", string(jsonA), string(jsonB))
	}
	if !strings.Contains(string(jsonA), `"patterns":[]`) {
		t.Errorf("expected nil patterns to serialize as [], got: %s", string(jsonA))
	}
	if !strings.Contains(string(jsonA), `"inventory":[]`) {
		t.Errorf("expected nil inventory to serialize as [], got: %s", string(jsonA))
	}
}

func TestRenderBuildSheet_BOMAndSections(t *testing.T) {
	project := domain.Project{
		ID:          "proj-001",
		UserID:      "user-001",
		Name:        "Main Base Autocrafting",
		Description: "Starter controller, storage priority, and autocrafting loop.",
		ProfileID:   "ae2-1.21.1-default",
	}
	profile := domain.CompatibilityProfile{
		ID:               "ae2-1.21.1-default",
		Name:             "AE2 1.21.1 Default",
		MinecraftVersion: "1.21.1",
		AE2Version:       "19.2.1",
		ChannelMode:      "default",
	}
	summary := &domain.SimulationSummary{
		Networks:          1,
		OnlineNetworks:    1,
		ChannelsUsed:      5,
		ChannelsAvailable: 32,
		EnergyStored:      8000,
		EnergyCapacity:    8000,
		EnergyGeneration:  400,
		EnergyConsumption: 18,
		ItemsStored:       192,
		StorageBytesUsed:  56,
		StorageCapacity:   9216,
		DiagnosticsCount:  1,
	}

	catalog := map[string]domain.BlockDefinition{
		"ae2:controller":            mkDef("ae2:controller", "ME Controller", "network", domain.FidelityExact),
		"ae2:energy_acceptor":       mkDef("ae2:energy_acceptor", "Energy Acceptor", "energy", domain.FidelityExact),
		"ae2:dense_smart_cable":     mkDef("ae2:dense_smart_cable", "Fluix Dense Smart Cable", "cables", domain.FidelityExact),
		"ae2:smart_cable":           mkDef("ae2:smart_cable", "Fluix Smart Cable", "cables", domain.FidelityExact),
		"ae2:drive":                 mkDef("ae2:drive", "ME Drive", "storage", domain.FidelityExact),
		"ae2:pattern_provider":      mkDef("ae2:pattern_provider", "ME Pattern Provider", "crafting", domain.FidelityExact),
		"ae2:molecular_assembler":   mkDef("ae2:molecular_assembler", "Molecular Assembler", "crafting", domain.FidelityExact),
		"ae2:crafting_storage_4k":   mkDef("ae2:crafting_storage_4k", "4k Crafting Storage", "crafting", domain.FidelityExact),
		"ae2:crafting_co_processor": mkDef("ae2:crafting_co_processor", "Crafting Co-Processing Unit", "crafting", domain.FidelityExact),
		"ae2:p2p_tunnel":            mkDef("ae2:p2p_tunnel", "ME P2P Tunnel", "network", domain.FidelityUnsupported),
	}

	cellItems := []map[string]any{
		{"item_id": "minecraft:iron_ingot", "quantity": 64},
	}
	driveCells := []map[string]any{
		{
			"tier":      "1k",
			"partition": []string{"minecraft:cobblestone"},
			"inverter":  false,
		},
		{
			"tier":      "4k",
			"partition": []string{"minecraft:dirt"},
			"inverter":  true,
			"items":     cellItems,
		},
		{
			"tier": "4k",
		},
	}

	providerPatterns := []map[string]any{
		{
			"id":           "pat-planks",
			"pattern_type": "crafting",
			"inputs":       []domain.ItemStack{{ItemID: "minecraft:oak_log", Quantity: 1}},
			"outputs":      []domain.ItemStack{{ItemID: "minecraft:oak_planks", Quantity: 4}},
		},
		{
			"id":           "pat-ingot",
			"pattern_type": "processing",
			"inputs":       []domain.ItemStack{{ItemID: "minecraft:raw_iron", Quantity: 1}},
			"outputs":      []domain.ItemStack{{ItemID: "minecraft:iron_ingot", Quantity: 1}},
		},
	}

	blocks := []domain.PlacedBlock{
		mkBlock("b-ma", "ae2:molecular_assembler", -1, 2, 2, domain.North, "", nil),
		mkBlock("b-coproc", "ae2:crafting_co_processor", 0, 2, 2, domain.North, "", nil),
		mkBlock("b-pp", "ae2:pattern_provider", -1, 1, 2, domain.Up, "Plank Crafter", map[string]any{
			"priority":      5,
			"variant":       "directional",
			"blocking_mode": true,
			"lock_mode":     "until_result_returned",
			"patterns":      providerPatterns,
		}),
		mkBlock("b-cpu4k", "ae2:crafting_storage_4k", 0, 1, 2, domain.North, "", nil),
		mkBlock("b-p2p", "ae2:p2p_tunnel", 3, 0, 2, domain.West, "", nil),
		mkBlock("b-drive", "ae2:drive", 2, 0, 2, domain.North, "Primary Storage", map[string]any{
			"priority": 10,
			"cells":    driveCells,
		}),
		mkBlock("b-sc-blue", "ae2:smart_cable", 1, 0, 2, domain.North, "", map[string]any{"color": "blue"}),
		mkBlock("b-sc-purple", "ae2:smart_cable", -1, 0, 2, domain.North, "", map[string]any{"color": "purple"}),
		mkBlock("b-dsc-2", "ae2:dense_smart_cable", 0, 0, 2, domain.North, "", map[string]any{
			"color":          "fluix",
			"anchored_faces": []string{"west"},
		}),
		mkBlock("b-dsc-1", "ae2:dense_smart_cable", 0, 0, 1, domain.North, "", map[string]any{"color": "fluix"}),
		mkBlock("b-ctrl", "ae2:controller", 0, 0, 0, domain.North, "Core", nil),
		mkBlock("b-ea", "ae2:energy_acceptor", -1, 0, 0, domain.East, "", map[string]any{"fe_input_per_tick": 800}),
	}

	patterns := []domain.ProjectPattern{
		// Duplicate ID already mounted on Pattern Provider b-pp; must be deduplicated in BOM.
		{
			ID:          "pat-planks",
			ProjectID:   "proj-001",
			PatternType: "crafting",
			Inputs:      []domain.ItemStack{{ItemID: "minecraft:oak_log", Quantity: 1}},
			Outputs:     []domain.ItemStack{{ItemID: "minecraft:oak_planks", Quantity: 4}},
		},
		// Project-level pattern not yet assigned to a provider.
		{
			ID:          "pat-stick",
			ProjectID:   "proj-001",
			PatternType: "crafting",
			Inputs:      []domain.ItemStack{{ItemID: "minecraft:oak_planks", Quantity: 2}},
			Outputs:     []domain.ItemStack{{ItemID: "minecraft:stick", Quantity: 4}},
		},
	}

	inventory := []domain.ProjectInventoryEntry{
		{
			ID:        "inv-1",
			ProjectID: "proj-001",
			ItemID:    "minecraft:oak_log",
			Quantity:  128,
		},
	}

	bp := BuildBlueprint(project, profile, blocks, patterns, inventory, summary)
	bp.ExportedAt = time.Date(2026, 11, 2, 15, 0, 0, 0, time.UTC)

	sheet := RenderBuildSheet(bp, catalog)

	requiredSnippets := []string{
		"# ME Lab Build Sheet: Main Base Autocrafting",
		"| Minecraft Version | `1.21.1` |",
		"| AE2 Version | `19.2.1` |",
		"| Channel Mode | `default` |",
		"## 1. Bill of Materials",
		"| ME Controller | `ae2:controller` | network | `exact` | 1 |",
		"| ME P2P Tunnel | `ae2:p2p_tunnel` | network | `unsupported` | 1 |",
		"| Fluix Dense Smart Cable | `ae2:dense_smart_cable` | cables | `exact` | 2 |",
		"| Fluix Smart Cable | `ae2:smart_cable` | cables | `exact` | 2 |",
		"| ME Drive | `ae2:drive` | storage | `exact` | 1 |",
		"| `1k` | 1K ME Item Storage Cell | 1 |",
		"| `4k` | 4K ME Item Storage Cell | 2 |",
		"| **Total** | **All Storage Cells** | **3** |",
		"| Crafting Patterns | Molecular Assembler | 2 |",
		"| Processing Patterns | External Machine / Furnace | 1 |",
		"| **Total Blank Patterns** | **Pattern Encoding Terminal** | **3** |",
		"## 2. Step-by-Step Y-Layer Placement Guide",
		"### Layer Y = 0 (8 blocks)",
		"### Layer Y = 1 (2 blocks)",
		"### Layer Y = 2 (2 blocks)",
		"fe_input=800 FE/t (400 AE/t)",
		"anchors=[west]",
		"variant=directional (push=up); priority=5; blocking=true; lock=until_result_returned; patterns=2",
		"UNSUPPORTED IN SIMULATOR",
		"## 3. Storage Cell & Partition Table",
		"Whitelist",
		"Blacklist (Inverter)",
		"64x `minecraft:iron_ingot`",
		"## 4. Encoded Patterns Table",
		"`pat-planks`",
		"`pat-ingot`",
		"`pat-stick`",
		"## 5. Pre-Flight Checklist Before Powering On",
		"Latest validation reported **1 diagnostic(s)**",
	}

	for _, snippet := range requiredSnippets {
		if !strings.Contains(sheet, snippet) {
			t.Errorf("build sheet missing expected snippet %q\n--- FULL SHEET ---\n%s", snippet, sheet)
		}
	}

	// Verify Y layers appear in strict ascending order.
	idxY0 := strings.Index(sheet, "### Layer Y = 0")
	idxY1 := strings.Index(sheet, "### Layer Y = 1")
	idxY2 := strings.Index(sheet, "### Layer Y = 2")
	if !(idxY0 >= 0 && idxY0 < idxY1 && idxY1 < idxY2) {
		t.Fatalf("Y-layer headings not in ascending order: Y0=%d, Y1=%d, Y2=%d", idxY0, idxY1, idxY2)
	}

	// Verify reversed input slice yields byte-identical build sheet.
	reversed := make([]domain.PlacedBlock, len(blocks))
	for i := range blocks {
		reversed[i] = blocks[len(blocks)-1-i]
	}
	bpRev := BuildBlueprint(project, profile, reversed, patterns, inventory, summary)
	bpRev.ExportedAt = bp.ExportedAt
	sheetRev := RenderBuildSheet(bpRev, catalog)
	if sheet != sheetRev {
		t.Fatal("expected byte-identical build sheet regardless of input block order")
	}
}

func TestRenderBuildSheet_EmptyBlueprint(t *testing.T) {
	bp := BuildBlueprint(
		domain.Project{ID: "empty-proj", Name: "Empty Grid"},
		domain.CompatibilityProfile{ID: "ae2-1.21.1-default", MinecraftVersion: "1.21.1", AE2Version: "19.2.1", ChannelMode: "default"},
		nil,
		nil,
		nil,
		nil,
	)
	sheet := RenderBuildSheet(bp, nil)
	if !strings.Contains(sheet, "_No blocks placed in this project._") {
		t.Fatalf("expected empty blocks placeholder in sheet:\n%s", sheet)
	}
}
