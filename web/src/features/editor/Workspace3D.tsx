import { useEffect, useMemo, useRef, useState } from 'react'
import { Canvas, type ThreeEvent, useFrame, useThree } from '@react-three/fiber'
import { Html, OrbitControls } from '@react-three/drei'
import * as THREE from 'three'
import {
  computeConnectedDirections,
  coordKey,
  directionOffset,
  isCableBlock,
  networkColorForId,
  readCableConfig,
  resolveBlockColor,
} from '../../lib/ae2/constants'
import type {
  BlockDefinition,
  CableChannelOverlay,
  Coord3D,
  DeviceRouteOverlay,
  Direction,
  PlacedBlock,
} from '../../lib/api'
import { type CameraPreset, selectActiveOverlay, useEditorStore } from './store'
import { Workspace2D } from './Workspace2D'

function detectWebGLSupport(): boolean {
  if (typeof window === 'undefined' || typeof document === 'undefined') {
    return false
  }
  if (typeof window.WebGLRenderingContext === 'undefined') {
    return false
  }
  try {
    const canvas = document.createElement('canvas')
    const ctx =
      canvas.getContext('webgl2') ||
      canvas.getContext('webgl') ||
      canvas.getContext('experimental-webgl')
    return Boolean(ctx)
  } catch {
    return false
  }
}

function facingConeTransform(facing: Direction): {
  position: [number, number, number]
  rotation: [number, number, number]
} {
  switch (facing) {
    case 'north':
      return { position: [0, 0, -0.52], rotation: [-Math.PI / 2, 0, 0] }
    case 'south':
      return { position: [0, 0, 0.52], rotation: [Math.PI / 2, 0, 0] }
    case 'east':
      return { position: [0.52, 0, 0], rotation: [0, 0, -Math.PI / 2] }
    case 'west':
      return { position: [-0.52, 0, 0], rotation: [0, 0, Math.PI / 2] }
    case 'up':
      return { position: [0, 0.52, 0], rotation: [0, 0, 0] }
    case 'down':
      return { position: [0, -0.52, 0], rotation: [Math.PI, 0, 0] }
  }
}

function panelRotation(facing: Direction): [number, number, number] {
  switch (facing) {
    case 'north':
      return [0, Math.PI, 0]
    case 'south':
      return [0, 0, 0]
    case 'east':
      return [0, Math.PI / 2, 0]
    case 'west':
      return [0, -Math.PI / 2, 0]
    case 'up':
      return [-Math.PI / 2, 0, 0]
    case 'down':
      return [Math.PI / 2, 0, 0]
  }
}

function CameraPresetController({ preset }: { preset: CameraPreset }) {
  const { camera } = useThree()

  useEffect(() => {
    switch (preset) {
      case 'top':
        camera.position.set(0, 14, 0.01)
        break
      case 'front':
        camera.position.set(0, 3, 13)
        break
      case 'side':
        camera.position.set(13, 3, 0)
        break
      case 'orbit':
      default:
        camera.position.set(8, 8, 9)
        break
    }
    camera.lookAt(1, 0, 0)
  }, [camera, preset])

  return <OrbitControls makeDefault enableDamping dampingFactor={0.08} target={[1, 0, 0]} />
}

function DiagnosticPulseWireframe() {
  const meshRef = useRef<THREE.Mesh>(null)
  const matRef = useRef<THREE.MeshBasicMaterial>(null)

  useFrame(({ clock }) => {
    const t = clock.getElapsedTime()
    const scale = 1.09 + Math.sin(t * 5.0) * 0.045
    if (meshRef.current) {
      meshRef.current.scale.set(scale, scale, scale)
    }
    if (matRef.current) {
      matRef.current.opacity = 0.72 + Math.sin(t * 5.0) * 0.25
    }
  })

  return (
    <mesh ref={meshRef}>
      <boxGeometry args={[1, 1, 1]} />
      <meshBasicMaterial ref={matRef} color="#ef4444" wireframe transparent opacity={0.9} />
    </mesh>
  )
}

interface ProceduralBlockMeshProps {
  block: PlacedBlock
  def?: BlockDefinition
  connectedDirs: Direction[]
  cableOverlay?: CableChannelOverlay
  routeOverlay?: DeviceRouteOverlay
  isSelected: boolean
  isHighlighted: boolean
  showNetworks: boolean
  showConnections: boolean
  showChannels: boolean
  showEnergy: boolean
  showWarnings: boolean
  hasWarning: boolean
  onBlockPointerDown: (e: ThreeEvent<MouseEvent>, block: PlacedBlock) => void
  onBlockPointerMove: (e: ThreeEvent<PointerEvent>, block: PlacedBlock) => void
  onBlockPointerOut: () => void
}

