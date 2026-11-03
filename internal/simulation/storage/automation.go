package storage

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/felipinho/me-lab/internal/domain"
	"github.com/felipinho/me-lab/internal/simulation/channels"
	"github.com/felipinho/me-lab/internal/simulation/energy"
	"github.com/felipinho/me-lab/internal/simulation/network"
	"github.com/felipinho/me-lab/internal/simulation/world"
)

// ExternalInventoryState is the mutable inventory attached to an external machine or container.
// It is deliberately small and JSON serializable so simulation resume/reset remains deterministic.
type ExternalInventoryState struct {
	Block    domain.PlacedBlock `json:"block"`
	Capacity int                `json:"capacity"`
	Items    []domain.ItemStack `json:"items"`
}

// BusState models an Import Bus, Export Bus, or Storage Bus attached to a network.
type BusState struct {
	BlockID         string         `json:"block_id"`
	Coord           domain.Coord3D `json:"coord"`
	NetworkID       string         `json:"network_id"`
	Kind            string         `json:"kind"`
	TargetBlockID   string         `json:"target_block_id,omitempty"`
	TargetCoord     domain.Coord3D `json:"target_coord"`
	HasChannel      bool           `json:"has_channel"`
	Online          bool           `json:"online"`
	Filters         []string       `json:"filters,omitempty"`
	Inverter        bool           `json:"inverter"`
	Acceleration    int            `json:"acceleration"`
	LastTransferred int            `json:"last_transferred"`
}

// BusTransfer describes one observable item movement during a simulation tick.
type BusTransfer struct {
	BusID     string
	Kind      string
	NetworkID string
	SourceID  string
	TargetID  string
	ItemID    string
	Quantity  int
}

// InitializeAutomation discovers external inventories and buses from the world snapshot.
func InitializeAutomation(
	st *State,
	w *world.World,
	nets []*network.Network,
	alloc *channels.Allocation,
	energyStates []*energy.NetworkEnergy,
) {
	if st == nil || w == nil {
		return
	}
	st.ExternalInventories = make([]*ExternalInventoryState, 0)
	st.Buses = make([]*BusState, 0)

	coordToNetwork := make(map[domain.Coord3D]string)
	for _, net := range nets {
		for _, node := range net.Nodes {
			coordToNetwork[node.Coord] = net.ID
		}
	}

	inventoryByCoord := make(map[domain.Coord3D]*ExternalInventoryState)
	for _, node := range w.Nodes {
		if !isExternalInventory(node) {
			continue
		}
		inv := &ExternalInventoryState{
			Block:    node.Block,
			Capacity: inventoryCapacity(node.Block),
			Items:    parseConfiguredItems(node.Block.Config),
		}
		st.ExternalInventories = append(st.ExternalInventories, inv)
		inventoryByCoord[node.Coord] = inv
	}

	for _, node := range w.Nodes {
		if node.IsCable {
			for face, subpartID := range node.SubpartsByFace {
				if !isBus(subpartID) {
					continue
				}
				st.Buses = append(st.Buses, newBusState(
					node.Block.ID+"/"+string(face)+"/"+subpartID,
					node.Coord,
					coordToNetwork[node.Coord],
					subpartID,
					node.Coord.Adjacent(face),
					node.Block.Config,
					alloc,
					inventoryByCoord,
				))
			}
		}
		if isBus(node.Block.BlockDefID) {
			facing := node.Facing
			st.Buses = append(st.Buses, newBusState(
				node.Block.ID,
				node.Coord,
				coordToNetwork[node.Coord],
				node.Block.BlockDefID,
				node.Coord.Adjacent(facing),
				node.Block.Config,
				alloc,
				inventoryByCoord,
			))
		}
	}

	sort.Slice(st.ExternalInventories, func(i, j int) bool {
		return st.ExternalInventories[i].Block.ID < st.ExternalInventories[j].Block.ID
	})
	sort.Slice(st.Buses, func(i, j int) bool {
		if st.Buses[i].Coord != st.Buses[j].Coord {
			return domain.CompareCoords(st.Buses[i].Coord, st.Buses[j].Coord) < 0
		}
		return st.Buses[i].BlockID < st.Buses[j].BlockID
	})
	st.SyncOnlineStatus(energyStates, channelMap(alloc))
}

