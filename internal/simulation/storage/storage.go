package storage

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/felipinho/me-lab/internal/domain"
	"github.com/felipinho/me-lab/internal/simulation/channels"
	"github.com/felipinho/me-lab/internal/simulation/diagnostics"
	"github.com/felipinho/me-lab/internal/simulation/energy"
	"github.com/felipinho/me-lab/internal/simulation/network"
	"github.com/felipinho/me-lab/internal/simulation/world"
)

// CellState holds the mutable, JSON-serializable state of a single ME Storage Cell inside an ME Drive.
type CellState struct {
	NetworkID    string             `json:"network_id"`
	DriveBlock   domain.PlacedBlock `json:"drive_block"`
	Slot         int                `json:"slot"`
	Tier         string             `json:"tier"`
	Priority     int                `json:"priority"`
	HasChannel   bool               `json:"has_channel"`
	Online       bool               `json:"online"`
	TotalBytes   int                `json:"total_bytes"`
	MaxTypes     int                `json:"max_types"`
	BytesPerType int                `json:"bytes_per_type"`
	ItemsPerByte int                `json:"items_per_byte"`
	Partition    []string           `json:"partition,omitempty"`
	Inverter     bool               `json:"inverter"`
	Items        []domain.ItemStack `json:"items"`
}

// State holds all ME Drive storage cells and external inventories/buses across the simulated world.
type State struct {
	Cells               []*CellState              `json:"cells"`
	ExternalInventories []*ExternalInventoryState `json:"external_inventories"`
	Buses               []*BusState               `json:"buses"`
}

type rawCellConfig struct {
	Tier      string             `json:"tier"`
	Partition []string           `json:"partition"`
	Inverter  bool               `json:"inverter"`
	Items     []domain.ItemStack `json:"items"`
}

func defaultCellTiers() map[string]domain.CellTierSpec {
	return map[string]domain.CellTierSpec{
		"1k":   {TotalBytes: 1024, MaxTypes: 63, BytesPerType: 8},
		"4k":   {TotalBytes: 4096, MaxTypes: 63, BytesPerType: 32},
		"16k":  {TotalBytes: 16384, MaxTypes: 63, BytesPerType: 128},
		"64k":  {TotalBytes: 65536, MaxTypes: 63, BytesPerType: 512},
		"256k": {TotalBytes: 262144, MaxTypes: 63, BytesPerType: 2048},
	}
}

func resolveTierSpec(tier string, profile domain.CompatibilityProfile) (string, domain.CellTierSpec) {
	norm := strings.ToLower(strings.TrimSpace(tier))
	if norm == "" {
		norm = "1k"
	}
	if len(profile.Config.CellTiers) > 0 {
		if spec, ok := profile.Config.CellTiers[norm]; ok && spec.TotalBytes > 0 {
			if spec.MaxTypes <= 0 {
				if profile.Config.MaxCellTypes > 0 {
					spec.MaxTypes = profile.Config.MaxCellTypes
				} else {
					spec.MaxTypes = 63
				}
			}
			return norm, spec
		}
	}
	defs := defaultCellTiers()
	if spec, ok := defs[norm]; ok {
		return norm, spec
	}
	return "1k", defs["1k"]
}

