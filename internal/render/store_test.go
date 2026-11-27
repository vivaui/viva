package render

import (
	"math/rand"
	"slices"
	"testing"

	"github.com/vivaui/viva/internal/scene"
)

func TestStorePreservesTreePaintOrder(t *testing.T) {
	graph := scene.New()
	firstGroup, _ := graph.AddGroup(graph.Root())
	first, _ := graph.AddRectangle(firstGroup, scene.Rect{X: 1}, scene.Color{})
	second, _ := graph.AddRectangle(firstGroup, scene.Rect{X: 2}, scene.Color{})
	secondGroup, _ := graph.AddGroup(graph.Root())
	third, _ := graph.AddRectangle(secondGroup, scene.Rect{X: 3}, scene.Color{})

	var store Store
	update, err := store.Apply(graph.DrainChanges(nil))
	if err != nil {
		t.Fatal(err)
	}
	if !update.Rebuild || update.Count != 3 {
		t.Fatalf("initial update = %+v", update)
	}

	want := []scene.ID{first, second, third}
	for index, rectangle := range store.Rectangles() {
		if rectangle.ID != want[index] {
			t.Fatalf("rectangle %d ID = %d, want %d", index, rectangle.ID, want[index])
		}
	}
}

func TestStoreUpdatesOneInstanceWithoutRebuild(t *testing.T) {
	graph := scene.NewWithCapacity(10_001)
	var target scene.ID
	for index := 0; index < 10_000; index++ {
		id, _ := graph.AddRectangle(graph.Root(), scene.Rect{X: float32(index)}, scene.Color{Alpha: 1})
		if index == 5_000 {
			target = id
		}
	}

	var store Store
	if _, err := store.Apply(graph.DrainChanges(nil)); err != nil {
		t.Fatal(err)
	}
	graph.SetRectangle(target, scene.Rect{X: 42}, scene.Color{Red: 1, Alpha: 1})
	update, err := store.Apply(graph.DrainChanges(nil))
	if err != nil {
		t.Fatal(err)
	}
	if update != (Update{First: 5_000, Count: 1}) {
		t.Fatalf("localized update = %+v", update)
	}
	if got := store.Rectangles()[5_000]; got.Bounds.X != 42 || got.Color.Red != 1 {
		t.Fatalf("updated rectangle = %+v", got)
	}
}

func TestStoreDeleteCompactsWithoutChangingOrder(t *testing.T) {
	graph := scene.New()
	first, _ := graph.AddRectangle(graph.Root(), scene.Rect{X: 1}, scene.Color{})
	second, _ := graph.AddRectangle(graph.Root(), scene.Rect{X: 2}, scene.Color{})
	third, _ := graph.AddRectangle(graph.Root(), scene.Rect{X: 3}, scene.Color{})
	var store Store
	store.Apply(graph.DrainChanges(nil))

	graph.Remove(second)
	update, err := store.Apply(graph.DrainChanges(nil))
	if err != nil {
		t.Fatal(err)
	}
	if update.Rebuild || !update.Shrink || update.First != 1 || update.Count != 1 {
		t.Fatalf("delete update = %+v", update)
	}
	if store.Rectangles()[0].ID != first || store.Rectangles()[1].ID != third {
		t.Fatalf("remaining rectangles = %+v", store.Rectangles())
	}
}

func TestStoreDeletesSubtreeChildrenBeforeParent(t *testing.T) {
	graph := scene.New()
	group, _ := graph.AddGroup(graph.Root())
	graph.AddRectangle(group, scene.Rect{X: 1}, scene.Color{})
	graph.AddRectangle(group, scene.Rect{X: 2}, scene.Color{})
	var store Store
	store.Apply(graph.DrainChanges(nil))

	graph.Remove(group)
	update, err := store.Apply(graph.DrainChanges(nil))
	if err != nil {
		t.Fatal(err)
	}
	if update.Rebuild || !update.Shrink || update.First != 0 || update.Count != 0 || len(store.Rectangles()) != 0 {
		t.Fatalf("subtree delete update = %+v, rectangles = %+v", update, store.Rectangles())
	}
}

