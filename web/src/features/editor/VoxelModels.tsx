import { useMemo } from 'react'
import * as THREE from 'three'
import type { BlockDefinition, CableChannelOverlay, Direction, PlacedBlock } from '@/lib/api'
import {
  ALL_DIRECTIONS,
  CABLE_COLOR_HEX,
  coordKey,
  directionOffset,
  readCableConfig,
  readDriveConfig,
  readPatternProviderConfig,
} from '@/lib/ae2/constants'
import {
  cableColorToAssetName,
  getTerminalScreenTexture,
  loadPixelTexture,
} from './textures'

/**
 * Three.js BoxGeometry material index order:
 * 0: +X (east), 1: -X (west), 2: +Y (up), 3: -Y (down), 4: +Z (south), 5: -Z (north)
 */
function buildCubeMaterials(faces: {
  north: THREE.Texture
  south: THREE.Texture
  east: THREE.Texture
  west: THREE.Texture
  up: THREE.Texture
  down: THREE.Texture
  opacity?: number
  transparent?: boolean
  alphaTest?: number
}): THREE.MeshStandardMaterial[] {
  const opts = {
    roughness: 0.85,
    metalness: 0.05,
    transparent: faces.transparent ?? (faces.opacity !== undefined && faces.opacity < 1),
    opacity: faces.opacity ?? 1,
    alphaTest: faces.alphaTest ?? 0,
  }
  return [
    new THREE.MeshStandardMaterial({ ...opts, map: faces.east }),
    new THREE.MeshStandardMaterial({ ...opts, map: faces.west }),
    new THREE.MeshStandardMaterial({ ...opts, map: faces.up }),
    new THREE.MeshStandardMaterial({ ...opts, map: faces.down }),
    new THREE.MeshStandardMaterial({ ...opts, map: faces.south }),
    new THREE.MeshStandardMaterial({ ...opts, map: faces.north }),
  ]
}

function orientHorizontalFaces(
  facing: Direction,
  front: THREE.Texture,
  side: THREE.Texture,
  back: THREE.Texture,
  top: THREE.Texture,
  bottom: THREE.Texture
) {
  const mapByDir: Record<Direction, THREE.Texture> = {
    north: side,
    south: side,
    east: side,
    west: side,
    up: top,
    down: bottom,
  }
  if (facing === 'up') {
    mapByDir.up = front
    mapByDir.down = back
  } else if (facing === 'down') {
    mapByDir.down = front
    mapByDir.up = back
  } else {
    mapByDir[facing] = front
    const opp =
      facing === 'north'
        ? 'south'
        : facing === 'south'
          ? 'north'
          : facing === 'east'
            ? 'west'
            : 'east'
    mapByDir[opp] = back
  }
  return mapByDir
}

function subpartRotationEuler(face: Direction): [number, number, number] {
  switch (face) {
    case 'south': // +Z
      return [0, 0, 0]
    case 'north': // -Z
      return [0, Math.PI, 0]
    case 'east': // +X
      return [0, Math.PI / 2, 0]
    case 'west': // -X
      return [0, -Math.PI / 2, 0]
    case 'up': // +Y
      return [-Math.PI / 2, 0, 0]
    case 'down': // -Y
      return [Math.PI / 2, 0, 0]
  }
}

interface SubpartAttachmentProps {
  subpartId: string
  face: Direction
  online?: boolean
  opacity?: number
}

