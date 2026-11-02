package crafting

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/felipinho/me-lab/internal/domain"
	"github.com/felipinho/me-lab/internal/simulation/channels"
	"github.com/felipinho/me-lab/internal/simulation/diagnostics"
	"github.com/felipinho/me-lab/internal/simulation/energy"
	"github.com/felipinho/me-lab/internal/simulation/network"
	"github.com/felipinho/me-lab/internal/simulation/storage"
	"github.com/felipinho/me-lab/internal/simulation/world"
)

// ProviderState tracks runtime configuration, round-robin index, and lock state of a Pattern Provider.
type ProviderState struct {
	BlockID         string               `json:"block_id"`
	NetworkID       string               `json:"network_id"`
	Block           domain.PlacedBlock   `json:"block"`
	Priority        int                  `json:"priority"`
	Variant         string               `json:"variant"` // "normal" | "directional"
	BlockingMode    bool                 `json:"blocking_mode"`
	LockMode        string               `json:"lock_mode"` // "none" | "until_result_returned"
	HasChannel      bool                 `json:"has_channel"`
	Online          bool                 `json:"online"`
	Patterns        []domain.PatternSpec `json:"patterns"`
	RoundRobinIndex int                  `json:"round_robin_index"`
	Locked          bool                 `json:"locked"`
	LockedByJobID   string               `json:"locked_by_job_id,omitempty"`
}

// CPUState tracks a Crafting CPU multiblock's online status and active job assignment.
type CPUState struct {
	ID           string               `json:"id"`
	NetworkID    string               `json:"network_id"`
	Blocks       []domain.PlacedBlock `json:"blocks"`
	TotalBytes   int                  `json:"total_bytes"`
	CoProcessors int                  `json:"co_processors"`
	HasChannel   bool                 `json:"has_channel"`
	Online       bool                 `json:"online"`
	ActiveJobID  string               `json:"active_job_id,omitempty"`
}

// JobStep represents a planned pattern execution stage within a Crafting CPU job.
type JobStep struct {
	StepIndex         int                `json:"step_index"`
	ProviderBlockID   string             `json:"provider_block_id"`
	Pattern           domain.PatternSpec `json:"pattern"`
	PrimaryOutputItem string             `json:"primary_output_item"`
	TotalBatches      int                `json:"total_batches"`
	DispatchedBatches int                `json:"dispatched_batches"`
	InFlightBatches   int                `json:"in_flight_batches"`
	CompletedBatches  int                `json:"completed_batches"`
}

// JobState holds complete state for an active, completed, or failed autocrafting job.
type JobState struct {
	ID              string         `json:"id"`
	NetworkID       string         `json:"network_id"`
	CPUID           string         `json:"cpu_id"`
	TargetItem      string         `json:"target_item"`
	TargetQuantity  int            `json:"target_quantity"`
	Status          string         `json:"status"` // "running", "completed", "failed"
	FailureReason   string         `json:"failure_reason,omitempty"`
	CoProcessors    int            `json:"co_processors"`
	BytesUsed       int            `json:"bytes_used"`
	BytesCapacity   int            `json:"bytes_capacity"`
	TotalBatches    int            `json:"total_batches"`
	CompletedCrafts int            `json:"completed_crafts"`
	ItemPool        map[string]int `json:"item_pool"`
	Steps           []*JobStep     `json:"steps"`
}

// MachineTask represents a single recipe batch currently executing inside a Molecular Assembler or External Machine.
type MachineTask struct {
	JobID           string             `json:"job_id"`
	StepIndex       int                `json:"step_index"`
	NetworkID       string             `json:"network_id"`
	ProviderBlockID string             `json:"provider_block_id"`
	MachineBlockID  string             `json:"machine_block_id"`
	PatternID       string             `json:"pattern_id"`
	PatternType     string             `json:"pattern_type"`
	Outputs         []domain.ItemStack `json:"outputs"`
	RemainingTicks  int                `json:"remaining_ticks"`
	TotalTicks      int                `json:"total_ticks"`
}

// MachineState holds state for a Molecular Assembler or External Machine block.
type MachineState struct {
	BlockID      string             `json:"block_id"`
	Block        domain.PlacedBlock `json:"block"`
	MachineType  string             `json:"machine_type"` // "molecular_assembler" | "external_machine"
	NetworkID    string             `json:"network_id,omitempty"`
	ProcessTicks int                `json:"process_ticks"`
	AutoReturn   bool               `json:"auto_return"`
	ActiveTask   *MachineTask       `json:"active_task,omitempty"`
	QueuedTasks  []*MachineTask     `json:"queued_tasks,omitempty"`
}

// State aggregates all JSON-serializable autocrafting state across ticks.
type State struct {
	Providers []*ProviderState `json:"providers"`
	CPUs      []*CPUState      `json:"cpus"`
	Machines  []*MachineState  `json:"machines"`
	Jobs      []*JobState      `json:"jobs"`
}

// EventEmitter creates sequentially numbered deterministic SimulationEvents.
type EventEmitter struct {
	SimulationID string
	NextSequence *int
}

