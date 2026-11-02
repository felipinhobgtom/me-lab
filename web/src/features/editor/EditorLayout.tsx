import { useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import {
  api,
  extractApiErrorMessage,
  type CompatibilityProfile,
} from '../../lib/api'
import { BlockPalette } from '../catalog/BlockPalette'
import { BlockInspector } from '../inspector/BlockInspector'
import { SimulationConsole } from '../simulation/SimulationConsole'
import {
  type CameraPreset,
  type EditorMode,
  type OverlayToggles,
  useEditorStore,
} from './store'
import { Workspace2D } from './Workspace2D'
import { Workspace3D } from './Workspace3D'
import './EditorLayout.css'

const MODES: Array<{ id: EditorMode; label: string }> = [
  { id: 'build', label: 'Build' },
  { id: 'configure', label: 'Configure' },
  { id: 'validate', label: 'Validate' },
  { id: 'simulate', label: 'Simulate' },
  { id: 'debug', label: 'Debug' },
]

const CAMERA_PRESETS: Array<{ id: CameraPreset; label: string }> = [
  { id: 'orbit', label: 'Orbit' },
  { id: 'top', label: 'Top' },
  { id: 'front', label: 'Front' },
  { id: 'side', label: 'Side' },
]

const OVERLAY_KEYS: Array<{ key: keyof OverlayToggles; label: string }> = [
  { key: 'networks', label: 'Networks' },
  { key: 'connections', label: 'Connections' },
  { key: 'channels', label: 'Channels' },
  { key: 'energy', label: 'Energy' },
  { key: 'warnings', label: 'Warnings' },
]

interface ExportModalData {
  title: string
  filename: string
  mimeType: string
  content: string
}

export function EditorLayout() {
  const { projectId } = useParams<{ projectId: string }>()
  const navigate = useNavigate()

  const project = useEditorStore((s) => s.project)
  const profile = useEditorStore((s) => s.profile)
  const pastCount = useEditorStore((s) => s.past.length)
  const futureCount = useEditorStore((s) => s.future.length)
  const selectedCount = useEditorStore((s) => s.selectedBlockIds.length)
  const hasClipboard = useEditorStore((s) => Boolean(s.clipboard?.items.length))
  const activeMode = useEditorStore((s) => s.activeMode)
  const viewMode = useEditorStore((s) => s.viewMode)
  const activeLayerY = useEditorStore((s) => s.activeLayerY)
  const cameraPreset = useEditorStore((s) => s.cameraPreset)
  const overlays = useEditorStore((s) => s.overlays)
  const saveStatus = useEditorStore((s) => s.saveStatus)
  const saveError = useEditorStore((s) => s.saveError)
  const actionBusy = useEditorStore((s) => s.actionBusy)

  const hydrateWorkspace = useEditorStore((s) => s.hydrateWorkspace)
  const resetWorkspace = useEditorStore((s) => s.resetWorkspace)
  const setActiveMode = useEditorStore((s) => s.setActiveMode)
  const setViewMode = useEditorStore((s) => s.setViewMode)
  const stepActiveLayerY = useEditorStore((s) => s.stepActiveLayerY)
  const setActiveLayerY = useEditorStore((s) => s.setActiveLayerY)
  const setCameraPreset = useEditorStore((s) => s.setCameraPreset)
  const toggleOverlay = useEditorStore((s) => s.toggleOverlay)
  const setActiveBlockDefId = useEditorStore((s) => s.setActiveBlockDefId)
  const clearHighlights = useEditorStore((s) => s.clearHighlights)

  const undo = useEditorStore((s) => s.undo)
  const redo = useEditorStore((s) => s.redo)
  const copySelected = useEditorStore((s) => s.copySelected)
  const pasteClipboard = useEditorStore((s) => s.pasteClipboard)
  const duplicateSelected = useEditorStore((s) => s.duplicateSelected)
  const rotateSelectedOrPlacement = useEditorStore((s) => s.rotateSelectedOrPlacement)
  const removeBlocks = useEditorStore((s) => s.removeBlocks)
  const saveLayout = useEditorStore((s) => s.saveLayout)
  const runValidation = useEditorStore((s) => s.runValidation)
  const startSimulation = useEditorStore((s) => s.startSimulation)

  const [loading, setLoading] = useState<boolean>(true)
  const [loadError, setLoadError] = useState<string | null>(null)
  const [exportModal, setExportModal] = useState<ExportModalData | null>(null)
  const [exportBusy, setExportBusy] = useState<boolean>(false)
  const [copiedBanner, setCopiedBanner] = useState<boolean>(false)

  useEffect(() => {
    if (!projectId) {
      return
    }
    let mounted = true
    setLoading(true)
    setLoadError(null)

    ;(async () => {
      try {
        const fetchedProject = await api.getProject(projectId)
        const [profiles, catalog, blocks] = await Promise.all([
          api.listProfiles().catch(() => [] as CompatibilityProfile[]),
          api.listBlockDefinitions(fetchedProject.profile_id),
          api.listBlocks(projectId),
        ])
        if (!mounted) {
          return
        }
        const matchedProfile =
          profiles.find((p) => p.id === fetchedProject.profile_id) ?? null
        hydrateWorkspace({
          project: fetchedProject,
          profile: matchedProfile,
          catalog,
          blocks,
        })
      } catch (err) {
        if (mounted) {
          setLoadError(extractApiErrorMessage(err, 'Failed to load workspace project'))
        }
      } finally {
        if (mounted) {
          setLoading(false)
        }
      }
    })()

    return () => {
      mounted = false
      resetWorkspace()
    }
  }, [hydrateWorkspace, projectId, resetWorkspace])

  // CAD Keyboard Shortcuts
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      const target = e.target
      if (
        target instanceof HTMLInputElement ||
        target instanceof HTMLTextAreaElement ||
        target instanceof HTMLSelectElement
      ) {
        return
      }

      const mod = e.ctrlKey || e.metaKey
      if (mod && e.key.toLowerCase() === 's') {
        e.preventDefault()
        void saveLayout()
        return
      }
      if (mod && e.key.toLowerCase() === 'z') {
        e.preventDefault()
        if (e.shiftKey) {
          redo()
        } else {
          undo()
        }
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
      if (!mod && (e.key === 'r' || e.key === 'R')) {
        e.preventDefault()
        rotateSelectedOrPlacement()
        return
      }
      if (!mod && (e.key === 'Delete' || e.key === 'Backspace')) {
        e.preventDefault()
        removeBlocks()
        return
      }
      if (e.key === 'Escape') {
        setActiveBlockDefId(null)
        clearHighlights()
      }
    }

    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [
    clearHighlights,
    copySelected,
    duplicateSelected,
    pasteClipboard,
    redo,
    removeBlocks,
    rotateSelectedOrPlacement,
    saveLayout,
    setActiveBlockDefId,
    undo,
  ])

  const handleExportJson = async () => {
    if (!projectId) return
    setExportBusy(true)
    try {
      if (saveStatus === 'unsaved') {
        await saveLayout()
      }
      const bp = await api.exportBlueprint(projectId)
      setExportModal({
        title: 'AE2 Blueprint JSON Export',
        filename: `${(project?.name ?? 'melab-blueprint').toLowerCase().replace(/[^a-z0-9]+/g, '-')}.json`,
        mimeType: 'application/json',
        content: JSON.stringify(bp, null, 2),
      })
    } finally {
      setExportBusy(false)
    }
  }

  const handleExportBuildSheet = async () => {
    if (!projectId) return
    setExportBusy(true)
    try {
      if (saveStatus === 'unsaved') {
        await saveLayout()
      }
      const res = await api.exportBuildSheet(projectId)
      setExportModal({
        title: 'Minecraft Survival Build Sheet (Markdown)',
        filename: `${(project?.name ?? 'melab-build-sheet').toLowerCase().replace(/[^a-z0-9]+/g, '-')}.md`,
        mimeType: 'text/markdown',
        content: res.content,
      })
    } finally {
      setExportBusy(false)
    }
  }

  const handleDownloadExport = () => {
    if (!exportModal) return
    const blob = new Blob([exportModal.content], { type: exportModal.mimeType })
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = exportModal.filename
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
    URL.revokeObjectURL(url)
  }

  const handleCopyExport = async () => {
    if (!exportModal || !navigator.clipboard) return
    await navigator.clipboard.writeText(exportModal.content)
    setCopiedBanner(true)
    setTimeout(() => setCopiedBanner(false), 1800)
  }

  if (loading) {
    return (
      <div className="editor-loading-screen">
        <span className="badge">LOADING WORKSPACE</span>
        <p>Hydrating AE2 1.21.1 block catalog &amp; topology…</p>
      </div>
    )
  }

  if (loadError) {
    return (
      <div className="editor-loading-screen">
        <span className="badge badge-unsupported">WORKSPACE ERROR</span>
        <p>{loadError}</p>
        <button type="button" className="btn btn-primary" onClick={() => navigate('/projects')}>
          ← Back to Projects
        </button>
      </div>
    )
  }

  const saveLabel =
    saveStatus === 'saved'
      ? 'Saved'
      : saveStatus === 'saving'
        ? 'Saving…'
        : saveStatus === 'error'
          ? 'Save Error'
          : 'Unsaved'

  return (
    <div className="editor-shell">
      {/* Primary CAD Top Bar */}
      <header className="editor-topbar">
        <div className="topbar-section">
          <button type="button" className="btn" onClick={() => navigate('/projects')}>
            ← Projects
          </button>
          <span className="project-title-text" title={project?.name}>
            {project?.name ?? `Project ${projectId}`}
          </span>
          <span className="badge">
            {profile
              ? `${profile.name} (${profile.minecraft_version})`
              : project?.profile_id ?? 'ae2-1.21.1'}
          </span>
          <span
            data-testid="save-status-badge"
            className={`badge save-badge-${saveStatus}`}
            title={saveError ?? undefined}
          >
            {saveLabel}
          </span>
          <button
            type="button"
            className="btn btn-primary"
            disabled={saveStatus === 'saving'}
            onClick={() => void saveLayout()}
          >
            Save Layout
          </button>
        </div>

        {/* Edit Tools */}
        <div className="topbar-section">
          <button
            type="button"
            className="btn"
            disabled={pastCount === 0}
            onClick={undo}
            title="Undo (Ctrl+Z)"
          >
            Undo
          </button>
          <button
            type="button"
            className="btn"
            disabled={futureCount === 0}
            onClick={redo}
            title="Redo (Ctrl+Y)"
          >
            Redo
          </button>
          <button
            type="button"
            className="btn"
            disabled={selectedCount === 0}
            onClick={copySelected}
            title="Copy Selected (Ctrl+C)"
          >
            Copy
          </button>
          <button
            type="button"
            className="btn"
            disabled={!hasClipboard}
            onClick={() => pasteClipboard()}
            title="Paste (Ctrl+V)"
          >
            Paste
          </button>
          <button
            type="button"
            className="btn"
            onClick={rotateSelectedOrPlacement}
            title="Rotate Clockwise (R)"
          >
            Rotate (R)
          </button>
          <button
            type="button"
            className="btn btn-danger"
            disabled={selectedCount === 0}
            onClick={() => removeBlocks()}
            title="Delete Selected (Del)"
          >
            Delete
          </button>
        </div>

        {/* Mode Selector & Execution / Export */}
        <div className="topbar-section">
          <div className="segmented-group" role="group" aria-label="Editor Mode">
            {MODES.map((m) => (
              <button
                key={m.id}
                type="button"
                aria-label={`${m.label} Mode`}
                className={`seg-btn ${activeMode === m.id ? 'active' : ''}`}
                onClick={() => setActiveMode(m.id)}
              >
                {m.label}
              </button>
            ))}
          </div>

          <div className="topbar-divider" />

          <button
            type="button"
            className="btn btn-certus"
            disabled={actionBusy}
            onClick={() => void runValidation()}
          >
            Validate
          </button>
          <button
            type="button"
            className="btn btn-primary"
            disabled={actionBusy}
            onClick={() => void startSimulation()}
          >
            Simulate
          </button>
          <button
            type="button"
            className="btn"
            disabled={exportBusy}
            onClick={() => void handleExportJson()}
          >
            Export JSON
          </button>
          <button
            type="button"
            className="btn"
            disabled={exportBusy}
            onClick={() => void handleExportBuildSheet()}
          >
            Build Sheet
          </button>
        </div>
      </header>

      {/* Secondary Viewport & Overlay Strip */}
      <div className="topbar-substrip">
        <div className="topbar-section">
          <div className="segmented-group" role="group" aria-label="Viewport Mode">
            <button
              type="button"
              className={`seg-btn ${viewMode === '3d' ? 'active-certus' : ''}`}
              onClick={() => setViewMode('3d')}
            >
              3D View
            </button>
            <button
              type="button"
              className={`seg-btn ${viewMode === '2d' ? 'active-certus' : ''}`}
              onClick={() => setViewMode('2d')}
            >
              2D Slice
            </button>
          </div>

          <div className="topbar-section">
            <button
              type="button"
              className="btn"
              style={{ padding: '2px 8px' }}
              aria-label="Layer Down"
              onClick={() => stepActiveLayerY(-1)}
            >
              Y −
            </button>
            <span className="badge" data-testid="active-layer-badge">
              Y = {activeLayerY}
            </span>
            <button
              type="button"
              className="btn"
              style={{ padding: '2px 8px' }}
              aria-label="Layer Up"
              onClick={() => stepActiveLayerY(1)}
            >
              Y +
            </button>
            {activeLayerY !== 0 && (
              <button
                type="button"
                className="btn"
                style={{ padding: '2px 6px', fontSize: 10 }}
                onClick={() => setActiveLayerY(0)}
              >
                Reset Y=0
              </button>
            )}
          </div>

          {viewMode === '3d' && (
            <div className="segmented-group" role="group" aria-label="Camera Presets">
              {CAMERA_PRESETS.map((cam) => (
                <button
                  key={cam.id}
                  type="button"
                  className={`seg-btn ${cameraPreset === cam.id ? 'active-certus' : ''}`}
                  onClick={() => setCameraPreset(cam.id)}
                >
                  {cam.label}
                </button>
              ))}
            </div>
          )}
        </div>

        <div className="topbar-section" role="group" aria-label="Visual Overlays">
          <span style={{ color: 'var(--text-muted)', fontSize: 10, fontWeight: 700 }}>
            OVERLAYS:
          </span>
          {OVERLAY_KEYS.map((ov) => (
            <button
              key={ov.key}
              type="button"
              className={`overlay-chip ${overlays[ov.key] ? 'active' : ''}`}
              onClick={() => toggleOverlay(ov.key)}
            >
              {ov.label}
            </button>
          ))}
        </div>
      </div>

      {/* Main 3-Column Technical CAD Workspace */}
      <div className="editor-main-split">
        <BlockPalette />

        <div className="editor-center-column">
          <div className="editor-viewport-box">
            {viewMode === '3d' ? <Workspace3D /> : <Workspace2D />}
          </div>
          <SimulationConsole />
        </div>

        <BlockInspector />
      </div>

      {/* Blueprint JSON / Build Sheet Export Modal */}
      {exportModal && (
        <div
          className="export-modal-backdrop"
          role="dialog"
          aria-modal="true"
          aria-label={exportModal.title}
        >
          <div className="export-modal-card">
            <div className="export-modal-header">
              <strong>{exportModal.title}</strong>
              <div className="topbar-section">
                {copiedBanner && <span className="badge badge-exact">Copied!</span>}
                <button
                  type="button"
                  className="btn"
                  onClick={() => void handleCopyExport()}
                >
                  Copy
                </button>
                <button
                  type="button"
                  className="btn btn-primary"
                  onClick={handleDownloadExport}
                >
                  Download {exportModal.filename}
                </button>
                <button
                  type="button"
                  className="btn btn-danger"
                  onClick={() => setExportModal(null)}
                >
                  Close
                </button>
              </div>
            </div>
            <pre className="export-modal-pre">{exportModal.content}</pre>
          </div>
        </div>
      )}
    </div>
  )
}
