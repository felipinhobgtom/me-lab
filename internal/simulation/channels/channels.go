package channels

import (
	"container/heap"
	"sort"

	"github.com/felipinho/me-lab/internal/domain"
	"github.com/felipinho/me-lab/internal/simulation/diagnostics"
	"github.com/felipinho/me-lab/internal/simulation/network"
	"github.com/felipinho/me-lab/internal/simulation/world"
)

// NetworkChannels summarizes channel allocation metrics for a single ME network.
type NetworkChannels struct {
	NetworkID         string
	ChannelsUsed      int
	ChannelsAvailable int
	AdHocOverloaded   bool
	TotalHops         int
}

// Allocation holds complete channel routing results across all networks in a World.
type Allocation struct {
	ByNetwork           map[string]*NetworkChannels
	DeviceRoutes        []domain.DeviceRouteOverlay
	HasChannelByBlockID map[string]bool
	CableOverlays       []domain.CableChannelOverlay
	Diagnostics         []domain.Diagnostic
}

type channelTarget struct {
	Block  domain.PlacedBlock
	Node   *world.Node
	Coord  domain.Coord3D
	Usage  int
	CPU    *network.CraftingCPU
	Uncost int64
	Unpath []domain.Coord3D
	HasTop bool
}

type ctrlFaceKey struct {
	coord domain.Coord3D
	face  domain.Direction
}

// Allocate executes AE2 channel assignment across all networks (infinite, ad-hoc, and 3-tier controller routing).
func Allocate(
	simID string,
	w *world.World,
	nets []*network.Network,
	profile domain.CompatibilityProfile,
) *Allocation {
	normalCap := profile.Config.NormalCableChannels
	if normalCap <= 0 {
		normalCap = 8
	}
	denseCap := profile.Config.DenseCableChannels
	if denseCap <= 0 {
		denseCap = 32
	}
	faceCap := profile.Config.ControllerFaceChannels
	if faceCap <= 0 {
		faceCap = 32
	}
	adHocLimit := profile.Config.AdHocChannelLimit
	if adHocLimit <= 0 {
		adHocLimit = 8
	}
	infinite := profile.Config.InfiniteChannels

	alloc := &Allocation{
		ByNetwork:           make(map[string]*NetworkChannels, len(nets)),
		DeviceRoutes:        make([]domain.DeviceRouteOverlay, 0),
		HasChannelByBlockID: make(map[string]bool),
		CableOverlays:       make([]domain.CableChannelOverlay, 0),
		Diagnostics:         make([]domain.Diagnostic, 0),
	}

	nodeCapacity := func(n *world.Node) int {
		if n.IsCable {
			if n.IsDenseCable {
				return denseCap
			}
			return normalCap
		}
		if n.TransmitsChannels() {
			return normalCap
		}
		return 0
	}

	for _, net := range nets {
		nc := &NetworkChannels{
			NetworkID: net.ID,
		}
		alloc.ByNetwork[net.ID] = nc

		targets := collectTargets(net)
		usedByCoord := make(map[domain.Coord3D]int, len(net.Nodes))
		overloadedByCoord := make(map[domain.Coord3D]bool)

		if net.HasController && !net.ControllerValid {
			// Invalid controller multiblock shuts down channel allocation on this network.
			nc.ChannelsAvailable = 0
			for _, t := range targets {
				markTargetChannel(alloc, t, false)
				alloc.DeviceRoutes = append(alloc.DeviceRoutes, domain.DeviceRouteOverlay{
					BlockID:    t.Block.ID,
					Coord:      t.Coord,
					NetworkID:  net.ID,
					HasChannel: false,
				})
				alloc.Diagnostics = append(alloc.Diagnostics,
					diagnostics.NewDeviceNoChannel(simID, t.Block, "ME Controller multiblock on this network is invalid"),
				)
			}
			appendNetworkCableOverlays(alloc, w, net, usedByCoord, overloadedByCoord, normalCap, denseCap, false, adHocLimit, infinite)
			continue
		}

		if net.IsAdHoc {
			allocateAdHoc(simID, w, net, nc, targets, alloc, usedByCoord, overloadedByCoord, normalCap, denseCap, adHocLimit, infinite)
			continue
		}

		// Valid controller-based network.
		nc.ChannelsAvailable = net.ExposedControllerFaces * faceCap
		allocateControllerNetwork(simID, w, net, nc, targets, alloc, usedByCoord, overloadedByCoord, nodeCapacity, normalCap, denseCap, faceCap, infinite)
	}

	sort.Slice(alloc.DeviceRoutes, func(i, j int) bool {
		if cmp := domain.CompareCoords(alloc.DeviceRoutes[i].Coord, alloc.DeviceRoutes[j].Coord); cmp != 0 {
			return cmp < 0
		}
		return alloc.DeviceRoutes[i].BlockID < alloc.DeviceRoutes[j].BlockID
	})
	sort.Slice(alloc.CableOverlays, func(i, j int) bool {
		if cmp := domain.CompareCoords(alloc.CableOverlays[i].Coord, alloc.CableOverlays[j].Coord); cmp != 0 {
			return cmp < 0
		}
		return alloc.CableOverlays[i].BlockID < alloc.CableOverlays[j].BlockID
	})
	alloc.Diagnostics = diagnostics.DeduplicateAndSort(alloc.Diagnostics)
	return alloc
}