func (e EventEmitter) Emit(tick int, eventType string, payload map[string]any) domain.SimulationEvent {
	seq := 1
	if e.NextSequence != nil {
		seq = *e.NextSequence
		*e.NextSequence = seq + 1
	}
	id := fmt.Sprintf("evt:%06d", seq)
	if e.SimulationID != "" {
		id = fmt.Sprintf("%s:evt:%06d", e.SimulationID, seq)
	}
	if payload == nil {
		payload = map[string]any{}
	}
	return domain.SimulationEvent{
		ID:           id,
		SimulationID: e.SimulationID,
		Sequence:     seq,
		Tick:         tick,
		EventType:    eventType,
		Payload:      payload,
	}
}

// Initialize constructs initial autocrafting state (providers, CPUs, machines) and plans all CraftingRequests.
func Initialize(
	simID string,
	w *world.World,
	nets []*network.Network,
	alloc *channels.Allocation,
	energyStates []*energy.NetworkEnergy,
	store *storage.State,
	projectPatterns []domain.ProjectPattern,
	requests []domain.CraftingRequest,
	profile domain.CompatibilityProfile,
	emitter EventEmitter,
) (*State, []domain.SimulationEvent, []domain.Diagnostic) {
	netOnline := make(map[string]bool, len(energyStates))
	for _, es := range energyStates {
		netOnline[es.NetworkID] = es.Online
	}

	coordToNetID := make(map[domain.Coord3D]string)
	for _, net := range nets {
		for _, n := range net.Nodes {
			coordToNetID[n.Coord] = net.ID
		}
	}

	maDefaultTicks := profile.Config.MolecularAssemblerTicks
	if maDefaultTicks <= 0 {
		maDefaultTicks = 4
	}
	extDefaultTicks := profile.Config.ExternalMachineTicks
	if extDefaultTicks <= 0 {
		extDefaultTicks = 8
	}

	st := &State{
		Providers: make([]*ProviderState, 0),
		CPUs:      make([]*CPUState, 0),
		Machines:  make([]*MachineState, 0),
		Jobs:      make([]*JobState, 0, len(requests)),
	}

	// Map project-level patterns attached via provider_block_id or pattern_ids.
	projPatByID := make(map[string]domain.ProjectPattern, len(projectPatterns))
	projPatByProvider := make(map[string][]domain.PatternSpec)
	for _, pp := range projectPatterns {
		projPatByID[pp.ID] = pp
		if provID := world.ConfigString(pp.Config, "provider_block_id", ""); provID != "" {
			projPatByProvider[provID] = append(projPatByProvider[provID], normalizePattern(domain.PatternSpec{
				ID:          pp.ID,
				PatternType: pp.PatternType,
				Inputs:      pp.Inputs,
				Outputs:     pp.Outputs,
			}, len(projPatByProvider[provID])+1))
		}
	}

	// Discover Pattern Providers and Crafting CPUs per network.
	for _, net := range nets {
		for _, n := range net.Nodes {
			if !n.IsPatternProvider() {
				continue
			}
			hasCh := alloc != nil && alloc.HasChannelByBlockID[n.Block.ID]
			lockMode := strings.ToLower(strings.TrimSpace(world.ConfigString(n.Block.Config, "lock_mode", "none")))
			if lockMode != "until_result_returned" {
				lockMode = "none"
			}
			pats := parseProviderPatterns(n.Block.Config)
			for _, refID := range world.ConfigStringSlice(n.Block.Config, "pattern_ids") {
				if pp, ok := projPatByID[refID]; ok {
					pats = append(pats, normalizePattern(domain.PatternSpec{
						ID:          pp.ID,
						PatternType: pp.PatternType,
						Inputs:      pp.Inputs,
						Outputs:     pp.Outputs,
					}, len(pats)+1))
				}
			}
			pats = append(pats, projPatByProvider[n.Block.ID]...)

			st.Providers = append(st.Providers, &ProviderState{
				BlockID:      n.Block.ID,
				NetworkID:    net.ID,
				Block:        n.Block,
				Priority:     world.ConfigInt(n.Block.Config, "priority", 0),
				Variant:      n.PatternProviderVariant(),
				BlockingMode: world.ConfigBool(n.Block.Config, "blocking_mode", false),
				LockMode:     lockMode,
				HasChannel:   hasCh,
				Online:       netOnline[net.ID] && hasCh,
				Patterns:     pats,
			})
		}

		for _, cpu := range net.CraftingCPUs {
			st.CPUs = append(st.CPUs, &CPUState{
				ID:           cpu.ID,
				NetworkID:    net.ID,
				Blocks:       cpu.Blocks(),
				TotalBytes:   cpu.TotalBytes,
				CoProcessors: cpu.CoProcessors,
				HasChannel:   cpu.HasChannel,
				Online:       netOnline[net.ID] && cpu.HasChannel,
			})
		}
	}

	// Sort providers deterministically by Priority DESC, then Coord ASC, then BlockID ASC.
	sort.Slice(st.Providers, func(i, j int) bool {
		if st.Providers[i].Priority != st.Providers[j].Priority {
			return st.Providers[i].Priority > st.Providers[j].Priority
		}
		if cmp := domain.CompareCoords(st.Providers[i].Block.Coord(), st.Providers[j].Block.Coord()); cmp != 0 {
			return cmp < 0
		}
		return st.Providers[i].BlockID < st.Providers[j].BlockID
	})

	// Discover all Molecular Assemblers and External Machines across the world in canonical coordinate order.
	for _, n := range w.Nodes {
		if n.IsMolecularAssembler() {
			st.Machines = append(st.Machines, &MachineState{
				BlockID:      n.Block.ID,
				Block:        n.Block,
				MachineType:  "molecular_assembler",
				NetworkID:    coordToNetID[n.Coord],
				ProcessTicks: maDefaultTicks,
				AutoReturn:   true,
			})
		} else if n.IsExternalMachine() {
			ticks := world.ConfigInt(n.Block.Config, "process_ticks", extDefaultTicks)
			if ticks <= 0 {
				ticks = extDefaultTicks
			}
			st.Machines = append(st.Machines, &MachineState{
				BlockID:      n.Block.ID,
				Block:        n.Block,
				MachineType:  "external_machine",
				ProcessTicks: ticks,
				AutoReturn:   world.ConfigBool(n.Block.Config, "auto_return", true),
			})
		}
	}

	events := make([]domain.SimulationEvent, 0)
	diags := make([]domain.Diagnostic, 0)

	// Plan each requested CraftingRequest in order.
	for idx, req := range requests {
		jobID := fmt.Sprintf("job-%d", idx+1)
		if req.ItemID == "" || req.Quantity <= 0 {
			continue
		}
		job, jobEvents, jobDiags := st.planAndReserveJob(simID, jobID, req, nets, store, profile, emitter)
		st.Jobs = append(st.Jobs, job)
		events = append(events, jobEvents...)
		diags = append(diags, jobDiags...)
	}

	return st, events, diagnostics.DeduplicateAndSort(diags)
}