function ProceduralBlockMesh({
  block,
  def,
  connectedDirs,
  cableOverlay,
  routeOverlay,
  isSelected,
  isHighlighted,
  showNetworks,
  showConnections,
  showChannels,
  showEnergy,
  showWarnings,
  hasWarning,
  onBlockPointerDown,
  onBlockPointerMove,
  onBlockPointerOut,
}: ProceduralBlockMeshProps) {
  const shape = def?.metadata.visual.shape ?? (isCableBlock(block, def) ? 'cable' : 'cube')
  const baseColor = resolveBlockColor(block, def)
  const accentColor = def?.metadata.visual.accent_color ?? '#38bdf8'
  const isSmart = Boolean(def?.metadata.is_smart_cable)
  const isDense = Boolean(def?.metadata.is_dense_cable || shape === 'dense_cable')
  const cableCfg = isCableBlock(block, def) ? readCableConfig(block.config) : null

  const channelRatio =
    cableOverlay && cableOverlay.capacity > 0
      ? Math.min(1, cableOverlay.channels_used / cableOverlay.capacity)
      : 0
  const smartBarColor = cableOverlay?.overloaded
    ? '#ef4444'
    : channelRatio > 0.75
      ? '#f59e0b'
      : channelRatio > 0
        ? '#38bdf8'
        : '#475569'

  const networkId = cableOverlay?.network_id ?? routeOverlay?.network_id
  const cone = facingConeTransform(block.facing)

  return (
    <group
      position={[block.x, block.y, block.z]}
      onClick={(e) => onBlockPointerDown(e, block)}
      onPointerMove={(e) => onBlockPointerMove(e, block)}
      onPointerOut={onBlockPointerOut}
    >
      {/* Cable & Dense Cable procedural cores + dynamic directional arms */}
      {(shape === 'cable' || shape === 'dense_cable') && (
        <group>
          <mesh>
            <boxGeometry
              args={isDense ? [0.48, 0.48, 0.48] : [0.28, 0.28, 0.28]}
            />
            <meshStandardMaterial color={baseColor} roughness={0.35} metalness={0.25} />
          </mesh>

          {isSmart && (
            <mesh>
              <boxGeometry
                args={isDense ? [0.51, 0.14, 0.51] : [0.31, 0.09, 0.31]}
              />
              <meshStandardMaterial
                color={smartBarColor}
                emissive={smartBarColor}
                emissiveIntensity={channelRatio > 0 || cableOverlay?.overloaded ? 0.75 : 0.15}
              />
            </mesh>
          )}

          {showConnections &&
            connectedDirs.map((dir) => {
              const off = directionOffset(dir)
              const thickness = isDense ? 0.4 : 0.22
              const length = 0.5
              const size: [number, number, number] =
                off.x !== 0
                  ? [length, thickness, thickness]
                  : off.y !== 0
                    ? [thickness, length, thickness]
                    : [thickness, thickness, length]
              return (
                <mesh
                  key={`arm-${dir}`}
                  position={[off.x * 0.25, off.y * 0.25, off.z * 0.25]}
                >
                  <boxGeometry args={size} />
                  <meshStandardMaterial color={baseColor} roughness={0.4} metalness={0.2} />
                </mesh>
              )
            })}

          {cableCfg?.anchored_faces.map((face) => {
            const off = directionOffset(face)
            const plateSize: [number, number, number] =
              off.x !== 0 ? [0.06, 0.36, 0.36] : off.y !== 0 ? [0.36, 0.06, 0.36] : [0.36, 0.36, 0.06]
            return (
              <mesh
                key={`anchor-${face}`}
                position={[off.x * 0.45, off.y * 0.45, off.z * 0.45]}
              >
                <boxGeometry args={plateSize} />
                <meshStandardMaterial color="#64748b" metalness={0.6} roughness={0.3} />
              </mesh>
            )
          })}
        </group>
      )}

      {/* ME Controller glowing core */}
      {shape === 'core' && (
        <group>
          <mesh>
            <boxGeometry args={[0.94, 0.94, 0.94]} />
            <meshStandardMaterial color="#1e293b" metalness={0.55} roughness={0.35} />
          </mesh>
          <mesh>
            <octahedronGeometry args={[0.42, 0]} />
            <meshStandardMaterial
              color={accentColor}
              emissive={baseColor}
              emissiveIntensity={0.9}
            />
          </mesh>
        </group>
      )}

      {/* Molecular Assembler / Crafting Frame */}
      {shape === 'frame' && (
        <group>
          <mesh>
            <boxGeometry args={[0.92, 0.92, 0.92]} />
            <meshStandardMaterial
              color={baseColor}
              transparent
              opacity={0.42}
              roughness={0.2}
              metalness={0.1}
            />
          </mesh>
          <mesh>
            <boxGeometry args={[0.94, 0.94, 0.94]} />
            <meshBasicMaterial color={accentColor} wireframe />
          </mesh>
          <mesh>
            <boxGeometry args={[0.44, 0.44, 0.44]} />
            <meshStandardMaterial color={accentColor} emissive={accentColor} emissiveIntensity={0.35} />
          </mesh>
        </group>
      )}

      {/* Directional Terminal / Monitor Panel */}
      {shape === 'panel' && (
        <group rotation={panelRotation(block.facing)}>
          <mesh position={[0, 0, -0.34]}>
            <boxGeometry args={[0.76, 0.76, 0.22]} />
            <meshStandardMaterial color={baseColor} roughness={0.35} metalness={0.4} />
          </mesh>
          <mesh position={[0, 0, -0.21]}>
            <boxGeometry args={[0.62, 0.52, 0.04]} />
            <meshStandardMaterial
              color={accentColor}
              emissive={accentColor}
              emissiveIntensity={0.65}
            />
          </mesh>
        </group>
      )}

      {/* Standard Solid AE2 Device Cube */}
      {shape !== 'cable' &&
        shape !== 'dense_cable' &&
        shape !== 'core' &&
        shape !== 'frame' &&
        shape !== 'panel' && (
          <group>
            <mesh>
              <boxGeometry args={[0.92, 0.92, 0.92]} />
              <meshStandardMaterial color={baseColor} roughness={0.42} metalness={0.3} />
            </mesh>
            <mesh>
              <boxGeometry args={[0.94, 0.2, 0.94]} />
              <meshStandardMaterial color={accentColor} roughness={0.3} metalness={0.4} />
            </mesh>
          </group>
        )}

      {/* Directional facing indicator cone for rotatable / push-face devices */}
      {(def?.metadata.rotatable || block.block_def_id === 'ae2:pattern_provider') && (
        <mesh position={cone.position} rotation={cone.rotation}>
          <coneGeometry args={[0.11, 0.2, 12]} />
          <meshStandardMaterial color="#38bdf8" emissive="#38bdf8" emissiveIntensity={0.45} />
        </mesh>
      )}

      {/* Network Ring */}
      {showNetworks && networkId && (
        <mesh position={[0, -0.47, 0]} rotation={[-Math.PI / 2, 0, 0]}>
          <ringGeometry args={[0.42, 0.49, 24]} />
          <meshBasicMaterial color={networkColorForId(networkId)} side={THREE.DoubleSide} />
        </mesh>
      )}

      {/* Selection Wireframe */}
      {isSelected && (
        <mesh>
          <boxGeometry args={[1.03, 1.03, 1.03]} />
          <meshBasicMaterial color="#38bdf8" wireframe />
        </mesh>
      )}

      {/* Crimson Diagnostic Highlight Pulse */}
      {isHighlighted && <DiagnosticPulseWireframe />}

      {/* Floating CAD Telemetry Overlay Tag */}
      {(isSelected ||
        isHighlighted ||
        (showChannels && (cableOverlay || routeOverlay)) ||
        (showWarnings && hasWarning) ||
        (showEnergy && (def?.metadata.energy_generation ?? 0) > 0)) && (
        <Html position={[0, 0.68, 0]} center distanceFactor={12} style={{ pointerEvents: 'none' }}>
          <div
            style={{
              display: 'flex',
              gap: '4px',
              alignItems: 'center',
              padding: '1px 5px',
              borderRadius: '3px',
              background: isHighlighted ? 'rgba(127,29,29,0.92)' : 'rgba(11,17,32,0.88)',
              border: `1px solid ${isHighlighted ? '#ef4444' : '#334155'}`,
              color: '#f1f5f9',
              fontFamily: 'var(--font-mono)',
              fontSize: '10px',
              whiteSpace: 'nowrap',
            }}
          >
            {showWarnings && hasWarning && <span style={{ color: '#f87171', fontWeight: 700 }}>!</span>}
            {showChannels && cableOverlay && (
              <span style={{ color: cableOverlay.overloaded ? '#fca5a5' : '#38bdf8' }}>
                {cableOverlay.channels_used}/{cableOverlay.capacity}ch
              </span>
            )}
            {showChannels && !cableOverlay && routeOverlay && (
              <span style={{ color: routeOverlay.has_channel ? '#6ee7b7' : '#fca5a5' }}>
                {routeOverlay.has_channel ? 'CH OK' : 'NO CH'}
              </span>
            )}
            {showEnergy && def && def.metadata.energy_cost > 0 && isSelected && (
              <span style={{ color: '#fcd34d' }}>-{def.metadata.energy_cost} AE/t</span>
            )}
          </div>
        </Html>
      )}
    </group>
  )
}

