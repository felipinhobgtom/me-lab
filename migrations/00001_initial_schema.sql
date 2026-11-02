-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY,
    email TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS compatibility_profiles (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    minecraft_version TEXT NOT NULL,
    ae2_version TEXT NOT NULL,
    channel_mode TEXT NOT NULL,
    config_json TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS projects (
    id TEXT PRIMARY KEY,
    owner_user_id TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    compatibility_profile_id TEXT NOT NULL,
    settings_json TEXT NOT NULL DEFAULT '{}',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (owner_user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (compatibility_profile_id) REFERENCES compatibility_profiles(id)
);

CREATE INDEX IF NOT EXISTS idx_projects_owner ON projects(owner_user_id);

CREATE TABLE IF NOT EXISTS block_definitions (
    id TEXT NOT NULL,
    profile_id TEXT NOT NULL,
    name TEXT NOT NULL,
    category TEXT NOT NULL,
    metadata_json TEXT NOT NULL,
    behavior_key TEXT NOT NULL,
    fidelity_label TEXT NOT NULL CHECK (fidelity_label IN ('exact', 'approximated', 'unsupported')),
    doc_reference TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (id, profile_id),
    FOREIGN KEY (profile_id) REFERENCES compatibility_profiles(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS project_blocks (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL,
    block_definition_id TEXT NOT NULL,
    x INTEGER NOT NULL,
    y INTEGER NOT NULL,
    z INTEGER NOT NULL,
    facing TEXT NOT NULL DEFAULT 'north',
    configuration_json TEXT NOT NULL DEFAULT '{}',
    label TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(project_id, x, y, z),
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_project_blocks_project ON project_blocks(project_id);

CREATE TABLE IF NOT EXISTS project_patterns (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL,
    pattern_type TEXT NOT NULL CHECK (pattern_type IN ('crafting', 'processing')),
    inputs_json TEXT NOT NULL DEFAULT '[]',
    outputs_json TEXT NOT NULL DEFAULT '[]',
    configuration_json TEXT NOT NULL DEFAULT '{}',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_project_patterns_project ON project_patterns(project_id);

CREATE TABLE IF NOT EXISTS project_inventory (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL,
    item_id TEXT NOT NULL,
    quantity INTEGER NOT NULL CHECK (quantity >= 0),
    storage_location_json TEXT NOT NULL DEFAULT '{}',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_project_inventory_project ON project_inventory(project_id);

CREATE TABLE IF NOT EXISTS simulation_runs (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL,
    snapshot_json TEXT NOT NULL,
    state_json TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('pending', 'running', 'paused', 'completed', 'failed')),
    seed INTEGER NOT NULL DEFAULT 0,
    current_tick INTEGER NOT NULL DEFAULT 0,
    started_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    finished_at DATETIME,
    summary_json TEXT NOT NULL DEFAULT '{}',
    overlay_json TEXT NOT NULL DEFAULT '{}',
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_simulation_runs_project ON simulation_runs(project_id);

CREATE TABLE IF NOT EXISTS simulation_events (
    id TEXT PRIMARY KEY,
    simulation_id TEXT NOT NULL,
    sequence_number INTEGER NOT NULL,
    tick INTEGER NOT NULL,
    event_type TEXT NOT NULL,
    payload_json TEXT NOT NULL DEFAULT '{}',
    UNIQUE(simulation_id, sequence_number),
    FOREIGN KEY (simulation_id) REFERENCES simulation_runs(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_simulation_events_sim ON simulation_events(simulation_id, sequence_number);

CREATE TABLE IF NOT EXISTS simulation_diagnostics (
    id TEXT PRIMARY KEY,
    simulation_id TEXT NOT NULL,
    code TEXT NOT NULL,
    severity TEXT NOT NULL CHECK (severity IN ('info', 'warning', 'error')),
    message TEXT NOT NULL,
    affected_blocks_json TEXT NOT NULL DEFAULT '[]',
    remediation_json TEXT NOT NULL DEFAULT '""',
    blocks_simulation INTEGER NOT NULL DEFAULT 0,
    FOREIGN KEY (simulation_id) REFERENCES simulation_runs(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_simulation_diagnostics_sim ON simulation_diagnostics(simulation_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS simulation_diagnostics;
DROP TABLE IF EXISTS simulation_events;
DROP TABLE IF EXISTS simulation_runs;
DROP TABLE IF EXISTS project_inventory;
DROP TABLE IF EXISTS project_patterns;
DROP TABLE IF EXISTS project_blocks;
DROP TABLE IF EXISTS block_definitions;
DROP TABLE IF EXISTS projects;
DROP TABLE IF EXISTS compatibility_profiles;
DROP TABLE IF EXISTS users;
-- +goose StatementEnd