func (s *State) planAndReserveJob(
	simID string,
	jobID string,
	req domain.CraftingRequest,
	nets []*network.Network,
	store *storage.State,
	profile domain.CompatibilityProfile,
	emitter EventEmitter,
) (*JobState, []domain.SimulationEvent, []domain.Diagnostic) {
	events := make([]domain.SimulationEvent, 0)
	diags := make([]domain.Diagnostic, 0)

	failJob := func(netID, reason string) *JobState {
		ev := emitter.Emit(0, "job_failed", map[string]any{
			"job_id":   jobID,
			"item_id":  req.ItemID,
			"quantity": req.Quantity,
			"reason":   reason,
		})
		events = append(events, ev)
		return &JobState{
			ID:             jobID,
			NetworkID:      netID,
			TargetItem:     req.ItemID,
			TargetQuantity: req.Quantity,
			Status:         "failed",
			FailureReason:  reason,
			ItemPool:       map[string]int{},
		}
	}

	// Find highest-priority online Pattern Provider producing req.ItemID.
	rootProv, _, foundRoot := s.findPatternForItem("", req.ItemID)
	if !foundRoot {
		var provBlocks []domain.PlacedBlock
		for _, p := range s.Providers {
			provBlocks = append(provBlocks, p.Block)
		}
		diags = append(diags, diagnostics.NewMissingCraftingPattern(simID, provBlocks, req.ItemID))
		return failJob("", diagnostics.CodeMissingCraftingPattern), events, diags
	}

	targetNetID := rootProv.NetworkID
	actualAvail := store.AvailableMap(targetNetID)
	virtualStock := copyIntMap(actualAvail)
	remainingActual := copyIntMap(actualAvail)
	reservedFromStorage := make(map[string]int)

	steps := make([]*JobStep, 0)
	missingRaw := make(map[string]int)

	var expand func(itemID string, neededQty int, isRoot bool, ancestors []string) []string
	expand = func(itemID string, neededQty int, isRoot bool, ancestors []string) []string {
		if neededQty <= 0 {
			return nil
		}
		if !isRoot {
			if avail := virtualStock[itemID]; avail > 0 {
				take := neededQty
				if avail < take {
					take = avail
				}
				virtualStock[itemID] -= take
				if remAct := remainingActual[itemID]; remAct > 0 {
					actTake := take
					if remAct < actTake {
						actTake = remAct
					}
					remainingActual[itemID] -= actTake
					reservedFromStorage[itemID] += actTake
				}
				neededQty -= take
			}
			if neededQty <= 0 {
				return nil
			}
		}

		for idx, anc := range ancestors {
			if anc == itemID {
				cycle := make([]string, 0, len(ancestors)-idx+1)
				cycle = append(cycle, ancestors[idx:]...)
				cycle = append(cycle, itemID)
				return cycle
			}
		}

		prov, pat, ok := s.findPatternForItem(targetNetID, itemID)
		if !ok {
			missingRaw[itemID] += neededQty
			return nil
		}

		outPerBatch := outputQuantityOf(pat, itemID)
		if outPerBatch <= 0 {
			missingRaw[itemID] += neededQty
			return nil
		}

		// Direct self-dependency check even if stock partially covered earlier quantities.
		for _, in := range pat.Inputs {
			if in.ItemID == itemID {
				return []string{itemID, itemID}
			}
		}

		batches := (neededQty + outPerBatch - 1) / outPerBatch
		produced := batches * outPerBatch
		surplus := produced - neededQty

		nextAncestors := make([]string, len(ancestors)+1)
		copy(nextAncestors, ancestors)
		nextAncestors[len(ancestors)] = itemID

		for _, in := range pat.Inputs {
			if in.ItemID == "" || in.Quantity <= 0 {
				continue
			}
			if cyc := expand(in.ItemID, in.Quantity*batches, false, nextAncestors); len(cyc) > 0 {
				return cyc
			}
		}

		if surplus > 0 {
			virtualStock[itemID] += surplus
		}
		for _, out := range pat.Outputs {
			if out.ItemID != itemID && out.Quantity > 0 {
				virtualStock[out.ItemID] += out.Quantity * batches
			}
		}

		// Merge into existing leaf/intermediate step if same provider+pattern, else append in post-order.
		merged := false
		for _, existing := range steps {
			if existing.ProviderBlockID == prov.BlockID && existing.Pattern.ID == pat.ID && existing.PrimaryOutputItem == itemID {
				existing.TotalBatches += batches
				merged = true
				break
			}
		}
		if !merged {
			steps = append(steps, &JobStep{
				StepIndex:         len(steps),
				ProviderBlockID:   prov.BlockID,
				Pattern:           pat,
				PrimaryOutputItem: itemID,
				TotalBatches:      batches,
			})
		}
		return nil
	}

	if cycle := expand(req.ItemID, req.Quantity, true, nil); len(cycle) > 0 {
		var provBlocks []domain.PlacedBlock
		for _, p := range s.Providers {
			if p.NetworkID == targetNetID {
				provBlocks = append(provBlocks, p.Block)
			}
		}
		diags = append(diags, diagnostics.NewRecursiveCraftingPattern(simID, provBlocks, cycle))
		return failJob(targetNetID, diagnostics.CodeRecursiveCraftingPattern), events, diags
	}

	if len(missingRaw) > 0 {
		missingKeys := sortedMapKeys(missingRaw)
		var driveBlocks []domain.PlacedBlock
		if store != nil {
			for _, c := range store.Cells {
				if c.NetworkID == targetNetID {
					driveBlocks = append(driveBlocks, c.DriveBlock)
				}
			}
		}
		for _, itemID := range missingKeys {
			diags = append(diags, diagnostics.NewMissingIngredients(simID, driveBlocks, itemID, missingRaw[itemID]))
		}
		return failJob(targetNetID, diagnostics.CodeMissingIngredients), events, diags
	}

	// Calculate required CPU storage bytes and total batches.
	totalReservedItems := 0
	for _, q := range reservedFromStorage {
		totalReservedItems += q
	}
	totalProducedItems := 0
	totalBatches := 0
	for i, step := range steps {
		step.StepIndex = i
		totalBatches += step.TotalBatches
		for _, out := range step.Pattern.Outputs {
			totalProducedItems += out.Quantity * step.TotalBatches
		}
	}
	ipb := profile.Config.ItemsPerByte
	if ipb <= 0 {
		ipb = 8
	}
	requiredBytes := (totalReservedItems + totalProducedItems + ipb - 1) / ipb
	if totalBatches > requiredBytes {
		requiredBytes = totalBatches
	}
	if requiredBytes < 1 {
		requiredBytes = 1
	}

	// Find free online Crafting CPU on targetNetID.
	freeCPUs := make([]*CPUState, 0)
	for _, cpu := range s.CPUs {
		if cpu.NetworkID == targetNetID && cpu.Online && cpu.ActiveJobID == "" {
			freeCPUs = append(freeCPUs, cpu)
		}
	}
	if len(freeCPUs) == 0 {
		var affected []domain.PlacedBlock
		for _, net := range nets {
			if net.ID == targetNetID {
				for _, n := range net.Nodes {
					if n.IsCPUBlock {
						affected = append(affected, n.Block)
					}
				}
				if len(affected) == 0 {
					affected = net.Blocks()
				}
			}
		}
		diags = append(diags, diagnostics.NewNoValidCraftingCPU(
			simID,
			affected,
			domain.SeverityError,
			fmt.Sprintf("no free online Crafting CPU on %s to execute %d x %s", targetNetID, req.Quantity, req.ItemID),
		))
		return failJob(targetNetID, diagnostics.CodeNoValidCraftingCPU), events, diags
	}

	// Prefer smallest-fitting free CPU (tie-break by most co-processors, then ID).
	sort.Slice(freeCPUs, func(i, j int) bool {
		if freeCPUs[i].TotalBytes != freeCPUs[j].TotalBytes {
			return freeCPUs[i].TotalBytes < freeCPUs[j].TotalBytes
		}
		if freeCPUs[i].CoProcessors != freeCPUs[j].CoProcessors {
			return freeCPUs[i].CoProcessors > freeCPUs[j].CoProcessors
		}
		return freeCPUs[i].ID < freeCPUs[j].ID
	})

	var selectedCPU *CPUState
	for _, cpu := range freeCPUs {
		if cpu.TotalBytes >= requiredBytes {
			selectedCPU = cpu
			break
		}
	}
	if selectedCPU == nil {
		largest := freeCPUs[len(freeCPUs)-1]
		diags = append(diags, diagnostics.NewInsufficientCPUStorage(
			simID,
			largest.Blocks,
			requiredBytes,
			largest.TotalBytes,
		))
		return failJob(targetNetID, diagnostics.CodeInsufficientCPUStorage), events, diags
	}

	// Reserve base ingredients from ME Storage into the Crafting CPU's internal item pool!
	itemPool := make(map[string]int, len(reservedFromStorage))
	selectedCPU.ActiveJobID = jobID

	events = append(events, emitter.Emit(0, "job_created", map[string]any{
		"job_id":        jobID,
		"network_id":    targetNetID,
		"cpu_id":        selectedCPU.ID,
		"item_id":       req.ItemID,
		"quantity":      req.Quantity,
		"bytes_used":    requiredBytes,
		"total_batches": totalBatches,
	}))

	for _, itemID := range sortedMapKeys(reservedFromStorage) {
		qty := reservedFromStorage[itemID]
		if qty <= 0 {
			continue
		}
		extracted := store.ExtractFromNetwork(targetNetID, itemID, qty)
		itemPool[itemID] += extracted
		events = append(events, emitter.Emit(0, "ingredient_reserved", map[string]any{
			"job_id":   jobID,
			"cpu_id":   selectedCPU.ID,
			"item_id":  itemID,
			"quantity": extracted,
		}))
	}

	job := &JobState{
		ID:             jobID,
		NetworkID:      targetNetID,
		CPUID:          selectedCPU.ID,
		TargetItem:     req.ItemID,
		TargetQuantity: req.Quantity,
		Status:         "running",
		CoProcessors:   selectedCPU.CoProcessors,
		BytesUsed:      requiredBytes,
		BytesCapacity:  selectedCPU.TotalBytes,
		TotalBatches:   totalBatches,
		ItemPool:       itemPool,
		Steps:          steps,
	}
	return job, events, diags
}

