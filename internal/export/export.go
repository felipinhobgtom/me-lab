package export

import (
	"cmp"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/felipinho/me-lab/internal/domain"
)

// SchemaVersion identifies the wire format version of exported ME Lab JSON blueprints.
const SchemaVersion = "1.0"

// Blueprint is a portable, deterministic snapshot of an ME Lab project and its validation telemetry.
type Blueprint struct {
	SchemaVersion     string                         `json:"schema_version"`
	ExportedAt        time.Time                      `json:"exported_at"`
	Project           domain.Project                 `json:"project"`
	Profile           domain.CompatibilityProfile    `json:"profile"`
	Blocks            []domain.PlacedBlock           `json:"blocks"`
	Patterns          []domain.ProjectPattern        `json:"patterns"`
	Inventory         []domain.ProjectInventoryEntry `json:"inventory"`
	ValidationSummary *domain.SimulationSummary      `json:"validation_summary,omitempty"`
}

type bomBlockRow struct {
	blockDefID string
	name       string
	category   string
	fidelity   domain.Fidelity
	quantity   int
}

type rawDriveCell struct {
	Tier      string             `json:"tier"`
	Partition []string           `json:"partition"`
	Inverter  bool               `json:"inverter"`
	Items     []domain.ItemStack `json:"items"`
}

type driveCellRow struct {
	coord     domain.Coord3D
	slot      int
	tier      string
	priority  int
	partition []string
	inverter  bool
	items     []domain.ItemStack
}

type encodedPatternRow struct {
	id          string
	patternType string
	location    string
	inputs      []domain.ItemStack
	outputs     []domain.ItemStack
}

// BuildBlueprint constructs a versioned Blueprint with blocks sorted deterministically by Y, then Z, then X.
func BuildBlueprint(
	project domain.Project,
	profile domain.CompatibilityProfile,
	blocks []domain.PlacedBlock,
	patterns []domain.ProjectPattern,
	inventory []domain.ProjectInventoryEntry,
	summary *domain.SimulationSummary,
) Blueprint {
	sortedBlocks := sortBlocksDeterministically(blocks)

	sortedPatterns := make([]domain.ProjectPattern, len(patterns))
	copy(sortedPatterns, patterns)
	slices.SortFunc(sortedPatterns, func(a, b domain.ProjectPattern) int {
		if c := cmp.Compare(a.ID, b.ID); c != 0 {
			return c
		}
		return cmp.Compare(a.PatternType, b.PatternType)
	})

	sortedInventory := make([]domain.ProjectInventoryEntry, len(inventory))
	copy(sortedInventory, inventory)
	slices.SortFunc(sortedInventory, func(a, b domain.ProjectInventoryEntry) int {
		if c := cmp.Compare(a.ItemID, b.ItemID); c != 0 {
			return c
		}
		return cmp.Compare(a.ID, b.ID)
	})

	var summaryCopy *domain.SimulationSummary
	if summary != nil {
		s := *summary
		summaryCopy = &s
	}

	return Blueprint{
		SchemaVersion:     SchemaVersion,
		ExportedAt:        time.Now().UTC(),
		Project:           project,
		Profile:           profile,
		Blocks:            sortedBlocks,
		Patterns:          sortedPatterns,
		Inventory:         sortedInventory,
		ValidationSummary: summaryCopy,
	}
}

