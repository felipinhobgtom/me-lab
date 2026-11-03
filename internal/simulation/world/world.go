package world

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/felipinho/me-lab/internal/domain"
	"github.com/felipinho/me-lab/internal/simulation/diagnostics"
)

// Node wraps a PlacedBlock with normalized catalog metadata and precomputed connectivity attributes.
type Node struct {
	Block            domain.PlacedBlock
	Def              domain.BlockDefinition
	Coord            domain.Coord3D
	Facing           domain.Direction
	IsMENode         bool
	IsUnsupported    bool
	IsController     bool
	IsCable          bool
	IsDenseCable     bool
	IsSmartCable     bool
	IsCPUBlock       bool
	CableColor       string
	AnchoredFaces    map[domain.Direction]bool
	QuartzFiberFaces map[domain.Direction]bool
	SubpartsByFace   map[domain.Direction]string
}

// World holds the deterministic 3D block topology and ME face-connectivity graph.
type World struct {
	Nodes       []*Node
	MENodes     []*Node
	ByCoord     map[domain.Coord3D]*Node
	ByID        map[string]*Node
	Adj         map[domain.Coord3D][]domain.Coord3D
	Diagnostics []domain.Diagnostic
}

// Build constructs a deterministic World from placed blocks and catalog definitions.
func Build(simID string, placed []domain.PlacedBlock, catalog map[string]domain.BlockDefinition) *World {
	sorted := make([]domain.PlacedBlock, len(placed))
	copy(sorted, placed)
	sort.Slice(sorted, func(i, j int) bool {
		if cmp := domain.CompareCoords(sorted[i].Coord(), sorted[j].Coord()); cmp != 0 {
			return cmp < 0
		}
		return sorted[i].ID < sorted[j].ID
	})

	w := &World{
		Nodes:       make([]*Node, 0, len(sorted)),
		MENodes:     make([]*Node, 0, len(sorted)),
		ByCoord:     make(map[domain.Coord3D]*Node, len(sorted)),
		ByID:        make(map[string]*Node, len(sorted)),
		Adj:         make(map[domain.Coord3D][]domain.Coord3D, len(sorted)),
		Diagnostics: []domain.Diagnostic{},
	}

	for _, b := range sorted {
		coord := b.Coord()
		if _, occupied := w.ByCoord[coord]; occupied {
			continue
		}
		if b.ID == "" {
			b.ID = fmt.Sprintf("block@%d,%d,%d", b.X, b.Y, b.Z)
		}
		def := resolveDefinition(b.BlockDefID, catalog)
		node := buildNode(b, def)
		w.Nodes = append(w.Nodes, node)
		w.ByCoord[coord] = node
		if b.ID != "" {
			w.ByID[b.ID] = node
		}

		if node.IsUnsupported {
			w.Diagnostics = append(w.Diagnostics, diagnostics.NewUnsupportedMechanic(simID, b, def.Name))
			continue
		}
		if node.IsMENode {
			w.MENodes = append(w.MENodes, node)
			w.Adj[coord] = []domain.Coord3D{}
		}
	}

	// Build undirected face-level ME connectivity edges in canonical coordinate order.
	for _, n := range w.MENodes {
		neighbors := make([]domain.Coord3D, 0, 6)
		for _, dir := range domain.AllDirections() {
			adjCoord := n.Coord.Adjacent(dir)
			other, ok := w.ByCoord[adjCoord]
			if !ok || !other.IsMENode {
				continue
			}
			if CanConnectME(n, dir, other) {
				neighbors = append(neighbors, adjCoord)
			}
		}
		sort.Slice(neighbors, func(i, j int) bool {
			return domain.CompareCoords(neighbors[i], neighbors[j]) < 0
		})
		w.Adj[n.Coord] = neighbors
	}

	// Emit BLOCK_DISCONNECTED warning for isolated ME blocks (excluding self-sufficient creative setups).
	for _, n := range w.MENodes {
		if len(w.Adj[n.Coord]) > 0 {
			continue
		}
		if n.IsCreativeEnergyCell() {
			continue
		}
		if n.IsController && len(w.MENodes) == 1 {
			continue
		}
		w.Diagnostics = append(w.Diagnostics, diagnostics.NewBlockDisconnected(simID, n.Block))
	}

	w.Diagnostics = diagnostics.DeduplicateAndSort(w.Diagnostics)
	return w
}