// SyncOnlineStatus gates every bus on both network power and a granted channel.
func (s *State) SyncAutomationOnlineStatus(energyStates []*energy.NetworkEnergy, hasChannel map[string]bool) {
	if s == nil {
		return
	}
	netOnline := make(map[string]bool, len(energyStates))
	for _, state := range energyStates {
		netOnline[state.NetworkID] = state.Online
	}
	for _, bus := range s.Buses {
		if hasChannel != nil {
			if channel, ok := hasChannel[bus.BlockID]; ok {
				bus.HasChannel = channel
			}
		}
		bus.Online = netOnline[bus.NetworkID] && bus.HasChannel
		bus.LastTransferred = 0
	}
}

// StepBuses advances all online buses once. Import and export operations are bounded,
// filter-aware, deterministic, and use ME storage's existing partition/priority rules.
func (s *State) StepBuses(networkEnergy map[string]bool) []BusTransfer {
	if s == nil {
		return nil
	}
	inventoryByID := make(map[string]*ExternalInventoryState, len(s.ExternalInventories))
	for _, inv := range s.ExternalInventories {
		inventoryByID[inv.Block.ID] = inv
	}
	transfers := make([]BusTransfer, 0)
	for _, bus := range s.Buses {
		bus.LastTransferred = 0
		if !bus.HasChannel || !networkEnergy[bus.NetworkID] {
			bus.Online = false
			continue
		}
		bus.Online = true
		inv := inventoryByID[bus.TargetBlockID]
		if inv == nil {
			continue
		}
		remaining := transferLimit(bus.Acceleration)
		switch bus.Kind {
		case "ae2:import_bus":
			for _, stack := range sortedStacks(inv.Items) {
				if remaining <= 0 {
					break
				}
				if !matchesFilter(stack.ItemID, bus.Filters, bus.Inverter) {
					continue
				}
				wanted := minInt(stack.Quantity, remaining)
				rem, _ := s.InsertInNetwork("", bus.NetworkID, stack.ItemID, wanted)
				moved := wanted - rem
				if moved <= 0 {
					continue
				}
				removeInventoryItems(inv, stack.ItemID, moved)
				remaining -= moved
				bus.LastTransferred += moved
				transfers = append(transfers, BusTransfer{BusID: bus.BlockID, Kind: "import", NetworkID: bus.NetworkID, SourceID: bus.TargetBlockID, TargetID: bus.NetworkID, ItemID: stack.ItemID, Quantity: moved})
			}
		case "ae2:export_bus":
			if len(bus.Filters) == 0 {
				continue
			}
			room := inv.Capacity - inventoryItemCount(inv.Items)
			if room <= 0 {
				continue
			}
			if remaining > room {
				remaining = room
			}
			for _, stack := range s.AvailableInNetwork(bus.NetworkID) {
				if remaining <= 0 {
					break
				}
				if !matchesFilter(stack.ItemID, bus.Filters, bus.Inverter) {
					continue
				}
				wanted := minInt(stack.Quantity, remaining)
				moved := s.ExtractFromNetwork(bus.NetworkID, stack.ItemID, wanted)
				if moved <= 0 {
					continue
				}
				addInventoryItems(inv, stack.ItemID, moved)
				remaining -= moved
				bus.LastTransferred += moved
				transfers = append(transfers, BusTransfer{BusID: bus.BlockID, Kind: "export", NetworkID: bus.NetworkID, SourceID: bus.NetworkID, TargetID: bus.TargetBlockID, ItemID: stack.ItemID, Quantity: moved})
			}
		case "ae2:storage_bus":
			// Storage Bus exposes the adjacent inventory as network storage in AE2. The
			// simulator keeps that inventory authoritative and does not duplicate it into cells.
			// Import/Export Bus are the explicit mutation mechanisms in this deterministic model.
		}
	}
	return transfers
}

