const API_BASE = '/api/v1'

export interface ApiError {
  error: {
    code: string
    message: string
    request_id: string
    details: Record<string, unknown>
  }
}

export interface PaginatedResponse<T> {
  data: T[]
  pagination: {
    page: number
    page_size: number
    total: number
  }
}

class ApiClient {
  private baseUrl: string
  private token: string | null = null

  constructor(baseUrl: string) {
    this.baseUrl = baseUrl
    this.token = localStorage.getItem('me_lab_token')
  }

  setToken(token: string | null) {
    this.token = token
    if (token) {
      localStorage.setItem('me_lab_token', token)
    } else {
      localStorage.removeItem('me_lab_token')
    }
  }

  getToken(): string | null {
    return this.token
  }

  private async request<T>(
    method: string,
    path: string,
    body?: unknown,
    params?: Record<string, string>
  ): Promise<T> {
    const url = new URL(`${this.baseUrl}${path}`, window.location.origin)
    if (params) {
      Object.entries(params).forEach(([key, value]) => {
        url.searchParams.set(key, value)
      })
    }

    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
    }

    if (this.token) {
      headers['Authorization'] = `Bearer ${this.token}`
    }

    const response = await fetch(url.toString(), {
      method,
      headers,
      body: body ? JSON.stringify(body) : undefined,
    })

    if (!response.ok) {
      const errorBody = await response.json().catch(() => ({
        error: {
          code: 'UNKNOWN_ERROR',
          message: response.statusText,
          request_id: '',
          details: {},
        },
      }))
      throw errorBody as ApiError
    }

    if (response.status === 204) {
      return undefined as T
    }

    return response.json()
  }

  // Health
  async health() {
    return this.request<{ status: string; version: string; timestamp: string }>('GET', '/health')
  }

  // Auth
  async register(email: string, password: string) {
    return this.request<{ user: { id: string; email: string }; token: string }>(
      'POST',
      '/auth/register',
      { email, password }
    )
  }

  async login(email: string, password: string) {
    return this.request<{ user: { id: string; email: string }; token: string }>(
      'POST',
      '/auth/login',
      { email, password }
    )
  }

  async me() {
    return this.request<{ id: string; email: string }>('GET', '/auth/me')
  }

  // Projects
  async listProjects(page = 1, pageSize = 20) {
    return this.request<PaginatedResponse<Project>>('GET', '/projects', undefined, {
      page: page.toString(),
      page_size: pageSize.toString(),
    })
  }

  async getProject(id: string) {
    return this.request<Project>('GET', `/projects/${id}`)
  }

  async createProject(data: CreateProjectRequest) {
    return this.request<Project>('POST', '/projects', data)
  }

  async updateProject(id: string, data: Partial<CreateProjectRequest>) {
    return this.request<Project>('PATCH', `/projects/${id}`, data)
  }

  async deleteProject(id: string) {
    return this.request<void>('DELETE', `/projects/${id}`)
  }

  // Blocks
  async listBlocks(projectId: string) {
    return this.request<PlacedBlock[]>('GET', `/projects/${projectId}/blocks`)
  }

  async placeBlock(projectId: string, data: PlaceBlockRequest) {
    return this.request<PlacedBlock>('POST', `/projects/${projectId}/blocks`, data)
  }

  async updateBlock(projectId: string, blockId: string, data: UpdateBlockRequest) {
    return this.request<PlacedBlock>('PATCH', `/projects/${projectId}/blocks/${blockId}`, data)
  }

  async deleteBlock(projectId: string, blockId: string) {
    return this.request<void>('DELETE', `/projects/${projectId}/blocks/${blockId}`)
  }

  async batchBlocks(projectId: string, operations: BatchBlockOperation[]) {
    return this.request<{ results: PlacedBlock[] }>(
      'POST',
      `/projects/${projectId}/blocks/batch`,
      { operations }
    )
  }

  // Catalog
  async listBlockDefinitions() {
    return this.request<BlockDefinition[]>('GET', '/catalog/blocks')
  }

  async getBlockDefinition(id: string) {
    return this.request<BlockDefinition>('GET', `/catalog/blocks/${id}`)
  }

  async listProfiles() {
    return this.request<CompatibilityProfile[]>('GET', '/catalog/profiles')
  }

  // Simulation
  async createSimulation(projectId: string) {
    return this.request<SimulationRun>('POST', `/projects/${projectId}/simulations`)
  }

  async listSimulations(projectId: string) {
    return this.request<PaginatedResponse<SimulationRun>>('GET', `/projects/${projectId}/simulations`)
  }

  async getSimulation(simulationId: string) {
    return this.request<SimulationRun>('GET', `/simulations/${simulationId}`)
  }

  async stepSimulation(simulationId: string) {
    return this.request<SimulationStepResult>('POST', `/simulations/${simulationId}/step`)
  }

  async pauseSimulation(simulationId: string) {
    return this.request<SimulationRun>('POST', `/simulations/${simulationId}/pause`)
  }

  async resumeSimulation(simulationId: string) {
    return this.request<SimulationRun>('POST', `/simulations/${simulationId}/resume`)
  }

  async resetSimulation(simulationId: string) {
    return this.request<SimulationRun>('POST', `/simulations/${simulationId}/reset`)
  }

  async getSimulationEvents(simulationId: string, page = 1, pageSize = 50) {
    return this.request<PaginatedResponse<SimulationEvent>>('GET', `/simulations/${simulationId}/events`, undefined, {
      page: page.toString(),
      page_size: pageSize.toString(),
    })
  }

  async getSimulationDiagnostics(simulationId: string) {
    return this.request<Diagnostic[]>('GET', `/simulations/${simulationId}/diagnostics`)
  }

  // Validate
  async validateProject(projectId: string) {
    return this.request<ValidationResult>('POST', `/projects/${projectId}/validate`)
  }

  // Export
  async exportBlueprint(projectId: string) {
    return this.request<unknown>('GET', `/projects/${projectId}/export/json`)
  }

  async exportBuildSheet(projectId: string) {
    return this.request<{ content: string }>('GET', `/projects/${projectId}/export/build-sheet`)
  }
}