// CanConnectME evaluates AE2 1.21.1 face-level connectivity between two adjacent ME nodes
// where b is located at a.Coord.Adjacent(dir).
func CanConnectME(a *Node, dir domain.Direction, b *Node) bool {
	if a == nil || b == nil || !a.IsMENode || !b.IsMENode {
		return false
	}
	opp := dir.Opposite()

	// Cable anchors / anchored_faces block connection across that face.
	if a.AnchoredFaces[dir] || b.AnchoredFaces[opp] {
		return false
	}

	// Directional Pattern Provider does NOT form an ME network edge on its push (Facing) side.
	if a.IsPatternProvider() && a.PatternProviderVariant() == "directional" && a.Facing == dir {
		return false
	}
	if b.IsPatternProvider() && b.PatternProviderVariant() == "directional" && b.Facing == opp {
		return false
	}

	// Colored cable rule: two cables connect iff at least one is "" / "fluix", or colors match.
	if a.IsCable && b.IsCable {
		ca := a.CableColor
		cb := b.CableColor
		if ca == "" {
			ca = "fluix"
		}
		if cb == "" {
			cb = "fluix"
		}
		if ca != "fluix" && cb != "fluix" && ca != cb {
			return false
		}
	}

	return true
}

// Connected reports whether two coordinates share an active ME network edge.
func (w *World) Connected(a, b domain.Coord3D) bool {
	for _, nb := range w.Adj[a] {
		if nb == b {
			return true
		}
	}
	return false
}

func buildNode(b domain.PlacedBlock, def domain.BlockDefinition) *Node {
	facing := b.Facing
	if !facing.IsValid() {
		facing = domain.North
	}

	anchored := make(map[domain.Direction]bool)
	for _, rawDir := range ConfigStringSlice(b.Config, "anchored_faces") {
		if d, ok := domain.ParseDirection(rawDir); ok {
			anchored[d] = true
		}
	}

	id := b.BlockDefID
	bkey := def.BehaviorKey

	quartzFaces := make(map[domain.Direction]bool)
	subparts := parseSubpartsByFace(b.Config)
	for dir, subID := range subparts {
		switch subID {
		case "ae2:cable_anchor":
			anchored[dir] = true
		case "ae2:quartz_fiber":
			anchored[dir] = true
			quartzFaces[dir] = true
		}
	}

	isUnsupported := def.Fidelity == domain.FidelityUnsupported || bkey == "unsupported" || isPostMVPUnsupportedID(id)
	isAnchor := id == "ae2:cable_anchor" || bkey == "cable_anchor" || id == "ae2:quartz_fiber" || bkey == "quartz_fiber"
	isExtMachine := isExternalMachineOrContainer(id, bkey)
	isMENode := !isUnsupported && !isAnchor && !isExtMachine

	isCable := def.Metadata.IsCable ||
		id == "ae2:fluix_cable" || id == "ae2:covered_cable" || id == "ae2:smart_cable" ||
		id == "ae2:dense_cable" || id == "ae2:dense_smart_cable" ||
		bkey == "cable" || bkey == "fluix_cable" || bkey == "smart_cable" ||
		bkey == "dense_cable" || bkey == "dense_smart_cable"

	isDense := def.Metadata.IsDenseCable ||
		id == "ae2:dense_cable" || id == "ae2:dense_smart_cable" ||
		bkey == "dense_cable" || bkey == "dense_smart_cable"

	isSmart := def.Metadata.IsSmartCable ||
		id == "ae2:smart_cable" || id == "ae2:dense_smart_cable" ||
		bkey == "smart_cable" || bkey == "dense_smart_cable"
	isController := id == "ae2:controller" || bkey == "controller"
	isCPU := isCraftingCPUBlock(id, def)

	color := ""
	if isCable {
		color = strings.ToLower(strings.TrimSpace(ConfigString(b.Config, "color", "fluix")))
		if color == "" {
			color = "fluix"
		}
	}

	return &Node{
		Block:            b,
		Def:              def,
		Coord:            b.Coord(),
		Facing:           facing,
		IsMENode:         isMENode,
		IsUnsupported:    isUnsupported,
		IsController:     isController,
		IsCable:          isCable,
		IsDenseCable:     isDense,
		IsSmartCable:     isSmart,
		IsCPUBlock:       isCPU,
		CableColor:       color,
		AnchoredFaces:    anchored,
		QuartzFiberFaces: quartzFaces,
		SubpartsByFace:   subparts,
	}
}