// RenderBuildSheet generates a practical Markdown build guide for reproducing a Blueprint in Minecraft.
func RenderBuildSheet(bp Blueprint, catalog map[string]domain.BlockDefinition) string {
	blocks := sortBlocksDeterministically(bp.Blocks)
	bomRows, hasUnsupported := collectBlockBOM(blocks, catalog)
	cellRows, cellTierCounts := collectDriveCells(blocks)
	patternRows, craftingCount, processingCount := collectEncodedPatterns(blocks, bp.Patterns)

	var sb strings.Builder
	sb.Grow(4096)

	projectName := strings.TrimSpace(bp.Project.Name)
	if projectName == "" {
		projectName = "Untitled ME Network"
	}
	schemaVer := bp.SchemaVersion
	if schemaVer == "" {
		schemaVer = SchemaVersion
	}

	sb.WriteString("# ME Lab Build Sheet: ")
	sb.WriteString(escapeMarkdownInline(projectName))
	sb.WriteString("\n\n")

	if desc := strings.TrimSpace(bp.Project.Description); desc != "" {
		sb.WriteString("> ")
		sb.WriteString(strings.ReplaceAll(desc, "\n", "\n> "))
		sb.WriteString("\n\n")
	}

	// Overview & Compatibility Profile
	sb.WriteString("## Project Overview & AE2 Compatibility Profile\n\n")
	sb.WriteString("| Property | Value |\n")
	sb.WriteString("| --- | --- |\n")
	fmt.Fprintf(&sb, "| Project ID | `%s` |\n", nonEmptyOr(bp.Project.ID, "n/a"))
	fmt.Fprintf(&sb, "| Blueprint Schema | `%s` |\n", schemaVer)
	if !bp.ExportedAt.IsZero() {
		fmt.Fprintf(&sb, "| Exported At (UTC) | `%s` |\n", bp.ExportedAt.UTC().Format(time.RFC3339))
	}
	fmt.Fprintf(&sb, "| Compatibility Profile | %s (`%s`) |\n", nonEmptyOr(bp.Profile.Name, "AE2 Default"), nonEmptyOr(bp.Profile.ID, "default"))
	fmt.Fprintf(&sb, "| Minecraft Version | `%s` |\n", nonEmptyOr(bp.Profile.MinecraftVersion, "1.21.1"))
	fmt.Fprintf(&sb, "| AE2 Version | `%s` |\n", nonEmptyOr(bp.Profile.AE2Version, "19.x"))
	fmt.Fprintf(&sb, "| Channel Mode | `%s` |\n", nonEmptyOr(bp.Profile.ChannelMode, "default"))
	fmt.Fprintf(&sb, "| Total Placed Blocks | %d |\n", len(blocks))
	if len(blocks) > 0 {
		minC, maxC := computeBounds(blocks)
		spanX := maxC.X - minC.X + 1
		spanY := maxC.Y - minC.Y + 1
		spanZ := maxC.Z - minC.Z + 1
		fmt.Fprintf(&sb, "| Footprint Bounds | `%s` to `%s` (`%dx%dx%d` X/Y/Z) |\n", minC.String(), maxC.String(), spanX, spanY, spanZ)
	}
	sb.WriteString("\n")

	if s := bp.ValidationSummary; s != nil {
		sb.WriteString("### Validation Telemetry Snapshot\n\n")
		sb.WriteString("| Metric | Value |\n")
		sb.WriteString("| --- | --- |\n")
		fmt.Fprintf(&sb, "| Networks Online | %d / %d |\n", s.OnlineNetworks, s.Networks)
		fmt.Fprintf(&sb, "| Channels Used | %d / %d |\n", s.ChannelsUsed, s.ChannelsAvailable)
		fmt.Fprintf(&sb, "| Energy Balance | +%.1f AE/t gen / -%.1f AE/t drain (%.0f / %.0f AE buffer) |\n",
			s.EnergyGeneration, s.EnergyConsumption, s.EnergyStored, s.EnergyCapacity)
		fmt.Fprintf(&sb, "| Storage Utilization | %d items (%d / %d bytes) |\n",
			s.ItemsStored, s.StorageBytesUsed, s.StorageCapacity)
		fmt.Fprintf(&sb, "| Diagnostics Count | %d |\n\n", s.DiagnosticsCount)
	}

	// Section 1: Bill of Materials
	sb.WriteString("## 1. Bill of Materials\n\n")
	sb.WriteString("### Network Blocks & Cables\n\n")
	if len(bomRows) == 0 {
		sb.WriteString("_No blocks placed in this project._\n\n")
	} else {
		sb.WriteString("| Block | Block ID | Category | Fidelity | Quantity |\n")
		sb.WriteString("| --- | --- | --- | --- | ---: |\n")
		for _, row := range bomRows {
			fmt.Fprintf(&sb, "| %s | `%s` | %s | `%s` | %d |\n",
				escapeTableCell(row.name),
				escapeTableCell(row.blockDefID),
				escapeTableCell(row.category),
				escapeTableCell(string(row.fidelity)),
				row.quantity,
			)
		}
		sb.WriteString("\n")
	}

	sb.WriteString("### Storage Cells\n\n")
	if len(cellRows) == 0 {
		sb.WriteString("_No ME Item Storage Cells configured inside ME Drives._\n\n")
	} else {
		sb.WriteString("| Cell Tier | Item Name | Quantity |\n")
		sb.WriteString("| --- | --- | ---: |\n")
		for _, tc := range orderedTierCounts(cellTierCounts) {
			fmt.Fprintf(&sb, "| `%s` | %s ME Item Storage Cell | %d |\n", tc.tier, strings.ToUpper(tc.tier), tc.count)
		}
		fmt.Fprintf(&sb, "| **Total** | **All Storage Cells** | **%d** |\n\n", len(cellRows))
	}

	sb.WriteString("### Encoded Patterns\n\n")
	sb.WriteString("| Pattern Type | Target Executor | Quantity |\n")
	sb.WriteString("| --- | --- | ---: |\n")
	fmt.Fprintf(&sb, "| Crafting Patterns | Molecular Assembler | %d |\n", craftingCount)
	fmt.Fprintf(&sb, "| Processing Patterns | External Machine / Furnace | %d |\n", processingCount)
	fmt.Fprintf(&sb, "| **Total Blank Patterns** | **Pattern Encoding Terminal** | **%d** |\n\n", len(patternRows))

	// Section 2: Step-by-Step Y-Layer Placement Guide
	sb.WriteString("## 2. Step-by-Step Y-Layer Placement Guide\n\n")
	sb.WriteString("Build from the lowest `Y` layer upward. Coordinates use `(x, y, z)` where `+X = East`, `+Y = Up`, and `+Z = South`.\n\n")
	if len(blocks) == 0 {
		sb.WriteString("_No placement coordinates to display._\n\n")
	} else {
		layers := groupBlocksByY(blocks)
		for _, layer := range layers {
			fmt.Fprintf(&sb, "### Layer Y = %d (%d %s)\n\n", layer.y, len(layer.blocks), pluralize(len(layer.blocks), "block", "blocks"))
			sb.WriteString("| Coordinate (x, y, z) | Block | Facing | Settings & Notes |\n")
			sb.WriteString("| --- | --- | --- | --- |\n")
			for _, b := range layer.blocks {
				displayName := blockDisplayName(b.BlockDefID, catalog)
				if lbl := strings.TrimSpace(b.Label); lbl != "" {
					displayName = fmt.Sprintf("%s (%s)", displayName, lbl)
				}
				facing := string(b.Facing)
				if facing == "" {
					facing = string(domain.North)
				}
				notes := describeBlockSettings(b, catalog)
				fmt.Fprintf(&sb, "| `%s` | %s (`%s`) | `%s` | %s |\n",
					b.Coord().String(),
					escapeTableCell(displayName),
					escapeTableCell(b.BlockDefID),
					escapeTableCell(facing),
					escapeTableCell(notes),
				)
			}
			sb.WriteString("\n")
		}
	}

	// Section 3: Storage Cell & Partition Table
	sb.WriteString("## 3. Storage Cell & Partition Table\n\n")
	if len(cellRows) == 0 {
		sb.WriteString("_No storage cells configured._\n\n")
	} else {
		sb.WriteString("| Drive Coord | Slot | Tier | Drive Priority | Partition Mode | Partition Filter | Initial Items |\n")
		sb.WriteString("| --- | ---: | --- | ---: | --- | --- | --- |\n")
		for _, cell := range cellRows {
			mode := "Unpartitioned"
			filter := "—"
			if len(cell.partition) > 0 {
				if cell.inverter {
					mode = "Blacklist (Inverter)"
				} else {
					mode = "Whitelist"
				}
				filter = "`" + strings.Join(cell.partition, "`, `") + "`"
			}
			initialItems := "Empty"
			if len(cell.items) > 0 {
				initialItems = formatItemStacks(cell.items)
			}
			fmt.Fprintf(&sb, "| `%s` | %d | `%s` | %d | %s | %s | %s |\n",
				cell.coord.String(),
				cell.slot,
				escapeTableCell(cell.tier),
				cell.priority,
				mode,
				escapeTableCell(filter),
				escapeTableCell(initialItems),
			)
		}
		sb.WriteString("\n")
	}

	if len(bp.Inventory) > 0 {
		inv := make([]domain.ProjectInventoryEntry, len(bp.Inventory))
		copy(inv, bp.Inventory)
		slices.SortFunc(inv, func(a, b domain.ProjectInventoryEntry) int {
			if c := cmp.Compare(a.ItemID, b.ItemID); c != 0 {
				return c
			}
			return cmp.Compare(a.ID, b.ID)
		})
		sb.WriteString("### Seeded Network Inventory\n\n")
		sb.WriteString("| Item ID | Quantity |\n")
		sb.WriteString("| --- | ---: |\n")
		for _, entry := range inv {
			fmt.Fprintf(&sb, "| `%s` | %d |\n", escapeTableCell(entry.ItemID), entry.Quantity)
		}
		sb.WriteString("\n")
	}

	// Section 4: Encoded Patterns Table
	sb.WriteString("## 4. Encoded Patterns Table\n\n")
	if len(patternRows) == 0 {
		sb.WriteString("_No crafting or processing patterns encoded._\n\n")
	} else {
		sb.WriteString("| Pattern ID | Type | Location | Inputs | Outputs |\n")
		sb.WriteString("| --- | --- | --- | --- | --- |\n")
		for _, pat := range patternRows {
			fmt.Fprintf(&sb, "| `%s` | `%s` | %s | %s | %s |\n",
				escapeTableCell(pat.id),
				escapeTableCell(pat.patternType),
				escapeTableCell(pat.location),
				escapeTableCell(formatItemStacks(pat.inputs)),
				escapeTableCell(formatItemStacks(pat.outputs)),
			)
		}
		sb.WriteString("\n")
	}

	// Section 5: Pre-Flight Checklist
	sb.WriteString("## 5. Pre-Flight Checklist Before Powering On\n\n")
	fmt.Fprintf(&sb, "- [ ] Confirm server/pack matches **Minecraft %s**, **AE2 %s**, and channel mode **`%s`**.\n",
		nonEmptyOr(bp.Profile.MinecraftVersion, "1.21.1"),
		nonEmptyOr(bp.Profile.AE2Version, "19.x"),
		nonEmptyOr(bp.Profile.ChannelMode, "default"),
	)
	sb.WriteString("- [ ] Verify the **ME Controller** forms a single connected multiblock within a `7x7x7` bounding box and no controller block has adjacent controllers on more than 1 opposite-face axis.\n")
	sb.WriteString("- [ ] Check **cable colors and cable anchors** at parallel runs so adjacent branches do not accidentally bridge or create controller conflicts.\n")
	sb.WriteString("- [ ] Verify channel loads stay within capacity (`32` channels per Controller face / Dense Cable trunk, `8` channels on Fluix / Smart Cable branches, or `8` total on ad-hoc controllerless networks).\n")
	sb.WriteString("- [ ] Confirm every **Crafting CPU** multiblock is a complete solid rectangular prism containing at least one Crafting Storage block.\n")
	sb.WriteString("- [ ] Check **directional Pattern Providers** point their push face directly into the target Molecular Assembler or processing machine and that machine outputs route back into the network.\n")
	sb.WriteString("- [ ] Format and insert **partitioned ME Storage Cells** into Cell Workbenches and ME Drives according to priorities before importing bulk items.\n")
	sb.WriteString("- [ ] Supply external FE power to **Energy Acceptors** (`2 FE = 1 AE`) and pre-charge **Energy Cells** before bringing high-drain subsystems online.\n")
	if hasUnsupported {
		sb.WriteString("- [ ] **Attention**: One or more blocks in this blueprint (`fidelity: unsupported`) are not simulated by ME Lab; manually verify their P2P/subnetwork behavior in-game.\n")
	}
	if bp.ValidationSummary != nil && bp.ValidationSummary.DiagnosticsCount > 0 {
		fmt.Fprintf(&sb, "- [ ] **Attention**: Latest validation reported **%d diagnostic(s)**; review diagnostics in ME Lab before building.\n", bp.ValidationSummary.DiagnosticsCount)
	}

	return sb.String()
}

