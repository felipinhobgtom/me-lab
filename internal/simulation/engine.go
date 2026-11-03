package simulation

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/felipinho/me-lab/internal/domain"
	"github.com/felipinho/me-lab/internal/simulation/channels"
	"github.com/felipinho/me-lab/internal/simulation/crafting"
	"github.com/felipinho/me-lab/internal/simulation/diagnostics"
	"github.com/felipinho/me-lab/internal/simulation/energy"
	"github.com/felipinho/me-lab/internal/simulation/network"
	"github.com/felipinho/me-lab/internal/simulation/storage"
	"github.com/felipinho/me-lab/internal/simulation/world"
)

// ErrNilState is returned when Step is invoked with a nil EngineState.
var ErrNilState = errors.New("simulation: nil engine state")

// Snapshot captures the complete deterministic input configuration for validation or simulation.
type Snapshot struct {
	SimulationID     string                            `json:"simulation_id,omitempty"`
	ProjectID        string                            `json:"project_id,omitempty"`
	Seed             int64                             `json:"seed"`
	StartedAt        time.Time                         `json:"started_at,omitempty"`
	Profile          domain.CompatibilityProfile       `json:"profile"`
	Catalog          map[string]domain.BlockDefinition `json:"catalog,omitempty"`
	Blocks           []domain.PlacedBlock              `json:"blocks"`
	Patterns         []domain.ProjectPattern           `json:"patterns,omitempty"`
	Inventory        []domain.ProjectInventoryEntry    `json:"inventory,omitempty"`
	CraftingRequests []domain.CraftingRequest          `json:"crafting_requests,omitempty"`
}

// NetworkMeta stores static structural metadata for an ME network inside EngineState.
type NetworkMeta struct {
	ID                string               `json:"id"`
	HasController     bool                 `json:"has_controller"`
	ControllerValid   bool                 `json:"controller_valid"`
	IsAdHoc           bool                 `json:"is_ad_hoc"`
	BlockCount        int                  `json:"block_count"`
	ChannelsUsed      int                  `json:"channels_used"`
	ChannelsAvailable int                  `json:"channels_available"`
	Blocks            []domain.PlacedBlock `json:"blocks"`
}

// EngineState holds the complete, pure-JSON-serializable runtime state of a simulation run.
type EngineState struct {
	SimulationID          string                       `json:"simulation_id,omitempty"`
	ProjectID             string                       `json:"project_id,omitempty"`
	Seed                  int64                        `json:"seed"`
	Status                domain.SimulationStatus      `json:"status"`
	CurrentTick           int                          `json:"current_tick"`
	NextSequence          int                          `json:"next_sequence"`
	StartedAt             time.Time                    `json:"started_at,omitempty"`
	FinishedAt            *time.Time                   `json:"finished_at,omitempty"`
	NetworkMetas          []NetworkMeta                `json:"network_metas"`
	Energy                []*energy.NetworkEnergy      `json:"energy"`
	CableOverlays         []domain.CableChannelOverlay `json:"cable_overlays"`
	DeviceRoutes          []domain.DeviceRouteOverlay  `json:"device_routes"`
	HasChannelByBlockID   map[string]bool              `json:"has_channel_by_block_id"`
	Storage               *storage.State               `json:"storage"`
	Crafting              *crafting.State              `json:"crafting"`
	StructuralDiagnostics []domain.Diagnostic          `json:"structural_diagnostics"`
	Diagnostics           []domain.Diagnostic          `json:"diagnostics"`
	Summary               domain.SimulationSummary     `json:"summary"`
	Overlay               domain.TopologyOverlay       `json:"overlay"`
}

// Engine provides stateless deterministic methods for AE2 validation and tick simulation.
type Engine struct{}

// NewEngine creates a new pure-Go AE2 simulation Engine.
func NewEngine() *Engine {
	return &Engine{}
}

// Validate runs static & topological AE2 validation without mutating external state.
func (e *Engine) Validate(snapshot Snapshot) domain.ValidationResult {
	return Validate(snapshot)
}

