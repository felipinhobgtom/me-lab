import { create } from 'zustand'
import {
  api,
  extractApiErrorMessage,
  type BatchBlockOperation,
  type BlockDefinition,
  type CompatibilityProfile,
  type Coord3D,
  type CraftingRequest,
  type Diagnostic,
  type Direction,
  type PlacedBlock,
  type Project,
  type SimulationEvent,
  type SimulationRun,
  type TopologyOverlay,
  type ValidationResult,
} from '../../lib/api'
import {
  coordKey,
  defaultConfigForBlock,
  rotateDirectionClockwise,
} from '../../lib/ae2/constants'

export type EditorMode = 'build' | 'configure' | 'validate' | 'simulate' | 'debug'
export type ViewMode = '3d' | '2d'
export type CameraPreset = 'orbit' | 'top' | 'front' | 'side'
export type SaveStatus = 'saved' | 'unsaved' | 'saving' | 'error'

export interface OverlayToggles {
  networks: boolean
  connections: boolean
  channels: boolean
  energy: boolean
  warnings: boolean
}

export interface ClipboardItem {
  block_def_id: string
  dx: number
  dy: number
  dz: number
  facing: Direction
  config: Record<string, unknown>
  label?: string
}

export interface ClipboardState {
  anchor: Coord3D
  items: ClipboardItem[]
  pasteCount: number
}

const MAX_HISTORY = 60

let localIdSeq = 1
function nextLocalBlockId(): string {
  localIdSeq += 1
  return `local-block-${Date.now()}-${localIdSeq}`
}

export interface EditorState {
  projectId: string | null
  project: Project | null
  profile: CompatibilityProfile | null
  catalog: BlockDefinition[]

  blocks: PlacedBlock[]
  past: PlacedBlock[][]
  future: PlacedBlock[][]

  activeMode: EditorMode
  viewMode: ViewMode
  activeLayerY: number
  cameraPreset: CameraPreset

  selectedBlockIds: string[]
  activeBlockDefId: string | null
  placementFacing: Direction
  clipboard: ClipboardState | null

  saveStatus: SaveStatus
  saveError: string | null
  actionBusy: boolean
  actionError: string | null

  validationResult: ValidationResult | null
  simulationRun: SimulationRun | null
  events: SimulationEvent[]
  diagnostics: Diagnostic[]
  selectedDiagnosticId: string | null
  highlightedCoords: Coord3D[]

  overlays: OverlayToggles

  // Workspace initialization
  hydrateWorkspace: (params: {
    project: Project
    profile: CompatibilityProfile | null
    catalog: BlockDefinition[]
    blocks: PlacedBlock[]
  }) => void
  resetWorkspace: () => void

  // Mode & View actions
  setActiveMode: (mode: EditorMode) => void
  setViewMode: (view: ViewMode) => void
  setActiveLayerY: (y: number) => void
  stepActiveLayerY: (delta: number) => void
  setCameraPreset: (preset: CameraPreset) => void
  toggleOverlay: (key: keyof OverlayToggles) => void

  // Palette & Selection actions
  setActiveBlockDefId: (blockDefId: string | null) => void
  setPlacementFacing: (facing: Direction) => void
  selectBlock: (blockId: string | null, multi?: boolean) => void
  clearSelection: () => void

  // Editing actions (Undo/Redo tracked)
  placeBlockAt: (coord: Coord3D, facingOverride?: Direction) => PlacedBlock | null
  removeBlocks: (blockIds?: string[]) => void
  rotateSelectedOrPlacement: () => void
  updateBlockFields: (
    blockId: string,
    patch: {
      facing?: Direction
      label?: string
      config?: Record<string, unknown>
    }
  ) => void
  copySelected: () => void
  pasteClipboard: (targetOrigin?: Coord3D) => void
  duplicateSelected: () => void
  undo: () => void
  redo: () => void

  // Diagnostic highlight actions
  selectDiagnostic: (diagnostic: Diagnostic | null) => void
  clearHighlights: () => void

  // Persistence & Execution actions
  saveLayout: () => Promise<boolean>
  runValidation: () => Promise<ValidationResult | null>
  startSimulation: (
    craftingRequests?: CraftingRequest[],
    autoSteps?: number
  ) => Promise<SimulationRun | null>
  stepSimulation: (ticks?: number) => Promise<void>
  pauseSimulation: () => Promise<void>
  resumeSimulation: () => Promise<void>
  resetSimulation: () => Promise<void>
}

