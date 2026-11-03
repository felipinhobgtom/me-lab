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
  | 'green'
  | 'red'
  | 'yellow'
  | 'cyan'
  | 'purple'
  | 'orange'
  | 'white'
  | 'black'
  | 'lime'
  | 'magenta'
  | 'light_blue'

export const CABLE_COLORS: readonly CableColor[] = [
  'fluix',
  'blue',
  'green',
  'red',
  'yellow',
  'cyan',
  'purple',
  'orange',
  'white',
  'black',
  'lime',
  'magenta',
  'light_blue',
] as const

export const CABLE_COLOR_HEX: Record<CableColor, string> = {
  fluix: '#8f5ccb',
  blue: '#2e88f5',
  green: '#22c55e',
  red: '#ef4444',
  yellow: '#eab308',
  cyan: '#06b6d4',
  purple: '#a855f7',
  orange: '#f97316',
  white: '#f3f4f6',
  black: '#27272a',
  lime: '#84cc16',
  magenta: '#d946ef',
  light_blue: '#38bdf8',
}

export const SUBPART_BLOCK_IDS: Record<string, true> = {
  'ae2:terminal': true,
  'ae2:crafting_terminal': true,
  'ae2:pattern_encoding_terminal': true,
  'ae2:pattern_access_terminal': true,
  'ae2:import_bus': true,
  'ae2:export_bus': true,
  'ae2:storage_bus': true,
  'ae2:quartz_fiber': true,
  'ae2:cable_anchor': true,
}

export const NON_ME_MACHINE_IDS: Record<string, true> = {
  'minecraft:furnace': true,
  'minecraft:blast_furnace': true,
  'minecraft:hopper': true,
  'minecraft:barrel': true,
  'ae2:inscriber': true,
  'ae2:charger': true,
  'ae2:cable_anchor': true,
  'ae2:quartz_fiber': true,
}

export const NETWORK_PALETTE = [
  '#4ade80',
  '#38bdf8',
  '#f59e0b',
  '#a78bfa',
  '#f472b6',
  '#2dd4bf',
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

export function cycleAllDirections(dir: Direction): Direction {
  const idx = ALL_DIRECTIONS.indexOf(dir)
  return ALL_DIRECTIONS[(idx + 1) % ALL_DIRECTIONS.length]
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
    return '#737373'
  }
  let hash = 0
  for (let i = 0; i < networkId.length; i++) {
    hash = (hash * 31 + networkId.charCodeAt(i)) | 0
  }
  const idx = Math.abs(hash) % NETWORK_PALETTE.length
  return NETWORK_PALETTE[idx]
}

export function isSubpartBlockId(blockDefId: string): boolean {
  return Boolean(SUBPART_BLOCK_IDS[blockDefId])
}

export function isCableBlockId(blockDefId: string, def?: BlockDefinition): boolean {
  if (def?.metadata.is_cable) {
    return true
  }
  return (
    blockDefId === 'ae2:fluix_cable' ||
    blockDefId === 'ae2:covered_cable' ||
    blockDefId === 'ae2:smart_cable' ||
    blockDefId === 'ae2:dense_cable' ||
    blockDefId === 'ae2:dense_smart_cable'
  )
}

export function isDenseCableBlockId(blockDefId: string, def?: BlockDefinition): boolean {
  if (def?.metadata.is_dense_cable) {
    return true
  }
  return blockDefId === 'ae2:dense_cable' || blockDefId === 'ae2:dense_smart_cable'
}