func TestStoreNoChangesDoesNoWork(t *testing.T) {
	var store Store
	update, err := store.Apply(nil)
	if err != nil {
		t.Fatal(err)
	}
	if !update.Empty() {
		t.Fatalf("empty apply = %+v", update)
	}
}

func BenchmarkStoreLocalizedUpdate(b *testing.B) {
	graph := scene.NewWithCapacity(10_001)
	var target scene.ID
	for index := 0; index < 10_000; index++ {
		id, _ := graph.AddRectangle(graph.Root(), scene.Rect{X: float32(index)}, scene.Color{Alpha: 1})
		if index == 5_000 {
			target = id
		}
	}
	changes := make([]scene.Change, 0, 10_001)
	changes = graph.DrainChanges(changes)
	var store Store
	if _, err := store.Apply(changes); err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for index := range b.N {
		graph.SetRectangle(target, scene.Rect{X: float32(index & 1)}, scene.Color{Alpha: 1})
		changes = graph.DrainChanges(changes[:0])
		update, err := store.Apply(changes)
		if err != nil || update.Count != 1 || update.Rebuild {
			b.Fatalf("localized update = %+v, err = %v", update, err)
		}
	}
}

// The scene relinks a deleted node's successor without reporting it. When that
// successor is queued earlier in the batch for another reason (a recolor), its
// upsert names the post-delete previous sibling, and the store must still
// accept it. Before deletes were applied first this failed with "structural
// mutation" and abandoned the whole batch.
func TestStoreAcceptsSuccessorUpsertQueuedBeforeItsPredecessorsDelete(t *testing.T) {
	graph := scene.New()
	root := graph.Root()
	kept, _ := graph.AddRectangle(root, scene.Rect{X: 1}, scene.Color{Alpha: 1})
	removed, _ := graph.AddRectangle(root, scene.Rect{X: 2}, scene.Color{Alpha: 1})
	successor, _ := graph.AddRectangle(root, scene.Rect{X: 3}, scene.Color{Alpha: 1})
	tail, _ := graph.AddRectangle(root, scene.Rect{X: 4}, scene.Color{Alpha: 1})

	var store Store
	if _, err := store.Apply(graph.DrainChanges(nil)); err != nil {
		t.Fatal(err)
	}

	recolored := scene.Color{Red: 1, Alpha: 1}
	graph.SetRectangle(successor, scene.Rect{X: 3}, recolored) // queued first
	graph.SetRectangle(tail, scene.Rect{X: 4}, recolored)
	graph.Remove(removed) // queued after the successor
	changes := graph.DrainChanges(nil)
	if len(changes) != 3 || changes[0].ID != successor || changes[2].Operation != scene.OperationDelete {
		t.Fatalf("the scenario no longer queues the successor before the delete: %+v", changes)
	}

	update, err := store.Apply(changes)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	// Everything from the deleted instance on moved, and the list is shorter.
	if update != (Update{First: 1, Count: 2, Shrink: true}) {
		t.Fatalf("update = %+v, want the suffix from the deleted instance, shrunk", update)
	}
	rectangles := store.Rectangles()
	if len(rectangles) != 3 || rectangles[0].ID != kept || rectangles[1].ID != successor || rectangles[2].ID != tail {
		t.Fatalf("instances = %+v", rectangles)
	}
	if rectangles[1].Color != recolored || rectangles[2].Color != recolored {
		t.Fatalf("recolors were lost: %+v", rectangles)
	}

	// The links stay coherent for later batches: removing the new neighbor works.
	graph.Remove(kept)
	graph.SetRectangle(successor, scene.Rect{X: 3}, scene.Color{Blue: 1, Alpha: 1})
	if _, err := store.Apply(graph.DrainChanges(nil)); err != nil {
		t.Fatalf("second apply: %v", err)
	}
	if got := store.Rectangles(); len(got) != 2 || got[0].ID != successor || got[0].Color.Blue != 1 {
		t.Fatalf("after the second batch: %+v", got)
	}
}

