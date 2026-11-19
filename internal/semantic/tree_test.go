package semantic

import "testing"

func TestTreeCoalescesLocalizedUpdates(t *testing.T) {
	tree := New("Viva")
	button, ok := tree.Add(tree.Root(), Node{Role: RoleButton, Label: "Save", Focusable: true})
	if !ok {
		t.Fatal("failed to add button")
	}
	tree.DrainChanges(nil)

	for index := 0; index < 100; index++ {
		tree.Set(button, Node{Role: RoleButton, Label: "Save", Value: string(rune('A' + index%26)), Focusable: true})
	}
	if tree.PendingChanges() != 1 {
		t.Fatalf("pending changes = %d, want 1", tree.PendingChanges())
	}
	changes := tree.DrainChanges(nil)
	if len(changes) != 1 || changes[0].ID != button || changes[0].Operation != OperationUpsert {
		t.Fatalf("changes = %+v", changes)
	}
	if changes := tree.DrainChanges(changes[:0]); len(changes) != 0 {
		t.Fatalf("unchanged drain produced %d changes", len(changes))
	}
}

func TestNewWithCapacityPublishesApplicationRoot(t *testing.T) {
	tree := NewWithCapacity("Viva", 10_000)
	changes := tree.DrainChanges(nil)
	if len(changes) != 1 || changes[0].ID != tree.Root() || changes[0].Node.Role != RoleApplication {
		t.Fatalf("initial changes = %+v", changes)
	}
}

func TestTreeDeletesChildrenBeforeParentAndRejectsStaleID(t *testing.T) {
	tree := New("Viva")
	group, _ := tree.Add(tree.Root(), Node{Role: RoleGroup})
	child, _ := tree.Add(group, Node{Role: RoleText, Label: "Child"})
	tree.DrainChanges(nil)

	if !tree.Remove(group) {
		t.Fatal("failed to remove group")
	}
	changes := tree.DrainChanges(nil)
	if len(changes) != 2 || changes[0].ID != child || changes[1].ID != group {
		t.Fatalf("delete order = %+v", changes)
	}
	reused, _ := tree.Add(tree.Root(), Node{Role: RoleButton})
	if reused == group || tree.Set(group, Node{Role: RoleGroup}) {
		t.Fatal("stale semantic ID remained valid")
	}
}

func TestTreeDropsUnpublishedCreateDelete(t *testing.T) {
	tree := New("Viva")
	tree.DrainChanges(nil)
	node, _ := tree.Add(tree.Root(), Node{Role: RoleText})
	tree.Remove(node)
	if changes := tree.DrainChanges(nil); len(changes) != 0 {
		t.Fatalf("unpublished node produced changes: %+v", changes)
	}
}

func TestTreeEmitsInvalidChange(t *testing.T) {
	tree := New("Viva")
	field, ok := tree.Add(tree.Root(), Node{Role: RoleTextField, Label: "Email", Focusable: true})
	if !ok {
		t.Fatal("failed to add field")
	}
	tree.DrainChanges(nil)

	tree.Set(field, Node{Role: RoleTextField, Label: "Email", Focusable: true, Invalid: true})
	changes := tree.DrainChanges(nil)
	if len(changes) != 1 || changes[0].ID != field || !changes[0].Node.Invalid {
		t.Fatalf("invalid change = %+v", changes)
	}

	tree.Set(field, Node{Role: RoleTextField, Label: "Email", Focusable: true})
	changes = tree.DrainChanges(changes[:0])
	if len(changes) != 1 || changes[0].Node.Invalid {
		t.Fatalf("valid change = %+v", changes)
	}
}

func BenchmarkSemanticNoChanges(b *testing.B) {
	tree, output, _ := benchmarkTree(b, 10_000)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		output = tree.DrainChanges(output[:0])
	}
}

func BenchmarkSemanticLocalizedUpdate(b *testing.B) {
	tree, output, target := benchmarkTree(b, 10_000)
	b.ReportAllocs()
	b.ResetTimer()
	for index := range b.N {
		data := Node{Role: RoleButton, Label: "Button", Value: "0", Focusable: true}
		if index&1 == 0 {
			data.Value = "1"
		}
		tree.Set(target, data)
		output = tree.DrainChanges(output[:0])
		if len(output) != 1 {
			b.Fatalf("change count = %d", len(output))
		}
	}
}

func benchmarkTree(b *testing.B, count int) (*Tree, []Change, ID) {
	b.Helper()
	tree := NewWithCapacity("Viva", count+1)
	var target ID
	for index := 0; index < count; index++ {
		id, _ := tree.Add(tree.Root(), Node{Role: RoleButton, Label: "Button", Value: "0", Focusable: true})
		if index == count/2 {
			target = id
		}
	}
	output := make([]Change, 0, count+1)
	output = tree.DrainChanges(output)
	return tree, output[:0], target
}