// Initialize constructs the tick-0 EngineState, plans requested crafting jobs, and returns initial events & diagnostics.
func (e *Engine) Initialize(snapshot Snapshot) (*EngineState, []domain.SimulationEvent, []domain.Diagnostic, error) {
	return Initialize(snapshot)
}

// Step advances EngineState by 1 tick.
func (e *Engine) Step(state *EngineState, profile domain.CompatibilityProfile, catalog map[string]domain.BlockDefinition) (domain.SimulationStepResult, error) {
	return Step(state, profile, catalog)
}

// StepN advances EngineState by up to ticks steps (clamped to >= 1).
func (e *Engine) StepN(state *EngineState, ticks int, profile domain.CompatibilityProfile, catalog map[string]domain.BlockDefinition) (domain.SimulationStepResult, error) {
	return StepN(state, ticks, profile, catalog)
}

// BuildCatalogMap indexes a slice of BlockDefinitions by their ID.
func BuildCatalogMap(defs []domain.BlockDefinition) map[string]domain.BlockDefinition {
	m := make(map[string]domain.BlockDefinition, len(defs))
	for _, d := range defs {
		m[d.ID] = d
	}
	return m
}

// Validate executes World -> Network/Multiblocks -> Channels -> Energy -> Storage -> Crafting checks
// and returns a domain.ValidationResult.
func Validate(snapshot Snapshot) domain.ValidationResult {
	state, _, diags, _ := Initialize(snapshot)
	valid := true
	for _, d := range diags {
		if d.Severity == domain.SeverityError || d.BlocksSimulation {
			valid = false
			break
		}
	}
	return domain.ValidationResult{
		Valid:       valid,
		Summary:     state.Summary,
		Diagnostics: diags,
		Overlay:     state.Overlay,
	}
}

