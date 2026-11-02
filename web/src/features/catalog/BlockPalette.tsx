import { useMemo, useState } from 'react'
import type { BlockDefinition } from '../../lib/api'
import { useEditorStore } from '../editor/store'
import './BlockPalette.css'

function formatChannelBadge(def: BlockDefinition): string {
  const meta = def.metadata
  if (def.id === 'ae2:controller') {
    return `${meta.channel_capacity || 32} Ch/Face`
  }
  if (meta.is_cable) {
    return `Cap ${meta.channel_capacity}`
  }
  if (meta.channel_usage > 0) {
    return `Uses ${meta.channel_usage} Ch`
  }
  if (meta.transmits_channels && meta.channel_capacity > 0) {
    return `Pass ${meta.channel_capacity} Ch`
  }
  return '0 Ch'
}

function shapeGlyph(shape: string): string {
  switch (shape) {
    case 'cable':
      return '┼'
    case 'dense_cable':
      return '╋'
    case 'core':
      return '◈'
    case 'frame':
      return '⊞'
    case 'panel':
      return '▤'
    default:
      return '■'
  }
}

export function BlockPalette() {
  const catalog = useEditorStore((s) => s.catalog)
  const activeBlockDefId = useEditorStore((s) => s.activeBlockDefId)
  const setActiveBlockDefId = useEditorStore((s) => s.setActiveBlockDefId)
  const setActiveMode = useEditorStore((s) => s.setActiveMode)
  const placementFacing = useEditorStore((s) => s.placementFacing)

  const [category, setCategory] = useState<string>('all')
  const [search, setSearch] = useState<string>('')

  const categories = useMemo(() => {
    const unique = new Set<string>()
    for (const item of catalog) {
      if (item.category) {
        unique.add(item.category)
      }
    }
    return ['all', ...Array.from(unique)]
  }, [catalog])

  const filteredBlocks = useMemo(() => {
    const q = search.trim().toLowerCase()
    return catalog.filter((def) => {
      if (category !== 'all' && def.category !== category) {
        return false
      }
      if (!q) {
        return true
      }
      return (
        def.name.toLowerCase().includes(q) ||
        def.id.toLowerCase().includes(q) ||
        def.category.toLowerCase().includes(q)
      )
    })
  }, [catalog, category, search])

  const handleSelectBrush = (blockId: string) => {
    setActiveBlockDefId(blockId)
    setActiveMode('build')
  }

  return (
    <aside className="block-palette" aria-label="Block Palette">
      <div className="palette-header">
        <div className="palette-title-row">
          <span className="palette-title">AE2 Component Catalog</span>
          <span className="badge">Face: {placementFacing}</span>
        </div>

        <input
          type="search"
          aria-label="Search Catalog"
          className="palette-search"
          placeholder="Filter blocks or IDs…"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
        />

        <div className="palette-categories" role="group" aria-label="Block Categories">
          {categories.map((cat) => (
            <button
              key={cat}
              type="button"
              className={`category-chip ${category === cat ? 'active' : ''}`}
              onClick={() => setCategory(cat)}
            >
              {cat.replace(/_/g, ' ')}
            </button>
          ))}
        </div>

        <button
          type="button"
          className={`btn palette-cursor-btn ${activeBlockDefId === null ? 'active' : ''}`}
          onClick={() => setActiveBlockDefId(null)}
        >
          ⌖ Select / Inspect Cursor (Esc)
        </button>
      </div>

      <div className="palette-list">
        {filteredBlocks.length === 0 ? (
          <div className="palette-empty">No catalog blocks match your filter.</div>
        ) : (
          filteredBlocks.map((def) => {
            const isSelected = activeBlockDefId === def.id
            const visualColor = def.metadata.visual.color || '#64748b'
            const accentColor = def.metadata.visual.accent_color || visualColor

            return (
              <button
                key={def.id}
                type="button"
                data-testid={`palette-item-${def.id}`}
                className={`palette-card ${isSelected ? 'selected' : ''}`}
                onClick={() => handleSelectBrush(def.id)}
              >
                <div className="palette-card-top">
                  <span
                    className="palette-swatch"
                    style={{
                      backgroundColor: visualColor,
                      borderColor: accentColor,
                    }}
                    aria-hidden="true"
                  >
                    {shapeGlyph(def.metadata.visual.shape)}
                  </span>
                  <div className="palette-card-info">
                    <div className="palette-card-name">{def.name}</div>
                    <div className="palette-card-id">{def.id}</div>
                  </div>
                </div>

                <div className="palette-card-badges">
                  <span className="badge">{formatChannelBadge(def)}</span>
                  <span className={`badge badge-${def.fidelity}`}>{def.fidelity}</span>
                </div>
              </button>
            )
          })
        )}
      </div>
    </aside>
  )
}
