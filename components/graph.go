package components

import (
	"math"

	"github.com/vivaui/viva/gfx"
)

// GraphNode is one node's display data: what it is called, the color group it
// belongs to, and whether the note it stands for exists. Where it lies and
// how big it gets belong to the simulation, which sizes a node by its link
// count.
type GraphNode struct {
	Label string
	Group int
	// Resolved is false for the target of a link whose note does not exist
	// yet: painted small and dim, as Obsidian paints unresolved links.
	Resolved bool
}

// GraphOptions configures one graph painting.
type GraphOptions struct {
	// Nodes is the identity list the graph declares its nodes in: hit and
	// focus identities are Nodes.At(i), so a click reads as the item number.
	// The zero scope paints a static preview with no interaction.
	Nodes gfx.Scope
	// SurfaceID names the graph's whole area, so an application can pan by
	// dragging the background. The zero ID paints no surface hit.
	SurfaceID gfx.ID
	// NodeColor overrides the color of a group. Nil paints the theme's chart
	// palette.
	NodeColor func(theme gfx.Theme, group int) gfx.Color
}

// GraphState is the caller-owned state of one graph: the viewport and the
// caches that keep a steady graph from allocating. Keep it as long as the
// graph lives and pass the same one every paint. A nil state paints from the
// simulation's current positions with the fitting viewport.
type GraphState struct {
	// Zoom scales graph units to pixels. Zero keeps the whole graph in view,
	// following the layout while it settles and freezing the fit the moment
	// the layout goes quiet or a node is grabbed — reheating the layout
	// afterwards swings the graph's bounds around, and a view recomputed from
	// them would drift under the pointer. The application writes a value when
	// the user zoomes, and Pan then moves the view in window pixels. Writing
	// zero back refits the view.
	Zoom float32
	Pan  gfx.Point

	// anchorX/anchorY is the graph-space point the view centers on: followed
	// while zoom stays zero, frozen with the fit.
	anchorX, anchorY float32

	selected     int
	selectedSeen bool
	hover        int
	neighbors    []int
	adjacent     []bool
	screen       []gfx.Point
	radii        []float32
	edgeBuilders [3]gfx.PathBuilder
}

// Select makes node i the selection, which paints a ring and keeps the
// keyboard's tab stop.
func (state *GraphState) Select(i int) {
	state.selected, state.selectedSeen = i, true
}

// ClearSelection removes the selection.
func (state *GraphState) ClearSelection() {
	state.selectedSeen = false
}

// Selection is the selected node, and false while none is.
func (state *GraphState) Selection() (int, bool) {
	return state.selected, state.selectedSeen
}

// GraphLayout reports where a painted graph put its nodes, in window pixels,
// and is the coordinate bridge an application's pointer and wheel handlers
// use: FromScreen turns a pointer position into graph units for the
// simulation, NodeAt finds the node a pointer is over, and ToScreen places a
// label or a preview beside one.
type GraphLayout struct {
	Bounds gfx.Rect
	Scale  float32

	points     []gfx.Point
	radii      []float32
	cx, cy     float32
	ox, oy     float32
	clicked    int
	wasClicked bool
}

// ToScreen places a graph-space point in window pixels.
func (layout GraphLayout) ToScreen(x, y float32) (float32, float32) {
	return layout.ox + (x-layout.cx)*layout.Scale, layout.oy + (y-layout.cy)*layout.Scale
}

// FromScreen is ToScreen's inverse: the graph-space point under a window
// position.
func (layout GraphLayout) FromScreen(x, y float32) (float32, float32) {
	return (x-layout.ox)/layout.Scale + layout.cx, (y-layout.oy)/layout.Scale + layout.cy
}

// NodeAt is the node a window position is over, within its radius plus a
// little slack, and false for empty space. It reads the positions of the last
// paint, so a pointer handler calls it between repaints.
func (layout GraphLayout) NodeAt(x, y float32) (int, bool) {
	for index, point := range layout.points {
		radius := layout.radii[index] + 4
		if (x-point.X)*(x-point.X)+(y-point.Y)*(y-point.Y) <= radius*radius {
			return index, true
		}
	}
	return 0, false
}

// Clicked is the node whose click this paint is delivering; the application
// opens the note here.
func (layout GraphLayout) Clicked() (int, bool) {
	return layout.clicked, layout.wasClicked
}

