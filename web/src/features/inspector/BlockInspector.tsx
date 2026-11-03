import { useMemo, useState } from 'react'
import {
  ALL_DIRECTIONS,
  CABLE_COLOR_HEX,
  CABLE_COLORS,
  CELL_TIERS,
  coordKey,
  isCableBlock,
  readBooleanConfig,
  readBusConfig,
  readCableConfig,
  readDriveConfig,
  readNumberConfig,
  readPatternProviderConfig,
  type CableColor,
} from '../../lib/ae2/constants'
import type { Direction, DriveCellConfig, PatternSpec } from '../../lib/api'
import { selectActiveOverlay, useEditorStore } from '../editor/store'
import './BlockInspector.css'

export function BlockInspector() {
  const blocks = useEditorStore((s) => s.blocks)
  const catalog = useEditorStore((s) => s.catalog)
  const selectedBlockIds = useEditorStore((s) => s.selectedBlockIds)
  const simulationRun = useEditorStore((s) => s.simulationRun)
  const validationResult = useEditorStore((s) => s.validationResult)

  const updateBlockFields = useEditorStore((s) => s.updateBlockFields)
  const rotateSelectedOrPlacement = useEditorStore((s) => s.rotateSelectedOrPlacement)
  const removeBlocks = useEditorStore((s) => s.removeBlocks)

  const [newPartitionInputs, setNewPartitionInputs] = useState<Record<number, string>>({})

  const selectedBlocks = useMemo(
    () => blocks.filter((b) => selectedBlockIds.includes(b.id)),
    [blocks, selectedBlockIds]
  )

  const primaryBlock = selectedBlocks.length === 1 ? selectedBlocks[0] : null
  const primaryDef = useMemo(
    () =>
      primaryBlock
        ? catalog.find((d) => d.id === primaryBlock.block_def_id)
        : undefined,
    [catalog, primaryBlock]
  )

  const activeOverlay = useMemo(
    () => selectActiveOverlay({ simulationRun, validationResult }),
    [simulationRun, validationResult]
  )

  const blockTelemetry = useMemo(() => {
    if (!primaryBlock || !activeOverlay) {
      return null
    }
    const key = coordKey(primaryBlock.x, primaryBlock.y, primaryBlock.z)
    const cable = activeOverlay.cables?.find(
      (c) => coordKey(c.coord.x, c.coord.y, c.coord.z) === key
    )
    const route = activeOverlay.device_routes?.find(
      (r) => coordKey(r.coord.x, r.coord.y, r.coord.z) === key
    )
    return { cable, route }
  }, [activeOverlay, primaryBlock])

  if (selectedBlocks.length === 0) {
    return (
      <aside className="block-inspector" aria-label="Block Inspector">
        <div className="inspector-header">
          <span className="inspector-title">Inspector</span>
          <span className="badge">{blocks.length} Blocks</span>
        </div>
        <div className="inspector-body">
          <div className="inspector-empty">
            <p>
              Select any placed block on the 3D/2D grid to configure channels, cable colors,
              storage cells, encoded patterns, and priorities.
            </p>
            <div className="shortcut-box">
              <div className="inspector-section-title">CAD Shortcuts</div>
              <div className="shortcut-row">
                <span>Rotate Facing</span>
                <span className="shortcut-key">R</span>
              </div>
              <div className="shortcut-row">
                <span>Delete Selected</span>
                <span className="shortcut-key">Del / Backspace</span>
              </div>
              <div className="shortcut-row">
                <span>Undo / Redo</span>
                <span className="shortcut-key">Ctrl+Z / Ctrl+Y</span>
              </div>
              <div className="shortcut-row">
                <span>Copy / Paste</span>
                <span className="shortcut-key">Ctrl+C / Ctrl+V</span>
              </div>
              <div className="shortcut-row">
                <span>Multi-Select</span>
                <span className="shortcut-key">Shift + Click</span>
              </div>
            </div>
          </div>
        </div>
      </aside>
    )
  }

  if (!primaryBlock) {
    return (
      <aside className="block-inspector" aria-label="Block Inspector">
        <div className="inspector-header">
          <span className="inspector-title">Multi-Selection</span>
          <span className="badge">{selectedBlocks.length} Selected</span>
        </div>
        <div className="inspector-body">
          <div className="inspector-card">
            <div className="inspector-section-title">Batch Operations</div>
            <div className="inspector-inline-row">
              <button
                type="button"
                className="btn"
                onClick={rotateSelectedOrPlacement}
              >
                Rotate All CW
              </button>
              <button
                type="button"
                className="btn btn-danger"
                onClick={() => removeBlocks()}
              >
                Delete ({selectedBlocks.length})
              </button>
            </div>
            <div className="inspector-desc mono">
              {selectedBlocks
                .slice(0, 8)
                .map((b) => `(${b.x}, ${b.y}, ${b.z}) ${b.block_def_id}`)
                .join('\n')}
            </div>
          </div>
        </div>
      </aside>
    )
  }

  const patchConfig = (extra: Record<string, unknown>) => {
    updateBlockFields(primaryBlock.id, {
      config: {
        ...primaryBlock.config,
        ...extra,
      },
    })
  }

  const showCableSection = Boolean(
    isCableBlock(primaryBlock, primaryDef) ||
      primaryDef?.metadata.connection_rules.supports_cable_color ||
      primaryDef?.metadata.connection_rules.supports_cable_anchors
  )
  const cableCfg = showCableSection ? readCableConfig(primaryBlock.config) : null
  const driveCfg =
    primaryBlock.block_def_id === 'ae2:drive'
      ? readDriveConfig(primaryBlock.config)
      : null
  const providerCfg =
    primaryBlock.block_def_id === 'ae2:pattern_provider'
      ? readPatternProviderConfig(primaryBlock.config)
      : null
  const busCfg =
    primaryBlock.block_def_id === 'ae2:import_bus' ||
    primaryBlock.block_def_id === 'ae2:export_bus' ||
    primaryBlock.block_def_id === 'ae2:storage_bus'
      ? readBusConfig(primaryBlock.config)
      : null
  return (
    <aside className="block-inspector" aria-label="Block Inspector">
      <div className="inspector-header">
        <span className="inspector-title">Component Inspector</span>
        <span className="badge" data-testid="inspector-coords">
          ({primaryBlock.x}, {primaryBlock.y}, {primaryBlock.z})
        </span>
      </div>

      <div className="inspector-body">
        {/* Identity & Documentation */}
        <div className="inspector-card">
          <div>
            <div className="inspector-block-title">
              {primaryDef?.name ?? primaryBlock.block_def_id}
            </div>
            <div className="inspector-block-id">{primaryBlock.block_def_id}</div>
          </div>

          <div className="inspector-inline-row" style={{ flexWrap: 'wrap' }}>
            {primaryDef && (
              <span className={`badge badge-${primaryDef.fidelity}`}>
                {primaryDef.fidelity}
              </span>
            )}
            {primaryDef && (
              <span className="badge">
                {primaryDef.metadata.energy_cost} AE/t
              </span>
            )}
            {primaryDef && primaryDef.metadata.channel_usage > 0 && (
              <span className="badge">Uses {primaryDef.metadata.channel_usage} Ch</span>
            )}
          </div>

          {primaryDef?.fidelity === 'unsupported' && (
            <div className="unsupported-banner" role="status">
              Post-MVP block: placing this component emits diagnostic{' '}
              <code>UNSUPPORTED_MECHANIC</code> during validation/simulation.
            </div>
          )}

          {primaryDef?.metadata.description && (
            <p className="inspector-desc">{primaryDef.metadata.description}</p>
          )}

          {primaryDef?.doc_reference && (
            <a
              href={primaryDef.doc_reference}
              target="_blank"
              rel="noreferrer"
              className="inspector-guide-link"
            >
              Official AE2 1.21.1 Guide ↗
            </a>
          )}
        </div>

        {/* Orientation & Label */}
        <div className="inspector-card">
          <div className="inspector-section-title">Placement &amp; Identity</div>

          <div className="inspector-field">
            <label htmlFor="inspector-label-input">User Label</label>
            <input
              id="inspector-label-input"
              aria-label="Block Label"
              type="text"
              placeholder="Optional node tag…"
              value={primaryBlock.label ?? ''}
              onChange={(e) =>
                updateBlockFields(primaryBlock.id, { label: e.target.value })
              }
            />
          </div>

          <div className="inspector-field">
            <label htmlFor="inspector-facing-select">Facing Orientation</label>
            <div className="inspector-inline-row">
              <select
                id="inspector-facing-select"
                aria-label="Block Facing"
                style={{ flex: 1 }}
                value={primaryBlock.facing}
                onChange={(e) => {
                  const val = e.target.value
                  if ((ALL_DIRECTIONS as readonly string[]).includes(val)) {
                    updateBlockFields(primaryBlock.id, { facing: val as Direction })
                  }
                }}
              >
                {ALL_DIRECTIONS.map((dir) => (
                  <option key={dir} value={dir}>
                    {dir.toUpperCase()}
                  </option>
                ))}
              </select>
              <button
                type="button"
                className="btn"
                onClick={rotateSelectedOrPlacement}
              >
                Rotate CW
              </button>
            </div>
          </div>
        </div>

        {/* Cable Color & Anchors */}
        {showCableSection && cableCfg && (
          <div className="inspector-card">
            <div className="inspector-section-title">Cable Color &amp; Anchors</div>

            <div className="inspector-field">
              <label htmlFor="inspector-cable-color">Cable Insulation Color</label>
              <select
                id="inspector-cable-color"
                aria-label="Cable Color"
                value={cableCfg.color}
                onChange={(e) =>
                  patchConfig({ color: e.target.value as CableColor })
                }
              >
                {CABLE_COLORS.map((col) => (
                  <option key={col} value={col}>
                    {col.toUpperCase()}
                  </option>
                ))}
              </select>
              <div className="cable-swatches">
                {CABLE_COLORS.map((col) => (
                  <button
                    key={col}
                    type="button"
                    title={`Color: ${col}`}
                    aria-label={`Select ${col} cable`}
                    className={`cable-swatch-btn ${
                      cableCfg.color === col ? 'active' : ''
                    }`}
                    style={{ backgroundColor: CABLE_COLOR_HEX[col] }}
                    onClick={() => patchConfig({ color: col })}
                  />
                ))}
              </div>
            </div>

            <div className="inspector-field">
              <label>Anchored Faces (Block Connection)</label>
              <div className="anchor-grid">
                {ALL_DIRECTIONS.map((face) => {
                  const checked = cableCfg.anchored_faces.includes(face)
                  return (
                    <label key={face} className="inspector-checkbox-row">
                      <input
                        type="checkbox"
                        aria-label={`Anchor ${face}`}
                        checked={checked}
                        onChange={(e) => {
                          const nextFaces = e.target.checked
                            ? [...cableCfg.anchored_faces, face]
                            : cableCfg.anchored_faces.filter((f) => f !== face)
                          patchConfig({ anchored_faces: nextFaces })
                        }}
                      />
                      <span>{face}</span>
                    </label>
                  )
                })}
              </div>
            </div>
          </div>
        )}

        {/* ME Drive Priority & Cells */}
        {driveCfg && (
          <div className="inspector-card">
            <div className="inspector-section-title">ME Drive &amp; Storage Cells</div>

            <div className="inspector-field">
              <label htmlFor="drive-priority-input">Storage Priority</label>
              <input
                id="drive-priority-input"
                aria-label="Drive Priority"
                type="number"
                value={driveCfg.priority}
                onChange={(e) =>
                  patchConfig({ priority: Number(e.target.value) || 0 })
                }
              />
            </div>

            <div className="sub-editor-header">
              <span style={{ fontSize: 11, fontWeight: 600 }}>
                Cells ({driveCfg.cells.length}/10)
              </span>
              <button
                type="button"
                className="btn btn-certus"
                disabled={driveCfg.cells.length >= 10}
                onClick={() => {
                  const nextCells: DriveCellConfig[] = [
                    ...driveCfg.cells,
                    { tier: '4k', partition: [], inverter: false, items: [] },
                  ]
                  patchConfig({ cells: nextCells })
                }}
              >
                + Add Cell
              </button>
            </div>

            {driveCfg.cells.map((cell, cellIdx) => (
              <div key={`cell-${cellIdx}`} className="sub-editor-card">
                <div className="sub-editor-header">
                  <select
                    aria-label={`Cell ${cellIdx + 1} Tier`}
                    value={cell.tier}
                    onChange={(e) => {
                      const next = structuredClone(driveCfg.cells)
                      next[cellIdx].tier = e.target.value as DriveCellConfig['tier']
                      patchConfig({ cells: next })
                    }}
                  >
                    {CELL_TIERS.map((tier) => (
                      <option key={tier} value={tier}>
                        {tier.toUpperCase()} Storage Cell
                      </option>
                    ))}
                  </select>
                  <button
                    type="button"
                    className="btn btn-danger"
                    onClick={() => {
                      const next = driveCfg.cells.filter((_, idx) => idx !== cellIdx)
                      patchConfig({ cells: next })
                    }}
                  >
                    Remove
                  </button>
                </div>

                {/* Initial Stored Items */}
                <div className="inspector-field">
                  <div className="sub-editor-header">
                    <label>Initial Items</label>
                    <button
                      type="button"
                      className="btn"
                      onClick={() => {
                        const next = structuredClone(driveCfg.cells)
                        next[cellIdx].items.push({
                          item_id: 'minecraft:iron_ingot',
                          quantity: 64,
                        })
                        patchConfig({ cells: next })
                      }}
                    >
                      + Item
                    </button>
                  </div>
                  {cell.items.map((item, itemIdx) => (
                    <div key={`item-${itemIdx}`} className="stack-row">
                      <input
                        type="text"
                        aria-label={`Cell ${cellIdx + 1} Item ${itemIdx + 1} ID`}
                        placeholder="modid:item_name"
                        value={item.item_id}
                        onChange={(e) => {
                          const next = structuredClone(driveCfg.cells)
                          next[cellIdx].items[itemIdx].item_id = e.target.value
                          patchConfig({ cells: next })
                        }}
                      />
                      <input
                        type="number"
                        min={1}
                        aria-label={`Cell ${cellIdx + 1} Item ${itemIdx + 1} Quantity`}
                        value={item.quantity}
                        onChange={(e) => {
                          const next = structuredClone(driveCfg.cells)
                          next[cellIdx].items[itemIdx].quantity = Math.max(
                            1,
                            Number(e.target.value) || 1
                          )
                          patchConfig({ cells: next })
                        }}
                      />
                      <button
                        type="button"
                        className="btn btn-danger"
                        style={{ padding: '4px 0' }}
                        onClick={() => {
                          const next = structuredClone(driveCfg.cells)
                          next[cellIdx].items.splice(itemIdx, 1)
                          patchConfig({ cells: next })
                        }}
                      >
                        ×
                      </button>
                    </div>
                  ))}
                </div>

                {/* Partition Filter & Inverter */}
                <div className="inspector-field">
                  <label>Partition Filter</label>
                  <div className="inspector-inline-row">
                    <input
                      type="text"
                      style={{ flex: 1 }}
                      placeholder="minecraft:oak_log"
                      value={newPartitionInputs[cellIdx] ?? ''}
                      onChange={(e) =>
                        setNewPartitionInputs((prev) => ({
                          ...prev,
                          [cellIdx]: e.target.value,
                        }))
                      }
                    />
                    <button
                      type="button"
                      className="btn"
                      onClick={() => {
                        const raw = (newPartitionInputs[cellIdx] ?? '').trim()
                        if (!raw) return
                        const next = structuredClone(driveCfg.cells)
                        if (!next[cellIdx].partition.includes(raw)) {
                          next[cellIdx].partition.push(raw)
                        }
                        patchConfig({ cells: next })
                        setNewPartitionInputs((prev) => ({ ...prev, [cellIdx]: '' }))
                      }}
                    >
                      Add
                    </button>
                  </div>
                  {cell.partition.length > 0 && (
                    <div className="partition-chips">
                      {cell.partition.map((partItem) => (
                        <span key={partItem} className="partition-chip">
                          {partItem}
                          <button
                            type="button"
                            style={{ background: 'transparent', padding: '0 2px' }}
                            onClick={() => {
                              const next = structuredClone(driveCfg.cells)
                              next[cellIdx].partition = next[cellIdx].partition.filter(
                                (p) => p !== partItem
                              )
                              patchConfig({ cells: next })
                            }}
                          >
                            ×
                          </button>
                        </span>
                      ))}
                    </div>
                  )}
                  <label className="inspector-checkbox-row">
                    <input
                      type="checkbox"
                      checked={cell.inverter}
                      onChange={(e) => {
                        const next = structuredClone(driveCfg.cells)
                        next[cellIdx].inverter = e.target.checked
                        patchConfig({ cells: next })
                      }}
                    />
                    <span>Inverter Card (Blacklist Partition)</span>
                  </label>
                </div>
              </div>
            ))}
          </div>
        )}

        {/* Import / Export / Storage Bus Editor */}
        {busCfg && (
          <div className="inspector-card">
            <div className="inspector-section-title">ME Bus Automation</div>
            <div className="inspector-field">
              <label htmlFor="bus-filter-field">
                {primaryBlock.block_def_id === 'ae2:export_bus' ? 'Export Filter (required)' : 'Item Filter'}
              </label>
              <textarea
                id="bus-filter-field"
                aria-label="Bus Item Filter"
                rows={2}
                value={busCfg.filters.join(', ')}
                placeholder="minecraft:iron_ingot, minecraft:oak_log"
                onChange={(e) =>
                  patchConfig({
                    filters: e.target.value
                      .split(/[,\n]/)
                      .map((item) => item.trim())
                      .filter(Boolean),
                  })
                }
              />
              <span className="inspector-block-id">
                {primaryBlock.block_def_id === 'ae2:export_bus'
                  ? 'Only filtered items leave ME storage.'
                  : 'Empty filter imports all accessible items.'}
              </span>
            </div>
            <div className="inspector-field">
              <label htmlFor="bus-acceleration-select">Acceleration Cards</label>
              <select
                id="bus-acceleration-select"
                aria-label="Bus Acceleration"
                value={busCfg.acceleration}
                onChange={(e) => patchConfig({ acceleration: Number(e.target.value) })}
              >
                <option value={0}>0 — 1 item/op</option>
                <option value={1}>1 — 8 items/op</option>
                <option value={2}>2 — 32 items/op</option>
                <option value={3}>3 — 64 items/op</option>
                <option value={4}>4 — 96 items/op</option>
              </select>
            </div>
            <label className="inspector-checkbox-row">
              <input
                type="checkbox"
                aria-label="Bus Inverter"
                checked={busCfg.inverter}
                onChange={(e) => patchConfig({ inverter: e.target.checked })}
              />
              <span>Inverter Card (blacklist filter)</span>
            </label>
          </div>
        )}

        {/* Pattern Provider Editor */}
        {providerCfg && (
          <div className="inspector-card">
            <div className="inspector-section-title">Pattern Provider &amp; Recipes</div>

            <div className="inspector-field">
              <label htmlFor="provider-priority-input">Provider Priority</label>
              <input
                id="provider-priority-input"
                aria-label="Provider Priority"
                type="number"
                value={providerCfg.priority}
                onChange={(e) =>
                  patchConfig({ priority: Number(e.target.value) || 0 })
                }
              />
            </div>

            <div className="inspector-field">
              <label htmlFor="provider-variant-select">Provider Variant</label>
              <select
                id="provider-variant-select"
                aria-label="Provider Variant"
                value={providerCfg.variant}
                onChange={(e) =>
                  patchConfig({
                    variant: e.target.value === 'directional' ? 'directional' : 'normal',
                  })
                }
              >
                <option value="normal">Full Block (All-Face Push)</option>
                <option value="directional">Directional Subpart (Push on Facing)</option>
              </select>
            </div>

            <label className="inspector-checkbox-row">
              <input
                type="checkbox"
                aria-label="Blocking Mode"
                checked={providerCfg.blocking_mode}
                onChange={(e) => patchConfig({ blocking_mode: e.target.checked })}
              />
              <span>Blocking Mode (Wait if target busy)</span>
            </label>

            <div className="inspector-field">
              <label htmlFor="provider-lock-select">Crafting Lock Mode</label>
              <select
                id="provider-lock-select"
                aria-label="Lock Mode"
                value={providerCfg.lock_mode}
                onChange={(e) =>
                  patchConfig({
                    lock_mode:
                      e.target.value === 'until_result_returned'
                        ? 'until_result_returned'
                        : 'none',
                  })
                }
              >
                <option value="none">Unlocked (Continuous Push)</option>
                <option value="until_result_returned">Lock Until Primary Result Returned</option>
              </select>
            </div>

            <div className="sub-editor-header">
              <span style={{ fontSize: 11, fontWeight: 600 }}>
                Encoded Patterns ({providerCfg.patterns.length})
              </span>
              <button
                type="button"
                className="btn btn-certus"
                onClick={() => {
                  const nextPatterns: PatternSpec[] = [
                    ...providerCfg.patterns,
                    {
                      id: `pat-${providerCfg.patterns.length + 1}`,
                      pattern_type: 'crafting',
                      inputs: [{ item_id: 'minecraft:oak_log', quantity: 1 }],
                      outputs: [{ item_id: 'minecraft:oak_planks', quantity: 4 }],
                    },
                  ]
                  patchConfig({ patterns: nextPatterns })
                }}
              >
                + Add Pattern
              </button>
            </div>

            {providerCfg.patterns.map((pat, patIdx) => (
              <div key={pat.id ?? `pat-${patIdx}`} className="sub-editor-card">
                <div className="sub-editor-header">
                  <select
                    aria-label={`Pattern ${patIdx + 1} Type`}
                    value={pat.pattern_type}
                    onChange={(e) => {
                      const next = structuredClone(providerCfg.patterns)
                      next[patIdx].pattern_type =
                        e.target.value === 'processing' ? 'processing' : 'crafting'
                      patchConfig({ patterns: next })
                    }}
                  >
                    <option value="crafting">Crafting (Assembler)</option>
                    <option value="processing">Processing (Machine)</option>
                  </select>
                  <button
                    type="button"
                    className="btn btn-danger"
                    onClick={() => {
                      const next = providerCfg.patterns.filter((_, idx) => idx !== patIdx)
                      patchConfig({ patterns: next })
                    }}
                  >
                    Remove
                  </button>
                </div>

                {/* Pattern Inputs */}
                <div className="inspector-field">
                  <div className="sub-editor-header">
                    <label>Inputs</label>
                    <button
                      type="button"
                      className="btn"
                      onClick={() => {
                        const next = structuredClone(providerCfg.patterns)
                        next[patIdx].inputs.push({
                          item_id: 'minecraft:iron_ingot',
                          quantity: 1,
                        })
                        patchConfig({ patterns: next })
                      }}
                    >
                      + Input
                    </button>
                  </div>
                  {pat.inputs.map((inp, inpIdx) => (
                    <div key={`in-${inpIdx}`} className="stack-row">
                      <input
                        type="text"
                        value={inp.item_id}
                        placeholder="modid:item"
                        onChange={(e) => {
                          const next = structuredClone(providerCfg.patterns)
                          next[patIdx].inputs[inpIdx].item_id = e.target.value
                          patchConfig({ patterns: next })
                        }}
                      />
                      <input
                        type="number"
                        min={1}
                        value={inp.quantity}
                        onChange={(e) => {
                          const next = structuredClone(providerCfg.patterns)
                          next[patIdx].inputs[inpIdx].quantity = Math.max(
                            1,
                            Number(e.target.value) || 1
                          )
                          patchConfig({ patterns: next })
                        }}
                      />
                      <button
                        type="button"
                        className="btn btn-danger"
                        style={{ padding: '4px 0' }}
                        onClick={() => {
                          const next = structuredClone(providerCfg.patterns)
                          next[patIdx].inputs.splice(inpIdx, 1)
                          patchConfig({ patterns: next })
                        }}
                      >
                        ×
                      </button>
                    </div>
                  ))}
                </div>

                {/* Pattern Outputs */}
                <div className="inspector-field">
                  <div className="sub-editor-header">
                    <label>Outputs</label>
                    <button
                      type="button"
                      className="btn"
                      onClick={() => {
                        const next = structuredClone(providerCfg.patterns)
                        next[patIdx].outputs.push({
                          item_id: 'minecraft:chest',
                          quantity: 1,
                        })
                        patchConfig({ patterns: next })
                      }}
                    >
                      + Output
                    </button>
                  </div>
                  {pat.outputs.map((out, outIdx) => (
                    <div key={`out-${outIdx}`} className="stack-row">
                      <input
                        type="text"
                        value={out.item_id}
                        placeholder="modid:item"
                        onChange={(e) => {
                          const next = structuredClone(providerCfg.patterns)
                          next[patIdx].outputs[outIdx].item_id = e.target.value
                          patchConfig({ patterns: next })
                        }}
                      />
                      <input
                        type="number"
                        min={1}
                        value={out.quantity}
                        onChange={(e) => {
                          const next = structuredClone(providerCfg.patterns)
                          next[patIdx].outputs[outIdx].quantity = Math.max(
                            1,
                            Number(e.target.value) || 1
                          )
                          patchConfig({ patterns: next })
                        }}
                      />
                      <button
                        type="button"
                        className="btn btn-danger"
                        style={{ padding: '4px 0' }}
                        onClick={() => {
                          const next = structuredClone(providerCfg.patterns)
                          next[patIdx].outputs.splice(outIdx, 1)
                          patchConfig({ patterns: next })
                        }}
                      >
                        ×
                      </button>
                    </div>
                  ))}
                </div>
              </div>
            ))}
          </div>
        )}

        {/* Energy Acceptor */}
        {primaryBlock.block_def_id === 'ae2:energy_acceptor' && (
          <div className="inspector-card">
            <div className="inspector-section-title">Energy Conversion (FE → AE)</div>
            <div className="inspector-field">
              <label htmlFor="fe-input-field">FE Input / Tick (2 FE = 1 AE)</label>
              <input
                id="fe-input-field"
                aria-label="FE Input Per Tick"
                type="number"
                min={0}
                value={readNumberConfig(primaryBlock.config, 'fe_input_per_tick', 800)}
                onChange={(e) =>
                  patchConfig({
                    fe_input_per_tick: Math.max(0, Number(e.target.value) || 0),
                  })
                }
              />
              <span className="inspector-block-id">
                = {readNumberConfig(primaryBlock.config, 'fe_input_per_tick', 800) / 2} AE/t
                supplied
              </span>
            </div>
          </div>
        )}

        {/* Energy Cell / Dense Energy Cell */}
        {(primaryBlock.block_def_id === 'ae2:energy_cell' ||
          primaryBlock.block_def_id === 'ae2:dense_energy_cell') && (
          <div className="inspector-card">
            <div className="inspector-section-title">Energy Buffer Charge</div>
            <div className="inspector-field">
              <label htmlFor="charge-pct-field">Initial Charge (%)</label>
              <input
                id="charge-pct-field"
                aria-label="Initial Charge Percent"
                type="number"
                min={0}
                max={100}
                value={readNumberConfig(primaryBlock.config, 'initial_charge_pct', 100)}
                onChange={(e) =>
                  patchConfig({
                    initial_charge_pct: Math.min(
                      100,
                      Math.max(0, Number(e.target.value) || 0)
                    ),
                  })
                }
              />
            </div>
          </div>
        )}

        {/* External Processing Machine */}
        {(primaryBlock.block_def_id === 'minecraft:furnace' ||
          primaryBlock.block_def_id === 'minecraft:blast_furnace' ||
          primaryBlock.block_def_id === 'ae2:inscriber') && (
          <div className="inspector-card">
            <div className="inspector-section-title">Processing Machine Settings</div>
            <div className="inspector-field">
              <label htmlFor="machine-ticks-field">Process Ticks per Operation</label>
              <input
                id="machine-ticks-field"
                aria-label="Process Ticks"
                type="number"
                min={1}
                value={readNumberConfig(primaryBlock.config, 'process_ticks', 8)}
                onChange={(e) =>
                  patchConfig({
                    process_ticks: Math.max(1, Number(e.target.value) || 1),
                  })
                }
              />
            </div>
            <label className="inspector-checkbox-row">
              <input
                type="checkbox"
                aria-label="Auto Return"
                checked={readBooleanConfig(primaryBlock.config, 'auto_return', true)}
                onChange={(e) => patchConfig({ auto_return: e.target.checked })}
              />
              <span>Auto-Return Output to Adjacent Provider</span>
            </label>
          </div>
        )}

        {/* Live Overlay Telemetry */}
        {(blockTelemetry?.cable || blockTelemetry?.route) && (
          <div className="inspector-card">
            <div className="inspector-section-title">Live Node Telemetry</div>
            {blockTelemetry.cable && (
              <div className="inspector-desc mono">
                <div>Network: {blockTelemetry.cable.network_id}</div>
                <div>
                  Channels: {blockTelemetry.cable.channels_used} /{' '}
                  {blockTelemetry.cable.capacity}
                </div>
                <div>
                  Status:{' '}
                  {blockTelemetry.cable.overloaded
                    ? 'OVERLOADED'
                    : blockTelemetry.cable.online
                      ? 'ONLINE'
                      : 'OFFLINE'}
                </div>
              </div>
            )}
            {blockTelemetry.route && (
              <div className="inspector-desc mono">
                <div>Network: {blockTelemetry.route.network_id}</div>
                <div>
                  Channel Granted: {blockTelemetry.route.has_channel ? 'YES' : 'NO'}
                </div>
                <div>
                  Power Service: {blockTelemetry.route.online ? 'ONLINE' : 'OFFLINE'}
                </div>
                {blockTelemetry.route.route && blockTelemetry.route.route.length > 0 && (
                  <div>
                    Route Hops: {blockTelemetry.route.route.length} (
                    {blockTelemetry.route.route
                      .map((c) => `(${c.x},${c.y},${c.z})`)
                      .join(' → ')}
                    )
                  </div>
                )}
              </div>
            )}
          </div>
        )}

        <button
          type="button"
          className="btn btn-danger"
          onClick={() => removeBlocks([primaryBlock.id])}
        >
          Delete Block
        </button>
      </div>
    </aside>
  )
}