func order(store *Store) []scene.ID {
	var ids []scene.ID
	for _, rectangle := range store.Rectangles() {
		ids = append(ids, rectangle.ID)
	}
	return ids
}

// A shape inserted into the middle of paint order lands there, and an existing
// node changed in the same batch (which already names the new node as its
// predecessor) does not get the batch rejected, wherever it sits in the queue.
func TestStoreInsertsInTheMiddleAlongsideAChangeToItsSuccessor(t *testing.T) {
	graph := scene.New()
	first, _ := graph.AddRectangle(graph.Root(), scene.Rect{X: 1}, scene.Color{})
	second, _ := graph.AddRectangle(graph.Root(), scene.Rect{X: 2}, scene.Color{})
	third, _ := graph.AddRectangle(graph.Root(), scene.Rect{X: 3}, scene.Color{})
	var store Store
	if _, err := store.Apply(graph.DrainChanges(nil)); err != nil {
		t.Fatal(err)
	}

	graph.SetRectangle(second, scene.Rect{X: 20}, scene.Color{Red: 1}) // queued before the insert
	inserted, _ := graph.InsertTriangle(graph.Root(), first, [3]scene.Point{{X: 1}, {X: 2}, {X: 3}}, scene.Color{Green: 1}, 5, scene.Shade{}, nil)
	update, err := store.Apply(graph.DrainChanges(nil))
	if err != nil {
		t.Fatal(err)
	}
	// The insert splices its tail into place instead of re-flattening the
	// page, so the update runs from the inserted instance to the end.
	if update.Rebuild || update.First != 1 || update.First+update.Count != 4 {
		t.Fatalf("insert update = %+v", update)
	}
	want := []scene.ID{first, inserted, second, third}
	for index, id := range order(&store) {
		if id != want[index] {
			t.Fatalf("paint order = %v, want %v", order(&store), want)
		}
	}
	if got := store.Rectangles()[1]; got.Kind != scene.KindTriangle || got.Points[2].X != 3 || got.EdgeMask != 5 {
		t.Fatalf("inserted triangle = %+v", got)
	}
	if got := store.Rectangles()[2]; got.Bounds.X != 20 || got.Color.Red != 1 {
		t.Fatalf("successor lost its change: %+v", got)
	}
}

// A node that draws a rectangle in one frame and a triangle in the next keeps
// its place and is an ordinary localized update, not a rebuild.
func TestStoreKeepsPlaceWhenAShapeChangesKind(t *testing.T) {
	graph := scene.New()
	graph.AddRectangle(graph.Root(), scene.Rect{X: 1}, scene.Color{})
	flip, _ := graph.AddRectangle(graph.Root(), scene.Rect{X: 2}, scene.Color{})
	graph.AddRectangle(graph.Root(), scene.Rect{X: 3}, scene.Color{})
	var store Store
	store.Apply(graph.DrainChanges(nil))

	points := [3]scene.Point{{X: 5}, {X: 6}, {X: 7}}
	color := scene.Color{Blue: 1}
	coverage := scene.StrokeCoverage{
		Points:   [3]scene.Point{{X: 5}, {X: 5, Y: 10}, {X: 7}},
		EdgeMask: scene.StrokeQuadEdgeMask | 1,
	}
	graph.SetTriangle(flip, points, color, 3, scene.Shade{}, &coverage)
	update, err := store.Apply(graph.DrainChanges(nil))
	if err != nil {
		t.Fatal(err)
	}
	if update != (Update{First: 1, Count: 1}) {
		t.Fatalf("kind change update = %+v", update)
	}
	if got := store.Rectangles()[1]; got.ID != flip || got.Kind != scene.KindTriangle || got.Points != points || got.Bounds != (scene.Rect{}) ||
		got.EdgeMask != 3 || got.Coverage == nil || *got.Coverage != coverage {
		t.Fatalf("flipped shape = %+v", got)
	}
	updatedCoverage := coverage
	if graph.SetTriangle(flip, points, color, 3, scene.Shade{}, &updatedCoverage) {
		t.Fatal("identical coverage queued an update")
	}
	updatedCoverage.Points[1].Y++
	graph.SetTriangle(flip, points, color, 3, scene.Shade{}, &updatedCoverage)
	if update, err := store.Apply(graph.DrainChanges(nil)); err != nil || update != (Update{First: 1, Count: 1}) {
		t.Fatalf("coverage update = %+v, error = %v", update, err)
	}
	if got := store.Rectangles()[1].Coverage; got == nil || *got != updatedCoverage {
		t.Fatalf("retained coverage = %v, want %+v", got, updatedCoverage)
	}
	graph.SetRoundedRectangle(flip, scene.Rect{X: 8}, scene.Color{}, 3)
	if _, err := store.Apply(graph.DrainChanges(nil)); err != nil {
		t.Fatal(err)
	}
	if got := store.Rectangles()[1]; got.Kind != scene.KindRectangle || got.Bounds.X != 8 || got.Radius != 3 || got.Coverage != nil {
		t.Fatalf("flipped back = %+v", got)
	}
}