export function defaultConfigForBlock(
  def: BlockDefinition | undefined,
  blockDefId: string,
  activeColor: CableColor = 'fluix'
): Record<string, unknown> {
  if (isCableBlockId(blockDefId, def)) {
    return {
      color: activeColor,
      anchored_faces: [],
      subparts: {},
    }
  }

  switch (blockDefId) {
    case 'ae2:energy_acceptor':
      return { fe_input_per_tick: 800 }
    case 'ae2:energy_cell':
    case 'ae2:dense_energy_cell':
      return { initial_charge_pct: 100 }
    case 'ae2:drive':
    case 'ae2:chest':
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
    case 'ae2:import_bus':
    case 'ae2:export_bus':
    case 'ae2:storage_bus':
      return { filters: [], inverter: false, acceleration: 0 }
    case 'ae2:pattern_provider':
      return {
        priority: 0,
        variant: 'normal',
        blocking_mode: false,
        lock_mode: 'none',
        patterns: [],
      }
    case 'minecraft:furnace':
      return { process_ticks: 8, auto_return: true }
    case 'minecraft:blast_furnace':
      return { process_ticks: 4, auto_return: true }
    case 'ae2:inscriber':
      return { process_ticks: 6, auto_return: true }
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

export function isCableColor(val: unknown): val is CableColor {
  return typeof val === 'string' && (CABLE_COLORS as readonly string[]).includes(val)
}

function isDirection(val: unknown): val is Direction {
  return typeof val === 'string' && (ALL_DIRECTIONS as readonly string[]).includes(val)
}

function isCellTier(val: unknown): val is DriveCellConfig['tier'] {
  return typeof val === 'string' && (CELL_TIERS as readonly string[]).includes(val)
}

export function readCableSubparts(
  config: Record<string, unknown> | undefined
): Partial<Record<Direction, string>> {
  const out: Partial<Record<Direction, string>> = {}
  const raw = config?.subparts
  if (!raw || typeof raw !== 'object') {
    return out
  }
  for (const [k, v] of Object.entries(raw)) {
    if (!isDirection(k)) continue
    if (typeof v === 'string' && v.trim() !== '') {
      out[k] = v.trim()
    } else if (v && typeof v === 'object' && 'block_def_id' in v && typeof v.block_def_id === 'string') {
      out[k] = v.block_def_id
    }
  }
  return out
}

export function readCableConfig(config: Record<string, unknown> | undefined): {
  color: CableColor
  anchored_faces: Direction[]
  subparts: Partial<Record<Direction, string>>
} {
  const colorRaw = config?.color
  const color: CableColor = isCableColor(colorRaw) ? colorRaw : 'fluix'
  const rawFaces = config?.anchored_faces
  const anchored_faces: Direction[] = Array.isArray(rawFaces)
    ? rawFaces.filter(isDirection)
    : []
  const subparts = readCableSubparts(config)
  return { color, anchored_faces, subparts }
}

function parseItemStackList(raw: unknown): ItemStack[] {
  if (!Array.isArray(raw)) {
    return []
  }
  const out: ItemStack[] = []
  for (const entry of raw) {
    if (
      entry &&
      typeof entry === 'object' &&
      'item_id' in entry &&
      typeof entry.item_id === 'string' &&
      'quantity' in entry &&
      typeof entry.quantity === 'number'
    ) {
      out.push({ item_id: entry.item_id, quantity: Math.max(1, Math.floor(entry.quantity)) })
    }
  }
  return out
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
  const blocking_mode = config?.blocking_mode === true
  const lock_mode =
    config?.lock_mode === 'until_result_returned' ? 'until_result_returned' : 'none'

  const rawPatterns = config?.patterns
  const patterns: PatternSpec[] = []
  if (Array.isArray(rawPatterns)) {
    for (let idx = 0; idx < rawPatterns.length; idx++) {
      const p = rawPatterns[idx]
      if (p && typeof p === 'object') {
        const id =
          'id' in p && typeof p.id === 'string' && p.id.length > 0 ? p.id : `pat-${idx + 1}`
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

export function readBusConfig(config: Record<string, unknown> | undefined): {
  filters: string[]
  inverter: boolean
  acceleration: number
} {
  const rawFilters = config?.filters
  const filters = Array.isArray(rawFilters)
    ? rawFilters.filter((item): item is string => typeof item === 'string' && item.trim() !== '')
    : typeof config?.filter === 'string' && config.filter.trim() !== ''
      ? [config.filter.trim()]
      : []
  const accelerationRaw =
    typeof config?.acceleration === 'number'
      ? config.acceleration
      : Number(config?.acceleration) || 0
  return {
    filters: [...new Set(filters.map((item) => item.trim()))],
    inverter: config?.inverter === true,
    acceleration: Math.min(4, Math.max(0, Math.floor(accelerationRaw))),
  }
}

export function isCableBlock(block: PlacedBlock, def?: BlockDefinition): boolean {
  return isCableBlockId(block.block_def_id, def)
}

function canBlockConnectOnFace(
  block: PlacedBlock,
  face: Direction,
  def?: BlockDefinition
): boolean {
  if (NON_ME_MACHINE_IDS[block.block_def_id]) {
    return false
  }
  if (def?.fidelity === 'unsupported') {
    return false
  }
  if (isCableBlock(block, def)) {
    const { anchored_faces, subparts } = readCableConfig(block.config)
    if (anchored_faces.includes(face)) {
      return false
    }
    const subOnFace = subparts[face]
    if (subOnFace === 'ae2:cable_anchor' || subOnFace === 'ae2:quartz_fiber') {
      return false
    }
    // Terminals/buses mounted on `face` occupy that face toward external blocks
    if (subOnFace) {
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
  return def?.metadata.visual.color ?? '#525252'
}

export function readNumberConfig(
  config: Record<string, unknown> | undefined,
  key: string,
  defaultValue: number
): number {
  const val = config?.[key]
  if (typeof val === 'number' && Number.isFinite(val)) {
    return val
  }
  if (typeof val === 'string' && val.trim() !== '') {
    const parsed = Number(val)
    if (Number.isFinite(parsed)) {
      return parsed
    }
  }
  return defaultValue
}

export function readBooleanConfig(
  config: Record<string, unknown> | undefined,
  key: string,
  defaultValue: boolean
): boolean {
  const val = config?.[key]
  if (typeof val === 'boolean') {
    return val
  }
  return defaultValue
}
