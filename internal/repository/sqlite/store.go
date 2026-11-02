package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/felipinho/me-lab/internal/catalog"
	"github.com/felipinho/me-lab/internal/domain"
	"github.com/google/uuid"
	moderncsqlite "modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

var (
	// ErrNotFound is returned when a requested entity does not exist.
	ErrNotFound = errors.New("record not found")
	// ErrConflict is returned when an operation violates a uniqueness constraint.
	ErrConflict = errors.New("record conflict")
)

// BatchOpType identifies the action performed by a BatchOperation.
type BatchOpType string

const (
	BatchOpCreate BatchOpType = "create"
	BatchOpPlace  BatchOpType = "place"
	BatchOpUpdate BatchOpType = "update"
	BatchOpDelete BatchOpType = "delete"
)

// BatchOperation describes a single block mutation executed atomically inside ApplyBlockBatch.
type BatchOperation struct {
	Op         BatchOpType      `json:"op"`
	ID         string           `json:"id,omitempty"`
	BlockDefID string           `json:"block_def_id,omitempty"`
	X          int              `json:"x"`
	Y          int              `json:"y"`
	Z          int              `json:"z"`
	Coord      *domain.Coord3D  `json:"coord,omitempty"`
	Facing     domain.Direction `json:"facing,omitempty"`
	Config     map[string]any   `json:"config,omitempty"`
	Label      string           `json:"label,omitempty"`
}

// Store provides SQLite persistence for users, projects, voxel blocks, patterns,
// inventories, and simulation runs.
type Store struct {
	db *sql.DB
}

// NewStore wraps db in a repository Store.
func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// DB exposes the underlying *sql.DB handle.
func (s *Store) DB() *sql.DB {
	return s.db
}

// ---------------------------------------------------------------------------
// Users
// ---------------------------------------------------------------------------

// CreateUser inserts a new user account, returning ErrConflict if the email or ID already exists.
func (s *Store) CreateUser(ctx context.Context, u domain.User) (domain.User, error) {
	if u.ID == "" {
		u.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	if u.CreatedAt.IsZero() {
		u.CreatedAt = now
	}
	if u.UpdatedAt.IsZero() {
		u.UpdatedAt = u.CreatedAt
	}

	const q = `INSERT INTO users (id, email, password_hash, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`
	if _, err := s.db.ExecContext(ctx, q, u.ID, u.Email, u.PasswordHash, u.CreatedAt.UTC(), u.UpdatedAt.UTC()); err != nil {
		return domain.User{}, mapConstraintError("create user", err)
	}
	return u, nil
}

// GetUserByEmail looks up a user by email address, returning ErrNotFound if absent.
func (s *Store) GetUserByEmail(ctx context.Context, email string) (domain.User, error) {
	const q = `SELECT id, email, password_hash, created_at, updated_at FROM users WHERE email = ?`
	var u domain.User
	err := s.db.QueryRowContext(ctx, q, email).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.User{}, ErrNotFound
	}
	if err != nil {
		return domain.User{}, fmt.Errorf("get user by email: %w", err)
	}
	return u, nil
}

// GetUserByID looks up a user by primary key, returning ErrNotFound if absent.
func (s *Store) GetUserByID(ctx context.Context, id string) (domain.User, error) {
	const q = `SELECT id, email, password_hash, created_at, updated_at FROM users WHERE id = ?`
	var u domain.User
	err := s.db.QueryRowContext(ctx, q, id).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.User{}, ErrNotFound
	}
	if err != nil {
		return domain.User{}, fmt.Errorf("get user by id: %w", err)
	}
	return u, nil
}

// ---------------------------------------------------------------------------
// Projects
// ---------------------------------------------------------------------------

// CreateProject persists a new project owned by p.UserID.
func (s *Store) CreateProject(ctx context.Context, p domain.Project) (domain.Project, error) {
	if p.ID == "" {
		p.ID = uuid.NewString()
	}
	if p.ProfileID == "" {
		p.ProfileID = catalog.DefaultProfileID
	}
	if p.Settings == nil {
		p.Settings = map[string]any{}
	}
	now := time.Now().UTC()
	if p.CreatedAt.IsZero() {
		p.CreatedAt = now
	}
	if p.UpdatedAt.IsZero() {
		p.UpdatedAt = p.CreatedAt
	}

	settingsJSON, err := marshalMapJSON(p.Settings)
	if err != nil {
		return domain.Project{}, fmt.Errorf("marshal project settings: %w", err)
	}

	const q = `
		INSERT INTO projects (id, owner_user_id, name, description, compatibility_profile_id, settings_json, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	if _, err := s.db.ExecContext(
		ctx,
		q,
		p.ID,
		p.UserID,
		p.Name,
		p.Description,
		p.ProfileID,
		settingsJSON,
		p.CreatedAt.UTC(),
		p.UpdatedAt.UTC(),
	); err != nil {
		return domain.Project{}, mapConstraintError("create project", err)
	}
	return p, nil
}

// GetProject retrieves a project by ID, returning ErrNotFound if absent.
func (s *Store) GetProject(ctx context.Context, id string) (domain.Project, error) {
	const q = `
		SELECT id, owner_user_id, name, description, compatibility_profile_id, settings_json, created_at, updated_at
		FROM projects
		WHERE id = ?`
	var (
		p            domain.Project
		settingsJSON string
	)
	err := s.db.QueryRowContext(ctx, q, id).Scan(
		&p.ID,
		&p.UserID,
		&p.Name,
		&p.Description,
		&p.ProfileID,
		&settingsJSON,
		&p.CreatedAt,
		&p.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Project{}, ErrNotFound
	}
	if err != nil {
		return domain.Project{}, fmt.Errorf("get project: %w", err)
	}

	p.Settings, err = unmarshalMapJSON(settingsJSON)
	if err != nil {
		return domain.Project{}, fmt.Errorf("unmarshal project settings: %w", err)
	}
	return p, nil
}

// ListProjectsByOwner returns a paginated slice of projects owned by userID along with total count.
func (s *Store) ListProjectsByOwner(ctx context.Context, userID string, limit, offset int) ([]domain.Project, int, error) {
	limit, offset = normalizePage(limit, offset)

	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM projects WHERE owner_user_id = ?`, userID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count projects: %w", err)
	}

	const q = `
		SELECT id, owner_user_id, name, description, compatibility_profile_id, settings_json, created_at, updated_at
		FROM projects
		WHERE owner_user_id = ?
		ORDER BY updated_at DESC, id ASC
		LIMIT ? OFFSET ?`
	rows, err := s.db.QueryContext(ctx, q, userID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list projects: %w", err)
	}
	defer rows.Close()

	projects := make([]domain.Project, 0)
	for rows.Next() {
		var (
			p            domain.Project
			settingsJSON string
		)
		if err := rows.Scan(
			&p.ID,
			&p.UserID,
			&p.Name,
			&p.Description,
			&p.ProfileID,
			&settingsJSON,
			&p.CreatedAt,
			&p.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan project: %w", err)
		}
		p.Settings, err = unmarshalMapJSON(settingsJSON)
		if err != nil {
			return nil, 0, fmt.Errorf("unmarshal project settings: %w", err)
		}
		projects = append(projects, p)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate projects: %w", err)
	}

	return projects, total, nil
}