const initialOverlays: OverlayToggles = {
  networks: true,
  connections: true,
  channels: true,
  energy: true,
  warnings: true,
}

export function selectActiveOverlay(state: {
  simulationRun: SimulationRun | null
  validationResult: ValidationResult | null
}): TopologyOverlay | null {
  if (state.simulationRun?.overlay) {
    return state.simulationRun.overlay
  }
  if (state.validationResult?.overlay) {
    return state.validationResult.overlay
  }
  return null
}

export const useEditorStore = create<EditorState>((set, get) => ({
  projectId: null,
  project: null,
  profile: null,
  catalog: [],

  blocks: [],
  past: [],
  future: [],

  activeMode: 'build',
  viewMode: '3d',
  activeLayerY: 0,
  cameraPreset: 'orbit',

  selectedBlockIds: [],
  activeBlockDefId: null,
  placementFacing: 'north',
  clipboard: null,

  saveStatus: 'saved',
  saveError: null,
  actionBusy: false,
  actionError: null,

  validationResult: null,
  simulationRun: null,
  events: [],
  diagnostics: [],
  selectedDiagnosticId: null,
  highlightedCoords: [],

  overlays: { ...initialOverlays },

  hydrateWorkspace: ({ project, profile, catalog, blocks }) => {
    set({
      projectId: project.id,
      project,
      profile,
      catalog,
      blocks: structuredClone(blocks),
      past: [],
      future: [],
      selectedBlockIds: [],
      activeBlockDefId: catalog[0]?.id ?? null,
      placementFacing: 'north',
      saveStatus: 'saved',
      saveError: null,
      actionError: null,
      validationResult: null,
      simulationRun: null,
      events: [],
      diagnostics: [],
      selectedDiagnosticId: null,
      highlightedCoords: [],
    })
  },

  resetWorkspace: () => {
    set({
      projectId: null,
      project: null,
      profile: null,
      catalog: [],
      blocks: [],
      past: [],
      future: [],
      activeMode: 'build',
      viewMode: '3d',
      activeLayerY: 0,
      cameraPreset: 'orbit',
      selectedBlockIds: [],
      activeBlockDefId: null,
      placementFacing: 'north',
      clipboard: null,
      saveStatus: 'saved',
      saveError: null,
      actionBusy: false,
      actionError: null,
      validationResult: null,
      simulationRun: null,
      events: [],
      diagnostics: [],
      selectedDiagnosticId: null,
      highlightedCoords: [],
      overlays: { ...initialOverlays },
    })
  },

  setActiveMode: (activeMode) => set({ activeMode }),
  setViewMode: (viewMode) => set({ viewMode }),
  setActiveLayerY: (activeLayerY) => set({ activeLayerY }),
  stepActiveLayerY: (delta) => set((s) => ({ activeLayerY: s.activeLayerY + delta })),
  setCameraPreset: (cameraPreset) => set({ cameraPreset }),
  toggleOverlay: (key) =>
    set((s) => ({
      overlays: {
        ...s.overlays,
        [key]: !s.overlays[key],
      },
    })),

  setActiveBlockDefId: (activeBlockDefId) => set({ activeBlockDefId }),
  setPlacementFacing: (placementFacing) => set({ placementFacing }),

  selectBlock: (blockId, multi = false) => {
    if (!blockId) {
      set({ selectedBlockIds: [] })
      return
    }
    set((state) => {
      if (!multi) {
        return { selectedBlockIds: [blockId] }
      }
      const exists = state.selectedBlockIds.includes(blockId)
      return {
        selectedBlockIds: exists
          ? state.selectedBlockIds.filter((id) => id !== blockId)
          : [...state.selectedBlockIds, blockId],
      }
    })
  },

  clearSelection: () => set({ selectedBlockIds: [] }),

  placeBlockAt: (coord, facingOverride) => {
    const state = get()
    const defId = state.activeBlockDefId
    if (!defId) {
      return null
    }
    const occupied = state.blocks.some(
      (b) => b.x === coord.x && b.y === coord.y && b.z === coord.z
    )
    if (occupied) {
      return null
    }

    const def = state.catalog.find((d) => d.id === defId)
    const now = new Date().toISOString()
    const created: PlacedBlock = {
      id: nextLocalBlockId(),
      project_id: state.projectId ?? '',
      block_def_id: defId,
      x: coord.x,
      y: coord.y,
      z: coord.z,
      facing: facingOverride ?? state.placementFacing,
      config: defaultConfigForBlock(def, defId),
      created_at: now,
      updated_at: now,
    }

    const nextPast = [...state.past, structuredClone(state.blocks)].slice(-MAX_HISTORY)
    set({
      past: nextPast,
      future: [],
      blocks: [...state.blocks, created],
      selectedBlockIds: [created.id],
      saveStatus: 'unsaved',
      saveError: null,
    })
    return created
  },

  removeBlocks: (blockIds) => {
    const state = get()
    const targetIds = blockIds ?? state.selectedBlockIds
    if (targetIds.length === 0) {
      return
    }
    const idSet = new Set(targetIds)
    const nextBlocks = state.blocks.filter((b) => !idSet.has(b.id))
    if (nextBlocks.length === state.blocks.length) {
      return
    }
    const nextPast = [...state.past, structuredClone(state.blocks)].slice(-MAX_HISTORY)
    set({
      past: nextPast,
      future: [],
      blocks: nextBlocks,
      selectedBlockIds: state.selectedBlockIds.filter((id) => !idSet.has(id)),
      saveStatus: 'unsaved',
      saveError: null,
    })
  },

  rotateSelectedOrPlacement: () => {
    const state = get()
    if (state.selectedBlockIds.length === 0) {
      set({ placementFacing: rotateDirectionClockwise(state.placementFacing) })
      return
    }
    const idSet = new Set(state.selectedBlockIds)
    const nextPast = [...state.past, structuredClone(state.blocks)].slice(-MAX_HISTORY)
    const now = new Date().toISOString()
    const nextBlocks = state.blocks.map((b) =>
      idSet.has(b.id)
        ? {
            ...b,
            facing: rotateDirectionClockwise(b.facing),
            updated_at: now,
          }
        : b
    )
    set({
      past: nextPast,
      future: [],
      blocks: nextBlocks,
      saveStatus: 'unsaved',
      saveError: null,
    })
  },

  updateBlockFields: (blockId, patch) => {
    const state = get()
    const idx = state.blocks.findIndex((b) => b.id === blockId)
    if (idx === -1) {
      return
    }
    const nextPast = [...state.past, structuredClone(state.blocks)].slice(-MAX_HISTORY)
    const target = state.blocks[idx]
    const updated: PlacedBlock = {
      ...target,
      facing: patch.facing !== undefined ? patch.facing : target.facing,
      label: patch.label !== undefined ? patch.label : target.label,
      config: patch.config !== undefined ? structuredClone(patch.config) : target.config,
      updated_at: new Date().toISOString(),
    }
    const nextBlocks = [...state.blocks]
    nextBlocks[idx] = updated
    set({
      past: nextPast,
      future: [],
      blocks: nextBlocks,
      saveStatus: 'unsaved',
      saveError: null,
    })
  },

  copySelected: () => {
    const state = get()
    const selected = state.blocks.filter((b) => state.selectedBlockIds.includes(b.id))
    if (selected.length === 0) {
      return
    }
    const anchor = { x: selected[0].x, y: selected[0].y, z: selected[0].z }
    const items: ClipboardItem[] = selected.map((b) => ({
      block_def_id: b.block_def_id,
      dx: b.x - anchor.x,
      dy: b.y - anchor.y,
      dz: b.z - anchor.z,
      facing: b.facing,
      config: structuredClone(b.config),
      label: b.label,
    }))
    set({
      clipboard: {
        anchor,
        items,
        pasteCount: 0,
      },
    })
  },

  pasteClipboard: (targetOrigin) => {
    const state = get()
    const clip = state.clipboard
    if (!clip || clip.items.length === 0) {
      return
    }

    const occupiedKeys = new Set(state.blocks.map((b) => coordKey(b.x, b.y, b.z)))
    let origin = targetOrigin ?? {
      x: clip.anchor.x + clip.pasteCount + 1,
      y: clip.anchor.y,
      z: clip.anchor.z,
    }

    if (!targetOrigin) {
      let guard = 0
      while (
        guard < 32 &&
        clip.items.some((it) => occupiedKeys.has(coordKey(origin.x + it.dx, origin.y + it.dy, origin.z + it.dz)))
      ) {
        origin = { x: origin.x + 1, y: origin.y, z: origin.z }
        guard += 1
      }
    }

    const now = new Date().toISOString()
    const pasted: PlacedBlock[] = []
    for (const item of clip.items) {
      const x = origin.x + item.dx
      const y = origin.y + item.dy
      const z = origin.z + item.dz
      if (occupiedKeys.has(coordKey(x, y, z))) {
        continue
      }
      occupiedKeys.add(coordKey(x, y, z))
      pasted.push({
        id: nextLocalBlockId(),
        project_id: state.projectId ?? '',
        block_def_id: item.block_def_id,
        x,
        y,
        z,
        facing: item.facing,
        config: structuredClone(item.config),
        label: item.label,
        created_at: now,
        updated_at: now,
      })
    }

    if (pasted.length === 0) {
      return
    }

    const nextPast = [...state.past, structuredClone(state.blocks)].slice(-MAX_HISTORY)
    set({
      past: nextPast,
      future: [],
      blocks: [...state.blocks, ...pasted],
      selectedBlockIds: pasted.map((b) => b.id),
      clipboard: {
        ...clip,
        pasteCount: clip.pasteCount + 1,
      },
      saveStatus: 'unsaved',
      saveError: null,
    })
  },

  duplicateSelected: () => {
    get().copySelected()
    get().pasteClipboard()
  },

  undo: () => {
    const state = get()
    if (state.past.length === 0) {
      return
    }
    const previous = state.past[state.past.length - 1]
    const nextPast = state.past.slice(0, -1)
    const nextFuture = [structuredClone(state.blocks), ...state.future].slice(0, MAX_HISTORY)
    const validIds = new Set(previous.map((b) => b.id))
    set({
      blocks: structuredClone(previous),
      past: nextPast,
      future: nextFuture,
      selectedBlockIds: state.selectedBlockIds.filter((id) => validIds.has(id)),
      saveStatus: 'unsaved',
      saveError: null,
    })
  },

  redo: () => {
    const state = get()
    if (state.future.length === 0) {
      return
    }
    const next = state.future[0]
    const nextFuture = state.future.slice(1)
    const nextPast = [...state.past, structuredClone(state.blocks)].slice(-MAX_HISTORY)
    const validIds = new Set(next.map((b) => b.id))
    set({
      blocks: structuredClone(next),
      past: nextPast,
      future: nextFuture,
      selectedBlockIds: state.selectedBlockIds.filter((id) => validIds.has(id)),
      saveStatus: 'unsaved',
      saveError: null,
    })
  },

  selectDiagnostic: (diagnostic) => {
    if (!diagnostic) {
      set({ selectedDiagnosticId: null, highlightedCoords: [] })
      return
    }
    const coords: Coord3D[] = (diagnostic.affected_blocks ?? []).map((b) => ({
      x: b.x,
      y: b.y,
      z: b.z,
    }))
    set((s) => ({
      selectedDiagnosticId: diagnostic.id,
      highlightedCoords: coords,
      activeLayerY: coords.length > 0 ? coords[0].y : s.activeLayerY,
    }))
  },

  clearHighlights: () => set({ selectedDiagnosticId: null, highlightedCoords: [] }),

  saveLayout: async () => {
    const state = get()
    if (!state.projectId) {
      return false
    }

    const selectedCoords = new Set(
      state.blocks
        .filter((b) => state.selectedBlockIds.includes(b.id))
        .map((b) => coordKey(b.x, b.y, b.z))
    )

    const operations: BatchBlockOperation[] = state.blocks.map((b) => ({
      operation: 'create',
      data: {
        block_def_id: b.block_def_id,
        x: b.x,
        y: b.y,
        z: b.z,
        facing: b.facing,
        config: b.config,
        ...(b.label ? { label: b.label } : {}),
      },
    }))

    set({ saveStatus: 'saving', saveError: null })
    try {
      const response = await api.batchBlocks(state.projectId, operations, true)
      const reconciled = response.results
      const nextSelectedIds = reconciled
        .filter((b) => selectedCoords.has(coordKey(b.x, b.y, b.z)))
        .map((b) => b.id)

      set({
        blocks: structuredClone(reconciled),
        selectedBlockIds: nextSelectedIds,
        saveStatus: 'saved',
        saveError: null,
      })
      return true
    } catch (err) {
      set({
        saveStatus: 'error',
        saveError: extractApiErrorMessage(err, 'Failed to save workspace layout'),
      })
      return false
    }
  },

  runValidation: async () => {
    const state = get()
    if (!state.projectId) {
      return null
    }
    if (state.saveStatus === 'unsaved' || state.saveStatus === 'error') {
      const saved = await get().saveLayout()
      if (!saved) {
        return null
      }
    }

    set({ actionBusy: true, actionError: null })
    try {
      const result = await api.validateProject(state.projectId)
      set({
        validationResult: result,
        diagnostics: result.diagnostics ?? [],
        selectedDiagnosticId: null,
        highlightedCoords: [],
        activeMode: 'validate',
        actionBusy: false,
      })
      return result
    } catch (err) {
      set({
        actionBusy: false,
        actionError: extractApiErrorMessage(err, 'Validation failed'),
      })
      return null
    }
  },

  startSimulation: async (craftingRequests, autoSteps) => {
    const state = get()
    if (!state.projectId) {
      return null
    }
    if (state.saveStatus === 'unsaved' || state.saveStatus === 'error') {
      const saved = await get().saveLayout()
      if (!saved) {
        return null
      }
    }

    set({ actionBusy: true, actionError: null })
    try {
      const run = await api.createSimulation(state.projectId, {
        ...(craftingRequests && craftingRequests.length > 0
          ? { crafting_requests: craftingRequests }
          : {}),
        ...(autoSteps !== undefined ? { auto_steps: autoSteps } : {}),
      })
      const [eventsRes, diagsRes] = await Promise.all([
        api
          .getSimulationEvents(run.id, 1, 100)
          .catch(() => ({ data: [] as SimulationEvent[], pagination: { page: 1, page_size: 100, total: 0 } })),
        api.getSimulationDiagnostics(run.id).catch(() => [] as Diagnostic[]),
      ])
      set({
        simulationRun: run,
        events: eventsRes.data ?? [],
        diagnostics: diagsRes,
        selectedDiagnosticId: null,
        highlightedCoords: [],
        activeMode: 'simulate',
        actionBusy: false,
      })
      return run
    } catch (err) {
      set({
        actionBusy: false,
        actionError: extractApiErrorMessage(err, 'Failed to start simulation'),
      })
      return null
    }
  },

  stepSimulation: async (ticks = 1) => {
    const sim = get().simulationRun
    if (!sim) {
      return
    }
    set({ actionBusy: true, actionError: null })
    try {
      const stepRes = await api.stepSimulation(sim.id, ticks)
      set((s) => {
        const existingSeqs = new Set(s.events.map((e) => `${e.tick}:${e.sequence}:${e.id}`))
        const appended = (stepRes.events ?? []).filter(
          (e) => !existingSeqs.has(`${e.tick}:${e.sequence}:${e.id}`)
        )
        return {
          simulationRun: stepRes.state,
          events: [...s.events, ...appended],
          diagnostics: stepRes.diagnostics ?? s.diagnostics,
          actionBusy: false,
        }
      })
    } catch (err) {
      set({
        actionBusy: false,
        actionError: extractApiErrorMessage(err, 'Simulation step failed'),
      })
    }
  },

  pauseSimulation: async () => {
    const sim = get().simulationRun
    if (!sim) {
      return
    }
    set({ actionBusy: true, actionError: null })
    try {
      const updated = await api.pauseSimulation(sim.id)
      set({ simulationRun: updated, actionBusy: false })
    } catch (err) {
      set({
        actionBusy: false,
        actionError: extractApiErrorMessage(err, 'Pause simulation failed'),
      })
    }
  },

  resumeSimulation: async () => {
    const sim = get().simulationRun
    if (!sim) {
      return
    }
    set({ actionBusy: true, actionError: null })
    try {
      const updated = await api.resumeSimulation(sim.id)
      set({ simulationRun: updated, actionBusy: false })
    } catch (err) {
      set({
        actionBusy: false,
        actionError: extractApiErrorMessage(err, 'Resume simulation failed'),
      })
    }
  },

  resetSimulation: async () => {
    const sim = get().simulationRun
    if (!sim) {
      return
    }
    set({ actionBusy: true, actionError: null })
    try {
      const resetRun = await api.resetSimulation(sim.id)
      const [eventsRes, diagsRes] = await Promise.all([
        api
          .getSimulationEvents(resetRun.id, 1, 100)
          .catch(() => ({ data: [] as SimulationEvent[], pagination: { page: 1, page_size: 100, total: 0 } })),
        api.getSimulationDiagnostics(resetRun.id).catch(() => [] as Diagnostic[]),
      ])
      set({
        simulationRun: resetRun,
        events: eventsRes.data ?? [],
        diagnostics: diagsRes,
        selectedDiagnosticId: null,
        highlightedCoords: [],
        actionBusy: false,
      })
    } catch (err) {
      set({
        actionBusy: false,
        actionError: extractApiErrorMessage(err, 'Reset simulation failed'),
      })
    }
  },
}))
