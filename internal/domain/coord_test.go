package domain

import "testing"

func TestCoord3DAdjacencyAndDirection(t *testing.T) {
	origin := Coord3D{X: 0, Y: 0, Z: 0}

	cases := []struct {
		dir      Direction
		expected Coord3D
		opposite Direction
	}{
		{North, Coord3D{0, 0, -1}, South},
		{South, Coord3D{0, 0, 1}, North},
		{East, Coord3D{1, 0, 0}, West},
		{West, Coord3D{-1, 0, 0}, East},
		{Up, Coord3D{0, 1, 0}, Down},
		{Down, Coord3D{0, -1, 0}, Up},
	}

	for _, tc := range cases {
		adj := origin.Adjacent(tc.dir)
		if adj != tc.expected {
			t.Fatalf("Adjacent(%s) = %v, want %v", tc.dir, adj, tc.expected)
		}
		if tc.dir.Opposite() != tc.opposite {
			t.Fatalf("Opposite(%s) = %s, want %s", tc.dir, tc.dir.Opposite(), tc.opposite)
		}
		gotDir, ok := origin.DirectionTo(adj)
		if !ok || gotDir != tc.dir {
			t.Fatalf("DirectionTo(%v) = (%s, %v), want (%s, true)", adj, gotDir, ok, tc.dir)
		}
		if adj.Adjacent(tc.opposite) != origin {
			t.Fatalf("round-trip from %v via %s did not return to origin", adj, tc.opposite)
		}
	}

	if len(origin.Neighbors()) != 6 {
		t.Fatalf("expected 6 neighbors, got %d", len(origin.Neighbors()))
	}
}

func TestDirectionParseAndRotate(t *testing.T) {
	d, ok := ParseDirection("  EAST ")
	if !ok || d != East {
		t.Fatalf("ParseDirection('  EAST ') = (%q, %v), want ('east', true)", d, ok)
	}
	if _, ok := ParseDirection("diagonal"); ok {
		t.Fatal("expected invalid direction to fail ParseDirection")
	}

	cur := North
	expected := []Direction{East, South, West, North}
	for _, want := range expected {
		cur = cur.RotateYClockwise()
		if cur != want {
			t.Fatalf("RotateYClockwise got %s, want %s", cur, want)
		}
	}
}

func TestCompareCoordsAndManhattan(t *testing.T) {
	a := Coord3D{X: 2, Y: 0, Z: 1}
	b := Coord3D{X: -1, Y: 1, Z: 0}
	if CompareCoords(a, b) >= 0 {
		t.Fatalf("expected lower Y coord %v to sort before %v", a, b)
	}
	if dist := a.ManhattanDistance(b); dist != 5 {
		t.Fatalf("expected ManhattanDistance 5, got %d", dist)
	}
}