func newBusState(id string, coord domain.Coord3D, networkID, kind string, target domain.Coord3D, cfg map[string]any, alloc *channels.Allocation, inventories map[domain.Coord3D]*ExternalInventoryState) *BusState {
	filters := configStrings(cfg, "filters")
	if len(filters) == 0 {
		filters = configStrings(cfg, "filter")
	}
	channelID := id
	if _, ok := inventories[target]; ok {
		// Keep the target lookup explicit; the bus remains visible even when it is not connected.
	}
	hasChannel := false
	if alloc != nil {
		hasChannel = alloc.HasChannelByBlockID[channelID]
		if !hasChannel {
			// Cable subparts share the parent cable's route allocation.
			parentID := strings.SplitN(id, "/", 2)[0]
			hasChannel = alloc.HasChannelByBlockID[parentID]
		}
		if !hasChannel {
			// Controller routing records the authoritative per-coordinate result.
			for _, route := range alloc.DeviceRoutes {
				if route.Coord == coord && route.HasChannel {
					hasChannel = true
					break
				}
			}
		}
		if !hasChannel {
			// Cable overlays carry aggregate usage for all subparts on that block.
			for _, cable := range alloc.CableOverlays {
				if cable.Coord == coord && !cable.Overloaded && cable.ChannelsUsed > 0 {
					hasChannel = true
					break
				}
			}
		}
	}
	return &BusState{
		BlockID:       id,
		Coord:         coord,
		NetworkID:     networkID,
		Kind:          kind,
		TargetBlockID: inventoryBlockID(inventories, target),
		TargetCoord:   target,
		HasChannel:    hasChannel,
		Filters:       dedupeStrings(filters),
		Inverter:      world.ConfigBool(cfg, "inverter", false),
		Acceleration:  world.ConfigInt(cfg, "acceleration", 0),
	}
}

func isBus(id string) bool {
	return id == "ae2:import_bus" || id == "ae2:export_bus" || id == "ae2:storage_bus"
}

func isExternalInventory(node *world.Node) bool {
	if node == nil {
		return false
	}
	switch node.Block.BlockDefID {
	case "minecraft:chest", "minecraft:barrel", "minecraft:furnace", "minecraft:blast_furnace", "ae2:inscriber", "ae2:charger", "melab:external_machine":
		return true
	default:
		return node.Def.BehaviorKey == "external_storage" || node.Def.BehaviorKey == "external_machine"
	}
}

func inventoryCapacity(block domain.PlacedBlock) int {
	if configured, ok := world.ConfigFloat(block.Config, "inventory_capacity", 0); ok && configured > 0 {
		return int(configured)
	}
	switch block.BlockDefID {
	case "minecraft:furnace", "minecraft:blast_furnace", "ae2:inscriber", "ae2:charger", "melab:external_machine":
		return 3 * 64
	default:
		return 27 * 64
	}
}

func parseConfiguredItems(cfg map[string]any) []domain.ItemStack {
	if cfg == nil {
		return []domain.ItemStack{}
	}
	raw := cfg["inventory"]
	if raw == nil {
		raw = cfg["items"]
	}
	if raw == nil {
		return []domain.ItemStack{}
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return []domain.ItemStack{}
	}
	var items []domain.ItemStack
	if err := json.Unmarshal(data, &items); err != nil {
		return []domain.ItemStack{}
	}
	return sortedStacks(items)
}

func inventoryBlockID(inventories map[domain.Coord3D]*ExternalInventoryState, coord domain.Coord3D) string {
	if inv := inventories[coord]; inv != nil {
		return inv.Block.ID
	}
	return ""
}