// Initialize runs tick-0 topology discovery, channel routing, energy evaluation, storage cell loading,
// and crafting tree planning + ingredient reservation.
func Initialize(snapshot Snapshot) (*EngineState, []domain.SimulationEvent, []domain.Diagnostic, error) {
	simID := snapshot.SimulationID

	// 1. Build 3D World & face-level ME connectivity.
	w := world.Build(simID, snapshot.Blocks, snapshot.Catalog)

	// 2. Discover ME Networks & validate Controller and Crafting CPU multiblocks.
	nets, netDiags := network.Discover(simID, w, snapshot.Profile)

	// 3. Route AE2 channels (infinite, ad-hoc, or 3-tier shortest path).
	alloc := channels.Allocate(simID, w, nets, snapshot.Profile)

	// 4. Initialize network energy buffers, generation, passive consumption, and online states.
	energyStates, energyDiags := energy.Initialize(simID, nets, alloc, snapshot.Profile)
	energy.BridgeQuartzFibers(simID, w, nets, energyStates, &energyDiags, alloc)
	syncDeviceRouteOnline(alloc, energyStates)

	// 5. Initialize ME Drives & Storage Cells and deterministic external automation targets.
	storeState, storeDiags := storage.Initialize(simID, nets, alloc, energyStates, snapshot.Inventory, snapshot.Profile)
	storage.InitializeAutomation(storeState, w, nets, alloc, energyStates)
	// 6. Initialize Pattern Providers, Crafting CPUs, Assemblers/External Machines, and plan CraftingRequests.
	nextSeq := 1
	emitter := crafting.EventEmitter{
		SimulationID: simID,
		NextSequence: &nextSeq,
	}
	craftState, initEvents, craftDiags := crafting.Initialize(
		simID,
		w,
		nets,
		alloc,
		energyStates,
		storeState,
		snapshot.Patterns,
		snapshot.CraftingRequests,
		snapshot.Profile,
		emitter,
	)

	structural := make([]domain.Diagnostic, 0)
	structural = append(structural, w.Diagnostics...)
	structural = append(structural, netDiags...)
	structural = append(structural, alloc.Diagnostics...)
	structural = append(structural, storeDiags...)
	structural = append(structural, craftDiags...)
	structural = diagnostics.DeduplicateAndSort(structural)

	allDiags := make([]domain.Diagnostic, 0, len(structural)+len(energyDiags))
	allDiags = append(allDiags, structural...)
	allDiags = append(allDiags, energyDiags...)
	allDiags = diagnostics.DeduplicateAndSort(allDiags)

	metas := make([]NetworkMeta, 0, len(nets))
	for _, net := range nets {
		nc := alloc.ByNetwork[net.ID]
		used, avail := 0, 0
		if nc != nil {
			used = nc.ChannelsUsed
			avail = nc.ChannelsAvailable
		}
		metas = append(metas, NetworkMeta{
			ID:                net.ID,
			HasController:     net.HasController,
			ControllerValid:   net.ControllerValid,
			IsAdHoc:           net.IsAdHoc,
			BlockCount:        len(net.Nodes),
			ChannelsUsed:      used,
			ChannelsAvailable: avail,
			Blocks:            net.Blocks(),
		})
	}

	cables := alloc.CableOverlays
	if cables == nil {
		cables = []domain.CableChannelOverlay{}
	}
	routes := alloc.DeviceRoutes
	if routes == nil {
		routes = []domain.DeviceRouteOverlay{}
	}

	state := &EngineState{
		SimulationID:          simID,
		ProjectID:             snapshot.ProjectID,
		Seed:                  snapshot.Seed,
		CurrentTick:           0,
		NextSequence:          nextSeq,
		StartedAt:             snapshot.StartedAt,
		NetworkMetas:          metas,
		Energy:                energyStates,
		CableOverlays:         cables,
		DeviceRoutes:          routes,
		HasChannelByBlockID:   alloc.HasChannelByBlockID,
		Storage:               storeState,
		Crafting:              craftState,
		StructuralDiagnostics: structural,
		Diagnostics:           allDiags,
	}

	state.Status = computeStatus(state, len(snapshot.CraftingRequests) > 0)
	state.recomputeSummaryAndOverlay()

	if initEvents == nil {
		initEvents = []domain.SimulationEvent{}
	}
	return state, initEvents, state.Diagnostics, nil
}

