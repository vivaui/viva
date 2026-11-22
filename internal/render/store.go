package render

import (
	"fmt"
	"slices"

	"github.com/vivaui/viva/internal/scene"
)

// Rectangle is one flattened rectangle instance in deterministic paint order.
type Rectangle struct {
	ID     scene.ID
	Kind   scene.Kind
	Bounds scene.Rect
	Color  scene.Color
	Radius float32
	Points [3]scene.Point
	Layer  uint16
	// EdgeMask selects the triangle edges that receive boundary feathering.
	EdgeMask uint8
	// Shade, when On, gives the triangle the colors of its second and third
	// corner; Color is the first's.
	Shade scene.Shade
	// Shadow places and blurs a KindShadow instance: Bounds and Radius are the
	// rectangle that casts it.
	Shadow scene.Shadow
	// Clipped reports that the instance is drawn only inside Clip, a region of
	// the window in logical pixels that cuts it. An instance wholly inside its
	// canvas clip is not Clipped. Like Layer, the field is not part of the
	// scene: the application layer stamps it on the instance after Apply.
	Clipped bool
	Clip    scene.Rect
}

// VertexColors is the color at each corner of a triangle: all Color for a flat
// one, so a renderer that interpolates colors across the face needs no special
// case for either.
func (rectangle Rectangle) VertexColors() [3]scene.Color {
	if !rectangle.Shade.On {
		return [3]scene.Color{rectangle.Color, rectangle.Color, rectangle.Color}
	}
	return [3]scene.Color{rectangle.Color, rectangle.Shade.Colors[0], rectangle.Shade.Colors[1]}
}

// Update describes the smallest contiguous rectangle range changed by Apply.
// Count may extend past the previous instance count when shapes were appended
// or inserted: a backend must grow its retained storage for that suffix rather
// than reject the count change. Shrink reports that the new total is smaller
// and the backend must truncate its retained storage to len(rectangles) after
// applying the suffix. Rebuild remains the fallback when the change cannot be
// expressed as a suffix.
type Update struct {
	First   int
	Count   int
	Rebuild bool
	Shrink  bool
}

// Empty reports whether an update requires no renderer work.
func (update Update) Empty() bool {
	return update.Count == 0 && !update.Rebuild && !update.Shrink
}

// node is one node of the retained tree. The geometry of a shape lives only in
// the flattened rectangles, reached through instanceBySlot; the node carries
// just the links and kind the tree needs. A node inserted by the current batch
// has no instance yet and is marked by a freshInstance-encoded slot, resolved
// from the batch's changes when the batch falls back to a rebuild.
type node struct {
	id       scene.ID
	parent   scene.ID
	first    scene.ID
	last     scene.ID
	previous scene.ID
	next     scene.ID
	kind     scene.Kind
	alive    bool
}

// Store retains renderer-owned nodes and a flat rectangle instance list.
// freshInstance marks an instanceBySlot entry that names a change in the batch
// being applied rather than a rectangle in the store: the node was inserted by
// this batch and has no instance until the batch finishes flattening it.
const freshInstance = uint32(1) << 31

type Store struct {
	nodes          []node
	instanceBySlot []uint32
	rectangles     []Rectangle
	// order is the paint order one rebuild computes for the new batch; scratch
	// is the walk stack. Both are reused, so their capacity tracks the largest
	// page rather than the number of rebuilds.
	order   []scene.ID
	scratch []scene.ID
	root    scene.ID
}

// Rectangles returns the current ordered rectangle instances. The result is
// invalidated by the next call to Apply.
func (store *Store) Rectangles() []Rectangle {
	return store.rectangles
}

