import { useMemo, useState } from 'react'
import type { ExternalInventoryView, ItemStack } from '../../lib/api'
import { VANILLA_ITEMS } from '../../lib/minecraft_vanilla_items'
import { selectActiveOverlay, useEditorStore } from '../editor/store'
import './VanillaInventoryModal.css'

type VanillaInventoryModalProps = {
  blockId: string
  onClose: () => void
}

const CONTAINER_IDS = new Set(['minecraft:chest', 'minecraft:barrel', 'minecraft:hopper'])
const FURNACE_IDS = new Set(['minecraft:furnace', 'minecraft:blast_furnace'])

function parseStacks(raw: unknown): ItemStack[] {
  if (!Array.isArray(raw)) return []
  return raw.flatMap((entry) => {
    if (!entry || typeof entry !== 'object') return []
    const value = entry as { item_id?: unknown; quantity?: unknown }
    const itemId = typeof value.item_id === 'string' ? value.item_id.trim() : ''
    const quantity = typeof value.quantity === 'number' ? Math.floor(value.quantity) : 0
    return itemId && quantity > 0 ? [{ item_id: itemId, quantity }] : []
  })
}

function insertStack(items: ItemStack[], stack: ItemStack, capacity: number): ItemStack[] {
  const itemId = stack.item_id.trim()
  if (!itemId || stack.quantity < 1) return items
  const existingIndex = items.findIndex((item) => item.item_id === itemId)
  if (existingIndex >= 0) {
    return items.map((item, index) =>
      index === existingIndex ? { ...item, quantity: Math.min(64, item.quantity + stack.quantity) } : item
    )
  }
  if (items.length >= capacity) return items
  return [...items, { item_id: itemId, quantity: Math.min(64, stack.quantity) }]
}

function itemLabel(itemId: string): string {
  return VANILLA_ITEMS.find((item) => item.id === itemId)?.name ?? itemId.split(':').at(-1) ?? itemId
}

function ItemSlot({ stack, onClick, label }: { stack?: ItemStack; onClick?: () => void; label: string }) {
  return (
    <button
      type="button"
      className={`vanilla-slot ${stack ? 'vanilla-slot-filled' : ''}`}
      aria-label={label}
      title={stack ? `${itemLabel(stack.item_id)} · ${stack.quantity}` : label}
      onClick={onClick}
      disabled={!onClick}
    >
      {stack && (
        <>
          <span className="vanilla-slot-item">{itemLabel(stack.item_id)}</span>
          <strong>{stack.quantity}</strong>
        </>
      )}
    </button>
  )
}

function EmptyPlayerInventory() {
  return (
    <div className="vanilla-player-inventory" aria-label="Player inventory">
      <div className="vanilla-slot-grid vanilla-player-grid">
        {Array.from({ length: 27 }, (_, index) => (
          <ItemSlot key={`player-${index}`} label={`Player inventory slot ${index + 1}`} />
        ))}
      </div>
      <div className="vanilla-slot-grid vanilla-hotbar-grid">
        {Array.from({ length: 9 }, (_, index) => (
          <ItemSlot key={`hotbar-${index}`} label={`Player hotbar slot ${index + 1}`} />
        ))}
      </div>
    </div>
  )
}