// SyncOnlineStatus updates Provider and CPU Online flags from current network energy and channel states.
func (s *State) SyncOnlineStatus(energyStates []*energy.NetworkEnergy, hasChannel map[string]bool) {
	if s == nil {
		return
	}
	netOnline := make(map[string]bool, len(energyStates))
	for _, es := range energyStates {
		netOnline[es.NetworkID] = es.Online
	}
	for _, p := range s.Providers {
		if hasChannel != nil {
			p.HasChannel = hasChannel[p.BlockID]
		}
		p.Online = netOnline[p.NetworkID] && p.HasChannel
	}
	for _, cpu := range s.CPUs {
		if hasChannel != nil && len(cpu.Blocks) > 0 {
			cpu.HasChannel = hasChannel[cpu.Blocks[0].ID]
		}
		cpu.Online = netOnline[cpu.NetworkID] && cpu.HasChannel
	}
}

// ActiveAssemblersByNetwork returns the count of currently running Molecular Assemblers per ME network.
func (s *State) ActiveAssemblersByNetwork() map[string]int {
	counts := make(map[string]int)
	if s == nil {
		return counts
	}
	for _, m := range s.Machines {
		if m.MachineType == "molecular_assembler" && m.ActiveTask != nil {
			netID := m.NetworkID
			if netID == "" {
				netID = m.ActiveTask.NetworkID
			}
			if netID != "" {
				counts[netID]++
			}
		}
	}
	return counts
}