// Step advances EngineState by 1 deterministic simulation tick.
func Step(
	state *EngineState,
	profile domain.CompatibilityProfile,
	catalog map[string]domain.BlockDefinition,
) (domain.SimulationStepResult, error) {
	_ = catalog // Topology and normalized metadata are captured inside EngineState at initialization.
	if state == nil {
		return domain.SimulationStepResult{}, ErrNilState
	}

	if state.Status == domain.SimStatusPaused {
		return domain.SimulationStepResult{
			Tick:        state.CurrentTick,
			Events:      []domain.SimulationEvent{},
			Diagnostics: state.Diagnostics,
			State:       state.ToSimulationRun(),
		}, nil
	}

	state.CurrentTick++
	tick := state.CurrentTick
	if state.NextSequence <= 0 {
		state.NextSequence = 1
	}

	// Build network block lookup for energy brownout diagnostics.
	netsForEnergy := make(map[string]*network.Network, len(state.NetworkMetas))
	for _, m := range state.NetworkMetas {
		nodes := make([]*world.Node, len(m.Blocks))
		for i, b := range m.Blocks {
			nodes[i] = &world.Node{Block: b, Coord: b.Coord()}
		}
		netsForEnergy[m.ID] = &network.Network{
			ID:    m.ID,
			Nodes: nodes,
		}
	}

	// 1. Advance Energy tick (charging active Molecular Assembler AE/t drain).
	activeAssemblers := state.Crafting.ActiveAssemblersByNetwork()
	tempAlloc := &channels.Allocation{
		CableOverlays: state.CableOverlays,
	}
	energyDiags := energy.StepEnergy(state.SimulationID, state.Energy, netsForEnergy, activeAssemblers, tempAlloc, profile)
	state.CableOverlays = tempAlloc.CableOverlays
	syncStateDeviceRouteOnline(state)

	// Sync online status across ME Storage, automation buses, and Crafting providers/CPUs.
	if state.Storage != nil {
		state.Storage.SyncOnlineStatus(state.Energy, state.HasChannelByBlockID)
	}
	if state.Crafting != nil {
		state.Crafting.SyncOnlineStatus(state.Energy, state.HasChannelByBlockID)
	}

	emitter := crafting.EventEmitter{
		SimulationID: state.SimulationID,
		NextSequence: &state.NextSequence,
	}
	tickEvents := make([]domain.SimulationEvent, 0)
	if state.Storage != nil {
		state.Storage.StepExternalMachines()
	}
	if state.Storage != nil {
		onlineByNetwork := make(map[string]bool, len(state.Energy))
		for _, es := range state.Energy {
			onlineByNetwork[es.NetworkID] = es.Online
		}
		for _, transfer := range state.Storage.StepBuses(onlineByNetwork) {
			tickEvents = append(tickEvents, emitter.Emit(tick, "bus_transfer", map[string]any{
				"bus_id": transfer.BusID, "kind": transfer.Kind, "network_id": transfer.NetworkID,
				"source_id": transfer.SourceID, "target_id": transfer.TargetID,
				"item_id": transfer.ItemID, "quantity": transfer.Quantity,
			}))
		}
	}

	// 2. Advance running Molecular Assemblers & External Machines, return outputs, and dispatch ready batches.
	craftEvents, craftStepDiags := state.Crafting.StepTick(
		state.SimulationID,
		tick,
		state.Energy,
		state.Storage,
		emitter,
	)
	tickEvents = append(tickEvents, craftEvents...)

	// Persist permanent crafting/storage errors (e.g. NO_ADJACENT_CRAFTING_INVENTORY, STORAGE_FULL)
	// while keeping PROVIDER_BLOCKED and INSUFFICIENT_ENERGY scoped to current tick conditions.
	var transientDiags []domain.Diagnostic
	for _, d := range craftStepDiags {
		if d.Code == diagnostics.CodeProviderBlocked {
			transientDiags = append(transientDiags, d)
		} else {
			state.StructuralDiagnostics = append(state.StructuralDiagnostics, d)
		}
	}
	state.StructuralDiagnostics = diagnostics.DeduplicateAndSort(state.StructuralDiagnostics)

	combined := make([]domain.Diagnostic, 0, len(state.StructuralDiagnostics)+len(energyDiags)+len(transientDiags))
	combined = append(combined, state.StructuralDiagnostics...)
	combined = append(combined, energyDiags...)
	combined = append(combined, transientDiags...)
	state.Diagnostics = diagnostics.DeduplicateAndSort(combined)

	hasJobs := state.Crafting != nil && len(state.Crafting.Jobs) > 0
	state.Status = computeStatus(state, hasJobs)
	state.recomputeSummaryAndOverlay()

	if tickEvents == nil {
		tickEvents = []domain.SimulationEvent{}
	}
	return domain.SimulationStepResult{
		Tick:        state.CurrentTick,
		Events:      tickEvents,
		Diagnostics: state.Diagnostics,
		State:       state.ToSimulationRun(),
	}, nil
}

// StepN advances EngineState by n ticks (defaulting to 1 if n <= 0), aggregating emitted events in sequence order.
func StepN(
	state *EngineState,
	ticks int,
	profile domain.CompatibilityProfile,
	catalog map[string]domain.BlockDefinition,
) (domain.SimulationStepResult, error) {
	if state == nil {
		return domain.SimulationStepResult{}, ErrNilState
	}
	if ticks <= 0 {
		ticks = 1
	}
	allEvents := make([]domain.SimulationEvent, 0)
	var last domain.SimulationStepResult
	for range ticks {
		res, err := Step(state, profile, catalog)
		if err != nil {
			return domain.SimulationStepResult{}, err
		}
		allEvents = append(allEvents, res.Events...)
		last = res
		if state.Status == domain.SimStatusPaused {
			break
		}
	}
	last.Events = allEvents
	return last, nil
}

