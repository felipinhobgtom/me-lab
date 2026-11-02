# Architecture Decision Records (ADR Summary)

This document summarizes the key architectural decisions shaping **ME Lab** — the Applied Energistics 2 (AE2) Network Designer & Simulator.

| ID | Decision | Status |
| --- | --- | --- |
| [ADR-001](#adr-001-pure-go-sqlite-persistence-moderncorgsqlite--goose) | Pure-Go SQLite Persistence (`modernc.org/sqlite` + Goose) | Accepted |
| [ADR-002](#adr-002-isolated-deterministic-simulation-core) | Isolated Deterministic Simulation Core (`internal/simulation`) | Accepted |
| [ADR-003](#adr-003-three-stage-ae2-channel-routing-model) | Three-Stage AE2 Channel Routing Model | Accepted |
| [ADR-004](#adr-004-reproducible-dev-environments-via-nixos-flakes--devenv) | Reproducible Dev Environments via NixOS Flakes + `devenv` | Accepted |
| [ADR-005](#adr-005-procedural-react-three-fiber-3d-editor--2d-y-slice-fallback) | Procedural React Three Fiber 3D Editor + 2D Y-Slice Fallback | Accepted |
| [ADR-006](#adr-006-azure-linux-vm--managed-disk--passwordless-blob-backups) | Azure Linux VM + Managed Disk + Passwordless Blob Backups | Accepted |

---

## ADR-001: Pure-Go SQLite Persistence (`modernc.org/sqlite` + Goose)

### Context
ME Lab persists user accounts, versioned AE2 compatibility profiles, block catalogs, 3D project placements, encoded patterns, seeded inventories, and deterministic tick-by-tick simulation traces. Traditional `mattn/go-sqlite3` requires `cgo` and a C toolchain, complicating cross-compilation, scratch/Alpine container builds, and reproducible Nix builds, while managed cloud RDBMS instances (Azure Database for PostgreSQL) consume a disproportionate share of a student cloud budget.

### Decision
Use **`modernc.org/sqlite`** with `CGO_ENABLED=0` and embedded SQL migrations managed by **`pressly/goose/v3`** (`migrations/00001_initial_schema.sql` via `migrations/embed.go`). Configure every connection pool (`internal/repository/sqlite/db.go`) with:
- `PRAGMA journal_mode=WAL` for non-blocking concurrent reads and atomic online `.backup` snapshots.
- `PRAGMA foreign_keys=ON` for cascading project/simulation cleanup and referential integrity.
- `PRAGMA busy_timeout=5000` and `SetMaxOpenConns(1)` to serialize writers cleanly without `SQLITE_BUSY` contention.

### Consequences
- **Positive**: Zero C compiler dependency; fast static `GOOS=linux` builds; zero external database network latency; single-file operational simplicity with online WAL-safe backups.
- **Trade-off**: Write throughput is vertically bounded to a single VM node (cannot horizontally scale active writers across multiple VMs without switching to a distributed database or LiteFS). For interactive CAD/simulation workloads, a single `Standard_B2s` node easily handles hundreds of concurrent projects.

---

## ADR-002: Isolated Deterministic Simulation Core

### Context
Players rely on ME Lab to diagnose subtle channel overloads, controller conflicts, storage partition overflows, and recursive autocrafting stalls before building expensive setups in Minecraft. If simulation outcomes vary across runs due to Go map iteration order, wall-clock timestamps, or database I/O side effects, debugging becomes untrustworthy.

### Decision
Isolate all AE2 mechanics inside **`internal/simulation/...`** (`world`, `network`, `channels`, `energy`, `storage`, `crafting`, `diagnostics`) as a pure Go domain engine with **zero dependencies on `net/http`, `database/sql`, or wall-clock time**:
- Every 3D traversal sorts coordinates deterministically via `domain.CompareCoords` (`Y`, then `Z`, then `X`).
- Simulation runs record an explicit `int64` `seed` and advance exclusively via discrete integer ticks (`current_tick`).
- Emitted `SimulationEvent` records receive strictly monotonic `sequence_number` values per simulation run.

### Consequences
- **Positive**: Given the same project snapshot, compatibility profile, and seed, tick-stepping produces byte-identical events, overlays, and diagnostics in unit tests, CI, and production.
- **Trade-off**: Application services must translate between persisted JSON snapshots (`simulation_runs.snapshot_json` / `state_json`) and in-memory engine structures on step boundaries.

---

## ADR-003: Three-Stage AE2 Channel Routing Model

### Context
In Applied Energistics 2, channel allocation on controller networks is hierarchical rather than an arbitrary max-flow graph: devices prefer shortest paths through local normal/smart cables (`8` channels max) into high-capacity dense cable trunks (`32` channels max) terminating on exposed ME Controller faces (`32` channels per face), while controllerless ("ad-hoc") networks share up to `8` channels network-wide.

### Decision
Model channel allocation in `internal/simulation/channels` using AE2's **3-stage routing hierarchy**:
1. **Ad-hoc check**: If a connected component has `0` controllers, allow up to `AdHocChannelLimit` (`8`) total channel-consuming devices across the component; exceeding `8` takes the entire ad-hoc network offline (`AD_HOC_TOO_MANY_CHANNELS`). If `InfiniteChannels` is enabled on the profile, bypass capacity limits while still computing routes.
2. **Stage 1 — Local Non-Dense Routing**: Route each channel-consuming device (and each valid Crafting CPU multiblock as `1` combined channel consumer) along shortest deterministic paths of normal/smart cables (`cap = 8`) to either an adjacent controller face or a dense cable trunk entry node.
3. **Stage 2 — Dense Trunk Routing**: Route trunk flows across dense/dense-smart cables (`cap = 32`) to exposed ME Controller block faces.
4. **Stage 3 — Controller Face Allocation**: Enforce `ControllerFaceChannels` (`32`) per exposed face of valid ME Controller multiblocks, respecting cable color isolation (`fluix` universal; dyed cables match same color only), `anchored_faces` disconnects, and directional `ae2:pattern_provider` push-face disconnects.

### Consequences
- **Positive**: Faithfully reproduces real in-game AE2 channel bottlenecks, smart-cable channel displays, dense trunk utilization, and ad-hoc network behavior; yields exact per-device `DeviceRouteOverlay` paths for 3D inspection.
- **Trade-off**: Slightly more code than naive BFS counting, mitigated by deterministic coordinate ordering and dedicated unit tests for edge capacities.

---

## ADR-004: Reproducible Dev Environments via NixOS Flakes + `devenv`

### Context
Contributors work across NixOS, other Linux distributions, macOS, and GitHub Actions runners, requiring consistent versions of Go (`1.26`), Node.js (`22`), SQLite (`sqlite3` CLI with `.backup`), Goose, `staticcheck`, `jq`, and Azure CLI (`az`).

### Decision
Provide first-class **`flake.nix`** and **`devenv.nix`** configurations alongside standard **`Makefile`** targets and **`compose.yaml`**:
- `nix develop` or `devenv shell` provisions exact pinned toolchains with `CGO_ENABLED=0` and local SQLite state paths preconfigured.
- Non-Nix contributors and CI workflows can equivalently use standard Go/Node setups or `docker compose up --build`.

### Consequences
- **Positive**: Zero "works on my machine" drift on NixOS/macOS while keeping onboarding friction-free for contributors using Docker or native package managers.
- **Trade-off**: Tooling versions in `flake.nix`, `go.mod`, `Dockerfile`, and `.github/workflows/ci.yml` must be kept aligned during major upgrades.

---

## ADR-005: Procedural React Three Fiber 3D Editor + 2D Y-Slice Fallback

### Context
AE2 networks are inherently 3D structures (7x7x7 controllers, vertical dense cable risers, rectangular-prism Crafting CPUs, directional Pattern Providers surrounding Molecular Assemblers). However:
1. Redistributing Mojang or AE2 modpack PNG textures creates copyright/licensing risk.
2. Inspecting interior layers of dense 3D multiblocks — or working on low-power integrated GPUs / headless browsers — can be awkward in a pure perspective camera.

### Decision
Build the frontend (`web/`) with **React 18 + TypeScript + Three.js (`@react-three/fiber`)** using **100% procedural geometry and color palettes** defined in catalog `BlockVisual` metadata (`cube`, `cable`, `dense_cable`, `panel`, `frame`, `core`), paired with a **synchronized 2D Y-layer slice editor**:
- Both views read and mutate the same editor state, selection, cable channel overlays, and diagnostic highlights.
- Users can toggle between 3D orbit/voxel placement and layer-by-layer 2D blueprints at any time (and export a matching Y-layer Markdown Build Sheet via `internal/export`).

### Consequences
- **Positive**: Clean MIT/open-source licensing with zero proprietary game assets; accessible editing on both desktop GPUs and lightweight laptops; effortless layer-by-layer interior editing.
- **Trade-off**: Blocks use stylized geometric shapes and color coding rather than pixel-art Minecraft textures.

---

## ADR-006: Azure Linux VM + Managed Disk + Passwordless Blob Backups

### Context
Production hosting must support HTTPS, persistent SQLite WAL storage across container redeployments, automated off-host backups with verified disaster recovery, and strict cost control suitable for an Azure for Students subscription.

### Decision
Deploy on a single **Azure Ubuntu 24.04 LTS VM (`Standard_B2s`)** provisioned declaratively via **`deployments/azure/main.bicep`**:
- **Ingress & TLS**: Azure NSG restricts inbound ports to `80/tcp`, `443/tcp`, and `22/tcp` (`sshAllowedCidr` only). Host Nginx terminates Let's Encrypt TLS 1.2/1.3 and proxies to loopback-bound Docker Compose services (`127.0.0.1:8080` Go API, `127.0.0.1:8081` Web SPA).
- **Persistent Storage**: A dedicated `32 GiB StandardSSD_LRS` Azure Managed Data Disk is mounted at `/mnt/melab-data` (`caching: None` for WAL `fsync` safety), isolating `/mnt/melab-data/me-lab.db` from OS disk replacements and container rebuilds.
- **Backups & Passwordless Auth**: A daily systemd timer runs `scripts/backup-sqlite.sh`, executing an atomic online `sqlite3 "$DATABASE_URL" ".backup '$TMP_BACKUP'"`, `gzip` compression, and SHA-256 checksumming, then uploading to Azure Blob Storage (`sqlite-backups`) using the VM's **System-Assigned Managed Identity** (`Storage Blob Data Contributor` RBAC — zero static storage keys). GitHub Actions deploys releases via **Azure OIDC federated credentials** and `az vm run-command invoke`.

### Consequences
- **Positive**: No long-lived cloud credentials stored on the VM or in GitHub; durable storage decoupled from containers; predictable low monthly cost that drops compute charges to `$0` whenever deallocated (`az vm deallocate`).
- **Trade-off**: Brief (~2–5 second) API restart window during `docker compose up -d` releases on a single VM.
