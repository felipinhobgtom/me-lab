import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import App from '../App'
import { useAuthStore } from '../features/auth/authStore'
import { EditorLayout } from '../features/editor/EditorLayout'
import { useEditorStore } from '../features/editor/store'
import {
  api,
  type BlockDefinition,
  type CompatibilityProfile,
  type PlacedBlock,
  type Project,
  type SimulationRun,
  type SimulationSummary,
  type TopologyOverlay,
  type ValidationResult,
} from '../lib/api'

const MOCK_PROFILE: CompatibilityProfile = {
  id: 'ae2-1.21.1-default',
  name: 'AE2 1.21.1 Default',
  minecraft_version: '1.21.1',
  ae2_version: '19.0.x',
  channel_mode: 'default',
  config: {
    normal_cable_channels: 8,
    dense_cable_channels: 32,
    controller_face_channels: 32,
    ad_hoc_channel_limit: 8,
    infinite_channels: false,
    controller_max_span: 7,
    controller_energy_per_block: 6,
    controller_energy_buffer: 8000,
    node_base_energy_buffer: 25,
    channel_hop_energy_cost: 0.0078125,
    fe_per_ae: 2,
    molecular_assembler_ticks: 4,
    molecular_assembler_active_ae: 20,
    external_machine_ticks: 8,
    items_per_byte: 8,
    max_cell_types: 63,
    cell_tiers: {
      '1k': { total_bytes: 1024, max_types: 63, bytes_per_type: 8 },
      '4k': { total_bytes: 4096, max_types: 63, bytes_per_type: 32 },
    },
    references: ['https://guide.appliedenergistics.org/1.21.1/'],
  },
}

const MOCK_PROJECT: Project = {
  id: 'proj-1',
  user_id: 'user-1',
  name: 'Test AE2 Network',
  description: 'Unit test project',
  profile_id: 'ae2-1.21.1-default',
  settings: {},
  created_at: '2026-11-02T12:00:00Z',
  updated_at: '2026-11-02T12:00:00Z',
}

const MOCK_CATALOG: BlockDefinition[] = [
  {
    id: 'ae2:controller',
    profile_id: 'ae2-1.21.1-default',
    name: 'ME Controller',
    category: 'network',
    behavior_key: 'controller',
    fidelity: 'exact',
    doc_reference: 'https://guide.appliedenergistics.org/1.21.1/items-blocks-machines/controller',
    metadata: {
      channel_usage: 0,
      channel_capacity: 32,
      transmits_channels: false,
      is_cable: false,
      is_smart_cable: false,
      is_dense_cable: false,
      energy_cost: 6,
      energy_capacity: 8000,
      energy_generation: 0,
      rotatable: false,
      description: 'Routes up to 32 channels per exposed face.',
      connection_rules: {
        connect_all_faces: true,
        connect_only_facing: false,
        disconnect_push_face: false,
        supports_cable_color: false,
        supports_cable_anchors: false,
      },
      visual: {
        color: '#4c1d95',
        accent_color: '#8b5cf6',
        shape: 'core',
        size: [1, 1, 1],
      },
    },
  },
  {
    id: 'ae2:smart_cable',
    profile_id: 'ae2-1.21.1-default',
    name: 'Fluix ME Smart Cable',
    category: 'cables',
    behavior_key: 'cable',
    fidelity: 'exact',
    doc_reference: 'https://guide.appliedenergistics.org/1.21.1/items-blocks-machines/cables',
    metadata: {
      channel_usage: 0,
      channel_capacity: 8,
      transmits_channels: true,
      is_cable: true,
      is_smart_cable: true,
      is_dense_cable: false,
      energy_cost: 0,
      energy_capacity: 0,
      energy_generation: 0,
      rotatable: false,
      description: 'Carries up to 8 channels with live channel bars.',
      connection_rules: {
        connect_all_faces: true,
        connect_only_facing: false,
        disconnect_push_face: false,
        supports_cable_color: true,
        supports_cable_anchors: true,
      },
      visual: {
        color: '#8b5cf6',
        accent_color: '#38bdf8',
        shape: 'cable',
        size: [0.3, 0.3, 0.3],
      },
    },
  },
  {
    id: 'ae2:drive',
    profile_id: 'ae2-1.21.1-default',
    name: 'ME Drive',
    category: 'storage',
    behavior_key: 'drive',
    fidelity: 'exact',
    doc_reference: 'https://guide.appliedenergistics.org/1.21.1/items-blocks-machines/drive',
    metadata: {
      channel_usage: 1,
      channel_capacity: 8,
      transmits_channels: true,
      is_cable: false,
      is_smart_cable: false,
      is_dense_cable: false,
      energy_cost: 2,
      energy_capacity: 25,
      energy_generation: 0,
      rotatable: true,
      description: 'Holds up to 10 ME storage cells.',
      connection_rules: {
        connect_all_faces: true,
        connect_only_facing: false,
        disconnect_push_face: false,
        supports_cable_color: false,
        supports_cable_anchors: false,
      },
      visual: {
        color: '#334155',
        accent_color: '#38bdf8',
        shape: 'cube',
        size: [1, 1, 1],
      },
    },
  },
]

