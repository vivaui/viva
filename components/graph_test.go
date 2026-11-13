package components

import (
	"strings"
	"testing"

	"github.com/vivaui/viva/gfx"
)

var graphTestTree gfx.Tree

func graphTestScope() gfx.Scope {
	return graphTestTree.Sub(0, graphTestTree.Name("nodes"))
}

func graphTestData() (*GraphSim, []GraphNode) {
	sim := NewGraphSim(6, []GraphLink{
		{Source: 0, Target: 1}, {Source: 1, Target: 2}, {Source: 2, Target: 0},
		{Source: 1, Target: 3}, {Source: 3, Target: 4}, {Source: 1, Target: 5},
	})
	nodes := []GraphNode{
		{Label: "Index", Group: 0, Resolved: true},
		{Label: "Graph", Group: 0, Resolved: true},
		{Label: "Links", Group: 1, Resolved: true},
		{Label: "Tags", Group: 1, Resolved: true},
		{Label: "Drafts", Group: 2, Resolved: true},
		{Label: "Missing", Group: 2},
	}
	return sim, nodes
}

func graphTestPaint(canvas *gfx.Canvas, state *GraphState, options GraphOptions) GraphLayout {
	if canvas != nil {
		canvas.SetMeasurer(chartTestMeasure)
		canvas.SetTree(&graphTestTree)
	}
	sim, nodes := graphTestData()
	return PaintGraph(canvas, gfx.LightTheme(), gfx.Rect{X: 10, Y: 10, Width: 300, Height: 220}, sim, nodes, state, options)
}

func TestPaintGraphPaintsEdgesNodesAndHits(t *testing.T) {
	var canvas gfx.Canvas
	state := &GraphState{}
	layout := graphTestPaint(&canvas, state, GraphOptions{Nodes: graphTestScope()})
	if layout.Scale <= 0 {
		t.Fatalf("scale = %v", layout.Scale)
	}
	if len(canvas.Triangles()) == 0 {
		t.Fatal("no edge geometry painted")
	}
	// One circle per resolved-or-not node and one hit per node, plus none for
	// a surface that was not named.
	if len(canvas.Rectangles()) != 6 {
		t.Fatalf("node circles = %d, want 6", len(canvas.Rectangles()))
	}
	if len(canvas.Hits()) != 6 {
		t.Fatalf("node hits = %d, want 6", len(canvas.Hits()))
	}
	for index, hit := range canvas.Hits() {
		if hit.ID != graphTestScope().At(index) || hit.Role != gfx.RoleLink {
			t.Fatalf("hit %d = %+v", index, hit)
		}
	}
}

func TestPaintGraphHoverHighlightsAndLabelsNeighbors(t *testing.T) {
	var canvas gfx.Canvas
	state := &GraphState{}
	graphTestPaint(&canvas, state, GraphOptions{Nodes: graphTestScope()})
	quiet := len(canvas.Texts())
	canvas.Reset()
	canvas.SetInteraction(graphTestScope().At(1), gfx.ID{}, gfx.ID{}, false)
	graphTestPaint(&canvas, state, GraphOptions{Nodes: graphTestScope()})
	if len(canvas.Texts()) <= quiet {
		t.Fatalf("hover painted %d labels, quiet graph %d", len(canvas.Texts()), quiet)
	}
	labels := ""
	for _, text := range canvas.Texts() {
		labels += text.Value + " "
	}
	for _, want := range []string{"Graph", "Index", "Links", "Tags", "Missing"} {
		if !strings.Contains(labels, want) {
			t.Fatalf("hover labels %q miss neighbor %q", labels, want)
		}
	}
}

func TestPaintGraphClickSelectsNode(t *testing.T) {
	var canvas gfx.Canvas
	state := &GraphState{}
	graphTestPaint(&canvas, state, GraphOptions{Nodes: graphTestScope()})
	canvas.Reset()
	canvas.BeginEventPass(graphTestScope().At(3), gfx.ClickInfo{})
	graphTestPaint(&canvas, state, GraphOptions{Nodes: graphTestScope()})
	canvas.EndEventPass()
	if clicked, ok := state.Selection(); !ok || clicked != 3 {
		t.Fatalf("selection after click = %d, %v", clicked, ok)
	}
	// The selected node holds the tab stop; every other node stays reachable
	// by pointer and arrows but out of Tab traversal.
	tabStops := 0
	for _, hit := range canvas.Hits() {
		if !hit.SkipTab {
			tabStops++
		}
	}
	if tabStops != 1 {
		t.Fatalf("graph declares %d tab stops, want the one selected node", tabStops)
	}
}

func TestGraphLayoutMapsBetweenSpaces(t *testing.T) {
	var canvas gfx.Canvas
	layout := graphTestPaint(&canvas, nil, GraphOptions{})
	x, y := layout.FromScreen(150, 200)
	sx, sy := layout.ToScreen(x, y)
	if abs32(sx-150) > 0.01 || abs32(sy-200) > 0.01 {
		t.Fatalf("round trip moved the point: (%v,%v) -> (%v,%v) -> (%v,%v)", 150, 200, x, y, sx, sy)
	}
	if node, ok := layout.NodeAt(layout.points[2].X, layout.points[2].Y); !ok || node != 2 {
		t.Fatalf("NodeAt over node 2 = %d, %v", node, ok)
	}
	if _, ok := layout.NodeAt(11, 11); ok {
		t.Fatal("NodeAt found a node in the corner")
	}
}

