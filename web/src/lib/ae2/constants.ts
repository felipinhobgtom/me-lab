import type {
  BlockDefinition,
  CableChannelOverlay,
  Coord3D,
  Direction,
  DriveCellConfig,
  ItemStack,
  PatternSpec,
  PlacedBlock,
} from '../api'

export type CableColor =
  | 'fluix'
  | 'blue'
  | 'red'
  | 'green'
  | 'yellow'
  | 'purple'
  | 'cyan'
  | 'orange'
  | 'white'

export const CABLE_COLORS: readonly CableColor[] = [
  'fluix',
  'blue',
  'red',
  'green',
  'yellow',
  'purple',
  'cyan',
  'orange',
  'white',
] as const

export const CABLE_COLOR_HEX: Record<CableColor, string> = {
  fluix: '#8b5cf6',
  blue: '#3b82f6',
  red: '#ef4444',
  green: '#10b981',
  yellow: '#eab308',
  purple: '#a855f7',
  cyan: '#06b6d4',
  orange: '#f97316',
  white: '#f8fafc',
}

export const NETWORK_PALETTE = [
  '#38bdf8',
  '#8b5cf6',
  '#34d399',
  '#f59e0b',
  '#ec4899',
  '#2dd4bf',
  '#a78bfa',
  '#fb7185',
]

export const ALL_DIRECTIONS: readonly Direction[] = [
  'north',
  'east',
  'south',
  'west',
  'up',
  'down',
] as const

export const CELL_TIERS: readonly DriveCellConfig['tier'][] = [
  '1k',
  '4k',
  '16k',
  '64k',
  '256k',
] as const

export function coordKey(x: number, y: number, z: number): string {
  return `${x},${y},${z}`
}

export function rotateDirectionClockwise(dir: Direction): Direction {
  switch (dir) {
    case 'north':
      return 'east'
    case 'east':
      return 'south'
    case 'south':
      return 'west'
    case 'west':
      return 'north'
    case 'up':
      return 'down'
    case 'down':
      return 'up'
  }
}

export function oppositeDirection(dir: Direction): Direction {
  switch (dir) {
    case 'north':
      return 'south'
    case 'south':
      return 'north'
    case 'east':
      return 'west'
    case 'west':
      return 'east'
    case 'up':
      return 'down'
    case 'down':
      return 'up'
  }
}

export function directionOffset(dir: Direction): Coord3D {
  switch (dir) {
    case 'north':
      return { x: 0, y: 0, z: -1 }
    case 'south':
      return { x: 0, y: 0, z: 1 }
    case 'east':
      return { x: 1, y: 0, z: 0 }
    case 'west':
      return { x: -1, y: 0, z: 0 }
    case 'up':
      return { x: 0, y: 1, z: 0 }
    case 'down':
      return { x: 0, y: -1, z: 0 }
  }
}

export function directionArrow2D(dir: Direction): string {
  switch (dir) {
    case 'north':
      return '↑'
    case 'south':
      return '↓'
    case 'east':
      return '→'
    case 'west':
      return '←'
    case 'up':
      return '▲'
    case 'down':
      return '▼'
  }
}

export function networkColorForId(networkId: string | undefined): string {
  if (!networkId) {
    return '#64748b'
  }
  let hash = 0
  for (let i = 0; i < networkId.length; i++) {
    hash = (hash * 31 + networkId.charCodeAt(i)) | 0
  }
  const idx = Math.abs(hash) % NETWORK_PALETTE.length
  return NETWORK_PALETTE[idx]
}

export function defaultConfigForBlock(
  def: BlockDefinition | undefined,
  blockDefId: string
): Record<string, unknown> {
  if (
    blockDefId === 'ae2:fluix_cable' ||
    blockDefId === 'ae2:smart_cable' ||
    blockDefId === 'ae2:dense_cable' ||
    blockDefId === 'ae2:dense_smart_cable' ||
    def?.metadata.is_cable
  ) {
    return {
      color: 'fluix',
      anchored_faces: [],
    }
  }

  switch (blockDefId) {
    case 'ae2:energy_acceptor':
      return { fe_input_per_tick: 800 }
    case 'ae2:energy_cell':
    case 'ae2:dense_energy_cell':
      return { initial_charge_pct: 100 }
    case 'ae2:drive':
      return {
        priority: 0,
        cells: [
          {
            tier: '4k',
            partition: [],
            inverter: false,
            items: [],
          },
        ],
      }
    case 'ae2:pattern_provider':
      return {
        priority: 0,
        variant: 'normal',
        blocking_mode: false,
        lock_mode: 'none',
        patterns: [],
      }
    case 'melab:external_machine':
      return {
        process_ticks: 8,
        auto_return: true,
      }
    default: {
      const generated: Record<string, unknown> = {}
      if (def?.metadata.config_schema) {
        for (const field of def.metadata.config_schema) {
          if (field.default !== undefined) {
            generated[field.key] = structuredClone(field.default)
          }
        }
      }
      return generated
    }
  }
}

