import { type MouseEvent, useMemo, useState } from 'react'
import {
  computeConnectedDirections,
  coordKey,
  directionArrow2D,
  networkColorForId,
  resolveBlockColor,
} from '../../lib/ae2/constants'
import type {
  BlockDefinition,
  CableChannelOverlay,
  DeviceRouteOverlay,
  PlacedBlock,
} from '../../lib/api'
import { selectActiveOverlay, useEditorStore } from './store'
import './Workspace2D.css'

function blockShortCode(def: BlockDefinition | undefined, blockDefId: string): string {
  const idPart = blockDefId.includes(':') ? blockDefId.split(':')[1] : blockDefId
  if (def?.metadata.is_dense_cable) return 'DC'
  if (def?.metadata.is_smart_cable) return 'SC'
  if (def?.metadata.is_cable) return 'CB'
  switch (idPart) {
    case 'controller':
      return 'CTL'
    case 'energy_acceptor':
      return 'EA'
    case 'energy_cell':
      return 'EC'
    case 'dense_energy_cell':
      return 'DEC'
    case 'creative_energy_cell':
      return 'INF'
    case 'drive':
      return 'DRV'
    case 'terminal':
      return 'TRM'
    case 'crafting_terminal':
      return 'CTM'
    case 'pattern_provider':
      return 'PRV'
    case 'molecular_assembler':
      return 'ASM'
    case 'crafting_co_processor':
      return 'COP'
    case 'crafting_unit':
      return 'CPU'
    case 'crafting_monitor':
      return 'MON'
    default:
      if (idPart.startsWith('crafting_storage_')) {
        return idPart.replace('crafting_storage_', '').toUpperCase()
      }
      return idPart.slice(0, 3).toUpperCase()
  }
}

export interface Workspace2DProps {
  fallbackNotice?: string
  onBlockContextMenu?: (blockId: string, x: number, y: number) => void
}