// StepTick advances running machines, returns completed craft outputs, completes finished jobs,
// and dispatches ready pattern batches (up to 1 + CoProcessors per CPU).
func (s *State) StepTick(
	simID string,
	tick int,
	energyStates []*energy.NetworkEnergy,
	store *storage.State,
	emitter EventEmitter,
) ([]domain.SimulationEvent, []domain.Diagnostic) {
	if s == nil {
		return nil, nil
	}

	netOnline := make(map[string]bool, len(energyStates))
	for _, es := range energyStates {
		netOnline[es.NetworkID] = es.Online
	}

	events := make([]domain.SimulationEvent, 0)
	diags := make([]domain.Diagnostic, 0)

	jobsByID := make(map[string]*JobState, len(s.Jobs))
	for _, j := range s.Jobs {
		jobsByID[j.ID] = j
	}
	providersByID := make(map[string]*ProviderState, len(s.Providers))
	for _, p := range s.Providers {
		providersByID[p.BlockID] = p
	}
	cpusByID := make(map[string]*CPUState, len(s.CPUs))
	for _, c := range s.CPUs {
		cpusByID[c.ID] = c
	}

	// Phase A: Advance active Molecular Assemblers and External Machines.
	for _, m := range s.Machines {
		if m.ActiveTask == nil {
			continue
		}
		task := m.ActiveTask
		if !netOnline[task.NetworkID] {
			// Network brownout stalls active crafting progress/return on that network.
			continue
		}

		task.RemainingTicks--
		if task.RemainingTicks > 0 {
			continue
		}

		// Craft completed!
		events = append(events, emitter.Emit(tick, "craft_completed", map[string]any{
			"job_id":            task.JobID,
			"machine_block_id":  m.BlockID,
			"provider_block_id": task.ProviderBlockID,
			"pattern_id":        task.PatternID,
			"outputs":           task.Outputs,
		}))

		job := jobsByID[task.JobID]
		prov := providersByID[task.ProviderBlockID]

		if m.AutoReturn {
			events = append(events, emitter.Emit(tick, "output_returned", map[string]any{
				"job_id":            task.JobID,
				"provider_block_id": task.ProviderBlockID,
				"machine_block_id":  m.BlockID,
				"outputs":           task.Outputs,
			}))

			if prov != nil && prov.LockMode == "until_result_returned" && prov.Locked && prov.LockedByJobID == task.JobID {
				prov.Locked = false
				prov.LockedByJobID = ""
			}

			if job != nil && job.Status == "running" {
				for _, out := range task.Outputs {
					if out.Quantity > 0 {
						job.ItemPool[out.ItemID] += out.Quantity
					}
				}
				if task.StepIndex >= 0 && task.StepIndex < len(job.Steps) {
					stStep := job.Steps[task.StepIndex]
					if stStep.InFlightBatches > 0 {
						stStep.InFlightBatches--
					}
					stStep.CompletedBatches++
				}
				job.CompletedCrafts++

				if isJobAllStepsComplete(job) {
					// Insert all produced items remaining in job.ItemPool into ME Storage!
					for _, itemID := range sortedMapKeys(job.ItemPool) {
						qty := job.ItemPool[itemID]
						if qty <= 0 {
							continue
						}
						_, insDiags := store.InsertInNetwork(simID, job.NetworkID, itemID, qty)
						diags = append(diags, insDiags...)
						delete(job.ItemPool, itemID)
					}
					job.Status = "completed"
					if cpu := cpusByID[job.CPUID]; cpu != nil && cpu.ActiveJobID == job.ID {
						cpu.ActiveJobID = ""
					}
					events = append(events, emitter.Emit(tick, "job_completed", map[string]any{
						"job_id":   job.ID,
						"cpu_id":   job.CPUID,
						"item_id":  job.TargetItem,
						"quantity": job.TargetQuantity,
					}))
				}
			}
		}

		// Promote next queued task on External Machine (if any).
		if len(m.QueuedTasks) > 0 {
			nextTask := m.QueuedTasks[0]
			m.QueuedTasks = m.QueuedTasks[1:]
			m.ActiveTask = nextTask
			events = append(events, emitter.Emit(tick, "assembler_started", map[string]any{
				"job_id":           nextTask.JobID,
				"machine_block_id": m.BlockID,
				"machine_type":     m.Block.BlockDefID,
				"duration_ticks":   nextTask.TotalTicks,
				"outputs":          nextTask.Outputs,
			}))
		} else {
			m.ActiveTask = nil
		}
	}

	// Build coordinate index of machines for O(1) adjacency checks.
	machineByCoord := make(map[domain.Coord3D]*MachineState, len(s.Machines))
	for _, m := range s.Machines {
		machineByCoord[m.Block.Coord()] = m
	}

	// Phase B: Dispatch up to (1 + CoProcessors) ready batches per running Crafting CPU job.
	for _, job := range s.Jobs {
		if job.Status != "running" {
			continue
		}
		if !netOnline[job.NetworkID] {
			continue
		}
		cpu := cpusByID[job.CPUID]
		if cpu == nil || !cpu.Online {
			continue
		}

		maxDispatches := 1 + job.CoProcessors
		if maxDispatches < 1 {
			maxDispatches = 1
		}

		dispatched := 0
		for dispatched < maxDispatches {
			advancedThisPass := false

			for _, step := range job.Steps {
				if dispatched >= maxDispatches || job.Status != "running" {
					break
				}
				if step.DispatchedBatches >= step.TotalBatches {
					continue
				}
				if !hasBatchInputs(job.ItemPool, step.Pattern.Inputs) {
					continue
				}

				prov := s.resolveStepProvider(job.NetworkID, step)
				if prov == nil || !prov.Online {
					continue
				}

				adjMachines := findValidAdjacentMachines(prov, step.Pattern.PatternType, machineByCoord)
				if len(adjMachines) == 0 {
					diags = append(diags, diagnostics.NewNoAdjacentCraftingInventory(simID, prov.Block, step.Pattern.PatternType))
					s.abortRunningJob(simID, tick, job, cpu, store, diagnostics.CodeNoAdjacentCraftingInventory, emitter, &events, &diags)
					break
				}

				if prov.LockMode == "until_result_returned" && prov.Locked {
					continue
				}

				targetMachine, chosenIdx, allBusy := selectTargetMachine(prov, adjMachines)
				if targetMachine == nil {
					if prov.BlockingMode && allBusy {
						diags = append(diags, diagnostics.NewProviderBlocked(simID, prov.Block, step.PrimaryOutputItem))
					}
					continue
				}

				// Consume 1 batch of inputs from job.ItemPool and dispatch!
				consumeBatchInputs(job.ItemPool, step.Pattern.Inputs)
				step.DispatchedBatches++
				step.InFlightBatches++
				dispatched++
				advancedThisPass = true

				prov.RoundRobinIndex = (chosenIdx + 1) % len(adjMachines)
				if prov.LockMode == "until_result_returned" {
					prov.Locked = true
					prov.LockedByJobID = job.ID
				}

				outputsCopy := make([]domain.ItemStack, len(step.Pattern.Outputs))
				copy(outputsCopy, step.Pattern.Outputs)

				task := &MachineTask{
					JobID:           job.ID,
					StepIndex:       step.StepIndex,
					NetworkID:       job.NetworkID,
					ProviderBlockID: prov.BlockID,
					MachineBlockID:  targetMachine.BlockID,
					PatternID:       step.Pattern.ID,
					PatternType:     step.Pattern.PatternType,
					Outputs:         outputsCopy,
					RemainingTicks:  targetMachine.ProcessTicks,
					TotalTicks:      targetMachine.ProcessTicks,
				}

				events = append(events, emitter.Emit(tick, "provider_dispatched", map[string]any{
					"job_id":            job.ID,
					"cpu_id":            job.CPUID,
					"provider_block_id": prov.BlockID,
					"machine_block_id":  targetMachine.BlockID,
					"pattern_id":        step.Pattern.ID,
					"pattern_type":      step.Pattern.PatternType,
					"inputs":            step.Pattern.Inputs,
					"outputs":           outputsCopy,
				}))

				if targetMachine.ActiveTask == nil {
					targetMachine.ActiveTask = task
					events = append(events, emitter.Emit(tick, "assembler_started", map[string]any{
						"job_id":           job.ID,
						"machine_block_id": targetMachine.BlockID,
						"machine_type":     targetMachine.Block.BlockDefID,
						"duration_ticks":   task.TotalTicks,
						"outputs":          outputsCopy,
					}))
				} else {
					targetMachine.QueuedTasks = append(targetMachine.QueuedTasks, task)
				}

				// Break inner step scan so round-robin / multi-batch of earliest ready step can continue up to maxDispatches.
				break
			}

			if !advancedThisPass || job.Status != "running" {
				break
			}
		}
	}

	return events, diagnostics.DeduplicateAndSort(diags)
}

