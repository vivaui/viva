package components

import (
	"math"
	"testing"
)

func graphTestLinks() []GraphLink {
	links := make([]GraphLink, 0, 20)
	for index := 0; index < 11; index++ {
		links = append(links, GraphLink{Source: index, Target: index + 1})
	}
	return append(links,
		GraphLink{Source: 0, Target: 5},
		GraphLink{Source: 2, Target: 9},
		GraphLink{Source: 3, Target: 11},
		GraphLink{Source: 12, Target: 0},
		GraphLink{Source: 12, Target: 7},
		GraphLink{Source: 1, Target: 8},
	)
}

func TestGraphSimSettlesAndIsDeterministic(t *testing.T) {
	first := NewGraphSim(13, graphTestLinks())
	ticks := 0
	for first.Tick() {
		ticks++
		if ticks > 1000 {
			t.Fatal("simulation never settled")
		}
	}
	if !first.Settled() {
		t.Fatal("settled false after ticks ran out")
	}
	if ticks < 100 {
		t.Fatalf("settled after %d ticks; alpha decay is far from d3's ~300", ticks)
	}
	second := NewGraphSim(13, graphTestLinks())
	for second.Tick() {
	}
	for index := 0; index < first.Len(); index++ {
		ax, ay := first.Position(index)
		bx, by := second.Position(index)
		if ax != bx || ay != by {
			t.Fatalf("node %d differs between identical runs: (%v,%v) vs (%v,%v)", index, ax, ay, bx, by)
		}
		if math.IsNaN(float64(ax)) || math.IsNaN(float64(ay)) {
			t.Fatalf("node %d left the plane", index)
		}
	}
}

func TestGraphSimGoldenLayout(t *testing.T) {
	sim := NewGraphSim(13, graphTestLinks())
	for sim.Tick() {
	}
	x, y := sim.Position(0)
	goldenX, goldenY := float32(-15.272539), float32(-6.077562)
	if diff := float32(math.Abs(float64(x - goldenX))); diff > 0.001 {
		t.Fatalf("node 0 x = %v, want %v (diff %v); the force model changed", x, goldenX, diff)
	}
	if diff := float32(math.Abs(float64(y - goldenY))); diff > 0.001 {
		t.Fatalf("node 0 y = %v, want %v (diff %v); the force model changed", y, goldenY, diff)
	}
}

func TestGraphSimFixedNodeStaysPutAndReheats(t *testing.T) {
	sim := NewGraphSim(13, graphTestLinks())
	for sim.Tick() {
	}
	sim.SetFixed(4, 500, -300)
	sim.Reheat(0.3)
	if sim.Settled() {
		t.Fatal("reheat left the simulation cold")
	}
	for ticks := 0; sim.Tick(); ticks++ {
		if x, y := sim.Position(4); x != 500 || y != -300 {
			t.Fatalf("fixed node moved to (%v,%v)", x, y)
		}
		if ticks > 1000 {
			t.Fatal("simulation never settled around the fixed node")
		}
	}
	sim.SetFree(4)
	sim.Reheat(0.3)
	x, y := sim.Position(4)
	moved := false
	for ticks := 0; sim.Tick(); ticks++ {
		if nx, ny := sim.Position(4); nx != x || ny != y {
			moved = true
			break
		}
		if ticks > 50 {
			break
		}
	}
	if !moved {
		t.Fatal("freed node stayed at the pinned position")
	}
}

func TestGraphSimDropsInvalidLinksAndCountsDegrees(t *testing.T) {
	sim := NewGraphSim(4, []GraphLink{
		{Source: 0, Target: 1},
		{Source: 1, Target: 2},
		{Source: 2, Target: 2},  // self
		{Source: 3, Target: 9},  // out of range
		{Source: -1, Target: 0}, // out of range
	})
	if sim.Len() != 4 {
		t.Fatalf("len = %d", sim.Len())
	}
	if got := sim.Degree(0) + sim.Degree(1) + sim.Degree(2) + sim.Degree(3); got != 4 {
		t.Fatalf("degrees sum to %d, want 4", got)
	}
	if neighbors := sim.Neighbors(1); len(neighbors) != 2 {
		t.Fatalf("node 1 neighbors = %v", neighbors)
	}
}

func TestGraphSimBoundsCoverEveryNode(t *testing.T) {
	sim := NewGraphSim(9, []GraphLink{{Source: 0, Target: 8}})
	for sim.Tick() {
	}
	minX, minY, maxX, maxY := sim.Bounds()
	for index := 0; index < sim.Len(); index++ {
		x, y := sim.Position(index)
		if x < minX || x > maxX || y < minY || y > maxY {
			t.Fatalf("node %d at (%v,%v) outside bounds (%v,%v)-(%v,%v)", index, x, y, minX, minY, maxX, maxY)
		}
	}
}