func collectTargets(net *network.Network) []*channelTarget {
	targets := make([]*channelTarget, 0)
	for _, n := range net.Nodes {
		if net.AllCPUClusterMemberSet[n.Coord] {
			continue
		}
		usage := n.EffectiveChannelUsage()
		if usage > 0 {
			targets = append(targets, &channelTarget{
				Block: n.Block,
				Node:  n,
				Coord: n.Coord,
				Usage: usage,
			})
		}
	}
	for _, cpu := range net.CraftingCPUs {
		targets = append(targets, &channelTarget{
			Block: cpu.Anchor.Block,
			Node:  cpu.Anchor,
			Coord: cpu.Anchor.Coord,
			Usage: 1,
			CPU:   cpu,
		})
	}
	return targets
}

func markTargetChannel(alloc *Allocation, t *channelTarget, has bool) {
	if t.CPU != nil {
		t.CPU.HasChannel = has
		for _, m := range t.CPU.Members {
			if m.Block.ID != "" {
				alloc.HasChannelByBlockID[m.Block.ID] = has
			}
		}
		return
	}
	if t.Block.ID != "" {
		alloc.HasChannelByBlockID[t.Block.ID] = has
	}
}

func allocateAdHoc(
	simID string,
	w *world.World,
	net *network.Network,
	nc *NetworkChannels,
	targets []*channelTarget,
	alloc *Allocation,
	usedByCoord map[domain.Coord3D]int,
	overloadedByCoord map[domain.Coord3D]bool,
	normalCap int,
	denseCap int,
	adHocLimit int,
	infinite bool,
) {
	sort.Slice(targets, func(i, j int) bool {
		if cmp := domain.CompareCoords(targets[i].Coord, targets[j].Coord); cmp != 0 {
			return cmp < 0
		}
		return targets[i].Block.ID < targets[j].Block.ID
	})

	totalRequired := 0
	for _, t := range targets {
		totalRequired += t.Usage
	}

	nc.ChannelsUsed = totalRequired
	if infinite {
		if totalRequired > adHocLimit {
			nc.ChannelsAvailable = totalRequired
		} else {
			nc.ChannelsAvailable = adHocLimit
		}
	} else {
		nc.ChannelsAvailable = adHocLimit
	}

	// Pick canonical anchor root on the ad-hoc network (first transmitting block, or first node).
	rootCoord := net.Nodes[0].Coord
	for _, n := range net.Nodes {
		if n.IsCable || n.TransmitsChannels() {
			rootCoord = n.Coord
			break
		}
	}

	if !infinite && totalRequired > adHocLimit {
		nc.AdHocOverloaded = true
		targetBlocks := make([]domain.PlacedBlock, len(targets))
		for i, t := range targets {
			targetBlocks[i] = t.Block
			markTargetChannel(alloc, t, false)
			alloc.DeviceRoutes = append(alloc.DeviceRoutes, domain.DeviceRouteOverlay{
				BlockID:    t.Block.ID,
				Coord:      t.Coord,
				NetworkID:  net.ID,
				HasChannel: false,
			})
			alloc.Diagnostics = append(alloc.Diagnostics,
				diagnostics.NewDeviceNoChannel(simID, t.Block, "ad-hoc network exceeded channel limit without an ME Controller"),
			)
		}
		alloc.Diagnostics = append(alloc.Diagnostics,
			diagnostics.NewNetworkNoController(simID, net.ID, targetBlocks, totalRequired, adHocLimit),
		)
		for _, n := range net.Nodes {
			if n.IsCable {
				usedByCoord[n.Coord] = totalRequired
				overloadedByCoord[n.Coord] = true
			}
		}
		appendNetworkCableOverlays(alloc, w, net, usedByCoord, overloadedByCoord, normalCap, denseCap, true, adHocLimit, infinite)
		return
	}

	// Ad-hoc within limit (or infinite channels): all reachable targets receive channels,
	// and cables (especially smart cables) report network-wide channels used!
	for _, t := range targets {
		route := bfsAdHocRoute(w, net, t, rootCoord)
		if route == nil {
			// Isolated behind non-transmitting terminals.
			markTargetChannel(alloc, t, false)
			alloc.DeviceRoutes = append(alloc.DeviceRoutes, domain.DeviceRouteOverlay{
				BlockID:    t.Block.ID,
				Coord:      t.Coord,
				NetworkID:  net.ID,
				HasChannel: false,
			})
			alloc.Diagnostics = append(alloc.Diagnostics,
				diagnostics.NewDeviceNoChannel(simID, t.Block, "no channel-transmitting path on ad-hoc network"),
			)
			continue
		}
		markTargetChannel(alloc, t, true)
		if len(route) > 1 {
			nc.TotalHops += (len(route) - 1) * t.Usage
		}
		alloc.DeviceRoutes = append(alloc.DeviceRoutes, domain.DeviceRouteOverlay{
			BlockID:    t.Block.ID,
			Coord:      t.Coord,
			NetworkID:  net.ID,
			HasChannel: true,
			Route:      route,
		})
	}

	for _, n := range net.Nodes {
		if n.IsCable {
			usedByCoord[n.Coord] = totalRequired
		}
	}
	appendNetworkCableOverlays(alloc, w, net, usedByCoord, overloadedByCoord, normalCap, denseCap, true, adHocLimit, infinite)
}