export function Workspace3D() {
  const [webglSupported] = useState<boolean>(() => detectWebGLSupport())
  const [ghostCoord, setGhostCoord] = useState<Coord3D | null>(null)

  const blocks = useEditorStore((s) => s.blocks)
  const catalog = useEditorStore((s) => s.catalog)
  const activeMode = useEditorStore((s) => s.activeMode)
  const activeLayerY = useEditorStore((s) => s.activeLayerY)
  const cameraPreset = useEditorStore((s) => s.cameraPreset)
  const activeBlockDefId = useEditorStore((s) => s.activeBlockDefId)
  const selectedBlockIds = useEditorStore((s) => s.selectedBlockIds)
  const highlightedCoords = useEditorStore((s) => s.highlightedCoords)
  const diagnostics = useEditorStore((s) => s.diagnostics)
  const overlays = useEditorStore((s) => s.overlays)
  const simulationRun = useEditorStore((s) => s.simulationRun)
  const validationResult = useEditorStore((s) => s.validationResult)

  const placeBlockAt = useEditorStore((s) => s.placeBlockAt)
  const selectBlock = useEditorStore((s) => s.selectBlock)
  const clearSelection = useEditorStore((s) => s.clearSelection)

  const catalogMap = useMemo(() => {
    const m = new Map<string, BlockDefinition>()
    for (const d of catalog) {
      m.set(d.id, d)
    }
    return m
  }, [catalog])

  const blocksByCoord = useMemo(() => {
    const m = new Map<string, PlacedBlock>()
    for (const b of blocks) {
      m.set(coordKey(b.x, b.y, b.z), b)
    }
    return m
  }, [blocks])

  const activeOverlay = useMemo(
    () => selectActiveOverlay({ simulationRun, validationResult }),
    [simulationRun, validationResult]
  )

  const cableOverlayByCoord = useMemo(() => {
    const m = new Map<string, CableChannelOverlay>()
    for (const c of activeOverlay?.cables ?? []) {
      m.set(coordKey(c.coord.x, c.coord.y, c.coord.z), c)
    }
    return m
  }, [activeOverlay])

  const routeOverlayByCoord = useMemo(() => {
    const m = new Map<string, DeviceRouteOverlay>()
    for (const r of activeOverlay?.device_routes ?? []) {
      m.set(coordKey(r.coord.x, r.coord.y, r.coord.z), r)
    }
    return m
  }, [activeOverlay])

  const highlightedSet = useMemo(
    () => new Set(highlightedCoords.map((c) => coordKey(c.x, c.y, c.z))),
    [highlightedCoords]
  )

  const warningSet = useMemo(() => {
    const s = new Set<string>()
    for (const d of diagnostics) {
      for (const ab of d.affected_blocks ?? []) {
        s.add(coordKey(ab.x, ab.y, ab.z))
      }
    }
    return s
  }, [diagnostics])

  if (!webglSupported) {
    return <Workspace2D fallbackNotice="WebGL unavailable — 2D slice active" />
  }

  const handleGroundMove = (e: ThreeEvent<PointerEvent>) => {
    if (activeMode !== 'build' || !activeBlockDefId) {
      setGhostCoord(null)
      return
    }
    const gx = Math.round(e.point.x)
    const gz = Math.round(e.point.z)
    if (!blocksByCoord.has(coordKey(gx, activeLayerY, gz))) {
      setGhostCoord({ x: gx, y: activeLayerY, z: gz })
    } else {
      setGhostCoord(null)
    }
  }

  const handleGroundClick = (e: ThreeEvent<MouseEvent>) => {
    e.stopPropagation()
    const gx = Math.round(e.point.x)
    const gz = Math.round(e.point.z)
    if (activeMode === 'build' && activeBlockDefId) {
      placeBlockAt({ x: gx, y: activeLayerY, z: gz })
    } else if (!e.nativeEvent.shiftKey) {
      clearSelection()
    }
  }

  const handleBlockPointerMove = (e: ThreeEvent<PointerEvent>, block: PlacedBlock) => {
    e.stopPropagation()
    if (activeMode === 'build' && activeBlockDefId && e.face) {
      const nx = block.x + Math.round(e.face.normal.x)
      const ny = block.y + Math.round(e.face.normal.y)
      const nz = block.z + Math.round(e.face.normal.z)
      if (!blocksByCoord.has(coordKey(nx, ny, nz))) {
        setGhostCoord({ x: nx, y: ny, z: nz })
        return
      }
    }
    setGhostCoord(null)
  }

  const handleBlockClick = (e: ThreeEvent<MouseEvent>, block: PlacedBlock) => {
    e.stopPropagation()
    if (activeMode === 'build' && activeBlockDefId && e.face && e.nativeEvent.altKey) {
      const nx = block.x + Math.round(e.face.normal.x)
      const ny = block.y + Math.round(e.face.normal.y)
      const nz = block.z + Math.round(e.face.normal.z)
      placeBlockAt({ x: nx, y: ny, z: nz })
      return
    }
    selectBlock(block.id, e.nativeEvent.shiftKey)
  }

  return (
    <div
      className="workspace-3d"
      data-testid="workspace-3d"
      style={{ width: '100%', height: '100%', position: 'relative', background: '#0b1120' }}
    >
      <Canvas camera={{ position: [8, 8, 9], fov: 45 }}>
        <color attach="background" args={['#0b1120']} />
        <ambientLight intensity={0.65} />
        <hemisphereLight args={['#e2e8f0', '#0f172a', 0.45]} />
        <directionalLight position={[12, 18, 10]} intensity={1.05} />
        <directionalLight position={[-10, -8, -10]} intensity={0.3} />

        <CameraPresetController preset={cameraPreset} />

        {/* Reference slice grid & origin axes */}
        <gridHelper
          args={[24, 24, '#334155', '#1e293b']}
          position={[0, activeLayerY - 0.5, 0]}
        />
        <axesHelper args={[3.5]} position={[-0.5, -0.49, -0.5]} />

        {/* Interactive placement plane at activeLayerY */}
        <mesh
          position={[0, activeLayerY - 0.5, 0]}
          rotation={[-Math.PI / 2, 0, 0]}
          onPointerMove={handleGroundMove}
          onPointerOut={() => setGhostCoord(null)}
          onClick={handleGroundClick}
        >
          <planeGeometry args={[28, 28]} />
          <meshBasicMaterial transparent opacity={0} depthWrite={false} />
        </mesh>

        {/* Placement Hover Ghost */}
        {ghostCoord && activeMode === 'build' && activeBlockDefId && (
          <mesh position={[ghostCoord.x, ghostCoord.y, ghostCoord.z]}>
            <boxGeometry args={[0.96, 0.96, 0.96]} />
            <meshBasicMaterial color="#8b5cf6" wireframe transparent opacity={0.65} />
          </mesh>
        )}

        {/* Placed AE2 Network Blocks */}
        {blocks.map((block) => {
          const key = coordKey(block.x, block.y, block.z)
          const def = catalogMap.get(block.block_def_id)
          const cableOv = cableOverlayByCoord.get(key)
          const routeOv = routeOverlayByCoord.get(key)
          const connectedDirs = computeConnectedDirections(
            block,
            blocksByCoord,
            catalogMap,
            cableOv
          )
          const hasWarning = Boolean(
            warningSet.has(key) ||
              cableOv?.overloaded ||
              (routeOv && !routeOv.has_channel)
          )

          return (
            <ProceduralBlockMesh
              key={block.id}
              block={block}
              def={def}
              connectedDirs={connectedDirs}
              cableOverlay={cableOv}
              routeOverlay={routeOv}
              isSelected={selectedBlockIds.includes(block.id)}
              isHighlighted={highlightedSet.has(key)}
              showNetworks={overlays.networks}
              showConnections={overlays.connections}
              showChannels={overlays.channels}
              showEnergy={overlays.energy}
              showWarnings={overlays.warnings}
              hasWarning={hasWarning}
              onBlockPointerDown={handleBlockClick}
              onBlockPointerMove={handleBlockPointerMove}
              onBlockPointerOut={() => setGhostCoord(null)}
            />
          )
        })}
      </Canvas>

      <div
        style={{
          position: 'absolute',
          bottom: 10,
          left: 12,
          padding: '4px 10px',
          borderRadius: 4,
          background: 'rgba(17, 24, 39, 0.88)',
          border: '1px solid #1e293b',
          fontFamily: 'var(--font-mono)',
          fontSize: 11,
          color: '#94a3b8',
          pointerEvents: 'none',
        }}
      >
        Click grid (Y={activeLayerY}) to place • Click block to inspect • Alt+Click block face to
        attach • Drag to orbit
      </div>
    </div>
  )
}
