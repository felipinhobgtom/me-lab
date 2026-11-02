import type { BatchBlockOperation, Direction } from '../../lib/api'

function createOp(
  block_def_id: string,
  x: number,
  y: number,
  z: number,
  facing: Direction = 'north',
  config: Record<string, unknown> = {},
  label?: string
): BatchBlockOperation {
  return {
    operation: 'create',
    data: {
      block_def_id,
      x,
      y,
      z,
      facing,
      config,
      ...(label ? { label } : {}),
    },
  }
}

export function buildValidAutocraftingFixtureOps(): BatchBlockOperation[] {
  return [
    // Energy subsystem
    createOp(
      'ae2:energy_acceptor',
      -1,
      0,
      0,
      'north',
      { fe_input_per_tick: 800 },
      'Main FE Intake'
    ),
    createOp(
      'ae2:energy_cell',
      -1,
      1,
      0,
      'north',
      { initial_charge_pct: 100 },
      'Buffer Cell'
    ),

    // ME Controller core
    createOp('ae2:controller', 0, 0, 0, 'north', {}, 'Core Controller'),

    // Dense trunk + Smart cable branch
    createOp(
      'ae2:dense_smart_cable',
      1,
      0,
      0,
      'north',
      { color: 'fluix', anchored_faces: [] },
      'Dense Trunk W'
    ),
    createOp(
      'ae2:dense_smart_cable',
      2,
      0,
      0,
      'north',
      { color: 'fluix', anchored_faces: [] },
      'Dense Trunk E'
    ),
    createOp('ae2:smart_cable', 3, 0, 0, 'north', { color: 'fluix', anchored_faces: [] }),
    createOp('ae2:smart_cable', 3, 0, 1, 'north', { color: 'fluix', anchored_faces: [] }),

    // Primary ME Drive with oak_log & iron_ingot inventory
    createOp(
      'ae2:drive',
      2,
      0,
      1,
      'north',
      {
        priority: 10,
        cells: [
          {
            tier: '4k',
            partition: [],
            inverter: false,
            items: [
              { item_id: 'minecraft:oak_log', quantity: 64 },
              { item_id: 'minecraft:iron_ingot', quantity: 32 },
            ],
          },
        ],
      },
      'Primary Storage Drive'
    ),

    // Operator terminal
    createOp('ae2:crafting_terminal', 3, 1, 0, 'south', {}, 'Main Crafting Terminal'),

    // 1x2x1 Crafting CPU multiblock (4k Storage + Co-Processor)
    createOp('ae2:crafting_storage_4k', 1, 0, -1, 'north', {}, 'CPU 4k Storage'),
    createOp('ae2:crafting_co_processor', 1, 1, -1, 'north', {}, 'CPU Co-Processor'),

    // Autocrafting Pattern Provider + 2 adjacent Molecular Assemblers
    createOp(
      'ae2:pattern_provider',
      4,
      0,
      0,
      'east',
      {
        priority: 5,
        variant: 'normal',
        blocking_mode: false,
        lock_mode: 'none',
        patterns: [
          {
            id: 'pat-planks',
            pattern_type: 'crafting',
            inputs: [{ item_id: 'minecraft:oak_log', quantity: 1 }],
            outputs: [{ item_id: 'minecraft:oak_planks', quantity: 4 }],
          },
          {
            id: 'pat-chest',
            pattern_type: 'crafting',
            inputs: [{ item_id: 'minecraft:oak_planks', quantity: 8 }],
            outputs: [{ item_id: 'minecraft:chest', quantity: 1 }],
          },
        ],
      },
      'Woodshop Provider'
    ),
    createOp('ae2:molecular_assembler', 5, 0, 0, 'north', {}, 'Assembler East'),
    createOp('ae2:molecular_assembler', 4, 1, 0, 'north', {}, 'Assembler Top'),
  ]
}

export function buildChannelBottleneckFixtureOps(): BatchBlockOperation[] {
  return [
    // Powered ad-hoc network WITHOUT an ME Controller
    createOp('ae2:energy_acceptor', -1, 0, 0, 'north', { fe_input_per_tick: 800 }),
    createOp('ae2:energy_cell', -1, 1, 0, 'north', { initial_charge_pct: 100 }),

    // Smart cable spine (capacity 8, no controller -> ad-hoc 8 channel max)
    createOp('ae2:smart_cable', 0, 0, 0, 'north', { color: 'fluix', anchored_faces: [] }),
    createOp('ae2:smart_cable', 1, 0, 0, 'north', { color: 'fluix', anchored_faces: [] }),
    createOp('ae2:smart_cable', 2, 0, 0, 'north', { color: 'fluix', anchored_faces: [] }),
    createOp('ae2:smart_cable', 3, 0, 0, 'north', { color: 'fluix', anchored_faces: [] }),
    createOp('ae2:smart_cable', 4, 0, 0, 'north', { color: 'fluix', anchored_faces: [] }),

    // 10 channel-consuming devices (exceeding 8-channel ad-hoc limit!)
    createOp('ae2:drive', 0, 0, 1, 'south', { priority: 10, cells: [{ tier: '1k', items: [] }] }, 'Drive 1'),
    createOp('ae2:drive', 1, 0, 1, 'south', { priority: 9, cells: [{ tier: '1k', items: [] }] }, 'Drive 2'),
    createOp('ae2:drive', 2, 0, 1, 'south', { priority: 8, cells: [{ tier: '1k', items: [] }] }, 'Drive 3'),
    createOp('ae2:drive', 3, 0, 1, 'south', { priority: 7, cells: [{ tier: '1k', items: [] }] }, 'Drive 4'),
    createOp('ae2:crafting_terminal', 4, 0, 1, 'south', {}, 'Crafting Term'),
    createOp('ae2:terminal', 0, 0, -1, 'north', {}, 'Access Term 1'),
    createOp('ae2:terminal', 1, 0, -1, 'north', {}, 'Access Term 2'),
    createOp('ae2:terminal', 2, 0, -1, 'north', {}, 'Access Term 3'),
    createOp('ae2:pattern_provider', 3, 0, -1, 'north', { priority: 0, variant: 'normal', patterns: [] }, 'Provider 1'),
    createOp('ae2:pattern_provider', 4, 0, -1, 'north', { priority: 0, variant: 'normal', patterns: [] }, 'Provider 2'),
  ]
}
