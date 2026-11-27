package scene

import "testing"

func TestSceneCoalescesLocalizedUpdates(t *testing.T) {
	graph := New()
	rectangle, ok := graph.AddRectangle(graph.Root(), Rect{Width: 20, Height: 10}, Color{Alpha: 1})
	if !ok {
		t.Fatal("failed to add rectangle")
	}

	changes := graph.DrainChanges(nil)
	if len(changes) != 2 {
		t.Fatalf("initial change count = %d, want 2", len(changes))
	}
	if graph.PendingChanges() != 0 {
		t.Fatalf("pending changes after drain = %d", graph.PendingChanges())
	}

	for index := 0; index < 100; index++ {
		graph.SetRectangle(
			rectangle,
			Rect{X: float32(index), Width: 20, Height: 10},
			Color{Red: float32(index) / 100, Alpha: 1},
		)
	}
	if graph.PendingChanges() != 1 {
		t.Fatalf("coalesced pending changes = %d, want 1", graph.PendingChanges())
	}

	changes = graph.DrainChanges(changes[:0])
	if len(changes) != 1 || changes[0].ID != rectangle {
		t.Fatalf("localized changes = %+v", changes)
	}
	if changes[0].Bounds.X != 99 {
		t.Fatalf("final bounds = %+v", changes[0].Bounds)
	}

	changes = graph.DrainChanges(changes[:0])
	if len(changes) != 0 {
		t.Fatalf("unchanged drain produced %d changes", len(changes))
	}
}

func TestSceneRejectsStaleIDsAfterReuse(t *testing.T) {
	graph := New()
	first, _ := graph.AddRectangle(graph.Root(), Rect{}, Color{})
	graph.DrainChanges(nil)

	if !graph.Remove(first) {
		t.Fatal("failed to remove live node")
	}
	changes := graph.DrainChanges(nil)
	if len(changes) != 1 || changes[0].Operation != OperationDelete {
		t.Fatalf("delete changes = %+v", changes)
	}

	reused, _ := graph.AddRectangle(graph.Root(), Rect{Width: 1}, Color{})
	if reused == first {
		t.Fatalf("reused ID did not advance generation: %v", reused)
	}
	if graph.SetRectangle(first, Rect{Width: 2}, Color{}) {
		t.Fatal("stale ID updated reused slot")
	}
}

func TestSceneRemovesSubtreeIncrementally(t *testing.T) {
	graph := New()
	group, _ := graph.AddGroup(graph.Root())
	first, _ := graph.AddRectangle(group, Rect{}, Color{})
	second, _ := graph.AddRectangle(group, Rect{}, Color{})
	graph.DrainChanges(nil)

	if !graph.Remove(group) {
		t.Fatal("failed to remove group")
	}
	changes := graph.DrainChanges(nil)
	if len(changes) != 3 {
		t.Fatalf("subtree delete count = %d, want 3", len(changes))
	}

	deleted := map[ID]bool{}
	for _, change := range changes {
		if change.Operation != OperationDelete {
			t.Fatalf("unexpected operation: %+v", change)
		}
		deleted[change.ID] = true
	}
	if !deleted[group] || !deleted[first] || !deleted[second] {
		t.Fatalf("deleted IDs = %+v", deleted)
	}
}

func TestSceneDropsUnpublishedCreateDelete(t *testing.T) {
	graph := New()
	graph.DrainChanges(nil)

	rectangle, _ := graph.AddRectangle(graph.Root(), Rect{}, Color{})
	graph.Remove(rectangle)
	if changes := graph.DrainChanges(nil); len(changes) != 0 {
		t.Fatalf("unpublished node produced changes: %+v", changes)
	}
}

func TestSceneRejectsInvalidParentsAndRootRemoval(t *testing.T) {
	graph := New()
	rectangle, _ := graph.AddRectangle(graph.Root(), Rect{}, Color{})
	if _, ok := graph.AddGroup(rectangle); ok {
		t.Fatal("rectangle accepted a child")
	}
	if graph.Remove(graph.Root()) {
		t.Fatal("root was removed")
	}
}

func BenchmarkSceneNoChanges(b *testing.B) {
	graph, output, _ := benchmarkScene(b, 10_000)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		output = graph.DrainChanges(output[:0])
	}
}

func BenchmarkSceneLocalizedUpdate(b *testing.B) {
	graph, output, target := benchmarkScene(b, 10_000)
	b.ReportAllocs()
	b.ResetTimer()
	for index := range b.N {
		bounds := Rect{X: float32(index & 1), Width: 20, Height: 20}
		if !graph.SetRectangle(target, bounds, Color{Alpha: 1}) {
			b.Fatal("localized update was rejected")
		}
		output = graph.DrainChanges(output[:0])
		if len(output) != 1 {
			b.Fatalf("change count = %d, want 1", len(output))
		}
	}
}

func BenchmarkSceneCreate10K(b *testing.B) {
	b.ReportAllocs()
	for range b.N {
		graph := NewWithCapacity(10_001)
		for index := 0; index < 10_000; index++ {
			graph.AddRectangle(
				graph.Root(),
				Rect{X: float32(index), Width: 20, Height: 20},
				Color{Alpha: 1},
			)
		}
		output := make([]Change, 0, 10_001)
		output = graph.DrainChanges(output)
		if len(output) != 10_001 {
			b.Fatalf("change count = %d", len(output))
		}
	}
}

