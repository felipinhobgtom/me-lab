import { useEffect, useMemo, useRef, useState } from 'react'
import { useParams } from 'react-router-dom'
import { api, extractApiErrorMessage, type BlockDefinition } from '@/lib/api'
import {
  CABLE_COLOR_HEX,
  CABLE_COLORS,
  isCableBlockId,
  type CableColor,
} from '@/lib/ae2/constants'
import { BlockInspector } from '../inspector/BlockInspector'
import { SANDBOX_PRESETS } from '../projects/fixtures'
import { SimulationConsole } from '../simulation/SimulationConsole'
import { useEditorStore } from './store'
import { getBlockIconUrl } from './textures'
import { Workspace2D } from './Workspace2D'
import { Workspace3D } from './Workspace3D'
import './EditorLayout.css'

const SANDBOX_STORAGE_KEY = 'melab_sandbox_project_id'

const CATEGORY_TABS = [
  { id: 'all', label: 'All Blocks' },
  { id: 'cables', label: 'Cables' },
  { id: 'network', label: 'Network & Power' },
  { id: 'storage', label: 'Storage & Terminals' },
  { id: 'autocrafting', label: 'Autocrafting' },
  { id: 'machines', label: 'Machines' },
] as const

interface ExportModalState {
  title: string
  filename: string
  mimeType: string
  content: string
}

