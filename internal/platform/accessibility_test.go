package platform

import (
	"testing"

	"github.com/vivaui/viva/internal/semantic"
)

func TestAccessibilityRolesMatchSemanticTree(t *testing.T) {
	for _, pair := range []struct {
		platform uint8
		semantic semantic.Role
	}{
		{A11yRoleApplication, semantic.RoleApplication}, {A11yRoleGroup, semantic.RoleGroup},
		{A11yRoleButton, semantic.RoleButton}, {A11yRoleText, semantic.RoleText},
		{A11yRoleTextField, semantic.RoleTextField}, {A11yRoleCheckbox, semantic.RoleCheckbox},
		{A11yRoleSlider, semantic.RoleSlider}, {A11yRoleImage, semantic.RoleImage},
		{A11yRoleLink, semantic.RoleLink}, {A11yRoleRadio, semantic.RoleRadio},
		{A11yRoleTab, semantic.RoleTab}, {A11yRoleMenuItem, semantic.RoleMenuItem},
		{A11yRoleCombobox, semantic.RoleCombobox}, {A11yRoleSwitch, semantic.RoleSwitch},
		{A11yRoleMenu, semantic.RoleMenu}, {A11yRoleMenubar, semantic.RoleMenubar},
		{A11yRoleListBox, semantic.RoleListBox}, {A11yRoleOption, semantic.RoleOption},
		{A11yRoleGrid, semantic.RoleGrid}, {A11yRoleGridCell, semantic.RoleGridCell},
		{A11yRoleHeading, semantic.RoleHeading}, {A11yRoleStatus, semantic.RoleStatus},
		{A11yRoleSeparator, semantic.RoleSeparator}, {A11yRoleProgress, semantic.RoleProgress},
	} {
		if pair.platform != uint8(pair.semantic) {
			t.Errorf("platform role %d is semantic role %d", pair.platform, pair.semantic)
		}
	}
}

func TestAccessibilityPatternsFollowRoleAndState(t *testing.T) {
	for _, test := range []struct {
		name string
		node AccessibilityNode
		want AccessibilityPatterns
	}{
		{"button", AccessibilityNode{Role: A11yRoleButton}, PatternInvoke},
		{"disclosure button", AccessibilityNode{Role: A11yRoleButton, Expandable: true}, PatternInvoke | PatternExpandCollapse},
		{"checkbox", AccessibilityNode{Role: A11yRoleCheckbox}, PatternToggle},
		{"tab", AccessibilityNode{Role: A11yRoleTab}, PatternSelectionItem},
		{"text field", AccessibilityNode{Role: A11yRoleTextField}, PatternValue | PatternText},
		{"slider with range", AccessibilityNode{Role: A11yRoleSlider, Range: rangeOf(1, 0, 10)}, PatternValue | PatternRangeValue},
		{"slider without range", AccessibilityNode{Role: A11yRoleSlider}, PatternValue},
		{"progress with range", AccessibilityNode{Role: A11yRoleProgress, Range: rangeOf(1, 0, 10)}, PatternRangeValue},
		{"progress without range", AccessibilityNode{Role: A11yRoleProgress}, 0},
		{"combobox", AccessibilityNode{Role: A11yRoleCombobox}, PatternValue | PatternExpandCollapse},
		{"list", AccessibilityNode{Role: A11yRoleListBox}, PatternSelection},
		{"heading", AccessibilityNode{Role: A11yRoleHeading}, 0},
	} {
		if got := test.node.Patterns(); got != test.want {
			t.Errorf("%s: patterns %b, want %b", test.name, got, test.want)
		}
	}
}

func TestAccessibilityContainerIsTheNearestEnclosingList(t *testing.T) {
	nodes := []AccessibilityNode{
		{Role: A11yRoleButton, X: 0, Y: 0, Width: 100, Height: 20},
		{Role: A11yRoleListBox, X: 0, Y: 30, Width: 100, Height: 100},
		{Role: A11yRoleOption, X: 0, Y: 30, Width: 100, Height: 20},
		{Role: A11yRoleOption, X: 0, Y: 50, Width: 100, Height: 20},
		{Role: A11yRoleButton, X: 0, Y: 200, Width: 100, Height: 20},
	}
	want := []int{-1, -1, 1, 1, -1}
	for index, parent := range want {
		if got := AccessibilityContainer(nodes, index); got != parent {
			t.Errorf("container of %d = %d, want %d", index, got, parent)
		}
	}
	if parents := AccessibilityParents(nil, nodes); len(parents) != len(nodes) || parents[2] != 1 {
		t.Errorf("parents = %v", parents)
	}
}

func TestDiffAccessibilityNamesWhatChanged(t *testing.T) {
	old := AccessibilityNode{Role: A11yRoleSlider, Value: "1", Range: rangeOf(1, 0, 10)}
	next := old
	next.Value, next.Range.Now, next.Selection = "2", 2, AccessibilitySelection{Start: 1, End: 1, Valid: true}
	got := DiffAccessibility(old, next)
	if want := ChangedValue | ChangedRange | ChangedSelection; got != want {
		t.Fatalf("diff = %b, want %b", got, want)
	}
	if DiffAccessibility(next, next) != 0 {
		t.Fatal("a node differs from itself")
	}
}

func TestAccessibilityIndexOfSkipsTheZeroID(t *testing.T) {
	nodes := []AccessibilityNode{{}, {}}
	if AccessibilityIndexOf(nodes, nodes[0].ID) != -1 {
		t.Fatal("the zero ID matched a node")
	}
}