// PaintGraph paints a force-directed link graph: edges as straight lines in
// one path per color, nodes as circles sized by their link count, labels for
// the nodes the pointer explores, and a hit region per visible node. Hovering
// a node highlights its edges and dims the rest. The simulation advances one
// tick per painted frame until it cools below its minimum alpha, and then the
// graph paints the same picture until something reheats it — the caller never
// schedules frames for it. A reduced-motion session, or a canvas with no
// frame clock, settles the layout in full at the first paint instead of
// animating.
func PaintGraph(canvas *gfx.Canvas, theme gfx.Theme, bounds gfx.Rect, sim *GraphSim, nodes []GraphNode, state *GraphState, options GraphOptions) GraphLayout {
	if bounds.Width <= 0 || bounds.Height <= 0 {
		bounds = gfx.Rect{Width: 320, Height: 200}
	}
	if sim == nil {
		sim = NewGraphSim(0, nil)
	}
	if len(nodes) < sim.Len() {
		nodes = append(nodes, make([]GraphNode, sim.Len()-len(nodes))...)
	}
	if state == nil {
		state = &GraphState{}
	}
	if !sim.Settled() {
		if canvas != nil && canvas.HasClock() && !canvas.ReduceMotion() {
			sim.Tick()
			canvas.RequestFrame()
		} else {
			for ticks := 0; ticks < 1000 && sim.Tick(); ticks++ {
			}
		}
	}

	// The viewport: a zero zoom keeps the whole graph in view, following the
	// layout while it settles. The moment the layout goes quiet — or a node is
	// grabbed — the followed fit freezes into Zoom and the anchor with it: a
	// drag reheats the layout and swings its bounds, and a view still
	// recomputed from those bounds would rescale and recenter under the
	// pointer until the graph looked pinned. An explicit zoom never follows,
	// and Pan moves the view.
	minX, minY, maxX, maxY := sim.Bounds()
	cx, cy := (minX+maxX)/2, (maxY+minY)/2
	scale := state.Zoom
	if scale <= 0 {
		scale = 0.9 * min(bounds.Width/(maxX-minX), bounds.Height/(maxY-minY))
		state.anchorX, state.anchorY = cx, cy
		grabbed := false
		if canvas != nil && !options.Nodes.IsZero() {
			_, grabbed = canvas.Pressed().In(options.Nodes)
		}
		if sim.Settled() || grabbed {
			state.Zoom = scale
		}
	}
	ox := bounds.X + bounds.Width/2 + state.Pan.X
	oy := bounds.Y + bounds.Height/2 + state.Pan.Y
	uiScale := theme.Metrics.ControlHeight / 32

	state.screen = growPoints(state.screen, sim.Len())
	state.radii = growFloats(state.radii, sim.Len())
	for index := 0; index < sim.Len(); index++ {
		x, y := sim.Position(index)
		radius := 2.2*float32(math.Sqrt(float64(sim.Degree(index))))*uiScale + 3*uiScale
		if !nodes[index].Resolved {
			radius = 2 * uiScale
		}
		radius = max(min(radius, 22*uiScale), 1.5*uiScale)
		state.screen[index] = gfx.Point{X: ox + (x-state.anchorX)*scale, Y: oy + (y-state.anchorY)*scale}
		state.radii[index] = radius
	}

	// Hover belongs to the pointer only while no press holds it: dragging a
	// node through a crowd would otherwise light up every node it passes. A
	// held node keeps its own edges lit (the drag shows what it is attached
	// to), a held background — a pan — keeps the graph quiet, and an idle
	// pointer or a press elsewhere leaves hover to the pointer.
	hover := -1
	if canvas != nil && !options.Nodes.IsZero() {
		pressed := canvas.Pressed()
		if node, dragging := pressed.In(options.Nodes); dragging {
			hover = node
		} else if pressed.IsZero() || pressed != options.SurfaceID {
			if index, ok := canvas.Hovered().In(options.Nodes); ok {
				hover = index
			}
		}
	}
	// The neighbor marks of the previous hover clear before the new ones set,
	// so a steady hover leaves the cache untouched.
	if len(state.adjacent) < len(nodes) {
		state.adjacent = make([]bool, len(nodes))
	}
	for _, neighbor := range state.neighbors {
		if neighbor >= 0 && neighbor < len(state.adjacent) {
			state.adjacent[neighbor] = false
		}
	}
	if hover != state.hover {
		state.hover = hover
		state.neighbors = sim.Neighbors(hover)
	}
	for _, neighbor := range state.neighbors {
		if neighbor >= 0 && neighbor < len(nodes) {
			state.adjacent[neighbor] = true
		}
	}

	layout := GraphLayout{
		Bounds: bounds, Scale: scale,
		points: state.screen[:sim.Len()], radii: state.radii[:sim.Len()],
		cx: state.anchorX, cy: state.anchorY, ox: ox, oy: oy,
	}
	if canvas == nil {
		return layout
	}

	canvas.Clip(bounds)
	if !options.SurfaceID.IsZero() {
		canvas.Hit(options.SurfaceID, bounds)
	}
	if sim.Len() == 0 {
		canvas.LabelCenter("No linked notes", bounds, gfx.TextStyle{Size: theme.Metrics.FontSizeM, Color: theme.MutedForeground})
		canvas.Unclip()
		return layout
	}
	if !options.Nodes.IsZero() {
		if index, ok := canvas.ClickedIn(options.Nodes); ok {
			layout.clicked, layout.wasClicked = index, true
			state.Select(index)
		}
	}
	selected, selectedSeen := state.Selection()

	// Edges, one path per color, drawn dimmed first and the hovered node's
	// edges last so they sit on top. Coordinates are relative to the bounds,
	// which DrawPath maps back onto themselves. ponytail: beyond a few
	// thousand visible edges, batch them as instanced line geometry instead of
	// per-frame tessellation.
	ordinary := theme.Border
	dimmed := theme.Border.Mix(theme.Background, 0.7)
	for bucket := 2; bucket >= 0; bucket-- {
		builder := &state.edgeBuilders[bucket]
		builder.Reset()
		for at := range sim.links {
			link := &sim.links[at]
			from, to := state.screen[link.Source], state.screen[link.Target]
			if !segmentNear(bounds, from, to) {
				continue
			}
			if hover >= 0 {
				if link.Source != hover && link.Target != hover {
					if bucket != 2 {
						continue
					}
				} else if bucket != 0 {
					continue
				}
			} else if bucket != 1 {
				continue
			}
			builder.MoveTo(from.X-bounds.X, from.Y-bounds.Y)
			builder.LineTo(to.X-bounds.X, to.Y-bounds.Y)
		}
		path := builder.Path(bounds.Width, bounds.Height)
		if path.Size().Width == 0 {
			continue
		}
		style := gfx.PathStyle{Stroke: ordinary, StrokeWidth: uiScale}
		switch bucket {
		case 0:
			style.Stroke, style.StrokeWidth = graphNodeColor(theme, options, nodes, hover), 1.8*uiScale
		case 2:
			style.Stroke = dimmed
		}
		canvas.DrawPath(path, bounds, style)
	}

	// Nodes: filled circles in their group color; a ring on the hovered, the
	// focused and the selected node, drawn before the hits below them.
	for index := 0; index < sim.Len(); index++ {
		point, radius := state.screen[index], state.radii[index]
		if !pointNear(bounds, point, radius) {
			continue
		}
		box := gfx.Rect{X: point.X - radius, Y: point.Y - radius, Width: radius * 2, Height: radius * 2}
		color := graphNodeColor(theme, options, nodes, index)
		if hover >= 0 && !state.adjacent[index] && index != hover {
			color = color.Mix(theme.Background, 0.65)
		}
		if !nodes[index].Resolved {
			color = theme.MutedForeground.Mix(theme.Background, 0.5)
		}
		canvas.Rounded(box, radius, color)
		if index == hover || (selectedSeen && index == selected) || (!options.Nodes.IsZero() && canvas.FocusVisible() == options.Nodes.At(index)) {
			ring := radius + 2*uiScale
			ringBox := gfx.Rect{X: point.X - ring, Y: point.Y - ring, Width: ring * 2, Height: ring * 2}
			canvas.RoundedBorder(ringBox, ring, max(1.5*uiScale, 1), theme.Ring, gfx.Color{})
		}
	}

	// Labels: the hovered node, its neighbors and the selection carry words;
	// a quiet graph shows none, which keeps a large one readable.
	labelStyle := gfx.TextStyle{Size: theme.Metrics.FontSizeS, Color: theme.Foreground}
	label := func(index int) {
		point, radius := state.screen[index], state.radii[index]
		width := canvas.TextWidth(nodes[index].Label, labelStyle.Size)
		box := gfx.Rect{
			X: point.X - width/2, Y: point.Y + radius + 2*uiScale,
			Width: width + 4, Height: labelStyle.Size + 2,
		}
		if box.X < bounds.X {
			box.X = bounds.X
		}
		if box.X+box.Width > bounds.X+bounds.Width {
			box.X = bounds.X + bounds.Width - box.Width
		}
		if box.Y+box.Height > bounds.Y+bounds.Height {
			box.Y = point.Y - radius - box.Height - 2*uiScale
		}
		canvas.LabelCenter(nodes[index].Label, box, labelStyle)
	}
	if hover >= 0 {
		label(hover)
		for _, neighbor := range state.neighbors {
			label(neighbor)
		}
	}
	if selectedSeen && selected != hover {
		label(selected)
	}

	// Hits: every visible node is a focusable link, its hit region the circle
	// itself — hit regions of one list never overlap, so a dense graph
	// resolves by node radius and the pointer finds the node it covers. The
	// selected node holds the tab stop and the rest stay out of Tab
	// traversal, reachable by pointer and by the application's arrow-key
	// handler (GraphStepForKey).
	for index := 0; index < sim.Len(); index++ {
		point, radius := state.screen[index], state.radii[index]
		if !pointNear(bounds, point, radius) || options.Nodes.IsZero() {
			continue
		}
		box := gfx.Rect{X: point.X - radius, Y: point.Y - radius, Width: radius * 2, Height: radius * 2}
		tabStop := (selectedSeen && index == selected) || (!selectedSeen && index == 0)
		canvas.HitWithOptions(options.Nodes.At(index), box, nodes[index].Label, gfx.HitOptions{
			Role:     gfx.RoleLink,
			Cursor:   gfx.PointerCursorHand,
			Selected: selectedSeen && index == selected,
			SkipTab:  !tabStop,
		})
	}
	canvas.Unclip()
	return layout
}