// Types
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

export interface BlockDefinition {
  id: string
  profile_id: string
  name: string
  category: string
  metadata: BlockMetadata
  behavior_key: string
  fidelity: 'exact' | 'approximated' | 'unsupported'
  doc_reference: string
}

export interface BlockMetadata {
  channel_usage: number
  channel_capacity: number
  energy_cost: number
  energy_capacity: number
  connection_rules: Record<string, unknown>
  visual: {
    color: string
    shape: string
    size: [number, number, number]
  }
}

export interface CompatibilityProfile {
  id: string
  name: string
  minecraft_version: string
  ae2_version: string
  channel_mode: string
  config: Record<string, unknown>
}

export type Direction = 'north' | 'south' | 'east' | 'west' | 'up' | 'down'

export interface SimulationRun {
  id: string
  project_id: string
  status: 'pending' | 'running' | 'paused' | 'completed' | 'failed'
  current_tick: number
  summary: SimulationSummary
  started_at: string
  finished_at?: string
}

export interface SimulationSummary {
  networks: number
  channels_used: number
  channels_available: number
  energy_generation: number
  energy_consumption: number
  items_stored: number
  storage_capacity: number
  active_jobs: number
  diagnostics_count: number
}

export interface SimulationStepResult {
  tick: number
  events: SimulationEvent[]
  state: SimulationRun
}

export interface SimulationEvent {
  id: string
  simulation_id: string
  sequence: number
  tick: number
  event_type: string
  payload: Record<string, unknown>
}

export interface Diagnostic {
  id: string
  simulation_id: string
  code: string
  severity: 'info' | 'warning' | 'error'
  message: string
  affected_blocks: Array<{ id: string; x: number; y: number; z: number }>
  remediation?: string
}

export interface ValidationResult {
  valid: boolean
  diagnostics: Diagnostic[]
}

export const api = new ApiClient(API_BASE)
