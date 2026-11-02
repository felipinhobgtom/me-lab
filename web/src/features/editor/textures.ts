import * as THREE from 'three'
import type { CableColor } from '@/lib/ae2/constants'

const textureCache = new Map<string, THREE.Texture>()
const terminalScreenCache = new Map<string, THREE.CanvasTexture>()
const loader = new THREE.TextureLoader()

function configurePixelTexture(tex: THREE.Texture): THREE.Texture {
  tex.magFilter = THREE.NearestFilter
  tex.minFilter = THREE.NearestFilter
  tex.generateMipmaps = false
  tex.colorSpace = THREE.SRGBColorSpace
  tex.needsUpdate = true
  return tex
}

export function loadPixelTexture(url: string): THREE.Texture {
  const existing = textureCache.get(url)
  if (existing) {
    return existing
  }
  const tex = configurePixelTexture(loader.load(url))
  textureCache.set(url, tex)
  return tex
}

export function cableColorToAssetName(color: CableColor | string | undefined): string {
  if (!color || color === 'fluix') {
    return 'transparent'
  }
  return color
}

type TerminalVariant =
  | 'terminal'
  | 'crafting_terminal'
  | 'pattern_encoding_terminal'
  | 'pattern_access_terminal'

function loadImageElement(url: string): Promise<HTMLImageElement> {
  const { promise, resolve, reject } = Promise.withResolvers<HTMLImageElement>()
  const img = new Image()
  img.crossOrigin = 'anonymous'
  img.onload = () => resolve(img)
  img.onerror = (e) => reject(e)
  img.src = url
  return promise
}

function drawTintedMask(
  ctx: CanvasRenderingContext2D,
  img: HTMLImageElement,
  tintHex: string
) {
  const off = document.createElement('canvas')
  off.width = 16
  off.height = 16
  const octx = off.getContext('2d')
  if (!octx) return
  octx.drawImage(img, 0, 0, 16, 16)
  octx.globalCompositeOperation = 'multiply'
  octx.fillStyle = tintHex
  octx.fillRect(0, 0, 16, 16)
  octx.globalCompositeOperation = 'destination-in'
  octx.drawImage(img, 0, 0, 16, 16)
  ctx.drawImage(off, 0, 0)
}

/**
 * Composites AE2's multi-layer grayscale terminal screen masks (`_dark`, `_medium`, `_bright`)
 * with official AE2 Certus/Fluix screen blues onto a crisp 16x16 pixel CanvasTexture.
 */
export function getTerminalScreenTexture(variant: TerminalVariant, online = true): THREE.Texture {
  const key = `${variant}:${online ? 'on' : 'off'}`
  const cached = terminalScreenCache.get(key)
  if (cached) {
    return cached
  }

  const canvas = document.createElement('canvas')
  canvas.width = 16
  canvas.height = 16
  const ctx = canvas.getContext('2d')
  if (ctx) {
    // Immediate pixel fallback while PNG masks finish decoding
    ctx.fillStyle = '#c8cdd6'
    ctx.fillRect(0, 0, 16, 16)
    ctx.fillStyle = online ? '#2574db' : '#262b33'
    ctx.fillRect(2, 2, 12, 12)
  }

  const tex = new THREE.CanvasTexture(canvas)
  configurePixelTexture(tex)
  terminalScreenCache.set(key, tex)

  if (typeof window !== 'undefined' && ctx) {
    void Promise.all([
      loadImageElement('/textures/ae2/part/monitor_back.png'),
      loadImageElement(`/textures/ae2/part/${variant}_dark.png`),
      loadImageElement(`/textures/ae2/part/${variant}_medium.png`),
      loadImageElement(`/textures/ae2/part/${variant}_bright.png`),
    ])
      .then(([bezel, dark, medium, bright]) => {
        ctx.clearRect(0, 0, 16, 16)
        ctx.drawImage(bezel, 0, 0, 16, 16)
        if (online) {
          drawTintedMask(ctx, dark, '#1d5bbf')
          drawTintedMask(ctx, medium, '#3894ff')
          drawTintedMask(ctx, bright, '#a8dcff')
        } else {
          drawTintedMask(ctx, dark, '#1f242d')
          drawTintedMask(ctx, medium, '#2d3542')
          drawTintedMask(ctx, bright, '#475569')
        }
        tex.needsUpdate = true
      })
      .catch(() => {
        // Fallback already painted
      })
  }

  return tex
}

