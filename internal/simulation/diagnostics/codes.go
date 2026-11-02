package diagnostics

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"github.com/felipinho/me-lab/internal/domain"
)

// Stable AE2 diagnostic codes emitted by the simulation engine.
const (
	CodeChannelCapacityExceeded         = "CHANNEL_CAPACITY_EXCEEDED"
	CodeDeviceNoChannel                 = "DEVICE_NO_CHANNEL"
	CodeNetworkNoController             = "NETWORK_NO_CONTROLLER"
	CodeMultipleControllersDisconnected = "MULTIPLE_CONTROLLERS_DISCONNECTED"
	CodeInvalidControllerMultiblock     = "INVALID_CONTROLLER_MULTIBLOCK"
	CodeInvalidCraftingCPUMultiblock    = "INVALID_CRAFTING_CPU_MULTIBLOCK"
	CodeBlockDisconnected               = "BLOCK_DISCONNECTED"
	CodeInsufficientEnergy              = "INSUFFICIENT_ENERGY"
	CodeStorageFull                     = "STORAGE_FULL"
	CodeMissingCraftingPattern          = "MISSING_CRAFTING_PATTERN"
	CodeRecursiveCraftingPattern        = "RECURSIVE_CRAFTING_PATTERN"
	CodeMissingIngredients              = "MISSING_INGREDIENTS"
	CodeInsufficientCPUStorage          = "INSUFFICIENT_CPU_STORAGE"
	CodeNoValidCraftingCPU              = "NO_VALID_CRAFTING_CPU"
	CodeNoAdjacentCraftingInventory     = "NO_ADJACENT_CRAFTING_INVENTORY"
	CodeProviderBlocked                 = "PROVIDER_BLOCKED"
	CodeUnsupportedMechanic             = "UNSUPPORTED_MECHANIC"
)

// AffectedFromBlock converts a PlacedBlock into an AffectedBlock reference.
func AffectedFromBlock(b domain.PlacedBlock) domain.AffectedBlock {
	return domain.AffectedBlock{
		ID: b.ID,
		X:  b.X,
		Y:  b.Y,
		Z:  b.Z,
	}
}

// AffectedFromBlocks converts a slice of PlacedBlocks into deterministically sorted,
// deduplicated AffectedBlock references.
func AffectedFromBlocks(blocks []domain.PlacedBlock) []domain.AffectedBlock {
	if len(blocks) == 0 {
		return []domain.AffectedBlock{}
	}
	seen := make(map[string]struct{}, len(blocks))
	out := make([]domain.AffectedBlock, 0, len(blocks))
	for _, b := range blocks {
		key := fmt.Sprintf("%s@%d,%d,%d", b.ID, b.X, b.Y, b.Z)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, AffectedFromBlock(b))
	}
	sortAffectedBlocks(out)
	return out
}

func sortAffectedBlocks(affected []domain.AffectedBlock) {
	sort.Slice(affected, func(i, j int) bool {
		ci := domain.Coord3D{X: affected[i].X, Y: affected[i].Y, Z: affected[i].Z}
		cj := domain.Coord3D{X: affected[j].X, Y: affected[j].Y, Z: affected[j].Z}
		if cmp := domain.CompareCoords(ci, cj); cmp != 0 {
			return cmp < 0
		}
		return affected[i].ID < affected[j].ID
	})
}

// DeterministicID computes a stable diagnostic ID scoped to simulationID (when non-empty).
func DeterministicID(simulationID, code, message string, affected []domain.AffectedBlock) string {
	h := sha256.New()
	h.Write([]byte(simulationID))
	h.Write([]byte{0})
	h.Write([]byte(code))
	h.Write([]byte{0})
	h.Write([]byte(message))
	h.Write([]byte{0})
	for _, a := range affected {
		fmt.Fprintf(h, "%s:%d,%d,%d;", a.ID, a.X, a.Y, a.Z)
	}
	digest := hex.EncodeToString(h.Sum(nil))[:16]
	if simulationID != "" {
		return fmt.Sprintf("%s:diag:%s", simulationID, digest)
	}
	return fmt.Sprintf("diag:%s", digest)
}

