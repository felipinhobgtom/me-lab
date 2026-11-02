package domain

import "time"

// Fidelity indicates how faithfully ME Lab models a given AE2 mechanic.
type Fidelity string

const (
	FidelityExact        Fidelity = "exact"
	FidelityApproximated Fidelity = "approximated"
	FidelityUnsupported  Fidelity = "unsupported"
)

// User represents a registered ME Lab user account.
type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// CellTierSpec defines AE2 byte and type storage parameters for a cell tier.
type CellTierSpec struct {
	TotalBytes   int `json:"total_bytes"`
	MaxTypes     int `json:"max_types"`
	BytesPerType int `json:"bytes_per_type"`
}

// ProfileConfig holds data-driven AE2 version mechanics for a compatibility profile.
type ProfileConfig struct {
	NormalCableChannels       int                     `json:"normal_cable_channels"`
	DenseCableChannels        int                     `json:"dense_cable_channels"`
	ControllerFaceChannels    int                     `json:"controller_face_channels"`
	AdHocChannelLimit         int                     `json:"ad_hoc_channel_limit"`
	InfiniteChannels          bool                    `json:"infinite_channels"`
	ControllerMaxSpan         int                     `json:"controller_max_span"`
	ControllerEnergyPerBlock  float64                 `json:"controller_energy_per_block"`
	ControllerEnergyBuffer    float64                 `json:"controller_energy_buffer"`
	NodeBaseEnergyBuffer      float64                 `json:"node_base_energy_buffer"`
	ChannelHopEnergyCost      float64                 `json:"channel_hop_energy_cost"`
	FEPerAE                   float64                 `json:"fe_per_ae"`
	MolecularAssemblerTicks   int                     `json:"molecular_assembler_ticks"`
	MolecularAssemblerActiveE float64                 `json:"molecular_assembler_active_ae"`
	ExternalMachineTicks      int                     `json:"external_machine_ticks"`
	ItemsPerByte              int                     `json:"items_per_byte"`
	MaxCellTypes              int                     `json:"max_cell_types"`
	CellTiers                 map[string]CellTierSpec `json:"cell_tiers"`
	References                []string                `json:"references"`
}

// CompatibilityProfile binds a project to a specific Minecraft/AE2 ruleset.
type CompatibilityProfile struct {
	ID               string        `json:"id"`
	Name             string        `json:"name"`
	MinecraftVersion string        `json:"minecraft_version"`
	AE2Version       string        `json:"ae2_version"`
	ChannelMode      string        `json:"channel_mode"`
	Config           ProfileConfig `json:"config"`
}

// ConnectionRules describes how a block connects on its 6 faces.
type ConnectionRules struct {
	ConnectAllFaces      bool     `json:"connect_all_faces"`
	ConnectOnlyFacing    bool     `json:"connect_only_facing"`
	DisconnectPushFace   bool     `json:"disconnect_push_face"`
	SupportsCableColor   bool     `json:"supports_cable_color"`
	SupportsCableAnchors bool     `json:"supports_cable_anchors"`
	AllowedCategories    []string `json:"allowed_categories,omitempty"`
	MultiblockGroup      string   `json:"multiblock_group,omitempty"`
}

// BlockVisual describes procedural 3D/2D rendering attributes (no proprietary textures).
type BlockVisual struct {
	Color       string     `json:"color"`
	AccentColor string     `json:"accent_color,omitempty"`
	Shape       string     `json:"shape"` // "cube", "cable", "dense_cable", "panel", "frame", "core"
	Size        [3]float64 `json:"size"`
}

// ConfigFieldSchema describes an editable block property for the UI inspector.
type ConfigFieldSchema struct {
	Key         string   `json:"key"`
	Label       string   `json:"label"`
	Type        string   `json:"type"` // "number", "boolean", "select", "string", "drive_cells", "patterns"
	Default     any      `json:"default,omitempty"`
	Options     []string `json:"options,omitempty"`
	Description string   `json:"description,omitempty"`
}

