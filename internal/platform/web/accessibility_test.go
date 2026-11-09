package web

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

func TestDescribeAccessibilityMapsEveryRole(t *testing.T) {
	tests := []struct {
		role     gfx.Role
		want     string
		element  string
		hasRange bool
	}{
		{gfx.RoleButton, "button", "div", false},
		{gfx.RoleLink, "link", "div", false},
		{gfx.RoleCheckbox, "checkbox", "div", false},
		{gfx.RoleRadio, "radio", "div", false},
		{gfx.RoleTab, "tab", "div", false},
		{gfx.RoleMenuItem, "menuitem", "div", false},
		{gfx.RoleCombobox, "combobox", "input", false},
		{gfx.RoleSwitch, "switch", "div", false},
		{gfx.RoleTextField, "textbox", "input", false},
		{gfx.RoleTerminal, "textbox", "textarea", false},
		{gfx.RoleSlider, "slider", "div", false},
		{gfx.RoleSlider, "slider", "input", true},
		{0, "button", "div", false},
	}

	hits := make([]gfx.Hit, len(tests))
	for index, test := range tests {
		hits[index] = gfx.Hit{ID: gfx.Item(int(uint32(index + 1))), Role: test.role}
		if test.hasRange {
			hits[index].Range = gfx.ValueRange{Now: 1, Min: 0, Max: 10, Step: 1, Set: true}
		}
	}
	descriptions := describeAccessibility(nil, hits, gfx.ID{})
	for index, test := range tests {
		if descriptions[index].Role != test.want || descriptions[index].Element != test.element {
			t.Fatalf("role %d = %q/%q, want %q/%q", test.role, descriptions[index].Element, descriptions[index].Role, test.element, test.want)
		}
	}
	if got := accessibilityElement(gfx.RoleTextField, true, false); got != "textarea" {
		t.Fatalf("multiline textbox element = %q", got)
	}
	if got := accessibilityElement(gfx.RoleSlider, false, true); got != "input" {
		t.Fatalf("range slider element = %q", got)
	}
}

func TestDescribeAccessibilityPreservesStateAndScope(t *testing.T) {
	hits := []gfx.Hit{
		{
			ID: gfx.Item(17), Scope: gfx.Item(4), Role: gfx.RoleTextField,
			Label: "Message", Value: "Hello", Hint: "Enter a reply",
			Disabled: true, Checked: true, Selected: true, Expanded: true,
			Multiline: true, ReadOnly: true,
		},
		{ID: gfx.Item(18), Scope: gfx.Item(9), Role: gfx.RoleButton, Label: "Outside"},
	}

	descriptions := describeAccessibility(make([]accessibilityDescription, 0, 2), hits, gfx.Item(4))
	got := descriptions[0]
	if got.DOMID != "viva-canvas-hit-1-17" || got.Element != "textarea" || got.Role != "textbox" || got.Label != "Message" || got.Value != "Hello" || got.Hint != "Enter a reply" {
		t.Fatalf("identity = %+v", got)
	}
	if got.Hidden || !got.Disabled || !got.Checked || !got.Selected || !got.Expanded || !got.Multiline || !got.ReadOnly {
		t.Fatalf("state = %+v", got)
	}
	if !descriptions[1].Hidden {
		t.Fatal("hit outside the trapped focus scope must be hidden from accessibility")
	}
}

func TestAccessibilityActiveDescendantRequiresAvailableNode(t *testing.T) {
	descriptions := []accessibilityDescription{
		{ID: gfx.Item(1), DOMID: accessibilityDOMID(gfx.Item(1))},
		{ID: gfx.Item(2), DOMID: accessibilityDOMID(gfx.Item(2)), Hidden: true},
		{ID: gfx.Item(3), DOMID: accessibilityDOMID(gfx.Item(3)), Disabled: true},
	}
	if got := accessibilityActiveDescendant(descriptions, gfx.Item(1)); got != "viva-canvas-hit-1-1" {
		t.Fatalf("active descendant = %q", got)
	}
	for _, focused := range []gfx.ID{gfx.ID{}, gfx.Item(2), gfx.Item(3), gfx.Item(99)} {
		if got := accessibilityActiveDescendant(descriptions, focused); got != "" {
			t.Fatalf("focused %v active descendant = %q", focused, got)
		}
	}
}

func TestSameAccessibilityOrder(t *testing.T) {
	descriptions := []accessibilityDescription{{ID: gfx.Item(7)}, {ID: gfx.Item(11)}}
	if !sameAccessibilityOrder([]gfx.ID{gfx.Item(7), gfx.Item(11)}, descriptions) {
		t.Fatal("equal order was reported changed")
	}
	if sameAccessibilityOrder([]gfx.ID{gfx.Item(11), gfx.Item(7)}, descriptions) || sameAccessibilityOrder([]gfx.ID{gfx.Item(7)}, descriptions) {
		t.Fatal("changed order was reported equal")
	}
}

func TestDescribeAccessibilityCarriesStructureAndRange(t *testing.T) {
	hits := []gfx.Hit{{
		ID: gfx.Item(1), Role: gfx.RoleSlider, Label: "Volume", Hint: "Drag", Description: "Output level",
		Level: 2, PosInSet: 1, SetSize: 3, Live: gfx.LivePolite, Expandable: true,
		Range: gfx.ValueRange{Now: 4, Min: 0, Max: 10, Step: 1, Set: true},
	}}
	got := describeAccessibility(nil, hits, gfx.ID{})
	if len(got) != 1 {
		t.Fatalf("descriptions = %d", len(got))
	}
	description := got[0]
	if description.Description != "Drag. Output level" || description.Level != 2 || description.PosInSet != 1 ||
		description.SetSize != 3 || description.Live != "polite" || !description.Expandable || description.Range != hits[0].Range {
		t.Fatalf("description = %+v", description)
	}
}

// A label is not a node of its own: it names the control with its identity, so
// the mirror must describe one stable node for both, not replace the control
// with a label node and back on every frame (which would tear down any editor
// attached to it).
func TestDescribeAccessibilityNamesTheControlWithItsLabel(t *testing.T) {
	label := gfx.Hit{ID: gfx.Item(7), Label: "Username", IsLabel: true}
	control := gfx.Hit{ID: gfx.Item(7), Role: gfx.RoleTextField, Value: "gopher"}
	got := describeAccessibility(nil, []gfx.Hit{label, control}, gfx.ID{})
	if len(got) != 1 {
		t.Fatalf("descriptions = %d, want only the control: %+v", len(got), got)
	}
	if got[0].Role != "textbox" || got[0].Label != "Username" || got[0].Value != "gopher" {
		t.Fatalf("control description = %+v", got[0])
	}
}