func benchmarkScene(b *testing.B, count int) (*Scene, []Change, ID) {
	b.Helper()
	graph := New()
	var target ID
	for index := 0; index < count; index++ {
		id, _ := graph.AddRectangle(
			graph.Root(),
			Rect{X: float32(index), Width: 20, Height: 20},
			Color{Alpha: 1},
		)
		if index == count/2 {
			target = id
		}
	}
	output := make([]Change, 0, count+1)
	output = graph.DrainChanges(output)
	return graph, output[:0], target
}

func TestSceneInsertsAShapeInTheMiddleOfPaintOrder(t *testing.T) {
	graph := New()
	first, _ := graph.AddRectangle(graph.Root(), Rect{X: 1}, Color{})
	graph.AddRectangle(graph.Root(), Rect{X: 3}, Color{})
	graph.DrainChanges(nil)

	middle, ok := graph.InsertRoundedRectangle(graph.Root(), first, Rect{X: 2}, Color{}, 4)
	if !ok {
		t.Fatal("insert after a child failed")
	}
	head, ok := graph.InsertTriangle(graph.Root(), 0, [3]Point{{X: 1}}, Color{}, 5, Shade{}, nil)
	if !ok {
		t.Fatal("insert as the first child failed")
	}
	changes := graph.DrainChanges(nil)
	// Only the new nodes are reported: neighbours keep their content and the
	// store relinks them itself.
	if len(changes) != 2 {
		t.Fatalf("inserting two nodes reported %d changes: %+v", len(changes), changes)
	}
	for _, change := range changes {
		switch change.ID {
		case middle:
			if change.PreviousSibling != first || change.Kind != KindRectangle || change.Radius != 4 {
				t.Fatalf("middle change = %+v", change)
			}
		case head:
			if change.PreviousSibling != 0 || change.Kind != KindTriangle || change.EdgeMask != 5 {
				t.Fatalf("head change = %+v", change)
			}
		}
	}

	// Removing from the middle relinks around it.
	graph.Remove(middle)
	graph.DrainChanges(nil)
	if after, ok := graph.InsertRoundedRectangle(graph.Root(), first, Rect{X: 5}, Color{}, 0); !ok || after == 0 {
		t.Fatal("insert after a removal failed")
	}
	if _, ok := graph.InsertRoundedRectangle(graph.Root(), middle, Rect{}, Color{}, 0); ok {
		t.Fatal("inserted after a removed node")
	}
}

func TestSceneLeafChangesKindInPlace(t *testing.T) {
	graph := New()
	id, _ := graph.AddRectangle(graph.Root(), Rect{X: 1, Width: 5}, Color{Alpha: 1})
	graph.DrainChanges(nil)

	if !graph.SetTriangle(id, [3]Point{{X: 1}, {X: 2}, {X: 3}}, Color{Alpha: 1}, 5, Shade{}, nil) {
		t.Fatal("a rectangle did not become a triangle")
	}
	changes := graph.DrainChanges(nil)
	if len(changes) != 1 || changes[0].Kind != KindTriangle || changes[0].Bounds != (Rect{}) || changes[0].Shape == nil || changes[0].Shape.Points[2].X != 3 || changes[0].EdgeMask != 5 {
		t.Fatalf("rectangle to triangle = %+v", changes)
	}
	if graph.SetTriangle(id, [3]Point{{X: 1}, {X: 2}, {X: 3}}, Color{Alpha: 1}, 5, Shade{}, nil) {
		t.Fatal("setting the same triangle again reported a change")
	}
	if !graph.SetRoundedRectangle(id, Rect{X: 9}, Color{Alpha: 1}, 2) {
		t.Fatal("a triangle did not become a rectangle")
	}
	changes = graph.DrainChanges(changes[:0])
	if len(changes) != 1 || changes[0].Kind != KindRectangle || changes[0].Shape != nil || changes[0].Radius != 2 {
		t.Fatalf("triangle to rectangle = %+v", changes)
	}
	if graph.SetTriangle(graph.Root(), [3]Point{}, Color{}, 0, Shade{}, nil) || graph.SetRectangle(graph.Root(), Rect{}, Color{}) {
		t.Fatal("a group became a shape")
	}
}

func TestTriangleShadeIsPublishedChangeCheckedAndClearedByARectangle(t *testing.T) {
	graph := New()
	points := [3]Point{{X: 1}, {X: 2}, {X: 3}}
	shade := Shade{On: true, Colors: [2]Color{{Red: 1, Alpha: 1}, {Blue: 1, Alpha: 1}}}
	id, _ := graph.AddTriangle(graph.Root(), points, Color{Green: 1, Alpha: 1}, 0, shade, nil)
	changes := graph.DrainChanges(nil)
	if changes[len(changes)-1].Shape == nil || changes[len(changes)-1].Shape.Shade != shade {
		t.Fatalf("published shape = %+v", changes[len(changes)-1].Shape)
	}
	if graph.SetTriangle(id, points, Color{Green: 1, Alpha: 1}, 0, shade, nil) {
		t.Fatal("setting the same shaded triangle reported a change")
	}
	if !graph.SetTriangle(id, points, Color{Green: 1, Alpha: 1}, 0, Shade{}, nil) {
		t.Fatal("dropping the shade reported no change")
	}
	graph.SetTriangle(id, points, Color{Green: 1, Alpha: 1}, 0, shade, nil)
	graph.SetRectangle(id, Rect{Width: 4}, Color{Alpha: 1})
	changes = graph.DrainChanges(changes[:0])
	if len(changes) != 1 || changes[0].Shape != nil {
		t.Fatalf("a rectangle kept the triangle's shade: %+v", changes)
	}
}