// GraphStepForKey is a graph's keyboard movement: the arrow keys step from
// the current node through its neighbors, wrapping at the ends, and Home and
// End go to the first and last neighbor. neighbors is the node's own list
// (GraphSim.Neighbors); with none, no key does anything.
func GraphStepForKey(key gfx.Key, current int, neighbors []int) (int, bool) {
	if len(neighbors) == 0 {
		return current, false
	}
	step := func(delta int) (int, bool) {
		for index, neighbor := range neighbors {
			if neighbor == current {
				next := (index + delta + len(neighbors)) % len(neighbors)
				return neighbors[next], true
			}
		}
		if delta > 0 {
			return neighbors[0], true
		}
		return neighbors[len(neighbors)-1], true
	}
	switch key {
	case gfx.KeyHome:
		return neighbors[0], true
	case gfx.KeyEnd:
		return neighbors[len(neighbors)-1], true
	case gfx.KeyRight, gfx.KeyDown:
		return step(1)
	case gfx.KeyLeft, gfx.KeyUp:
		return step(-1)
	}
	return current, false
}

// graphNodeColor is the color of node index's group.
func graphNodeColor(theme gfx.Theme, options GraphOptions, nodes []GraphNode, index int) gfx.Color {
	if index < 0 || index >= len(nodes) {
		return theme.Chart[0]
	}
	if options.NodeColor != nil {
		return options.NodeColor(theme, nodes[index].Group)
	}
	return theme.Chart[nodes[index].Group%len(theme.Chart)]
}

// segmentNear reports whether an edge from one point to another can touch the
// bounds: both endpoints culled is invisible.
func segmentNear(bounds gfx.Rect, from, to gfx.Point) bool {
	return pointNear(bounds, from, 0) || pointNear(bounds, to, 0)
}

// pointNear reports whether a circle of radius at a point reaches the bounds.
func pointNear(bounds gfx.Rect, point gfx.Point, radius float32) bool {
	return point.X+radius >= bounds.X && point.X-radius <= bounds.X+bounds.Width &&
		point.Y+radius >= bounds.Y && point.Y-radius <= bounds.Y+bounds.Height
}

func growPoints(values []gfx.Point, size int) []gfx.Point {
	if len(values) >= size {
		return values[:size]
	}
	return append(values, make([]gfx.Point, size-len(values))...)
}

func growFloats(values []float32, size int) []float32 {
	if len(values) >= size {
		return values[:size]
	}
	return append(values, make([]float32, size-len(values))...)
}