function isCableColor(val: unknown): val is CableColor {
  return typeof val === 'string' && (CABLE_COLORS as readonly string[]).includes(val)
}

function isDirection(val: unknown): val is Direction {
  return typeof val === 'string' && (ALL_DIRECTIONS as readonly string[]).includes(val)
}

function isCellTier(val: unknown): val is DriveCellConfig['tier'] {
  return typeof val === 'string' && (CELL_TIERS as readonly string[]).includes(val)
}

export function readCableConfig(config: Record<string, unknown> | undefined): {
  color: CableColor
  anchored_faces: Direction[]
} {
  const colorRaw = config?.color
  const color: CableColor = isCableColor(colorRaw) ? colorRaw : 'fluix'
  const rawFaces = config?.anchored_faces
  const anchored_faces: Direction[] = Array.isArray(rawFaces)
    ? rawFaces.filter(isDirection)
    : []
  return { color, anchored_faces }
}

function parseItemStackList(raw: unknown): ItemStack[] {
  if (!Array.isArray(raw)) {
    return []
  }
  const items: ItemStack[] = []
  for (const entry of raw) {
    if (entry && typeof entry === 'object' && 'item_id' in entry && 'quantity' in entry) {
      const itemId = typeof entry.item_id === 'string' ? entry.item_id : ''
      const qty = typeof entry.quantity === 'number' ? entry.quantity : Number(entry.quantity) || 1
      items.push({ item_id: itemId, quantity: Math.max(1, Math.floor(qty)) })
    }
  }
  return items
}

export function readDriveConfig(config: Record<string, unknown> | undefined): {
  priority: number
  cells: Required<DriveCellConfig>[]
} {
  const rawPriority = config?.priority
  const priority = typeof rawPriority === 'number' ? rawPriority : Number(rawPriority) || 0
  const rawCells = config?.cells
  const cells: Required<DriveCellConfig>[] = []
  if (Array.isArray(rawCells)) {
    for (const c of rawCells) {
      if (c && typeof c === 'object') {
        const tierVal = 'tier' in c ? c.tier : '4k'
        const tier = isCellTier(tierVal) ? tierVal : '4k'
        const partition =
          'partition' in c && Array.isArray(c.partition)
            ? c.partition.filter((p: unknown): p is string => typeof p === 'string')
            : []
        const inverter = 'inverter' in c && typeof c.inverter === 'boolean' ? c.inverter : false
        const items = 'items' in c ? parseItemStackList(c.items) : []
        cells.push({ tier, partition, inverter, items })
      }
    }
  }
  return { priority, cells }
}

export function readPatternProviderConfig(config: Record<string, unknown> | undefined): {
  priority: number
  variant: 'normal' | 'directional'
  blocking_mode: boolean
  lock_mode: 'none' | 'until_result_returned'
  patterns: PatternSpec[]
} {
  const rawPriority = config?.priority
  const priority = typeof rawPriority === 'number' ? rawPriority : Number(rawPriority) || 0
  const variant = config?.variant === 'directional' ? 'directional' : 'normal'
  const blocking_mode = typeof config?.blocking_mode === 'boolean' ? config.blocking_mode : false
  const lock_mode =
    config?.lock_mode === 'until_result_returned' ? 'until_result_returned' : 'none'

  const patterns: PatternSpec[] = []
  if (Array.isArray(config?.patterns)) {
    for (const p of config.patterns) {
      if (p && typeof p === 'object') {
        const id = 'id' in p && typeof p.id === 'string' ? p.id : undefined
        const pattern_type =
          'pattern_type' in p && p.pattern_type === 'processing' ? 'processing' : 'crafting'
        const inputs = 'inputs' in p ? parseItemStackList(p.inputs) : []
        const outputs = 'outputs' in p ? parseItemStackList(p.outputs) : []
        patterns.push({ id, pattern_type, inputs, outputs })
      }
    }
  }

  return { priority, variant, blocking_mode, lock_mode, patterns }
}