const EMPTY_SUMMARY: SimulationSummary = {
  networks: 1,
  online_networks: 1,
  channels_used: 1,
  channels_available: 8,
  energy_stored: 8000,
  energy_capacity: 8000,
  energy_generation: 400,
  energy_consumption: 8,
  items_stored: 64,
  storage_bytes_used: 40,
  storage_capacity: 4096,
  active_jobs: 0,
  completed_jobs: 0,
  failed_jobs: 0,
  diagnostics_count: 0,
}

const EMPTY_OVERLAY: TopologyOverlay = {
  networks: [],
  cables: [],
  device_routes: [],
  storage_cells: [],
  stored_totals: [],
  crafting_jobs: [],
}

function renderEditor(projectId = 'proj-1') {
  return render(
    <MemoryRouter initialEntries={[`/projects/${projectId}/editor`]}>
      <Routes>
        <Route path="/projects/:projectId/editor" element={<EditorLayout />} />
      </Routes>
    </MemoryRouter>
  )
}

describe('ME Lab Technical Editor', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
    window.localStorage.clear()
    useEditorStore.getState().resetWorkspace()
    api.setToken('test-jwt-token')

    vi.spyOn(api, 'getProject').mockResolvedValue(MOCK_PROJECT)
    vi.spyOn(api, 'listProfiles').mockResolvedValue([MOCK_PROFILE])
    vi.spyOn(api, 'listBlockDefinitions').mockResolvedValue(MOCK_CATALOG)
    vi.spyOn(api, 'listBlocks').mockResolvedValue([])
  })

  it('1. selects a block from the palette and places it onto the coordinate grid', async () => {
    renderEditor()
    await screen.findByText('Test AE2 Network')

    // Select ME Drive in the left catalog palette
    fireEvent.click(screen.getByTestId('palette-item-ae2:drive'))
    expect(useEditorStore.getState().activeBlockDefId).toBe('ae2:drive')

    // Click coordinate cell (2, 0, 1) on the Y=0 slice grid
    const targetCell = screen.getByTestId('grid-cell-2-0-1')
    fireEvent.click(targetCell)

    const { blocks, saveStatus } = useEditorStore.getState()
    expect(blocks).toHaveLength(1)
    expect(blocks[0]).toMatchObject({
      block_def_id: 'ae2:drive',
      x: 2,
      y: 0,
      z: 1,
      facing: 'north',
    })
    expect(saveStatus).toBe('unsaved')
    expect(screen.getByTestId('save-status-badge')).toHaveTextContent('Unsaved')
    expect(screen.getByTestId('inspector-coords')).toHaveTextContent('(2, 0, 1)')
    expect(targetCell).toHaveTextContent('DRV')
  })

  it('2. rotates placed blocks and updates inspector properties and device configs', async () => {
    renderEditor()
    await screen.findByText('Test AE2 Network')

    // Place a Smart Cable at (1, 0, 0)
    fireEvent.click(screen.getByTestId('palette-item-ae2:smart_cable'))
    fireEvent.click(screen.getByTestId('grid-cell-1-0-0'))

    // Place an ME Drive at (2, 0, 0) (becomes selected)
    fireEvent.click(screen.getByTestId('palette-item-ae2:drive'))
    fireEvent.click(screen.getByTestId('grid-cell-2-0-0'))

    // Rotate selected ME Drive clockwise (north -> east)
    fireEvent.click(screen.getByRole('button', { name: 'Rotate (R)' }))
    let driveBlock = useEditorStore
      .getState()
      .blocks.find((b) => b.block_def_id === 'ae2:drive')
    expect(driveBlock?.facing).toBe('east')

    // Update facing, user label, and storage priority via Inspector
    fireEvent.change(screen.getByLabelText('Block Facing'), {
      target: { value: 'south' },
    })
    fireEvent.change(screen.getByLabelText('Block Label'), {
      target: { value: 'Ore Vault Drive' },
    })
    fireEvent.change(screen.getByLabelText('Drive Priority'), {
      target: { value: '25' },
    })

    driveBlock = useEditorStore
      .getState()
      .blocks.find((b) => b.block_def_id === 'ae2:drive')
    expect(driveBlock?.facing).toBe('south')
    expect(driveBlock?.label).toBe('Ore Vault Drive')
    expect(driveBlock?.config.priority).toBe(25)

    // Select Smart Cable at (1, 0, 0) and update Cable Color + Anchored Face
    fireEvent.click(screen.getByTestId('grid-cell-1-0-0'))
    fireEvent.change(screen.getByLabelText('Cable Color'), {
      target: { value: 'cyan' },
    })
    fireEvent.click(screen.getByLabelText('Anchor north'))

    const cableBlock = useEditorStore
      .getState()
      .blocks.find((b) => b.block_def_id === 'ae2:smart_cable')
    expect(cableBlock?.config).toEqual({
      color: 'cyan',
      anchored_faces: ['north'],
    })
  })

  it('3. pushes and pops workspace snapshots accurately through Undo and Redo', async () => {
    renderEditor()
    await screen.findByText('Test AE2 Network')

    // Place Controller at (0, 0, 0) and Smart Cable at (1, 0, 0)
    fireEvent.click(screen.getByTestId('palette-item-ae2:controller'))
    fireEvent.click(screen.getByTestId('grid-cell-0-0-0'))

    fireEvent.click(screen.getByTestId('palette-item-ae2:smart_cable'))
    fireEvent.click(screen.getByTestId('grid-cell-1-0-0'))

    expect(useEditorStore.getState().blocks).toHaveLength(2)

    const undoBtn = screen.getByRole('button', { name: 'Undo' })
    const redoBtn = screen.getByRole('button', { name: 'Redo' })

    // Undo second placement
    fireEvent.click(undoBtn)
    expect(useEditorStore.getState().blocks).toHaveLength(1)
    expect(useEditorStore.getState().blocks[0].block_def_id).toBe('ae2:controller')

    // Undo first placement
    fireEvent.click(undoBtn)
    expect(useEditorStore.getState().blocks).toHaveLength(0)
    expect(undoBtn).toBeDisabled()

    // Redo both placements
    fireEvent.click(redoBtn)
    expect(useEditorStore.getState().blocks).toHaveLength(1)
    fireEvent.click(redoBtn)
    expect(useEditorStore.getState().blocks).toHaveLength(2)
    expect(redoBtn).toBeDisabled()
  })

  it('4. reconciles local edits with persisted server blocks via batchBlocks on Save Layout', async () => {
    const initialServerBlock: PlacedBlock = {
      id: 'srv-ctrl-1',
      project_id: 'proj-1',
      block_def_id: 'ae2:controller',
      x: 0,
      y: 0,
      z: 0,
      facing: 'north',
      config: {},
      created_at: '2026-11-02T12:00:00Z',
      updated_at: '2026-11-02T12:00:00Z',
    }
    vi.spyOn(api, 'listBlocks').mockResolvedValue([initialServerBlock])

    let resolveBatch!: (value: { results: PlacedBlock[] }) => void
    const batchPromise = new Promise<{ results: PlacedBlock[] }>((resolve) => {
      resolveBatch = resolve
    })
    const batchSpy = vi.spyOn(api, 'batchBlocks').mockReturnValue(batchPromise)

    renderEditor()
    await screen.findByText('Test AE2 Network')
    expect(screen.getByTestId('save-status-badge')).toHaveTextContent('Saved')

    // Place a new local ME Drive at (2, 0, 1)
    fireEvent.click(screen.getByTestId('palette-item-ae2:drive'))
    fireEvent.click(screen.getByTestId('grid-cell-2-0-1'))
    expect(screen.getByTestId('save-status-badge')).toHaveTextContent('Unsaved')

    // Click Save Layout — MUST show Saving… and NOT Saved until batchBlocks resolves
    fireEvent.click(screen.getByRole('button', { name: 'Save Layout' }))
    expect(screen.getByTestId('save-status-badge')).toHaveTextContent('Saving…')
    expect(batchSpy).toHaveBeenCalledWith(
      'proj-1',
      expect.arrayContaining([
        expect.objectContaining({
          operation: 'create',
          data: expect.objectContaining({ block_def_id: 'ae2:controller', x: 0, y: 0, z: 0 }),
        }),
        expect.objectContaining({
          operation: 'create',
          data: expect.objectContaining({ block_def_id: 'ae2:drive', x: 2, y: 0, z: 1 }),
        }),
      ]),
      true
    )

    // Resolve backend persistence response with canonical server UUIDs
    resolveBatch({
      results: [
        initialServerBlock,
        {
          id: 'srv-drive-99',
          project_id: 'proj-1',
          block_def_id: 'ae2:drive',
          x: 2,
          y: 0,
          z: 1,
          facing: 'north',
          config: { priority: 0, cells: [{ tier: '4k', partition: [], inverter: false, items: [] }] },
          created_at: '2026-11-02T12:01:00Z',
          updated_at: '2026-11-02T12:01:00Z',
        },
      ],
    })

    await waitFor(() => {
      expect(screen.getByTestId('save-status-badge')).toHaveTextContent('Saved')
    })

    const reconciled = useEditorStore.getState().blocks
    expect(reconciled.map((b) => b.id)).toEqual(['srv-ctrl-1', 'srv-drive-99'])
    expect(useEditorStore.getState().selectedBlockIds).toEqual(['srv-drive-99'])
  })

  it('5. triggers validation and simulation and highlights affected coordinates when clicking a diagnostic', async () => {
    const faultBlocks: PlacedBlock[] = [
      {
        id: 'blk-cable-0',
        project_id: 'proj-1',
        block_def_id: 'ae2:smart_cable',
        x: 0,
        y: 0,
        z: 0,
        facing: 'north',
        config: { color: 'fluix', anchored_faces: [] },
        created_at: '2026-11-02T12:00:00Z',
        updated_at: '2026-11-02T12:00:00Z',
      },
      {
        id: 'blk-drive-fault',
        project_id: 'proj-1',
        block_def_id: 'ae2:drive',
        x: 3,
        y: 0,
        z: 1,
        facing: 'north',
        config: { priority: 0, cells: [] },
        created_at: '2026-11-02T12:00:00Z',
        updated_at: '2026-11-02T12:00:00Z',
      },
    ]
    vi.spyOn(api, 'listBlocks').mockResolvedValue(faultBlocks)

    const mockValidation: ValidationResult = {
      valid: false,
      summary: { ...EMPTY_SUMMARY, diagnostics_count: 1 },
      diagnostics: [
        {
          id: 'diag-ch-1',
          code: 'DEVICE_NO_CHANNEL',
          severity: 'error',
          message: 'ME Drive at (3, 0, 1) failed to acquire a channel.',
          remediation: 'Add an ME Controller or route via Dense Cable.',
          blocks_simulation: false,
          affected_blocks: [{ id: 'blk-drive-fault', x: 3, y: 0, z: 1 }],
        },
      ],
      overlay: EMPTY_OVERLAY,
    }
    const validateSpy = vi.spyOn(api, 'validateProject').mockResolvedValue(mockValidation)

    const mockSimRun: SimulationRun = {
      id: 'sim-run-1',
      project_id: 'proj-1',
      status: 'running',
      seed: 42,
      current_tick: 0,
      summary: EMPTY_SUMMARY,
      overlay: EMPTY_OVERLAY,
      started_at: '2026-11-02T12:02:00Z',
    }
    const createSimSpy = vi.spyOn(api, 'createSimulation').mockResolvedValue(mockSimRun)
    vi.spyOn(api, 'getSimulationEvents').mockResolvedValue({
      data: [
        {
          id: 'ev-1',
          simulation_id: 'sim-run-1',
          sequence: 1,
          tick: 0,
          event_type: 'job_created',
          payload: { item_id: 'minecraft:chest', quantity: 2 },
        },
      ],
      pagination: { page: 1, page_size: 100, total: 1 },
    })
    vi.spyOn(api, 'getSimulationDiagnostics').mockResolvedValue(mockValidation.diagnostics)
    const stepSimSpy = vi.spyOn(api, 'stepSimulation').mockResolvedValue({
      tick: 1,
      events: [
        {
          id: 'ev-2',
          simulation_id: 'sim-run-1',
          sequence: 2,
          tick: 1,
          event_type: 'provider_dispatched',
          payload: { batch: 1 },
        },
      ],
      diagnostics: mockValidation.diagnostics,
      state: { ...mockSimRun, current_tick: 1 },
    })

    renderEditor()
    await screen.findByText('Test AE2 Network')

    // Trigger topology validation
    fireEvent.click(screen.getByRole('button', { name: 'Validate' }))

    const diagButton = await screen.findByTestId('diagnostic-item-DEVICE_NO_CHANNEL')
    expect(validateSpy).toHaveBeenCalledWith('proj-1')
    expect(diagButton).toHaveTextContent('ME Drive at (3, 0, 1) failed to acquire a channel.')

    // Before clicking diagnostic, cell (3, 0, 1) is not highlighted
    const affectedCell = screen.getByTestId('grid-cell-3-0-1')
    expect(affectedCell).toHaveAttribute('data-highlighted', 'false')

    // Click diagnostic row -> highlights affected block coordinates (3, 0, 1)
    fireEvent.click(diagButton)
    expect(useEditorStore.getState().highlightedCoords).toEqual([{ x: 3, y: 0, z: 1 }])
    expect(affectedCell).toHaveAttribute('data-highlighted', 'true')
    expect(affectedCell).toHaveClass('cell-diagnostic-highlight')

    // Start Simulation with Crafting Request & Step +1 tick
    fireEvent.click(screen.getByRole('button', { name: 'Start Simulation' }))
    await waitFor(() => {
      expect(createSimSpy).toHaveBeenCalledWith('proj-1', {
        crafting_requests: [
          { item_id: 'minecraft:chest', quantity: 2, source: 'terminal' },
        ],
      })
    })
    expect(await screen.findByText('TICK 0')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Step +1' }))
    await waitFor(() => {
      expect(stepSimSpy).toHaveBeenCalledWith('sim-run-1', 1)
    })
    expect(await screen.findByText('TICK 1')).toBeInTheDocument()
  })
})