export function SubpartMesh({ subpartId, face, online = true, opacity = 1 }: SubpartAttachmentProps) {
  const rot = subpartRotationEuler(face)
  const isTransparent = opacity < 1

  const isTerminal =
    subpartId === 'ae2:terminal' ||
    subpartId === 'ae2:crafting_terminal' ||
    subpartId === 'ae2:pattern_encoding_terminal' ||
    subpartId === 'ae2:pattern_access_terminal'

  const screenTex = useMemo(() => {
    if (!isTerminal) return null
    const variant =
      subpartId === 'ae2:crafting_terminal'
        ? 'crafting_terminal'
        : subpartId === 'ae2:pattern_encoding_terminal'
          ? 'pattern_encoding_terminal'
          : subpartId === 'ae2:pattern_access_terminal'
            ? 'pattern_access_terminal'
            : 'terminal'
    return getTerminalScreenTexture(variant, online)
  }, [isTerminal, online, subpartId])

  const backTex = useMemo(() => loadPixelTexture('/textures/ae2/part/monitor_back.png'), [])
  const sideTex = useMemo(() => loadPixelTexture('/textures/ae2/part/monitor_sides.png'), [])

  const terminalMaterials = useMemo(() => {
    if (!screenTex) return undefined
    return buildCubeMaterials({
      east: sideTex,
      west: sideTex,
      up: sideTex,
      down: sideTex,
      south: screenTex,
      north: backTex,
      opacity,
      transparent: isTransparent,
    })
  }, [backTex, isTransparent, opacity, screenTex, sideTex])

  if (isTerminal && terminalMaterials) {
    return (
      <group rotation={rot}>
        {/* White/metallic AE2 mounting stem from cable core to display back */}
        <mesh position={[0, 0, 0.29]}>
          <boxGeometry args={[0.25, 0.25, 0.22]} />
          <meshStandardMaterial
            map={backTex}
            roughness={0.7}
            transparent={isTransparent}
            opacity={opacity}
          />
        </mesh>
        {/* 12x12x2 pixel AE2 illuminated terminal panel */}
        <mesh position={[0, 0, 0.4375]} material={terminalMaterials}>
          <boxGeometry args={[0.75, 0.75, 0.125]} />
        </mesh>
      </group>
    )
  }

  if (subpartId === 'ae2:quartz_fiber' || subpartId === 'ae2:cable_anchor') {
    const tex = loadPixelTexture(
      subpartId === 'ae2:quartz_fiber'
        ? '/textures/ae2/part/quartz_fiber.png'
        : '/textures/ae2/part/cable_anchor.png'
    )
    return (
      <group rotation={rot}>
        <mesh position={[0, 0, 0.34]}>
          <boxGeometry args={[0.22, 0.22, 0.32]} />
          <meshStandardMaterial
            map={tex}
            color={subpartId === 'ae2:quartz_fiber' ? '#f8fafc' : '#cbd5e1'}
            roughness={0.5}
            transparent={isTransparent}
            opacity={opacity}
          />
        </mesh>
      </group>
    )
  }

  // Import Bus / Export Bus / Storage Bus / P2P Tunnel stepped AE2 collar + plate
  const frontUrl =
    subpartId === 'ae2:import_bus'
      ? '/textures/ae2/part/import_bus.png'
      : subpartId === 'ae2:export_bus'
        ? '/textures/ae2/part/export_bus.png'
        : subpartId === 'ae2:storage_bus'
          ? '/textures/ae2/part/storage_bus.png'
          : '/textures/ae2/part/p2p_tunnel_front.png'
  const sidesUrl =
    subpartId === 'ae2:import_bus'
      ? '/textures/ae2/part/import_bus_sides.png'
      : subpartId === 'ae2:export_bus'
        ? '/textures/ae2/part/export_bus_sides.png'
        : subpartId === 'ae2:storage_bus'
          ? '/textures/ae2/part/storage_bus_sides.png'
          : '/textures/ae2/part/p2p_tunnel_sides.png'

  const frontTex = loadPixelTexture(frontUrl)
  const busSidesTex = loadPixelTexture(sidesUrl)
  const plateSize = subpartId === 'ae2:storage_bus' ? 0.625 : 0.5

  return (
    <group rotation={rot}>
      <mesh position={[0, 0, 0.28]}>
        <boxGeometry args={[0.28, 0.28, 0.22]} />
        <meshStandardMaterial
          map={busSidesTex}
          roughness={0.75}
          transparent={isTransparent}
          opacity={opacity}
        />
      </mesh>
      <mesh position={[0, 0, 0.4375]}>
        <boxGeometry args={[plateSize, plateSize, 0.125]} />
        <meshStandardMaterial
          map={frontTex}
          roughness={0.75}
          transparent={isTransparent}
          opacity={opacity}
        />
      </mesh>
    </group>
  )
}

interface VoxelBlockModelProps {
  block: PlacedBlock
  def?: BlockDefinition
  blocksByCoord: Map<string, PlacedBlock>
  connectedDirs: Direction[]
  cableOverlay?: CableChannelOverlay
  online?: boolean
  invalidController?: boolean
  isCraftingActive?: boolean
  opacity?: number
}

