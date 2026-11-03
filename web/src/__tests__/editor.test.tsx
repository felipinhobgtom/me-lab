import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import App from '../App'
import { api } from '../lib/api'
import type {
  BlockDefinition,
  CompatibilityProfile,
  PlacedBlock,
  Project,
  SimulationRun,
  SimulationStepResult,
  TopologyOverlay,
} from '../lib/api'
import { useEditorStore } from '../features/editor/store'
import { SANDBOX_PRESETS } from '../features/projects/fixtures'

// Test stub: the UI never reads the engine-side profile numbers, so an empty object is enough here.
const stubProfileConfig = {} as CompatibilityProfile['config']

const project: Project = {
  id: 'proj-1',
  user_id: 'sandbox',
  name: 'AE2 Sandbox',
  description: '',
  profile_id: 'ae2-1.21.1-default',
  settings: {},
  created_at: '2026-11-02T00:00:00Z',
  updated_at: '2026-11-02T00:00:00Z',
}

function def(id: string, name: string, category: string, extra: Partial<BlockDefinition['metadata']> = {}): BlockDefinition {
  return {
    id,
    profile_id: 'ae2-1.21.1-default',
    name,
    category,
    behavior_key: id,
    fidelity: 'exact',
    doc_reference: 'https://guide.appliedenergistics.org/1.21.1/',
    metadata: {
      channel_usage: 0,
      channel_capacity: 0,
      transmits_channels: false,
      is_cable: false,
      is_smart_cable: false,
      is_dense_cable: false,
      energy_cost: 0,
      energy_capacity: 0,
      energy_generation: 0,
      rotatable: true,
      description: '',
      connection_rules: { connect_all_faces: true },
      visual: { color: '#888', shape: 'cube', size: [1, 1, 1] },
      ...extra,
    },
  } as BlockDefinition
}

const driveDefinition = def('ae2:drive', 'ME Drive', 'storage')
driveDefinition.behavior_key = 'drive'

const catalog: BlockDefinition[] = [
  def('ae2:controller', 'ME Controller', 'network'),
  def('ae2:smart_cable', 'ME Smart Cable', 'cables', {
    is_cable: true,
    is_smart_cable: true,
    channel_capacity: 8,
    transmits_channels: true,
  }),
  def('ae2:crafting_terminal', 'ME Crafting Terminal', 'storage', { channel_usage: 1 }),
  driveDefinition,
  def('minecraft:furnace', 'Furnace', 'machines'),
  def('minecraft:chest', 'Chest', 'machines'),
]
function mockBackend(initialBlocks: PlacedBlock[] = []) {
  vi.spyOn(api, 'listProfiles').mockResolvedValue([
    {
      id: 'ae2-1.21.1-default',
      name: 'AE2 1.21.1',
      minecraft_version: '1.21.1',
      ae2_version: '19.2',
      channel_mode: 'default',
      config: stubProfileConfig,
    },
  ])
  const getProject = vi.spyOn(api, 'getProject').mockResolvedValue(project)
  const createProject = vi.spyOn(api, 'createProject').mockResolvedValue(project)
  vi.spyOn(api, 'listBlockDefinitions').mockResolvedValue(catalog)
  vi.spyOn(api, 'listBlocks').mockResolvedValue(initialBlocks)
  const batch = vi.spyOn(api, 'batchBlocks').mockImplementation(async (_projectId, operations) => ({
    results: operations.map((operation, index) => ({
      id: `saved-${index}`,
      project_id: project.id,
      block_def_id: operation.data?.block_def_id ?? 'ae2:controller',
      x: operation.data?.x ?? 0,
      y: operation.data?.y ?? 0,
      z: operation.data?.z ?? 0,
      facing: operation.data?.facing ?? 'south',
      config: operation.data?.config ?? {},
      created_at: '2026-01-01T00:00:00Z',
      updated_at: '2026-01-01T00:00:00Z',
    })),
  }))
  const overlay: TopologyOverlay = {
    networks: [],
    cables: [],
    device_routes: [],
    storage_cells: [],
    stored_totals: [],
    crafting_jobs: [],
    automation_buses: [],
    external_inventories: [],
  }
  const runtime: SimulationRun = {
    id: 'runtime-1',
    project_id: project.id,
    status: 'running',
    seed: 1,
    current_tick: 0,
    summary: {
      networks: 0,
      online_networks: 0,
      channels_used: 0,
      channels_available: 0,
      energy_stored: 0,
      energy_capacity: 0,
      energy_generation: 0,
      energy_consumption: 0,
      items_stored: 0,
      storage_bytes_used: 0,
      storage_capacity: 0,
      active_jobs: 0,
      completed_jobs: 0,
      failed_jobs: 0,
      diagnostics_count: 0,
    },
    overlay,
    started_at: '2026-01-01T00:00:00Z',
  }
  const createSimulation = vi.spyOn(api, 'createSimulation').mockResolvedValue(runtime)
  vi.spyOn(api, 'getSimulationEvents').mockResolvedValue({
    data: [],
    pagination: { page: 1, page_size: 100, total: 0 },
  })
  vi.spyOn(api, 'getSimulationDiagnostics').mockResolvedValue([])
  const stepSimulation = vi.spyOn(api, 'stepSimulation').mockImplementation(
    async (_simulationId, ticks = 1): Promise<SimulationStepResult> => ({
      tick: ticks,
      events: [],
      diagnostics: [],
      state: { ...runtime, current_tick: ticks },
    }),
  )
  return { getProject, createProject, batch, createSimulation, stepSimulation }
}

