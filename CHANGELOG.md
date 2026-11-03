# Changelog

All notable changes to **ME Lab — AE2 Network Designer & Simulator** are documented in this file.

## [Unreleased]

### Added
- ME Drive storage-cell workflow: empty drives now expose the official ten-slot inventory, cells can be added from the context menu or ME Drive inspector, double-click opens the same configuration, configured cell item stacks remain real simulation storage, and non-empty cells cannot be silently discarded.
- Backend validation now enforces AE2 storage-cell tiers, ten-cell ME Drive limit, one-cell ME Chest limit, positive quantities, and the 63 distinct-type cell limit. Sources: [ME Drive](https://guide.appliedenergistics.org/1.21.1/items-blocks-machines/drive/), [Storage Cells](https://guide.appliedenergistics.org/1.21.1/items-blocks-machines/storage_cells/), [Bytes and Types](https://guide.appliedenergistics.org/1.21.1/ae2-mechanics/bytes-and-types/).
- Powered AE2 automation: Energy Cells/acceptors now gate device service, Import/Export Bus transfers, filters, inverter and acceleration settings, bus telemetry, and deterministic `bus_transfer` events.
- Continuous AE2 runtime: saved layouts now recreate the live network automatically, tick continuously at a Minecraft-equivalent 20 TPS cadence while open, and expose read-only diagnostics/telemetry without manual Validate, Simulate, Step, Pause, Resume, or Reset controls. Sources: [Channels](https://guide.appliedenergistics.org/1.21.1/ae2-mechanics/channels), [Energy](https://guide.appliedenergistics.org/1.21.1/ae2-mechanics/energy), [Autocrafting](https://guide.appliedenergistics.org/1.21.1/ae2-mechanics/autocrafting).
- Minecraft-style E creative inventory: the block picker now uses pixel-art inventory slots, keyboard-friendly tab navigation, search, and explicit `Minecraft Vanilla` / `Applied Energistics 2` sections with AE2 category filters. Source: [Minecraft Creative inventory](https://minecraft.wiki/w/Creative_inventory).
- API overlay for per-device online status, automation buses, and external inventories.
- Minecraft chest renderer now separates body, lid, side, top, and bottom pixel textures instead of reusing the wooden-plank texture across all faces.


## [1.0.0] - 2026-11-02

### Added
- **Milestone 0 — Infrastructure & NixOS Tooling:** Monorepo layout, Go 1.26 module, React 18 + TypeScript + Vite + React Three Fiber setup, NixOS `flake.nix` + `devenv.nix` + `.envrc`, non-root multi-stage `Dockerfile` and `web/Dockerfile`, `compose.yaml`, and GitHub Actions CI/CD workflows (`.github/workflows/ci.yml`, `.github/workflows/deploy-azure.yml`).
- **Milestone 1 — API, Auth, SQLite & Projects:** Embedded Goose SQLite migrations across all 10 relational tables (`WAL` mode, `foreign_keys=ON`, `busy_timeout=5000`), Argon2id password hashing, HS256 JWT auth, owner-scoped Project CRUD, `/openapi.json` (OpenAPI 3.1.0), and `/docs` Swagger UI explorer.
- **Milestone 2 — 3D/2D CAD Editor & AE2 1.21.1 Catalog:** Embedded compatibility profiles (`ae2-1.21.1-default`, `ae2-1.21.1-x2`, `ae2-1.21.1-infinite`) and 29 block definitions; procedural 3D R3F workspace + synchronized 2D Y-slice editor; searchable palette; context-sensitive block inspector; Undo/Redo, Copy/Paste/Duplicate, rotation, and transactional batch save/load.
- **Milestone 3 — Network & Channel Simulation:** Pure-Go 3D world graph, cable color & anchor isolation, directional Pattern Provider face isolation, ME Controller multiblock validator (`7x7x7`, single cluster, `<= 1` axis rule), Crafting CPU solid rectangular-prism validator, ad-hoc 8-channel model, and 3-tier shortest-path controller channel routing with 3D/2D visual overlays and fault highlighting.
- **Milestone 4 — Storage, Energy & Autocrafting:** Exact AE2 cell byte/type math (`1k`–`256k`, 63 types, priority, whitelist/Inverter partitions), network AE buffers & FE conversion, and deterministic tick-stepped autocrafting (`job_created` through `job_completed`, Co-Processor parallel batch dispatch, Blocking & Lock modes, pause/resume/step/reset).
- **Milestone 5 & 6 — Blueprint Export, Visual Documentation & Azure Ops:** Deterministic JSON blueprint and Markdown Build Sheet + BOM exporters, 6 Mermaid architecture/ER/sequence/topology/deployment diagrams, 8 live application screenshots including the immediate empty sandbox and textured guide showcase, Azure Bicep infrastructure (`deployments/azure/main.bicep`), and tested online WAL-safe SQLite `.backup` + restore scripts (`scripts/`).
