#!/usr/bin/env bash
# End-to-end verification test for ME Lab SQLite backup & restore scripts.
# Creates a live WAL-mode SQLite DB using migrations/00001_initial_schema.sql,
# populates sample relational rows (including uncheckpointed WAL frames), runs
# backup-sqlite.sh with LOCAL_BACKUP_DIR, wipes the database, runs
# restore-sqlite.sh, and asserts exact row & content equality across all tables.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
MIGRATION_FILE="${REPO_ROOT}/migrations/00001_initial_schema.sql"
BACKUP_SCRIPT="${REPO_ROOT}/scripts/backup-sqlite.sh"
RESTORE_SCRIPT="${REPO_ROOT}/scripts/restore-sqlite.sh"

WORK_DIR="$(mktemp -d)"
cleanup() {
  rm -rf "$WORK_DIR"
}
trap cleanup EXIT

DB_DIR="${WORK_DIR}/live"
BACKUP_DIR="${WORK_DIR}/backups"
TAMPER_DIR="${WORK_DIR}/tampered"
mkdir -p "$DB_DIR" "$BACKUP_DIR" "$TAMPER_DIR"

export DATABASE_URL="${DB_DIR}/me-lab.db"
export LOCAL_BACKUP_DIR="$BACKUP_DIR"

echo "[test] Creating live WAL-mode SQLite database at ${DATABASE_URL}..."
sqlite3 "$DATABASE_URL" "PRAGMA journal_mode=WAL; PRAGMA foreign_keys=ON;" >/dev/null
sed '/^-- +goose Down/,$d' "$MIGRATION_FILE" | sqlite3 "$DATABASE_URL"

echo "[test] Seeding relational test data across ME Lab schema (with active WAL frames)..."
sqlite3 "$DATABASE_URL" <<'SQL'
PRAGMA foreign_keys = ON;
PRAGMA wal_autocheckpoint = 0;

INSERT INTO users (id, email, password_hash, created_at, updated_at)
VALUES ('u-001', 'engineer@melab.example', '$argon2id$v=19$m=65536,t=3,p=4$c2FsdA$aGFzaA', '2026-11-02T10:00:00Z', '2026-11-02T10:00:00Z');

INSERT INTO compatibility_profiles (id, name, minecraft_version, ae2_version, channel_mode, config_json)
VALUES ('ae2-1.21.1-default', 'AE2 1.21.1 Default', '1.21.1', '19.2.1', 'default', '{"normal_cable_channels":8,"dense_cable_channels":32}');

INSERT INTO block_definitions (id, profile_id, name, category, metadata_json, behavior_key, fidelity_label, doc_reference)
VALUES
  ('ae2:controller', 'ae2-1.21.1-default', 'ME Controller', 'network', '{}', 'controller', 'exact', 'https://guide.appliedenergistics.org/1.21.1/items-blocks-machines/controller'),
  ('ae2:drive', 'ae2-1.21.1-default', 'ME Drive', 'storage', '{}', 'drive', 'exact', 'https://guide.appliedenergistics.org/1.21.1/items-blocks-machines/drive');

INSERT INTO projects (id, owner_user_id, name, description, compatibility_profile_id, settings_json, created_at, updated_at)
VALUES ('p-001', 'u-001', 'Starter Base Network', 'Backup restore verification project', 'ae2-1.21.1-default', '{}', '2026-11-02T10:05:00Z', '2026-11-02T10:05:00Z');

INSERT INTO project_blocks (id, project_id, block_definition_id, x, y, z, facing, configuration_json, label, created_at, updated_at)
VALUES
  ('pb-001', 'p-001', 'ae2:controller', 0, 0, 0, 'north', '{}', 'Core Controller', '2026-11-02T10:06:00Z', '2026-11-02T10:06:00Z'),
  ('pb-002', 'p-001', 'ae2:drive', 1, 0, 0, 'north', '{"priority":10}', 'Primary Drive', '2026-11-02T10:06:05Z', '2026-11-02T10:06:05Z');

INSERT INTO project_patterns (id, project_id, pattern_type, inputs_json, outputs_json, configuration_json, created_at)
VALUES ('pat-001', 'p-001', 'crafting', '[{"item_id":"minecraft:oak_log","quantity":1}]', '[{"item_id":"minecraft:oak_planks","quantity":4}]', '{}', '2026-11-02T10:07:00Z');

INSERT INTO project_inventory (id, project_id, item_id, quantity, storage_location_json, created_at)
VALUES ('inv-001', 'p-001', 'minecraft:oak_log', 256, '{}', '2026-11-02T10:08:00Z');

INSERT INTO simulation_runs (id, project_id, snapshot_json, state_json, status, seed, current_tick, started_at, summary_json, overlay_json)
VALUES ('sim-001', 'p-001', '{}', '{}', 'completed', 42, 16, '2026-11-02T10:09:00Z', '{"networks":1,"online_networks":1}', '{}');

INSERT INTO simulation_events (id, simulation_id, sequence_number, tick, event_type, payload_json)
VALUES ('ev-001', 'sim-001', 1, 8, 'craft_completed', '{"item_id":"minecraft:oak_planks","quantity":4}');