export function VanillaInventoryModal({ blockId, onClose }: VanillaInventoryModalProps) {
  const blocks = useEditorStore((state) => state.blocks)
  const simulationRun = useEditorStore((state) => state.simulationRun)
  const validationResult = useEditorStore((state) => state.validationResult)
  const updateBlockFields = useEditorStore((state) => state.updateBlockFields)
  const block = blocks.find((candidate) => candidate.id === blockId)
  const activeOverlay = useMemo(
    () => selectActiveOverlay({ simulationRun, validationResult }),
    [simulationRun, validationResult]
  )
  const [itemIdInput, setItemIdInput] = useState('minecraft:oak_log')
  const [itemQuantityInput, setItemQuantityInput] = useState(1)
  const [itemSearch, setItemSearch] = useState('')
  const [machineTarget, setMachineTarget] = useState<'input' | 'fuel'>('input')
  const visibleItems = useMemo(() => {
    const query = itemSearch.trim().toLowerCase()
    return VANILLA_ITEMS.filter(
      (item) => !query || item.id.includes(query) || item.name.toLowerCase().includes(query)
    ).slice(0, 96)
  }, [itemSearch])

  if (!block) return null

  const isFurnace = FURNACE_IDS.has(block.block_def_id)
  const isContainer = CONTAINER_IDS.has(block.block_def_id)
  const externalInventory: ExternalInventoryView | null =
    activeOverlay?.external_inventories?.find((inventory) => inventory.block_id === block.id) ?? null
  const containerCapacity = block.block_def_id === 'minecraft:hopper' ? 5 : 27
  const containerItems = externalInventory?.items ?? parseStacks(block.config.inventory)
  const machineInput = externalInventory?.input ?? parseStacks(block.config.input)
  const machineFuel = externalInventory?.fuel ?? parseStacks(block.config.fuel)
  const machineOutput = externalInventory?.output ?? parseStacks(block.config.output)

  const updateStacks = (key: string, items: ItemStack[]) => {
    updateBlockFields(block.id, { config: { ...block.config, [key]: items } })
  }
  const addContainerItem = (itemId = itemIdInput) => {
    updateStacks(
      'inventory',
      insertStack(containerItems, { item_id: itemId, quantity: itemQuantityInput }, containerCapacity)
    )
  }
  const addMachineItem = (target: 'input' | 'fuel', itemId = itemIdInput) => {
    const current = target === 'input' ? machineInput : machineFuel
    updateStacks(target, insertStack(current, { item_id: itemId, quantity: itemQuantityInput }, 1))
  }
  const chooseItem = (itemId: string) => {
    setItemIdInput(itemId)
    if (isContainer) addContainerItem(itemId)
    if (isFurnace) addMachineItem(machineTarget, itemId)
  }
  const removeContainerItem = (slot: number) => {
    updateStacks('inventory', containerItems.filter((_, index) => index !== slot))
  }
  const removeMachineItem = (key: 'input' | 'fuel' | 'output') => {
    updateStacks(key, [])
  }
  const title = block.block_def_id === 'minecraft:barrel'
    ? 'Barrel'
    : block.block_def_id === 'minecraft:hopper'
      ? 'Hopper'
      : block.block_def_id === 'minecraft:blast_furnace'
        ? 'Blast Furnace'
        : block.block_def_id === 'minecraft:smoker'
          ? 'Smoker'
          : isFurnace
            ? 'Furnace'
            : 'Chest'

  return (
    <div className="vanilla-inventory-backdrop" onClick={onClose}>
      <section
        className="vanilla-inventory-modal"
        role="dialog"
        aria-label={`${title} inventory`}
        data-testid={isFurnace ? 'furnace-inventory-panel' : 'container-inventory-panel'}
        onClick={(event) => event.stopPropagation()}
      >
        <header className="vanilla-inventory-header">
          <div>
            <span className="vanilla-inventory-kicker">Minecraft Java 1.21.1</span>
            <h2>{title}</h2>
          </div>
          <button type="button" className="vanilla-close-button" aria-label="Close inventory" onClick={onClose}>
            ×
          </button>
        </header>

        <div className="vanilla-inventory-main">
          <div className="vanilla-inventory-left">
            {isFurnace ? (
              <>
                <div className="vanilla-machine-grid" aria-label="Furnace slots">
                  <div className="vanilla-machine-column">
                    <span>Input</span>
                    <ItemSlot stack={machineInput[0]} label="Furnace input slot" onClick={() => removeMachineItem('input')} />
                  </div>
                  <div className="vanilla-machine-column">
                    <span>Fuel</span>
                    <ItemSlot stack={machineFuel[0]} label="Furnace fuel slot" onClick={() => removeMachineItem('fuel')} />
                  </div>
                  <div className="vanilla-machine-arrow">→</div>
                  <div className="vanilla-machine-column">
                    <span>Output</span>
                    <ItemSlot stack={machineOutput[0]} label="Furnace output slot" onClick={() => removeMachineItem('output')} />
                  </div>
                </div>
                <div className="vanilla-machine-status">
                  <span>{externalInventory?.lit ? 'Burning' : 'Idle'}</span>
                  <span>{externalInventory?.cook_time ?? 0}/{externalInventory?.cook_time_total ?? 200} ticks</span>
                </div>
                <div className="vanilla-progress"><span style={{ width: `${Math.min(100, ((externalInventory?.cook_time ?? 0) / Math.max(1, externalInventory?.cook_time_total ?? 200)) * 100)}%` }} /></div>
              </>
            ) : (
              <>
                <div className="vanilla-slot-grid vanilla-container-grid" aria-label={`${title} inventory`}>
                  {Array.from({ length: containerCapacity }, (_, slot) => (
                    <ItemSlot
                      key={`${block.id}-slot-${slot}`}
                      stack={containerItems[slot]}
                      label={containerItems[slot] ? `Remove ${itemLabel(containerItems[slot].item_id)}` : `Empty slot ${slot + 1}`}
                      onClick={containerItems[slot] ? () => removeContainerItem(slot) : undefined}
                    />
                  ))}
                </div>
              </>
            )}
            <div className="vanilla-section-label">Player Inventory</div>
            <EmptyPlayerInventory />
          </div>

          <aside className="vanilla-item-picker" aria-label="Vanilla item picker">
            <div className="vanilla-section-label">Creative Items</div>
            <input
              type="search"
              aria-label="Search vanilla items"
              placeholder="Search items"
              value={itemSearch}
              onChange={(event) => setItemSearch(event.target.value)}
            />
            <div className="vanilla-item-controls">
              <input
                aria-label={isFurnace ? 'Machine item ID' : 'Container item ID'}
                value={itemIdInput}
                onChange={(event) => setItemIdInput(event.target.value)}
                placeholder="minecraft:oak_log"
              />
              <input
                aria-label={isFurnace ? 'Machine item quantity' : 'Container item quantity'}
                type="number"
                min={1}
                max={64}
                value={itemQuantityInput}
                onChange={(event) => setItemQuantityInput(Math.max(1, Number(event.target.value) || 1))}
              />
            </div>
            {isFurnace && (
              <div className="vanilla-machine-targets">
                <button type="button" className={machineTarget === 'input' ? 'active' : ''} onClick={() => setMachineTarget('input')}>Target: Input</button>
                <button type="button" className={machineTarget === 'fuel' ? 'active' : ''} onClick={() => setMachineTarget('fuel')}>Target: Fuel</button>
                <button type="button" onClick={() => addMachineItem('input')}>Add Input</button>
                <button type="button" onClick={() => addMachineItem('fuel')}>Add Fuel</button>
              </div>
            )}
            {isContainer && (
              <button type="button" className="vanilla-add-button" onClick={() => addContainerItem()}>
                Insert
              </button>
            )}
            <div className="vanilla-item-list">
              {visibleItems.map((item) => (
                <button type="button" key={item.id} className="vanilla-item-choice" onClick={() => chooseItem(item.id)} title={item.id}>
                  <span>{item.name}</span>
                  <small>{item.id}</small>
                </button>
              ))}
            </div>
          </aside>
        </div>
        <footer className="vanilla-inventory-footer">
          <span>{isFurnace ? 'Input, fuel and output are runtime slots.' : `${containerCapacity} storage slots.`}</span>
          <span>Click a filled slot to extract · click an item to insert</span>
        </footer>
      </section>
    </div>
  )
}