func TestStoreAppendsTailWithoutRebuild(t *testing.T) {
	graph := scene.NewWithCapacity(10_001)
	for index := 0; index < 10000; index++ {
		graph.AddRectangle(graph.Root(), scene.Rect{X: float32(index)}, scene.Color{Alpha: 1})
	}
	var store Store
	if _, err := store.Apply(graph.DrainChanges(nil)); err != nil {
		t.Fatal(err)
	}
	added, _ := graph.AddRectangle(graph.Root(), scene.Rect{X: 10000}, scene.Color{Red: 1, Alpha: 1})
	update, err := store.Apply(graph.DrainChanges(nil))
	if err != nil {
		t.Fatal(err)
	}
	if update.Rebuild || update != (Update{First: 10000, Count: 1}) {
		t.Fatalf("append update = %+v, want a one-instance suffix", update)
	}
	rectangles := store.Rectangles()
	if len(rectangles) != 10001 || rectangles[10000].ID != added || rectangles[10000].Color.Red != 1 {
		t.Fatalf("appended rectangles = %d, last %+v", len(rectangles), rectangles[len(rectangles)-1])
	}
}

func TestStoreHandlesAnUpdateAndAnAppendInOneBatch(t *testing.T) {
	graph := scene.NewWithCapacity(101)
	var target scene.ID
	for index := 0; index < 100; index++ {
		id, _ := graph.AddRectangle(graph.Root(), scene.Rect{X: float32(index)}, scene.Color{Alpha: 1})
		if index == 5 {
			target = id
		}
	}
	var store Store
	if _, err := store.Apply(graph.DrainChanges(nil)); err != nil {
		t.Fatal(err)
	}
	graph.SetRectangle(target, scene.Rect{X: 42}, scene.Color{Red: 1, Alpha: 1})
	added, _ := graph.AddRectangle(graph.Root(), scene.Rect{X: 100}, scene.Color{Blue: 1, Alpha: 1})
	update, err := store.Apply(graph.DrainChanges(nil))
	if err != nil {
		t.Fatal(err)
	}
	if update.Rebuild || update.First > 5 || update.First+update.Count != 101 {
		t.Fatalf("mixed update = %+v, want a suffix range covering the change and the append", update)
	}
	if store.Rectangles()[5].Bounds.X != 42 || store.Rectangles()[100].ID != added {
		t.Fatalf("rectangles = %+v / %+v", store.Rectangles()[5], store.Rectangles()[100])
	}
}

