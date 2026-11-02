package domain

import "fmt"

// Coord3D represents a position in a 3D block grid.
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

// Neighbors returns the six coordinates directly adjacent to c.
func (c Coord3D) Neighbors() []Coord3D {
	dirs := AllDirections()
	neighbors := make([]Coord3D, len(dirs))
	for i, d := range dirs {
		neighbors[i] = c.Adjacent(d)
	}
	return neighbors
}

// String returns a human-readable representation of the coordinate.
func (c Coord3D) String() string {
	return fmt.Sprintf("(%d, %d, %d)", c.X, c.Y, c.Z)
}

// Direction represents a cardinal or vertical direction in 3D space.
type Direction int

const (
	North Direction = iota // -Z
	South                  // +Z
	East                   // +X
	West                   // -X
	Up                     // +Y
	Down                   // -Y
)

// Opposite returns the direction opposite to d.
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
		return d
	}
}

// Offset returns the unit vector offset for this direction as a Coord3D.
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

// String returns the name of the direction.
func (d Direction) String() string {
	switch d {
	case North:
		return "North"
	case South:
		return "South"
	case East:
		return "East"
	case West:
		return "West"
	case Up:
		return "Up"
	case Down:
		return "Down"
	default:
		return "Unknown"
	}
}

// AllDirections returns a slice containing all six directions.
func AllDirections() []Direction {
	return []Direction{North, South, East, West, Up, Down}
}

// Face represents which face of a block is being referenced.
type Face int

const (
	FaceNorth Face = iota
	FaceSouth
	FaceEast
	FaceWest
	FaceTop
	FaceBottom
)

// String returns the name of the face.
func (f Face) String() string {
	switch f {
	case FaceNorth:
		return "North"
	case FaceSouth:
		return "South"
	case FaceEast:
		return "East"
	case FaceWest:
		return "West"
	case FaceTop:
		return "Top"
	case FaceBottom:
		return "Bottom"
	default:
		return "Unknown"
	}
}
