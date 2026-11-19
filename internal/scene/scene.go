package scene

// ID is a stable, generational scene-node identity.
type ID uint64

// Kind identifies the data carried by a scene node.
type Kind uint8

const (
	KindGroup Kind = iota + 1
	KindRectangle
	KindTriangle
	// KindShadow is a rounded rectangle's soft drop shadow. Bounds and Radius
	// are the rectangle that casts it; Shadow says where the shadow falls and how
	// far it blurs. The rectangle itself is cut out of the shadow.
	KindShadow
)

// Operation describes how a renderer should update one retained node.
type Operation uint8

const (
	OperationUpsert Operation = iota + 1
	OperationDelete
)

// Rect is an axis-aligned rectangle in logical pixels.
type Rect struct {
	X      float32
	Y      float32
	Width  float32
	Height float32
}

// Color is a linear RGBA color.
type Color struct {
	Red   float32
	Green float32
	Blue  float32
	Alpha float32
}

// Point is one logical-pixel scene coordinate.
type Point struct {
	X float32
	Y float32
}

// Shade makes a triangle vary from its Color at Points[0] to Colors[0] at
// Points[1] and Colors[1] at Points[2], linearly across its face. The zero Shade
// leaves the triangle one flat color.
type Shade struct {
	Colors [2]Color
	On     bool
}

// Shadow places and softens a KindShadow node, in logical pixels, like one
// layer of a CSS box-shadow: the shadow is the casting rectangle moved by
// OffsetX and OffsetY, grown by Spread on every side (shrunk when negative) and
// blurred over a Blur-wide band either side of its edge.
type Shadow struct {
	OffsetX float32
	OffsetY float32
	Blur    float32
	Spread  float32
}

// Extent is how far a shadow reaches beyond the rectangle that casts it, in
// every direction: the most the quad a renderer draws it in has to grow.
func (shadow Shadow) Extent() float32 {
	return shadow.Blur + max(shadow.Spread, 0) + max(abs32(shadow.OffsetX), abs32(shadow.OffsetY))
}

func abs32(value float32) float32 {
	if value < 0 {
		return -value
	}
	return value
}

// Shape is the geometry only a triangle or a shadow carries. Rectangle
// changes leave Change.Shape nil, so the common change is small.
type Shape struct {
	Points [3]Point
	Shade  Shade
	Shadow Shadow
}

// Change is one renderer-facing incremental scene update.
type Change struct {
	ID              ID
	Operation       Operation
	Parent          ID
	PreviousSibling ID
	Kind            Kind
	Bounds          Rect
	Color           Color
	Radius          float32
	EdgeMask        uint8
	// Shape is the triangle points or shadow of a KindTriangle or KindShadow
	// change, and nil for a rectangle. It points into the Scene's reusable
	// change scratch and is valid until the next DrainChanges.
	Shape *Shape
}

type node struct {
	parent      ID
	firstChild  ID
	lastChild   ID
	previous    ID
	next        ID
	bounds      Rect
	color       Color
	radius      float32
	points      [3]Point
	edgeMask    uint8
	shade       Shade
	shadow      Shadow
	generation  uint32
	kind        Kind
	alive       bool
	pendingFree bool
	queued      bool
	published   bool
}

// Scene stores retained nodes in flat slots with stable generational IDs.
type Scene struct {
	nodes        []node
	free         []uint32
	dirty        []ID
	scratch      []ID
	shapeScratch []Shape
	root         ID
}

// New creates a scene with one root group.
func New() *Scene {
	return NewWithCapacity(1)
}

// NewWithCapacity creates a scene sized for an expected total node count.
func NewWithCapacity(capacity int) *Scene {
	if capacity < 1 {
		capacity = 1
	}
	scene := &Scene{
		nodes: make([]node, 1, capacity),
		dirty: make([]ID, 0, capacity),
	}
	scene.nodes[0] = node{
		generation: 1,
		kind:       KindGroup,
		alive:      true,
	}
	scene.root = makeID(0, 1)
	scene.markDirty(scene.root, &scene.nodes[0])
	return scene
}

// Root returns the permanent root group.
func (scene *Scene) Root() ID {
	return scene.root
}

