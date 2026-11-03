const API_BASE = '/api/v1'
const TOKEN_STORAGE_KEY = 'me_lab_token'

export interface ApiError {
  error: {
    code: string
    message: string
    request_id: string
    details?: Record<string, unknown>
  }
}

export function extractApiErrorMessage(err: unknown, fallback = 'Request failed'): string {
  if (err && typeof err === 'object') {
    if ('error' in err && err.error && typeof err.error === 'object' && 'message' in err.error) {
      const msg = err.error.message
      if (typeof msg === 'string' && msg.trim().length > 0) {
        return msg
      }
    }
    if ('message' in err && typeof err.message === 'string' && err.message.trim().length > 0) {
      return err.message
    }
  }
  return fallback
}

export interface PaginatedResponse<T> {
  data: T[]
  pagination: {
    page: number
    page_size: number
    total: number
  }
}

export interface Coord3D {
  x: number
  y: number
  z: number
}

export type Direction = 'north' | 'south' | 'east' | 'west' | 'up' | 'down'

export type Fidelity = 'exact' | 'approximated' | 'unsupported'

export interface User {
  id: string
  email: string
  created_at?: string
  updated_at?: string
}

export interface AuthResponse {
  user: User
  token: string
}

export interface CellTierSpec {
  total_bytes: number
  max_types: number
  bytes_per_type: number
}

export interface ProfileConfig {
  normal_cable_channels: number
  dense_cable_channels: number
  controller_face_channels: number
  ad_hoc_channel_limit: number
  infinite_channels: boolean
  controller_max_span: number
  controller_energy_per_block: number
  controller_energy_buffer: number
  node_base_energy_buffer: number
  channel_hop_energy_cost: number
  fe_per_ae: number
  molecular_assembler_ticks: number
  molecular_assembler_active_ae: number
  external_machine_ticks: number
  items_per_byte: number
  max_cell_types: number
  cell_tiers: Record<string, CellTierSpec>
  references: string[]
}

export interface CompatibilityProfile {
  id: string
  name: string
  minecraft_version: string
  ae2_version: string
  channel_mode: string
  config: ProfileConfig
}

export interface ConnectionRules {
  connect_all_faces: boolean
  connect_only_facing: boolean
  disconnect_push_face: boolean
  supports_cable_color: boolean
  supports_cable_anchors: boolean
  allowed_categories?: string[]
  multiblock_group?: string
}

export type BlockShape = 'cube' | 'cable' | 'dense_cable' | 'panel' | 'frame' | 'core'

export interface BlockVisual {
  color: string
  accent_color?: string
  shape: BlockShape | string
  size: [number, number, number]
}

export interface ConfigFieldSchema {
  key: string
  label: string
  type: 'number' | 'boolean' | 'select' | 'string' | 'drive_cells' | 'patterns' | string
  default?: unknown
  options?: string[]
  description?: string
}

export interface BlockMetadata {
  channel_usage: number
  channel_capacity: number
  transmits_channels: boolean
  is_cable: boolean
  is_smart_cable: boolean
  is_dense_cable: boolean
  energy_cost: number
  energy_capacity: number
  energy_generation: number
  crafting_storage_bytes?: number
  is_co_processor?: boolean
  rotatable: boolean
  description: string
  connection_rules: ConnectionRules
  config_schema?: ConfigFieldSchema[]
  visual: BlockVisual
}

export interface BlockDefinition {
  id: string
  profile_id: string
  name: string
  category: string
  metadata: BlockMetadata
  behavior_key: string
  fidelity: Fidelity
  doc_reference: string
}

export interface Project {
  id: string
  user_id: string
  name: string
  description: string
  profile_id: string
  settings: Record<string, unknown>
  created_at: string
  updated_at: string
}

export interface CreateProjectRequest {
  name: string
  description?: string
  profile_id: string
  settings?: Record<string, unknown>
}

export interface UpdateProjectRequest {
  name?: string
  description?: string
  profile_id?: string
  settings?: Record<string, unknown>
}