export function Workspace2D({ fallbackNotice, onBlockContextMenu }: Workspace2DProps) {
  const blocks = useEditorStore((s) => s.blocks)
  const catalog = useEditorStore((s) => s.catalog)
  const activeLayerY = useEditorStore((s) => s.activeLayerY)
  const activeMode = useEditorStore((s) => s.activeMode)
  const activeBlockDefId = useEditorStore((s) => s.activeBlockDefId)
  const placementFacing = useEditorStore((s) => s.placementFacing)
  const selectedBlockIds = useEditorStore((s) => s.selectedBlockIds)
  const highlightedCoords = useEditorStore((s) => s.highlightedCoords)
  const diagnostics = useEditorStore((s) => s.diagnostics)
  const overlays = useEditorStore((s) => s.overlays)
  const simulationRun = useEditorStore((s) => s.simulationRun)
  const validationResult = useEditorStore((s) => s.validationResult)

  const placeBlockAt = useEditorStore((s) => s.placeBlockAt)
  const selectBlock = useEditorStore((s) => s.selectBlock)
  const setInspectorOpen = useEditorStore((s) => s.setInspectorOpen)
  const clearSelection = useEditorStore((s) => s.clearSelection)
  const [hoverCoord, setHoverCoord] = useState<{ x: number; z: number } | null>(null)

  const catalogMap = useMemo(() => {
    const map = new Map<string, BlockDefinition>()
    for (const d of catalog) {
      map.set(d.id, d)
    }
    return map
  }, [catalog])

  const blocksByCoord = useMemo(() => {
    const map = new Map<string, PlacedBlock>()
    for (const b of blocks) {
      map.set(coordKey(b.x, b.y, b.z), b)
    }
    return map
  }, [blocks])

  const verticalColumnHints = useMemo(() => {
    const map = new Map<string, number[]>()
    for (const b of blocks) {
      if (b.y !== activeLayerY) {
        const key = `${b.x},${b.z}`
        const arr = map.get(key) ?? []
        arr.push(b.y)
        map.set(key, arr)
      }
    }
    return map
  }, [blocks, activeLayerY])

  const activeOverlay = useMemo(
    () => selectActiveOverlay({ simulationRun, validationResult }),
    [simulationRun, validationResult]
  )

  const cableOverlayByCoord = useMemo(() => {
    const map = new Map<string, CableChannelOverlay>()
    for (const c of activeOverlay?.cables ?? []) {
      map.set(coordKey(c.coord.x, c.coord.y, c.coord.z), c)
    }
    return map
  }, [activeOverlay])

  const deviceRouteByCoord = useMemo(() => {
    const map = new Map<string, DeviceRouteOverlay>()
    for (const r of activeOverlay?.device_routes ?? []) {
      map.set(coordKey(r.coord.x, r.coord.y, r.coord.z), r)
    }
    return map
  }, [activeOverlay])

  const highlightedSet = useMemo(
    () => new Set(highlightedCoords.map((c) => coordKey(c.x, c.y, c.z))),
    [highlightedCoords]
  )

  const warningCoordSet = useMemo(() => {
    const set = new Set<string>()
    for (const d of diagnostics) {
      for (const ab of d.affected_blocks ?? []) {
        set.add(coordKey(ab.x, ab.y, ab.z))
      }
    }
    return set
  }, [diagnostics])

  const { xValues, zValues } = useMemo(() => {
    let minX = -5
    let maxX = 7
    let minZ = -4
    let maxZ = 4
    for (const b of blocks) {
      if (b.x - 1 < minX) minX = b.x - 1
      if (b.x + 1 > maxX) maxX = b.x + 1
      if (b.z - 1 < minZ) minZ = b.z - 1
      if (b.z + 1 > maxZ) maxZ = b.z + 1
    }
    for (const c of highlightedCoords) {
      if (c.x - 1 < minX) minX = c.x - 1
      if (c.x + 1 > maxX) maxX = c.x + 1
      if (c.z - 1 < minZ) minZ = c.z - 1
      if (c.z + 1 > maxZ) maxZ = c.z + 1
    }
    const xs: number[] = []
    for (let x = minX; x <= maxX; x++) xs.push(x)
    const zs: number[] = []
    for (let z = minZ; z <= maxZ; z++) zs.push(z)
    return { xValues: xs, zValues: zs }
  }, [blocks, highlightedCoords])

  const handleCellClick = (
    e: MouseEvent<HTMLButtonElement>,
    x: number,
    z: number,
    existing: PlacedBlock | undefined
  ) => {
    if (existing) {
      selectBlock(existing.id, e.shiftKey)
      return
    }
    if (activeMode === 'build' && activeBlockDefId) {
      placeBlockAt({ x, y: activeLayerY, z })
    } else if (!e.shiftKey) {
      clearSelection()
    }
  }

  const handleCellContextMenu = (
    e: MouseEvent<HTMLButtonElement>,
    block: PlacedBlock | undefined
  ) => {
    if (!block) return
    e.preventDefault()
    e.stopPropagation()
    onBlockContextMenu?.(block.id, e.clientX, e.clientY)
  }

  const handleCellDoubleClick = (
    e: MouseEvent<HTMLButtonElement>,
    block: PlacedBlock | undefined
  ) => {
    if (!block) return
    e.preventDefault()
    e.stopPropagation()
    selectBlock(block.id, false)
    setInspectorOpen(true)
  }

  return (
    <div className="workspace-2d" data-testid="workspace-2d">
      <div className="workspace-2d-hud">
        <div className="workspace-2d-compass">
          <span>SLICE Y = {activeLayerY}</span>
          <span>•</span>
          <span>N (-Z) ↑ | E (+X) →</span>
          {fallbackNotice && <span className="badge">{fallbackNotice}</span>}
        </div>
        <div>
          {hoverCoord
            ? `Cursor: (${hoverCoord.x}, ${activeLayerY}, ${hoverCoord.z})`
            : `Tool: ${activeBlockDefId ?? 'Select'} (Facing ${placementFacing})`}
        </div>
      </div>

      <div className="workspace-2d-scroll">
        <div
          className="slice-grid-table"
          style={{
            gridTemplateColumns: `38px repeat(${xValues.length}, 52px)`,
          }}
        >
          <div className="grid-axis-corner">Z \ X</div>
          {xValues.map((x) => (
            <div key={`col-${x}`} className="grid-axis-label">
              {x}
            </div>
          ))}

          {zValues.map((z) => (
            <div key={`row-wrap-${z}`} style={{ display: 'contents' }}>
              <div className="grid-axis-label grid-axis-row-label">{z}</div>
              {xValues.map((x) => {
                const key = coordKey(x, activeLayerY, z)
                const block = blocksByCoord.get(key)
                const def = block ? catalogMap.get(block.block_def_id) : undefined
                const isSelected = block ? selectedBlockIds.includes(block.id) : false
                const isHighlighted = highlightedSet.has(key)
                const cableOv = cableOverlayByCoord.get(key)
                const routeOv = deviceRouteByCoord.get(key)
                const networkId = cableOv?.network_id ?? routeOv?.network_id
                const connectedDirs = block
                  ? computeConnectedDirections(block, blocksByCoord, catalogMap, cableOv)
                  : []
                const otherYs = verticalColumnHints.get(`${x},${z}`)
                const color = block ? resolveBlockColor(block, def) : undefined
                const shape = def?.metadata.visual.shape ?? 'cube'
                const isDense = Boolean(def?.metadata.is_dense_cable || shape === 'dense_cable')

                const hasWarning =
                  overlays.warnings &&
                  Boolean(
                    warningCoordSet.has(key) ||
                      cableOv?.overloaded ||
                      (routeOv && !routeOv.has_channel)
                  )

                let channelLabel: string | null = null
                let channelFault = false
                if (overlays.channels && block) {
                  if (cableOv) {
                    channelLabel = `${cableOv.channels_used}/${cableOv.capacity}`
                    channelFault = cableOv.overloaded
                  } else if (routeOv) {
                    channelLabel = routeOv.has_channel ? '1/1' : '0/1'
                    channelFault = !routeOv.has_channel
                  } else if (def?.metadata.is_cable) {
                    channelLabel = `0/${def.metadata.channel_capacity}`
                  }
                }

                return (
                  <button
                    key={key}
                    type="button"
                    data-testid={`grid-cell-${x}-${activeLayerY}-${z}`}
                    data-coord={key}
                    data-highlighted={isHighlighted ? 'true' : 'false'}
                    aria-label={`Grid cell (${x}, ${activeLayerY}, ${z})`}
                    className={[
                      'slice-cell',
                      x === 0 && z === 0 ? 'origin-cell' : '',
                      isSelected ? 'selected' : '',
                      isHighlighted ? 'cell-diagnostic-highlight' : '',
                    ]
                      .filter(Boolean)
                      .join(' ')}
                    onMouseEnter={() => setHoverCoord({ x, z })}
                    onMouseLeave={() => setHoverCoord(null)}
                    onClick={(e) => handleCellClick(e, x, z, block)}
                    onDoubleClick={(e) => handleCellDoubleClick(e, block)}
                    onContextMenu={(e) => handleCellContextMenu(e, block)}
                  >
                    {!block && otherYs && otherYs.length > 0 && (
                      <span className="cell-below-hint">Y:{otherYs[0]}</span>
                    )}

                    {block && overlays.networks && networkId && (
                      <span
                        className="cell-network-ring"
                        style={{ borderColor: networkColorForId(networkId) }}
                      />
                    )}

                    {block &&
                      overlays.connections &&
                      connectedDirs.map((dir) => {
                        if (dir === 'up' || dir === 'down') {
                          return null
                        }
                        return (
                          <span
                            key={dir}
                            className={`cell-conn-arm cell-conn-${dir} ${
                              isDense ? 'cell-conn-dense' : ''
                            }`}
                            style={{ backgroundColor: color }}
                          />
                        )
                      })}

                    {block && (
                      <span
                        className={`cell-block-node cell-node-${shape}`}
                        style={{
                          backgroundColor: color,
                          borderColor: def?.metadata.visual.accent_color ?? 'rgba(255,255,255,0.35)',
                        }}
                      >
                        <span>{blockShortCode(def, block.block_def_id)}</span>
                        {(def?.metadata.rotatable || !def?.metadata.is_cable) && (
                          <span>{directionArrow2D(block.facing)}</span>
                        )}
                      </span>
                    )}

                    {block &&
                      overlays.connections &&
                      (connectedDirs.includes('up') || connectedDirs.includes('down')) && (
                        <span className="cell-vertical-tag">
                          {connectedDirs.includes('up') ? '▲' : ''}
                          {connectedDirs.includes('down') ? '▼' : ''}
                        </span>
                      )}

                    {channelLabel && (
                      <span className={`cell-channel-tag ${channelFault ? 'fault' : ''}`}>
                        {channelLabel}
                      </span>
                    )}

                    {hasWarning && <span className="cell-warning-dot">!</span>}
                  </button>
                )
              })}
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}
