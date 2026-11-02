package energy

import (
	"github.com/felipinho/me-lab/internal/domain"
	"github.com/felipinho/me-lab/internal/simulation/channels"
	"github.com/felipinho/me-lab/internal/simulation/diagnostics"
	"github.com/felipinho/me-lab/internal/simulation/network"
	"github.com/felipinho/me-lab/internal/simulation/world"
)

// InfiniteEnergyCapacity is a finite, JSON-serializable AE sentinel representing Creative Energy Cell capacity.
const InfiniteEnergyCapacity = 1_000_000_000.0

// NetworkEnergy holds per-network AE energy state across simulation ticks.
type NetworkEnergy struct {
	NetworkID         string  `json:"network_id"`
	Stored            float64 `json:"stored"`
	Capacity          float64 `json:"capacity"`
	Generation        float64 `json:"generation"`
	BaseConsumption   float64 `json:"base_consumption"`
	Consumption       float64 `json:"consumption"`
	HasCreativeSource bool    `json:"has_creative_source"`
	HasDevices        bool    `json:"has_devices"`
	StructuralOffline bool    `json:"structural_offline"`
	Online            bool    `json:"online"`
}

// Initialize computes initial energy capacity, generation, passive consumption, initial stored buffer,
// and online power status for each ME network.
func Initialize(
	simID string,
	nets []*network.Network,
	alloc *channels.Allocation,
	profile domain.CompatibilityProfile,
) ([]*NetworkEnergy, []domain.Diagnostic) {
	nodeBaseBuf := profile.Config.NodeBaseEnergyBuffer
	if nodeBaseBuf <= 0 {
		nodeBaseBuf = 25.0
	}
	ctrlBuf := profile.Config.ControllerEnergyBuffer
	if ctrlBuf <= 0 {
		ctrlBuf = 8000.0
	}
	ctrlCost := profile.Config.ControllerEnergyPerBlock
	if ctrlCost <= 0 {
		ctrlCost = 6.0
	}
	hopCost := profile.Config.ChannelHopEnergyCost
	if hopCost <= 0 {
		hopCost = 0.0078125
	}
	fePerAE := profile.Config.FEPerAE
	if fePerAE <= 0 {
		fePerAE = 2.0
	}

	states := make([]*NetworkEnergy, 0, len(nets))
	diags := make([]domain.Diagnostic, 0)

	for _, net := range nets {
		var (
			capacity          float64
			nodeAndCtrlBuffer float64
			cellInitialCharge float64
			generation        float64
			baseConsumption   float64
			hasCreative       bool
			hasDevices        bool
		)

		for _, n := range net.Nodes {
			capacity += nodeBaseBuf
			nodeAndCtrlBuffer += nodeBaseBuf

			if n.EffectiveChannelUsage() > 0 || n.IsCPUBlock || n.IsController {
				hasDevices = true
			}

			switch {
			case n.IsCreativeEnergyCell():
				hasCreative = true

			case n.IsController:
				cCap := n.Def.Metadata.EnergyCapacity
				if cCap <= 0 {
					cCap = ctrlBuf
				}
				capacity += cCap
				nodeAndCtrlBuffer += cCap
				baseConsumption += ctrlCost

			case n.IsEnergyCell():
				cCap := n.Def.Metadata.EnergyCapacity
				if cCap <= 0 {
					cCap = 200_000.0
				}
				capacity += cCap
				pct := readChargePct(n.Block.Config)
				cellInitialCharge += cCap * (pct / 100.0)
				baseConsumption += n.Def.Metadata.EnergyCost

			case n.IsDenseEnergyCell():
				cCap := n.Def.Metadata.EnergyCapacity
				if cCap <= 0 {
					cCap = 1_600_000.0
				}
				capacity += cCap
				pct := readChargePct(n.Block.Config)
				cellInitialCharge += cCap * (pct / 100.0)
				baseConsumption += n.Def.Metadata.EnergyCost

			case n.IsEnergyAcceptor():
				capacity += n.Def.Metadata.EnergyCapacity
				nodeAndCtrlBuffer += n.Def.Metadata.EnergyCapacity
				feIn, hasKey := world.ConfigFloat(n.Block.Config, "fe_input_per_tick", 800.0)
				if !hasKey && n.Def.Metadata.EnergyGeneration > 0 {
					generation += n.Def.Metadata.EnergyGeneration
				} else if feIn > 0 {
					generation += feIn / fePerAE
				}
				baseConsumption += n.Def.Metadata.EnergyCost

			default:
				capacity += n.Def.Metadata.EnergyCapacity
				nodeAndCtrlBuffer += n.Def.Metadata.EnergyCapacity
				baseConsumption += n.Def.Metadata.EnergyCost
				if n.Def.Metadata.EnergyGeneration > 0 {
					generation += n.Def.Metadata.EnergyGeneration
				}
			}
		}

		structuralOffline := net.HasController && !net.ControllerValid
		if alloc != nil {
			if nc, ok := alloc.ByNetwork[net.ID]; ok {
				baseConsumption += float64(nc.TotalHops) * hopCost
				if nc.AdHocOverloaded {
					structuralOffline = true
				}
			}
		}

		var stored float64
		var energyOnline bool

		if hasCreative {
			if capacity < InfiniteEnergyCapacity {
				capacity = InfiniteEnergyCapacity
			}
			generation = InfiniteEnergyCapacity
			stored = capacity
			energyOnline = true
		} else {
			if generation > 0 && generation >= baseConsumption {
				stored = clamp(cellInitialCharge+nodeAndCtrlBuffer, 0, capacity)
			} else {
				stored = clamp(cellInitialCharge, 0, capacity)
			}

			if baseConsumption > 0 {
				energyOnline = (stored + generation) >= baseConsumption
			} else if hasDevices {
				energyOnline = stored > 0 || generation > 0
			} else {
				// Passive cable-only segment without power sources does not emit error unless devices exist,
				// though its power state reflects whether any power source is present.
				energyOnline = stored > 0 || generation > 0
			}

			if !energyOnline && (baseConsumption > 0 || hasDevices) {
				diags = append(diags, diagnostics.NewInsufficientEnergy(
					simID,
					net.ID,
					net.Blocks(),
					stored+generation,
					baseConsumption,
				))
			}
		}

		ne := &NetworkEnergy{
			NetworkID:         net.ID,
			Stored:            stored,
			Capacity:          capacity,
			Generation:        generation,
			BaseConsumption:   baseConsumption,
			Consumption:       baseConsumption,
			HasCreativeSource: hasCreative,
			HasDevices:        hasDevices,
			StructuralOffline: structuralOffline,
			Online:            energyOnline && !structuralOffline,
		}
		states = append(states, ne)
	}

	SyncCableOnlineStatus(alloc, states)
	return states, diagnostics.DeduplicateAndSort(diags)
}