// New constructs a domain.Diagnostic with sorted AffectedBlocks and a deterministic ID.
func New(
	simulationID string,
	code string,
	severity domain.DiagnosticSeverity,
	message string,
	blocks []domain.PlacedBlock,
	remediation string,
	blocksSimulation bool,
) domain.Diagnostic {
	affected := AffectedFromBlocks(blocks)
	return domain.Diagnostic{
		ID:               DeterministicID(simulationID, code, message, affected),
		SimulationID:     simulationID,
		Code:             code,
		Severity:         severity,
		Message:          message,
		AffectedBlocks:   affected,
		Remediation:      remediation,
		BlocksSimulation: blocksSimulation,
	}
}

func NewChannelCapacityExceeded(simID string, bottleneckBlocks []domain.PlacedBlock, used, capacity int) domain.Diagnostic {
	msg := fmt.Sprintf("Channel bottleneck reached (%d/%d channels in use); downstream devices cannot acquire channels through saturated path.", used, capacity)
	rem := "Upgrade the saturated branch to Dense Cable (32 channels) or route branches to separate ME Controller faces."
	return New(simID, CodeChannelCapacityExceeded, domain.SeverityError, msg, bottleneckBlocks, rem, false)
}

func NewDeviceNoChannel(simID string, block domain.PlacedBlock, reason string) domain.Diagnostic {
	msg := fmt.Sprintf("Device %s at (%d, %d, %d) could not acquire an ME channel", blockLabel(block), block.X, block.Y, block.Z)
	if reason != "" {
		msg += ": " + reason
	} else {
		msg += "."
	}
	rem := "Ensure a valid path with free channel capacity exists to an ME Controller, or keep ad-hoc networks within the channel limit."
	return New(simID, CodeDeviceNoChannel, domain.SeverityError, msg, []domain.PlacedBlock{block}, rem, false)
}

func NewNetworkNoController(simID string, networkID string, blocks []domain.PlacedBlock, used, limit int) domain.Diagnostic {
	msg := fmt.Sprintf("Ad-hoc network %s requires %d channels, exceeding the %d-channel controllerless limit.", networkID, used, limit)
	rem := "Add an ME Controller to this network or split devices across isolated subnetworks using Cable Anchors."
	return New(simID, CodeNetworkNoController, domain.SeverityError, msg, blocks, rem, true)
}

func NewMultipleControllersDisconnected(simID string, controllers []domain.PlacedBlock) domain.Diagnostic {
	msg := fmt.Sprintf("Multiple disconnected ME Controller clusters (%d controller blocks total) are wired to the same ME network.", len(controllers))
	rem := "Merge all ME Controller blocks into one contiguous multiblock, or separate the subnetworks using Cable Anchors."
	return New(simID, CodeMultipleControllersDisconnected, domain.SeverityError, msg, controllers, rem, true)
}

func NewInvalidControllerMultiblock(simID string, controllers []domain.PlacedBlock, reason string) domain.Diagnostic {
	msg := fmt.Sprintf("Invalid ME Controller multiblock: %s.", reason)
	rem := "Keep all ME Controller blocks in a single connected multiblock within a 7x7x7 bounding box, with at most 1 opposite-neighbor axis per controller block."
	return New(simID, CodeInvalidControllerMultiblock, domain.SeverityError, msg, controllers, rem, true)
}

func NewInvalidCraftingCPUMultiblock(simID string, blocks []domain.PlacedBlock, reason string) domain.Diagnostic {
	msg := fmt.Sprintf("Invalid Crafting CPU multiblock: %s.", reason)
	rem := "Arrange Crafting Unit, Crafting Storage, Co-Processor, and Monitor blocks into a solid, gap-free rectangular prism."
	return New(simID, CodeInvalidCraftingCPUMultiblock, domain.SeverityError, msg, blocks, rem, false)
}

