import type { BatchBlockOperation } from '@/lib/api'

export interface SandboxPreset {
  id: string
  name: string
  description: string
  operations: BatchBlockOperation[]
}

/**
 * Complete AE2 1.21.1 installation modeled after the official AE2 Guide 3D showcase:
 * - Main network: ME Controller + ME Drive + Energy Acceptor + 3 Terminals on Blue Smart Cable + Blue Dense Smart trunk
 * - 2x3x1 Crafting CPU tower (Co-Processors + 16k Crafting Storage + Crafting Monitor) + 1x1x1 auxiliary CPU
 * - 2x2x2 Molecular Assembler & Pattern Provider cube
 * - Quartz Fiber-bridged Green Smart Cable subnetwork with Furnace + Hopper -> Pattern Provider + Inscriber
 */
export const GUIDE_SHOWCASE_PRESET: SandboxPreset = {
  id: 'guide-showcase',
  name: 'AE2 Complete Base Setup',
  description:
    'Controller, ME Drive, 3 Terminals, Dense/Smart trunks, 2x3x1 Crafting CPU, 2x2x2 Assembler cube, and Furnace/Hopper/Inscriber processing branch.',
  operations: [
    // Front Core Row (Z = 1, Y = 0)
    {
      operation: 'create',
      data: {
        block_def_id: 'ae2:energy_acceptor',
        x: 0,
        y: 0,
        z: 1,
        facing: 'south',
        config: { fe_input_per_tick: 1600 },
      },
    },
    {
      operation: 'create',
      data: {
        block_def_id: 'ae2:drive',
        x: 1,
        y: 0,
        z: 1,
        facing: 'south',
        config: {
          priority: 10,
          cells: [
            {
              tier: '16k',
              partition: [],
              inverter: false,
              items: [
                { item_id: 'minecraft:oak_log', quantity: 128 },
                { item_id: 'minecraft:raw_iron', quantity: 64 },
              ],
            },
            {
              tier: '4k',
              partition: [],
              inverter: false,
              items: [
                { item_id: 'ae2:certus_quartz_crystal', quantity: 64 },
                { item_id: 'minecraft:redstone', quantity: 64 },
              ],
            },
            { tier: '4k', partition: [], inverter: false, items: [] },
            { tier: '64k', partition: [], inverter: false, items: [] },
          ],
        },
      },
    },
    {
      operation: 'create',
      data: {
        block_def_id: 'ae2:controller',
        x: 2,
        y: 0,
        z: 1,
        facing: 'south',
        config: {},
      },
    },

    // Blue Dense Smart Cable Trunk (Right & Back to Controller/CPU/Assemblers)
    {
      operation: 'create',
      data: {
        block_def_id: 'ae2:dense_smart_cable',
        x: 3,
        y: 0,
        z: 1,
        facing: 'north',
        config: { color: 'blue', anchored_faces: [], subparts: {} },
      },
    },
    {
      operation: 'create',
      data: {
        block_def_id: 'ae2:dense_smart_cable',
        x: 4,
        y: 0,
        z: 1,
        facing: 'north',
        config: { color: 'blue', anchored_faces: [], subparts: {} },
      },
    },
    {
      operation: 'create',
      data: {
        block_def_id: 'ae2:dense_smart_cable',
        x: 4,
        y: 0,
        z: 0,
        facing: 'north',
        config: { color: 'blue', anchored_faces: [], subparts: {} },
      },
    },
    {
      operation: 'create',
      data: {
        block_def_id: 'ae2:dense_smart_cable',
        x: 2,
        y: 0,
        z: 0,
        facing: 'north',
        config: { color: 'blue', anchored_faces: [], subparts: {} },
      },
    },

    // Blue Smart Cable + 3 Illuminated Terminals (Y = 1, Z = 1)
    {
      operation: 'create',
      data: {
        block_def_id: 'ae2:smart_cable',
        x: 0,
        y: 1,
        z: 1,
        facing: 'south',
        config: {
          color: 'blue',
          anchored_faces: [],
          subparts: { south: 'ae2:terminal' },
        },
      },
    },
    {
      operation: 'create',
      data: {
        block_def_id: 'ae2:smart_cable',
        x: 1,
        y: 1,
        z: 1,
        facing: 'south',
        config: {
          color: 'blue',
          anchored_faces: [],
          subparts: { south: 'ae2:crafting_terminal' },
        },
      },
    },
    {
      operation: 'create',
      data: {
        block_def_id: 'ae2:smart_cable',
        x: 2,
        y: 1,
        z: 1,
        facing: 'south',
        config: {
          color: 'blue',
          anchored_faces: [],
          subparts: { south: 'ae2:pattern_encoding_terminal' },
        },
      },
    },
    // Blue Smart drop on left with Quartz Fiber bridge to Green processing subnetwork
    {
      operation: 'create',
      data: {
        block_def_id: 'ae2:smart_cable',
        x: -1,
        y: 1,
        z: 1,
        facing: 'west',
        config: {
          color: 'blue',
          anchored_faces: [],
          subparts: { west: 'ae2:quartz_fiber' },
        },
      },
    },
    {
      operation: 'create',
      data: {
        block_def_id: 'ae2:smart_cable',
        x: -1,
        y: 0,
        z: 1,
        facing: 'north',
        config: { color: 'blue', anchored_faces: [], subparts: {} },
      },
    },

    // Back-Center 2x3x1 Crafting CPU Multiblock Tower (X = 0..1, Y = 0..2, Z = -1)
    {
      operation: 'create',
      data: {
        block_def_id: 'ae2:crafting_storage_16k',
        x: 0,
        y: 0,
        z: -1,
        facing: 'south',
        config: {},
      },
    },
    {
      operation: 'create',
      data: {
        block_def_id: 'ae2:crafting_co_processor',
        x: 1,
        y: 0,
        z: -1,
        facing: 'south',
        config: {},
      },
    },
    {
      operation: 'create',
      data: {
        block_def_id: 'ae2:crafting_co_processor',
        x: 0,
        y: 1,
        z: -1,
        facing: 'south',
        config: {},
      },
    },
    {
      operation: 'create',
      data: {
        block_def_id: 'ae2:crafting_co_processor',
        x: 1,
        y: 1,
        z: -1,
        facing: 'south',
        config: {},
      },
    },
    {
      operation: 'create',
      data: {
        block_def_id: 'ae2:crafting_co_processor',
        x: 0,
        y: 2,
        z: -1,
        facing: 'south',
        config: {},
      },
    },
    {
      operation: 'create',
      data: {
        block_def_id: 'ae2:crafting_monitor',
        x: 1,
        y: 2,
        z: -1,
        facing: 'south',
        config: {},
      },
    },
    // Connect Controller trunk to CPU tower + Secondary 4k CPU block at (2, 1, -1)
    {
      operation: 'create',
      data: {
        block_def_id: 'ae2:dense_smart_cable',
        x: 2,
        y: 0,
        z: -1,
        facing: 'north',
        config: { color: 'blue', anchored_faces: [], subparts: {} },
      },
    },
    {
      operation: 'create',
      data: {
        block_def_id: 'ae2:crafting_storage_4k',
        x: 2,
        y: 1,
        z: -1,
        facing: 'south',
        config: {},
      },
    },

    // Back-Right 2x2x2 Molecular Assembler + Pattern Provider Checkerboard Cube (X = 3..4, Y = 0..1, Z = -2..-1)
    {
      operation: 'create',
      data: {
        block_def_id: 'ae2:pattern_provider',
        x: 3,
        y: 0,
        z: -1,
        facing: 'north',
        config: {
          priority: 10,
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
      },
    },
    {
      operation: 'create',
      data: {
        block_def_id: 'ae2:molecular_assembler',
        x: 4,
        y: 0,
        z: -1,
        facing: 'south',
        config: {},
      },
    },
    {
      operation: 'create',
      data: {
        block_def_id: 'ae2:molecular_assembler',
        x: 3,
        y: 1,
        z: -1,
        facing: 'south',
        config: {},
      },
    },
    {
      operation: 'create',
      data: {
        block_def_id: 'ae2:pattern_provider',
        x: 4,
        y: 1,
        z: -1,
        facing: 'north',
        config: { priority: 5, variant: 'normal', blocking_mode: false, lock_mode: 'none', patterns: [] },
      },
    },
    {
      operation: 'create',
      data: {
        block_def_id: 'ae2:molecular_assembler',
        x: 3,
        y: 0,
        z: -2,
        facing: 'south',
        config: {},
      },
    },
    {
      operation: 'create',
      data: {
        block_def_id: 'ae2:pattern_provider',
        x: 4,
        y: 0,
        z: -2,
        facing: 'north',
        config: { priority: 5, variant: 'normal', blocking_mode: false, lock_mode: 'none', patterns: [] },
      },
    },
    {
      operation: 'create',
      data: {
        block_def_id: 'ae2:pattern_provider',
        x: 3,
        y: 1,
        z: -2,
        facing: 'north',
        config: { priority: 5, variant: 'normal', blocking_mode: false, lock_mode: 'none', patterns: [] },
      },
    },
    {
      operation: 'create',
      data: {
        block_def_id: 'ae2:molecular_assembler',
        x: 4,
        y: 1,
        z: -2,
        facing: 'south',
        config: {},
      },
    },

    // Left Furnace + Hopper + Inscriber Processing Branch (X = -2 .. -4)
    {
      operation: 'create',
      data: {
        block_def_id: 'ae2:pattern_provider',
        x: -2,
        y: 0,
        z: 1,
        facing: 'up',
        config: {
          priority: 5,
          variant: 'normal',
          blocking_mode: false,
          lock_mode: 'none',
          patterns: [
            {
              id: 'pat-iron',
              pattern_type: 'processing',
              inputs: [{ item_id: 'minecraft:raw_iron', quantity: 1 }],
              outputs: [{ item_id: 'minecraft:iron_ingot', quantity: 1 }],
            },
          ],
        },
      },
    },
    {
      operation: 'create',
      data: {
        block_def_id: 'minecraft:furnace',
        x: -3,
        y: 1,
        z: 1,
        facing: 'south',
        config: { process_ticks: 6, auto_return: true },
      },
    },
    {
      operation: 'create',
      data: {
        block_def_id: 'minecraft:hopper',
        x: -3,
        y: 0,
        z: 1,
        facing: 'east',
        config: {},
      },
    },
    // Green Smart Cable subnetwork feeding Furnace & Inscriber
    {
      operation: 'create',
      data: {
        block_def_id: 'ae2:smart_cable',
        x: -2,
        y: 1,
        z: 1,
        facing: 'north',
        config: { color: 'green', anchored_faces: [], subparts: {} },
      },
    },
    {
      operation: 'create',
      data: {
        block_def_id: 'ae2:smart_cable',
        x: -2,
        y: 2,
        z: 1,
        facing: 'north',
        config: { color: 'green', anchored_faces: [], subparts: {} },
      },
    },
    {
      operation: 'create',
      data: {
        block_def_id: 'ae2:smart_cable',
        x: -3,
        y: 2,
        z: 1,
        facing: 'down',
        config: {
          color: 'green',
          anchored_faces: [],
          subparts: { down: 'ae2:export_bus' },
        },
      },
    },
    {
      operation: 'create',
      data: {
        block_def_id: 'ae2:inscriber',
        x: -4,
        y: 0,
        z: 1,
        facing: 'south',
        config: { process_ticks: 6 },
      },
    },
    {
      operation: 'create',
      data: {
        block_def_id: 'ae2:smart_cable',
        x: -4,
        y: 1,
        z: 1,
        facing: 'down',
        config: {
          color: 'green',
          anchored_faces: [],
          subparts: { down: 'ae2:import_bus' },
        },
      },
    },
    {
      operation: 'create',
      data: {
        block_def_id: 'ae2:smart_cable',
        x: -4,
        y: 2,
        z: 1,
        facing: 'north',
        config: { color: 'green', anchored_faces: [], subparts: {} },
      },
    },
  ],
}

export const FAULT_OVERLOAD_PRESET: SandboxPreset = {
  id: 'channel-fault',
  name: 'Ad-Hoc Channel Overload (Fault Demo)',
  description:
    'Controllerless ad-hoc network with 10 channel-consuming ME Drives exceeding the 8-channel limit.',
  operations: [
    {
      operation: 'create',
      data: {
        block_def_id: 'ae2:energy_acceptor',
        x: -1,
        y: 0,
        z: 0,
        facing: 'south',
        config: { fe_input_per_tick: 800 },
      },
    },
    ...[0, 1, 2, 3, 4].map(
      (x): BatchBlockOperation => ({
        operation: 'create',
        data: {
          block_def_id: 'ae2:smart_cable',
          x,
          y: 0,
          z: 0,
          facing: 'north',
          config: { color: 'fluix', anchored_faces: [], subparts: {} },
        },
      })
    ),
    ...[0, 1, 2, 3, 4].map(
      (x): BatchBlockOperation => ({
        operation: 'create',
        data: {
          block_def_id: 'ae2:drive',
          x,
          y: 0,
          z: -1,
          facing: 'south',
          config: { priority: 0, cells: [{ tier: '4k', items: [] }] },
        },
      })
    ),
    ...[0, 1, 2, 3, 4].map(
      (x): BatchBlockOperation => ({
        operation: 'create',
        data: {
          block_def_id: 'ae2:drive',
          x,
          y: 0,
          z: 1,
          facing: 'south',
          config: { priority: 0, cells: [{ tier: '4k', items: [] }] },
        },
      })
    ),
  ],
}

export const SANDBOX_PRESETS: readonly SandboxPreset[] = [
  GUIDE_SHOWCASE_PRESET,
  FAULT_OVERLOAD_PRESET,
]