// BridgeQuartzFibers shares AE power generation across ME networks bridged by Quartz Fibers
// while preserving channel and storage isolation between subnetworks.
func BridgeQuartzFibers(
	simID string,
	w *world.World,
	nets []*network.Network,
	states []*NetworkEnergy,
	diags *[]domain.Diagnostic,
	alloc *channels.Allocation,
) {
	if w == nil || len(nets) < 2 || len(states) < 2 {
		return
	}
	coordToNet := make(map[domain.Coord3D]string)
	for _, net := range nets {
		for _, n := range net.Nodes {
			coordToNet[n.Coord] = net.ID
		}
	}

	adjNets := make(map[string]map[string]bool)
	link := func(a, b string) {
		if a == "" || b == "" || a == b {
			return
		}
		if adjNets[a] == nil {
			adjNets[a] = make(map[string]bool)
		}
		if adjNets[b] == nil {
			adjNets[b] = make(map[string]bool)
		}
		adjNets[a][b] = true
		adjNets[b][a] = true
	}

	for _, n := range w.Nodes {
		if n.IsQuartzFiber() {
			var touching []string
			for _, dir := range domain.AllDirections() {
				if nid := coordToNet[n.Coord.Adjacent(dir)]; nid != "" {
					touching = append(touching, nid)
				}
			}
			for i := range touching {
				for j := i + 1; j < len(touching); j++ {
					link(touching[i], touching[j])
				}
			}
		}
		if len(n.QuartzFiberFaces) > 0 {
			srcNet := coordToNet[n.Coord]
			for dir := range n.QuartzFiberFaces {
				dstNet := coordToNet[n.Coord.Adjacent(dir)]
				link(srcNet, dstNet)
			}
		}
	}
	if len(adjNets) == 0 {
		return
	}

	stateByID := make(map[string]*NetworkEnergy, len(states))
	for _, st := range states {
		stateByID[st.NetworkID] = st
	}

	visited := make(map[string]bool)
	poweredNetIDs := make(map[string]bool)
	for _, st := range states {
		if visited[st.NetworkID] || len(adjNets[st.NetworkID]) == 0 {
			continue
		}
		queue := []string{st.NetworkID}
		visited[st.NetworkID] = true
		var group []*NetworkEnergy
		var totalGen, totalDrain, totalStored float64
		hasCreative := false
		for len(queue) > 0 {
			cur := queue[0]
			queue = queue[1:]
			if s := stateByID[cur]; s != nil {
				group = append(group, s)
				totalGen += s.Generation
				totalDrain += s.BaseConsumption
				totalStored += s.Stored
				if s.HasCreativeSource {
					hasCreative = true
				}
			}
			for nb := range adjNets[cur] {
				if !visited[nb] {
					visited[nb] = true
					queue = append(queue, nb)
				}
			}
		}
		if hasCreative || (totalGen+totalStored >= totalDrain && totalGen > 0) {
			surplus := totalGen - totalDrain
			if surplus < 0 {
				surplus = 0
			}
			for _, member := range group {
				if member.Generation == 0 {
					member.Generation = member.BaseConsumption + surplus
					if member.Stored == 0 && member.Capacity > 0 {
						member.Stored = member.Capacity
					}
				}
				if !member.StructuralOffline {
					member.Online = true
					poweredNetIDs[member.NetworkID] = true
				}
			}
		}
	}

	if len(poweredNetIDs) > 0 && diags != nil {
		filtered := make([]domain.Diagnostic, 0, len(*diags))
		for _, d := range *diags {
			if d.Code == diagnostics.CodeInsufficientEnergy && len(d.AffectedBlocks) > 0 {
				fb := d.AffectedBlocks[0]
				if poweredNetIDs[coordToNet[domain.Coord3D{X: fb.X, Y: fb.Y, Z: fb.Z}]] {
					continue
				}
			}
			filtered = append(filtered, d)
		}
		*diags = filtered
	}
	SyncCableOnlineStatus(alloc, states)
	_ = simID
}

