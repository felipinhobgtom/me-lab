package catalog

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"maps"
	"slices"
	"strings"

	"github.com/felipinho/me-lab/internal/domain"
)

// DefaultProfileID is the canonical AE2 1.21.1 compatibility profile identifier.
const DefaultProfileID = "ae2-1.21.1-default"

//go:embed data/profiles/*.json data/blocks/*.json
var catalogFS embed.FS

// Registry provides immutable, in-memory lookup over AE2 compatibility profiles
// and materialized block definitions.
type Registry struct {
	profiles        []domain.CompatibilityProfile
	profileByID     map[string]domain.CompatibilityProfile
	blocksByProfile map[string][]domain.BlockDefinition
	blockIndex      map[string]map[string]domain.BlockDefinition
	allBlocks       []domain.BlockDefinition
}

// NewRegistry loads embedded AE2 profile and block JSON definitions and materializes
// block definitions for every compatibility profile.
func NewRegistry() (*Registry, error) {
	profiles, err := loadProfiles(catalogFS)
	if err != nil {
		return nil, err
	}

	templates, err := loadBlockTemplates(catalogFS)
	if err != nil {
		return nil, err
	}

	reg := &Registry{
		profiles:        make([]domain.CompatibilityProfile, 0, len(profiles)),
		profileByID:     make(map[string]domain.CompatibilityProfile, len(profiles)),
		blocksByProfile: make(map[string][]domain.BlockDefinition, len(profiles)),
		blockIndex:      make(map[string]map[string]domain.BlockDefinition, len(profiles)),
		allBlocks:       make([]domain.BlockDefinition, 0, len(profiles)*len(templates)),
	}

	for _, p := range profiles {
		clonedProfile := cloneProfile(p)
		reg.profiles = append(reg.profiles, clonedProfile)
		reg.profileByID[p.ID] = clonedProfile

		profileBlocks := make([]domain.BlockDefinition, 0, len(templates))
		byID := make(map[string]domain.BlockDefinition, len(templates))

		for _, tmpl := range templates {
			mat := materializeBlock(tmpl, clonedProfile)
			profileBlocks = append(profileBlocks, mat)
			byID[mat.ID] = mat
			reg.allBlocks = append(reg.allBlocks, mat)
		}

		reg.blocksByProfile[p.ID] = profileBlocks
		reg.blockIndex[p.ID] = byID
	}

	return reg, nil
}

// Profiles returns all compatibility profiles in canonical display order.
func (r *Registry) Profiles() []domain.CompatibilityProfile {
	out := make([]domain.CompatibilityProfile, len(r.profiles))
	for i, p := range r.profiles {
		out[i] = cloneProfile(p)
	}
	return out
}

// GetProfile looks up a compatibility profile by ID.
func (r *Registry) GetProfile(id string) (domain.CompatibilityProfile, bool) {
	p, ok := r.profileByID[id]
	if !ok {
		return domain.CompatibilityProfile{}, false
	}
	return cloneProfile(p), true
}

// BlocksForProfile returns all materialized block definitions for profileID.
func (r *Registry) BlocksForProfile(profileID string) []domain.BlockDefinition {
	blocks, ok := r.blocksByProfile[profileID]
	if !ok {
		return []domain.BlockDefinition{}
	}
	out := make([]domain.BlockDefinition, len(blocks))
	for i, b := range blocks {
		out[i] = cloneBlock(b)
	}
	return out
}

// GetBlock looks up a single block definition for the given profileID and blockID.
func (r *Registry) GetBlock(profileID, blockID string) (domain.BlockDefinition, bool) {
	byID, ok := r.blockIndex[profileID]
	if !ok {
		return domain.BlockDefinition{}, false
	}
	b, ok := byID[blockID]
	if !ok {
		return domain.BlockDefinition{}, false
	}
	return cloneBlock(b), true
}

// AllBlocks returns every materialized (profileID, blockID) definition across all profiles.
func (r *Registry) AllBlocks() []domain.BlockDefinition {
	out := make([]domain.BlockDefinition, len(r.allBlocks))
	for i, b := range r.allBlocks {
		out[i] = cloneBlock(b)
	}
	return out
}

