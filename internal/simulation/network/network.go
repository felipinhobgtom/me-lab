package network

import (
	"fmt"
	"sort"

	"github.com/felipinho/me-lab/internal/domain"
	"github.com/felipinho/me-lab/internal/simulation/diagnostics"
	"github.com/felipinho/me-lab/internal/simulation/world"
)

// CraftingCPU represents a validated rectangular-prism Crafting CPU multiblock on an ME network.
type CraftingCPU struct {
	ID           string
	NetworkID    string
	Anchor       *world.Node
	Members      []*world.Node
	MemberSet    map[domain.Coord3D]bool
	TotalBytes   int
	CoProcessors int
	HasChannel   bool
}

// Blocks returns the PlacedBlocks comprising this Crafting CPU in canonical order.
func (c *CraftingCPU) Blocks() []domain.PlacedBlock {
	out := make([]domain.PlacedBlock, len(c.Members))
	for i, m := range c.Members {
		out[i] = m.Block
	}
	return out
}

// Network represents a connected ME component discovered via deterministic BFS.
type Network struct {
	ID                     string
	Nodes                  []*world.Node
	NodeSet                map[domain.Coord3D]*world.Node
	Controllers            []*world.Node
	ControllerSet          map[domain.Coord3D]*world.Node
	HasController          bool
	ControllerValid        bool
	IsAdHoc                bool
	ExposedControllerFaces int
	CraftingCPUs           []*CraftingCPU
	AllCPUClusterMemberSet map[domain.Coord3D]bool
}

// Blocks returns all PlacedBlocks in this network in canonical order.
func (n *Network) Blocks() []domain.PlacedBlock {
	out := make([]domain.PlacedBlock, len(n.Nodes))
	for i, node := range n.Nodes {
		out[i] = node.Block
	}
	return out
}

// Discover partitions ME nodes in w into deterministic connected ME networks and validates
// controller and Crafting CPU multiblocks within each network.
func Discover(simID string, w *world.World, profile domain.CompatibilityProfile) ([]*Network, []domain.Diagnostic) {
	visited := make(map[domain.Coord3D]bool, len(w.MENodes))
	rawComponents := make([][]*world.Node, 0)

	for _, start := range w.MENodes {
		if visited[start.Coord] {
			continue
		}
		comp := make([]*world.Node, 0, 8)
		queue := []domain.Coord3D{start.Coord}
		visited[start.Coord] = true

		for len(queue) > 0 {
			curr := queue[0]
			queue = queue[1:]
			node := w.ByCoord[curr]
			if node != nil {
				comp = append(comp, node)
			}
			for _, nb := range w.Adj[curr] {
				if !visited[nb] {
					visited[nb] = true
					queue = append(queue, nb)
				}
			}
		}

		sort.Slice(comp, func(i, j int) bool {
			if cmp := domain.CompareCoords(comp[i].Coord, comp[j].Coord); cmp != 0 {
				return cmp < 0
			}
			return comp[i].Block.ID < comp[j].Block.ID
		})
		rawComponents = append(rawComponents, comp)
	}

	sort.Slice(rawComponents, func(i, j int) bool {
		return domain.CompareCoords(rawComponents[i][0].Coord, rawComponents[j][0].Coord) < 0
	})

	maxSpan := profile.Config.ControllerMaxSpan
	if maxSpan <= 0 {
		maxSpan = 7
	}

	networks := make([]*Network, 0, len(rawComponents))
	allDiags := make([]domain.Diagnostic, 0)
	nextCPUIndex := 0

	for idx, comp := range rawComponents {
		net := &Network{
			ID:                     fmt.Sprintf("net-%d", idx+1),
			Nodes:                  comp,
			NodeSet:                make(map[domain.Coord3D]*world.Node, len(comp)),
			Controllers:            make([]*world.Node, 0),
			ControllerSet:          make(map[domain.Coord3D]*world.Node),
			AllCPUClusterMemberSet: make(map[domain.Coord3D]bool),
		}
		for _, n := range comp {
			net.NodeSet[n.Coord] = n
			if n.IsController {
				net.Controllers = append(net.Controllers, n)
				net.ControllerSet[n.Coord] = n
			}
			if n.IsCPUBlock {
				net.AllCPUClusterMemberSet[n.Coord] = true
			}
		}

		ctrlDiags := validateControllers(simID, w, net, maxSpan)
		allDiags = append(allDiags, ctrlDiags...)

		cpus, cpuDiags := discoverCraftingCPUs(simID, w, net, &nextCPUIndex)
		net.CraftingCPUs = cpus
		allDiags = append(allDiags, cpuDiags...)

		networks = append(networks, net)
	}

	return networks, diagnostics.DeduplicateAndSort(allDiags)
}