func resolveDefinition(blockDefID string, catalog map[string]domain.BlockDefinition) domain.BlockDefinition {
	if catalog != nil {
		if def, ok := catalog[blockDefID]; ok {
			return enrichCanonicalDefaults(blockDefID, def)
		}
	}
	return enrichCanonicalDefaults(blockDefID, domain.BlockDefinition{ID: blockDefID})
}

func enrichCanonicalDefaults(id string, def domain.BlockDefinition) domain.BlockDefinition {
	if def.ID == "" {
		def.ID = id
	}
	if def.Fidelity == "" {
		if isPostMVPUnsupportedID(id) {
			def.Fidelity = domain.FidelityUnsupported
		} else if id == "melab:external_machine" {
			def.Fidelity = domain.FidelityApproximated
		} else {
			def.Fidelity = domain.FidelityExact
		}
	}

	switch id {
	case "ae2:fluix_cable", "ae2:covered_cable":
		def.Metadata.IsCable = true
		def.Metadata.TransmitsChannels = true
	case "ae2:smart_cable":
		def.Metadata.IsCable = true
		def.Metadata.IsSmartCable = true
		def.Metadata.TransmitsChannels = true
	case "ae2:dense_cable":
		def.Metadata.IsCable = true
		def.Metadata.IsDenseCable = true
		def.Metadata.TransmitsChannels = true
	case "ae2:dense_smart_cable":
		def.Metadata.IsCable = true
		def.Metadata.IsDenseCable = true
		def.Metadata.IsSmartCable = true
		def.Metadata.TransmitsChannels = true
	case "ae2:molecular_assembler":
		def.Metadata.TransmitsChannels = true
		def.Metadata.ChannelUsage = 0
	case "ae2:drive", "ae2:chest", "ae2:interface", "ae2:pattern_provider":
		def.Metadata.TransmitsChannels = true
		if def.Metadata.ChannelUsage == 0 {
			def.Metadata.ChannelUsage = 1
		}
	case "ae2:terminal", "ae2:crafting_terminal", "ae2:pattern_encoding_terminal", "ae2:pattern_access_terminal",
		"ae2:import_bus", "ae2:export_bus", "ae2:storage_bus", "ae2:storage_monitor":
		if def.Metadata.ChannelUsage == 0 {
			def.Metadata.ChannelUsage = 1
		}
	case "ae2:energy_acceptor", "ae2:energy_cell", "ae2:dense_energy_cell", "ae2:creative_energy_cell":
		def.Metadata.TransmitsChannels = true
	case "ae2:crafting_unit", "ae2:crafting_monitor":
		def.Metadata.TransmitsChannels = true
		def.Metadata.ChannelUsage = 0
	case "ae2:crafting_co_processor":
		def.Metadata.TransmitsChannels = true
		def.Metadata.IsCoProcessor = true
		def.Metadata.ChannelUsage = 0
	case "ae2:crafting_storage_1k":
		def.Metadata.TransmitsChannels = true
		def.Metadata.ChannelUsage = 0
		if def.Metadata.CraftingStorageBytes == 0 {
			def.Metadata.CraftingStorageBytes = 1024
		}
	case "ae2:crafting_storage_4k":
		def.Metadata.TransmitsChannels = true
		def.Metadata.ChannelUsage = 0
		if def.Metadata.CraftingStorageBytes == 0 {
			def.Metadata.CraftingStorageBytes = 4096
		}
	case "ae2:crafting_storage_16k":
		def.Metadata.TransmitsChannels = true
		def.Metadata.ChannelUsage = 0
		if def.Metadata.CraftingStorageBytes == 0 {
			def.Metadata.CraftingStorageBytes = 16384
		}
	case "ae2:crafting_storage_64k":
		def.Metadata.TransmitsChannels = true
		def.Metadata.ChannelUsage = 0
		if def.Metadata.CraftingStorageBytes == 0 {
			def.Metadata.CraftingStorageBytes = 65536
		}
	case "ae2:crafting_storage_256k":
		def.Metadata.TransmitsChannels = true
		def.Metadata.ChannelUsage = 0
		if def.Metadata.CraftingStorageBytes == 0 {
			def.Metadata.CraftingStorageBytes = 262144
		}
	}
	return def
}