func (s *State) abortRunningJob(
	simID string,
	tick int,
	job *JobState,
	cpu *CPUState,
	store *storage.State,
	reason string,
	emitter EventEmitter,
	events *[]domain.SimulationEvent,
	diags *[]domain.Diagnostic,
) {
	job.Status = "failed"
	job.FailureReason = reason
	if cpu != nil && cpu.ActiveJobID == job.ID {
		cpu.ActiveJobID = ""
	}
	// Refund any unconsumed items in job.ItemPool back to ME Storage.
	if store != nil {
		for _, itemID := range sortedMapKeys(job.ItemPool) {
			qty := job.ItemPool[itemID]
			if qty > 0 {
				_, insDiags := store.InsertInNetwork(simID, job.NetworkID, itemID, qty)
				*diags = append(*diags, insDiags...)
			}
			delete(job.ItemPool, itemID)
		}
	}
	*events = append(*events, emitter.Emit(tick, "job_failed", map[string]any{
		"job_id":   job.ID,
		"cpu_id":   job.CPUID,
		"item_id":  job.TargetItem,
		"quantity": job.TargetQuantity,
		"reason":   reason,
	}))
}

func (s *State) findPatternForItem(networkID, itemID string) (*ProviderState, domain.PatternSpec, bool) {
	for _, p := range s.Providers {
		if !p.Online {
			continue
		}
		if networkID != "" && p.NetworkID != networkID {
			continue
		}
		for _, pat := range p.Patterns {
			if outputQuantityOf(pat, itemID) > 0 {
				return p, pat, true
			}
		}
	}
	return nil, domain.PatternSpec{}, false
}