func NewBlockDisconnected(simID string, block domain.PlacedBlock) domain.Diagnostic {
	msg := fmt.Sprintf("ME block %s at (%d, %d, %d) has no connected ME neighbors.", blockLabel(block), block.X, block.Y, block.Z)
	rem := "Connect this block to an ME Cable or compatible adjacent ME device, checking cable colors, anchors, and directional faces."
	return New(simID, CodeBlockDisconnected, domain.SeverityWarning, msg, []domain.PlacedBlock{block}, rem, false)
}

func NewInsufficientEnergy(simID string, networkID string, blocks []domain.PlacedBlock, available, required float64) domain.Diagnostic {
	msg := fmt.Sprintf("Network %s has insufficient energy (%.2f AE available vs %.2f AE/t required); network is offline.", networkID, available, required)
	rem := "Connect an ME Energy Acceptor supplied with FE/t, add charged Energy Cells, or reduce active device load."
	return New(simID, CodeInsufficientEnergy, domain.SeverityError, msg, blocks, rem, true)
}

func NewStorageFull(simID string, drives []domain.PlacedBlock, itemID string, unstoredQty int) domain.Diagnostic {
	msg := fmt.Sprintf("ME Storage could not store %d x %s; all eligible online storage cells are full or restricted by partition.", unstoredQty, itemID)
	rem := "Add higher-tier ME Storage Cells to an online ME Drive, free type slots (max 63 types/cell), or adjust cell partitions."
	return New(simID, CodeStorageFull, domain.SeverityError, msg, drives, rem, false)
}

func NewMissingCraftingPattern(simID string, providers []domain.PlacedBlock, itemID string) domain.Diagnostic {
	msg := fmt.Sprintf("No online Pattern Provider encodes a crafting or processing pattern producing %s.", itemID)
	rem := "Encode a pattern outputting the requested item inside an online ME Pattern Provider."
	return New(simID, CodeMissingCraftingPattern, domain.SeverityError, msg, providers, rem, false)
}

func NewRecursiveCraftingPattern(simID string, providers []domain.PlacedBlock, cycle []string) domain.Diagnostic {
	msg := fmt.Sprintf("Recursive crafting dependency cycle detected: %s.", strings.Join(cycle, " -> "))
	rem := "Break the circular pattern dependency or stock one of the intermediate items directly in ME Storage."
	return New(simID, CodeRecursiveCraftingPattern, domain.SeverityError, msg, providers, rem, false)
}

func NewMissingIngredients(simID string, drives []domain.PlacedBlock, itemID string, missingQty int) domain.Diagnostic {
	msg := fmt.Sprintf("Missing %d x %s in online ME Storage to satisfy crafting request.", missingQty, itemID)
	rem := "Insert the required raw ingredients into an online ME Drive cell or add a Pattern Provider recipe producing them."
	return New(simID, CodeMissingIngredients, domain.SeverityError, msg, drives, rem, false)
}

func NewInsufficientCPUStorage(simID string, cpuBlocks []domain.PlacedBlock, requiredBytes, maxAvailableBytes int) domain.Diagnostic {
	msg := fmt.Sprintf("Crafting job requires %d bytes of CPU storage, exceeding maximum available free Crafting CPU capacity (%d bytes).", requiredBytes, maxAvailableBytes)
	rem := "Add larger Crafting Storage blocks (4k, 16k, 64k, 256k) to the Crafting CPU multiblock."
	return New(simID, CodeInsufficientCPUStorage, domain.SeverityError, msg, cpuBlocks, rem, false)
}

func NewNoValidCraftingCPU(simID string, blocks []domain.PlacedBlock, severity domain.DiagnosticSeverity, reason string) domain.Diagnostic {
	msg := fmt.Sprintf("No usable Crafting CPU available: %s.", reason)
	rem := "Build a solid rectangular-prism Crafting CPU with at least one Crafting Storage block on an online ME channel."
	return New(simID, CodeNoValidCraftingCPU, severity, msg, blocks, rem, false)
}