// Pause transitions a running simulation to paused.
func Pause(state *EngineState) {
	if state != nil && state.Status == domain.SimStatusRunning {
		state.Status = domain.SimStatusPaused
	}
}

// Resume transitions a paused simulation back to running.
func Resume(state *EngineState) {
	if state != nil && state.Status == domain.SimStatusPaused {
		hasJobs := state.Crafting != nil && len(state.Crafting.Jobs) > 0
		state.Status = computeStatus(state, hasJobs)
		if state.Status == domain.SimStatusPaused {
			state.Status = domain.SimStatusRunning
		}
	}
}

// ToSimulationRun projects EngineState into a domain.SimulationRun model.
func (s *EngineState) ToSimulationRun() domain.SimulationRun {
	overlayCopy := s.Overlay
	return domain.SimulationRun{
		ID:          s.SimulationID,
		ProjectID:   s.ProjectID,
		Status:      s.Status,
		Seed:        s.Seed,
		CurrentTick: s.CurrentTick,
		Summary:     s.Summary,
		Overlay:     &overlayCopy,
		StartedAt:   s.StartedAt,
		FinishedAt:  s.FinishedAt,
	}
}

// MarshalState serializes EngineState to JSON bytes.
func MarshalState(state *EngineState) ([]byte, error) {
	if state == nil {
		return nil, ErrNilState
	}
	return json.Marshal(state)
}

// UnmarshalState deserializes EngineState from JSON bytes.
func UnmarshalState(data []byte) (*EngineState, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("simulation: empty state JSON")
	}
	var st EngineState
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, fmt.Errorf("simulation: unmarshal state: %w", err)
	}
	return &st, nil
}

// MarshalSnapshot serializes Snapshot to JSON bytes.
func MarshalSnapshot(snap Snapshot) ([]byte, error) {
	return json.Marshal(snap)
}

// UnmarshalSnapshot deserializes Snapshot from JSON bytes.
func UnmarshalSnapshot(data []byte) (Snapshot, error) {
	if len(data) == 0 {
		return Snapshot{}, fmt.Errorf("simulation: empty snapshot JSON")
	}
	var snap Snapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return Snapshot{}, fmt.Errorf("simulation: unmarshal snapshot: %w", err)
	}
	return snap, nil
}

func syncDeviceRouteOnline(alloc *channels.Allocation, states []*energy.NetworkEnergy) {
	if alloc == nil {
		return
	}
	onlineByNetwork := make(map[string]bool, len(states))
	for _, state := range states {
		onlineByNetwork[state.NetworkID] = state.Online
	}
	for i := range alloc.DeviceRoutes {
		route := &alloc.DeviceRoutes[i]
		route.Online = route.HasChannel && onlineByNetwork[route.NetworkID]
	}
}

func syncStateDeviceRouteOnline(state *EngineState) {
	if state == nil {
		return
	}
	onlineByNetwork := make(map[string]bool, len(state.Energy))
	for _, energyState := range state.Energy {
		onlineByNetwork[energyState.NetworkID] = energyState.Online
	}
	for i := range state.DeviceRoutes {
		route := &state.DeviceRoutes[i]
		route.Online = route.HasChannel && onlineByNetwork[route.NetworkID]
	}
}

func computeStatus(state *EngineState, hasJobs bool) domain.SimulationStatus {
	if state.Status == domain.SimStatusPaused {
		return domain.SimStatusPaused
	}
	active, completed, failed := 0, 0, 0
	if state.Crafting != nil {
		active, completed, failed = state.Crafting.SummaryCounts()
	}

	if hasJobs {
		if active > 0 {
			return domain.SimStatusRunning
		}
		if failed > 0 && completed == 0 {
			return domain.SimStatusFailed
		}
		if completed > 0 {
			return domain.SimStatusCompleted
		}
	}

	for _, d := range state.Diagnostics {
		if d.BlocksSimulation {
			return domain.SimStatusFailed
		}
	}
	return domain.SimStatusRunning
}