func sortBlocksDeterministically(blocks []domain.PlacedBlock) []domain.PlacedBlock {
	sorted := make([]domain.PlacedBlock, len(blocks))
	copy(sorted, blocks)
	slices.SortFunc(sorted, func(a, b domain.PlacedBlock) int {
		if c := domain.CompareCoords(a.Coord(), b.Coord()); c != 0 {
			return c
		}
		if c := cmp.Compare(a.BlockDefID, b.BlockDefID); c != 0 {
			return c
		}
		return cmp.Compare(a.ID, b.ID)
	})
	return sorted
}

func collectBlockBOM(blocks []domain.PlacedBlock, catalog map[string]domain.BlockDefinition) ([]bomBlockRow, bool) {
	byID := make(map[string]*bomBlockRow, len(blocks))
	hasUnsupported := false

	for _, b := range blocks {
		fid := blockFidelity(b.BlockDefID, catalog)
		if fid == domain.FidelityUnsupported {
			hasUnsupported = true
		}
		if existing, ok := byID[b.BlockDefID]; ok {
			existing.quantity++
			continue
		}
		byID[b.BlockDefID] = &bomBlockRow{
			blockDefID: b.BlockDefID,
			name:       blockDisplayName(b.BlockDefID, catalog),
			category:   blockCategory(b.BlockDefID, catalog),
			fidelity:   fid,
			quantity:   1,
		}
	}

	rows := make([]bomBlockRow, 0, len(byID))
	for _, r := range byID {
		rows = append(rows, *r)
	}
	slices.SortFunc(rows, func(a, b bomBlockRow) int {
		if c := cmp.Compare(categoryRank(a.category), categoryRank(b.category)); c != 0 {
			return c
		}
		if c := cmp.Compare(a.category, b.category); c != 0 {
			return c
		}
		if c := cmp.Compare(a.name, b.name); c != 0 {
			return c
		}
		return cmp.Compare(a.blockDefID, b.blockDefID)
	})
	return rows, hasUnsupported
}