// Initialize builds the ME Storage state from all placed ae2:drive blocks and merges any ProjectInventoryEntry records.
func Initialize(
	simID string,
	nets []*network.Network,
	alloc *channels.Allocation,
	energyStates []*energy.NetworkEnergy,
	inventory []domain.ProjectInventoryEntry,
	profile domain.CompatibilityProfile,
) (*State, []domain.Diagnostic) {
	itemsPerByte := profile.Config.ItemsPerByte
	if itemsPerByte <= 0 {
		itemsPerByte = 8
	}

	netOnline := make(map[string]bool, len(energyStates))
	for _, es := range energyStates {
		netOnline[es.NetworkID] = es.Online
	}

	st := &State{
		Cells:               make([]*CellState, 0),
		ExternalInventories: make([]*ExternalInventoryState, 0),
		Buses:               make([]*BusState, 0),
	}
	diags := make([]domain.Diagnostic, 0)

	for _, net := range nets {
		for _, n := range net.Nodes {
			if !n.IsDrive() {
				continue
			}
			priority := world.ConfigInt(n.Block.Config, "priority", 0)
			hasCh := false
			if alloc != nil {
				hasCh = alloc.HasChannelByBlockID[n.Block.ID]
			}
			online := netOnline[net.ID] && hasCh

			rawCells := parseDriveCells(n.Block.Config)
			for slotIdx, rc := range rawCells {
				tierName, spec := resolveTierSpec(rc.Tier, profile)
				cs := &CellState{
					NetworkID:    net.ID,
					DriveBlock:   n.Block,
					Slot:         slotIdx,
					Tier:         tierName,
					Priority:     priority,
					HasChannel:   hasCh,
					Online:       online,
					TotalBytes:   spec.TotalBytes,
					MaxTypes:     spec.MaxTypes,
					BytesPerType: spec.BytesPerType,
					ItemsPerByte: itemsPerByte,
					Partition:    dedupeStrings(rc.Partition),
					Inverter:     rc.Inverter,
					Items:        []domain.ItemStack{},
				}
				// Load initial items declared directly on the cell, enforcing byte/type/partition rules.
				for _, stack := range rc.Items {
					if stack.ItemID == "" || stack.Quantity <= 0 {
						continue
					}
					accepted := cs.addDirect(stack.ItemID, stack.Quantity)
					if accepted < stack.Quantity {
						diags = append(diags, diagnostics.NewStorageFull(
							simID,
							[]domain.PlacedBlock{n.Block},
							stack.ItemID,
							stack.Quantity-accepted,
						))
					}
				}
				st.Cells = append(st.Cells, cs)
			}
		}
	}

	// Sort cells canonically by DriveCoord, then Slot ASC for storage view consistency.
	sort.Slice(st.Cells, func(i, j int) bool {
		if cmp := domain.CompareCoords(st.Cells[i].DriveBlock.Coord(), st.Cells[j].DriveBlock.Coord()); cmp != 0 {
			return cmp < 0
		}
		if st.Cells[i].DriveBlock.ID != st.Cells[j].DriveBlock.ID {
			return st.Cells[i].DriveBlock.ID < st.Cells[j].DriveBlock.ID
		}
		return st.Cells[i].Slot < st.Cells[j].Slot
	})

	// Merge project-level initial inventory entries.
	for _, entry := range inventory {
		if entry.ItemID == "" || entry.Quantity <= 0 {
			continue
		}
		targetBlockID := world.ConfigString(entry.StorageLocation, "block_id", "")
		if targetBlockID != "" {
			targetSlot := world.ConfigInt(entry.StorageLocation, "slot", -1)
			rem := st.seedTargeted(targetBlockID, targetSlot, entry.ItemID, entry.Quantity)
			if rem > 0 {
				diags = append(diags, diagnostics.NewStorageFull(simID, st.driveBlocks(""), entry.ItemID, rem))
			}
			continue
		}
		rem, insDiags := st.Insert(simID, entry.ItemID, entry.Quantity)
		if rem == entry.Quantity && !st.hasOnlineCells("") && len(st.Cells) > 0 {
			// Allow initial project inventory seeding onto offline cells when the network starts unpowered.
			rem = st.insertInternal("", entry.ItemID, entry.Quantity, false)
			if rem > 0 {
				diags = append(diags, diagnostics.NewStorageFull(simID, st.driveBlocks(""), entry.ItemID, rem))
			}
		} else {
			diags = append(diags, insDiags...)
		}
	}

	return st, diagnostics.DeduplicateAndSort(diags)
}

// SyncOnlineStatus updates every storage cell and automation bus from current network power/channel state.
func (s *State) SyncOnlineStatus(energyStates []*energy.NetworkEnergy, hasChannel map[string]bool) {
	if s == nil {
		return
	}
	netOnline := make(map[string]bool, len(energyStates))
	for _, es := range energyStates {
		netOnline[es.NetworkID] = es.Online
	}
	for _, c := range s.Cells {
		if hasChannel != nil {
			c.HasChannel = hasChannel[c.DriveBlock.ID]
		}
		c.Online = netOnline[c.NetworkID] && c.HasChannel
	}
	s.SyncAutomationOnlineStatus(energyStates, hasChannel)
}