// UpdateProject updates mutable fields of an existing project, returning ErrNotFound if absent.
func (s *Store) UpdateProject(ctx context.Context, p domain.Project) (domain.Project, error) {
	existing, err := s.GetProject(ctx, p.ID)
	if err != nil {
		return domain.Project{}, err
	}

	if p.Name != "" {
		existing.Name = p.Name
	}
	existing.Description = p.Description
	if p.ProfileID != "" {
		existing.ProfileID = p.ProfileID
	}
	if p.Settings != nil {
		existing.Settings = p.Settings
	}
	existing.UpdatedAt = time.Now().UTC()

	settingsJSON, err := marshalMapJSON(existing.Settings)
	if err != nil {
		return domain.Project{}, fmt.Errorf("marshal project settings: %w", err)
	}

	const q = `
		UPDATE projects
		SET name = ?, description = ?, compatibility_profile_id = ?, settings_json = ?, updated_at = ?
		WHERE id = ?`
	res, err := s.db.ExecContext(ctx, q, existing.Name, existing.Description, existing.ProfileID, settingsJSON, existing.UpdatedAt, existing.ID)
	if err != nil {
		return domain.Project{}, mapConstraintError("update project", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return domain.Project{}, fmt.Errorf("update project rows affected: %w", err)
	}
	if affected == 0 {
		return domain.Project{}, ErrNotFound
	}
	return existing, nil
}

// DeleteProject deletes a project (and cascading child rows), returning ErrNotFound if absent.
func (s *Store) DeleteProject(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM projects WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete project: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete project rows affected: %w", err)
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

// ---------------------------------------------------------------------------
// Blocks
// ---------------------------------------------------------------------------

type queryer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// ListBlocks returns all placed blocks in projectID in deterministic (Y, Z, X) order.
func (s *Store) ListBlocks(ctx context.Context, projectID string) ([]domain.PlacedBlock, error) {
	return listBlocksWith(ctx, s.db, projectID)
}

func listBlocksWith(ctx context.Context, q queryer, projectID string) ([]domain.PlacedBlock, error) {
	const sqlStr = `
		SELECT id, project_id, block_definition_id, x, y, z, facing, configuration_json, label, created_at, updated_at
		FROM project_blocks
		WHERE project_id = ?
		ORDER BY y ASC, z ASC, x ASC`
	rows, err := q.QueryContext(ctx, sqlStr, projectID)
	if err != nil {
		return nil, fmt.Errorf("list blocks: %w", err)
	}
	defer rows.Close()

	blocks := make([]domain.PlacedBlock, 0)
	for rows.Next() {
		b, err := scanPlacedBlock(rows)
		if err != nil {
			return nil, err
		}
		blocks = append(blocks, b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate blocks: %w", err)
	}
	return blocks, nil
}

// GetBlock fetches a single placed block in projectID by blockID, returning ErrNotFound if absent.
func (s *Store) GetBlock(ctx context.Context, projectID, blockID string) (domain.PlacedBlock, error) {
	return getBlockWith(ctx, s.db, projectID, blockID)
}

func getBlockWith(ctx context.Context, q queryer, projectID, blockID string) (domain.PlacedBlock, error) {
	const sqlStr = `
		SELECT id, project_id, block_definition_id, x, y, z, facing, configuration_json, label, created_at, updated_at
		FROM project_blocks
		WHERE project_id = ? AND id = ?`
	row := q.QueryRowContext(ctx, sqlStr, projectID, blockID)
	b, err := scanPlacedBlockRow(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.PlacedBlock{}, ErrNotFound
	}
	if err != nil {
		return domain.PlacedBlock{}, err
	}
	return b, nil
}

// CreateBlock places a single block in a project's 3D grid, returning ErrConflict if (project_id, x, y, z) is occupied.
func (s *Store) CreateBlock(ctx context.Context, b domain.PlacedBlock) (domain.PlacedBlock, error) {
	return createBlockWith(ctx, s.db, b)
}

func createBlockWith(ctx context.Context, q queryer, b domain.PlacedBlock) (domain.PlacedBlock, error) {
	if b.ID == "" {
		b.ID = uuid.NewString()
	}
	if !b.Facing.IsValid() {
		b.Facing = domain.North
	}
	if b.Config == nil {
		b.Config = map[string]any{}
	}
	now := time.Now().UTC()
	if b.CreatedAt.IsZero() {
		b.CreatedAt = now
	}
	if b.UpdatedAt.IsZero() {
		b.UpdatedAt = b.CreatedAt
	}

	cfgJSON, err := marshalMapJSON(b.Config)
	if err != nil {
		return domain.PlacedBlock{}, fmt.Errorf("marshal block config: %w", err)
	}

	const sqlStr = `
		INSERT INTO project_blocks (id, project_id, block_definition_id, x, y, z, facing, configuration_json, label, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	if _, err := q.ExecContext(
		ctx,
		sqlStr,
		b.ID,
		b.ProjectID,
		b.BlockDefID,
		b.X,
		b.Y,
		b.Z,
		string(b.Facing),
		cfgJSON,
		b.Label,
		b.CreatedAt.UTC(),
		b.UpdatedAt.UTC(),
	); err != nil {
		return domain.PlacedBlock{}, mapConstraintError("create block", err)
	}
	return b, nil
}

// UpdateBlock modifies an existing block in b.ProjectID, returning ErrConflict on coordinate collision or ErrNotFound if absent.
func (s *Store) UpdateBlock(ctx context.Context, b domain.PlacedBlock) (domain.PlacedBlock, error) {
	return updateBlockWith(ctx, s.db, b)
}

func updateBlockWith(ctx context.Context, q queryer, b domain.PlacedBlock) (domain.PlacedBlock, error) {
	existing, err := getBlockWith(ctx, q, b.ProjectID, b.ID)
	if err != nil {
		return domain.PlacedBlock{}, err
	}

	if b.BlockDefID != "" {
		existing.BlockDefID = b.BlockDefID
	}
	existing.X = b.X
	existing.Y = b.Y
	existing.Z = b.Z
	if b.Facing.IsValid() {
		existing.Facing = b.Facing
	}
	if b.Config != nil {
		existing.Config = b.Config
	}
	existing.Label = b.Label
	existing.UpdatedAt = time.Now().UTC()

	cfgJSON, err := marshalMapJSON(existing.Config)
	if err != nil {
		return domain.PlacedBlock{}, fmt.Errorf("marshal block config: %w", err)
	}

	const sqlStr = `
		UPDATE project_blocks
		SET block_definition_id = ?, x = ?, y = ?, z = ?, facing = ?, configuration_json = ?, label = ?, updated_at = ?
		WHERE project_id = ? AND id = ?`
	res, err := q.ExecContext(
		ctx,
		sqlStr,
		existing.BlockDefID,
		existing.X,
		existing.Y,
		existing.Z,
		string(existing.Facing),
		cfgJSON,
		existing.Label,
		existing.UpdatedAt,
		existing.ProjectID,
		existing.ID,
	)
	if err != nil {
		return domain.PlacedBlock{}, mapConstraintError("update block", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return domain.PlacedBlock{}, fmt.Errorf("update block rows affected: %w", err)
	}
	if affected == 0 {
		return domain.PlacedBlock{}, ErrNotFound
	}
	return existing, nil
}

// DeleteBlock removes a placed block from projectID, returning ErrNotFound if absent.
func (s *Store) DeleteBlock(ctx context.Context, projectID, blockID string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM project_blocks WHERE project_id = ? AND id = ?`, projectID, blockID)
	if err != nil {
		return fmt.Errorf("delete block: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete block rows affected: %w", err)
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

// ApplyBlockBatch executes a slice of block create/update/delete operations (optionally clearing
// existing blocks first when replaceAll is true) inside a single atomic SQLite transaction.
func (s *Store) ApplyBlockBatch(ctx context.Context, projectID string, ops []BatchOperation, replaceAll bool) ([]domain.PlacedBlock, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin block batch tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := ensureProjectExists(ctx, tx, projectID); err != nil {
		return nil, err
	}

	if replaceAll {
		if _, err := tx.ExecContext(ctx, `DELETE FROM project_blocks WHERE project_id = ?`, projectID); err != nil {
			return nil, fmt.Errorf("clear project blocks: %w", err)
		}
	}

	for i, op := range ops {
		action := strings.ToLower(strings.TrimSpace(string(op.Op)))
		switch action {
		case "create", "place", "add":
			x, y, z := resolveOpCoords(op)
			placed := domain.PlacedBlock{
				ID:         op.ID,
				ProjectID:  projectID,
				BlockDefID: op.BlockDefID,
				X:          x,
				Y:          y,
				Z:          z,
				Facing:     op.Facing,
				Config:     op.Config,
				Label:      op.Label,
			}
			if _, err := createBlockWith(ctx, tx, placed); err != nil {
				return nil, err
			}

		case "update":
			existing, err := getBlockWith(ctx, tx, projectID, op.ID)
			if err != nil {
				return nil, err
			}
			if op.BlockDefID != "" {
				existing.BlockDefID = op.BlockDefID
			}
			if op.Coord != nil {
				existing.X = op.Coord.X
				existing.Y = op.Coord.Y
				existing.Z = op.Coord.Z
			} else if op.X != 0 || op.Y != 0 || op.Z != 0 {
				existing.X = op.X
				existing.Y = op.Y
				existing.Z = op.Z
			}
			if op.Facing.IsValid() {
				existing.Facing = op.Facing
			}
			if op.Config != nil {
				existing.Config = op.Config
			}
			if op.Label != "" {
				existing.Label = op.Label
			}
			if _, err := updateBlockWith(ctx, tx, existing); err != nil {
				return nil, err
			}

		case "delete", "remove":
			var res sql.Result
			if op.ID != "" {
				res, err = tx.ExecContext(ctx, `DELETE FROM project_blocks WHERE project_id = ? AND id = ?`, projectID, op.ID)
			} else {
				x, y, z := resolveOpCoords(op)
				res, err = tx.ExecContext(ctx, `DELETE FROM project_blocks WHERE project_id = ? AND x = ? AND y = ? AND z = ?`, projectID, x, y, z)
			}
			if err != nil {
				return nil, fmt.Errorf("batch delete op %d: %w", i, err)
			}
			affected, err := res.RowsAffected()
			if err != nil {
				return nil, fmt.Errorf("batch delete op %d rows affected: %w", i, err)
			}
			if affected == 0 {
				return nil, ErrNotFound
			}

		default:
			return nil, fmt.Errorf("unsupported batch operation %q at index %d", op.Op, i)
		}
	}

	blocks, err := listBlocksWith(ctx, tx, projectID)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit block batch: %w", err)
	}
	return blocks, nil
}

// ---------------------------------------------------------------------------
// Patterns & Inventory
// ---------------------------------------------------------------------------

// ListPatterns returns all project-level patterns for projectID.
func (s *Store) ListPatterns(ctx context.Context, projectID string) ([]domain.ProjectPattern, error) {
	const q = `
		SELECT id, project_id, pattern_type, inputs_json, outputs_json, configuration_json
		FROM project_patterns
		WHERE project_id = ?
		ORDER BY rowid ASC`
	rows, err := s.db.QueryContext(ctx, q, projectID)
	if err != nil {
		return nil, fmt.Errorf("list patterns: %w", err)
	}
	defer rows.Close()

	patterns := make([]domain.ProjectPattern, 0)
	for rows.Next() {
		var (
			pat        domain.ProjectPattern
			inputsRaw  string
			outputsRaw string
			configRaw  string
		)
		if err := rows.Scan(&pat.ID, &pat.ProjectID, &pat.PatternType, &inputsRaw, &outputsRaw, &configRaw); err != nil {
			return nil, fmt.Errorf("scan pattern: %w", err)
		}
		if err := json.Unmarshal([]byte(inputsRaw), &pat.Inputs); err != nil {
			return nil, fmt.Errorf("unmarshal pattern inputs: %w", err)
		}
		if pat.Inputs == nil {
			pat.Inputs = []domain.ItemStack{}
		}
		if err := json.Unmarshal([]byte(outputsRaw), &pat.Outputs); err != nil {
			return nil, fmt.Errorf("unmarshal pattern outputs: %w", err)
		}
		if pat.Outputs == nil {
			pat.Outputs = []domain.ItemStack{}
		}
		pat.Config, err = unmarshalMapJSON(configRaw)
		if err != nil {
			return nil, fmt.Errorf("unmarshal pattern config: %w", err)
		}
		patterns = append(patterns, pat)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate patterns: %w", err)
	}
	return patterns, nil
}

// ReplacePatterns atomically replaces all project-level patterns for projectID.
func (s *Store) ReplacePatterns(ctx context.Context, projectID string, patterns []domain.ProjectPattern) ([]domain.ProjectPattern, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin replace patterns tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := ensureProjectExists(ctx, tx, projectID); err != nil {
		return nil, err
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM project_patterns WHERE project_id = ?`, projectID); err != nil {
		return nil, fmt.Errorf("clear project patterns: %w", err)
	}

	const insertSQL = `
		INSERT INTO project_patterns (id, project_id, pattern_type, inputs_json, outputs_json, configuration_json)
		VALUES (?, ?, ?, ?, ?, ?)`
	stmt, err := tx.PrepareContext(ctx, insertSQL)
	if err != nil {
		return nil, fmt.Errorf("prepare insert pattern: %w", err)
	}
	defer stmt.Close()

	out := make([]domain.ProjectPattern, 0, len(patterns))
	for _, pat := range patterns {
		if pat.ID == "" {
			pat.ID = uuid.NewString()
		}
		pat.ProjectID = projectID
		if pat.Inputs == nil {
			pat.Inputs = []domain.ItemStack{}
		}
		if pat.Outputs == nil {
			pat.Outputs = []domain.ItemStack{}
		}
		if pat.Config == nil {
			pat.Config = map[string]any{}
		}

		inJSON, err := json.Marshal(pat.Inputs)
		if err != nil {
			return nil, fmt.Errorf("marshal pattern inputs: %w", err)
		}
		outJSON, err := json.Marshal(pat.Outputs)
		if err != nil {
			return nil, fmt.Errorf("marshal pattern outputs: %w", err)
		}
		cfgJSON, err := marshalMapJSON(pat.Config)
		if err != nil {
			return nil, fmt.Errorf("marshal pattern config: %w", err)
		}

		if _, err := stmt.ExecContext(ctx, pat.ID, pat.ProjectID, pat.PatternType, string(inJSON), string(outJSON), cfgJSON); err != nil {
			return nil, mapConstraintError("insert pattern", err)
		}
		out = append(out, pat)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit replace patterns: %w", err)
	}
	return out, nil
}

// ListInventory returns all project-level initial inventory records for projectID.
func (s *Store) ListInventory(ctx context.Context, projectID string) ([]domain.ProjectInventoryEntry, error) {
	const q = `
		SELECT id, project_id, item_id, quantity, storage_location_json
		FROM project_inventory
		WHERE project_id = ?
		ORDER BY rowid ASC`
	rows, err := s.db.QueryContext(ctx, q, projectID)
	if err != nil {
		return nil, fmt.Errorf("list inventory: %w", err)
	}
	defer rows.Close()

	entries := make([]domain.ProjectInventoryEntry, 0)
	for rows.Next() {
		var (
			entry  domain.ProjectInventoryEntry
			locRaw string
		)
		if err := rows.Scan(&entry.ID, &entry.ProjectID, &entry.ItemID, &entry.Quantity, &locRaw); err != nil {
			return nil, fmt.Errorf("scan inventory entry: %w", err)
		}
		entry.StorageLocation, err = unmarshalMapJSON(locRaw)
		if err != nil {
			return nil, fmt.Errorf("unmarshal inventory storage_location: %w", err)
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate inventory: %w", err)
	}
	return entries, nil
}

// ReplaceInventory atomically replaces all project-level initial inventory records for projectID.
func (s *Store) ReplaceInventory(ctx context.Context, projectID string, entries []domain.ProjectInventoryEntry) ([]domain.ProjectInventoryEntry, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin replace inventory tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := ensureProjectExists(ctx, tx, projectID); err != nil {
		return nil, err
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM project_inventory WHERE project_id = ?`, projectID); err != nil {
		return nil, fmt.Errorf("clear project inventory: %w", err)
	}

	const insertSQL = `
		INSERT INTO project_inventory (id, project_id, item_id, quantity, storage_location_json)
		VALUES (?, ?, ?, ?, ?)`
	stmt, err := tx.PrepareContext(ctx, insertSQL)
	if err != nil {
		return nil, fmt.Errorf("prepare insert inventory: %w", err)
	}
	defer stmt.Close()

	out := make([]domain.ProjectInventoryEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.ID == "" {
			entry.ID = uuid.NewString()
		}
		entry.ProjectID = projectID
		if entry.StorageLocation == nil {
			entry.StorageLocation = map[string]any{}
		}

		locJSON, err := marshalMapJSON(entry.StorageLocation)
		if err != nil {
			return nil, fmt.Errorf("marshal inventory storage_location: %w", err)
		}

		if _, err := stmt.ExecContext(ctx, entry.ID, entry.ProjectID, entry.ItemID, entry.Quantity, locJSON); err != nil {
			return nil, mapConstraintError("insert inventory", err)
		}
		out = append(out, entry)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit replace inventory: %w", err)
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Simulations
// ---------------------------------------------------------------------------

// CreateSimulationRun persists a simulation run, its initial snapshot/state JSON,
// initial events, and initial diagnostics inside a single transaction.
func (s *Store) CreateSimulationRun(
	ctx context.Context,
	run domain.SimulationRun,
	snapshotJSON, stateJSON []byte,
	events []domain.SimulationEvent,
	diags []domain.Diagnostic,
) error {
	if run.ID == "" {
		run.ID = uuid.NewString()
	}
	if run.Status == "" {
		run.Status = domain.SimStatusPending
	}
	if run.StartedAt.IsZero() {
		run.StartedAt = time.Now().UTC()
	}
	if len(snapshotJSON) == 0 {
		snapshotJSON = []byte("{}")
	}
	if len(stateJSON) == 0 {
		stateJSON = []byte("{}")
	}

	summaryBytes, err := json.Marshal(run.Summary)
	if err != nil {
		return fmt.Errorf("marshal simulation summary: %w", err)
	}
	overlayStr, err := marshalOverlayJSON(run.Overlay)
	if err != nil {
		return fmt.Errorf("marshal simulation overlay: %w", err)
	}

	var finishedAt sql.NullTime
	if run.FinishedAt != nil {
		finishedAt = sql.NullTime{Time: run.FinishedAt.UTC(), Valid: true}
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin create simulation tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	const insertRunSQL = `
		INSERT INTO simulation_runs (
			id, project_id, snapshot_json, state_json, status, seed, current_tick,
			started_at, finished_at, summary_json, overlay_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	if _, err := tx.ExecContext(
		ctx,
		insertRunSQL,
		run.ID,
		run.ProjectID,
		string(snapshotJSON),
		string(stateJSON),
		string(run.Status),
		run.Seed,
		run.CurrentTick,
		run.StartedAt.UTC(),
		finishedAt,
		string(summaryBytes),
		overlayStr,
	); err != nil {
		return mapConstraintError("create simulation run", err)
	}

	if err := insertSimulationEvents(ctx, tx, run.ID, events); err != nil {
		return err
	}
	if err := insertSimulationDiagnostics(ctx, tx, run.ID, diags); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit create simulation run: %w", err)
	}
	return nil
}

// GetSimulationRun fetches a simulation run along with its raw snapshotJSON and stateJSON blobs.
func (s *Store) GetSimulationRun(ctx context.Context, simID string) (domain.SimulationRun, []byte, []byte, error) {
	const q = `
		SELECT id, project_id, snapshot_json, state_json, status, seed, current_tick,
		       started_at, finished_at, summary_json, overlay_json
		FROM simulation_runs
		WHERE id = ?`
	var (
		run         domain.SimulationRun
		snapshotStr string
		stateStr    string
		statusStr   string
		finishedAt  sql.NullTime
		summaryStr  string
		overlayStr  string
	)
	err := s.db.QueryRowContext(ctx, q, simID).Scan(
		&run.ID,
		&run.ProjectID,
		&snapshotStr,
		&stateStr,
		&statusStr,
		&run.Seed,
		&run.CurrentTick,
		&run.StartedAt,
		&finishedAt,
		&summaryStr,
		&overlayStr,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.SimulationRun{}, nil, nil, ErrNotFound
	}
	if err != nil {
		return domain.SimulationRun{}, nil, nil, fmt.Errorf("get simulation run: %w", err)
	}

	run.Status = domain.SimulationStatus(statusStr)
	if finishedAt.Valid {
		t := finishedAt.Time
		run.FinishedAt = &t
	}
	if err := json.Unmarshal([]byte(summaryStr), &run.Summary); err != nil {
		return domain.SimulationRun{}, nil, nil, fmt.Errorf("unmarshal simulation summary: %w", err)
	}
	run.Overlay, err = unmarshalOverlayJSON(overlayStr)
	if err != nil {
		return domain.SimulationRun{}, nil, nil, fmt.Errorf("unmarshal simulation overlay: %w", err)
	}

	return run, []byte(snapshotStr), []byte(stateStr), nil
}

// ListSimulationRuns returns a paginated slice of simulation runs for projectID along with total count.
func (s *Store) ListSimulationRuns(ctx context.Context, projectID string, limit, offset int) ([]domain.SimulationRun, int, error) {
	limit, offset = normalizePage(limit, offset)

	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM simulation_runs WHERE project_id = ?`, projectID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count simulation runs: %w", err)
	}

	const q = `
		SELECT id, project_id, status, seed, current_tick, started_at, finished_at, summary_json, overlay_json
		FROM simulation_runs
		WHERE project_id = ?
		ORDER BY started_at DESC, id DESC
		LIMIT ? OFFSET ?`
	rows, err := s.db.QueryContext(ctx, q, projectID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list simulation runs: %w", err)
	}
	defer rows.Close()

	runs := make([]domain.SimulationRun, 0)
	for rows.Next() {
		var (
			run        domain.SimulationRun
			statusStr  string
			finishedAt sql.NullTime
			summaryStr string
			overlayStr string
		)
		if err := rows.Scan(
			&run.ID,
			&run.ProjectID,
			&statusStr,
			&run.Seed,
			&run.CurrentTick,
			&run.StartedAt,
			&finishedAt,
			&summaryStr,
			&overlayStr,
		); err != nil {
			return nil, 0, fmt.Errorf("scan simulation run: %w", err)
		}
		run.Status = domain.SimulationStatus(statusStr)
		if finishedAt.Valid {
			t := finishedAt.Time
			run.FinishedAt = &t
		}
		if err := json.Unmarshal([]byte(summaryStr), &run.Summary); err != nil {
			return nil, 0, fmt.Errorf("unmarshal simulation summary: %w", err)
		}
		run.Overlay, err = unmarshalOverlayJSON(overlayStr)
		if err != nil {
			return nil, 0, fmt.Errorf("unmarshal simulation overlay: %w", err)
		}
		runs = append(runs, run)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate simulation runs: %w", err)
	}
	return runs, total, nil
}

// UpdateSimulationRunState updates mutable run state, appends newEvents, and replaces active diagnostics
// inside a single transaction.
func (s *Store) UpdateSimulationRunState(
	ctx context.Context,
	run domain.SimulationRun,
	stateJSON []byte,
	newEvents []domain.SimulationEvent,
	replaceDiags []domain.Diagnostic,
) error {
	if len(stateJSON) == 0 {
		stateJSON = []byte("{}")
	}
	summaryBytes, err := json.Marshal(run.Summary)
	if err != nil {
		return fmt.Errorf("marshal simulation summary: %w", err)
	}
	overlayStr, err := marshalOverlayJSON(run.Overlay)
	if err != nil {
		return fmt.Errorf("marshal simulation overlay: %w", err)
	}

	var finishedAt sql.NullTime
	if run.FinishedAt != nil {
		finishedAt = sql.NullTime{Time: run.FinishedAt.UTC(), Valid: true}
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin update simulation tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	const updateSQL = `
		UPDATE simulation_runs
		SET state_json = ?, status = ?, current_tick = ?, finished_at = ?, summary_json = ?, overlay_json = ?
		WHERE id = ?`
	res, err := tx.ExecContext(
		ctx,
		updateSQL,
		string(stateJSON),
		string(run.Status),
		run.CurrentTick,
		finishedAt,
		string(summaryBytes),
		overlayStr,
		run.ID,
	)
	if err != nil {
		return fmt.Errorf("update simulation run: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update simulation rows affected: %w", err)
	}
	if affected == 0 {
		return ErrNotFound
	}

	if err := insertSimulationEvents(ctx, tx, run.ID, newEvents); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM simulation_diagnostics WHERE simulation_id = ?`, run.ID); err != nil {
		return fmt.Errorf("clear simulation diagnostics: %w", err)
	}
	if err := insertSimulationDiagnostics(ctx, tx, run.ID, replaceDiags); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit update simulation run: %w", err)
	}
	return nil
}

// ResetSimulationRunState resets a simulation run back to its initial state, replacing all events
// and diagnostics inside a single transaction.
func (s *Store) ResetSimulationRunState(
	ctx context.Context,
	run domain.SimulationRun,
	stateJSON []byte,
	initialEvents []domain.SimulationEvent,
	diags []domain.Diagnostic,
) error {
	if len(stateJSON) == 0 {
		stateJSON = []byte("{}")
	}
	if run.StartedAt.IsZero() {
		run.StartedAt = time.Now().UTC()
	}
	summaryBytes, err := json.Marshal(run.Summary)
	if err != nil {
		return fmt.Errorf("marshal simulation summary: %w", err)
	}
	overlayStr, err := marshalOverlayJSON(run.Overlay)
	if err != nil {
		return fmt.Errorf("marshal simulation overlay: %w", err)
	}

	var finishedAt sql.NullTime
	if run.FinishedAt != nil {
		finishedAt = sql.NullTime{Time: run.FinishedAt.UTC(), Valid: true}
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin reset simulation tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	const updateSQL = `
		UPDATE simulation_runs
		SET state_json = ?, status = ?, seed = ?, current_tick = ?, started_at = ?, finished_at = ?, summary_json = ?, overlay_json = ?
		WHERE id = ?`
	res, err := tx.ExecContext(
		ctx,
		updateSQL,
		string(stateJSON),
		string(run.Status),
		run.Seed,
		run.CurrentTick,
		run.StartedAt.UTC(),
		finishedAt,
		string(summaryBytes),
		overlayStr,
		run.ID,
	)
	if err != nil {
		return fmt.Errorf("reset simulation run: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("reset simulation rows affected: %w", err)
	}
	if affected == 0 {
		return ErrNotFound
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM simulation_events WHERE simulation_id = ?`, run.ID); err != nil {
		return fmt.Errorf("clear simulation events: %w", err)
	}
	if err := insertSimulationEvents(ctx, tx, run.ID, initialEvents); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM simulation_diagnostics WHERE simulation_id = ?`, run.ID); err != nil {
		return fmt.Errorf("clear simulation diagnostics: %w", err)
	}
	if err := insertSimulationDiagnostics(ctx, tx, run.ID, diags); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit reset simulation run: %w", err)
	}
	return nil
}

// ListSimulationEvents returns paginated simulation events ordered by sequence_number ASC.
func (s *Store) ListSimulationEvents(ctx context.Context, simID string, limit, offset int) ([]domain.SimulationEvent, int, error) {
	limit, offset = normalizePage(limit, offset)

	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM simulation_events WHERE simulation_id = ?`, simID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count simulation events: %w", err)
	}

	const q = `
		SELECT id, simulation_id, sequence_number, tick, event_type, payload_json
		FROM simulation_events
		WHERE simulation_id = ?
		ORDER BY sequence_number ASC
		LIMIT ? OFFSET ?`
	rows, err := s.db.QueryContext(ctx, q, simID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list simulation events: %w", err)
	}
	defer rows.Close()

	events := make([]domain.SimulationEvent, 0)
	for rows.Next() {
		var (
			ev         domain.SimulationEvent
			payloadRaw string
		)
		if err := rows.Scan(&ev.ID, &ev.SimulationID, &ev.Sequence, &ev.Tick, &ev.EventType, &payloadRaw); err != nil {
			return nil, 0, fmt.Errorf("scan simulation event: %w", err)
		}
		ev.Payload, err = unmarshalMapJSON(payloadRaw)
		if err != nil {
			return nil, 0, fmt.Errorf("unmarshal event payload: %w", err)
		}
		events = append(events, ev)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate simulation events: %w", err)
	}
	return events, total, nil
}

// ListSimulationDiagnostics returns all diagnostics persisted for simID.
func (s *Store) ListSimulationDiagnostics(ctx context.Context, simID string) ([]domain.Diagnostic, error) {
	const q = `
		SELECT id, simulation_id, code, severity, message, affected_blocks_json, remediation_json, blocks_simulation
		FROM simulation_diagnostics
		WHERE simulation_id = ?
		ORDER BY rowid ASC`
	rows, err := s.db.QueryContext(ctx, q, simID)
	if err != nil {
		return nil, fmt.Errorf("list simulation diagnostics: %w", err)
	}
	defer rows.Close()

	diags := make([]domain.Diagnostic, 0)
	for rows.Next() {
		var (
			d             domain.Diagnostic
			severityRaw   string
			affectedRaw   string
			remediationJS string
			blocksSimInt  int
		)
		if err := rows.Scan(
			&d.ID,
			&d.SimulationID,
			&d.Code,
			&severityRaw,
			&d.Message,
			&affectedRaw,
			&remediationJS,
			&blocksSimInt,
		); err != nil {
			return nil, fmt.Errorf("scan simulation diagnostic: %w", err)
		}
		d.Severity = domain.DiagnosticSeverity(severityRaw)
		if err := json.Unmarshal([]byte(affectedRaw), &d.AffectedBlocks); err != nil {
			return nil, fmt.Errorf("unmarshal diagnostic affected_blocks: %w", err)
		}
		if d.AffectedBlocks == nil {
			d.AffectedBlocks = []domain.AffectedBlock{}
		}
		if err := json.Unmarshal([]byte(remediationJS), &d.Remediation); err != nil {
			d.Remediation = remediationJS
		}
		d.BlocksSimulation = blocksSimInt != 0
		diags = append(diags, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate simulation diagnostics: %w", err)
	}
	return diags, nil
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

func insertSimulationEvents(ctx context.Context, tx *sql.Tx, simID string, events []domain.SimulationEvent) error {
	if len(events) == 0 {
		return nil
	}
	const q = `
		INSERT INTO simulation_events (id, simulation_id, sequence_number, tick, event_type, payload_json)
		VALUES (?, ?, ?, ?, ?, ?)`
	stmt, err := tx.PrepareContext(ctx, q)
	if err != nil {
		return fmt.Errorf("prepare insert simulation event: %w", err)
	}
	defer stmt.Close()

	for _, ev := range events {
		id := ev.ID
		if id == "" {
			id = uuid.NewString()
		}
		payloadJSON, err := marshalMapJSON(ev.Payload)
		if err != nil {
			return fmt.Errorf("marshal event payload: %w", err)
		}
		if _, err := stmt.ExecContext(ctx, id, simID, ev.Sequence, ev.Tick, ev.EventType, payloadJSON); err != nil {
			return mapConstraintError("insert simulation event", err)
		}
	}
	return nil
}

func insertSimulationDiagnostics(ctx context.Context, tx *sql.Tx, simID string, diags []domain.Diagnostic) error {
	if len(diags) == 0 {
		return nil
	}
	const q = `
		INSERT INTO simulation_diagnostics (
			id, simulation_id, code, severity, message, affected_blocks_json, remediation_json, blocks_simulation
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	stmt, err := tx.PrepareContext(ctx, q)
	if err != nil {
		return fmt.Errorf("prepare insert simulation diagnostic: %w", err)
	}
	defer stmt.Close()

	for _, d := range diags {
		id := d.ID
		if id == "" {
			id = uuid.NewString()
		}
		affected := d.AffectedBlocks
		if affected == nil {
			affected = []domain.AffectedBlock{}
		}
		affectedJSON, err := json.Marshal(affected)
		if err != nil {
			return fmt.Errorf("marshal diagnostic affected_blocks: %w", err)
		}
		remediationJSON, err := json.Marshal(d.Remediation)
		if err != nil {
			return fmt.Errorf("marshal diagnostic remediation: %w", err)
		}
		blocksSim := 0
		if d.BlocksSimulation {
			blocksSim = 1
		}
		if _, err := stmt.ExecContext(
			ctx,
			id,
			simID,
			d.Code,
			string(d.Severity),
			d.Message,
			string(affectedJSON),
			string(remediationJSON),
			blocksSim,
		); err != nil {
			return mapConstraintError("insert simulation diagnostic", err)
		}
	}
	return nil
}

func scanPlacedBlock(rows *sql.Rows) (domain.PlacedBlock, error) {
	var (
		b       domain.PlacedBlock
		facing  string
		cfgJSON string
	)
	if err := rows.Scan(
		&b.ID,
		&b.ProjectID,
		&b.BlockDefID,
		&b.X,
		&b.Y,
		&b.Z,
		&facing,
		&cfgJSON,
		&b.Label,
		&b.CreatedAt,
		&b.UpdatedAt,
	); err != nil {
		return domain.PlacedBlock{}, fmt.Errorf("scan placed block: %w", err)
	}
	b.Facing = domain.Direction(facing)
	cfg, err := unmarshalMapJSON(cfgJSON)
	if err != nil {
		return domain.PlacedBlock{}, fmt.Errorf("unmarshal block config: %w", err)
	}
	b.Config = cfg
	return b, nil
}

func scanPlacedBlockRow(row *sql.Row) (domain.PlacedBlock, error) {
	var (
		b       domain.PlacedBlock
		facing  string
		cfgJSON string
	)
	if err := row.Scan(
		&b.ID,
		&b.ProjectID,
		&b.BlockDefID,
		&b.X,
		&b.Y,
		&b.Z,
		&facing,
		&cfgJSON,
		&b.Label,
		&b.CreatedAt,
		&b.UpdatedAt,
	); err != nil {
		return domain.PlacedBlock{}, err
	}
	b.Facing = domain.Direction(facing)
	cfg, err := unmarshalMapJSON(cfgJSON)
	if err != nil {
		return domain.PlacedBlock{}, fmt.Errorf("unmarshal block config: %w", err)
	}
	b.Config = cfg
	return b, nil
}

func ensureProjectExists(ctx context.Context, q queryer, projectID string) error {
	var one int
	err := q.QueryRowContext(ctx, `SELECT 1 FROM projects WHERE id = ?`, projectID).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("check project exists: %w", err)
	}
	return nil
}

func resolveOpCoords(op BatchOperation) (int, int, int) {
	if op.Coord != nil {
		return op.Coord.X, op.Coord.Y, op.Coord.Z
	}
	return op.X, op.Y, op.Z
}

func normalizePage(limit, offset int) (int, int) {
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

func marshalMapJSON(m map[string]any) (string, error) {
	if m == nil {
		return "{}", nil
	}
	b, err := json.Marshal(m)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func unmarshalMapJSON(raw string) (map[string]any, error) {
	if raw == "" || raw == "{}" || raw == "null" {
		return map[string]any{}, nil
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return nil, err
	}
	if m == nil {
		m = map[string]any{}
	}
	return m, nil
}

func marshalOverlayJSON(ov *domain.TopologyOverlay) (string, error) {
	if ov == nil {
		return "{}", nil
	}
	b, err := json.Marshal(ov)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func unmarshalOverlayJSON(raw string) (*domain.TopologyOverlay, error) {
	if raw == "" || raw == "{}" || raw == "null" {
		return nil, nil
	}
	var ov domain.TopologyOverlay
	if err := json.Unmarshal([]byte(raw), &ov); err != nil {
		return nil, err
	}
	return &ov, nil
}

func mapConstraintError(op string, err error) error {
	var sqliteErr *moderncsqlite.Error
	if errors.As(err, &sqliteErr) {
		switch sqliteErr.Code() {
		case sqlite3.SQLITE_CONSTRAINT_UNIQUE, sqlite3.SQLITE_CONSTRAINT_PRIMARYKEY:
			return ErrConflict
		case sqlite3.SQLITE_CONSTRAINT_FOREIGNKEY:
			return ErrNotFound
		}
	}
	msg := err.Error()
	if strings.Contains(msg, "UNIQUE constraint failed") || strings.Contains(msg, "PRIMARY KEY constraint failed") {
		return ErrConflict
	}
	if strings.Contains(msg, "FOREIGN KEY constraint failed") {
		return ErrNotFound
	}
	return fmt.Errorf("%s: %w", op, err)
}