func TestStoreInsertsInsideAGroupRebuildsInPaintOrder(t *testing.T) {
	graph := scene.New()
	first, _ := graph.AddRectangle(graph.Root(), scene.Rect{X: 1}, scene.Color{})
	group, _ := graph.AddGroup(graph.Root())
	inside, _ := graph.AddRectangle(group, scene.Rect{X: 2}, scene.Color{})
	last, _ := graph.AddRectangle(graph.Root(), scene.Rect{X: 3}, scene.Color{})
	var store Store
	if _, err := store.Apply(graph.DrainChanges(nil)); err != nil {
		t.Fatal(err)
	}

	// A new first child of the group is neither a root append nor a splice the
	// previous-sibling path can place without walking the tree.
	inserted, ok := graph.InsertRoundedRectangle(group, 0, scene.Rect{X: 4}, scene.Color{Red: 1}, 2)
	if !ok {
		t.Fatal("insert failed")
	}
	update, err := store.Apply(graph.DrainChanges(nil))
	if err != nil {
		t.Fatal(err)
	}
	if !update.Rebuild || update.Count != 4 {
		t.Fatalf("group insert update = %+v", update)
	}
	rectangles := store.Rectangles()
	want := []scene.ID{first, inserted, inside, last}
	if len(rectangles) != len(want) {
		t.Fatalf("rectangles = %+v", rectangles)
	}
	for index, id := range want {
		if rectangles[index].ID != id {
			t.Fatalf("rectangles[%d].ID = %d, want %d", index, rectangles[index].ID, id)
		}
	}
	if got := rectangles[1]; got.Bounds.X != 4 || got.Radius != 2 || got.Color.Red != 1 {
		t.Fatalf("inserted rectangle = %+v", got)
	}
}

func TestStoreDeletesOneShapeAndInsertsAnotherBeforeIt(t *testing.T) {
	graph := scene.New()
	first, _ := graph.AddRectangle(graph.Root(), scene.Rect{X: 1}, scene.Color{})
	second, _ := graph.AddRectangle(graph.Root(), scene.Rect{X: 2}, scene.Color{})
	third, _ := graph.AddRectangle(graph.Root(), scene.Rect{X: 3}, scene.Color{})
	var store Store
	if _, err := store.Apply(graph.DrainChanges(nil)); err != nil {
		t.Fatal(err)
	}

	if !graph.Remove(second) {
		t.Fatal("remove failed")
	}
	inserted, ok := graph.InsertRoundedRectangle(graph.Root(), first, scene.Rect{X: 4}, scene.Color{}, 0)
	if !ok {
		t.Fatal("insert failed")
	}
	update, err := store.Apply(graph.DrainChanges(nil))
	if err != nil {
		t.Fatal(err)
	}
	// One instance left and one took its place: the suffix from there, same length.
	if update != (Update{First: 1, Count: 2}) {
		t.Fatalf("delete plus insert update = %+v", update)
	}
	rectangles := store.Rectangles()
	want := []scene.ID{first, inserted, third}
	for index, id := range want {
		if index >= len(rectangles) || rectangles[index].ID != id {
			t.Fatalf("rectangles = %+v, want IDs %v", rectangles, want)
		}
	}
	if len(rectangles) != len(want) {
		t.Fatalf("rectangles = %+v", rectangles)
	}

	// The renumbered slots must let the next localized update find the instance.
	if !graph.SetRectangle(inserted, scene.Rect{X: 9}, scene.Color{}) {
		t.Fatal("update failed")
	}
	update, err = store.Apply(graph.DrainChanges(nil))
	if err != nil {
		t.Fatal(err)
	}
	if update != (Update{First: 1, Count: 1}) {
		t.Fatalf("follow-up update = %+v", update)
	}
	if got := store.Rectangles()[1]; got.ID != inserted || got.Bounds.X != 9 {
		t.Fatalf("updated rectangle = %+v", got)
	}
}