// AddGroup creates a retained group below parent.
func (scene *Scene) AddGroup(parent ID) (ID, bool) {
	return scene.add(parent, KindGroup, Rect{}, Color{}, 0, [3]Point{}, 0, Shade{}, Shadow{})
}

// AddRectangle creates a retained rectangle below parent.
func (scene *Scene) AddRectangle(parent ID, bounds Rect, color Color) (ID, bool) {
	return scene.AddRoundedRectangle(parent, bounds, color, 0)
}

// AddRoundedRectangle creates a retained rounded rectangle below parent.
func (scene *Scene) AddRoundedRectangle(parent ID, bounds Rect, color Color, radius float32) (ID, bool) {
	return scene.add(parent, KindRectangle, bounds, color, radius, [3]Point{}, 0, Shade{}, Shadow{})
}

// AddTriangle creates one retained vector-mesh triangle below parent.
func (scene *Scene) AddTriangle(parent ID, points [3]Point, color Color, edgeMask uint8, shade Shade) (ID, bool) {
	return scene.add(parent, KindTriangle, Rect{}, color, 0, points, edgeMask, shade, Shadow{})
}

func (scene *Scene) add(parent ID, kind Kind, bounds Rect, color Color, radius float32, points [3]Point, edgeMask uint8, shade Shade, shadow Shadow) (ID, bool) {
	parentNode, ok := scene.liveNode(parent)
	if !ok {
		return 0, false
	}
	return scene.insert(parent, parentNode.lastChild, kind, bounds, color, radius, points, edgeMask, shade, shadow)
}

// InsertRoundedRectangle creates a rounded rectangle below parent, right after
// the child previous (the first child when previous is zero), so a shape can
// appear in the middle of the paint order without disturbing its neighbours.
func (scene *Scene) InsertRoundedRectangle(parent, previous ID, bounds Rect, color Color, radius float32) (ID, bool) {
	return scene.insert(parent, previous, KindRectangle, bounds, color, radius, [3]Point{}, 0, Shade{}, Shadow{})
}

// InsertShadow creates the soft shadow of a rounded rectangle below parent, right
// after the child previous (the first child when previous is zero).
func (scene *Scene) InsertShadow(parent, previous ID, bounds Rect, color Color, radius float32, shadow Shadow) (ID, bool) {
	return scene.insert(parent, previous, KindShadow, bounds, color, radius, [3]Point{}, 0, Shade{}, shadow)
}

// InsertTriangle creates a triangle below parent, right after the child
// previous (the first child when previous is zero).
func (scene *Scene) InsertTriangle(parent, previous ID, points [3]Point, color Color, edgeMask uint8, shade Shade) (ID, bool) {
	return scene.insert(parent, previous, KindTriangle, Rect{}, color, 0, points, edgeMask, shade, Shadow{})
}

// insert links a new child of parent after previous. A previous that is not a
// child of parent is rejected.
func (scene *Scene) insert(parent, previous ID, kind Kind, bounds Rect, color Color, radius float32, points [3]Point, edgeMask uint8, shade Shade, shadow Shadow) (ID, bool) {
	parentNode, ok := scene.liveNode(parent)
	if !ok || parentNode.kind != KindGroup {
		return 0, false
	}
	if previous != 0 {
		before, ok := scene.liveNode(previous)
		if !ok || before.parent != parent {
			return 0, false
		}
	}

	index, added := scene.allocate()
	parentNode, _ = scene.liveNode(parent) // allocate may have moved the nodes
	child := &scene.nodes[index]
	child.kind = kind
	child.alive = true
	child.parent = parent
	child.bounds = bounds
	child.color = color
	child.radius = radius
	child.points = points
	child.edgeMask = edgeMask
	child.shade = shade
	child.shadow = shadow
	id := makeID(index, child.generation)

	child.previous = previous
	if previous == 0 {
		child.next = parentNode.firstChild
		parentNode.firstChild = id
	} else {
		before, _ := scene.liveNode(previous)
		child.next = before.next
		before.next = id
	}
	if child.next == 0 {
		parentNode.lastChild = id
	} else {
		next, _ := scene.liveNode(child.next)
		next.previous = id
	}

	scene.markDirty(id, child)
	return id, added
}