export function readNumberConfig(
  config: Record<string, unknown> | undefined,
  key: string,
  fallback: number
): number {
  const raw = config?.[key]
  if (typeof raw === 'number' && Number.isFinite(raw)) {
    return raw
  }
  if (typeof raw === 'string' && raw.trim() !== '') {
    const parsed = Number(raw)
    if (Number.isFinite(parsed)) {
      return parsed
    }
  }
  return fallback
}

export function readBooleanConfig(
  config: Record<string, unknown> | undefined,
  key: string,
  fallback: boolean
): boolean {
  const raw = config?.[key]
  return typeof raw === 'boolean' ? raw : fallback
}

export function isCableBlock(block: PlacedBlock, def?: BlockDefinition): boolean {
  if (def) {
    return def.metadata.is_cable
  }
  return (
    block.block_def_id === 'ae2:fluix_cable' ||
    block.block_def_id === 'ae2:smart_cable' ||
    block.block_def_id === 'ae2:dense_cable' ||
    block.block_def_id === 'ae2:dense_smart_cable'
  )
}

function canBlockConnectOnFace(
  block: PlacedBlock,
  face: Direction,
  def?: BlockDefinition
): boolean {
  if (block.block_def_id === 'ae2:cable_anchor') {
    return false
  }
  if (isCableBlock(block, def)) {
    const { anchored_faces } = readCableConfig(block.config)
    if (anchored_faces.includes(face)) {
      return false
    }
  }
  if (block.block_def_id === 'ae2:pattern_provider') {
    const { variant } = readPatternProviderConfig(block.config)
    if (variant === 'directional' && block.facing === face) {
      return false
    }
  }
  if (def?.metadata.connection_rules.connect_only_facing) {
    return block.facing === face
  }
  return true
}

export function canBlocksConnect(
  a: PlacedBlock,
  dirFromAToB: Direction,
  b: PlacedBlock,
  catalogMap: Map<string, BlockDefinition>
): boolean {
  const defA = catalogMap.get(a.block_def_id)
  const defB = catalogMap.get(b.block_def_id)
  const opp = oppositeDirection(dirFromAToB)

  if (!canBlockConnectOnFace(a, dirFromAToB, defA)) {
    return false
  }
  if (!canBlockConnectOnFace(b, opp, defB)) {
    return false
  }

  if (isCableBlock(a, defA) && isCableBlock(b, defB)) {
    const colorA = readCableConfig(a.config).color
    const colorB = readCableConfig(b.config).color
    if (colorA !== 'fluix' && colorB !== 'fluix' && colorA !== colorB) {
      return false
    }
  }

  return true
}

export function computeConnectedDirections(
  block: PlacedBlock,
  blocksByCoord: Map<string, PlacedBlock>,
  catalogMap: Map<string, BlockDefinition>,
  cableOverlay?: CableChannelOverlay
): Direction[] {
  if (cableOverlay && cableOverlay.connections.length > 0) {
    const dirs: Direction[] = []
    for (const dir of ALL_DIRECTIONS) {
      const off = directionOffset(dir)
      const nx = block.x + off.x
      const ny = block.y + off.y
      const nz = block.z + off.z
      if (cableOverlay.connections.some((c) => c.x === nx && c.y === ny && c.z === nz)) {
        dirs.push(dir)
      }
    }
    return dirs
  }

  const dirs: Direction[] = []
  for (const dir of ALL_DIRECTIONS) {
    const off = directionOffset(dir)
    const neighbor = blocksByCoord.get(coordKey(block.x + off.x, block.y + off.y, block.z + off.z))
    if (neighbor && canBlocksConnect(block, dir, neighbor, catalogMap)) {
      dirs.push(dir)
    }
  }
  return dirs
}

export function resolveBlockColor(block: PlacedBlock, def?: BlockDefinition): string {
  if (isCableBlock(block, def)) {
    const { color } = readCableConfig(block.config)
    return CABLE_COLOR_HEX[color]
  }
  return def?.metadata.visual.color ?? '#64748b'
}