func isPostMVPUnsupportedID(id string) bool {
	switch id {
	case "ae2:p2p_tunnel", "ae2:quantum_ring":
		return true
	default:
		return false
	}
}

func isCraftingCPUBlock(id string, def domain.BlockDefinition) bool {
	if def.Metadata.ConnectionRules.MultiblockGroup == "crafting_cpu" ||
		def.Metadata.CraftingStorageBytes > 0 ||
		def.Metadata.IsCoProcessor {
		return true
	}
	switch def.BehaviorKey {
	case "crafting_unit", "crafting_storage", "crafting_co_processor", "crafting_monitor":
		return true
	}
	switch id {
	case "ae2:crafting_unit",
		"ae2:crafting_storage_1k",
		"ae2:crafting_storage_4k",
		"ae2:crafting_storage_16k",
		"ae2:crafting_storage_64k",
		"ae2:crafting_storage_256k",
		"ae2:crafting_co_processor",
		"ae2:crafting_monitor":
		return true
	default:
		return false
	}
}

// Node behavioral helpers shared across simulation subpackages.

func (n *Node) IsPatternProvider() bool {
	return n.Block.BlockDefID == "ae2:pattern_provider" || n.Def.BehaviorKey == "pattern_provider"
}

func (n *Node) PatternProviderVariant() string {
	v := strings.ToLower(strings.TrimSpace(ConfigString(n.Block.Config, "variant", "normal")))
	if v == "directional" {
		return "directional"
	}
	return "normal"
}

func (n *Node) IsMolecularAssembler() bool {
	return n.Block.BlockDefID == "ae2:molecular_assembler" || n.Def.BehaviorKey == "molecular_assembler"
}

func (n *Node) IsExternalMachine() bool {
	switch n.Block.BlockDefID {
	case "minecraft:furnace", "minecraft:blast_furnace", "minecraft:barrel",
		"ae2:inscriber", "ae2:charger", "melab:external_machine":
		return true
	}
	return n.Def.BehaviorKey == "external_machine"
}

func (n *Node) IsQuartzFiber() bool {
	return n.Block.BlockDefID == "ae2:quartz_fiber" || n.Def.BehaviorKey == "quartz_fiber"
}

func (n *Node) IsDrive() bool {
	return n.Block.BlockDefID == "ae2:drive" || n.Block.BlockDefID == "ae2:chest" || n.Def.BehaviorKey == "drive"
}

func (n *Node) IsEnergyAcceptor() bool {
	return n.Block.BlockDefID == "ae2:energy_acceptor" || n.Def.BehaviorKey == "energy_acceptor"
}

func (n *Node) IsEnergyCell() bool {
	return n.Block.BlockDefID == "ae2:energy_cell" || n.Def.BehaviorKey == "energy_cell"
}

func (n *Node) IsDenseEnergyCell() bool {
	return n.Block.BlockDefID == "ae2:dense_energy_cell" || n.Def.BehaviorKey == "dense_energy_cell"
}

func (n *Node) IsCreativeEnergyCell() bool {
	return n.Block.BlockDefID == "ae2:creative_energy_cell" || n.Def.BehaviorKey == "creative_energy_cell"
}