// Apply consumes incremental scene changes and updates retained render state.
func (store *Store) Apply(changes []scene.Change) (Update, error) {
	if len(changes) == 0 {
		return Update{}, nil
	}

	if changesAreDeletes(changes) {
		return store.deleteBatch(changes)
	}

	oldCount := len(store.rectangles)
	structureChanged := false
	deleted := false
	// appended collects new shape nodes whose insertion may be a pure append
	// after the last instance. If the chain holds, the store can append them
	// without rebuilding (and re-uploading) the whole instance list.
	var appended []scene.Change
	// inserted indexes the changes that create a shape, in insertion order. A
	// rebuild resolves their geometry from this batch instead of the rectangles
	// the store already holds.
	var inserted []uint32
	appendOnly := oldCount > 0

	// Deletes are applied before upserts. The scene relinks a deleted node's
	// successor without reporting that successor, so an upsert of it (queued
	// earlier in the same batch for another reason, such as a new color) carries
	// the post-delete previous sibling. Applied in queue order it would see the
	// stale link and be rejected as a structural mutation, dropping the rest of
	// the batch.
	for _, change := range changes {
		switch change.Operation {
		case scene.OperationDelete:
			if err := store.remove(change.ID); err != nil {
				return Update{}, err
			}
			structureChanged = true
			deleted = true
			appendOnly = false
		case scene.OperationUpsert:
		default:
			return Update{}, fmt.Errorf("render: unsupported operation %d", change.Operation)
		}
	}

	// New nodes are linked before changed ones are applied. The scene reports a
	// node's current neighbours, so an existing node's upsert queued before the
	// insert of its new predecessor already names that predecessor, which the
	// store only learns of when it reaches the insert.
	for index, change := range changes {
		if change.Operation != scene.OperationUpsert || store.exists(change.ID) {
			continue
		}
		if _, _, err := store.upsert(change); err != nil {
			return Update{}, err
		}
		if change.Kind != scene.KindGroup {
			slot, _ := change.ID.Slot()
			store.instanceBySlot[slot] = freshInstance | uint32(len(inserted))
			inserted = append(inserted, uint32(index))
		}
		structureChanged = true
		if change.Kind == scene.KindGroup || change.Parent != store.root {
			appendOnly = false
		} else {
			appended = append(appended, change)
		}
	}

	first := oldCount
	last := -1
	for _, change := range changes {
		if change.Operation != scene.OperationUpsert {
			continue
		}
		_, instance, err := store.upsert(change)
		if err != nil {
			return Update{}, err
		}
		if instance >= 0 {
			if instance < first {
				first = instance
			}
			if instance > last {
				last = instance
			}
		}
	}

	if structureChanged {
		// The common structural change is an overlay or a row appended after
		// everything already on screen. The scene inserts shapes directly under
		// its root, so a chain of new shapes whose previous sibling is the old
		// last instance is exactly a suffix of paint order.
		if appendOnly && len(appended) > 0 {
			previous := store.rectangles[oldCount-1].ID
			for _, change := range appended {
				if change.PreviousSibling != previous {
					appendOnly = false
					break
				}
				previous = change.ID
			}
		}
		switch {
		case appendOnly && len(appended) > 0:
			for _, change := range appended {
				slot, _ := change.ID.Slot()
				store.instanceBySlot[slot] = uint32(len(store.rectangles) + 1)
				store.rectangles = append(store.rectangles, rectangleOf(change))
			}
			if last < oldCount+len(appended)-1 {
				last = oldCount + len(appended) - 1
			}
			if first > oldCount {
				first = oldCount
			}
		case !deleted && len(appended) > 0 && oldCount > 0:
			// Insertions in the middle of the page move the tail of paint
			// order, but the scene's neighbour links say exactly where each
			// new shape goes. Splicing it in and renumbering the slots after
			// it costs one shift of the instance list instead of re-flattening
			// and re-uploading the whole page. Deletions still rebuild,
			// because every backend's retained store is sized to the count.
			for _, change := range appended {
				at := 0
				if change.PreviousSibling != 0 {
					slot, _ := change.PreviousSibling.Slot()
					instance := store.instanceIndex(slot, change.PreviousSibling)
					if instance < 0 {
						if err := store.rebuild(changes, inserted); err != nil {
							return Update{}, err
						}
						return Update{Count: len(store.rectangles), Rebuild: true}, nil
					}
					at = instance + 1
				}
				store.rectangles = append(store.rectangles, Rectangle{})
				copy(store.rectangles[at+1:], store.rectangles[at:])
				store.rectangles[at] = rectangleOf(change)
				store.reindexFrom(at)
				if first > at {
					first = at
				}
			}
			if last < len(store.rectangles)-1 {
				last = len(store.rectangles) - 1
			}
		default:
			if err := store.rebuild(changes, inserted); err != nil {
				return Update{}, err
			}
			return Update{Count: len(store.rectangles), Rebuild: true}, nil
		}
	}
	if last < first {
		return Update{}, nil
	}
	return Update{First: first, Count: last - first + 1}, nil
}