func TestStoreDeletesNonContiguousInstancesInOrder(t *testing.T) {
	graph := scene.New()
	root := graph.Root()
	first, _ := graph.AddRectangle(root, scene.Rect{X: 1}, scene.Color{})
	second, _ := graph.AddRectangle(root, scene.Rect{X: 2}, scene.Color{})
	third, _ := graph.AddRectangle(root, scene.Rect{X: 3}, scene.Color{})
	fourth, _ := graph.AddRectangle(root, scene.Rect{X: 4}, scene.Color{})
	var store Store
	if _, err := store.Apply(graph.DrainChanges(nil)); err != nil {
		t.Fatal(err)
	}
	graph.Remove(first)
	graph.Remove(third)
	update, err := store.Apply(graph.DrainChanges(nil))
	if err != nil {
		t.Fatal(err)
	}
	if update.Rebuild || !update.Shrink || update.First != 0 || update.Count != 2 {
		t.Fatalf("delete update = %+v", update)
	}
	got := store.Rectangles()
	if len(got) != 2 || got[0].ID != second || got[1].ID != fourth {
		t.Fatalf("remaining = %+v", got)
	}
}

// storeModel is the tree a random test builds alongside a scene: the paint
// order the store must end each batch in is its depth-first walk.
type storeModel struct {
	id       scene.ID
	group    bool
	color    scene.Color
	children []*storeModel
}

func (model *storeModel) walk(visit func(parent, node *storeModel)) {
	for _, child := range model.children {
		visit(model, child)
		if child.group {
			child.walk(visit)
		}
	}
}

// TestStoreUpdatesKeepABackendInStep applies random batches of appends,
// insertions, deletions, recolors and groups, and checks after each one that
// the store holds the scene's paint order and that a backend which applies only
// the reported Update (rebuild, or a range that may grow the list or be cut
// short with Shrink) holds exactly the store's instances.
func TestStoreUpdatesKeepABackendInStep(t *testing.T) {
	random := rand.New(rand.NewSource(7))
	graph := scene.New()
	root := &storeModel{id: graph.Root(), group: true}
	var store Store
	var backend []Rectangle
	color := func() scene.Color { return scene.Color{Red: float32(random.Intn(1000)), Alpha: 1} }
	pick := func(groups bool) (parent, node *storeModel, index int) {
		var parents, nodes []*storeModel
		root.walk(func(p, n *storeModel) {
			if groups || !n.group {
				parents, nodes = append(parents, p), append(nodes, n)
			}
		})
		if len(nodes) == 0 {
			return nil, nil, -1
		}
		at := random.Intn(len(nodes))
		return parents[at], nodes[at], slices.Index(parents[at].children, nodes[at])
	}
	groupOf := func() *storeModel {
		groups := []*storeModel{root}
		root.walk(func(_, n *storeModel) {
			if n.group {
				groups = append(groups, n)
			}
		})
		return groups[random.Intn(len(groups))]
	}
	for batch := 0; batch < 3000; batch++ {
		for operations := 1 + random.Intn(5); operations > 0; operations-- {
			switch random.Intn(6) {
			case 0, 1: // insert a shape after a random sibling, or first
				parent := groupOf()
				at := random.Intn(len(parent.children) + 1)
				previous := scene.ID(0)
				if at > 0 {
					previous = parent.children[at-1].id
				}
				shade := color()
				id, ok := graph.InsertRoundedRectangle(parent.id, previous, scene.Rect{Width: 1}, shade, 0)
				if !ok {
					t.Fatal("insert failed")
				}
				parent.children = slices.Insert(parent.children, at, &storeModel{id: id, color: shade})
			case 2: // remove a shape or a whole group
				if parent, node, index := pick(true); node != nil {
					if !graph.Remove(node.id) {
						t.Fatal("remove failed")
					}
					parent.children = slices.Delete(parent.children, index, index+1)
				}
			case 3, 4: // recolor a shape
				if _, node, _ := pick(false); node != nil {
					node.color.Red++ // a change, which the scene queues
					if !graph.SetRectangle(node.id, scene.Rect{Width: 1}, node.color) {
						t.Fatal("recolor failed")
					}
				}
			case 5: // a group holding a few shapes, at the end of the page
				id, ok := graph.AddGroup(graph.Root())
				if !ok {
					t.Fatal("group failed")
				}
				group := &storeModel{id: id, group: true}
				for count := random.Intn(3); count > 0; count-- {
					shade := color()
					child, _ := graph.AddRectangle(id, scene.Rect{Width: 1}, shade)
					group.children = append(group.children, &storeModel{id: child, color: shade})
				}
				root.children = append(root.children, group)
			}
		}

		update, err := store.Apply(graph.DrainChanges(nil))
		if err != nil {
			t.Fatalf("batch %d: %v", batch, err)
		}
		rectangles := store.Rectangles()
		switch {
		case update.Rebuild:
			backend = append(backend[:0], rectangles...)
		case update.Count > 0 || update.Shrink:
			if end := update.First + update.Count; end > len(backend) {
				backend = append(backend, make([]Rectangle, end-len(backend))...)
			}
			copy(backend[update.First:], rectangles[update.First:update.First+update.Count])
			if update.Shrink {
				backend = backend[:len(rectangles)]
			}
		}
		if !slices.Equal(backend, rectangles) {
			t.Fatalf("batch %d: update %+v left the backend with %d instances, the store %d", batch, update, len(backend), len(rectangles))
		}
		var want []*storeModel
		root.walk(func(_, n *storeModel) {
			if !n.group {
				want = append(want, n)
			}
		})
		if len(rectangles) != len(want) {
			t.Fatalf("batch %d: %d instances, want %d", batch, len(rectangles), len(want))
		}
		for index, node := range want {
			if rectangles[index].ID != node.id || rectangles[index].Color != node.color {
				t.Fatalf("batch %d: instance %d is %d %v, want %d %v", batch, index, rectangles[index].ID, rectangles[index].Color, node.id, node.color)
			}
		}
	}
}

