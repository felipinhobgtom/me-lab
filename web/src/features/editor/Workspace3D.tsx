import { useEffect, useMemo, useRef, useState } from 'react'
import type { ElementRef } from 'react'
import { Canvas, useFrame, useThree, type ThreeEvent } from '@react-three/fiber'
import { Html, OrbitControls } from '@react-three/drei'
import * as THREE from 'three'
import type {
  BlockDefinition,
  CableChannelOverlay,
  Coord3D,
  Direction,
  PlacedBlock,
} from '@/lib/api'
import {
  computeConnectedDirections,
  coordKey,
  defaultConfigForBlock,
  isCableBlockId,
  isDenseCableBlockId,
  isSubpartBlockId,
  oppositeDirection,
  readCableConfig,
} from '@/lib/ae2/constants'
import { selectActiveOverlay, useEditorStore, type CameraPreset } from './store'
import { VoxelBlockModel } from './VoxelModels'
import { Workspace2D } from './Workspace2D'

type OrbitControlsInstance = ElementRef<typeof OrbitControls>

const CAMERA_KEYS = new Set(['w', 'a', 's', 'd'])

function isEditableTarget(target: EventTarget | null): boolean {
  const element = target as HTMLElement | null
  if (!element) return false
  return (
    element.isContentEditable ||
    element.tagName === 'INPUT' ||
    element.tagName === 'TEXTAREA' ||
    element.tagName === 'SELECT'
  )
}

function detectWebGLAvailable(): boolean {
  if (typeof window === 'undefined' || typeof document === 'undefined') {
    return false
  }
  if (
    typeof window.WebGLRenderingContext === 'undefined' &&
    typeof window.WebGL2RenderingContext === 'undefined'
  ) {
    return false
  }
  try {
    const canvas = document.createElement('canvas')
    const gl = canvas.getContext('webgl2') ?? canvas.getContext('webgl')
    return Boolean(gl)
  } catch {
    return false
  }
}

function normalToDirection(normal: THREE.Vector3): Direction {
  const ax = Math.abs(normal.x)
  const ay = Math.abs(normal.y)
  const az = Math.abs(normal.z)
  if (ay >= ax && ay >= az) {
    return normal.y >= 0 ? 'up' : 'down'
  }
  if (ax >= az) {
    return normal.x >= 0 ? 'east' : 'west'
  }
  return normal.z >= 0 ? 'south' : 'north'
}

function smartAutoFacing(
  blockDefId: string,
  clickedFace: Direction,
  fallbackFacing: Direction
): Direction {
  // Hoppers and Buses point INTO the block face you clicked against!
  if (blockDefId === 'minecraft:hopper') {
    const into = oppositeDirection(clickedFace)
    return into === 'up' ? 'down' : into
  }
  if (
    blockDefId === 'ae2:import_bus' ||
    blockDefId === 'ae2:export_bus' ||
    blockDefId === 'ae2:storage_bus'
  ) {
    return oppositeDirection(clickedFace)
  }
  // Terminals face OUTWARD along the clicked face normal
  if (
    blockDefId === 'ae2:terminal' ||
    blockDefId === 'ae2:crafting_terminal' ||
    blockDefId === 'ae2:pattern_encoding_terminal' ||
    blockDefId === 'ae2:pattern_access_terminal'
  ) {
    return clickedFace
  }
  // Horizontal blocks (Drive, Furnace, Monitor) adopt horizontal clicked face or current placementFacing
  if (clickedFace !== 'up' && clickedFace !== 'down') {
    if (
      blockDefId === 'ae2:drive' ||
      blockDefId === 'ae2:chest' ||
      blockDefId === 'minecraft:chest' ||
      blockDefId === 'minecraft:furnace' ||
      blockDefId === 'minecraft:blast_furnace' ||
      blockDefId === 'ae2:crafting_monitor'
    ) {
      return clickedFace
    }
  }
  return fallbackFacing
}

