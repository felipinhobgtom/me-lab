package network

import (
	"fmt"
	"sort"

	"github.com/felipinho/me-lab/internal/domain"
	"github.com/felipinho/me-lab/internal/simulation/diagnostics"
	"github.com/felipinho/me-lab/internal/simulation/world"
)

func discoverCraftingCPUs(simID string, w *world.World, net *Network, nextCPUIndex *int) ([]*CraftingCPU, []domain.Diagnostic) {
	cpuNodes := make([]*world.Node, 0)
	cpuNodeMap := make(map[domain.Coord3D]*world.Node)
	for _, n := range net.Nodes {
		if n.IsCPUBlock {
			cpuNodes = append(cpuNodes, n)
			cpuNodeMap[n.Coord] = n
		}
	}
	if len(cpuNodes) == 0 {
		return nil, nil
	}

	visited := make(map[domain.Coord3D]bool, len(cpuNodes))
	clusters := make([][]*world.Node, 0)

	for _, start := range cpuNodes {
		if visited[start.Coord] {
			continue
		}
		cluster := make([]*world.Node, 0, 4)
		queue := []domain.Coord3D{start.Coord}
		visited[start.Coord] = true

		for len(queue) > 0 {
			curr := queue[0]
			queue = queue[1:]
			cluster = append(cluster, cpuNodeMap[curr])

			for _, dir := range domain.AllDirections() {
				adj := curr.Adjacent(dir)
				if _, ok := cpuNodeMap[adj]; !ok || visited[adj] {
					continue
				}
				if w.Connected(curr, adj) {
					visited[adj] = true
					queue = append(queue, adj)
				}
			}
		}

		sort.Slice(cluster, func(i, j int) bool {
			if cmp := domain.CompareCoords(cluster[i].Coord, cluster[j].Coord); cmp != 0 {
				return cmp < 0
			}
			return cluster[i].Block.ID < cluster[j].Block.ID
		})
		clusters = append(clusters, cluster)
	}

	sort.Slice(clusters, func(i, j int) bool {
		return domain.CompareCoords(clusters[i][0].Coord, clusters[j][0].Coord) < 0
	})

	validCPUs := make([]*CraftingCPU, 0, len(clusters))
	diags := make([]domain.Diagnostic, 0)

	for _, cluster := range clusters {
		blocks := make([]domain.PlacedBlock, len(cluster))
		minX, maxX := cluster[0].Coord.X, cluster[0].Coord.X
		minY, maxY := cluster[0].Coord.Y, cluster[0].Coord.Y
		minZ, maxZ := cluster[0].Coord.Z, cluster[0].Coord.Z

		totalBytes := 0
		coProcs := 0
		memberSet := make(map[domain.Coord3D]bool, len(cluster))

		for i, n := range cluster {
			blocks[i] = n.Block
			memberSet[n.Coord] = true
			if n.Coord.X < minX {
				minX = n.Coord.X
			}
			if n.Coord.X > maxX {
				maxX = n.Coord.X
			}
			if n.Coord.Y < minY {
				minY = n.Coord.Y
			}
			if n.Coord.Y > maxY {
				maxY = n.Coord.Y
			}
			if n.Coord.Z < minZ {
				minZ = n.Coord.Z
			}
			if n.Coord.Z > maxZ {
				maxZ = n.Coord.Z
			}
			totalBytes += n.Def.Metadata.CraftingStorageBytes
			if n.Def.Metadata.IsCoProcessor {
				coProcs++
			}
		}

		dx := maxX - minX + 1
		dy := maxY - minY + 1
		dz := maxZ - minZ + 1
		expectedVol := dx * dy * dz

		if expectedVol != len(cluster) {
			diags = append(diags, diagnostics.NewInvalidCraftingCPUMultiblock(
				simID,
				blocks,
				fmt.Sprintf("cluster has %d blocks inside %dx%dx%d bounding box (expected %d blocks for a solid rectangular prism)", len(cluster), dx, dy, dz, expectedVol),
			))
			continue
		}

		if totalBytes <= 0 {
			diags = append(diags, diagnostics.NewNoValidCraftingCPU(
				simID,
				blocks,
				domain.SeverityWarning,
				"Crafting CPU multiblock has 0 bytes of crafting storage",
			))
			continue
		}

		*nextCPUIndex++
		validCPUs = append(validCPUs, &CraftingCPU{
			ID:           fmt.Sprintf("cpu-%d", *nextCPUIndex),
			NetworkID:    net.ID,
			Anchor:       cluster[0],
			Members:      cluster,
			MemberSet:    memberSet,
			TotalBytes:   totalBytes,
			CoProcessors: coProcs,
		})
	}

	return validCPUs, diags
}