export interface PlacedBlock {
  id: string
  project_id: string
  block_def_id: string
  x: number
  y: number
  z: number
  facing: Direction
  config: Record<string, unknown>
  label?: string
  created_at: string
  updated_at: string
}

export interface PlaceBlockRequest {
  block_def_id: string
  x: number
  y: number
  z: number
  facing?: Direction
  config?: Record<string, unknown>
  label?: string
}

export interface UpdateBlockRequest {
  block_def_id?: string
  facing?: Direction
  config?: Record<string, unknown>
  label?: string
  x?: number
  y?: number
  z?: number
}

export interface BatchBlockOperation {
  operation: 'create' | 'update' | 'delete'
  block_id?: string
  data?: PlaceBlockRequest | UpdateBlockRequest
}

export interface ItemStack {
  item_id: string
  quantity: number
}

export interface PatternSpec {
  id?: string
  pattern_type: 'crafting' | 'processing'
  inputs: ItemStack[]
  outputs: ItemStack[]
}

export interface DriveCellConfig {
  tier: '1k' | '4k' | '16k' | '64k' | '256k'
  partition?: string[]
  inverter?: boolean
  items?: ItemStack[]
}

export interface ProjectPattern {
  id: string
  project_id: string
  pattern_type: string
  inputs: ItemStack[]
  outputs: ItemStack[]
  config: Record<string, unknown>
}

export interface ProjectInventoryEntry {
  id: string
  project_id: string
  item_id: string
  quantity: number
  storage_location: Record<string, unknown>
}

export interface CraftingRequest {
  item_id: string
  quantity: number
  source?: string
}

export interface CreateSimulationRequest {
  crafting_requests?: CraftingRequest[]
  auto_steps?: number
  seed?: number
}

export interface AffectedBlock {
  id: string
  x: number
  y: number
  z: number
}

export type DiagnosticSeverity = 'info' | 'warning' | 'error'

export interface Diagnostic {
  id: string
  simulation_id?: string
  code: string
  severity: DiagnosticSeverity
  message: string
  affected_blocks: AffectedBlock[]
  remediation?: string
  blocks_simulation: boolean
}

export type SimulationStatus = 'pending' | 'running' | 'paused' | 'completed' | 'failed'

export interface SimulationSummary {
  networks: number
  online_networks: number
  channels_used: number
  channels_available: number
  energy_stored: number
  energy_capacity: number
  energy_generation: number
  energy_consumption: number
  items_stored: number
  storage_bytes_used: number
  storage_capacity: number
  active_jobs: number
  completed_jobs: number
  failed_jobs: number
  diagnostics_count: number
}

export interface CableChannelOverlay {
  block_id: string
  coord: Coord3D
  network_id: string
  channels_used: number
  capacity: number
  overloaded: boolean
  online: boolean
  color?: string
  connections: Coord3D[]
}

export interface DeviceRouteOverlay {
  block_id: string
  coord: Coord3D
  network_id: string
  has_channel: boolean
  online: boolean
  route?: Coord3D[]
}

export interface AutomationBusOverlay {
  block_id: string
  coord: Coord3D
  network_id: string
  kind: string
  target_block_id?: string
  target_coord: Coord3D
  has_channel: boolean
  online: boolean
  last_transferred: number
}

export interface ExternalInventoryView {
  block_id: string
  coord: Coord3D
  capacity: number
  items: ItemStack[]
  input?: ItemStack[]
  fuel?: ItemStack[]
  output?: ItemStack[]
  lit?: boolean
  cook_time?: number
  cook_time_total?: number
  burn_time?: number
  burn_time_total?: number
}
export interface NetworkSummaryOverlay {
  id: string
  has_controller: boolean
  controller_valid: boolean
  is_ad_hoc: boolean
  online: boolean
  block_count: number
  channels_used: number
  channels_available: number
  energy_stored: number
  energy_capacity: number
  energy_generation: number
  energy_consumption: number
}

export interface CraftingJobView {
  id: string
  network_id: string
  cpu_id: string
  target_item: string
  target_quantity: number
  status: string
  failure_reason?: string
  co_processors: number
  bytes_used: number
  bytes_capacity: number
  total_batches: number
  completed_crafts: number
  pending_items?: ItemStack[]
}