func loadProfiles(fsys fs.FS) ([]domain.CompatibilityProfile, error) {
	entries, err := fs.ReadDir(fsys, "data/profiles")
	if err != nil {
		return nil, fmt.Errorf("read profiles dir: %w", err)
	}

	profiles := make([]domain.CompatibilityProfile, 0, len(entries))
	seen := make(map[string]struct{}, len(entries))

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		raw, err := fs.ReadFile(fsys, "data/profiles/"+entry.Name())
		if err != nil {
			return nil, fmt.Errorf("read profile %s: %w", entry.Name(), err)
		}
		var p domain.CompatibilityProfile
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, fmt.Errorf("parse profile %s: %w", entry.Name(), err)
		}
		if p.ID == "" {
			return nil, fmt.Errorf("profile %s missing id", entry.Name())
		}
		if _, dup := seen[p.ID]; dup {
			return nil, fmt.Errorf("duplicate profile id %q", p.ID)
		}
		seen[p.ID] = struct{}{}
		profiles = append(profiles, p)
	}

	if len(profiles) == 0 {
		return nil, fmt.Errorf("no compatibility profiles loaded")
	}

	profilePriority := map[string]int{
		"ae2-1.21.1-default":  0,
		"ae2-1.21.1-x2":       1,
		"ae2-1.21.1-infinite": 2,
	}
	slices.SortFunc(profiles, func(a, b domain.CompatibilityProfile) int {
		pa, oka := profilePriority[a.ID]
		pb, okb := profilePriority[b.ID]
		switch {
		case oka && okb:
			return pa - pb
		case oka:
			return -1
		case okb:
			return 1
		default:
			return strings.Compare(a.ID, b.ID)
		}
	})

	return profiles, nil
}

func loadBlockTemplates(fsys fs.FS) ([]domain.BlockDefinition, error) {
	entries, err := fs.ReadDir(fsys, "data/blocks")
	if err != nil {
		return nil, fmt.Errorf("read blocks dir: %w", err)
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			names = append(names, entry.Name())
		}
	}
	slices.Sort(names)

	var templates []domain.BlockDefinition
	seen := make(map[string]struct{})

	for _, name := range names {
		raw, err := fs.ReadFile(fsys, "data/blocks/"+name)
		if err != nil {
			return nil, fmt.Errorf("read block file %s: %w", name, err)
		}
		var batch []domain.BlockDefinition
		if err := json.Unmarshal(raw, &batch); err != nil {
			return nil, fmt.Errorf("parse block file %s: %w", name, err)
		}
		for _, b := range batch {
			if b.ID == "" {
				return nil, fmt.Errorf("block definition in %s missing id", name)
			}
			if _, dup := seen[b.ID]; dup {
				return nil, fmt.Errorf("duplicate block definition id %q", b.ID)
			}
			switch b.Fidelity {
			case domain.FidelityExact, domain.FidelityApproximated, domain.FidelityUnsupported:
			default:
				return nil, fmt.Errorf("block %q has invalid fidelity %q", b.ID, b.Fidelity)
			}
			seen[b.ID] = struct{}{}
			templates = append(templates, b)
		}
	}

	if len(templates) == 0 {
		return nil, fmt.Errorf("no block definitions loaded")
	}

	return templates, nil
}

func materializeBlock(tmpl domain.BlockDefinition, profile domain.CompatibilityProfile) domain.BlockDefinition {
	def := cloneBlock(tmpl)
	def.ProfileID = profile.ID

	switch {
	case def.ID == "ae2:controller":
		def.Metadata.ChannelCapacity = profile.Config.ControllerFaceChannels
	case def.Metadata.IsDenseCable:
		def.Metadata.ChannelCapacity = profile.Config.DenseCableChannels
	case def.Metadata.TransmitsChannels && def.Metadata.ChannelCapacity > 0:
		def.Metadata.ChannelCapacity = profile.Config.NormalCableChannels
	}

	return def
}

func cloneProfile(p domain.CompatibilityProfile) domain.CompatibilityProfile {
	out := p
	if p.Config.CellTiers != nil {
		out.Config.CellTiers = maps.Clone(p.Config.CellTiers)
	}
	if p.Config.References != nil {
		out.Config.References = slices.Clone(p.Config.References)
	}
	return out
}

func cloneBlock(b domain.BlockDefinition) domain.BlockDefinition {
	out := b
	if b.Metadata.ConnectionRules.AllowedCategories != nil {
		out.Metadata.ConnectionRules.AllowedCategories = slices.Clone(b.Metadata.ConnectionRules.AllowedCategories)
	}
	if b.Metadata.ConfigSchema != nil {
		schema := make([]domain.ConfigFieldSchema, len(b.Metadata.ConfigSchema))
		for i, field := range b.Metadata.ConfigSchema {
			f := field
			if field.Options != nil {
				f.Options = slices.Clone(field.Options)
			}
			schema[i] = f
		}
		out.Metadata.ConfigSchema = schema
	}
	return out
}