// changesAreDeletes reports whether a batch only removes nodes. Deletions are
// the common structural change that can be expressed as a suffix update: the
// retained rectangles compact in place, and the backend only needs to truncate
// its store after copying the moved suffix.
func changesAreDeletes(changes []scene.Change) bool {
	for _, change := range changes {
		if change.Operation != scene.OperationDelete {
			return false
		}
	}
	return true
}

func (store *Store) deleteBatch(changes []scene.Change) (Update, error) {
	var instances []int
	for _, change := range changes {
		slot, ok := change.ID.Slot()
		if !ok || int(slot) >= len(store.nodes) {
			return Update{}, fmt.Errorf("render: delete references invalid node %d", change.ID)
		}
		if instance := store.instanceIndex(slot, change.ID); instance >= 0 {
			instances = append(instances, instance)
		}
		if err := store.remove(change.ID); err != nil {
			return Update{}, err
		}
	}
	if len(instances) == 0 {
		return Update{}, nil
	}
	slices.Sort(instances)
	first := instances[0]
	for index := len(instances) - 1; index >= 0; index-- {
		at := instances[index]
		copy(store.rectangles[at:], store.rectangles[at+1:])
		store.rectangles = store.rectangles[:len(store.rectangles)-1]
	}
	store.reindexFrom(first)
	return Update{First: first, Count: len(store.rectangles) - first, Shrink: true}, nil
}

// rectangleOf is the render instance a scene change describes.
func rectangleOf(change scene.Change) Rectangle {
	var shape scene.Shape
	if change.Shape != nil {
		shape = *change.Shape
	}
	return Rectangle{
		ID:       change.ID,
		Kind:     change.Kind,
		Bounds:   change.Bounds,
		Color:    change.Color,
		Radius:   change.Radius,
		Points:   shape.Points,
		EdgeMask: change.EdgeMask,
		Shade:    shape.Shade,
		Shadow:   shape.Shadow,
	}
}

// reindexFrom repoints every slot whose instance is at or after index,
// after an insertion shifted the tail of the instance list.
func (store *Store) reindexFrom(index int) {
	for position := index; position < len(store.rectangles); position++ {
		if slot, ok := store.rectangles[position].ID.Slot(); ok {
			store.instanceBySlot[slot] = uint32(position + 1)
		}
	}
}

// exists reports whether the store already holds the node with this identity.
func (store *Store) exists(id scene.ID) bool {
	slot, ok := id.Slot()
	return ok && int(slot) < len(store.nodes) && store.nodes[slot].alive && store.nodes[slot].id == id
}