// BlockMetadata captures simulation + rendering parameters for a catalog block.
type BlockMetadata struct {
	ChannelUsage         int                 `json:"channel_usage"`
	ChannelCapacity      int                 `json:"channel_capacity"`
	TransmitsChannels    bool                `json:"transmits_channels"`
	IsCable              bool                `json:"is_cable"`
	IsSmartCable         bool                `json:"is_smart_cable"`
	IsDenseCable         bool                `json:"is_dense_cable"`
	EnergyCost           float64             `json:"energy_cost"`
	EnergyCapacity       float64             `json:"energy_capacity"`
	EnergyGeneration     float64             `json:"energy_generation"`
	CraftingStorageBytes int                 `json:"crafting_storage_bytes,omitempty"`
	IsCoProcessor        bool                `json:"is_co_processor,omitempty"`
	Rotatable            bool                `json:"rotatable"`
	Description          string              `json:"description"`
	ConnectionRules      ConnectionRules     `json:"connection_rules"`
	ConfigSchema         []ConfigFieldSchema `json:"config_schema,omitempty"`
	Visual               BlockVisual         `json:"visual"`
}

// BlockDefinition represents an entry in the versioned AE2 block catalog.
type BlockDefinition struct {
	ID           string        `json:"id"`
	ProfileID    string        `json:"profile_id"`
	Name         string        `json:"name"`
	Category     string        `json:"category"`
	Metadata     BlockMetadata `json:"metadata"`
	BehaviorKey  string        `json:"behavior_key"`
	Fidelity     Fidelity      `json:"fidelity"`
	DocReference string        `json:"doc_reference"`
}