function CameraRig({ preset }: { preset: CameraPreset }) {
  const { camera } = useThree()
  useEffect(() => {
    switch (preset) {
      case 'top':
        camera.position.set(0.5, 16, 0.51)
        break
      case 'front':
        camera.position.set(0.5, 4, 14)
        break
      case 'side':
        camera.position.set(14, 4, 0.5)
        break
      case 'orbit':
      default:
        camera.position.set(-7.5, 6.5, 9.5)
        break
    }
    camera.lookAt(0.5, 0.5, 0)
  }, [camera, preset])
  return null
}
function KeyboardCameraControls({
  controlsRef,
}: {
  controlsRef: { current: OrbitControlsInstance | null }
}) {
  const { camera } = useThree()
  const keysRef = useRef<Set<string>>(new Set())

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (isEditableTarget(event.target)) return
      const key = event.key.toLowerCase()
      if (!CAMERA_KEYS.has(key) && key !== 'shift') return
      keysRef.current.add(key)
      if (CAMERA_KEYS.has(key)) event.preventDefault()
    }
    const onKeyUp = (event: KeyboardEvent) => {
      keysRef.current.delete(event.key.toLowerCase())
    }
    const clearKeys = () => keysRef.current.clear()

    window.addEventListener('keydown', onKeyDown)
    window.addEventListener('keyup', onKeyUp)
    window.addEventListener('blur', clearKeys)
    return () => {
      window.removeEventListener('keydown', onKeyDown)
      window.removeEventListener('keyup', onKeyUp)
      window.removeEventListener('blur', clearKeys)
    }
  }, [])

  useFrame((_, delta) => {
    const keys = keysRef.current
    if (![...CAMERA_KEYS].some((key) => keys.has(key))) return

    const forward = new THREE.Vector3()
    camera.getWorldDirection(forward)
    forward.y = 0
    if (forward.lengthSq() < 0.0001) return
    forward.normalize()

    const right = new THREE.Vector3().crossVectors(forward, camera.up).normalize()
    const movement = new THREE.Vector3()
    if (keys.has('w')) movement.add(forward)
    if (keys.has('s')) movement.sub(forward)
    if (keys.has('d')) movement.add(right)
    if (keys.has('a')) movement.sub(right)
    if (movement.lengthSq() === 0) return

    const speed = keys.has('shift') ? 12 : 5
    const offset = movement.normalize().multiplyScalar(speed * delta)
    camera.position.add(offset)
    if (controlsRef.current) {
      controlsRef.current.target.add(offset)
      controlsRef.current.update()
    }
  })

  return null
}


function DiagnosticPulseBox({ active }: { active: boolean }) {
  const matRef = useRef<THREE.MeshBasicMaterial | null>(null)
  useFrame(({ clock }) => {
    if (matRef.current && active) {
      matRef.current.opacity = 0.65 + 0.35 * Math.sin(clock.getElapsedTime() * 6)
    }
  })
  if (!active) return null
  return (
    <mesh>
      <boxGeometry args={[1.06, 1.06, 1.06]} />
      <meshBasicMaterial ref={matRef} color="#ef4444" wireframe transparent opacity={0.9} />
    </mesh>
  )
}

interface HoverTarget {
  placeCoord: Coord3D
  clickedBlockId?: string
  clickedFace: Direction
  attachAsSubpart: boolean
}
interface DragState {
  pointerId: number
  planeY: number
  startPoint: THREE.Vector3
  blockIds: string[]
}