func (store *Store) upsert(change scene.Change) (bool, int, error) {
	slot, ok := change.ID.Slot()
	if !ok {
		return false, -1, fmt.Errorf("render: invalid node ID %d", change.ID)
	}
	store.ensureSlots(slot)
	current := &store.nodes[slot]
	if current.alive {
		if current.id != change.ID {
			return false, -1, fmt.Errorf("render: slot %d still owns node %d", slot, current.id)
		}
		// A shape may turn from a rectangle into a triangle or a shadow and back without
		// moving: it keeps its place in paint order. Anything else about its
		// place in the tree is structural.
		if current.parent != change.Parent || current.previous != change.PreviousSibling || (current.kind != change.Kind && (current.kind == scene.KindGroup || change.Kind == scene.KindGroup)) {
			return false, -1, fmt.Errorf("render: structural mutation requires delete and insert for node %d", change.ID)
		}
		current.kind = change.Kind
		instance := store.instanceIndex(slot, change.ID)
		if instance >= 0 {
			// Update the drawn geometry in place, leaving Layer, Clipped and
			// Clip for the application layer to stamp.
			rectangle := &store.rectangles[instance]
			rectangle.Kind = change.Kind
			rectangle.Bounds = change.Bounds
			rectangle.Color = change.Color
			rectangle.Radius = change.Radius
			if change.Shape != nil {
				rectangle.Points = change.Shape.Points
				rectangle.Shade = change.Shape.Shade
				rectangle.Shadow = change.Shape.Shadow
			}
			rectangle.EdgeMask = change.EdgeMask
		}
		return false, instance, nil
	}

	current.id = change.ID
	current.parent = change.Parent
	current.previous = change.PreviousSibling
	current.kind = change.Kind
	current.alive = true

	if change.Parent == 0 {
		if store.root != 0 && store.root != change.ID {
			return false, -1, fmt.Errorf("render: multiple roots %d and %d", store.root, change.ID)
		}
		store.root = change.ID
		return true, -1, nil
	}

	parent, err := store.live(change.Parent)
	if err != nil {
		return false, -1, fmt.Errorf("render: parent for node %d: %w", change.ID, err)
	}
	if parent.kind != scene.KindGroup {
		return false, -1, fmt.Errorf("render: parent %d is not a group", change.Parent)
	}

	if change.PreviousSibling == 0 {
		current.next = parent.first
		if parent.first != 0 {
			next, err := store.live(parent.first)
			if err != nil {
				return false, -1, err
			}
			next.previous = change.ID
		} else {
			parent.last = change.ID
		}
		parent.first = change.ID
	} else {
		previous, err := store.live(change.PreviousSibling)
		if err != nil {
			return false, -1, fmt.Errorf("render: previous sibling for node %d: %w", change.ID, err)
		}
		if previous.parent != change.Parent {
			return false, -1, fmt.Errorf("render: previous sibling %d has a different parent", change.PreviousSibling)
		}
		current.next = previous.next
		previous.next = change.ID
		if current.next != 0 {
			next, err := store.live(current.next)
			if err != nil {
				return false, -1, err
			}
			next.previous = change.ID
		} else {
			parent.last = change.ID
		}
	}

	return true, -1, nil
}

func (store *Store) remove(id scene.ID) error {
	slot, ok := id.Slot()
	if !ok || int(slot) >= len(store.nodes) {
		return fmt.Errorf("render: delete references invalid node %d", id)
	}
	current := &store.nodes[slot]
	if !current.alive || current.id != id {
		return fmt.Errorf("render: delete references stale node %d", id)
	}

	if current.parent == 0 {
		if store.root == id {
			store.root = 0
		}
	} else {
		parent, err := store.live(current.parent)
		if err != nil {
			return err
		}
		if current.previous == 0 {
			parent.first = current.next
		} else {
			previous, err := store.live(current.previous)
			if err != nil {
				return err
			}
			previous.next = current.next
		}
		if current.next == 0 {
			parent.last = current.previous
		} else {
			next, err := store.live(current.next)
			if err != nil {
				return err
			}
			next.previous = current.previous
		}
	}

	*current = node{}
	store.instanceBySlot[slot] = 0
	return nil
}