func (s *State) resolveStepProvider(networkID string, step *JobStep) *ProviderState {
	for _, p := range s.Providers {
		if p.BlockID == step.ProviderBlockID && p.Online {
			return p
		}
	}
	for _, p := range s.Providers {
		if p.NetworkID == networkID && p.Online {
			for _, pat := range p.Patterns {
				if pat.ID == step.Pattern.ID || outputQuantityOf(pat, step.PrimaryOutputItem) > 0 {
					return p
				}
			}
		}
	}
	return nil
}

func findValidAdjacentMachines(
	prov *ProviderState,
	patternType string,
	machineByCoord map[domain.Coord3D]*MachineState,
) []*MachineState {
	wantType := "molecular_assembler"
	if strings.EqualFold(patternType, "processing") {
		wantType = "external_machine"
	}

	var dirs []domain.Direction
	if prov.Variant == "directional" {
		facing := prov.Block.Facing
		if !facing.IsValid() {
			facing = domain.North
		}
		dirs = []domain.Direction{facing}
	} else {
		dirs = domain.AllDirections()
	}

	out := make([]*MachineState, 0, len(dirs))
	for _, dir := range dirs {
		adj := prov.Block.Coord().Adjacent(dir)
		if m, ok := machineByCoord[adj]; ok && m.MachineType == wantType {
			out = append(out, m)
		}
	}
	return out
}

func selectTargetMachine(prov *ProviderState, adj []*MachineState) (*MachineState, int, bool) {
	n := len(adj)
	if n == 0 {
		return nil, 0, false
	}
	start := prov.RoundRobinIndex % n
	if start < 0 {
		start = 0
	}

	// First pass: look for an idle machine starting from RoundRobinIndex.
	for offset := range n {
		idx := (start + offset) % n
		m := adj[idx]
		if m.ActiveTask == nil && len(m.QueuedTasks) == 0 {
			return m, idx, false
		}
	}

	// All adjacent machines are currently busy!
	if prov.BlockingMode {
		return nil, 0, true
	}

	// When blocking_mode == false:
	// - External machines have an input inventory buffer and can accept queued batches round-robin.
	// - Molecular Assemblers hold a single craft at a time and wait until free.
	if adj[start].MachineType == "external_machine" {
		return adj[start], start, true
	}
	return nil, 0, true
}

