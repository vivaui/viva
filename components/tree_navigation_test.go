package components

import (
	"testing"

	"github.com/vivaui/viva/ui"
)

func TestNavigateTreeMovesAmongVisibleRowsAndBranches(t *testing.T) {
	items := []TreeNavigationItem{
		{Depth: 0, Branch: true, Expanded: true},
		{Depth: 1, Branch: true, Expanded: true},
		{Depth: 2},
		{Depth: 1, Disabled: true},
		{Depth: 1, Branch: true},
		{Depth: 0},
	}
	cases := []struct {
		name   string
		at     int
		key    ui.Key
		page   int
		want   int
		action TreeNavigationAction
	}{
		{"down skips disabled", 2, ui.KeyDown, 3, 4, TreeNavigationMove},
		{"up skips disabled", 4, ui.KeyUp, 3, 2, TreeNavigationMove},
		{"home", 4, ui.KeyHome, 3, 0, TreeNavigationMove},
		{"end", 0, ui.KeyEnd, 3, 5, TreeNavigationMove},
		{"page down", 0, ui.KeyPageDown, 3, 4, TreeNavigationMove},
		{"page up", 5, ui.KeyPageUp, 3, 2, TreeNavigationMove},
		{"right child", 0, ui.KeyRight, 3, 1, TreeNavigationMove},
		{"right open", 4, ui.KeyRight, 3, 4, TreeNavigationExpand},
		{"left close", 1, ui.KeyLeft, 3, 1, TreeNavigationCollapse},
		{"left parent", 2, ui.KeyLeft, 3, 1, TreeNavigationMove},
		{"left finds parent across siblings", 4, ui.KeyLeft, 3, 0, TreeNavigationMove},
		{"right leaf", 2, ui.KeyRight, 3, 2, TreeNavigationNone},
		{"end edge", 5, ui.KeyEnd, 3, 5, TreeNavigationNone},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			index, action, handled := NavigateTree(items, test.at, test.key, test.page)
			if !handled || index != test.want || action != test.action {
				t.Fatalf("NavigateTree = (%d, %v, %t), want (%d, %v, true)", index, action, handled, test.want, test.action)
			}
		})
	}
	if _, _, handled := NavigateTree(items, 0, ui.KeyEnter, 3); handled {
		t.Fatal("activation belongs to the application")
	}
	if _, _, handled := NavigateTree(items, 3, ui.KeyDown, 3); handled {
		t.Fatal("disabled rows cannot navigate")
	}
}

func TestNavigateTreeDoesNotTreatNonchildAsChild(t *testing.T) {
	items := []TreeNavigationItem{{Depth: 255, Branch: true, Expanded: true}, {Depth: 0}}
	if index, action, handled := NavigateTree(items, 0, ui.KeyRight, 0); !handled || index != 0 || action != TreeNavigationNone {
		t.Fatalf("Right at depth limit = (%d, %v, %t)", index, action, handled)
	}
}

func TestNavigateTreeSkipsDisabledFirstChild(t *testing.T) {
	items := []TreeNavigationItem{
		{Branch: true, Expanded: true},
		{Depth: 1, Disabled: true},
		{Depth: 1},
	}
	if index, action, handled := NavigateTree(items, 0, ui.KeyRight, 2); !handled || index != 2 || action != TreeNavigationMove {
		t.Fatalf("Right past disabled child = (%d, %v, %t)", index, action, handled)
	}
}

var benchmarkTreeNavigationIndex int

func BenchmarkNavigateTree(b *testing.B) {
	items := make([]TreeNavigationItem, 1000)
	for index := range items {
		items[index] = TreeNavigationItem{Depth: 1}
	}
	items[0] = TreeNavigationItem{Branch: true, Expanded: true}
	b.ReportAllocs()
	for b.Loop() {
		benchmarkTreeNavigationIndex, _, _ = NavigateTree(items, 500, ui.KeyLeft, 30)
	}
}