// SetRectangle updates one rectangle and queues at most one incremental change.
func (scene *Scene) SetRectangle(id ID, bounds Rect, color Color) bool {
	return scene.SetRoundedRectangle(id, bounds, color, 0)
}

// SetRoundedRectangle updates one rounded rectangle and queues one change. A
// triangle becomes a rectangle in place and keeps its position in paint order.
func (scene *Scene) SetRoundedRectangle(id ID, bounds Rect, color Color, radius float32) bool {
	current, ok := scene.liveNode(id)
	if !ok || current.kind == KindGroup {
		return false
	}
	if current.kind == KindRectangle && current.bounds == bounds && current.color == color && current.radius == radius {
		return false
	}
	current.kind = KindRectangle
	current.bounds = bounds
	current.color = color
	current.radius = radius
	current.points = [3]Point{}
	current.edgeMask = 0
	current.shade = Shade{}
	current.shadow = Shadow{}
	scene.markDirty(id, current)
	return true
}

// SetShadow updates one soft shadow and queues one change. Any other shape
// becomes a shadow in place and keeps its position in paint order.
func (scene *Scene) SetShadow(id ID, bounds Rect, color Color, radius float32, shadow Shadow) bool {
	current, ok := scene.liveNode(id)
	if !ok || current.kind == KindGroup {
		return false
	}
	if current.kind == KindShadow && current.bounds == bounds && current.color == color && current.radius == radius && current.shadow == shadow {
		return false
	}
	current.kind = KindShadow
	current.bounds = bounds
	current.color = color
	current.radius = radius
	current.points = [3]Point{}
	current.edgeMask = 0
	current.shade = Shade{}
	current.shadow = shadow
	scene.markDirty(id, current)
	return true
}

// SetTriangle updates one retained vector-mesh triangle. A rectangle becomes a
// triangle in place and keeps its position in paint order.
func (scene *Scene) SetTriangle(id ID, points [3]Point, color Color, edgeMask uint8, shade Shade) bool {
	current, ok := scene.liveNode(id)
	if !ok || current.kind == KindGroup {
		return false
	}
	if current.kind == KindTriangle && current.points == points && current.color == color && current.edgeMask == edgeMask && current.shade == shade {
		return false
	}
	current.kind = KindTriangle
	current.points = points
	current.color = color
	current.edgeMask = edgeMask
	current.shade = shade
	current.bounds = Rect{}
	current.radius = 0
	current.shadow = Shadow{}
	scene.markDirty(id, current)
	return true
}

// Remove deletes a node and its descendants. The root cannot be removed.
func (scene *Scene) Remove(id ID) bool {
	current, ok := scene.liveNode(id)
	if !ok || id == scene.root {
		return false
	}

	scene.detach(current)
	scene.scratch = append(scene.scratch[:0], id)
	for cursor := 0; cursor < len(scene.scratch); cursor++ {
		currentID := scene.scratch[cursor]
		currentNode, exists := scene.liveNode(currentID)
		if !exists {
			continue
		}

		for child := currentNode.firstChild; child != 0; {
			childNode, childExists := scene.liveNode(child)
			if !childExists {
				break
			}
			next := childNode.next
			scene.scratch = append(scene.scratch, child)
			child = next
		}

	}
	for index := len(scene.scratch) - 1; index >= 0; index-- {
		currentID := scene.scratch[index]
		currentNode, exists := scene.liveNode(currentID)
		if exists {
			currentNode.alive = false
			currentNode.pendingFree = true
			scene.markDirty(currentID, currentNode)
		}
	}
	return true
}

func (scene *Scene) detach(current *node) {
	parent, _ := scene.liveNode(current.parent)
	if current.previous == 0 {
		parent.firstChild = current.next
	} else {
		previous, _ := scene.liveNode(current.previous)
		previous.next = current.next
	}
	if current.next == 0 {
		parent.lastChild = current.previous
	} else {
		next, _ := scene.liveNode(current.next)
		next.previous = current.previous
	}
}