func (s *EngineState) recomputeSummaryAndOverlay() {
	energyByNet := make(map[string]*energy.NetworkEnergy, len(s.Energy))
	onlineNets := 0
	var totStored, totCap, totGen, totCons float64
	for _, es := range s.Energy {
		energyByNet[es.NetworkID] = es
		if es.Online {
			onlineNets++
		}
		totStored += es.Stored
		totCap += es.Capacity
		totGen += es.Generation
		totCons += es.Consumption
	}

	netOverlays := make([]domain.NetworkSummaryOverlay, 0, len(s.NetworkMetas))
	totChUsed, totChAvail := 0, 0
	for _, m := range s.NetworkMetas {
		totChUsed += m.ChannelsUsed
		totChAvail += m.ChannelsAvailable
		es := energyByNet[m.ID]
		var eStored, eCap, eGen, eCons float64
		var online bool
		if es != nil {
			online = es.Online
			eStored = es.Stored
			eCap = es.Capacity
			eGen = es.Generation
			eCons = es.Consumption
		}
		netOverlays = append(netOverlays, domain.NetworkSummaryOverlay{
			ID:                m.ID,
			HasController:     m.HasController,
			ControllerValid:   m.ControllerValid,
			IsAdHoc:           m.IsAdHoc,
			Online:            online,
			BlockCount:        m.BlockCount,
			ChannelsUsed:      m.ChannelsUsed,
			ChannelsAvailable: m.ChannelsAvailable,
			EnergyStored:      eStored,
			EnergyCapacity:    eCap,
			EnergyGeneration:  eGen,
			EnergyConsumption: eCons,
		})
	}

	itemsStored, storageBytesUsed, storageCapacity := 0, 0, 0
	cellViews := []domain.StorageCellView{}
	storedTotals := []domain.ItemStack{}
	automationBuses := []domain.AutomationBusOverlay{}
	externalInventories := []domain.ExternalInventoryView{}
	if s.Storage != nil {
		itemsStored, storageBytesUsed, storageCapacity = s.Storage.SummaryMetrics()
		cellViews = s.Storage.CellViews()
		storedTotals = s.Storage.AvailableTotals()
		automationBuses = s.Storage.BusViews()
		externalInventories = s.Storage.ExternalInventoryViews()
	}

	activeJobs, completedJobs, failedJobs := 0, 0, 0
	jobViews := []domain.CraftingJobView{}
	if s.Crafting != nil {
		activeJobs, completedJobs, failedJobs = s.Crafting.SummaryCounts()
		jobViews = s.Crafting.JobViews()
	}

	cables := s.CableOverlays
	if cables == nil {
		cables = []domain.CableChannelOverlay{}
	}
	routes := s.DeviceRoutes
	if routes == nil {
		routes = []domain.DeviceRouteOverlay{}
	}

	s.Summary = domain.SimulationSummary{
		Networks:          len(s.NetworkMetas),
		OnlineNetworks:    onlineNets,
		ChannelsUsed:      totChUsed,
		ChannelsAvailable: totChAvail,
		EnergyStored:      totStored,
		EnergyCapacity:    totCap,
		EnergyGeneration:  totGen,
		EnergyConsumption: totCons,
		ItemsStored:       itemsStored,
		StorageBytesUsed:  storageBytesUsed,
		StorageCapacity:   storageCapacity,
		ActiveJobs:        activeJobs,
		CompletedJobs:     completedJobs,
		FailedJobs:        failedJobs,
		DiagnosticsCount:  len(s.Diagnostics),
	}

	s.Overlay = domain.TopologyOverlay{
		Networks:            netOverlays,
		Cables:              cables,
		DeviceRoutes:        routes,
		StorageCells:        cellViews,
		StoredTotals:        storedTotals,
		CraftingJobs:        jobViews,
		AutomationBuses:     automationBuses,
		ExternalInventories: externalInventories,
	}
}