export interface StorageCellView {
  drive_block_id: string
  drive_coord: Coord3D
  slot: number
  tier: string
  priority: number
  online: boolean
  bytes_used: number
  bytes_total: number
  types_used: number
  types_total: number
  partition?: string[]
  inverter: boolean
  items: ItemStack[]
}

export interface TopologyOverlay {
  networks: NetworkSummaryOverlay[]
  cables: CableChannelOverlay[]
  device_routes: DeviceRouteOverlay[]
  storage_cells: StorageCellView[]
  stored_totals: ItemStack[]
  crafting_jobs: CraftingJobView[]
  automation_buses: AutomationBusOverlay[]
  external_inventories: ExternalInventoryView[]
}

export interface SimulationRun {
  id: string
  project_id: string
  status: SimulationStatus
  seed: number
  current_tick: number
  summary: SimulationSummary
  overlay?: TopologyOverlay
  started_at: string
  finished_at?: string
}

export interface SimulationEvent {
  id: string
  simulation_id: string
  sequence: number
  tick: number
  event_type: string
  payload: Record<string, unknown>
}

export interface ValidationResult {
  valid: boolean
  summary: SimulationSummary
  diagnostics: Diagnostic[]
  overlay: TopologyOverlay
}

export interface SimulationStepResult {
  tick: number
  events: SimulationEvent[]
  diagnostics: Diagnostic[]
  state: SimulationRun
}

export interface BlueprintExport {
  schema_version: string
  exported_at: string
  project: Project
  profile: CompatibilityProfile
  blocks: PlacedBlock[]
  patterns?: ProjectPattern[]
  inventory?: ProjectInventoryEntry[]
  validation_summary?: SimulationSummary
}

export class ApiClient {
  private baseUrl: string
  private token: string | null = null

  constructor(baseUrl: string) {
    this.baseUrl = baseUrl
    if (typeof window !== 'undefined' && window.localStorage) {
      this.token = window.localStorage.getItem(TOKEN_STORAGE_KEY)
    }
  }

  setToken(token: string | null) {
    this.token = token
    if (typeof window !== 'undefined' && window.localStorage) {
      if (token) {
        window.localStorage.setItem(TOKEN_STORAGE_KEY, token)
      } else {
        window.localStorage.removeItem(TOKEN_STORAGE_KEY)
      }
    }
  }

  getToken(): string | null {
    if (!this.token && typeof window !== 'undefined' && window.localStorage) {
      this.token = window.localStorage.getItem(TOKEN_STORAGE_KEY)
    }
    return this.token
  }

  private buildHeaders(hasBody: boolean): Record<string, string> {
    const headers: Record<string, string> = {
      Accept: 'application/json, text/markdown, text/plain',
    }
    if (hasBody) {
      headers['Content-Type'] = 'application/json'
    }
    const currentToken = this.getToken()
    if (currentToken) {
      headers['Authorization'] = `Bearer ${currentToken}`
    }
    return headers
  }

  private async request<T>(
    method: string,
    path: string,
    body?: unknown,
    params?: Record<string, string>,
    useOriginRoot = false
  ): Promise<T> {
    const origin = typeof window !== 'undefined' ? window.location.origin : 'http://localhost'
    const fullPath = useOriginRoot ? path : `${this.baseUrl}${path}`
    const url = new URL(fullPath, origin)

    if (params) {
      Object.entries(params).forEach(([key, value]) => {
        if (value !== undefined && value !== '') {
          url.searchParams.set(key, value)
        }
      })
    }

    const hasBody = body !== undefined
    const response = await fetch(url.toString(), {
      method,
      headers: this.buildHeaders(hasBody),
      body: hasBody ? JSON.stringify(body) : undefined,
    })

    if (!response.ok) {
      const errorBody = await response.json().catch(() => ({
        error: {
          code: 'UNKNOWN_ERROR',
          message: response.statusText || `HTTP ${response.status}`,
          request_id: '',
          details: {},
        },
      }))
      throw errorBody as ApiError
    }

    if (response.status === 204) {
      return undefined as T
    }

    const contentType = response.headers.get('content-type') ?? ''
    if (contentType.includes('text/markdown') || contentType.includes('text/plain')) {
      const text = await response.text()
      return text as unknown as T
    }

    return response.json() as Promise<T>
  }