func (n *Node) TransmitsChannels() bool {
	return n.IsCable || n.Def.Metadata.TransmitsChannels
}

func (n *Node) EffectiveChannelUsage() int {
	if n.IsCPUBlock || n.IsController || n.IsMolecularAssembler() {
		return 0
	}
	if n.IsCable {
		ch := 0
		for _, subID := range n.SubpartsByFace {
			if subpartConsumesChannel(subID) {
				ch++
			}
		}
		return ch
	}
	return n.Def.Metadata.ChannelUsage
}

func isExternalMachineOrContainer(id, bkey string) bool {
	switch id {
	case "minecraft:chest", "minecraft:furnace", "minecraft:blast_furnace", "minecraft:hopper", "minecraft:barrel",
		"ae2:inscriber", "ae2:charger", "melab:external_machine":
		return true
	}
	return bkey == "external_machine" || bkey == "hopper" || bkey == "external_storage"
}

func subpartConsumesChannel(subID string) bool {
	switch subID {
	case "ae2:terminal", "ae2:crafting_terminal", "ae2:pattern_encoding_terminal", "ae2:pattern_access_terminal",
		"ae2:import_bus", "ae2:export_bus", "ae2:storage_bus", "ae2:storage_monitor":
		return true
	default:
		return false
	}
}

func parseSubpartsByFace(cfg map[string]any) map[domain.Direction]string {
	out := make(map[domain.Direction]string)
	if cfg == nil {
		return out
	}
	raw, ok := cfg["subparts"].(map[string]any)
	if !ok {
		return out
	}
	for k, v := range raw {
		dir, valid := domain.ParseDirection(k)
		if !valid {
			continue
		}
		switch val := v.(type) {
		case string:
			if strings.TrimSpace(val) != "" {
				out[dir] = strings.TrimSpace(val)
			}
		case map[string]any:
			if id, ok := val["block_def_id"].(string); ok && strings.TrimSpace(id) != "" {
				out[dir] = strings.TrimSpace(id)
			} else if id, ok := val["type"].(string); ok && strings.TrimSpace(id) != "" {
				out[dir] = strings.TrimSpace(id)
			}
		}
	}
	return out
}

// Generic map[string]any configuration readers resilient to both Go literals and JSON unmarshaling.

func ConfigString(cfg map[string]any, key, defVal string) string {
	if cfg == nil {
		return defVal
	}
	raw, ok := cfg[key]
	if !ok || raw == nil {
		return defVal
	}
	if s, ok := raw.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", raw)
}

func ConfigInt(cfg map[string]any, key string, defVal int) int {
	if cfg == nil {
		return defVal
	}
	raw, ok := cfg[key]
	if !ok || raw == nil {
		return defVal
	}
	switch v := raw.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case float32:
		return int(v)
	case string:
		if parsed, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			return parsed
		}
	}
	return defVal
}

func ConfigFloat(cfg map[string]any, key string, defVal float64) (float64, bool) {
	if cfg == nil {
		return defVal, false
	}
	raw, ok := cfg[key]
	if !ok || raw == nil {
		return defVal, false
	}
	switch v := raw.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case string:
		if parsed, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
			return parsed, true
		}
	}
	return defVal, false
}

func ConfigBool(cfg map[string]any, key string, defVal bool) bool {
	if cfg == nil {
		return defVal
	}
	raw, ok := cfg[key]
	if !ok || raw == nil {
		return defVal
	}
	switch v := raw.(type) {
	case bool:
		return v
	case string:
		b, err := strconv.ParseBool(strings.TrimSpace(v))
		if err == nil {
			return b
		}
	}
	return defVal
}

func ConfigStringSlice(cfg map[string]any, key string) []string {
	if cfg == nil {
		return nil
	}
	raw, ok := cfg[key]
	if !ok || raw == nil {
		return nil
	}
	switch v := raw.(type) {
	case []string:
		out := make([]string, len(v))
		copy(out, v)
		return out
	case []any:
		out := make([]string, 0, len(v))
		for _, elem := range v {
			if s, ok := elem.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}