func categoryRank(cat string) int {
	switch strings.ToLower(cat) {
	case "network", "controller":
		return 1
	case "energy", "power":
		return 2
	case "cables", "cable":
		return 3
	case "storage":
		return 4
	case "crafting", "autocrafting":
		return 5
	default:
		return 6
	}
}

func collectDriveCells(blocks []domain.PlacedBlock) ([]driveCellRow, map[string]int) {
	var rows []driveCellRow
	tierCounts := make(map[string]int)

	for _, b := range blocks {
		if b.BlockDefID != "ae2:drive" || b.Config == nil {
			continue
		}
		priority, _ := configInt(b.Config, "priority")
		rawCells, ok := b.Config["cells"]
		if !ok || rawCells == nil {
			continue
		}
		cells := decodeSlice[rawDriveCell](rawCells)
		for idx, c := range cells {
			tier := strings.ToLower(strings.TrimSpace(c.Tier))
			if tier == "" {
				tier = "1k"
			}
			tierCounts[tier]++
			rows = append(rows, driveCellRow{
				coord:     b.Coord(),
				slot:      idx + 1,
				tier:      tier,
				priority:  priority,
				partition: c.Partition,
				inverter:  c.Inverter,
				items:     c.Items,
			})
		}
	}
	return rows, tierCounts
}