func bfsAdHocRoute(w *world.World, net *network.Network, t *channelTarget, root domain.Coord3D) []domain.Coord3D {
	if t.Coord == root {
		return []domain.Coord3D{t.Coord}
	}
	prev := make(map[domain.Coord3D]domain.Coord3D)
	visited := map[domain.Coord3D]bool{t.Coord: true}
	queue := []domain.Coord3D{t.Coord}

	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]
		if curr == root {
			return reconstructPath(t.Coord, root, prev)
		}
		for _, nb := range w.Adj[curr] {
			if visited[nb] {
				continue
			}
			nbNode, ok := net.NodeSet[nb]
			if !ok {
				continue
			}
			if nb != root && !nbNode.TransmitsChannels() {
				continue
			}
			visited[nb] = true
			prev[nb] = curr
			queue = append(queue, nb)
		}
	}
	return nil
}

func allocateControllerNetwork(
	simID string,
	w *world.World,
	net *network.Network,
	nc *NetworkChannels,
	targets []*channelTarget,
	alloc *Allocation,
	usedByCoord map[domain.Coord3D]int,
	overloadedByCoord map[domain.Coord3D]bool,
	nodeCapacity func(*world.Node) int,
	normalCap int,
	denseCap int,
	faceCap int,
	infinite bool,
) {
	ctrlFaceUsed := make(map[ctrlFaceKey]int)

	// Precompute topological 3-tier distance from each target to Controller so closer devices claim channels first.
	for _, t := range targets {
		route, cost, ok := findRouteToController(w, net, t, usedByCoord, ctrlFaceUsed, nodeCapacity, faceCap, false)
		t.Unpath = route
		t.Uncost = cost
		t.HasTop = ok
	}

	sort.Slice(targets, func(i, j int) bool {
		if targets[i].HasTop != targets[j].HasTop {
			return targets[i].HasTop
		}
		if targets[i].Uncost != targets[j].Uncost {
			return targets[i].Uncost < targets[j].Uncost
		}
		if len(targets[i].Unpath) != len(targets[j].Unpath) {
			return len(targets[i].Unpath) < len(targets[j].Unpath)
		}
		if cmp := domain.CompareCoords(targets[i].Coord, targets[j].Coord); cmp != 0 {
			return cmp < 0
		}
		return targets[i].Block.ID < targets[j].Block.ID
	})

	bottleneckReported := make(map[domain.Coord3D]bool)

	for _, t := range targets {
		if !t.HasTop {
			markTargetChannel(alloc, t, false)
			alloc.DeviceRoutes = append(alloc.DeviceRoutes, domain.DeviceRouteOverlay{
				BlockID:    t.Block.ID,
				Coord:      t.Coord,
				NetworkID:  net.ID,
				HasChannel: false,
			})
			alloc.Diagnostics = append(alloc.Diagnostics,
				diagnostics.NewDeviceNoChannel(simID, t.Block, "no channel-transmitting path to ME Controller"),
			)
			continue
		}

		if infinite {
			markTargetChannel(alloc, t, true)
			recordRouteUsage(net, t, t.Unpath, usedByCoord, ctrlFaceUsed)
			nc.ChannelsUsed += t.Usage
			nc.TotalHops += (len(t.Unpath) - 1) * t.Usage
			alloc.DeviceRoutes = append(alloc.DeviceRoutes, domain.DeviceRouteOverlay{
				BlockID:    t.Block.ID,
				Coord:      t.Coord,
				NetworkID:  net.ID,
				HasChannel: true,
				Route:      t.Unpath,
			})
			continue
		}

		route, _, ok := findRouteToController(w, net, t, usedByCoord, ctrlFaceUsed, nodeCapacity, faceCap, true)
		if ok {
			markTargetChannel(alloc, t, true)
			recordRouteUsage(net, t, route, usedByCoord, ctrlFaceUsed)
			nc.ChannelsUsed += t.Usage
			nc.TotalHops += (len(route) - 1) * t.Usage
			alloc.DeviceRoutes = append(alloc.DeviceRoutes, domain.DeviceRouteOverlay{
				BlockID:    t.Block.ID,
				Coord:      t.Coord,
				NetworkID:  net.ID,
				HasChannel: true,
				Route:      route,
			})
			continue
		}

		// Topological path exists, but all paths are capacity-saturated!
		markTargetChannel(alloc, t, false)
		alloc.DeviceRoutes = append(alloc.DeviceRoutes, domain.DeviceRouteOverlay{
			BlockID:    t.Block.ID,
			Coord:      t.Coord,
			NetworkID:  net.ID,
			HasChannel: false,
		})
		alloc.Diagnostics = append(alloc.Diagnostics,
			diagnostics.NewDeviceNoChannel(simID, t.Block, "all paths to ME Controller are saturated at channel capacity"),
		)

		// Identify bottleneck block(s) along t.Unpath that are at full capacity.
		for idx := range len(t.Unpath) - 1 {
			c := t.Unpath[idx]
			n := net.NodeSet[c]
			if n == nil {
				continue
			}
			capN := nodeCapacity(n)
			if capN > 0 && usedByCoord[c]+t.Usage > capN {
				overloadedByCoord[c] = true
				if !bottleneckReported[c] {
					bottleneckReported[c] = true
					alloc.Diagnostics = append(alloc.Diagnostics,
						diagnostics.NewChannelCapacityExceeded(simID, []domain.PlacedBlock{n.Block}, usedByCoord[c], capN),
					)
				}
			}
		}

		// Also check if the controller entry face itself was saturated (e.g., 32/32 on Dense Cable into 1 controller face).
		if len(t.Unpath) >= 2 {
			prevCoord := t.Unpath[len(t.Unpath)-2]
			ctrlCoord := t.Unpath[len(t.Unpath)-1]
			if faceDir, adjOK := ctrlCoord.DirectionTo(prevCoord); adjOK {
				fk := ctrlFaceKey{coord: ctrlCoord, face: faceDir}
				if ctrlFaceUsed[fk]+t.Usage > faceCap {
					overloadedByCoord[prevCoord] = true
					if !bottleneckReported[ctrlCoord] {
						bottleneckReported[ctrlCoord] = true
						prevNode := net.NodeSet[prevCoord]
						ctrlNode := net.NodeSet[ctrlCoord]
						var bBlocks []domain.PlacedBlock
						if prevNode != nil {
							bBlocks = append(bBlocks, prevNode.Block)
						}
						if ctrlNode != nil {
							bBlocks = append(bBlocks, ctrlNode.Block)
						}
						alloc.Diagnostics = append(alloc.Diagnostics,
							diagnostics.NewChannelCapacityExceeded(simID, bBlocks, ctrlFaceUsed[fk], faceCap),
						)
					}
				}
			}
		}
	}

	if infinite && nc.ChannelsUsed > nc.ChannelsAvailable {
		nc.ChannelsAvailable = nc.ChannelsUsed
	}

	appendNetworkCableOverlays(alloc, w, net, usedByCoord, overloadedByCoord, normalCap, denseCap, false, 0, infinite)
}