func TestPaintGraphHoverYieldsToPress(t *testing.T) {
	var canvas gfx.Canvas
	state := &GraphState{}
	options := GraphOptions{Nodes: graphTestScope(), SurfaceID: gfx.Item(90)}
	quiet := func() int {
		canvas.Reset()
		graphTestPaint(&canvas, state, options)
		return len(canvas.Texts())
	}
	// A held node owns the highlight even while the pointer sweeps others:
	// dragging through a crowd must not light up the nodes it passes. Node 5
	// is a neighbor of the held node 1 but not of the swept node 2, so its
	// label is what tells the two hovers apart.
	quiet()
	canvas.SetInteraction(graphTestScope().At(2), graphTestScope().At(1), gfx.ID{}, false)
	graphTestPaint(&canvas, state, options)
	labels := ""
	for _, text := range canvas.Texts() {
		labels += text.Value + " "
	}
	if !strings.Contains(labels, "Graph") || !strings.Contains(labels, "Missing") {
		t.Fatalf("held node's labels missing: %q", labels)
	}
	// A held background — a pan — keeps the graph quiet entirely.
	canvas.SetInteraction(graphTestScope().At(2), gfx.Item(90), gfx.ID{}, false)
	if count := quiet(); count != 0 {
		t.Fatalf("pan painted %d labels", count)
	}
}

func TestPaintGraphFreezesViewportWhenSettled(t *testing.T) {
	var canvas gfx.Canvas
	canvas.SetReduceMotion(true)
	canvas.SetMeasurer(chartTestMeasure)
	canvas.SetTree(&graphTestTree)
	sim, nodes := graphTestData()
	state := &GraphState{}
	options := GraphOptions{Nodes: graphTestScope()}
	bounds := gfx.Rect{X: 10, Y: 10, Width: 300, Height: 220}
	theme := gfx.LightTheme()
	paint := func() GraphLayout {
		canvas.Reset()
		return PaintGraph(&canvas, theme, bounds, sim, nodes, state, options)
	}
	first := paint()
	if state.Zoom <= 0 {
		t.Fatal("a settled paint left the fit following instead of frozen into Zoom")
	}
	// A drag to a far corner reheats the layout and swings its bounds with
	// it; the frozen view must not chase them back under the pointer, which
	// is what made a long drag look pinned.
	sim.SetFixed(4, 4000, -4000)
	sim.Reheat(0.3)
	for ticks := 0; ticks < 20 && sim.Tick(); ticks++ {
	}
	second := paint()
	if second.Scale != first.Scale {
		t.Fatalf("scale drifted from %v to %v while the layout reheated", first.Scale, second.Scale)
	}
	sx, sy := first.ToScreen(30, -30)
	tx, ty := second.ToScreen(30, -30)
	if sx != tx || sy != ty {
		t.Fatalf("world point (30,-30) moved on screen from (%v,%v) to (%v,%v)", sx, sy, tx, ty)
	}
	// Writing Zoom back to zero refits once the layout is quiet again.
	sim.SetFree(4)
	for sim.Tick() {
	}
	state.Zoom = 0
	refit := paint()
	if refit.Scale == first.Scale {
		t.Fatalf("refit scale %v equals the frozen one", refit.Scale)
	}
	if state.Zoom <= 0 {
		t.Fatal("refit left Zoom following")
	}
}

func TestPaintGraphStaticPreviewWithoutScope(t *testing.T) {
	var canvas gfx.Canvas
	graphTestPaint(&canvas, nil, GraphOptions{})
	if len(canvas.Hits()) != 0 {
		t.Fatalf("static preview declared %d hits", len(canvas.Hits()))
	}
	if len(canvas.Rectangles()) == 0 {
		t.Fatal("static preview painted nothing")
	}
}

func TestGraphStepForKeyStepsNeighbors(t *testing.T) {
	neighbors := []int{4, 7, 9}
	cases := [...]struct {
		key    gfx.Key
		from   int
		want   int
		moving bool
	}{
		{gfx.KeyRight, 4, 7, true},
		{gfx.KeyRight, 9, 4, true},
		{gfx.KeyLeft, 4, 9, true},
		{gfx.KeyLeft, 7, 4, true},
		{gfx.KeyDown, 7, 9, true},
		{gfx.KeyUp, 9, 7, true},
		{gfx.KeyHome, 9, 4, true},
		{gfx.KeyEnd, 4, 9, true},
		{gfx.KeyEnter, 4, 4, false},
		{gfx.KeyRight, 5, 4, true}, // not a neighbor: first neighbor
	}
	for _, test := range cases {
		got, moved := GraphStepForKey(test.key, test.from, neighbors)
		if moved != test.moving || got != test.want {
			t.Fatalf("%v from %d = (%d,%v), want (%d,%v)", test.key, test.from, got, moved, test.want, test.moving)
		}
	}
	if _, moved := GraphStepForKey(gfx.KeyRight, 1, nil); moved {
		t.Fatal("step moved with no neighbors")
	}
}

func TestPaintGraphSteadyStateAllocatesNothing(t *testing.T) {
	var canvas gfx.Canvas
	canvas.SetReduceMotion(true)
	sim, nodes := graphTestData()
	state := &GraphState{}
	options := GraphOptions{Nodes: graphTestScope()}
	bounds := gfx.Rect{X: 10, Y: 10, Width: 300, Height: 220}
	theme := gfx.LightTheme()
	paint := func() {
		canvas.Reset()
		canvas.SetMeasurer(chartTestMeasure)
		canvas.SetTree(&graphTestTree)
		canvas.SetInteraction(graphTestScope().At(1), gfx.ID{}, gfx.ID{}, false)
		PaintGraph(&canvas, theme, bounds, sim, nodes, state, options)
	}
	paint()
	allocations := testing.AllocsPerRun(10, paint)
	if allocations > 0 {
		t.Fatalf("steady graph paint allocated %v times", allocations)
	}
}