// rebuild re-flattens the instance list after a structural change the
// incremental paths cannot express: a batch mixing deletes with upserts, a
// shape inserted somewhere other than the root's tail, or an insertion whose
// previous sibling has no instance.
//
// Geometry lives only in the flattened rectangles. A live shape reaches its
// rectangle through instanceBySlot; a shape this batch inserted carries the
// freshInstance marker and is taken from changes instead. Deletions compact the
// list in place. Insertions first compute the new paint order from the retained
// tree, then rewrite the list from the back: an existing instance only ever
// moves to a later position, so a descending write cannot overwrite an instance
// it has not read yet.
func (store *Store) rebuild(changes []scene.Change, inserted []uint32) error {
	kept := store.rectangles[:0]
	for _, rectangle := range store.rectangles {
		slot, ok := rectangle.ID.Slot()
		if !ok {
			continue
		}
		current := &store.nodes[slot]
		if current.alive && current.id == rectangle.ID {
			kept = append(kept, rectangle)
			store.instanceBySlot[slot] = uint32(len(kept))
		}
	}
	store.rectangles = kept
	if len(inserted) == 0 {
		return nil
	}
	if store.root == 0 {
		return fmt.Errorf("render: inserted shapes without a root")
	}

	// A batch that deleted something and then appended at the root's tail is
	// still an append once the deletions are compacted, which is the common
	// shape of a delete: skip the walk and the descending rewrite entirely.
	last := scene.ID(0)
	if count := len(store.rectangles); count > 0 {
		last = store.rectangles[count-1].ID
	}
	tail := true
	for _, index := range inserted {
		change := changes[index]
		if change.Parent != store.root || change.PreviousSibling != last {
			tail = false
			break
		}
		last = change.ID
	}
	if tail {
		for _, index := range inserted {
			change := changes[index]
			slot, _ := change.ID.Slot()
			store.instanceBySlot[slot] = uint32(len(store.rectangles) + 1)
			store.rectangles = append(store.rectangles, rectangleOf(change))
		}
		return nil
	}

	store.order = store.order[:0]
	store.scratch = store.scratch[:0]
	store.scratch = append(store.scratch, store.root)
	for len(store.scratch) > 0 {
		last := len(store.scratch) - 1
		id := store.scratch[last]
		store.scratch = store.scratch[:last]
		current, err := store.live(id)
		if err != nil {
			return err
		}
		if current.kind == scene.KindRectangle || current.kind == scene.KindTriangle || current.kind == scene.KindShadow {
			store.order = append(store.order, id)
		}
		for child := current.last; child != 0; {
			store.scratch = append(store.scratch, child)
			childNode, err := store.live(child)
			if err != nil {
				return err
			}
			child = childNode.previous
		}
	}

	count := len(store.order)
	if count < len(store.rectangles) {
		store.rectangles = store.rectangles[:count]
	}
	for len(store.rectangles) < count {
		store.rectangles = append(store.rectangles, Rectangle{})
	}
	for position := count - 1; position >= 0; position-- {
		id := store.order[position]
		slot, _ := id.Slot()
		rectangle, err := store.rectangleFor(slot, id, changes, inserted)
		if err != nil {
			return err
		}
		store.rectangles[position] = rectangle
		store.instanceBySlot[slot] = uint32(position + 1)
	}
	store.rectangles = store.rectangles[:count]
	return nil
}

// rectangleFor is the geometry of one shape during a rebuild: the instance the
// store already held, or the batch change that inserted it.
func (store *Store) rectangleFor(slot uint32, id scene.ID, changes []scene.Change, inserted []uint32) (Rectangle, error) {
	encoded := store.instanceBySlot[slot]
	if encoded&freshInstance != 0 {
		index := encoded &^ freshInstance
		if int(index) >= len(inserted) {
			return Rectangle{}, fmt.Errorf("render: shape %d has no insert record", id)
		}
		return rectangleOf(changes[inserted[index]]), nil
	}
	if index := int(encoded - 1); index >= 0 && index < len(store.rectangles) && store.rectangles[index].ID == id {
		return store.rectangles[index], nil
	}
	return Rectangle{}, fmt.Errorf("render: shape %d lost its geometry", id)
}

func (store *Store) ensureSlots(slot uint32) {
	required := int(slot) + 1
	if required <= len(store.nodes) {
		return
	}
	store.nodes = append(store.nodes, make([]node, required-len(store.nodes))...)
	store.instanceBySlot = append(store.instanceBySlot, make([]uint32, required-len(store.instanceBySlot))...)
}

func (store *Store) live(id scene.ID) (*node, error) {
	slot, ok := id.Slot()
	if !ok || int(slot) >= len(store.nodes) {
		return nil, fmt.Errorf("node %d has no render slot", id)
	}
	current := &store.nodes[slot]
	if !current.alive || current.id != id {
		return nil, fmt.Errorf("node %d is not live", id)
	}
	return current, nil
}

func (store *Store) instanceIndex(slot uint32, id scene.ID) int {
	encoded := store.instanceBySlot[slot]
	if encoded == 0 {
		return -1
	}
	index := int(encoded - 1)
	if index >= len(store.rectangles) || store.rectangles[index].ID != id {
		return -1
	}
	return index
}
