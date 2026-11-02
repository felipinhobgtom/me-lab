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