// JobViews converts internal JobStates into domain.CraftingJobViews for UI overlays.
func (s *State) JobViews() []domain.CraftingJobView {
	if s == nil || len(s.Jobs) == 0 {
		return []domain.CraftingJobView{}
	}
	views := make([]domain.CraftingJobView, 0, len(s.Jobs))
	for _, j := range s.Jobs {
		var pending []domain.ItemStack
		if j.Status == "running" {
			pendMap := make(map[string]int)
			for _, st := range j.Steps {
				remBatches := st.TotalBatches - st.CompletedBatches
				if remBatches <= 0 {
					continue
				}
				for _, out := range st.Pattern.Outputs {
					if out.Quantity > 0 {
						pendMap[out.ItemID] += out.Quantity * remBatches
					}
				}
			}
			for _, id := range sortedMapKeys(pendMap) {
				pending = append(pending, domain.ItemStack{ItemID: id, Quantity: pendMap[id]})
			}
		}
		views = append(views, domain.CraftingJobView{
			ID:              j.ID,
			NetworkID:       j.NetworkID,
			CPUID:           j.CPUID,
			TargetItem:      j.TargetItem,
			TargetQuantity:  j.TargetQuantity,
			Status:          j.Status,
			FailureReason:   j.FailureReason,
			CoProcessors:    j.CoProcessors,
			BytesUsed:       j.BytesUsed,
			BytesCapacity:   j.BytesCapacity,
			TotalBatches:    j.TotalBatches,
			CompletedCrafts: j.CompletedCrafts,
			PendingItems:    pending,
		})
	}
	return views
}

// SummaryCounts returns (activeJobs, completedJobs, failedJobs).
func (s *State) SummaryCounts() (int, int, int) {
	if s == nil {
		return 0, 0, 0
	}
	active, completed, failed := 0, 0, 0
	for _, j := range s.Jobs {
		switch j.Status {
		case "running":
			active++
		case "completed":
			completed++
		case "failed":
			failed++
		}
	}
	return active, completed, failed
}

func isJobAllStepsComplete(job *JobState) bool {
	if len(job.Steps) == 0 {
		return false
	}
	for _, st := range job.Steps {
		if st.CompletedBatches < st.TotalBatches || st.InFlightBatches > 0 {
			return false
		}
	}
	return true
}

func hasBatchInputs(pool map[string]int, inputs []domain.ItemStack) bool {
	for _, in := range inputs {
		if in.Quantity <= 0 {
			continue
		}
		if pool[in.ItemID] < in.Quantity {
			return false
		}
	}
	return true
}

func consumeBatchInputs(pool map[string]int, inputs []domain.ItemStack) {
	for _, in := range inputs {
		if in.Quantity <= 0 {
			continue
		}
		pool[in.ItemID] -= in.Quantity
		if pool[in.ItemID] <= 0 {
			delete(pool, in.ItemID)
		}
	}
}

func outputQuantityOf(pat domain.PatternSpec, itemID string) int {
	qty := 0
	for _, out := range pat.Outputs {
		if out.ItemID == itemID && out.Quantity > 0 {
			qty += out.Quantity
		}
	}
	return qty
}

func parseProviderPatterns(cfg map[string]any) []domain.PatternSpec {
	if cfg == nil {
		return nil
	}
	raw, ok := cfg["patterns"]
	if !ok || raw == nil {
		return nil
	}
	b, err := json.Marshal(raw)
	if err != nil {
		return nil
	}
	var specs []domain.PatternSpec
	if err := json.Unmarshal(b, &specs); err != nil {
		return nil
	}
	out := make([]domain.PatternSpec, 0, len(specs))
	for idx, sp := range specs {
		norm := normalizePattern(sp, idx+1)
		if len(norm.Outputs) > 0 {
			out = append(out, norm)
		}
	}
	return out
}

func normalizePattern(sp domain.PatternSpec, index int) domain.PatternSpec {
	if strings.TrimSpace(sp.ID) == "" {
		sp.ID = fmt.Sprintf("pat-%d", index)
	}
	pt := strings.ToLower(strings.TrimSpace(sp.PatternType))
	if pt != "processing" {
		pt = "crafting"
	}
	sp.PatternType = pt
	sp.Inputs = consolidateStacks(sp.Inputs)
	sp.Outputs = consolidateStacks(sp.Outputs)
	return sp
}

func consolidateStacks(stacks []domain.ItemStack) []domain.ItemStack {
	if len(stacks) == 0 {
		return nil
	}
	order := make([]string, 0, len(stacks))
	sums := make(map[string]int, len(stacks))
	for _, st := range stacks {
		id := strings.TrimSpace(st.ItemID)
		if id == "" || st.Quantity <= 0 {
			continue
		}
		if _, seen := sums[id]; !seen {
			order = append(order, id)
		}
		sums[id] += st.Quantity
	}
	out := make([]domain.ItemStack, 0, len(order))
	for _, id := range order {
		out = append(out, domain.ItemStack{ItemID: id, Quantity: sums[id]})
	}
	return out
}

func copyIntMap(src map[string]int) map[string]int {
	dst := make(map[string]int, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func sortedMapKeys(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