export function VoxelBlockModel({
  block,
  def,
  blocksByCoord,
  connectedDirs,
  cableOverlay,
  online = true,
  invalidController = false,
  isCraftingActive = false,
  opacity = 1,
}: VoxelBlockModelProps) {
  const id = block.block_def_id
  const isGhost = opacity < 1

  // ---------------------------------------------------------------------------
  // 1. AE2 Cables (Glass, Covered, Smart, Dense Covered, Dense Smart)
  // ---------------------------------------------------------------------------
  if (
    id === 'ae2:fluix_cable' ||
    id === 'ae2:covered_cable' ||
    id === 'ae2:smart_cable' ||
    id === 'ae2:dense_cable' ||
    id === 'ae2:dense_smart_cable' ||
    def?.metadata.is_cable
  ) {
    const { color, subparts } = readCableConfig(block.config)
    const assetColor = cableColorToAssetName(color)
    const isDense = id === 'ae2:dense_cable' || id === 'ae2:dense_smart_cable'
    const isSmart = id === 'ae2:smart_cable' || id === 'ae2:dense_smart_cable'
    const isGlass = id === 'ae2:fluix_cable'

    const corePath = isDense
      ? `/textures/ae2/part/cable/core/dense_smart/${assetColor}.png`
      : isGlass
        ? `/textures/ae2/part/cable/core/glass/${assetColor}.png`
        : `/textures/ae2/part/cable/core/covered/${assetColor}.png`

    const armPath =
      id === 'ae2:dense_smart_cable'
        ? `/textures/ae2/part/cable/dense_smart/${assetColor}.png`
        : id === 'ae2:dense_cable'
          ? `/textures/ae2/part/cable/dense_covered/${assetColor}.png`
          : id === 'ae2:smart_cable'
            ? `/textures/ae2/part/cable/smart/${assetColor}.png`
            : isGlass
              ? `/textures/ae2/part/cable/glass/${assetColor}.png`
              : `/textures/ae2/part/cable/covered/${assetColor}.png`

    const coreTex = loadPixelTexture(corePath)
    const armTex = loadPixelTexture(armPath)
    const coreSize = isDense ? 0.5625 : isGlass ? 0.25 : 0.375
    const armThick = isDense ? 0.5 : isGlass ? 0.25 : 0.3125
    const channelsUsed = cableOverlay?.channels_used ?? 0

    // Include directions that either connect to neighbor ME blocks OR hold mounted subparts
    const subpartEntries = ALL_DIRECTIONS.map((d) => [d, subparts[d]] as const).filter(
      (entry): entry is [Direction, string] => typeof entry[1] === 'string' && entry[1].length > 0
    )

    return (
      <group>
        {/* Cable Core */}
        <mesh>
          <boxGeometry args={[coreSize, coreSize, coreSize]} />
          <meshStandardMaterial
            map={coreTex}
            roughness={0.75}
            transparent={isGhost}
            opacity={opacity}
          />
        </mesh>

        {/* Connected ME Neighbor Arms */}
        {connectedDirs.map((dir) => {
          const off = directionOffset(dir)
          const neighbor = blocksByCoord.get(
            coordKey(block.x + off.x, block.y + off.y, block.z + off.z)
          )
          // Step-down collar when Dense cable attaches directly to a standard cable or device
          const neighborIsDense =
            neighbor?.block_def_id === 'ae2:dense_cable' ||
            neighbor?.block_def_id === 'ae2:dense_smart_cable' ||
            neighbor?.block_def_id === 'ae2:controller'
          const actualThick = isDense && neighbor && !neighborIsDense ? 0.34 : armThick
          const armLen = 0.5 - coreSize / 2
          const centerDist = coreSize / 2 + armLen / 2
          const size: [number, number, number] = [
            off.x !== 0 ? armLen : actualThick,
            off.y !== 0 ? armLen : actualThick,
            off.z !== 0 ? armLen : actualThick,
          ]
          return (
            <group key={dir}>
              <mesh
                position={[off.x * centerDist, off.y * centerDist, off.z * centerDist]}
              >
                <boxGeometry args={size} />
                <meshStandardMaterial
                  map={armTex}
                  roughness={0.75}
                  transparent={isGhost}
                  opacity={opacity}
                />
              </mesh>
              {isSmart && channelsUsed > 0 && !isGhost && (
                <mesh
                  position={[off.x * centerDist, off.y * centerDist, off.z * centerDist]}
                >
                  <boxGeometry
                    args={[size[0] * 1.015, size[1] * 1.015, size[2] * 1.015]}
                  />
                  <meshBasicMaterial
                    color={
                      cableOverlay?.overloaded
                        ? '#ef4444'
                        : CABLE_COLOR_HEX[color]
                    }
                    wireframe={false}
                    transparent
                    opacity={0.35}
                  />
                </mesh>
              )}
            </group>
          )
        })}

        {/* Mounted Cable Subparts (Terminals, Import/Export/Storage Buses, Quartz Fiber, Anchors) */}
        {subpartEntries.map(([dir, subId]) => (
          <SubpartMesh
            key={`sub-${dir}`}
            subpartId={subId}
            face={dir}
            online={online}
            opacity={opacity}
          />
        ))}
      </group>
    )
  }

  // ---------------------------------------------------------------------------
  // 2. Standalone Subpart Blocks (placed directly in a voxel cell)
  // ---------------------------------------------------------------------------
  if (
    id === 'ae2:terminal' ||
    id === 'ae2:crafting_terminal' ||
    id === 'ae2:pattern_encoding_terminal' ||
    id === 'ae2:pattern_access_terminal' ||
    id === 'ae2:import_bus' ||
    id === 'ae2:export_bus' ||
    id === 'ae2:storage_bus' ||
    id === 'ae2:p2p_tunnel'
  ) {
    const coreTex = loadPixelTexture('/textures/ae2/part/cable/core/covered/transparent.png')
    const armTex = loadPixelTexture('/textures/ae2/part/cable/covered/transparent.png')
    return (
      <group>
        {/* Integrated Fluix cable hub so standalone terminals/buses visually connect to adjacent cables */}
        <mesh>
          <boxGeometry args={[0.34, 0.34, 0.34]} />
          <meshStandardMaterial
            map={coreTex}
            roughness={0.75}
            transparent={isGhost}
            opacity={opacity}
          />
        </mesh>
        {connectedDirs.map((dir) => {
          if (dir === block.facing) return null
          const off = directionOffset(dir)
          const armLen = 0.33
          const dist = 0.17 + armLen / 2
          return (
            <mesh key={dir} position={[off.x * dist, off.y * dist, off.z * dist]}>
              <boxGeometry
                args={[
                  off.x !== 0 ? armLen : 0.28,
                  off.y !== 0 ? armLen : 0.28,
                  off.z !== 0 ? armLen : 0.28,
                ]}
              />
              <meshStandardMaterial
                map={armTex}
                roughness={0.75}
                transparent={isGhost}
                opacity={opacity}
              />
            </mesh>
          )
        })}
        <SubpartMesh subpartId={id} face={block.facing} online={online} opacity={opacity} />
      </group>
    )
  }

  if (id === 'ae2:quartz_fiber' || id === 'ae2:cable_anchor') {
    const tex = loadPixelTexture(
      id === 'ae2:quartz_fiber'
        ? '/textures/ae2/part/quartz_fiber.png'
        : '/textures/ae2/part/cable_anchor.png'
    )
    return (
      <mesh>
        <boxGeometry args={[0.32, 0.32, 0.32]} />
        <meshStandardMaterial
          map={tex}
          roughness={0.6}
          transparent={isGhost}
          opacity={opacity}
        />
      </mesh>
    )
  }

  // ---------------------------------------------------------------------------
  // 3. ME Controller (with official column texture + glowing circuit lights!)
  // ---------------------------------------------------------------------------
  if (id === 'ae2:controller') {
    const hasAbove =
      blocksByCoord.get(coordKey(block.x, block.y + 1, block.z))?.block_def_id === 'ae2:controller'
    const hasBelow =
      blocksByCoord.get(coordKey(block.x, block.y - 1, block.z))?.block_def_id === 'ae2:controller'
    const isColumn = hasAbove || hasBelow

    const baseTex = loadPixelTexture(
      invalidController
        ? isColumn
          ? '/textures/ae2/block/controller_column_conflict.png'
          : '/textures/ae2/block/controller_conflict.png'
        : isColumn
          ? '/textures/ae2/block/controller_column.png'
          : '/textures/ae2/block/controller.png'
    )
    const lightsTex = loadPixelTexture(
      isColumn
        ? '/textures/ae2/block/controller_column_lights.png'
        : '/textures/ae2/block/controller_lights.png'
    )
    const capTex = loadPixelTexture('/textures/ae2/block/controller.png')

    return (
      <group>
        <mesh>
          <boxGeometry args={[1, 1, 1]} />
          <meshStandardMaterial
            map={isColumn ? baseTex : capTex}
            roughness={0.8}
            transparent={isGhost}
            opacity={opacity}
          />
        </mesh>
        {online && !invalidController && !isGhost && (
          <mesh>
            <boxGeometry args={[1.004, 1.004, 1.004]} />
            <meshBasicMaterial
              map={lightsTex}
              color="#76ff4d"
              transparent
              alphaTest={0.08}
            />
          </mesh>
        )}
      </group>
    )
  }

  // ---------------------------------------------------------------------------
  // 4. ME Drive (with 10 3D Storage Cell Bays & Status LEDs!)
  // ---------------------------------------------------------------------------
  if (id === 'ae2:drive') {
    const frontTex = loadPixelTexture('/textures/ae2/block/drive/drive_front.png')
    const sideTex = loadPixelTexture('/textures/ae2/block/generics/side.png')
    const topTex = loadPixelTexture('/textures/ae2/block/generics/top.png')
    const botTex = loadPixelTexture('/textures/ae2/block/generics/bottom.png')
    const backTex = loadPixelTexture('/textures/ae2/block/generics/back.png')

    const mats = buildCubeMaterials({
      ...orientHorizontalFaces(block.facing, frontTex, sideTex, backTex, topTex, botTex),
      opacity,
      transparent: isGhost,
    })

    const { cells } = readDriveConfig(block.config)
    const frontRot = subpartRotationEuler(
      block.facing === 'up' || block.facing === 'down' ? 'south' : block.facing
    )

    return (
      <group>
        <mesh material={mats}>
          <boxGeometry args={[1, 1, 1]} />
        </mesh>
        {!isGhost && cells.length > 0 && (
          <group rotation={frontRot}>
            {cells.slice(0, 10).map((cell, idx) => {
              const col = idx % 2 === 0 ? -0.185 : 0.185
              const row = 0.31 - Math.floor(idx / 2) * 0.155
              const tierColor =
                cell.tier === '256k'
                  ? '#a855f7'
                  : cell.tier === '64k'
                    ? '#ef4444'
                    : cell.tier === '16k'
                      ? '#f59e0b'
                      : cell.tier === '4k'
                        ? '#38bdf8'
                        : '#60a5fa'
              const ledColor = !online
                ? '#ef4444'
                : cell.items.length > 0
                  ? '#22d3ee'
                  : '#4ade80'
              return (
                <group key={idx} position={[col, row, 0.502]}>
                  <mesh>
                    <boxGeometry args={[0.24, 0.095, 0.02]} />
                    <meshStandardMaterial color={tierColor} roughness={0.5} />
                  </mesh>
                  <mesh position={[0.075, 0, 0.012]}>
                    <boxGeometry args={[0.045, 0.045, 0.01]} />
                    <meshBasicMaterial color={ledColor} />
                  </mesh>
                </group>
              )
            })}
          </group>
        )}
      </group>
    )
  }

  // ---------------------------------------------------------------------------
  // 5. Molecular Assembler (Translucent Quartz Glass Frame + Inner Core!)
  // ---------------------------------------------------------------------------
  if (id === 'ae2:molecular_assembler') {
    const frameTex = loadPixelTexture('/textures/ae2/block/molecular_assembler.png')
    const coreTex = loadPixelTexture('/textures/ae2/block/molecular_assembler_lights.png')
    return (
      <group>
        {/* Translucent blue-tinted glass interior */}
        <mesh>
          <boxGeometry args={[0.92, 0.92, 0.92]} />
          <meshStandardMaterial
            color="#bae6fd"
            transparent
            opacity={isGhost ? opacity * 0.2 : 0.18}
            roughness={0.2}
          />
        </mesh>
        {/* Outer metallic frame with cut-out glass alpha */}
        <mesh>
          <boxGeometry args={[1, 1, 1]} />
          <meshStandardMaterial
            map={frameTex}
            transparent
            alphaTest={0.1}
            opacity={opacity}
            side={THREE.DoubleSide}
            roughness={0.6}
          />
        </mesh>
        {/* Inner Molecular Crafting Core */}
        <mesh>
          <boxGeometry args={[0.36, 0.36, 0.36]} />
          <meshStandardMaterial
            map={coreTex}
            emissive={isCraftingActive ? '#38bdf8' : '#000000'}
            emissiveIntensity={isCraftingActive ? 0.9 : 0}
            transparent={isGhost}
            opacity={opacity}
          />
        </mesh>
      </group>
    )
  }

  // ---------------------------------------------------------------------------
  // 6. AE2 Inscriber & Charger (Authentic Open-Sided Press Frames!)
  // ---------------------------------------------------------------------------
  if (id === 'ae2:inscriber' || id === 'ae2:charger') {
    const tex = loadPixelTexture(
      id === 'ae2:inscriber'
        ? '/textures/ae2/block/inscriber.png'
        : '/textures/ae2/block/charger.png'
    )
    const insideTex = loadPixelTexture('/textures/ae2/block/inscriber_inside.png')
    const rot = subpartRotationEuler(
      block.facing === 'up' || block.facing === 'down' ? 'south' : block.facing
    )
    return (
      <group rotation={rot}>
        {/* Bottom & Top press plates */}
        <mesh position={[0, -0.375, 0]}>
          <boxGeometry args={[1, 0.25, 1]} />
          <meshStandardMaterial map={tex} roughness={0.7} transparent={isGhost} opacity={opacity} />
        </mesh>
        <mesh position={[0, 0.375, 0]}>
          <boxGeometry args={[1, 0.25, 1]} />
          <meshStandardMaterial map={tex} roughness={0.7} transparent={isGhost} opacity={opacity} />
        </mesh>
        {/* Back & Side structural pillars */}
        <mesh position={[0, 0, -0.375]}>
          <boxGeometry args={[1, 0.5, 0.25]} />
          <meshStandardMaterial map={tex} roughness={0.7} transparent={isGhost} opacity={opacity} />
        </mesh>
        {id === 'ae2:inscriber' && (
          <>
            <mesh position={[-0.4, 0, 0.35]}>
              <boxGeometry args={[0.18, 0.5, 0.18]} />
              <meshStandardMaterial
                map={insideTex}
                roughness={0.7}
                transparent={isGhost}
                opacity={opacity}
              />
            </mesh>
            <mesh position={[0.4, 0, 0.35]}>
              <boxGeometry args={[0.18, 0.5, 0.18]} />
              <meshStandardMaterial
                map={insideTex}
                roughness={0.7}
                transparent={isGhost}
                opacity={opacity}
              />
            </mesh>
            {/* Stamp piston */}
            <mesh position={[0, isCraftingActive ? 0.04 : 0.16, 0]}>
              <boxGeometry args={[0.5, 0.12, 0.5]} />
              <meshStandardMaterial
                map={insideTex}
                roughness={0.5}
                transparent={isGhost}
                opacity={opacity}
              />
            </mesh>
          </>
        )}
      </group>
    )
  }

  // ---------------------------------------------------------------------------
  // 7. Vanilla Minecraft Hopper (3-Tier Funnel + Directional Output Spout!)
  // ---------------------------------------------------------------------------
  if (id === 'minecraft:hopper') {
    const outTex = loadPixelTexture('/textures/minecraft/block/hopper_outside.png')
    const topTex = loadPixelTexture('/textures/minecraft/block/hopper_top.png')
    const spoutOff = directionOffset(block.facing === 'up' ? 'down' : block.facing)
    const isSideSpout = spoutOff.y === 0
    return (
      <group>
        {/* Top Rim Bowl (16x6x16) */}
        <mesh position={[0, 0.3125, 0]}>
          <boxGeometry args={[1, 0.375, 1]} />
          <meshStandardMaterial
            map={topTex}
            roughness={0.85}
            transparent={isGhost}
            opacity={opacity}
          />
        </mesh>
        {/* Middle Funnel (8x6x8) */}
        <mesh position={[0, -0.0625, 0]}>
          <boxGeometry args={[0.5, 0.375, 0.5]} />
          <meshStandardMaterial
            map={outTex}
            roughness={0.85}
            transparent={isGhost}
            opacity={opacity}
          />
        </mesh>
        {/* Output Spout (4x4x4) pointing down or sideways into Pattern Provider */}
        <mesh
          position={
            isSideSpout
              ? [spoutOff.x * 0.34, -0.125, spoutOff.z * 0.34]
              : [0, -0.375, 0]
          }
        >
          <boxGeometry args={[0.25, 0.25, 0.25]} />
          <meshStandardMaterial
            map={outTex}
            roughness={0.85}
            transparent={isGhost}
            opacity={opacity}
          />
        </mesh>
      </group>
    )
  }

  // ---------------------------------------------------------------------------
  // 8. ME Pattern Provider (Normal all-face vs Directional arrow variant!)
  // ---------------------------------------------------------------------------
  if (id === 'ae2:pattern_provider') {
    const { variant } = readPatternProviderConfig(block.config)
    const baseTex = loadPixelTexture('/textures/ae2/block/pattern_provider.png')
    if (variant === 'directional') {
      const frontTex = loadPixelTexture('/textures/ae2/block/pattern_provider_alternate_front.png')
      const arrowTex = loadPixelTexture('/textures/ae2/block/pattern_provider_alternate_arrow.png')
      const backTex = loadPixelTexture('/textures/ae2/block/pattern_provider_alternate.png')
      const mats = buildCubeMaterials({
        ...orientHorizontalFaces(block.facing, frontTex, arrowTex, backTex, arrowTex, arrowTex),
        opacity,
        transparent: isGhost,
      })
      return (
        <mesh material={mats}>
          <boxGeometry args={[1, 1, 1]} />
        </mesh>
      )
    }
    return (
      <mesh>
        <boxGeometry args={[1, 1, 1]} />
        <meshStandardMaterial
          map={baseTex}
          roughness={0.8}
          transparent={isGhost}
          opacity={opacity}
        />
      </mesh>
    )
  }

  // ---------------------------------------------------------------------------
  // 9. Crafting Monitor (with illuminated purple AE2 job display screen on Facing)
  // ---------------------------------------------------------------------------
  if (id === 'ae2:crafting_monitor') {
    const sideTex = loadPixelTexture('/textures/ae2/block/crafting/unit.png')
    const frontBaseTex = loadPixelTexture('/textures/ae2/block/crafting/monitor.png')
    const screenMaskTex = loadPixelTexture('/textures/ae2/block/crafting/monitor_light_bright.png')
    const mats = buildCubeMaterials({
      ...orientHorizontalFaces(block.facing, frontBaseTex, sideTex, sideTex, sideTex, sideTex),
      opacity,
      transparent: isGhost,
    })
    const frontRot = subpartRotationEuler(block.facing)
    return (
      <group>
        <mesh material={mats}>
          <boxGeometry args={[1, 1, 1]} />
        </mesh>
        {online && !isGhost && (
          <group rotation={frontRot}>
            <mesh position={[0, 0, 0.502]}>
              <planeGeometry args={[0.98, 0.98]} />
              <meshBasicMaterial
                map={screenMaskTex}
                color="#a855f7"
                transparent
                alphaTest={0.05}
              />
            </mesh>
          </group>
        )}
      </group>
    )
  }

  // ---------------------------------------------------------------------------
  // 10. Multi-Face Oriented Full Cubes (Furnace, Blast Furnace, Barrel, ME Chest)
  // ---------------------------------------------------------------------------
  if (id === 'minecraft:furnace' || id === 'minecraft:blast_furnace') {
    const prefix = id === 'minecraft:blast_furnace' ? 'blast_furnace' : 'furnace'
    const frontTex = loadPixelTexture(
      `/textures/minecraft/block/${prefix}_${isCraftingActive ? 'front_on' : 'front'}.png`
    )
    const sideTex = loadPixelTexture(`/textures/minecraft/block/${prefix}_side.png`)
    const topTex = loadPixelTexture(`/textures/minecraft/block/${prefix}_top.png`)
    const mats = buildCubeMaterials({
      ...orientHorizontalFaces(block.facing, frontTex, sideTex, sideTex, topTex, topTex),
      opacity,
      transparent: isGhost,
    })
    return (
      <mesh material={mats}>
        <boxGeometry args={[1, 1, 1]} />
      </mesh>
    )
  }

  if (id === 'minecraft:barrel') {
    const topTex = loadPixelTexture('/textures/minecraft/block/barrel_top.png')
    const sideTex = loadPixelTexture('/textures/minecraft/block/barrel_side.png')
    const botTex = loadPixelTexture('/textures/minecraft/block/barrel_bottom.png')
    const mats = buildCubeMaterials({
      ...orientHorizontalFaces(block.facing, topTex, sideTex, botTex, sideTex, sideTex),
      opacity,
      transparent: isGhost,
    })
    return (
      <mesh material={mats}>
        <boxGeometry args={[1, 1, 1]} />
      </mesh>
    )
  }

  if (id === 'ae2:chest') {
    const frontTex = loadPixelTexture('/textures/ae2/block/chest/front.png')
    const topTex = loadPixelTexture('/textures/ae2/block/chest/top.png')
    const sideTex = loadPixelTexture('/textures/ae2/block/generics/side.png')
    const backTex = loadPixelTexture('/textures/ae2/block/generics/back.png')
    const botTex = loadPixelTexture('/textures/ae2/block/generics/bottom.png')
    const mats = buildCubeMaterials({
      ...orientHorizontalFaces(block.facing, frontTex, sideTex, backTex, topTex, botTex),
      opacity,
      transparent: isGhost,
    })
    return (
      <mesh material={mats}>
        <boxGeometry args={[1, 1, 1]} />
      </mesh>
    )
  }

  // ---------------------------------------------------------------------------
  // 11. Uniform Official AE2 Textured Cubes (Energy Acceptor, Cells, CPU Storage, Co-Processor, Interface)
  // ---------------------------------------------------------------------------
  const uniformUrl =
    id === 'ae2:energy_acceptor'
      ? '/textures/ae2/block/energy_acceptor.png'
      : id === 'ae2:energy_cell'
        ? '/textures/ae2/block/energy_cell_4.png'
        : id === 'ae2:dense_energy_cell'
          ? '/textures/ae2/block/dense_energy_cell_4.png'
          : id === 'ae2:creative_energy_cell'
            ? '/textures/ae2/block/creative_energy_cell.png'
            : id === 'ae2:interface'
              ? '/textures/ae2/block/interface.png'
              : id === 'ae2:crafting_unit'
                ? '/textures/ae2/block/crafting/unit.png'
                : id === 'ae2:crafting_storage_1k'
                  ? '/textures/ae2/block/crafting/1k_storage.png'
                  : id === 'ae2:crafting_storage_4k'
                    ? '/textures/ae2/block/crafting/4k_storage.png'
                    : id === 'ae2:crafting_storage_16k'
                      ? '/textures/ae2/block/crafting/16k_storage.png'
                      : id === 'ae2:crafting_storage_64k'
                        ? '/textures/ae2/block/crafting/64k_storage.png'
                        : id === 'ae2:crafting_storage_256k'
                          ? '/textures/ae2/block/crafting/256k_storage.png'
                          : id === 'ae2:crafting_co_processor'
                            ? '/textures/ae2/block/crafting/accelerator.png'
                            : id === 'ae2:quantum_ring'
                              ? '/textures/ae2/block/quantum_ring.png'
                              : '/textures/ae2/block/generics/side.png'

  const tex = loadPixelTexture(uniformUrl)
  return (
    <mesh>
      <boxGeometry args={[1, 1, 1]} />
      <meshStandardMaterial
        map={tex}
        roughness={0.8}
        transparent={isGhost}
        opacity={opacity}
      />
    </mesh>
  )
}
