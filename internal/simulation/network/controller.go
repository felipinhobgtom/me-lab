package network

import (
	"fmt"

	"github.com/felipinho/me-lab/internal/domain"
	"github.com/felipinho/me-lab/internal/simulation/diagnostics"
	"github.com/felipinho/me-lab/internal/simulation/world"
)

func validateControllers(simID string, w *world.World, net *Network, maxSpan int) []domain.Diagnostic {
	if len(net.Controllers) == 0 {
		net.HasController = false
		net.ControllerValid = false
		net.IsAdHoc = true
		net.ExposedControllerFaces = 0
		return nil
	}

	net.HasController = true
	net.IsAdHoc = false

	ctrlBlocks := make([]domain.PlacedBlock, len(net.Controllers))
	for i, c := range net.Controllers {
		ctrlBlocks[i] = c.Block
	}

	diags := make([]domain.Diagnostic, 0)
	valid := true

	// 1. All controller blocks on the network must form 1 connected component among controller blocks.
	visited := make(map[domain.Coord3D]bool, len(net.Controllers))
	queue := []domain.Coord3D{net.Controllers[0].Coord}
	visited[net.Controllers[0].Coord] = true
	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]
		for _, dir := range domain.AllDirections() {
			adj := curr.Adjacent(dir)
			if _, isCtrl := net.ControllerSet[adj]; !isCtrl || visited[adj] {
				continue
			}
			if w.Connected(curr, adj) {
				visited[adj] = true
				queue = append(queue, adj)
			}
		}
	}
	if len(visited) < len(net.Controllers) {
		valid = false
		diags = append(diags,
			diagnostics.NewMultipleControllersDisconnected(simID, ctrlBlocks),
			diagnostics.NewInvalidControllerMultiblock(simID, ctrlBlocks, "multiple disconnected ME Controller clusters exist on the same ME network"),
		)
	}

	// 2. Bounding box along X, Y, Z must not exceed maxSpan (7 in AE2 1.21.1).
	minX, maxX := net.Controllers[0].Coord.X, net.Controllers[0].Coord.X
	minY, maxY := net.Controllers[0].Coord.Y, net.Controllers[0].Coord.Y
	minZ, maxZ := net.Controllers[0].Coord.Z, net.Controllers[0].Coord.Z
	for _, c := range net.Controllers[1:] {
		if c.Coord.X < minX {
			minX = c.Coord.X
		}
		if c.Coord.X > maxX {
			maxX = c.Coord.X
		}
		if c.Coord.Y < minY {
			minY = c.Coord.Y
		}
		if c.Coord.Y > maxY {
			maxY = c.Coord.Y
		}
		if c.Coord.Z < minZ {
			minZ = c.Coord.Z
		}
		if c.Coord.Z > maxZ {
			maxZ = c.Coord.Z
		}
	}
	spanX := maxX - minX + 1
	spanY := maxY - minY + 1
	spanZ := maxZ - minZ + 1
	if spanX > maxSpan || spanY > maxSpan || spanZ > maxSpan {
		valid = false
		diags = append(diags, diagnostics.NewInvalidControllerMultiblock(
			simID,
			ctrlBlocks,
			fmt.Sprintf("bounding box %dx%dx%d exceeds maximum allowed %dx%dx%d", spanX, spanY, spanZ, maxSpan, maxSpan, maxSpan),
		))
	}

	// 3. Cross/axis rule: no controller block may have controller neighbors on BOTH sides along > 1 axis.
	var axisViolators []domain.PlacedBlock
	isConnectedCtrl := func(from domain.Coord3D, dir domain.Direction) bool {
		adj := from.Adjacent(dir)
		if _, ok := net.ControllerSet[adj]; !ok {
			return false
		}
		return w.Connected(from, adj)
	}

	exposedFaces := 0
	for _, c := range net.Controllers {
		pos := c.Coord
		ns := isConnectedCtrl(pos, domain.North) && isConnectedCtrl(pos, domain.South)
		ew := isConnectedCtrl(pos, domain.East) && isConnectedCtrl(pos, domain.West)
		ud := isConnectedCtrl(pos, domain.Up) && isConnectedCtrl(pos, domain.Down)

		axes := 0
		if ns {
			axes++
		}
		if ew {
			axes++
		}
		if ud {
			axes++
		}
		if axes > 1 {
			axisViolators = append(axisViolators, c.Block)
		}

		for _, dir := range domain.AllDirections() {
			if c.AnchoredFaces[dir] {
				continue
			}
			if !isConnectedCtrl(pos, dir) {
				exposedFaces++
			}
		}
	}

	if len(axisViolators) > 0 {
		valid = false
		diags = append(diags, diagnostics.NewInvalidControllerMultiblock(
			simID,
			ctrlBlocks,
			"controller block has opposite controller neighbors along more than 1 axis (cross/filled-plane shape forbidden)",
		))
	}

	net.ControllerValid = valid
	if valid {
		net.ExposedControllerFaces = exposedFaces
	} else {
		net.ExposedControllerFaces = 0
	}
	return diags
}