// TypesUsed returns the count of distinct item types currently stored in this cell.
func (c *CellState) TypesUsed() int {
	n := 0
	for _, it := range c.Items {
		if it.Quantity > 0 {
			n++
		}
	}
	return n
}

// TotalItemCount returns the total item quantity across all stacks in this cell.
func (c *CellState) TotalItemCount() int {
	total := 0
	for _, it := range c.Items {
		if it.Quantity > 0 {
			total += it.Quantity
		}
	}
	return total
}

// BytesUsed computes exact AE2 storage bytes used: typesUsed*BytesPerType + ceil(totalItems / ItemsPerByte).
func (c *CellState) BytesUsed() int {
	types := c.TypesUsed()
	totalQty := c.TotalItemCount()
	if types == 0 || totalQty == 0 {
		return 0
	}
	ipb := c.ItemsPerByte
	if ipb <= 0 {
		ipb = 8
	}
	itemBytes := (totalQty + ipb - 1) / ipb
	return types*c.BytesPerType + itemBytes
}

// QuantityOf returns the stored count of itemID in this cell.
func (c *CellState) QuantityOf(itemID string) int {
	for _, it := range c.Items {
		if it.ItemID == itemID {
			return it.Quantity
		}
	}
	return 0
}

// AllowsItem evaluates partition whitelist / inverter blacklist rules for itemID.
func (c *CellState) AllowsItem(itemID string) bool {
	if len(c.Partition) == 0 {
		return true
	}
	inPartition := false
	for _, p := range c.Partition {
		if p == itemID {
			inPartition = true
			break
		}
	}
	if c.Inverter {
		return !inPartition
	}
	return inPartition
}

func (c *CellState) isWhitelistPartitionedFor(itemID string) bool {
	if c.Inverter || len(c.Partition) == 0 {
		return false
	}
	for _, p := range c.Partition {
		if p == itemID {
			return true
		}
	}
	return false
}

// CanAccept returns how many items of itemID (up to qty) can fit in this cell right now.
func (c *CellState) CanAccept(itemID string, qty int) int {
	if qty <= 0 || itemID == "" || !c.AllowsItem(itemID) {
		return 0
	}
	existing := c.QuantityOf(itemID)
	types := c.TypesUsed()
	newTypes := types
	if existing == 0 {
		if types >= c.MaxTypes {
			return 0
		}
		newTypes = types + 1
	}
	typeOverhead := newTypes * c.BytesPerType
	availItemBytes := c.TotalBytes - typeOverhead
	if availItemBytes <= 0 {
		return 0
	}
	ipb := c.ItemsPerByte
	if ipb <= 0 {
		ipb = 8
	}
	maxItems := availItemBytes * ipb
	room := maxItems - c.TotalItemCount()
	if room <= 0 {
		return 0
	}
	if room < qty {
		return room
	}
	return qty
}

func (c *CellState) addDirect(itemID string, qty int) int {
	accept := c.CanAccept(itemID, qty)
	if accept <= 0 {
		return 0
	}
	for i := range c.Items {
		if c.Items[i].ItemID == itemID {
			c.Items[i].Quantity += accept
			return accept
		}
	}
	c.Items = append(c.Items, domain.ItemStack{ItemID: itemID, Quantity: accept})
	sort.Slice(c.Items, func(i, j int) bool {
		return c.Items[i].ItemID < c.Items[j].ItemID
	})
	return accept
}

func (c *CellState) removeDirect(itemID string, qty int) int {
	if qty <= 0 {
		return 0
	}
	for i := range c.Items {
		if c.Items[i].ItemID == itemID {
			take := qty
			if c.Items[i].Quantity < take {
				take = c.Items[i].Quantity
			}
			c.Items[i].Quantity -= take
			if c.Items[i].Quantity == 0 {
				c.Items = append(c.Items[:i], c.Items[i+1:]...)
			}
			return take
		}
	}
	return 0
}