  // Health (at origin root /health, NOT /api/v1/health)
  async health() {
    return this.request<{ status: string; version: string; timestamp: string }>(
      'GET',
      '/health',
      undefined,
      undefined,
      true
    )
  }

  // Auth
  async register(email: string, password: string): Promise<AuthResponse> {
    const res = await this.request<AuthResponse>('POST', '/auth/register', { email, password })
    if (res?.token) {
      this.setToken(res.token)
    }
    return res
  }

  async login(email: string, password: string): Promise<AuthResponse> {
    const res = await this.request<AuthResponse>('POST', '/auth/login', { email, password })
    if (res?.token) {
      this.setToken(res.token)
    }
    return res
  }

  async me(): Promise<User> {
    const res = await this.request<User | { user: User }>('GET', '/auth/me')
    if (res && typeof res === 'object' && 'user' in res && res.user) {
      return res.user
    }
    return res as User
  }

  async logout(): Promise<void> {
    try {
      await this.request<unknown>('POST', '/auth/logout')
    } catch {
      // Stateless JWT servers may omit /auth/logout or reject expired tokens; always clear local token.
    } finally {
      this.setToken(null)
    }
  }

  // Catalog
  async listProfiles(): Promise<CompatibilityProfile[]> {
    const res = await this.request<CompatibilityProfile[] | { data: CompatibilityProfile[] }>(
      'GET',
      '/catalog/profiles'
    )
    return Array.isArray(res) ? res : (res.data ?? [])
  }

  async listBlockDefinitions(profileId?: string): Promise<BlockDefinition[]> {
    const params: Record<string, string> = {}
    if (profileId) {
      params.profile_id = profileId
    }
    const res = await this.request<BlockDefinition[] | { data: BlockDefinition[] }>(
      'GET',
      '/catalog/blocks',
      undefined,
      params
    )
    return Array.isArray(res) ? res : (res.data ?? [])
  }

  async getBlockDefinition(id: string, profileId?: string): Promise<BlockDefinition> {
    const params: Record<string, string> = {}
    if (profileId) {
      params.profile_id = profileId
    }
    return this.request<BlockDefinition>(
      'GET',
      `/catalog/blocks/${encodeURIComponent(id)}`,
      undefined,
      params
    )
  }

  // Projects
  async listProjects(page = 1, pageSize = 20): Promise<PaginatedResponse<Project>> {
    return this.request<PaginatedResponse<Project>>('GET', '/projects', undefined, {
      page: page.toString(),
      page_size: pageSize.toString(),
    })
  }

  async getProject(id: string): Promise<Project> {
    return this.request<Project>('GET', `/projects/${id}`)
  }

  async createProject(data: CreateProjectRequest): Promise<Project> {
    return this.request<Project>('POST', '/projects', data)
  }

  async updateProject(id: string, data: UpdateProjectRequest): Promise<Project> {
    return this.request<Project>('PATCH', `/projects/${id}`, data)
  }

  async deleteProject(id: string): Promise<void> {
    return this.request<void>('DELETE', `/projects/${id}`)
  }

  // Blocks
  async listBlocks(projectId: string): Promise<PlacedBlock[]> {
    const res = await this.request<PlacedBlock[] | { data?: PlacedBlock[]; results?: PlacedBlock[] }>(
      'GET',
      `/projects/${projectId}/blocks`
    )
    if (Array.isArray(res)) {
      return res
    }
    return res.data ?? res.results ?? []
  }

  async placeBlock(projectId: string, data: PlaceBlockRequest): Promise<PlacedBlock> {
    return this.request<PlacedBlock>('POST', `/projects/${projectId}/blocks`, data)
  }