// DrainChanges appends coalesced incremental changes to destination.
func (scene *Scene) DrainChanges(destination []Change) []Change {
	shapeCount := 0
	for _, id := range scene.dirty {
		_, current, ok := scene.node(id)
		if ok && current.queued && !current.pendingFree && (current.kind == KindTriangle || current.kind == KindShadow) {
			shapeCount++
		}
	}
	if cap(scene.shapeScratch) < shapeCount {
		scene.shapeScratch = make([]Shape, 0, shapeCount)
	} else {
		scene.shapeScratch = scene.shapeScratch[:0]
	}
	for _, id := range scene.dirty {
		index, current, ok := scene.node(id)
		if !ok || !current.queued {
			continue
		}
		current.queued = false

		if current.pendingFree {
			if current.published {
				destination = append(destination, Change{
					ID:        id,
					Operation: OperationDelete,
				})
			}
			scene.reclaim(index, current)
			continue
		}

		var shape *Shape
		if current.kind == KindTriangle || current.kind == KindShadow {
			scene.shapeScratch = append(scene.shapeScratch, Shape{
				Points: current.points, Shade: current.shade, Shadow: current.shadow,
			})
			shape = &scene.shapeScratch[len(scene.shapeScratch)-1]
		}
		destination = append(destination, Change{
			ID:              id,
			Operation:       OperationUpsert,
			Parent:          current.parent,
			PreviousSibling: current.previous,
			Kind:            current.kind,
			Bounds:          current.bounds,
			Color:           current.color,
			Radius:          current.radius,
			EdgeMask:        current.edgeMask,
			Shape:           shape,
		})
		current.published = true
	}
	scene.dirty = scene.dirty[:0]
	return destination
}

// Slot returns the zero-based storage slot encoded in a valid ID.
func (id ID) Slot() (uint32, bool) {
	index, _, ok := splitID(id)
	return index, ok
}

// PendingChanges reports the number of distinct dirty nodes.
func (scene *Scene) PendingChanges() int {
	return len(scene.dirty)
}

func (scene *Scene) allocate() (uint32, bool) {
	if len(scene.free) == 0 {
		index := uint32(len(scene.nodes))
		scene.nodes = append(scene.nodes, node{generation: 1})
		return index, true
	}

	last := len(scene.free) - 1
	index := scene.free[last]
	scene.free = scene.free[:last]
	generation := scene.nodes[index].generation
	scene.nodes[index] = node{generation: generation}
	return index, true
}

func (scene *Scene) reclaim(index uint32, current *node) {
	nextGeneration := current.generation + 1
	if nextGeneration == 0 {
		nextGeneration = 1
	}
	*current = node{generation: nextGeneration}
	scene.free = append(scene.free, index)
}

func (scene *Scene) markDirty(id ID, current *node) {
	if current.queued {
		return
	}
	current.queued = true
	scene.dirty = append(scene.dirty, id)
}

func (scene *Scene) liveNode(id ID) (*node, bool) {
	_, current, ok := scene.node(id)
	return current, ok && current.alive
}

func (scene *Scene) node(id ID) (uint32, *node, bool) {
	index, generation, ok := splitID(id)
	if !ok || int(index) >= len(scene.nodes) {
		return 0, nil, false
	}
	current := &scene.nodes[index]
	if current.generation != generation {
		return 0, nil, false
	}
	return index, current, true
}

func makeID(index uint32, generation uint32) ID {
	return ID(uint64(generation)<<32 | uint64(index+1))
}

func splitID(id ID) (uint32, uint32, bool) {
	index := uint32(id)
	generation := uint32(uint64(id) >> 32)
	if index == 0 || generation == 0 {
		return 0, 0, false
	}
	return index - 1, generation, true
}

// Len is how many nodes are alive, the root included: what a test compares before
// and after a page is shown and removed to see that nothing stayed behind.
func (scene *Scene) Len() int { return len(scene.nodes) - len(scene.free) }

// Slots is how many node slots the scene has ever allocated. It stays above
// Len while free slots are retained for reuse; a caller that sees a large gap
// can rebuild the scene to give the memory back.
func (scene *Scene) Slots() int { return len(scene.nodes) }
