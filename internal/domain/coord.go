package domain

import (
	"fmt"
	"strings"
)

// Coord3D represents an integer position in the 3D voxel grid.
type Coord3D struct {
	X int `json:"x"`
	Y int `json:"y"`
	Z int `json:"z"`
}

// Adjacent returns the coordinate adjacent to c in the given direction.
func (c Coord3D) Adjacent(dir Direction) Coord3D {
	off := dir.Offset()
	return Coord3D{
		X: c.X + off.X,
		Y: c.Y + off.Y,
		Z: c.Z + off.Z,
	}
}

// Neighbors returns the six coordinates directly adjacent to c in canonical order.
func (c Coord3D) Neighbors() []Coord3D {
	dirs := AllDirections()
	neighbors := make([]Coord3D, len(dirs))
	for i, d := range dirs {
		neighbors[i] = c.Adjacent(d)
	}
	return neighbors
}

// ManhattanDistance returns the L1 grid distance between two coordinates.
func (c Coord3D) ManhattanDistance(other Coord3D) int {
	dx := c.X - other.X
	if dx < 0 {
		dx = -dx
	}
	dy := c.Y - other.Y
	if dy < 0 {
		dy = -dy
	}
	dz := c.Z - other.Z
	if dz < 0 {
		dz = -dz
	}
	return dx + dy + dz
}

// DirectionTo returns the cardinal/vertical direction from c to an immediately adjacent neighbor, if adjacent.
func (c Coord3D) DirectionTo(neighbor Coord3D) (Direction, bool) {
	for _, d := range AllDirections() {
		if c.Adjacent(d) == neighbor {
			return d, true
		}
	}
	return "", false
}

// String returns a human-readable coordinate string.
func (c Coord3D) String() string {
	return fmt.Sprintf("(%d, %d, %d)", c.X, c.Y, c.Z)
}

// CompareCoords defines a deterministic total ordering over 3D coordinates (Y, then Z, then X).
func CompareCoords(a, b Coord3D) int {
	if a.Y != b.Y {
		if a.Y < b.Y {
			return -1
		}
		return 1
	}
	if a.Z != b.Z {
		if a.Z < b.Z {
			return -1
		}
		return 1
	}
	if a.X != b.X {
		if a.X < b.X {
			return -1
		}
		return 1
	}
	return 0
}

// Direction represents one of the six block faces / grid directions.
type Direction string

const (
	North Direction = "north" // -Z
	South Direction = "south" // +Z
	East  Direction = "east"  // +X
	West  Direction = "west"  // -X
	Up    Direction = "up"    // +Y
	Down  Direction = "down"  // -Y
)

// ParseDirection normalizes and validates a direction string.
func ParseDirection(raw string) (Direction, bool) {
	d := Direction(strings.ToLower(strings.TrimSpace(raw)))
	return d, d.IsValid()
}

// IsValid reports whether d is one of the six valid directions.
func (d Direction) IsValid() bool {
	switch d {
	case North, South, East, West, Up, Down:
		return true
	default:
		return false
	}
}

// Opposite returns the opposite direction.
func (d Direction) Opposite() Direction {
	switch d {
	case North:
		return South
	case South:
		return North
	case East:
		return West
	case West:
		return East
	case Up:
		return Down
	case Down:
		return Up
	default:
		return North
	}
}

// RotateYClockwise rotates horizontal directions 90 degrees clockwise around +Y.
func (d Direction) RotateYClockwise() Direction {
	switch d {
	case North:
		return East
	case East:
		return South
	case South:
		return West
	case West:
		return North
	default:
		return d
	}
}

// Offset returns the unit step vector for d.
func (d Direction) Offset() Coord3D {
	switch d {
	case North:
		return Coord3D{0, 0, -1}
	case South:
		return Coord3D{0, 0, 1}
	case East:
		return Coord3D{1, 0, 0}
	case West:
		return Coord3D{-1, 0, 0}
	case Up:
		return Coord3D{0, 1, 0}
	case Down:
		return Coord3D{0, -1, 0}
	default:
		return Coord3D{}
	}
}

// String returns the canonical lowercase direction string.
func (d Direction) String() string {
	return string(d)
}

// AllDirections returns all six directions in deterministic canonical order.
func AllDirections() []Direction {
	return []Direction{North, South, East, West, Up, Down}
}