INSERT INTO simulation_diagnostics (id, simulation_id, code, severity, message, affected_blocks_json, remediation_json, blocks_simulation)
VALUES ('diag-001', 'sim-001', 'CHANNEL_INFO', 'info', '1 / 32 channels used', '[]', '""', 0);
SQL

dump_canonical_state() {
  sqlite3 "$1" <<'SQL'
SELECT 'users', id, email, password_hash FROM users ORDER BY id;
SELECT 'profiles', id, name, minecraft_version, ae2_version, channel_mode FROM compatibility_profiles ORDER BY id;
SELECT 'blocks_def', id, profile_id, name, fidelity_label FROM block_definitions ORDER BY id, profile_id;
SELECT 'projects', id, owner_user_id, name, compatibility_profile_id FROM projects ORDER BY id;
SELECT 'project_blocks', id, project_id, block_definition_id, x, y, z, facing, label FROM project_blocks ORDER BY id;
SELECT 'project_patterns', id, project_id, pattern_type, inputs_json, outputs_json FROM project_patterns ORDER BY id;
SELECT 'project_inventory', id, project_id, item_id, quantity FROM project_inventory ORDER BY id;
SELECT 'simulation_runs', id, project_id, status, seed, current_tick, summary_json FROM simulation_runs ORDER BY id;
SELECT 'simulation_events', id, simulation_id, sequence_number, tick, event_type, payload_json FROM simulation_events ORDER BY id;
SELECT 'simulation_diagnostics', id, simulation_id, code, severity, message FROM simulation_diagnostics ORDER BY id;
SQL
}

PRE_STATE="$(dump_canonical_state "$DATABASE_URL")"
if [ -z "$PRE_STATE" ]; then
  echo "ERROR: Pre-backup state query returned empty output" >&2
  exit 1
fi

echo "[test] Running backup-sqlite.sh and verifying retention pruning..."
BACKUP_TIMESTAMP="20261102T010000Z" "$BACKUP_SCRIPT"
BACKUP_TIMESTAMP="20261102T020000Z" RETENTION_COUNT=1 "$BACKUP_SCRIPT"

if [ -f "${BACKUP_DIR}/me-lab-backup-20261102T010000Z.db.gz" ]; then
  echo "ERROR: Expected older backup 20261102T010000Z to be pruned when RETENTION_COUNT=1" >&2
  exit 1
fi
LATEST_ARCHIVE="${BACKUP_DIR}/me-lab-backup-20261102T020000Z.db.gz"
if [ ! -f "$LATEST_ARCHIVE" ] || [ ! -f "${LATEST_ARCHIVE}.sha256" ]; then
  echo "ERROR: Expected backup archive and .sha256 file in ${BACKUP_DIR}" >&2
  exit 1
fi

echo "[test] Verifying tampered checksum is rejected by restore-sqlite.sh..."
cp -f "$LATEST_ARCHIVE" "${TAMPER_DIR}/me-lab-backup-20261102T020000Z.db.gz"
printf '0000000000000000000000000000000000000000000000000000000000000000  me-lab-backup-20261102T020000Z.db.gz\n' \
  > "${TAMPER_DIR}/me-lab-backup-20261102T020000Z.db.gz.sha256"
if LOCAL_BACKUP_DIR="$TAMPER_DIR" DATABASE_URL="${WORK_DIR}/should-not-exist.db" "$RESTORE_SCRIPT" >/dev/null 2>&1; then
  echo "ERROR: restore-sqlite.sh succeeded despite tampered SHA-256 checksum" >&2
  exit 1
fi

echo "[test] Wiping live SQLite database + WAL/SHM files..."
rm -f "$DATABASE_URL" "${DATABASE_URL}-wal" "${DATABASE_URL}-shm"
if [ -f "$DATABASE_URL" ]; then
  echo "ERROR: Failed to delete live database" >&2
  exit 1
fi

echo "[test] Restoring SQLite database via restore-sqlite.sh..."
"$RESTORE_SCRIPT"

if [ ! -f "$DATABASE_URL" ]; then
  echo "ERROR: Restored database file missing at ${DATABASE_URL}" >&2
  exit 1
fi

POST_INTEGRITY="$(sqlite3 "$DATABASE_URL" "PRAGMA integrity_check;")"
POST_FK="$(sqlite3 "$DATABASE_URL" "PRAGMA foreign_key_check;")"
if [ "$POST_INTEGRITY" != "ok" ] || [ -n "$POST_FK" ]; then
  echo "ERROR: Post-restore health check failed (integrity=${POST_INTEGRITY}, fk=${POST_FK})" >&2
  exit 1
fi

POST_STATE="$(dump_canonical_state "$DATABASE_URL")"
if [ "$PRE_STATE" != "$POST_STATE" ]; then
  echo "ERROR: Restored database contents do not match pre-backup database!" >&2
  echo "--- EXPECTED ---" >&2
  echo "$PRE_STATE" >&2
  echo "--- ACTUAL ---" >&2
  echo "$POST_STATE" >&2
  exit 1
fi

echo "[test] PASS: Online WAL backup, checksum verification, wipe, and atomic restore verified across all 10 tables."
