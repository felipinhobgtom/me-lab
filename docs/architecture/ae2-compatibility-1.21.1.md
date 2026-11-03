# AE2 1.21.1 Compatibility & Simulation Fidelity Specification

ME Lab binds every project to an explicit **Compatibility Profile** (`compatibility_profiles` table). The primary baseline profile targets **Minecraft `1.21.1`** with **Applied Energistics 2 `19.x`** (`ae2-1.21.1-default`, plus `x2` and `infinite` channel-mode variants).

Every catalog block carries an explicit `fidelity` classification (`internal/domain/models.go`):
- **`exact`**: Simulated according to AE2 1.21.1 rules, capacities, byte/type formulas, and structural constraints.
- **`approximated`**: Simplified deterministic model of external/modpack machine interactions (e.g., `melab:external_machine` representing a modded furnace/crusher driven by processing patterns).
- **`unsupported`**: Cataloged so players can document full layouts and Bill of Materials, but placing one in a simulated project emits diagnostic `UNSUPPORTED_MECHANIC` rather than silently faking behavior.

---

## 1. Block & Mechanic Fidelity Matrix

| Block / Mechanic ID | Display Name | Category | Channels (Use / Transmit) | Fidelity | Official AE2 1.21.1 Guide Reference |
| --- | --- | --- | --- | --- | --- |
| `ae2:fluix_cable` | ME Fluix Glass Cable | `cables` | `0` / `8` | `exact` | [Glass Cable](https://guide.appliedenergistics.org/1.21.1/items-blocks-machines/cables#glass-cable) |
| `ae2:smart_cable` | ME Fluix Smart Cable | `cables` | `0` / `8` (smart display) | `exact` | [Smart Cable](https://guide.appliedenergistics.org/1.21.1/items-blocks-machines/cables#smart-cable) |
| `ae2:dense_cable` | ME Dense Covered Cable | `cables` | `0` / `32` (dense) | `exact` | [Dense Cable](https://guide.appliedenergistics.org/1.21.1/items-blocks-machines/cables#dense-cable) |
| `ae2:dense_smart_cable` | ME Fluix Dense Smart Cable | `cables` | `0` / `32` (dense + smart) | `exact` | [Dense Smart Cable](https://guide.appliedenergistics.org/1.21.1/items-blocks-machines/cables#dense-cable) |
| `ae2:cable_anchor` | Cable Anchor | `cables` | `0` / `0` (face severing) | `exact` | [Cable Anchor](https://guide.appliedenergistics.org/1.21.1/items-blocks-machines/cable_anchor) |
| `ae2:controller` | ME Controller | `network` | `0` / `32` per exposed face | `exact` | [ME Controller](https://guide.appliedenergistics.org/1.21.1/items-blocks-machines/controller) |
| `ae2:energy_acceptor` | Energy Acceptor | `energy` | `0` / `8` (`2 FE = 1 AE`) | `exact` | [Energy Acceptor](https://guide.appliedenergistics.org/1.21.1/items-blocks-machines/energy_acceptor) |
| `ae2:energy_cell` | Energy Cell | `energy` | `0` / `8` (`200,000 AE`) | `exact` | [Energy Cells](https://guide.appliedenergistics.org/1.21.1/items-blocks-machines/energy_cells) |
| `ae2:dense_energy_cell` | Dense Energy Cell | `energy` | `0` / `8` (`1,600,000 AE`) | `exact` | [Energy Cells](https://guide.appliedenergistics.org/1.21.1/items-blocks-machines/energy_cells) |
| `ae2:creative_energy_cell` | Creative Energy Cell | `energy` | `0` / `8` ($\infty$ AE) | `exact` | [Energy Cells](https://guide.appliedenergistics.org/1.21.1/items-blocks-machines/energy_cells) |
| `ae2:drive` | ME Drive | `storage` | `1` / `8` (up to 10 cells) | `exact` | [ME Drive](https://guide.appliedenergistics.org/1.21.1/items-blocks-machines/drive) |
| `ae2:terminal` | ME Terminal | `storage` | `1` / `8` | `exact` | [Terminals](https://guide.appliedenergistics.org/1.21.1/items-blocks-machines/terminals) |
| `ae2:crafting_terminal` | ME Crafting Terminal | `storage` | `1` / `8` | `exact` | [Crafting Terminal](https://guide.appliedenergistics.org/1.21.1/items-blocks-machines/terminals#crafting-terminal) |
| `ae2:pattern_provider` | ME Pattern Provider | `crafting` | `1` / `8` (except directional push face) | `exact` | [Pattern Provider](https://guide.appliedenergistics.org/1.21.1/items-blocks-machines/pattern_provider) |
| `ae2:molecular_assembler` | Molecular Assembler | `crafting` | `0` / `8` | `exact` | [Molecular Assembler](https://guide.appliedenergistics.org/1.21.1/items-blocks-machines/molecular_assembler) |
| `ae2:crafting_unit` | Crafting Unit | `crafting` | `1` per CPU multiblock | `exact` | [Crafting CPU](https://guide.appliedenergistics.org/1.21.1/items-blocks-machines/crafting_cpu_multiblock) |
| `ae2:crafting_storage_1k` | 1k Crafting Storage | `crafting` | `1` per CPU (`1,024 B`) | `exact` | [Crafting CPU](https://guide.appliedenergistics.org/1.21.1/items-blocks-machines/crafting_cpu_multiblock) |
| `ae2:crafting_storage_4k` | 4k Crafting Storage | `crafting` | `1` per CPU (`4,096 B`) | `exact` | [Crafting CPU](https://guide.appliedenergistics.org/1.21.1/items-blocks-machines/crafting_cpu_multiblock) |
| `ae2:crafting_storage_16k` | 16k Crafting Storage | `crafting` | `1` per CPU (`16,384 B`) | `exact` | [Crafting CPU](https://guide.appliedenergistics.org/1.21.1/items-blocks-machines/crafting_cpu_multiblock) |
| `ae2:crafting_storage_64k` | 64k Crafting Storage | `crafting` | `1` per CPU (`65,536 B`) | `exact` | [Crafting CPU](https://guide.appliedenergistics.org/1.21.1/items-blocks-machines/crafting_cpu_multiblock) |
| `ae2:crafting_storage_256k` | 256k Crafting Storage | `crafting` | `1` per CPU (`262,144 B`) | `exact` | [Crafting CPU](https://guide.appliedenergistics.org/1.21.1/items-blocks-machines/crafting_cpu_multiblock) |
| `ae2:crafting_co_processor` | Crafting Co-Processing Unit | `crafting` | `1` per CPU (`+1` batch/t) | `exact` | [Crafting CPU](https://guide.appliedenergistics.org/1.21.1/items-blocks-machines/crafting_cpu_multiblock) |
| `ae2:crafting_monitor` | Crafting Monitor | `crafting` | `1` per CPU (job display) | `exact` | [Crafting CPU](https://guide.appliedenergistics.org/1.21.1/items-blocks-machines/crafting_cpu_multiblock) |
| `melab:external_machine` | External Processing Machine | `crafting` | `0` / `0` (configurable ticks) | `approximated` | [Processing Patterns](https://guide.appliedenergistics.org/1.21.1/ae2-mechanics/autocrafting#processing-patterns) |
| `ae2:p2p_tunnel` | ME P2P Tunnel | `network` | N/A (emits `UNSUPPORTED_MECHANIC`) | `unsupported` | [P2P Tunnels](https://guide.appliedenergistics.org/1.21.1/items-blocks-machines/p2p_tunnels) |
| `ae2:storage_bus` | ME Storage Bus | `storage` | `1` / cable-dependent | `approximated` | [Storage Bus](https://guide.appliedenergistics.org/1.21.1/items-blocks-machines/storage_bus) |
| `ae2:import_bus` | ME Import Bus | `storage` | `1` / cable-dependent | `approximated` | [Import Bus](https://guide.appliedenergistics.org/1.21.1/items-blocks-machines/import_bus) |
| `ae2:export_bus` | ME Export Bus | `storage` | `1` / cable-dependent | `approximated` | [Export Bus](https://guide.appliedenergistics.org/1.21.1/items-blocks-machines/export_bus) |
| `ae2:quantum_ring` | ME Quantum Network Bridge | `network` | N/A (emits `UNSUPPORTED_MECHANIC`) | `unsupported` | [Quantum Bridge](https://guide.appliedenergistics.org/1.21.1/items-blocks-machines/quantum_bridge) |

### 1.1 Vanilla containers and Minecraft-style inventory screens

Vanilla containers are not rendered as generic AE2 Block Configuration. Double-clicking a supported chest, barrel, hopper, furnace, or blast furnace opens a dedicated Minecraft-style inventory screen with the block slots, the player's 27-slot inventory and 9-slot hotbar layout, runtime furnace state, and a searchable vanilla item picker. The item picker writes namespaced `minecraft:*` stacks into the authoritative block configuration; runtime overlays remain read-only observations of processing and extraction.

- Chest, barrel and hopper use their real 27/27/5 storage capacities.
- Furnace and blast furnace expose only input, fuel and output. Their processing and output remain continuous runtime behavior.
- Clicking a filled block slot extracts that configured stack. Clicking an item in the vanilla picker inserts it into the first compatible slot; furnace input and fuel can also be targeted explicitly.
- The generic Block Configuration panel remains available for placement, orientation and AE2-specific settings, but it no longer contains the vanilla container inventory cards.

The block catalog includes the renderable vanilla block-state IDs from the official Minecraft Java 1.21.1 client asset registry (1,057 generated vanilla block entries in addition to the explicitly modeled vanilla machines) and the frontend item picker includes the 1,754 item-model IDs from that same version-matched registry. Generic catalog entries are labeled `approximated` and use original procedural visuals; they do not claim an implementation of every block's machine behavior.

Version-matched sources:
- [Minecraft Java Edition 1.21.1](https://www.minecraft.net/en-us/article/minecraft-java-edition-1-21-1)
- [Minecraft 1.21.1 version page](https://minecraft.wiki/w/Java_Edition_1.21.1)
- [Chest container slots](https://minecraft.wiki/w/Chest#Container)
- [Furnace processing slots](https://minecraft.wiki/w/Furnace#Processing_items)
- [Creative inventory (1.21-era revision)](https://minecraft.wiki/w/Creative_inventory?oldid=2654837)

---

### 1.2 Vanilla container and furnace runtime

The editor opens supported Minecraft inventories through the same right-click/double-click inspector used for AE2 devices. Inventory mutations are persisted into the block configuration and are consumed by the live runtime; there is no simulation or step button.

Version-matched sources consulted for this contract:
- [Chest — Container](https://minecraft.wiki/w/Chest#Container)
- [Furnace — Processing items](https://minecraft.wiki/w/Furnace#Processing_items)
- [Blast Furnace — Smelting](https://minecraft.wiki/w/Blast_Furnace#Smelting)
- [Smelting — Fuel](https://minecraft.wiki/w/Smelting#Fuel)

---

## 2. Exact Subsystem Rules & Mathematical Formulas

### 2.1 Adjacency, Cable Colors & Anchors
Ref: [Cables](https://guide.appliedenergistics.org/1.21.1/items-blocks-machines/cables) & [Subnetworks](https://guide.appliedenergistics.org/1.21.1/ae2-mechanics/subnetworks)

Two orthogonally adjacent blocks $A$ at $\vec{p}$ and $B$ at $\vec{p} + \hat{d}$ connect across face $d \in \{\text{north}, \text{south}, \text{east}, \text{west}, \text{up}, \text{down}\}$ if and only if:
1. Neither $A$ has $d \in \text{anchored\_faces}(A)$ nor $B$ has $\text{opposite}(d) \in \text{anchored\_faces}(B)$.
2. Neither block disconnects face $d$ via directional orientation: when `ae2:pattern_provider` has `variant == "directional"`, its push face (`Facing`) pushes crafting/processing ingredients to an adjacent machine and **does not** form an ME network edge on that face.
3. If both $A$ and $B$ are colorable cables, their colors $c_A, c_B$ satisfy:
   $$\text{compatible}(c_A, c_B) \iff (c_A = \text{"fluix"}) \lor (c_B = \text{"fluix"}) \lor (c_A = c_B)$$

### 2.2 ME Controller Multiblock Constraints
Ref: [ME Controller](https://guide.appliedenergistics.org/1.21.1/items-blocks-machines/controller)

A connected network component is valid with respect to controllers when:
1. **Uniqueness**: At most **1** face-connected cluster of `ae2:controller` blocks exists on the network. Two disconnected controller clusters on the same cable network trigger `CONTROLLER_CONFLICT` and force the network offline.
2. **7x7x7 Bounding Box**: For controller coordinates $C$, along every axis $u \in \{X, Y, Z\}$:
   $$\max_{p \in C}(p_u) - \min_{p \in C}(p_u) + 1 \le 7$$
3. **Single Opposite-Neighbor Axis Rule (No Crosses)**: For each controller block $p \in C$, count axes $u \in \{X, Y, Z\}$ where **both** $p + \hat{u} \in C$ and $p - \hat{u} \in C$:
   $$\text{oppositeAxes}(p) = \sum_{u \in \{X,Y,Z\}} \mathbb{I}\big[(p + \hat{u} \in C) \land (p - \hat{u} \in C)\big] \le 1$$
   Violating `7x7x7` or `oppositeAxes(p) > 1` marks the controller invalid and takes the network offline.

### 2.3 Channel Capacities & Modes
Ref: [Channels](https://guide.appliedenergistics.org/1.21.1/ae2-mechanics/channels)

| Channel Mode | Ad-Hoc Limit (No Controller) | Normal / Smart Cable & Device Passthrough | Dense / Dense Smart Cable | Controller Exposed Face |
| --- | ---: | ---: | ---: | ---: |
| `default` | `8` | `8` | `32` | `32` |
| `x2` | `16` | `16` | `64` | `64` |
| `infinite` | $\infty$ | $\infty$ | $\infty$ | $\infty$ |

- Full-block ME devices (`ae2:drive`, normal `ae2:pattern_provider`, `ae2:energy_acceptor`, `ae2:energy_cell`, `ae2:molecular_assembler`) transmit up to normal cable capacity (`8` in `default`), **never** dense `32`-channel capacity — placing a full-block device inline on a dense trunk bottlenecks downstream traffic to `8` channels, matching in-game AE2 behavior.
- Each valid Crafting CPU multiblock consumes **1 channel total** regardless of block count.

### 2.4 Energy Conversion, Buffers & Tick Balance
Ref: [ME Network Energy](https://guide.appliedenergistics.org/1.21.1/ae2-mechanics/energy)

- **Forge Energy Conversion**: $2\text{ FE} = 1\text{ AE}$:
  $$E_{\text{gen}}(\text{AE/t}) = \sum_{i \in \text{Acceptors}} \frac{\text{fe\_input\_per\_tick}_i}{2} + \sum_{j \in \text{Creative}} \infty$$
- **Network Energy Capacity**:
  $$E_{\text{cap}}(\text{AE}) = 8{,}000 \cdot N_{\text{controller}} + 200{,}000 \cdot N_{\text{energy\_cell}} + 1{,}600{,}000 \cdot N_{\text{dense\_energy\_cell}} + E_{\text{nodes}}$$
- **Per-Tick Update**:
  $$E_{t+1} = \min\!\Big(E_{\text{cap}},\; E_t + E_{\text{gen}} - \big(E_{\text{passive}} + E_{\text{active},t}\big)\Big)$$
  where each `ae2:controller` drains $6\text{ AE/t}$ passively plus per-device idle and active crafting costs (`MolecularAssemblerActiveE` per active craft tick). If $E_t + E_{\text{gen}} < E_{\text{passive}}$, the network enters **energy brownout** (`INSUFFICIENT_ENERGY`), halting channel service and crafting progression until power is restored.

### 2.5 ME Item Storage Cell Byte & Type Math
Ref: [Bytes and Types](https://guide.appliedenergistics.org/1.21.1/ae2-mechanics/bytes-and-types)

Every standard ME Item Storage Cell holds at most **$T_{\max} = 63$ distinct item types** and scales type overhead linearly with total byte capacity (`8` items per data byte):

| Cell Tier | Total Capacity ($B_{\text{total}}$) | Overhead per Distinct Type ($B_{\text{type}}$) | Max Items (1 Type) | Max Items (63 Types) |
| --- | ---: | ---: | ---: | ---: |
| `1k` | `1,024 B` | `8 B` | `8,128` | `4,160` |
| `4k` | `4,096 B` | `32 B` | `32,512` | `16,640` |
| `16k` | `16,384 B` | `128 B` | `130,048` | `66,560` |
| `64k` | `65,536 B` | `512 B` | `520,192` | `266,240` |
| `256k` | `262,144 B` | `2,048 B` | `2,080,768` | `1,064,960` |

For a cell storing $k \le 63$ distinct item types with quantities $q_1, \dots, q_k$:
$$B_{\text{used}} = k \cdot B_{\text{type}} + \sum_{i=1}^{k} \left\lceil \frac{q_i}{8} \right\rceil \quad \text{(or aggregate item byte ceil per profile config)}, \qquad B_{\text{used}} \le B_{\text{total}}$$

**Insertion Order & Partitioning**:
1. Online `ae2:drive` blocks are ordered by descending `priority`, then deterministic coordinate `(Y, Z, X)` and cell slot index.
2. Cells with a non-empty `partition` list act as a **whitelist** when `inverter == false` ($x \in \text{partition}$) or a **blacklist** when `inverter == true` ($x \notin \text{partition}$).


**ME Drive editor contract (Minecraft 1.21.1 / AE2 19.x):**
- An ME Drive exposes exactly ten slots and starts with no implicit storage cell. A cell is added explicitly through the inspector or the block context menu; double-clicking the drive opens the same configuration surface.
- Each configured slot is a real item storage cell in the simulation snapshot. Its tier, partition, inverter state, and initial item stacks are persisted in `project_blocks.configuration_json`, then loaded into mutable runtime cell state for insertion/extraction and automation.
- The editor refuses to remove a configured cell while it contains items instead of silently discarding its contents. Runtime storage follows the documented priority, partition, 63-type, byte-capacity, power, and channel rules above.

Consulted version-matched sources for this contract:
- [The ME Drive](https://guide.appliedenergistics.org/1.21.1/items-blocks-machines/drive/)
- [Storage Cells](https://guide.appliedenergistics.org/1.21.1/items-blocks-machines/storage_cells/)
- [Bytes and Types](https://guide.appliedenergistics.org/1.21.1/ae2-mechanics/bytes-and-types/)
### 2.6 Crafting CPU Multiblocks & Recursive Autocrafting
Ref: [Autocrafting](https://guide.appliedenergistics.org/1.21.1/ae2-mechanics/autocrafting) & [Crafting CPU](https://guide.appliedenergistics.org/1.21.1/items-blocks-machines/crafting_cpu_multiblock)

1. **Rectangular Prism Validation**: A face-connected cluster $M$ of Crafting CPU blocks (`ae2:crafting_unit`, `ae2:crafting_storage_*`, `ae2:crafting_co_processor`, `ae2:crafting_monitor`) is valid if and only if:
   $$|M| = (\Delta X) \cdot (\Delta Y) \cdot (\Delta Z) \quad \land \quad \sum_{b \in M} \text{CraftingStorageBytes}(b) > 0$$
   Incomplete/L-shaped clusters emit `INVALID_CRAFTING_CPU`.
2. **Parallel Dispatch Rate**: An online CPU with $C_{\text{coproc}}$ co-processing units dispatches up to:
   $$D_{\text{tick}} = 1 + C_{\text{coproc}}$$
   ready pattern batches per simulation tick across online `ae2:pattern_provider` blocks (respecting provider `priority`, `blocking_mode`, and `lock_mode == "until_result_returned"`).
3. **Execution**:
   - `crafting` patterns push to an adjacent `ae2:molecular_assembler` (`exact`, completes in `MolecularAssemblerTicks` ticks and auto-returns output to the adjacent Pattern Provider).
   - `processing` patterns push to an adjacent `melab:external_machine` (`approximated`, completes in configured `process_ticks` [default `8`] and returns output when `auto_return == true`).

### 2.7 Powered Bus Automation
Ref: [Import Bus](https://guide.appliedenergistics.org/1.21.1/items-blocks-machines/import_bus), [Export Bus](https://guide.appliedenergistics.org/1.21.1/items-blocks-machines/export_bus), [Pattern Provider](https://guide.appliedenergistics.org/1.21.1/items-blocks-machines/pattern_provider)

The simulator treats every channel-using AE device as requiring **both** a granted channel and an online powered network. A brownout therefore stops storage access, crafting progression, and bus transfers; it does not merely paint the cable red.

`Import Bus` and `Export Bus` can be mounted as cable subparts or represented as standalone devices. Each operation is deterministic:

1. The target inventory is the block immediately in the bus-facing direction.
2. Filters are item IDs. Import has an empty-filter “import all” default; Export requires at least one filter.
3. An inverter changes the filter into a blacklist.
4. Acceleration cards move `1`, `8`, `32`, `64`, or `96` items per tick, matching the official speed table.
5. Import uses ME storage insertion rules (drive priority, partition and cell capacity). If storage is full, the item remains in the external inventory.
6. Export extracts only what the target inventory can hold. Transfers emit `bus_transfer` events and are exposed through `overlay.automation_buses` and `overlay.external_inventories`.

The current `Storage Bus` model exposes its target and power/channel status but intentionally does not duplicate external items into ME cells. This avoids double-counting and leaves the external inventory authoritative; explicit mutation is performed by Import/Export Bus.

### 2.8 Pattern Provider and Processing Contract
Ref: [Patterns](https://guide.appliedenergistics.org/1.21.1/items-blocks-machines/patterns) and [Molecular Assembler](https://guide.appliedenergistics.org/1.21.1/items-blocks-machines/molecular_assembler)

- Crafting, smithing and stonecutting-style patterns are dispatched only to Molecular Assemblers.
- Processing patterns are dispatched to adjacent external machines; the simulator uses the configured processing duration and `auto_return` behavior.
- A Pattern Provider never becomes an item buffer for its ingredients: the CPU job pool is the source of the batch, and the result must return to ME storage or a provider return path.
- Provider priority, blocking mode, lock-until-result, CPU capacity, co-processors, recursive-cycle detection, and deterministic coordinate tie-breaks are part of job planning.

### 2.9 Continuous Runtime Contract
Ref: [Channels](https://guide.appliedenergistics.org/1.21.1/ae2-mechanics/channels), [Energy](https://guide.appliedenergistics.org/1.21.1/ae2-mechanics/energy), and [Autocrafting](https://guide.appliedenergistics.org/1.21.1/ae2-mechanics/autocrafting)

- The saved project layout is the source of truth. After each successful automatic persistence, the live ME runtime is recreated from that layout without a user-triggered validation or simulation action.
- While the editor is open, the runtime advances continuously at a Minecraft-equivalent cadence: the browser transport batches five engine ticks every 250 ms, yielding 20 engine ticks per second. This batching is only a transport optimization; all engine rules remain tick-based.
- Each tick observes and applies the documented channel routing, powered-network availability, energy buffer, storage-cell insertion/extraction, bus automation, pattern-provider dispatch, and crafting-CPU progression rules. Diagnostics and telemetry report the current state but never start, pause, reset, or gate execution.
- The UI exposes runtime status, tick, diagnostics, events, jobs, and telemetry as read-only observations. There are no manual Validate, Simulate, Step, Pause, Resume, or Reset controls.

### 2.10 Crafting Terminal and Pattern Encoding Terminal
Refs: [Terminals](https://guide.appliedenergistics.org/1.21.1/items-blocks-machines/terminals), [Patterns](https://guide.appliedenergistics.org/1.21.1/items-blocks-machines/patterns), [Autocrafting](https://guide.appliedenergistics.org/1.21.1/ae2-mechanics/autocrafting), and [Pattern Provider](https://guide.appliedenergistics.org/1.21.1/items-blocks-machines/pattern_provider)

- The **ME Crafting Terminal** is a live storage view with a crafting request surface. It reads the current network item totals and craftable pattern outputs, and a request is queued directly into the already-running Crafting CPU state. There is no “simulate”, “run”, or confirmation step; the request either becomes a runtime job or reports the same live diagnostic that prevented planning.
- Crafting requests use the existing storage, provider, Molecular Assembler, CPU, channel, and energy rules. The terminal does not maintain a second inventory or a client-side recipe result. The runtime remains authoritative and persists the resulting job/event state.
- The **ME Pattern Encoding Terminal** consumes a blank-pattern counter in its terminal configuration and stores encoded crafting or processing patterns there. Encoding records a recipe; it does not execute it. An encoded pattern can then be installed into a selected Pattern Provider, where it becomes available to the live crafting planner.
- Crafting patterns are represented as explicit ingredient/output stacks and processing patterns as explicit input/output stacks. This implementation does not claim smithing or stonecutting terminal parity yet; those pattern types remain outside the supported encoding surface until their recipe-slot and dispatch rules are implemented.
- The official guide defines the terminal roles and pattern semantics, but does not specify a persistence JSON schema or UI error wording. Those are implementation details here and are deliberately kept separate from the runtime rules above.