type tierCount struct {
	tier  string
	count int
}

func orderedTierCounts(counts map[string]int) []tierCount {
	canonical := []string{"1k", "4k", "16k", "64k", "256k"}
	seen := make(map[string]bool, len(canonical))
	out := make([]tierCount, 0, len(counts))

	for _, t := range canonical {
		if n := counts[t]; n > 0 {
			seen[t] = true
			out = append(out, tierCount{
				tier:  t,
				count: n,
			})
		}
	}

	var extras []string
	for t, n := range counts {
		if n > 0 && !seen[t] {
			extras = append(extras, t)
		}
	}
	slices.Sort(extras)
	for _, t := range extras {
		out = append(out, tierCount{
			tier:  t,
			count: counts[t],
		})
	}
	return out
}

func collectEncodedPatterns(blocks []domain.PlacedBlock, projectPatterns []domain.ProjectPattern) ([]encodedPatternRow, int, int) {
	var rows []encodedPatternRow
	seenIDs := make(map[string]bool)
	craftingCount := 0
	processingCount := 0
	seq := 1

	for _, b := range blocks {
		if b.BlockDefID != "ae2:pattern_provider" || b.Config == nil {
			continue
		}
		rawPats, ok := b.Config["patterns"]
		if !ok || rawPats == nil {
			continue
		}
		specs := decodeSlice[domain.PatternSpec](rawPats)
		for _, p := range specs {
			patID := strings.TrimSpace(p.ID)
			if patID != "" {
				seenIDs[patID] = true
			} else {
				patID = fmt.Sprintf("inline-%d", seq)
			}
			seq++
			patType := normalizePatternType(p.PatternType)
			if patType == "processing" {
				processingCount++
			} else {
				craftingCount++
			}
			rows = append(rows, encodedPatternRow{
				id:          patID,
				patternType: patType,
				location:    fmt.Sprintf("Pattern Provider at %s", b.Coord().String()),
				inputs:      p.Inputs,
				outputs:     p.Outputs,
			})
		}
	}

	sortedProjPats := make([]domain.ProjectPattern, len(projectPatterns))
	copy(sortedProjPats, projectPatterns)
	slices.SortFunc(sortedProjPats, func(a, b domain.ProjectPattern) int {
		if c := cmp.Compare(a.ID, b.ID); c != 0 {
			return c
		}
		return cmp.Compare(a.PatternType, b.PatternType)
	})

	for _, p := range sortedProjPats {
		patID := strings.TrimSpace(p.ID)
		if patID != "" && seenIDs[patID] {
			continue
		}
		if patID == "" {
			patID = fmt.Sprintf("project-%d", seq)
			seq++
		}
		patType := normalizePatternType(p.PatternType)
		if patType == "processing" {
			processingCount++
		} else {
			craftingCount++
		}
		loc := "Project Pattern Library"
		if coordStr, ok := configString(p.Config, "provider_coord"); ok && coordStr != "" {
			loc = fmt.Sprintf("Pattern Provider at %s", coordStr)
		}
		rows = append(rows, encodedPatternRow{
			id:          patID,
			patternType: patType,
			location:    loc,
			inputs:      p.Inputs,
			outputs:     p.Outputs,
		})
	}

	return rows, craftingCount, processingCount
}