export function EditorLayout() {
  const { projectId: routeProjectId } = useParams<{ projectId?: string }>()

  const project = useEditorStore((s) => s.project)
  const projectId = useEditorStore((s) => s.projectId)
  const catalog = useEditorStore((s) => s.catalog)
  const blocks = useEditorStore((s) => s.blocks)
  const pastCount = useEditorStore((s) => s.past.length)
  const futureCount = useEditorStore((s) => s.future.length)
  const viewMode = useEditorStore((s) => s.viewMode)
  const activeLayerY = useEditorStore((s) => s.activeLayerY)
  const hotbar = useEditorStore((s) => s.hotbar)
  const activeBlockDefId = useEditorStore((s) => s.activeBlockDefId)
  const activeCableColor = useEditorStore((s) => s.activeCableColor)
  const placementFacing = useEditorStore((s) => s.placementFacing)
  const selectedBlockIds = useEditorStore((s) => s.selectedBlockIds)
  const inspectorOpen = useEditorStore((s) => s.inspectorOpen)
  const simDrawerOpen = useEditorStore((s) => s.simDrawerOpen)
  const overlays = useEditorStore((s) => s.overlays)
  const saveStatus = useEditorStore((s) => s.saveStatus)
  const actionBusy = useEditorStore((s) => s.actionBusy)
  const validationResult = useEditorStore((s) => s.validationResult)
  const simulationRun = useEditorStore((s) => s.simulationRun)
  const diagnostics = useEditorStore((s) => s.diagnostics)

  const hydrateWorkspace = useEditorStore((s) => s.hydrateWorkspace)
  const setViewMode = useEditorStore((s) => s.setViewMode)
  const stepActiveLayerY = useEditorStore((s) => s.stepActiveLayerY)
  const setHotbarSlot = useEditorStore((s) => s.setHotbarSlot)
  const setActiveBlockDefId = useEditorStore((s) => s.setActiveBlockDefId)
  const setActiveCableColor = useEditorStore((s) => s.setActiveCableColor)
  const setInspectorOpen = useEditorStore((s) => s.setInspectorOpen)
  const setSimDrawerOpen = useEditorStore((s) => s.setSimDrawerOpen)
  const toggleOverlay = useEditorStore((s) => s.toggleOverlay)
  const undo = useEditorStore((s) => s.undo)
  const redo = useEditorStore((s) => s.redo)
  const copySelected = useEditorStore((s) => s.copySelected)
  const pasteClipboard = useEditorStore((s) => s.pasteClipboard)
  const duplicateSelected = useEditorStore((s) => s.duplicateSelected)
  const rotateSelectedOrPlacement = useEditorStore((s) => s.rotateSelectedOrPlacement)
  const removeBlocks = useEditorStore((s) => s.removeBlocks)
  const clearAllBlocks = useEditorStore((s) => s.clearAllBlocks)
  const loadPresetOperations = useEditorStore((s) => s.loadPresetOperations)
  const clearHighlights = useEditorStore((s) => s.clearHighlights)
  const saveLayout = useEditorStore((s) => s.saveLayout)
  const runValidation = useEditorStore((s) => s.runValidation)
  const startSimulation = useEditorStore((s) => s.startSimulation)

  const [loading, setLoading] = useState<boolean>(true)
  const [loadError, setLoadError] = useState<string | null>(null)
  const [paletteOpen, setPaletteOpen] = useState<boolean>(false)
  const [paletteQuery, setPaletteQuery] = useState<string>('')
  const [paletteCategory, setPaletteCategory] = useState<string>('all')
  const [targetHotbarSlot, setTargetHotbarSlot] = useState<number>(0)
  const [presetsMenuOpen, setPresetsMenuOpen] = useState<boolean>(false)
  const [exportMenuOpen, setExportMenuOpen] = useState<boolean>(false)
  const [exportModal, setExportModal] = useState<ExportModalState | null>(null)

  const autoSyncTimerRef = useRef<number | null>(null)

  // Initialize Sandbox automatically without login or project dashboard!
  useEffect(() => {
    let cancelled = false

    const initSandbox = async () => {
      setLoading(true)
      setLoadError(null)
      try {
        const profiles = await api.listProfiles()
        const defaultProfile =
          profiles.find((p) => p.id === 'ae2-1.21.1-default') ?? profiles[0] ?? null
        const profileId = defaultProfile?.id ?? 'ae2-1.21.1-default'

        let proj = null
        const targetId = routeProjectId ?? window.localStorage.getItem(SANDBOX_STORAGE_KEY)
        if (targetId) {
          try {
            proj = await api.getProject(targetId)
          } catch {
            proj = null
          }
        }

        if (!proj) {
          proj = await api.createProject({
            name: 'AE2 Sandbox',
            description: 'Interactive 3D AE2 Sandbox',
            profile_id: profileId,
          })
          if (!routeProjectId) {
            window.localStorage.setItem(SANDBOX_STORAGE_KEY, proj.id)
          }
        }

        const [defs, initialBlocks] = await Promise.all([
          api.listBlockDefinitions(proj.profile_id),
          api.listBlocks(proj.id),
        ])

        if (cancelled) return
        const resolvedProfile =
          profiles.find((p) => p.id === proj.profile_id) ?? defaultProfile
        hydrateWorkspace({
          project: proj,
          profile: resolvedProfile,
          catalog: defs,
          blocks: initialBlocks,
        })
      } catch (err) {
        if (!cancelled) {
          setLoadError(extractApiErrorMessage(err))
        }
      } finally {
        if (!cancelled) {
          setLoading(false)
        }
      }
    }

    void initSandbox()
    return () => {
      cancelled = true
    }
  }, [hydrateWorkspace, routeProjectId])

  // Debounced silent SQLite auto-save + live topology validation when blocks change
  useEffect(() => {
    if (saveStatus !== 'unsaved' || !projectId) {
      return
    }
    if (autoSyncTimerRef.current !== null) {
      window.clearTimeout(autoSyncTimerRef.current)
    }
    autoSyncTimerRef.current = window.setTimeout(() => {
      void saveLayout().then((ok) => {
        if (ok && useEditorStore.getState().blocks.length > 0) {
          void runValidation()
        }
      })
    }, 650)

    return () => {
      if (autoSyncTimerRef.current !== null) {
        window.clearTimeout(autoSyncTimerRef.current)
      }
    }
  }, [blocks, projectId, runValidation, saveLayout, saveStatus])

  // Keyboard shortcuts (1-9 hotbar, E block palette, R rotate, X break, C configure, Esc hand/close, Undo/Redo/Copy/Paste)
  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      const target = e.target as HTMLElement | null
      if (
        target &&
        (target.tagName === 'INPUT' ||
          target.tagName === 'TEXTAREA' ||
          target.tagName === 'SELECT' ||
          target.isContentEditable)
      ) {
        if (e.key === 'Escape') {
          target.blur()
        }
        return
      }

      const mod = e.ctrlKey || e.metaKey
      if (mod && e.key.toLowerCase() === 'z') {
        e.preventDefault()
        if (e.shiftKey) redo()
        else undo()
        return
      }
      if (mod && e.key.toLowerCase() === 'y') {
        e.preventDefault()
        redo()
        return
      }
      if (mod && e.key.toLowerCase() === 'c') {
        e.preventDefault()
        copySelected()
        return
      }
      if (mod && e.key.toLowerCase() === 'v') {
        e.preventDefault()
        pasteClipboard()
        return
      }
      if (mod && e.key.toLowerCase() === 'd') {
        e.preventDefault()
        duplicateSelected()
        return
      }
      if (mod && e.key.toLowerCase() === 's') {
        e.preventDefault()
        void saveLayout()
        return
      }

      if (!mod && e.key >= '1' && e.key <= '9') {
        const idx = Number(e.key) - 1
        const defId = hotbar[idx]
        if (defId) {
          setTargetHotbarSlot(idx)
          setActiveBlockDefId(defId)
        }
        return
      }

      if (!mod && (e.key === 'e' || e.key === 'E')) {
        e.preventDefault()
        setPaletteOpen((prev) => !prev)
        return
      }
      if (!mod && (e.key === 'r' || e.key === 'R')) {
        e.preventDefault()
        rotateSelectedOrPlacement()
        return
      }
      if (!mod && (e.key === 'x' || e.key === 'X')) {
        e.preventDefault()
        setActiveBlockDefId(activeBlockDefId === '__break__' ? hotbar[0] ?? null : '__break__')
        return
      }
      if (!mod && (e.key === 'c' || e.key === 'C')) {
        if (selectedBlockIds.length > 0) {
          e.preventDefault()
          setInspectorOpen(!inspectorOpen)
        }
        return
      }
      if (!mod && (e.key === 'Delete' || e.key === 'Backspace')) {
        if (selectedBlockIds.length > 0) {
          e.preventDefault()
          removeBlocks()
        }
        return
      }
      if (e.key === 'Escape') {
        if (paletteOpen) {
          setPaletteOpen(false)
        } else if (inspectorOpen) {
          setInspectorOpen(false)
        } else if (simDrawerOpen) {
          setSimDrawerOpen(false)
        } else {
          setActiveBlockDefId(null)
          clearHighlights()
        }
      }
    }

    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [
    activeBlockDefId,
    clearHighlights,
    copySelected,
    duplicateSelected,
    hotbar,
    inspectorOpen,
    paletteOpen,
    pasteClipboard,
    redo,
    removeBlocks,
    rotateSelectedOrPlacement,
    saveLayout,
    selectedBlockIds.length,
    setActiveBlockDefId,
    setInspectorOpen,
    setSimDrawerOpen,
    simDrawerOpen,
    undo,
  ])

  const catalogById = useMemo(() => {
    const m = new Map<string, BlockDefinition>()
    for (const d of catalog) m.set(d.id, d)
    return m
  }, [catalog])

  const filteredCatalog = useMemo(() => {
    const q = paletteQuery.trim().toLowerCase()
    return catalog.filter((d) => {
      if (d.fidelity === 'unsupported' && !q) {
        return false
      }
      if (paletteCategory !== 'all') {
        if (paletteCategory === 'network') {
          if (d.category !== 'network' && d.category !== 'energy') return false
        } else if (d.category !== paletteCategory) {
          return false
        }
      }
      if (q && !d.name.toLowerCase().includes(q) && !d.id.toLowerCase().includes(q)) {
        return false
      }
      return true
    })
  }, [catalog, paletteCategory, paletteQuery])

  const activeDef = activeBlockDefId ? catalogById.get(activeBlockDefId) : undefined
  const holdingCable = Boolean(
    activeBlockDefId && isCableBlockId(activeBlockDefId, activeDef)
  )

  const activeSummary = simulationRun?.summary ?? validationResult?.summary ?? null
  const errorCount = diagnostics.filter((d) => d.severity === 'error').length

  const handleValidateClick = async () => {
    const res = await runValidation()
    if (res && !res.valid) {
      setSimDrawerOpen(true)
    }
  }

  const handleSimulateClick = async () => {
    setSimDrawerOpen(true)
    if (!simulationRun && blocks.length > 0) {
      await startSimulation()
    }
  }

  const handleExportJson = async () => {
    setExportMenuOpen(false)
    if (!projectId) return
    if (saveStatus === 'unsaved') await saveLayout()
    const bp = await api.exportBlueprint(projectId)
    setExportModal({
      title: 'AE2 Blueprint (JSON)',
      filename: 'ae2-sandbox-blueprint.json',
      mimeType: 'application/json',
      content: JSON.stringify(bp, null, 2),
    })
  }

  const handleExportBuildSheet = async () => {
    setExportMenuOpen(false)
    if (!projectId) return
    if (saveStatus === 'unsaved') await saveLayout()
    const res = await api.exportBuildSheet(projectId)
    setExportModal({
      title: 'Minecraft AE2 Build Sheet',
      filename: 'ae2-build-sheet.md',
      mimeType: 'text/markdown',
      content: res.content,
    })
  }

  const handleLoadPreset = async (presetId: string) => {
    setPresetsMenuOpen(false)
    const preset = SANDBOX_PRESETS.find((p) => p.id === presetId)
    if (!preset) return
    loadPresetOperations(preset.operations)
    const saved = await saveLayout()
    if (saved) {
      const res = await runValidation()
      if (res && !res.valid) {
        setSimDrawerOpen(true)
      }
    }
  }

  const handleDownloadModal = () => {
    if (!exportModal) return
    const blob = new Blob([exportModal.content], { type: exportModal.mimeType })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = exportModal.filename
    a.click()
    URL.revokeObjectURL(url)
  }

  if (loading) {
    return (
      <div className="sandbox-loading">
        <span>Loading AE2 3D Sandbox…</span>
      </div>
    )
  }

  if (loadError) {
    return (
      <div className="sandbox-loading">
        <span>Failed to connect to ME Lab backend: {loadError}</span>
      </div>
    )
  }

  return (
    <div className="sandbox-root" data-project-name={project?.name ?? 'AE2 Sandbox'}>
      {/* Full-bleed 3D Voxel Canvas (or optional 2D Y-Slice Grid) */}
      <main className="sandbox-stage">
        {viewMode === '3d' ? <Workspace3D /> : <Workspace2D />}
      </main>

      {/* Top-Left Minimal Sandbox Toolbar */}
      <header className="hud-pill hud-top-left">
        <span className="hud-brand" title={project?.name ?? 'ME Lab'}>
          ME Lab
        </span>

        <div className="hud-sep" />

        <button
          type="button"
          className="hud-btn"
          onClick={clearAllBlocks}
          disabled={blocks.length === 0}
          title="Clear Sandbox to Empty Grid"
        >
          Clear
        </button>

        <div className="hud-dropdown-wrap">
          <button
            type="button"
            className="hud-btn"
            onClick={() => {
              setPresetsMenuOpen((v) => !v)
              setExportMenuOpen(false)
            }}
          >
            Examples ▾
          </button>
          {presetsMenuOpen && (
            <div className="hud-menu">
              {SANDBOX_PRESETS.map((preset) => (
                <button
                  key={preset.id}
                  type="button"
                  className="hud-menu-item"
                  onClick={() => void handleLoadPreset(preset.id)}
                >
                  <strong>{preset.name}</strong>
                  <span>{preset.description}</span>
                </button>
              ))}
            </div>
          )}
        </div>

        <div className="hud-sep" />

        <button
          type="button"
          className="hud-btn"
          onClick={undo}
          disabled={pastCount === 0}
          title="Undo (Ctrl+Z)"
        >
          Undo
        </button>
        <button
          type="button"
          className="hud-btn"
          onClick={redo}
          disabled={futureCount === 0}
          title="Redo (Ctrl+Y)"
        >
          Redo
        </button>
        <button
          type="button"
          className="hud-btn"
          onClick={rotateSelectedOrPlacement}
          title="Rotate Facing Clockwise (R)"
        >
          Rotate (R)
        </button>

        <div className="hud-sep" />

        <button
          type="button"
          className={`hud-btn ${viewMode === '3d' ? 'active' : ''}`}
          onClick={() => setViewMode(viewMode === '3d' ? '2d' : '3d')}
          title="Switch between 3D Voxel View and 2D Layer Slice"
        >
          {viewMode === '3d' ? '2D Slice' : '3D View'}
        </button>
        {viewMode === '2d' && (
          <>
            <button
              type="button"
              aria-label="Layer Down"
              className="hud-btn"
              onClick={() => stepActiveLayerY(-1)}
            >
              Y−
            </button>
            <span className="hud-mono">Y={activeLayerY}</span>
            <button
              type="button"
              aria-label="Layer Up"
              className="hud-btn"
              onClick={() => stepActiveLayerY(1)}
            >
              Y+
            </button>
          </>
        )}

        <div className="hud-dropdown-wrap">
          <button
            type="button"
            className="hud-btn"
            onClick={() => {
              setExportMenuOpen((v) => !v)
              setPresetsMenuOpen(false)
            }}
          >
            Export ▾
          </button>
          {exportMenuOpen && (
            <div className="hud-menu hud-menu-compact">
              <button
                type="button"
                className="hud-menu-item"
                onClick={() => void handleExportJson()}
              >
                <strong>Export JSON Blueprint</strong>
              </button>
              <button
                type="button"
                className="hud-menu-item"
                onClick={() => void handleExportBuildSheet()}
              >
                <strong>Build Sheet (Markdown)</strong>
              </button>
            </div>
          )}
        </div>

        <span
          className="hud-save-dot"
          data-testid="save-status-badge"
          title={`Persistence: ${saveStatus}`}
        >
          {saveStatus === 'saving'
            ? 'Syncing…'
            : saveStatus === 'unsaved'
              ? 'Unsaved'
              : saveStatus === 'error'
                ? 'Sync Error'
                : 'Saved'}
        </span>
        {saveStatus === 'unsaved' && (
          <button
            type="button"
            className="hud-btn"
            onClick={() => void saveLayout()}
          >
            Save Layout
          </button>
        )}
      </header>

      {/* Top-Right Network Status & Simulation Trigger Pill */}
      <div className="hud-pill hud-top-right">
        {blocks.length === 0 ? (
          <span className="hud-status-text">Empty Sandbox · Click grid to place blocks</span>
        ) : (
          <span className="hud-status-text">
            <span
              className={`status-dot ${
                errorCount > 0
                  ? 'dot-error'
                  : activeSummary && activeSummary.online_networks > 0
                    ? 'dot-ok'
                    : 'dot-idle'
              }`}
            />
            {activeSummary
              ? `${activeSummary.channels_used}/${activeSummary.channels_available} Ch · ${Math.round(
                  activeSummary.energy_consumption
                )}/${Math.round(activeSummary.energy_generation)} AE/t`
              : `${blocks.length} blocks`}
          </span>
        )}

        {selectedBlockIds.length > 0 && (
          <>
            <div className="hud-sep" />
            <button
              type="button"
              className={`hud-btn ${inspectorOpen ? 'active' : ''}`}
              onClick={() => setInspectorOpen(!inspectorOpen)}
              title="Configure selected block (Double-click or C)"
            >
              Configure
            </button>
            <button
              type="button"
              className="hud-btn hud-btn-danger"
              onClick={() => removeBlocks()}
              title="Delete selected block (Del)"
            >
              Delete
            </button>
          </>
        )}

        <div className="hud-sep" />

        <button
          type="button"
          className={`hud-btn ${overlays.channels ? 'active' : ''}`}
          onClick={() => toggleOverlay('channels')}
          title="Toggle 3D Channel Labels"
        >
          Channels
        </button>

        <button
          type="button"
          className="hud-btn"
          disabled={actionBusy}
          onClick={() => void handleValidateClick()}
        >
          Validate
        </button>

        <button
          type="button"
          className={`hud-btn hud-btn-accent ${simDrawerOpen ? 'active' : ''}`}
          disabled={actionBusy}
          onClick={() => void handleSimulateClick()}
        >
          Simulate{errorCount > 0 ? ` (${errorCount})` : ''}
        </button>
      </div>

      {/* Bottom-Center Minecraft Creative Hotbar & Dye Color Picker */}
      <div className="hud-bottom-dock">
        {holdingCable && (
          <div className="cable-color-strip" aria-label="Cable Dye Colors">
            {CABLE_COLORS.slice(0, 10).map((col: CableColor) => (
              <button
                key={col}
                type="button"
                title={`Cable Color: ${col}`}
                className={`color-dot-btn ${activeCableColor === col ? 'selected' : ''}`}
                style={{ backgroundColor: CABLE_COLOR_HEX[col] }}
                onClick={() => setActiveCableColor(col)}
              />
            ))}
          </div>
        )}

        <div className="hotbar-caption">
          {activeBlockDefId === null
            ? 'Inspect Hand — Click to configure; drag blocks on the grid; WASD moves the camera'
            : activeBlockDefId === '__break__'
              ? 'Break Tool — Click any block or cable subpart to remove'
              : `${activeDef?.name ?? activeBlockDefId}${
                  holdingCable ? ` (${activeCableColor})` : ''
                } · Facing: ${placementFacing.toUpperCase()} [R] · Double-click block to configure · Shift+Click to break`}
        </div>

        <div className="hotbar-bar" role="toolbar" aria-label="Block Hotbar">
          {/* Inspect / Configure Hand Tool */}
          <button
            type="button"
            className={`hotbar-slot ${activeBlockDefId === null ? 'active' : ''}`}
            onClick={() => setActiveBlockDefId(null)}
            title="Inspect / Configure Block (Esc)"
          >
            <span className="slot-glyph">✋</span>
            <span className="slot-key">Esc</span>
          </button>

          {/* Break Tool */}
          <button
            type="button"
            className={`hotbar-slot ${activeBlockDefId === '__break__' ? 'active danger' : ''}`}
            onClick={() =>
              setActiveBlockDefId(activeBlockDefId === '__break__' ? hotbar[0] ?? null : '__break__')
            }
            title="Break Block / Subpart (X or Shift+Click)"
          >
            <span className="slot-glyph">✕</span>
            <span className="slot-key">X</span>
          </button>

          <div className="hotbar-divider" />

          {/* 9 Customizable Official AE2 Block Slots */}
          {hotbar.map((defId, idx) => {
            const def = catalogById.get(defId)
            const isSelected = activeBlockDefId === defId
            const iconColor = isCableBlockId(defId, def) ? activeCableColor : 'fluix'
            return (
              <button
                key={`${idx}-${defId}`}
                type="button"
                data-testid={`palette-item-${defId}`}
                className={`hotbar-slot ${isSelected ? 'active' : ''}`}
                onClick={() => {
                  setTargetHotbarSlot(idx)
                  setActiveBlockDefId(defId)
                }}
                title={`${def?.name ?? defId} (Key ${idx + 1})`}
              >
                <img
                  src={getBlockIconUrl(defId, iconColor)}
                  alt={def?.name ?? defId}
                  className="pixel-icon"
                  draggable={false}
                />
                <span className="slot-key">{idx + 1}</span>
              </button>
            )
          })}

          <div className="hotbar-divider" />

          {/* All Blocks Creative Inventory Trigger */}
          <button
            type="button"
            className={`hotbar-slot hotbar-more ${paletteOpen ? 'active' : ''}`}
            onClick={() => setPaletteOpen((v) => !v)}
            title="Open Full AE2 & Minecraft Block Catalog (E)"
          >
            <span className="slot-glyph">⊞</span>
            <span className="slot-key">E</span>
          </button>
        </div>
      </div>

      {/* Minecraft Creative-Style Block Picker Modal ([E]) */}
      {paletteOpen && (
        <div
          className="sandbox-backdrop"
          onClick={() => setPaletteOpen(false)}
        >
          <div
            className="creative-modal"
            role="dialog"
            aria-label="AE2 Block Catalog"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="creative-modal-header">
              <input
                type="search"
                autoFocus
                className="creative-search"
                placeholder="Search AE2 & Minecraft blocks (e.g. Controller, Dense Cable, Furnace, Hopper, Bus)…"
                value={paletteQuery}
                onChange={(e) => setPaletteQuery(e.target.value)}
              />
              <button
                type="button"
                className="hud-btn"
                onClick={() => setPaletteOpen(false)}
              >
                Close (Esc)
              </button>
            </div>

            <div className="creative-tabs">
              {CATEGORY_TABS.map((cat) => (
                <button
                  key={cat.id}
                  type="button"
                  className={`hud-btn ${paletteCategory === cat.id ? 'active' : ''}`}
                  onClick={() => setPaletteCategory(cat.id)}
                >
                  {cat.label}
                </button>
              ))}
            </div>

            <div className="creative-grid">
              {filteredCatalog.map((def) => {
                const iconCol = isCableBlockId(def.id, def) ? activeCableColor : 'fluix'
                return (
                  <button
                    key={def.id}
                    type="button"
                    data-testid={`catalog-card-${def.id}`}
                    className={`creative-item ${activeBlockDefId === def.id ? 'selected' : ''}`}
                    onClick={() => {
                      setHotbarSlot(targetHotbarSlot, def.id)
                      setPaletteOpen(false)
                    }}
                  >
                    <img
                      src={getBlockIconUrl(def.id, iconCol)}
                      alt={def.name}
                      className="pixel-icon-lg"
                      draggable={false}
                    />
                    <div className="creative-item-meta">
                      <span className="creative-item-name">{def.name}</span>
                      <span className="creative-item-sub">
                        {def.metadata.is_cable
                          ? `${def.metadata.channel_capacity} ch cable`
                          : def.metadata.channel_usage > 0
                            ? `${def.metadata.channel_usage} ch`
                            : def.category}
                      </span>
                    </div>
                  </button>
                )
              })}
            </div>
          </div>
        </div>
      )}

      {/* Floating On-Demand Block Configuration Card */}
      {inspectorOpen && (
        <div className="floating-inspector-panel">
          <div className="floating-panel-bar">
            <span>Block Configuration</span>
            <button
              type="button"
              className="hud-btn"
              onClick={() => setInspectorOpen(false)}
            >
              ✕
            </button>
          </div>
          <div className="floating-panel-scroll">
            <BlockInspector />
          </div>
        </div>
      )}

      {/* Floating On-Demand Simulation & Diagnostics Drawer */}
      {simDrawerOpen && (
        <div className="floating-sim-panel">
          <div className="floating-panel-bar">
            <span>AE2 Simulation & Diagnostics</span>
            <button
              type="button"
              className="hud-btn"
              onClick={() => setSimDrawerOpen(false)}
            >
              ✕
            </button>
          </div>
          <div className="floating-sim-body">
            <SimulationConsole />
          </div>
        </div>
      )}

      {/* Export Blueprint / Build Sheet Modal */}
      {exportModal && (
        <div className="sandbox-backdrop" onClick={() => setExportModal(null)}>
          <div
            className="creative-modal"
            role="dialog"
            aria-label={exportModal.title}
            onClick={(e) => e.stopPropagation()}
          >
            <div className="creative-modal-header">
              <strong>{exportModal.title}</strong>
              <div style={{ display: 'flex', gap: '8px' }}>
                <button
                  type="button"
                  className="hud-btn hud-btn-accent"
                  onClick={handleDownloadModal}
                >
                  Download {exportModal.filename}
                </button>
                <button
                  type="button"
                  className="hud-btn"
                  onClick={() => setExportModal(null)}
                >
                  Close
                </button>
              </div>
            </div>
            <pre className="export-pre">{exportModal.content}</pre>
          </div>
        </div>
      )}
    </div>
  )
}