func recordRouteUsage(
	net *network.Network,
	t *channelTarget,
	route []domain.Coord3D,
	usedByCoord map[domain.Coord3D]int,
	ctrlFaceUsed map[ctrlFaceKey]int,
) {
	for i := range len(route) - 1 {
		c := route[i]
		n := net.NodeSet[c]
		if n == nil || !n.TransmitsChannels() {
			continue
		}
		// Count a Crafting CPU's internal member blocks once (not double-charging if internal hops occurred).
		if t.CPU != nil && t.CPU.MemberSet[c] && i < len(route)-1 && t.CPU.MemberSet[route[i+1]] {
			continue
		}
		usedByCoord[c] += t.Usage
	}
	if len(route) >= 2 {
		prevCoord := route[len(route)-2]
		ctrlCoord := route[len(route)-1]
		if faceDir, ok := ctrlCoord.DirectionTo(prevCoord); ok {
			ctrlFaceUsed[ctrlFaceKey{coord: ctrlCoord, face: faceDir}] += t.Usage
		}
	}
}

func appendNetworkCableOverlays(
	alloc *Allocation,
	w *world.World,
	net *network.Network,
	usedByCoord map[domain.Coord3D]int,
	overloadedByCoord map[domain.Coord3D]bool,
	normalCap int,
	denseCap int,
	isAdHoc bool,
	adHocLimit int,
	infinite bool,
) {
	for _, n := range net.Nodes {
		if !n.IsCable {
			continue
		}
		capVal := normalCap
		if n.IsDenseCable {
			capVal = denseCap
		}
		if isAdHoc && !infinite && adHocLimit < capVal {
			capVal = adHocLimit
		}
		conns := make([]domain.Coord3D, len(w.Adj[n.Coord]))
		copy(conns, w.Adj[n.Coord])

		over := overloadedByCoord[n.Coord]
		if infinite {
			over = false
		}

		alloc.CableOverlays = append(alloc.CableOverlays, domain.CableChannelOverlay{
			BlockID:      n.Block.ID,
			Coord:        n.Coord,
			NetworkID:    net.ID,
			ChannelsUsed: usedByCoord[n.Coord],
			Capacity:     capVal,
			Overloaded:   over,
			Online:       true,
			Color:        n.CableColor,
			Connections:  conns,
		})
	}
}