func normalizePatternType(raw string) string {
	if strings.EqualFold(strings.TrimSpace(raw), "processing") {
		return "processing"
	}
	return "crafting"
}

type yLayerGroup struct {
	y      int
	blocks []domain.PlacedBlock
}

func groupBlocksByY(sortedBlocks []domain.PlacedBlock) []yLayerGroup {
	var groups []yLayerGroup
	for _, b := range sortedBlocks {
		if len(groups) == 0 || groups[len(groups)-1].y != b.Y {
			groups = append(groups, yLayerGroup{
				y:      b.Y,
				blocks: []domain.PlacedBlock{b},
			})
			continue
		}
		groups[len(groups)-1].blocks = append(groups[len(groups)-1].blocks, b)
	}
	return groups
}

func computeBounds(blocks []domain.PlacedBlock) (domain.Coord3D, domain.Coord3D) {
	first := blocks[0].Coord()
	minC := first
	maxC := first
	for _, b := range blocks[1:] {
		if b.X < minC.X {
			minC.X = b.X
		}
		if b.X > maxC.X {
			maxC.X = b.X
		}
		if b.Y < minC.Y {
			minC.Y = b.Y
		}
		if b.Y > maxC.Y {
			maxC.Y = b.Y
		}
		if b.Z < minC.Z {
			minC.Z = b.Z
		}
		if b.Z > maxC.Z {
			maxC.Z = b.Z
		}
	}
	return minC, maxC
}