  async updateBlock(projectId: string, blockId: string, data: UpdateBlockRequest): Promise<PlacedBlock> {
    return this.request<PlacedBlock>('PATCH', `/projects/${projectId}/blocks/${blockId}`, data)
  }

  async deleteBlock(projectId: string, blockId: string): Promise<void> {
    return this.request<void>('DELETE', `/projects/${projectId}/blocks/${blockId}`)
  }

  async batchBlocks(
    projectId: string,
    operations: BatchBlockOperation[],
    replaceAll = false
  ): Promise<{ results: PlacedBlock[] }> {
    const res = await this.request<
      { results?: PlacedBlock[]; blocks?: PlacedBlock[]; data?: PlacedBlock[] } | PlacedBlock[]
    >('POST', `/projects/${projectId}/blocks/batch`, {
      operations,
      replace_all: replaceAll,
    })
    if (Array.isArray(res)) {
      return { results: res }
    }
    return { results: res.results ?? res.blocks ?? res.data ?? [] }
  }

  // Validation & Simulation
  async validateProject(projectId: string): Promise<ValidationResult> {
    return this.request<ValidationResult>('POST', `/projects/${projectId}/validate`)
  }

  async createSimulation(
    projectId: string,
    payload?: CreateSimulationRequest
  ): Promise<SimulationRun> {
    return this.request<SimulationRun>(
      'POST',
      `/projects/${projectId}/simulations`,
      payload ?? {}
    )
  }

  async listSimulations(
    projectId: string,
    page = 1,
    pageSize = 20
  ): Promise<PaginatedResponse<SimulationRun>> {
    return this.request<PaginatedResponse<SimulationRun>>(
      'GET',
      `/projects/${projectId}/simulations`,
      undefined,
      {
        page: page.toString(),
        page_size: pageSize.toString(),
      }
    )
  }

  async getSimulation(simId: string): Promise<SimulationRun> {
    return this.request<SimulationRun>('GET', `/simulations/${simId}`)
  }

  async queueCrafting(simId: string, request: CraftingRequest): Promise<SimulationRun> {
    return this.request<SimulationRun>('POST', `/simulations/${simId}/craft`, request)
  }

  async stepSimulation(simId: string, ticks = 1): Promise<SimulationStepResult> {
    return this.request<SimulationStepResult>('POST', `/simulations/${simId}/step`, { ticks })
  }

  async pauseSimulation(simId: string): Promise<SimulationRun> {
    return this.request<SimulationRun>('POST', `/simulations/${simId}/pause`)
  }

  async resumeSimulation(simId: string): Promise<SimulationRun> {
    return this.request<SimulationRun>('POST', `/simulations/${simId}/resume`)
  }

  async resetSimulation(simId: string): Promise<SimulationRun> {
    return this.request<SimulationRun>('POST', `/simulations/${simId}/reset`)
  }

  async getSimulationEvents(
    simId: string,
    page = 1,
    pageSize = 50
  ): Promise<PaginatedResponse<SimulationEvent>> {
    return this.request<PaginatedResponse<SimulationEvent>>(
      'GET',
      `/simulations/${simId}/events`,
      undefined,
      {
        page: page.toString(),
        page_size: pageSize.toString(),
      }
    )
  }

  async getSimulationDiagnostics(simId: string): Promise<Diagnostic[]> {
    const res = await this.request<Diagnostic[] | { data: Diagnostic[] }>(
      'GET',
      `/simulations/${simId}/diagnostics`
    )
    return Array.isArray(res) ? res : (res.data ?? [])
  }

  // Export
  async exportBlueprint(projectId: string): Promise<BlueprintExport> {
    return this.request<BlueprintExport>('GET', `/projects/${projectId}/export/json`)
  }

  async exportBuildSheet(projectId: string): Promise<{ content: string }> {
    const res = await this.request<string | { content?: string; markdown?: string }>(
      'GET',
      `/projects/${projectId}/export/build-sheet`
    )
    if (typeof res === 'string') {
      return { content: res }
    }
    return { content: res.content ?? res.markdown ?? '' }
  }
}

export const api = new ApiClient(API_BASE)