function renderApp() {
  return render(
    <MemoryRouter initialEntries={['/']}>
      <App />
    </MemoryRouter>
  )
}

describe('ME Lab zero-login AE2 sandbox', () => {
  beforeEach(() => {
    vi.restoreAllMocks()
    window.localStorage.clear()
    useEditorStore.getState().resetWorkspace()
  })

  it('opens straight into an empty sandbox with no login or project selection', async () => {
    const { createProject } = mockBackend()
    renderApp()

    await screen.findByText(/Empty Sandbox/i)
    expect(createProject).toHaveBeenCalledTimes(1)
    expect(screen.queryByText(/Sign In/i)).toBeNull()
    expect(screen.queryByText(/Workspace Projects/i)).toBeNull()
    expect(useEditorStore.getState().blocks).toHaveLength(0)
    expect(window.localStorage.getItem('melab_sandbox_project_id')).toBe('proj-1')
  })

  it('opens a Minecraft-style creative inventory with Vanilla and AE2 sections', async () => {
    mockBackend()
    renderApp()
    await screen.findByText(/Empty Sandbox/i)

    fireEvent.keyDown(window, { key: 'e' })
    expect(screen.getByRole('dialog', { name: 'Creative Inventory' })).toBeInTheDocument()
    expect(screen.getByRole('tab', { name: 'Minecraft' })).toBeInTheDocument()
    expect(screen.getByRole('tab', { name: 'AE2' })).toBeInTheDocument()
    expect(screen.getByText('Minecraft Vanilla')).toBeInTheDocument()
    expect(screen.getByText('Applied Energistics 2')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('tab', { name: 'Minecraft' }))
    fireEvent.click(screen.getByRole('tab', { name: 'Functional' }))
    fireEvent.click(screen.getByRole('button', { name: 'Furnace' }))

    expect(useEditorStore.getState().activeBlockDefId).toBe('minecraft:furnace')
    expect(screen.queryByRole('dialog', { name: 'Creative Inventory' })).toBeNull()
  })

  it('starts and advances the ME runtime without manual execution controls', async () => {
    const { createSimulation, stepSimulation } = mockBackend()
    renderApp()
    await screen.findByText(/Empty Sandbox/i)

    const state = useEditorStore.getState()
    state.setActiveBlockDefId('ae2:controller')
    state.placeBlockAt({ x: 0, y: 0, z: 0 }, 'south')

    await waitFor(() => expect(createSimulation).toHaveBeenCalledTimes(1), { timeout: 2000 })
    await waitFor(() => expect(stepSimulation).toHaveBeenCalled(), { timeout: 2000 })
    expect(screen.getByText('ME NETWORK RUNTIME')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /simulate/i })).toBeNull()
    expect(screen.queryByRole('button', { name: /validate/i })).toBeNull()
    expect(screen.queryByRole('button', { name: /start simulation/i })).toBeNull()
    expect(screen.queryByRole('button', { name: /step/i })).toBeNull()
  })

  it('opens a block context menu with properties and removal actions', async () => {
    mockBackend()
    renderApp()
    await screen.findByText(/Empty Sandbox/i)

    const state = useEditorStore.getState()
    state.setViewMode('2d')
    state.setActiveBlockDefId('ae2:controller')
    const placed = state.placeBlockAt({ x: 0, y: 0, z: 0 }, 'south')!
    expect(useEditorStore.getState().blocks).toHaveLength(1)
    await waitFor(() => expect(screen.getByTestId('grid-cell-0-0-0')).toHaveClass('selected'))

    fireEvent.contextMenu(screen.getByTestId('grid-cell-0-0-0'), {
      clientX: 120,
      clientY: 140,
    })
    await waitFor(() =>
      expect(screen.getByRole('menu')).toHaveAccessibleName('Actions for ME Controller')
    )

    fireEvent.click(screen.getByRole('menuitem', { name: 'Properties' }))
    expect(screen.getByText('Block Configuration')).toBeInTheDocument()

    fireEvent.contextMenu(screen.getByTestId('grid-cell-0-0-0'), {
      clientX: 120,
      clientY: 140,
    })
    await waitFor(() => expect(screen.getByRole('menu')).toBeInTheDocument())
    fireEvent.click(screen.getByRole('menuitem', { name: 'Remove' }))
    expect(useEditorStore.getState().blocks).toHaveLength(0)
    expect(useEditorStore.getState().selectedBlockIds).not.toContain(placed.id)
  })

  it('opens ME Drive storage controls and adds real storage cells from context or double click', async () => {
    mockBackend()
    renderApp()
    await screen.findByText(/Empty Sandbox/i)

    const state = useEditorStore.getState()
    state.setViewMode('2d')
    state.setActiveBlockDefId('ae2:drive')
    const placed = state.placeBlockAt({ x: 0, y: 0, z: 0 }, 'south')!
    expect(placed.config.cells).toEqual([])
    await waitFor(() => expect(screen.getByTestId('grid-cell-0-0-0')).toHaveClass('selected'))

    fireEvent.contextMenu(screen.getByTestId('grid-cell-0-0-0'), {
      clientX: 120,
      clientY: 140,
    })
    await waitFor(() =>
      expect(screen.getByRole('menuitem', { name: 'Add Storage Cell' })).toBeInTheDocument()
    )
    fireEvent.click(screen.getByRole('menuitem', { name: 'Add Storage Cell' }))
    expect(useEditorStore.getState().blocks[0].config.cells).toHaveLength(1)
    expect(screen.getByText('ME Drive & Storage Cells')).toBeInTheDocument()

    state.updateBlockFields(placed.id, {
      config: {
        priority: 0,
        cells: [
          {
            tier: '4k',
            partition: [],
            inverter: false,
            items: [{ item_id: 'minecraft:iron_ingot', quantity: 64 }],
          },
        ],
      },
    })
    expect(state.removeStorageCell(placed.id, 0)).toBe(false)
    state.updateBlockFields(placed.id, {
      config: {
        priority: 0,
        cells: [{ tier: '4k', partition: [], inverter: false, items: [] }],
      },
    })
    expect(state.removeStorageCell(placed.id, 0)).toBe(true)

    state.addStorageCell(placed.id)
    fireEvent.doubleClick(screen.getByTestId('grid-cell-0-0-0'))
    expect(screen.getByText('ME Drive & Storage Cells')).toBeInTheDocument()
  })

  it('reuses the remembered sandbox project instead of creating a new one', async () => {
    window.localStorage.setItem('melab_sandbox_project_id', 'proj-1')
    const { createProject, getProject } = mockBackend()
    renderApp()

    await screen.findByText(/Empty Sandbox/i)
    expect(getProject).toHaveBeenCalledWith('proj-1')
    expect(createProject).not.toHaveBeenCalled()
  })

  it('attaches a terminal onto a smart cable face as a subpart, not as a separate block', async () => {
    mockBackend()
    renderApp()
    await screen.findByText(/Empty Sandbox/i)

    const state = useEditorStore.getState()
    state.setActiveBlockDefId('ae2:smart_cable')
    const cable = state.placeBlockAt({ x: 0, y: 0, z: 0 }, 'south')
    expect(cable).not.toBeNull()

    const ok = useEditorStore.getState().attachSubpartToCable(cable!.id, 'south', 'ae2:crafting_terminal')
    expect(ok).toBe(true)

    const blocks = useEditorStore.getState().blocks
    expect(blocks).toHaveLength(1)
    expect(blocks[0].config.subparts).toEqual({ south: 'ae2:crafting_terminal' })
  })

  it('refuses to attach subparts to dense cables', async () => {
    mockBackend()
    renderApp()
    await screen.findByText(/Empty Sandbox/i)

    const dense: BlockDefinition = def('ae2:dense_smart_cable', 'Dense', 'cables', {
      is_cable: true,
      is_dense_cable: true,
    })
    useEditorStore.setState({ catalog: [...catalog, dense] })
    const s = useEditorStore.getState()
    s.setActiveBlockDefId('ae2:dense_smart_cable')
    const placed = s.placeBlockAt({ x: 0, y: 0, z: 0 }, 'south')

    expect(useEditorStore.getState().attachSubpartToCable(placed!.id, 'up', 'ae2:terminal')).toBe(false)
  })

  it('clears the grid and restores it with undo', async () => {
    mockBackend()
    renderApp()
    await screen.findByText(/Empty Sandbox/i)

    const s = useEditorStore.getState()
    s.setActiveBlockDefId('ae2:controller')
    s.placeBlockAt({ x: 0, y: 0, z: 0 }, 'south')
    s.placeBlockAt({ x: 1, y: 0, z: 0 }, 'south')
    expect(useEditorStore.getState().blocks).toHaveLength(2)

    useEditorStore.getState().clearAllBlocks()
    expect(useEditorStore.getState().blocks).toHaveLength(0)

    useEditorStore.getState().undo()
    expect(useEditorStore.getState().blocks).toHaveLength(2)
  })

  it('moves blocks with collision checks and keeps the operation undoable', async () => {
    mockBackend()
    renderApp()
    await screen.findByText(/Empty Sandbox/i)

    const s = useEditorStore.getState()
    s.setActiveBlockDefId('ae2:controller')
    const first = s.placeBlockAt({ x: 0, y: 0, z: 0 }, 'south')!
    const second = s.placeBlockAt({ x: 2, y: 0, z: 0 }, 'south')!

    expect(useEditorStore.getState().moveBlocks([first.id], { x: 2, y: 0, z: 0 })).toBe(false)
    expect(useEditorStore.getState().blocks.find((b) => b.id === first.id)?.x).toBe(0)

    expect(useEditorStore.getState().moveBlocks([first.id], { x: 1, y: 0, z: 0 })).toBe(true)
    expect(useEditorStore.getState().blocks.find((b) => b.id === first.id)?.x).toBe(1)
    expect(useEditorStore.getState().blocks.find((b) => b.id === second.id)?.x).toBe(2)

    useEditorStore.getState().undo()
    expect(useEditorStore.getState().blocks.find((b) => b.id === first.id)?.x).toBe(0)
  })

  it('rotates a Minecraft chest through the shared editor action', async () => {
    mockBackend()
    renderApp()
    await screen.findByText(/Empty Sandbox/i)

    const chest = def('minecraft:chest', 'Chest', 'storage')
    useEditorStore.setState({ catalog: [...catalog, chest] })
    const s = useEditorStore.getState()
    s.setActiveBlockDefId('minecraft:chest')
    const placed = s.placeBlockAt({ x: 0, y: 0, z: 0 }, 'south')!
    s.selectBlock(placed.id, false)
    s.rotateSelectedOrPlacement()

    expect(useEditorStore.getState().blocks[0].facing).toBe('west')
  })

  it('loads the guide showcase preset and auto-saves it through the API', async () => {
    const { batch } = mockBackend()
    vi.spyOn(api, 'validateProject').mockResolvedValue({
      valid: true,
      summary: {} as never,
      diagnostics: [],
      overlay: {} as never,
    })
    renderApp()
    await screen.findByText(/Empty Sandbox/i)

    fireEvent.click(screen.getByRole('button', { name: /Examples/i }))
    fireEvent.click(await screen.findByText('AE2 Complete Base Setup'))

    const showcase = SANDBOX_PRESETS.find((p) => p.id === 'guide-showcase')!
    await waitFor(() =>
      expect(useEditorStore.getState().blocks).toHaveLength(showcase.operations.length)
    )
    await waitFor(() => expect(batch).toHaveBeenCalled())
    expect(batch.mock.calls[0][2]).toBe(true)
  })

  it('only lists real AE2 / Minecraft block ids in presets', () => {
    for (const preset of SANDBOX_PRESETS) {
      for (const op of preset.operations) {
        const id = op.data && 'block_def_id' in op.data ? op.data.block_def_id : ''
        expect(id).toMatch(/^(ae2|minecraft):/)
      }
    }
  })
})