// StepEnergy advances network energy buffers by 1 tick, accounting for active Molecular Assemblers.
func StepEnergy(
	simID string,
	states []*NetworkEnergy,
	netsByID map[string]*network.Network,
	activeAssemblersByNet map[string]int,
	alloc *channels.Allocation,
	profile domain.CompatibilityProfile,
) []domain.Diagnostic {
	activeAssemblerCost := profile.Config.MolecularAssemblerActiveE
	if activeAssemblerCost <= 0 {
		activeAssemblerCost = 20.0
	}

	diags := make([]domain.Diagnostic, 0)
	for _, ne := range states {
		activeCount := 0
		if activeAssemblersByNet != nil {
			activeCount = activeAssemblersByNet[ne.NetworkID]
		}
		ne.Consumption = ne.BaseConsumption + float64(activeCount)*activeAssemblerCost

		if ne.HasCreativeSource {
			ne.Stored = ne.Capacity
			ne.Online = !ne.StructuralOffline
			continue
		}

		available := ne.Stored + ne.Generation
		insufficient := false
		if ne.Consumption > 0 {
			insufficient = available < ne.Consumption
		} else if ne.HasDevices {
			insufficient = available <= 0
		}

		ne.Stored = clamp(ne.Stored+ne.Generation-ne.Consumption, 0, ne.Capacity)

		if insufficient {
			ne.Online = false
			var affected []domain.PlacedBlock
			if netsByID != nil {
				if net, ok := netsByID[ne.NetworkID]; ok {
					affected = net.Blocks()
				}
			}
			diags = append(diags, diagnostics.NewInsufficientEnergy(
				simID,
				ne.NetworkID,
				affected,
				available,
				ne.Consumption,
			))
		} else {
			ne.Online = !ne.StructuralOffline
		}
	}

	SyncCableOnlineStatus(alloc, states)
	return diagnostics.DeduplicateAndSort(diags)
}

// SyncCableOnlineStatus updates CableChannelOverlay.Online flags from current network energy states.
func SyncCableOnlineStatus(alloc *channels.Allocation, states []*NetworkEnergy) {
	if alloc == nil {
		return
	}
	onlineMap := make(map[string]bool, len(states))
	for _, s := range states {
		onlineMap[s.NetworkID] = s.Online
	}
	for i := range alloc.CableOverlays {
		alloc.CableOverlays[i].Online = onlineMap[alloc.CableOverlays[i].NetworkID]
	}
}

func readChargePct(cfg map[string]any) float64 {
	pct, ok := world.ConfigFloat(cfg, "initial_charge_pct", 100.0)
	if !ok {
		return 100.0
	}
	return clamp(pct, 0, 100)
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