// Tiered shortest-path Dijkstra from target to any ME Controller face.
func findRouteToController(
	w *world.World,
	net *network.Network,
	t *channelTarget,
	usedByCoord map[domain.Coord3D]int,
	ctrlFaceUsed map[ctrlFaceKey]int,
	nodeCapacity func(*world.Node) int,
	faceCap int,
	enforceCap bool,
) ([]domain.Coord3D, int64, bool) {
	startNode := net.NodeSet[t.Coord]
	if startNode == nil {
		return nil, 0, false
	}

	// If target itself is a channel-transmitting machine (e.g. Drive), it must have remaining capacity for its own channel.
	if enforceCap && startNode.TransmitsChannels() {
		if !(t.CPU != nil && len(t.CPU.Members) > 1) {
			capStart := nodeCapacity(startNode)
			if capStart > 0 && usedByCoord[t.Coord]+t.Usage > capStart {
				return nil, 0, false
			}
		}
	}

	dist := map[domain.Coord3D]int64{t.Coord: 0}
	hops := map[domain.Coord3D]int{t.Coord: 0}
	prev := make(map[domain.Coord3D]domain.Coord3D)

	pq := &dijkstraHeap{
		{coord: t.Coord, cost: 0, hops: 0},
	}
	heap.Init(pq)

	for pq.Len() > 0 {
		item := heap.Pop(pq).(dijkstraItem)
		curr := item.coord

		if best, ok := dist[curr]; ok && (item.cost > best || (item.cost == best && item.hops > hops[curr])) {
			continue
		}

		currNode := net.NodeSet[curr]
		if currNode == nil {
			continue
		}
		if currNode.IsController && curr != t.Coord {
			return reconstructPath(t.Coord, curr, prev), item.cost, true
		}

		currTier := 0
		if curr != t.Coord {
			currTier = nodeTier(currNode)
		}

		for _, nb := range w.Adj[curr] {
			nbNode, ok := net.NodeSet[nb]
			if !ok {
				continue
			}

			var stepCost int64
			if nbNode.IsController {
				faceDir, adjOK := nb.DirectionTo(curr)
				if !adjOK {
					continue
				}
				if enforceCap && ctrlFaceUsed[ctrlFaceKey{coord: nb, face: faceDir}]+t.Usage > faceCap {
					continue
				}
				// If transitioning out of CPU anchor/member, ensure that exiting CPU block has capacity.
				if enforceCap && t.CPU != nil && t.CPU.MemberSet[curr] {
					capCurr := nodeCapacity(currNode)
					if capCurr > 0 && usedByCoord[curr]+t.Usage > capCurr {
						continue
					}
				}
				stepCost = 1
			} else {
				if !nbNode.TransmitsChannels() {
					continue
				}
				internalCPUHop := t.CPU != nil && t.CPU.MemberSet[curr] && t.CPU.MemberSet[nb]
				if enforceCap && !internalCPUHop {
					// When leaving CPU into external node, check exiting CPU member capacity too.
					if t.CPU != nil && t.CPU.MemberSet[curr] {
						capCurr := nodeCapacity(currNode)
						if capCurr > 0 && usedByCoord[curr]+t.Usage > capCurr {
							continue
						}
					}
					capNB := nodeCapacity(nbNode)
					if capNB > 0 && usedByCoord[nb]+t.Usage > capNB {
						continue
					}
				}

				if internalCPUHop {
					stepCost = 1
				} else {
					nbTier := nodeTier(nbNode)
					stepCost = baseTierStepCost(nbTier)
					if currTier > 0 && nbTier < currTier {
						stepCost += 10_000_000
					}
				}
			}

			newCost := item.cost + stepCost
			newHops := item.hops + 1

			oldCost, seen := dist[nb]
			if !seen || newCost < oldCost || (newCost == oldCost && (newHops < hops[nb] || (newHops == hops[nb] && domain.CompareCoords(curr, prev[nb]) < 0))) {
				dist[nb] = newCost
				hops[nb] = newHops
				prev[nb] = curr
				heap.Push(pq, dijkstraItem{coord: nb, cost: newCost, hops: newHops})
			}
		}
	}

	return nil, 0, false
}