func describeBlockSettings(b domain.PlacedBlock, catalog map[string]domain.BlockDefinition) string {
	var parts []string

	if blockFidelity(b.BlockDefID, catalog) == domain.FidelityUnsupported {
		parts = append(parts, "UNSUPPORTED IN SIMULATOR")
	}

	cfg := b.Config
	switch b.BlockDefID {
	case "ae2:fluix_cable", "ae2:smart_cable", "ae2:dense_cable", "ae2:dense_smart_cable":
		color, ok := configString(cfg, "color")
		if !ok || color == "" {
			color = "fluix"
		}
		parts = append(parts, "color="+color)
		if anchors := configStringSlice(cfg, "anchored_faces"); len(anchors) > 0 {
			parts = append(parts, "anchors=["+strings.Join(anchors, ", ")+"]")
		}

	case "ae2:energy_acceptor":
		if fe, ok := configFloat(cfg, "fe_input_per_tick"); ok {
			parts = append(parts, fmt.Sprintf("fe_input=%.0f FE/t (%.0f AE/t)", fe, fe/2.0))
		}

	case "ae2:energy_cell", "ae2:dense_energy_cell":
		if pct, ok := configFloat(cfg, "initial_charge_pct"); ok {
			parts = append(parts, fmt.Sprintf("initial_charge=%.0f%%", pct))
		}

	case "ae2:drive":
		prio, _ := configInt(cfg, "priority")
		parts = append(parts, fmt.Sprintf("priority=%d", prio))
		if rawCells, ok := cfg["cells"]; ok && rawCells != nil {
			cells := decodeSlice[rawDriveCell](rawCells)
			parts = append(parts, fmt.Sprintf("cells=%d", len(cells)))
		}

	case "ae2:pattern_provider":
		prio, _ := configInt(cfg, "priority")
		variant, ok := configString(cfg, "variant")
		if !ok || variant == "" {
			variant = "normal"
		}
		if variant == "directional" {
			pushFace := string(b.Facing)
			if pushFace == "" {
				pushFace = string(domain.North)
			}
			parts = append(parts, fmt.Sprintf("variant=directional (push=%s)", pushFace))
		} else {
			parts = append(parts, "variant=normal")
		}
		parts = append(parts, fmt.Sprintf("priority=%d", prio))
		if blocking, ok := configBool(cfg, "blocking_mode"); ok && blocking {
			parts = append(parts, "blocking=true")
		}
		if lockMode, ok := configString(cfg, "lock_mode"); ok && lockMode != "" && lockMode != "none" {
			parts = append(parts, "lock="+lockMode)
		}
		if rawPats, ok := cfg["patterns"]; ok && rawPats != nil {
			pats := decodeSlice[domain.PatternSpec](rawPats)
			parts = append(parts, fmt.Sprintf("patterns=%d", len(pats)))
		}

	case "melab:external_machine":
		if ticks, ok := configInt(cfg, "process_ticks"); ok {
			parts = append(parts, fmt.Sprintf("process_ticks=%d", ticks))
		}
		if autoRet, ok := configBool(cfg, "auto_return"); ok {
			parts = append(parts, fmt.Sprintf("auto_return=%t", autoRet))
		}
	}

	if len(parts) == 0 {
		return "Default"
	}
	return strings.Join(parts, "; ")
}