function VoxelSandboxScene() {
  const blocks = useEditorStore((s) => s.blocks)
  const catalog = useEditorStore((s) => s.catalog)
  const selectedBlockIds = useEditorStore((s) => s.selectedBlockIds)
  const activeBlockDefId = useEditorStore((s) => s.activeBlockDefId)
  const activeCableColor = useEditorStore((s) => s.activeCableColor)
  const placementFacing = useEditorStore((s) => s.placementFacing)
  const cameraPreset = useEditorStore((s) => s.cameraPreset)
  const overlays = useEditorStore((s) => s.overlays)
  const highlightedCoords = useEditorStore((s) => s.highlightedCoords)
  const validationResult = useEditorStore((s) => s.validationResult)
  const simulationRun = useEditorStore((s) => s.simulationRun)

  const selectBlock = useEditorStore((s) => s.selectBlock)
  const setActiveBlockDefId = useEditorStore((s) => s.setActiveBlockDefId)
  const setActiveCableColor = useEditorStore((s) => s.setActiveCableColor)
  const setInspectorOpen = useEditorStore((s) => s.setInspectorOpen)
  const placeBlockAt = useEditorStore((s) => s.placeBlockAt)
  const attachSubpartToCable = useEditorStore((s) => s.attachSubpartToCable)
  const removeSubpartFromCable = useEditorStore((s) => s.removeSubpartFromCable)
  const removeBlocks = useEditorStore((s) => s.removeBlocks)
  const moveBlocks = useEditorStore((s) => s.moveBlocks)

  const controlsRef = useRef<OrbitControlsInstance | null>(null)
  const dragStateRef = useRef<DragState | null>(null)
  const [dragOffset, setDragOffset] = useState<Coord3D>({ x: 0, y: 0, z: 0 })
  const [isDragging, setIsDragging] = useState(false)

  const [hoverTarget, setHoverTarget] = useState<HoverTarget | null>(null)
  const pointerDownRef = useRef<{ x: number; y: number } | null>(null)

  const recordPointerDown = (e: ThreeEvent<PointerEvent>) => {
    pointerDownRef.current = { x: e.nativeEvent.clientX, y: e.nativeEvent.clientY }
  }

  const wasDragged = (e: ThreeEvent<MouseEvent>): boolean => {
    if (!pointerDownRef.current) return false
    const dx = e.nativeEvent.clientX - pointerDownRef.current.x
    const dy = e.nativeEvent.clientY - pointerDownRef.current.y
    return dx * dx + dy * dy > 36 // > 6px movement = camera orbit/pan
  }
  const getDragPlanePoint = (
    e: ThreeEvent<PointerEvent>,
    planeY: number
  ): THREE.Vector3 | null => {
    const plane = new THREE.Plane(new THREE.Vector3(0, 1, 0), -planeY)
    return e.ray.intersectPlane(plane, new THREE.Vector3())
  }


  const activeOverlay = selectActiveOverlay({ simulationRun, validationResult })

  const catalogMap = useMemo(() => {
    const m = new Map<string, BlockDefinition>()
    for (const d of catalog) m.set(d.id, d)
    return m
  }, [catalog])

  const blocksByCoord = useMemo(() => {
    const m = new Map<string, PlacedBlock>()
    for (const b of blocks) m.set(coordKey(b.x, b.y, b.z), b)
    return m
  }, [blocks])

  const cableOverlayById = useMemo(() => {
    const m = new Map<string, CableChannelOverlay>()
    if (activeOverlay) {
      for (const c of activeOverlay.cables) m.set(c.block_id, c)
    }
    return m
  }, [activeOverlay])

  const deviceRouteById = useMemo(() => {
    const m = new Map<string, { has_channel: boolean; network_id: string }>()
    if (activeOverlay) {
      for (const r of activeOverlay.device_routes) {
        m.set(r.block_id, { has_channel: r.has_channel, network_id: r.network_id })
      }
    }
    return m
  }, [activeOverlay])

  const invalidControllerNetIds = useMemo(() => {
    const out: Record<string, true> = {}
    if (activeOverlay) {
      for (const n of activeOverlay.networks) {
        if (n.has_controller && !n.controller_valid) {
          out[n.id] = true
        }
      }
    }
    return out
  }, [activeOverlay])

  const isRunningJobs = Boolean(
    activeOverlay?.crafting_jobs.some((j) => j.status === 'running')
  )

  const highlightedSet = useMemo(() => {
    const s = new Set<string>()
    for (const c of highlightedCoords) s.add(coordKey(c.x, c.y, c.z))
    return s
  }, [highlightedCoords])

  const computeBlockFaceTarget = (
    block: PlacedBlock,
    e: ThreeEvent<PointerEvent | MouseEvent>
  ): HoverTarget => {
    const normal = e.face?.normal ? e.face.normal.clone() : new THREE.Vector3(0, 1, 0)
    // Transform local mesh normal to world space
    normal.transformDirection(e.object.matrixWorld)
    const face = normalToDirection(normal)
    const nx = block.x + Math.round(normal.x)
    const ny = block.y + Math.round(normal.y)
    const nz = block.z + Math.round(normal.z)

    const def = catalogMap.get(block.block_def_id)
    const isNonDenseCable =
      isCableBlockId(block.block_def_id, def) && !isDenseCableBlockId(block.block_def_id, def)
    const attachAsSubpart = Boolean(
      activeBlockDefId &&
        activeBlockDefId !== '__break__' &&
        isSubpartBlockId(activeBlockDefId) &&
        isNonDenseCable &&
        !e.nativeEvent.altKey
    )

    return {
      placeCoord: { x: nx, y: ny, z: nz },
      clickedBlockId: block.id,
      clickedFace: face,
      attachAsSubpart,
    }
  }

  const handleGroundMove = (e: ThreeEvent<PointerEvent>) => {
    e.stopPropagation()
    const gx = Math.round(e.point.x)
    const gz = Math.round(e.point.z)
    if (Math.abs(gx) > 32 || Math.abs(gz) > 32) {
      setHoverTarget(null)
      return
    }
    setHoverTarget({
      placeCoord: { x: gx, y: 0, z: gz },
      clickedFace: 'up',
      attachAsSubpart: false,
    })
  }

  const handleGroundClick = (e: ThreeEvent<MouseEvent>) => {
    e.stopPropagation()
    if (wasDragged(e)) return
    if (!activeBlockDefId || activeBlockDefId === '__break__' || e.nativeEvent.shiftKey) {
      selectBlock(null)
      return
    }
    const gx = Math.round(e.point.x)
    const gz = Math.round(e.point.z)
    if (!blocksByCoord.has(coordKey(gx, 0, gz))) {
      placeBlockAt({ x: gx, y: 0, z: gz }, placementFacing)
    }
  }

  const handleBlockMove = (block: PlacedBlock, e: ThreeEvent<PointerEvent>) => {
    e.stopPropagation()
    setHoverTarget(computeBlockFaceTarget(block, e))
  }
  const handleBlockPointerDown = (block: PlacedBlock, e: ThreeEvent<PointerEvent>) => {
    recordPointerDown(e)
    e.stopPropagation()
    if (e.button !== 0 || activeBlockDefId !== null || e.nativeEvent.shiftKey) return

    const blockIds = selectedBlockIds.includes(block.id)
      ? selectedBlockIds
      : [block.id]
    const startPoint = getDragPlanePoint(e, block.y)
    if (!startPoint) return

    if (!selectedBlockIds.includes(block.id)) {
      selectBlock(block.id, false)
    }
    dragStateRef.current = {
      pointerId: e.pointerId,
      planeY: block.y,
      startPoint,
      blockIds,
    }
    setDragOffset({ x: 0, y: 0, z: 0 })
    setIsDragging(true)
    if (controlsRef.current) controlsRef.current.enabled = false
    const pointerTarget = e.target as unknown as {
      setPointerCapture?: (pointerId: number) => void
    }
    pointerTarget.setPointerCapture?.(e.pointerId)
  }

  const handleBlockDragMove = (block: PlacedBlock, e: ThreeEvent<PointerEvent>) => {
    const drag = dragStateRef.current
    if (!drag || drag.pointerId !== e.pointerId || !drag.blockIds.includes(block.id)) {
      handleBlockMove(block, e)
      return
    }
    e.stopPropagation()
    const point = getDragPlanePoint(e, drag.planeY)
    if (!point) return
    const nextOffset = {
      x: Math.round(point.x - drag.startPoint.x),
      y: 0,
      z: Math.round(point.z - drag.startPoint.z),
    }
    setDragOffset((current) =>
      current.x === nextOffset.x && current.z === nextOffset.z ? current : nextOffset
    )
  }

  const finishBlockDrag = (e: ThreeEvent<PointerEvent>) => {
    const drag = dragStateRef.current
    if (!drag || drag.pointerId !== e.pointerId) return

    e.stopPropagation()
    if (dragOffset.x !== 0 || dragOffset.y !== 0 || dragOffset.z !== 0) {
      moveBlocks(drag.blockIds, dragOffset)
    }
    dragStateRef.current = null
    setDragOffset({ x: 0, y: 0, z: 0 })
    setIsDragging(false)
    if (controlsRef.current) controlsRef.current.enabled = true
    const pointerTarget = e.target as unknown as {
      releasePointerCapture?: (pointerId: number) => void
    }
    pointerTarget.releasePointerCapture?.(e.pointerId)
  }


  const handleBlockClick = (block: PlacedBlock, e: ThreeEvent<MouseEvent>) => {
    e.stopPropagation()
    if (wasDragged(e)) return

    const target = computeBlockFaceTarget(block, e)

    // Shift+Click or Break tool -> break subpart on clicked face first, or break block
    if (e.nativeEvent.shiftKey || activeBlockDefId === '__break__') {
      if (isCableBlockId(block.block_def_id)) {
        const { subparts } = readCableConfig(block.config)
        if (subparts[target.clickedFace]) {
          removeSubpartFromCable(block.id, target.clickedFace)
          return
        }
      }
      removeBlocks([block.id])
      return
    }

    // Hand / Inspect cursor (activeBlockDefId === null) -> select & open config GUI
    if (!activeBlockDefId) {
      selectBlock(block.id, false)
      setInspectorOpen(true)
      return
    }

    // Attach Terminal / Bus / Quartz Fiber / Anchor directly onto a non-dense Cable face!
    if (target.attachAsSubpart) {
      attachSubpartToCable(block.id, target.clickedFace, activeBlockDefId)
      return
    }

    // Place held block on the adjacent voxel face!
    const destKey = coordKey(target.placeCoord.x, target.placeCoord.y, target.placeCoord.z)
    if (!blocksByCoord.has(destKey)) {
      const autoFacing = smartAutoFacing(activeBlockDefId, target.clickedFace, placementFacing)
      placeBlockAt(target.placeCoord, autoFacing)
    } else {
      selectBlock(block.id, false)
    }
  }

  const handleBlockDoubleClick = (block: PlacedBlock, e: ThreeEvent<MouseEvent>) => {
    e.stopPropagation()
    selectBlock(block.id, false)
    setInspectorOpen(true)
  }

  const handleBlockAuxClick = (block: PlacedBlock, e: ThreeEvent<MouseEvent>) => {
    // Middle-click = Minecraft Pick Block!
    if (e.nativeEvent.button === 1) {
      e.stopPropagation()
      setActiveBlockDefId(block.block_def_id)
      if (isCableBlockId(block.block_def_id)) {
        setActiveCableColor(readCableConfig(block.config).color)
      }
    }
  }

  // Ghost block preview
  const ghostBlock = useMemo((): PlacedBlock | null => {
    if (
      !hoverTarget ||
      !activeBlockDefId ||
      activeBlockDefId === '__break__' ||
      hoverTarget.attachAsSubpart
    ) {
      return null
    }
    const { x, y, z } = hoverTarget.placeCoord
    if (blocksByCoord.has(coordKey(x, y, z))) {
      return null
    }
    const def = catalogMap.get(activeBlockDefId)
    return {
      id: '__ghost__',
      project_id: '',
      block_def_id: activeBlockDefId,
      x,
      y,
      z,
      facing: smartAutoFacing(activeBlockDefId, hoverTarget.clickedFace, placementFacing),
      config: defaultConfigForBlock(def, activeBlockDefId, activeCableColor),
      created_at: '',
      updated_at: '',
    }
  }, [
    activeBlockDefId,
    activeCableColor,
    blocksByCoord,
    catalogMap,
    hoverTarget,
    placementFacing,
  ])

  const ghostConnectedDirs = useMemo(() => {
    if (!ghostBlock) return []
    return computeConnectedDirections(ghostBlock, blocksByCoord, catalogMap)
  }, [blocksByCoord, catalogMap, ghostBlock])

  return (
    <>
      <color attach="background" args={['#171717']} />
      <ambientLight intensity={1.25} />
      <directionalLight position={[10, 18, 12]} intensity={1.15} />
      <directionalLight position={[-10, 8, -10]} intensity={0.45} />

      <CameraRig preset={cameraPreset} />
      <KeyboardCameraControls controlsRef={controlsRef} />
      <OrbitControls
        ref={controlsRef}
        makeDefault
        enableDamping
        dampingFactor={0.1}
        enabled={!isDragging}
        target={[0.5, 0.5, 0]}
      />

      {/* Subtle AE2 Guide neutral dark voxel grid aligned to 1x1 block edges */}
      <gridHelper args={[32, 32, '#2e2e2e', '#242424']} position={[0.5, -0.5, 0.5]} />

      {/* Invisible Y=0 raycast build plane */}
      <mesh
        rotation={[-Math.PI / 2, 0, 0]}
        position={[0.5, -0.501, 0.5]}
        onPointerDown={recordPointerDown}
        onPointerMove={handleGroundMove}
        onPointerOut={() => setHoverTarget(null)}
        onClick={handleGroundClick}
      >
        <planeGeometry args={[64, 64]} />
        <meshBasicMaterial visible={false} />
      </mesh>

      {/* Placed AE2 & Minecraft Blocks */}
      {blocks.map((block) => {
        const def = catalogMap.get(block.block_def_id)
        const cOverlay = cableOverlayById.get(block.id)
        const dRoute = deviceRouteById.get(block.id)
        const connectedDirs = computeConnectedDirections(
          block,
          blocksByCoord,
          catalogMap,
          overlays.connections ? cOverlay : undefined
        )
        const isSelected = selectedBlockIds.includes(block.id)
        const isDiagHighlighted =
          overlays.warnings && highlightedSet.has(coordKey(block.x, block.y, block.z))
        const isDragTarget = dragStateRef.current?.blockIds.includes(block.id) ?? false
        const renderOffset = isDragTarget ? dragOffset : { x: 0, y: 0, z: 0 }

        const online =
          cOverlay !== undefined
            ? cOverlay.online
            : dRoute !== undefined
              ? dRoute.has_channel
              : true
        const invalidCtrl = Boolean(
          cOverlay?.network_id && invalidControllerNetIds[cOverlay.network_id]
        )

        return (
          <group
            key={block.id}
            position={[
              block.x + renderOffset.x,
              block.y + renderOffset.y,
              block.z + renderOffset.z,
            ]}
            onPointerDown={(e) => handleBlockPointerDown(block, e)}
            onPointerMove={(e) => handleBlockDragMove(block, e)}
            onClick={(e) => handleBlockClick(block, e)}
            onDoubleClick={(e) => handleBlockDoubleClick(block, e)}
            onPointerUp={(e) => {
              finishBlockDrag(e)
              if (e.nativeEvent.button === 1) {
                handleBlockAuxClick(block, e as unknown as ThreeEvent<MouseEvent>)
              }
            }}
            onPointerCancel={finishBlockDrag}
          >
            <VoxelBlockModel
              block={block}
              def={def}
              blocksByCoord={blocksByCoord}
              connectedDirs={connectedDirs}
              cableOverlay={cOverlay}
              online={online}
              invalidController={invalidCtrl}
              isCraftingActive={isRunningJobs}
            />

            {/* Subtle Minecraft black/white selection box */}
            {isSelected && (
              <mesh>
                <boxGeometry args={[1.015, 1.015, 1.015]} />
                <meshBasicMaterial color="#e5e5e5" wireframe transparent opacity={0.45} />
              </mesh>
            )}

            {/* Crimson diagnostic fault wireframe when inspecting errors */}
            <DiagnosticPulseBox active={isDiagHighlighted} />

            {/* Optional Opt-In Channel Overlay Tag (off by default to keep scene clean!) */}
            {overlays.channels && (cOverlay || dRoute) && (
              <Html position={[0, 0.68, 0]} center distanceFactor={11}>
                <div
                  style={{
                    background:
                      cOverlay?.overloaded || (dRoute && !dRoute.has_channel)
                        ? 'rgba(153, 27, 27, 0.92)'
                        : 'rgba(23, 23, 23, 0.88)',
                    border: '1px solid rgba(255,255,255,0.16)',
                    color: '#f5f5f5',
                    padding: '1px 5px',
                    borderRadius: '3px',
                    fontSize: '10px',
                    fontFamily: 'ui-monospace, monospace',
                    whiteSpace: 'nowrap',
                    pointerEvents: 'none',
                  }}
                >
                  {cOverlay
                    ? `${cOverlay.channels_used}/${cOverlay.capacity} ch`
                    : dRoute?.has_channel
                      ? '1 ch'
                      : 'NO CH'}
                </div>
              </Html>
            )}
          </group>
        )
      })}

      {/* Translucent Placement Ghost Preview */}
      {ghostBlock && (
        <group position={[ghostBlock.x, ghostBlock.y, ghostBlock.z]}>
          <VoxelBlockModel
            block={ghostBlock}
            def={catalogMap.get(ghostBlock.block_def_id)}
            blocksByCoord={blocksByCoord}
            connectedDirs={ghostConnectedDirs}
            opacity={0.45}
          />
          <mesh>
            <boxGeometry args={[1.01, 1.01, 1.01]} />
            <meshBasicMaterial color="#ffffff" wireframe transparent opacity={0.28} />
          </mesh>
        </group>
      )}

      {/* Subtle Wireframe Indicator when attaching a Terminal/Bus onto an existing Cable */}
      {hoverTarget?.attachAsSubpart && hoverTarget.clickedBlockId && (
        (() => {
          const cb = blocks.find((b) => b.id === hoverTarget.clickedBlockId)
          if (!cb) return null
          return (
            <group position={[cb.x, cb.y, cb.z]}>
              <mesh>
                <boxGeometry args={[0.85, 0.85, 0.85]} />
                <meshBasicMaterial color="#a3e635" wireframe transparent opacity={0.6} />
              </mesh>
            </group>
          )
        })()
      )}
    </>
  )
}

export function Workspace3D() {
  const [webglSupported] = useState<boolean>(() => detectWebGLAvailable())

  if (!webglSupported) {
    return <Workspace2D fallbackNotice="WebGL unavailable — showing 2D layer grid" />
  }

  return (
    <div className="sandbox-canvas-wrap" data-testid="workspace-3d">
      <Canvas
        camera={{ position: [-7.5, 6.5, 9.5], fov: 42, near: 0.1, far: 250 }}
        gl={{ antialias: true, powerPreference: 'high-performance' }}
      >
        <VoxelSandboxScene />
      </Canvas>
    </div>
  )
}