func nodeTier(n *world.Node) int {
	if n.IsController {
		return 4
	}
	if n.IsCable && n.IsDenseCable {
		return 3
	}
	if n.IsCable {
		return 2
	}
	return 1
}

func baseTierStepCost(tier int) int64 {
	switch tier {
	case 3:
		return 10 // Dense cable trunk
	case 2:
		return 1_000 // Normal / smart cable branch
	default:
		return 100_000 // Channel-transmitting full-block machine
	}
}

func reconstructPath(start, goal domain.Coord3D, prev map[domain.Coord3D]domain.Coord3D) []domain.Coord3D {
	rev := make([]domain.Coord3D, 0, 8)
	for c := goal; ; {
		rev = append(rev, c)
		if c == start {
			break
		}
		p, ok := prev[c]
		if !ok {
			return nil
		}
		c = p
	}
	for i, j := 0, len(rev)-1; i < j; i, j = i+1, j-1 {
		rev[i], rev[j] = rev[j], rev[i]
	}
	return rev
}

type dijkstraItem struct {
	coord domain.Coord3D
	cost  int64
	hops  int
}

type dijkstraHeap []dijkstraItem

func (h dijkstraHeap) Len() int { return len(h) }
func (h dijkstraHeap) Less(i, j int) bool {
	if h[i].cost != h[j].cost {
		return h[i].cost < h[j].cost
	}
	if h[i].hops != h[j].hops {
		return h[i].hops < h[j].hops
	}
	return domain.CompareCoords(h[i].coord, h[j].coord) < 0
}
func (h dijkstraHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }
func (h *dijkstraHeap) Push(x any)   { *h = append(*h, x.(dijkstraItem)) }
func (h *dijkstraHeap) Pop() any {
	old := *h
	n := len(old)
	item := old[n-1]
	*h = old[:n-1]
	return item
}