describe('API Root Health, Route Protection, and Acceptance Fixtures', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
    window.localStorage.clear()
    useAuthStore.getState().clearSession()
    useEditorStore.getState().resetWorkspace()
  })

  it('fetches /health at origin root instead of /api/v1/health', async () => {
    const fetchSpy = vi.spyOn(globalThis, 'fetch').mockResolvedValue(
      new Response(
        JSON.stringify({ status: 'ok', version: '0.1.0', timestamp: '2026-11-02T12:00:00Z' }),
        { status: 200, headers: { 'Content-Type': 'application/json' } }
      )
    )

    const res = await api.health()
    expect(res.status).toBe('ok')
    expect(fetchSpy).toHaveBeenCalledTimes(1)
    const requestedUrl = String(fetchSpy.mock.calls[0][0])
    expect(requestedUrl).toMatch(/\/health$/)
    expect(requestedUrl).not.toContain('/api/v1/health')
  })

  it('redirects unauthenticated /projects visits to /login and provisions Valid Autocrafting Fixture after sign-in', async () => {
    vi.spyOn(api, 'health').mockResolvedValue({
      status: 'ok',
      version: '0.1.0',
      timestamp: '2026-11-02T12:00:00Z',
    })
    vi.spyOn(api, 'login').mockImplementation(async (email) => {
      const auth = { user: { id: 'u-1', email }, token: 'jwt-valid' }
      api.setToken(auth.token)
      return auth
    })
    vi.spyOn(api, 'me').mockResolvedValue({ id: 'u-1', email: 'eng@melab.local' })
    vi.spyOn(api, 'listProjects').mockResolvedValue({
      data: [],
      pagination: { page: 1, page_size: 50, total: 0 },
    })
    vi.spyOn(api, 'listProfiles').mockResolvedValue([MOCK_PROFILE])
    vi.spyOn(api, 'listBlockDefinitions').mockResolvedValue(MOCK_CATALOG)
    vi.spyOn(api, 'listBlocks').mockResolvedValue([])
    const createProjSpy = vi.spyOn(api, 'createProject').mockResolvedValue({
      ...MOCK_PROJECT,
      id: 'proj-fixture-1',
      name: 'AE2 1.21.1 Autocrafting Reference Network',
    })
    const batchSpy = vi.spyOn(api, 'batchBlocks').mockResolvedValue({ results: [] })
    vi.spyOn(api, 'getProject').mockResolvedValue({
      ...MOCK_PROJECT,
      id: 'proj-fixture-1',
      name: 'AE2 1.21.1 Autocrafting Reference Network',
    })

    render(
      <MemoryRouter initialEntries={['/projects']}>
        <App />
      </MemoryRouter>
    )

    expect(await screen.findByRole('button', { name: 'Sign In to Workspace' })).toBeInTheDocument()

    fireEvent.change(screen.getByLabelText('Engineer Email'), {
      target: { value: 'eng@melab.local' },
    })
    fireEvent.change(screen.getByLabelText('Password'), {
      target: { value: 'supersecret123' },
    })
    fireEvent.click(screen.getByRole('button', { name: 'Sign In to Workspace' }))

    const fixtureBtn = await screen.findByRole('button', {
      name: 'Load Valid Autocrafting Fixture',
    })
    fireEvent.click(fixtureBtn)

    await waitFor(() => {
      expect(createProjSpy).toHaveBeenCalledWith(
        expect.objectContaining({
          profile_id: 'ae2-1.21.1-default',
        })
      )
      expect(batchSpy).toHaveBeenCalledWith(
        'proj-fixture-1',
        expect.arrayContaining([
          expect.objectContaining({
            operation: 'create',
            data: expect.objectContaining({ block_def_id: 'ae2:controller', x: 0, y: 0, z: 0 }),
          }),
          expect.objectContaining({
            operation: 'create',
            data: expect.objectContaining({
              block_def_id: 'ae2:pattern_provider',
              x: 4,
              y: 0,
              z: 0,
            }),
          }),
        ]),
        true
      )
    })
  })
})