/**
 * Returns the crisp 16x16 official PNG icon path for any catalog block in the hotbar / palette.
 */
export function getBlockIconUrl(blockDefId: string, cableColor: CableColor = 'fluix'): string {
  const col = cableColorToAssetName(cableColor)
  switch (blockDefId) {
    case 'ae2:fluix_cable':
      return `/textures/ae2/part/cable/glass/${col}.png`
    case 'ae2:covered_cable':
      return `/textures/ae2/part/cable/covered/${col}.png`
    case 'ae2:smart_cable':
      return `/textures/ae2/part/cable/smart/${col}.png`
    case 'ae2:dense_cable':
      return `/textures/ae2/part/cable/dense_covered/${col}.png`
    case 'ae2:dense_smart_cable':
      return `/textures/ae2/part/cable/dense_smart/${col}.png`
    case 'ae2:quartz_fiber':
      return '/textures/ae2/part/quartz_fiber.png'
    case 'ae2:cable_anchor':
      return '/textures/ae2/part/cable_anchor.png'
    case 'ae2:controller':
      return '/textures/ae2/block/controller.png'
    case 'ae2:energy_acceptor':
      return '/textures/ae2/block/energy_acceptor.png'
    case 'ae2:energy_cell':
      return '/textures/ae2/block/energy_cell_4.png'
    case 'ae2:dense_energy_cell':
      return '/textures/ae2/block/dense_energy_cell_4.png'
    case 'ae2:creative_energy_cell':
      return '/textures/ae2/block/creative_energy_cell.png'
    case 'ae2:drive':
      return '/textures/ae2/block/drive/drive_front.png'
    case 'ae2:chest':
      return '/textures/ae2/block/chest/front.png'
    case 'ae2:interface':
      return '/textures/ae2/block/interface.png'
    case 'ae2:terminal':
      return '/textures/ae2/part/terminal_bright.png'
    case 'ae2:crafting_terminal':
      return '/textures/ae2/part/crafting_terminal_bright.png'
    case 'ae2:pattern_encoding_terminal':
      return '/textures/ae2/part/pattern_encoding_terminal_bright.png'
    case 'ae2:pattern_access_terminal':
      return '/textures/ae2/part/pattern_access_terminal_bright.png'
    case 'ae2:import_bus':
      return '/textures/ae2/part/import_bus.png'
    case 'ae2:export_bus':
      return '/textures/ae2/part/export_bus.png'
    case 'ae2:storage_bus':
      return '/textures/ae2/part/storage_bus.png'
    case 'ae2:pattern_provider':
      return '/textures/ae2/block/pattern_provider.png'
    case 'ae2:molecular_assembler':
      return '/textures/ae2/block/molecular_assembler.png'
    case 'ae2:crafting_unit':
      return '/textures/ae2/block/crafting/unit.png'
    case 'ae2:crafting_storage_1k':
      return '/textures/ae2/block/crafting/1k_storage.png'
    case 'ae2:crafting_storage_4k':
      return '/textures/ae2/block/crafting/4k_storage.png'
    case 'ae2:crafting_storage_16k':
      return '/textures/ae2/block/crafting/16k_storage.png'
    case 'ae2:crafting_storage_64k':
      return '/textures/ae2/block/crafting/64k_storage.png'
    case 'ae2:crafting_storage_256k':
      return '/textures/ae2/block/crafting/256k_storage.png'
    case 'ae2:crafting_co_processor':
      return '/textures/ae2/block/crafting/accelerator.png'
    case 'ae2:crafting_monitor':
      return '/textures/ae2/block/crafting/monitor.png'
    case 'ae2:inscriber':
      return '/textures/ae2/block/inscriber.png'
    case 'ae2:charger':
      return '/textures/ae2/block/charger.png'
    case 'minecraft:furnace':
      return '/textures/minecraft/block/furnace_front.png'
    case 'minecraft:blast_furnace':
      return '/textures/minecraft/block/blast_furnace_front.png'
    case 'minecraft:hopper':
      return '/textures/minecraft/block/hopper_outside.png'
    case 'minecraft:barrel':
      return '/textures/minecraft/block/barrel_side.png'
    case 'ae2:p2p_tunnel':
      return '/textures/ae2/part/p2p_tunnel_front.png'
    case 'ae2:quantum_ring':
      return '/textures/ae2/block/quantum_ring.png'
    default:
      return '/textures/ae2/block/generics/side.png'
  }
}