func NewNoAdjacentCraftingInventory(simID string, provider domain.PlacedBlock, patternType string) domain.Diagnostic {
	targetDesc := "ME Molecular Assembler"
	if patternType == "processing" {
		targetDesc = "External Processing Machine"
	}
	msg := fmt.Sprintf("Pattern Provider %s at (%d, %d, %d) has no adjacent %s on its valid output face(s) for %s pattern.", blockLabel(provider), provider.X, provider.Y, provider.Z, targetDesc, patternType)
	rem := fmt.Sprintf("Place an %s adjacent to the Pattern Provider (on its facing side if directional, or any side if normal).", targetDesc)
	return New(simID, CodeNoAdjacentCraftingInventory, domain.SeverityError, msg, []domain.PlacedBlock{provider}, rem, false)
}

func NewProviderBlocked(simID string, provider domain.PlacedBlock, targetItem string) domain.Diagnostic {
	msg := fmt.Sprintf("Pattern Provider %s at (%d, %d, %d) has blocking_mode enabled and all adjacent machines are busy; delaying %s batch.", blockLabel(provider), provider.X, provider.Y, provider.Z, targetItem)
	rem := "Add additional adjacent machines around the Pattern Provider for parallel round-robin dispatch or wait for current crafts to finish."
	return New(simID, CodeProviderBlocked, domain.SeverityWarning, msg, []domain.PlacedBlock{provider}, rem, false)
}

func NewUnsupportedMechanic(simID string, block domain.PlacedBlock, blockName string) domain.Diagnostic {
	name := blockName
	if name == "" {
		name = block.BlockDefID
	}
	msg := fmt.Sprintf("Block %s (%s) at (%d, %d, %d) uses an AE2 mechanic marked unsupported in this compatibility profile.", name, block.BlockDefID, block.X, block.Y, block.Z)
	rem := "Remove or replace this post-MVP block; its mechanic is not simulated and will be ignored during simulation ticks."
	return New(simID, CodeUnsupportedMechanic, domain.SeverityWarning, msg, []domain.PlacedBlock{block}, rem, false)
}

// DeduplicateAndSort deduplicates diagnostics by ID and returns them in deterministic order.
func DeduplicateAndSort(diags []domain.Diagnostic) []domain.Diagnostic {
	if len(diags) == 0 {
		return []domain.Diagnostic{}
	}
	seen := make(map[string]struct{}, len(diags))
	out := make([]domain.Diagnostic, 0, len(diags))
	for _, d := range diags {
		if d.AffectedBlocks == nil {
			d.AffectedBlocks = []domain.AffectedBlock{}
		}
		if _, ok := seen[d.ID]; ok {
			continue
		}
		seen[d.ID] = struct{}{}
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool {
		ri, rj := severityRank(out[i].Severity), severityRank(out[j].Severity)
		if ri != rj {
			return ri < rj
		}
		if out[i].Code != out[j].Code {
			return out[i].Code < out[j].Code
		}
		if len(out[i].AffectedBlocks) > 0 && len(out[j].AffectedBlocks) > 0 {
			ci := domain.Coord3D{X: out[i].AffectedBlocks[0].X, Y: out[i].AffectedBlocks[0].Y, Z: out[i].AffectedBlocks[0].Z}
			cj := domain.Coord3D{X: out[j].AffectedBlocks[0].X, Y: out[j].AffectedBlocks[0].Y, Z: out[j].AffectedBlocks[0].Z}
			if cmp := domain.CompareCoords(ci, cj); cmp != 0 {
				return cmp < 0
			}
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func severityRank(s domain.DiagnosticSeverity) int {
	switch s {
	case domain.SeverityError:
		return 0
	case domain.SeverityWarning:
		return 1
	default:
		return 2
	}
}

func blockLabel(b domain.PlacedBlock) string {
	if b.Label != "" {
		return b.Label
	}
	if b.BlockDefID != "" {
		return b.BlockDefID
	}
	return b.ID
}