// Project represents a user-owned AE2 network design project.
type Project struct {
	ID          string         `json:"id"`
	UserID      string         `json:"user_id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	ProfileID   string         `json:"profile_id"`
	Settings    map[string]any `json:"settings"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

// PlacedBlock represents an instance of a catalog block placed in a project's 3D grid.
type PlacedBlock struct {
	ID         string         `json:"id"`
	ProjectID  string         `json:"project_id"`
	BlockDefID string         `json:"block_def_id"`
	X          int            `json:"x"`
	Y          int            `json:"y"`
	Z          int            `json:"z"`
	Facing     Direction      `json:"facing"`
	Config     map[string]any `json:"config"`
	Label      string         `json:"label,omitempty"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
}

// Coord returns the 3D coordinate of the placed block.
func (b PlacedBlock) Coord() Coord3D {
	return Coord3D{X: b.X, Y: b.Y, Z: b.Z}
}

// ItemStack represents a quantity of a specific item (format: "modid:item_name").
type ItemStack struct {
	ItemID   string `json:"item_id"`
	Quantity int    `json:"quantity"`
}

// PatternSpec defines a crafting or processing recipe encoded in a pattern.
type PatternSpec struct {
	ID          string      `json:"id,omitempty"`
	PatternType string      `json:"pattern_type"` // "crafting" | "processing"
	Inputs      []ItemStack `json:"inputs"`
	Outputs     []ItemStack `json:"outputs"`
}

// ProjectPattern persists project-level reusable patterns in SQLite.
type ProjectPattern struct {
	ID          string         `json:"id"`
	ProjectID   string         `json:"project_id"`
	PatternType string         `json:"pattern_type"`
	Inputs      []ItemStack    `json:"inputs"`
	Outputs     []ItemStack    `json:"outputs"`
	Config      map[string]any `json:"config"`
}

// ProjectInventoryEntry persists project-level initial inventory records in SQLite.
type ProjectInventoryEntry struct {
	ID              string         `json:"id"`
	ProjectID       string         `json:"project_id"`
	ItemID          string         `json:"item_id"`
	Quantity        int            `json:"quantity"`
	StorageLocation map[string]any `json:"storage_location"`
}

// CraftingRequest specifies an item and quantity to request during simulation.
type CraftingRequest struct {
	ItemID   string `json:"item_id"`
	Quantity int    `json:"quantity"`
	Source   string `json:"source,omitempty"` // e.g. "terminal"
}

// AffectedBlock identifies a block and coordinate implicated in a diagnostic.
type AffectedBlock struct {
	ID string `json:"id"`
	X  int    `json:"x"`
	Y  int    `json:"y"`
	Z  int    `json:"z"`
}

// DiagnosticSeverity classifies diagnostic importance.
type DiagnosticSeverity string

const (
	SeverityInfo    DiagnosticSeverity = "info"
	SeverityWarning DiagnosticSeverity = "warning"
	SeverityError   DiagnosticSeverity = "error"
)

// Diagnostic reports an actionable AE2 structural, channel, energy, storage, or crafting issue.
type Diagnostic struct {
	ID               string             `json:"id"`
	SimulationID     string             `json:"simulation_id,omitempty"`
	Code             string             `json:"code"`
	Severity         DiagnosticSeverity `json:"severity"`
	Message          string             `json:"message"`
	AffectedBlocks   []AffectedBlock    `json:"affected_blocks"`
	Remediation      string             `json:"remediation,omitempty"`
	BlocksSimulation bool               `json:"blocks_simulation"`
}

// SimulationStatus represents the lifecycle state of a simulation run.
type SimulationStatus string

const (
	SimStatusPending   SimulationStatus = "pending"
	SimStatusRunning   SimulationStatus = "running"
	SimStatusPaused    SimulationStatus = "paused"
	SimStatusCompleted SimulationStatus = "completed"
	SimStatusFailed    SimulationStatus = "failed"
)

// SimulationSummary provides high-level network, channel, energy, storage, and crafting telemetry.
type SimulationSummary struct {
	Networks          int     `json:"networks"`
	OnlineNetworks    int     `json:"online_networks"`
	ChannelsUsed      int     `json:"channels_used"`
	ChannelsAvailable int     `json:"channels_available"`
	EnergyStored      float64 `json:"energy_stored"`
	EnergyCapacity    float64 `json:"energy_capacity"`
	EnergyGeneration  float64 `json:"energy_generation"`
	EnergyConsumption float64 `json:"energy_consumption"`
	ItemsStored       int     `json:"items_stored"`
	StorageBytesUsed  int     `json:"storage_bytes_used"`
	StorageCapacity   int     `json:"storage_capacity"`
	ActiveJobs        int     `json:"active_jobs"`
	CompletedJobs     int     `json:"completed_jobs"`
	FailedJobs        int     `json:"failed_jobs"`
	DiagnosticsCount  int     `json:"diagnostics_count"`
}

// CableChannelOverlay exposes per-block channel usage and capacity for 3D/2D overlays.
type CableChannelOverlay struct {
	BlockID      string    `json:"block_id"`
	Coord        Coord3D   `json:"coord"`
	NetworkID    string    `json:"network_id"`
	ChannelsUsed int       `json:"channels_used"`
	Capacity     int       `json:"capacity"`
	Overloaded   bool      `json:"overloaded"`
	Online       bool      `json:"online"`
	Color        string    `json:"color,omitempty"`
	Connections  []Coord3D `json:"connections"`
}

// DeviceRouteOverlay exposes whether a channel-using device got a channel and its exact path.
type DeviceRouteOverlay struct {
	BlockID    string    `json:"block_id"`
	Coord      Coord3D   `json:"coord"`
	NetworkID  string    `json:"network_id"`
	HasChannel bool      `json:"has_channel"`
	Route      []Coord3D `json:"route,omitempty"`
}

// NetworkSummaryOverlay exposes per-network state for inspection in Debug Mode.
type NetworkSummaryOverlay struct {
	ID                string  `json:"id"`
	HasController     bool    `json:"has_controller"`
	ControllerValid   bool    `json:"controller_valid"`
	IsAdHoc           bool    `json:"is_ad_hoc"`
	Online            bool    `json:"online"`
	BlockCount        int     `json:"block_count"`
	ChannelsUsed      int     `json:"channels_used"`
	ChannelsAvailable int     `json:"channels_available"`
	EnergyStored      float64 `json:"energy_stored"`
	EnergyCapacity    float64 `json:"energy_capacity"`
	EnergyGeneration  float64 `json:"energy_generation"`
	EnergyConsumption float64 `json:"energy_consumption"`
}

// CraftingJobView exposes live Crafting CPU job state to the UI.
type CraftingJobView struct {
	ID              string      `json:"id"`
	NetworkID       string      `json:"network_id"`
	CPUID           string      `json:"cpu_id"`
	TargetItem      string      `json:"target_item"`
	TargetQuantity  int         `json:"target_quantity"`
	Status          string      `json:"status"` // "running", "completed", "failed"
	FailureReason   string      `json:"failure_reason,omitempty"`
	CoProcessors    int         `json:"co_processors"`
	BytesUsed       int         `json:"bytes_used"`
	BytesCapacity   int         `json:"bytes_capacity"`
	TotalBatches    int         `json:"total_batches"`
	CompletedCrafts int         `json:"completed_crafts"`
	PendingItems    []ItemStack `json:"pending_items,omitempty"`
}

// StorageCellView exposes drive cell occupancy and partition status to the UI.
type StorageCellView struct {
	DriveBlockID string      `json:"drive_block_id"`
	DriveCoord   Coord3D     `json:"drive_coord"`
	Slot         int         `json:"slot"`
	Tier         string      `json:"tier"`
	Priority     int         `json:"priority"`
	Online       bool        `json:"online"`
	BytesUsed    int         `json:"bytes_used"`
	BytesTotal   int         `json:"bytes_total"`
	TypesUsed    int         `json:"types_used"`
	TypesTotal   int         `json:"types_total"`
	Partition    []string    `json:"partition,omitempty"`
	Inverter     bool        `json:"inverter"`
	Items        []ItemStack `json:"items"`
}

// TopologyOverlay aggregates all visual overlay and debug state for the frontend.
type TopologyOverlay struct {
	Networks     []NetworkSummaryOverlay `json:"networks"`
	Cables       []CableChannelOverlay   `json:"cables"`
	DeviceRoutes []DeviceRouteOverlay    `json:"device_routes"`
	StorageCells []StorageCellView       `json:"storage_cells"`
	StoredTotals []ItemStack             `json:"stored_totals"`
	CraftingJobs []CraftingJobView       `json:"crafting_jobs"`
}

// SimulationRun represents a persisted simulation run record.
type SimulationRun struct {
	ID          string            `json:"id"`
	ProjectID   string            `json:"project_id"`
	Status      SimulationStatus  `json:"status"`
	Seed        int64             `json:"seed"`
	CurrentTick int               `json:"current_tick"`
	Summary     SimulationSummary `json:"summary"`
	Overlay     *TopologyOverlay  `json:"overlay,omitempty"`
	StartedAt   time.Time         `json:"started_at"`
	FinishedAt  *time.Time        `json:"finished_at,omitempty"`
}

// SimulationEvent records a discrete deterministic event on a simulation tick.
type SimulationEvent struct {
	ID           string         `json:"id"`
	SimulationID string         `json:"simulation_id"`
	Sequence     int            `json:"sequence"`
	Tick         int            `json:"tick"`
	EventType    string         `json:"event_type"`
	Payload      map[string]any `json:"payload"`
}

// ValidationResult is returned by POST /api/v1/projects/{projectId}/validate.
type ValidationResult struct {
	Valid       bool              `json:"valid"`
	Summary     SimulationSummary `json:"summary"`
	Diagnostics []Diagnostic      `json:"diagnostics"`
	Overlay     TopologyOverlay   `json:"overlay"`
}

// SimulationStepResult is returned when stepping a simulation forward.
type SimulationStepResult struct {
	Tick        int               `json:"tick"`
	Events      []SimulationEvent `json:"events"`
	Diagnostics []Diagnostic      `json:"diagnostics"`
	State       SimulationRun     `json:"state"`
}