// Insert inserts up to qty of itemID across all online drives (any network), returning remaining unstored count
// and emitting STORAGE_FULL if remainder > 0.
func (s *State) Insert(simID, itemID string, qty int) (int, []domain.Diagnostic) {
	return s.InsertInNetwork(simID, "", itemID, qty)
}

// InsertInNetwork inserts up to qty of itemID into online drives on networkID (or all online drives if networkID == "").
func (s *State) InsertInNetwork(simID, networkID, itemID string, qty int) (int, []domain.Diagnostic) {
	if qty <= 0 || itemID == "" {
		return 0, nil
	}
	rem := s.insertInternal(networkID, itemID, qty, true)
	if rem > 0 {
		return rem, []domain.Diagnostic{
			diagnostics.NewStorageFull(simID, s.driveBlocks(networkID), itemID, rem),
		}
	}
	return 0, nil
}

func (s *State) insertInternal(networkID, itemID string, qty int, requireOnline bool) int {
	candidates := make([]*CellState, 0, len(s.Cells))
	for _, c := range s.Cells {
		if networkID != "" && c.NetworkID != networkID {
			continue
		}
		if requireOnline && !c.Online {
			continue
		}
		if !c.AllowsItem(itemID) {
			continue
		}
		candidates = append(candidates, c)
	}

	preferenceRank := func(c *CellState) int {
		if c.QuantityOf(itemID) > 0 {
			return 0
		}
		if c.isWhitelistPartitionedFor(itemID) {
			return 1
		}
		return 2
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Priority != candidates[j].Priority {
			return candidates[i].Priority > candidates[j].Priority // Priority DESC
		}
		ri, rj := preferenceRank(candidates[i]), preferenceRank(candidates[j])
		if ri != rj {
			return ri < rj // Prefer existing stack, then partitioned, then empty unpartitioned
		}
		if cmp := domain.CompareCoords(candidates[i].DriveBlock.Coord(), candidates[j].DriveBlock.Coord()); cmp != 0 {
			return cmp < 0
		}
		return candidates[i].Slot < candidates[j].Slot // Slot ASC
	})

	rem := qty
	for _, c := range candidates {
		if rem <= 0 {
			break
		}
		added := c.addDirect(itemID, rem)
		rem -= added
	}
	return rem
}

func (s *State) seedTargeted(blockID string, slot int, itemID string, qty int) int {
	rem := qty
	for _, c := range s.Cells {
		if c.DriveBlock.ID != blockID {
			continue
		}
		if slot >= 0 && c.Slot != slot {
			continue
		}
		added := c.addDirect(itemID, rem)
		rem -= added
		if rem <= 0 {
			break
		}
	}
	return rem
}

// Extract removes up to qty of itemID from online drives, returning the actual extracted quantity.
func (s *State) Extract(itemID string, qty int) int {
	return s.ExtractFromNetwork("", itemID, qty)
}

// ExtractFromNetwork removes up to qty of itemID from online drives on networkID (or any online network if "").
// Extraction pulls from lowest-priority drives first (preserving high-priority partitioned stock) or deterministic order.
func (s *State) ExtractFromNetwork(networkID, itemID string, qty int) int {
	if qty <= 0 || itemID == "" {
		return 0
	}
	candidates := make([]*CellState, 0, len(s.Cells))
	for _, c := range s.Cells {
		if networkID != "" && c.NetworkID != networkID {
			continue
		}
		if !c.Online || c.QuantityOf(itemID) == 0 {
			continue
		}
		candidates = append(candidates, c)
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Priority != candidates[j].Priority {
			return candidates[i].Priority > candidates[j].Priority
		}
		if cmp := domain.CompareCoords(candidates[i].DriveBlock.Coord(), candidates[j].DriveBlock.Coord()); cmp != 0 {
			return cmp < 0
		}
		return candidates[i].Slot < candidates[j].Slot
	})

	extracted := 0
	for _, c := range candidates {
		if extracted >= qty {
			break
		}
		extracted += c.removeDirect(itemID, qty-extracted)
	}
	return extracted
}