// Deleting most of a large page gives back the batch scratch that page needed,
// once the page left is small.
func TestStoreReleasesTheScratchOfAPageThatShrank(t *testing.T) {
	graph := scene.NewWithCapacity(20_001)
	ids := make([]scene.ID, 0, 20_000)
	for index := 0; index < 20_000; index++ {
		id, _ := graph.AddRectangle(graph.Root(), scene.Rect{X: float32(index)}, scene.Color{Alpha: 1})
		ids = append(ids, id)
	}
	var store Store
	if _, err := store.Apply(graph.DrainChanges(nil)); err != nil {
		t.Fatal(err)
	}
	for _, id := range ids[1:] {
		graph.Remove(id)
	}
	if _, err := store.Apply(graph.DrainChanges(nil)); err != nil {
		t.Fatal(err)
	}
	if cap(store.deleted) > 4096 {
		t.Fatalf("the store keeps %d scratch for a one-shape page", cap(store.deleted))
	}
}

// A page that stays large keeps the scratch its batches use, so its next large
// batch does not allocate again.
func TestStoreKeepsTheScratchOfAPageThatStaysLarge(t *testing.T) {
	graph := scene.NewWithCapacity(20_001)
	ids := make([]scene.ID, 0, 20_000)
	for index := 0; index < 20_000; index++ {
		id, _ := graph.AddRectangle(graph.Root(), scene.Rect{X: float32(index)}, scene.Color{Alpha: 1})
		ids = append(ids, id)
	}
	var store Store
	if _, err := store.Apply(graph.DrainChanges(nil)); err != nil {
		t.Fatal(err)
	}
	for _, id := range ids[:10_000] {
		graph.Remove(id)
	}
	if _, err := store.Apply(graph.DrainChanges(nil)); err != nil {
		t.Fatal(err)
	}
	if cap(store.deleted) < 10_000 {
		t.Fatalf("the store dropped the scratch of a %d-instance page: %d", len(store.Rectangles()), cap(store.deleted))
	}
}