func configStrings(cfg map[string]any, key string) []string {
	if cfg == nil || cfg[key] == nil {
		return nil
	}
	switch value := cfg[key].(type) {
	case string:
		if strings.TrimSpace(value) == "" {
			return nil
		}
		return []string{strings.TrimSpace(value)}
	case []string:
		return append([]string(nil), value...)
	}
	data, err := json.Marshal(cfg[key])
	if err != nil {
		return nil
	}
	var values []string
	if json.Unmarshal(data, &values) != nil {
		return nil
	}
	return values
}

func channelMap(alloc *channels.Allocation) map[string]bool {
	if alloc == nil {
		return nil
	}
	return alloc.HasChannelByBlockID
}

func transferLimit(acceleration int) int {
	switch acceleration {
	case 1:
		return 8
	case 2:
		return 32
	case 3:
		return 64
	case 4:
		return 96
	default:
		return 1
	}
}

func matchesFilter(itemID string, filters []string, inverter bool) bool {
	if len(filters) == 0 {
		return !inverter
	}
	found := false
	for _, filter := range filters {
		if filter == itemID {
			found = true
			break
		}
	}
	if inverter {
		return !found
	}
	return found
}

func sortedStacks(items []domain.ItemStack) []domain.ItemStack {
	out := make([]domain.ItemStack, 0, len(items))
	for _, item := range items {
		if item.ItemID == "" || item.Quantity <= 0 {
			continue
		}
		found := false
		for i := range out {
			if out[i].ItemID == item.ItemID {
				out[i].Quantity += item.Quantity
				found = true
				break
			}
		}
		if !found {
			out = append(out, domain.ItemStack{ItemID: item.ItemID, Quantity: item.Quantity})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ItemID < out[j].ItemID })
	return out
}

func inventoryItemCount(items []domain.ItemStack) int {
	total := 0
	for _, item := range items {
		total += item.Quantity
	}
	return total
}

func addInventoryItems(inv *ExternalInventoryState, itemID string, quantity int) {
	if quantity <= 0 {
		return
	}
	for i := range inv.Items {
		if inv.Items[i].ItemID == itemID {
			inv.Items[i].Quantity += quantity
			return
		}
	}
	inv.Items = append(inv.Items, domain.ItemStack{ItemID: itemID, Quantity: quantity})
	inv.Items = sortedStacks(inv.Items)
}

func removeInventoryItems(inv *ExternalInventoryState, itemID string, quantity int) {
	if quantity <= 0 {
		return
	}
	for i := range inv.Items {
		if inv.Items[i].ItemID != itemID {
			continue
		}
		inv.Items[i].Quantity -= quantity
		if inv.Items[i].Quantity <= 0 {
			inv.Items = append(inv.Items[:i], inv.Items[i+1:]...)
		}
		return
	}
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// BusViews renders live automation telemetry for the API overlay.
func (s *State) BusViews() []domain.AutomationBusOverlay {
	if s == nil || len(s.Buses) == 0 {
		return []domain.AutomationBusOverlay{}
	}
	views := make([]domain.AutomationBusOverlay, 0, len(s.Buses))
	for _, bus := range s.Buses {
		views = append(views, domain.AutomationBusOverlay{
			BlockID: bus.BlockID, Coord: bus.Coord, NetworkID: bus.NetworkID, Kind: bus.Kind,
			TargetBlockID: bus.TargetBlockID, TargetCoord: bus.TargetCoord,
			HasChannel: bus.HasChannel, Online: bus.Online, LastTransferred: bus.LastTransferred,
		})
	}
	return views
}

// ExternalInventoryViews renders mutable external inventories for the API overlay.
func (s *State) ExternalInventoryViews() []domain.ExternalInventoryView {
	if s == nil || len(s.ExternalInventories) == 0 {
		return []domain.ExternalInventoryView{}
	}
	views := make([]domain.ExternalInventoryView, 0, len(s.ExternalInventories))
	for _, inv := range s.ExternalInventories {
		views = append(views, domain.ExternalInventoryView{
			BlockID: inv.Block.ID, Coord: inv.Block.Coord(), Capacity: inv.Capacity, Items: sortedStacks(inv.Items),
		})
	}
	return views
}