func blockDisplayName(blockDefID string, catalog map[string]domain.BlockDefinition) string {
	if catalog != nil {
		if def, ok := catalog[blockDefID]; ok && strings.TrimSpace(def.Name) != "" {
			return def.Name
		}
	}
	raw := blockDefID
	if idx := strings.IndexByte(raw, ':'); idx >= 0 {
		raw = raw[idx+1:]
	}
	words := strings.Split(raw, "_")
	for i, w := range words {
		if w == "" {
			continue
		}
		if w == "ae2" || w == "me" || w == "cpu" || w == "p2p" {
			words[i] = strings.ToUpper(w)
			continue
		}
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(words, " ")
}

func blockCategory(blockDefID string, catalog map[string]domain.BlockDefinition) string {
	if catalog != nil {
		if def, ok := catalog[blockDefID]; ok && strings.TrimSpace(def.Category) != "" {
			return def.Category
		}
	}
	return "general"
}

func blockFidelity(blockDefID string, catalog map[string]domain.BlockDefinition) domain.Fidelity {
	if catalog != nil {
		if def, ok := catalog[blockDefID]; ok && def.Fidelity != "" {
			return def.Fidelity
		}
	}
	return domain.FidelityExact
}

func decodeSlice[T any](raw any) []T {
	if raw == nil {
		return nil
	}
	if typed, ok := raw.([]T); ok {
		out := make([]T, len(typed))
		copy(out, typed)
		return out
	}
	b, err := json.Marshal(raw)
	if err != nil {
		return nil
	}
	var out []T
	if err := json.Unmarshal(b, &out); err != nil {
		return nil
	}
	return out
}

func configString(cfg map[string]any, key string) (string, bool) {
	if cfg == nil {
		return "", false
	}
	v, ok := cfg[key]
	if !ok || v == nil {
		return "", false
	}
	s, ok := v.(string)
	return strings.TrimSpace(s), ok
}

func configBool(cfg map[string]any, key string) (bool, bool) {
	if cfg == nil {
		return false, false
	}
	v, ok := cfg[key]
	if !ok || v == nil {
		return false, false
	}
	b, ok := v.(bool)
	return b, ok
}

func configInt(cfg map[string]any, key string) (int, bool) {
	if cfg == nil {
		return 0, false
	}
	v, ok := cfg[key]
	if !ok || v == nil {
		return 0, false
	}
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	case json.Number:
		if i, err := n.Int64(); err == nil {
			return int(i), true
		}
	case string:
		if i, err := strconv.Atoi(strings.TrimSpace(n)); err == nil {
			return i, true
		}
	}
	return 0, false
}

func configFloat(cfg map[string]any, key string) (float64, bool) {
	if cfg == nil {
		return 0, false
	}
	v, ok := cfg[key]
	if !ok || v == nil {
		return 0, false
	}
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case json.Number:
		if f, err := n.Float64(); err == nil {
			return f, true
		}
	}
	return 0, false
}

func configStringSlice(cfg map[string]any, key string) []string {
	if cfg == nil {
		return nil
	}
	v, ok := cfg[key]
	if !ok || v == nil {
		return nil
	}
	switch s := v.(type) {
	case []string:
		out := make([]string, 0, len(s))
		for _, item := range s {
			if trimmed := strings.TrimSpace(item); trimmed != "" {
				out = append(out, trimmed)
			}
		}
		return out
	case []any:
		out := make([]string, 0, len(s))
		for _, item := range s {
			if str, ok := item.(string); ok {
				if trimmed := strings.TrimSpace(str); trimmed != "" {
					out = append(out, trimmed)
				}
			}
		}
		return out
	}
	return nil
}

func formatItemStacks(stacks []domain.ItemStack) string {
	if len(stacks) == 0 {
		return "—"
	}
	parts := make([]string, 0, len(stacks))
	for _, s := range stacks {
		parts = append(parts, fmt.Sprintf("%dx `%s`", s.Quantity, s.ItemID))
	}
	return strings.Join(parts, ", ")
}

func nonEmptyOr(val, fallback string) string {
	if strings.TrimSpace(val) == "" {
		return fallback
	}
	return val
}

func pluralize(n int, singular, plural string) string {
	if n == 1 {
		return singular
	}
	return plural
}

func escapeMarkdownInline(s string) string {
	return strings.NewReplacer("\r", "", "\n", " ").Replace(s)
}

func escapeTableCell(s string) string {
	r := strings.NewReplacer("|", "\\|", "\r", "", "\n", " ")
	return r.Replace(s)
}