// AvailableTotals returns sorted aggregate item quantities across all online storage cells.
func (s *State) AvailableTotals() []domain.ItemStack {
	return s.AvailableInNetwork("")
}

// AvailableInNetwork returns sorted aggregate item quantities across online storage cells on networkID (or all if "").
func (s *State) AvailableInNetwork(networkID string) []domain.ItemStack {
	totals := s.AvailableMap(networkID)
	out := make([]domain.ItemStack, 0, len(totals))
	for id, q := range totals {
		if q > 0 {
			out = append(out, domain.ItemStack{ItemID: id, Quantity: q})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ItemID < out[j].ItemID
	})
	return out
}

// AvailableMap returns a map of itemID -> total online quantity on networkID (or all if "").
func (s *State) AvailableMap(networkID string) map[string]int {
	totals := make(map[string]int)
	if s == nil {
		return totals
	}
	for _, c := range s.Cells {
		if !c.Online {
			continue
		}
		if networkID != "" && c.NetworkID != networkID {
			continue
		}
		for _, it := range c.Items {
			if it.Quantity > 0 {
				totals[it.ItemID] += it.Quantity
			}
		}
	}
	return totals
}

// CellViews renders StorageCellView overlays for all cells in deterministic order.
func (s *State) CellViews() []domain.StorageCellView {
	if s == nil || len(s.Cells) == 0 {
		return []domain.StorageCellView{}
	}
	views := make([]domain.StorageCellView, 0, len(s.Cells))
	for _, c := range s.Cells {
		itemsCopy := make([]domain.ItemStack, len(c.Items))
		copy(itemsCopy, c.Items)
		var partCopy []string
		if len(c.Partition) > 0 {
			partCopy = make([]string, len(c.Partition))
			copy(partCopy, c.Partition)
		}
		views = append(views, domain.StorageCellView{
			DriveBlockID: c.DriveBlock.ID,
			DriveCoord:   c.DriveBlock.Coord(),
			Slot:         c.Slot,
			Tier:         c.Tier,
			Priority:     c.Priority,
			Online:       c.Online,
			BytesUsed:    c.BytesUsed(),
			BytesTotal:   c.TotalBytes,
			TypesUsed:    c.TypesUsed(),
			TypesTotal:   c.MaxTypes,
			Partition:    partCopy,
			Inverter:     c.Inverter,
			Items:        itemsCopy,
		})
	}
	return views
}

// SummaryMetrics returns (totalOnlineItems, totalOnlineBytesUsed, totalOnlineBytesCapacity).
func (s *State) SummaryMetrics() (int, int, int) {
	if s == nil {
		return 0, 0, 0
	}
	items, used, capBytes := 0, 0, 0
	for _, c := range s.Cells {
		if !c.Online {
			continue
		}
		items += c.TotalItemCount()
		used += c.BytesUsed()
		capBytes += c.TotalBytes
	}
	return items, used, capBytes
}

func (s *State) hasOnlineCells(networkID string) bool {
	for _, c := range s.Cells {
		if c.Online && (networkID == "" || c.NetworkID == networkID) {
			return true
		}
	}
	return false
}

func (s *State) driveBlocks(networkID string) []domain.PlacedBlock {
	var blocks []domain.PlacedBlock
	for _, c := range s.Cells {
		if networkID == "" || c.NetworkID == networkID {
			blocks = append(blocks, c.DriveBlock)
		}
	}
	return blocks
}

func parseDriveCells(cfg map[string]any) []rawCellConfig {
	if cfg == nil {
		return nil
	}
	raw, ok := cfg["cells"]
	if !ok || raw == nil {
		return nil
	}
	b, err := json.Marshal(raw)
	if err != nil {
		return nil
	}
	var out []rawCellConfig
	if err := json.Unmarshal(b, &out); err != nil {
		return nil
	}
	return out
}

func dedupeStrings(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		trimmed := strings.TrimSpace(s)
		if trimmed == "" || seen[trimmed] {
			continue
		}
		seen[trimmed] = true
		out = append(out, trimmed)
	}
	return out
}
