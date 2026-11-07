package web

import (
	"testing"

	"github.com/vivaui/viva/ui"
)

func TestDescribeAccessibilityMapsEveryRole(t *testing.T) {
	tests := []struct {
		role     ui.Role
		want     string
		element  string
		hasRange bool
	}{
		{ui.RoleButton, "button", "div", false},
		{ui.RoleLink, "link", "div", false},
		{ui.RoleCheckbox, "checkbox", "div", false},
		{ui.RoleRadio, "radio", "div", false},
		{ui.RoleTab, "tab", "div", false},
		{ui.RoleMenuItem, "menuitem", "div", false},
		{ui.RoleCombobox, "combobox", "input", false},
		{ui.RoleSwitch, "switch", "div", false},
		{ui.RoleTextField, "textbox", "input", false},
		{ui.RoleTerminal, "textbox", "textarea", false},
		{ui.RoleSlider, "slider", "div", false},
		{ui.RoleSlider, "slider", "input", true},
		{0, "button", "div", false},
	}

	hits := make([]ui.Hit, len(tests))
	for index, test := range tests {
		hits[index] = ui.Hit{ID: ui.Item(int(uint32(index + 1))), Role: test.role}
		if test.hasRange {
			hits[index].Range = ui.ValueRange{Now: 1, Min: 0, Max: 10, Step: 1, Set: true}
		}
	}
	descriptions := describeAccessibility(nil, hits, ui.ID{})
	for index, test := range tests {
		if descriptions[index].Role != test.want || descriptions[index].Element != test.element {
			t.Fatalf("role %d = %q/%q, want %q/%q", test.role, descriptions[index].Element, descriptions[index].Role, test.element, test.want)
		}
	}
	if got := accessibilityElement(ui.RoleTextField, true, false); got != "textarea" {
		t.Fatalf("multiline textbox element = %q", got)
	}
	if got := accessibilityElement(ui.RoleSlider, false, true); got != "input" {
		t.Fatalf("range slider element = %q", got)
	}
}

func TestDescribeAccessibilityPreservesStateAndScope(t *testing.T) {
	hits := []ui.Hit{
		{
			ID: ui.Item(17), Scope: ui.Item(4), Role: ui.RoleTextField,
			Label: "Message", Value: "Hello", Hint: "Enter a reply",
			Disabled: true, Checked: true, Selected: true, Expanded: true,
			Multiline: true, ReadOnly: true,
		},
		{ID: ui.Item(18), Scope: ui.Item(9), Role: ui.RoleButton, Label: "Outside"},
	}

	descriptions := describeAccessibility(make([]accessibilityDescription, 0, 2), hits, ui.Item(4))
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
		{ID: ui.Item(1), DOMID: accessibilityDOMID(ui.Item(1))},
		{ID: ui.Item(2), DOMID: accessibilityDOMID(ui.Item(2)), Hidden: true},
		{ID: ui.Item(3), DOMID: accessibilityDOMID(ui.Item(3)), Disabled: true},
	}
	if got := accessibilityActiveDescendant(descriptions, ui.Item(1)); got != "viva-canvas-hit-1-1" {
		t.Fatalf("active descendant = %q", got)
	}
	for _, focused := range []ui.ID{ui.ID{}, ui.Item(2), ui.Item(3), ui.Item(99)} {
		if got := accessibilityActiveDescendant(descriptions, focused); got != "" {
			t.Fatalf("focused %v active descendant = %q", focused, got)
		}
	}
}

func TestSameAccessibilityOrder(t *testing.T) {
	descriptions := []accessibilityDescription{{ID: ui.Item(7)}, {ID: ui.Item(11)}}
	if !sameAccessibilityOrder([]ui.ID{ui.Item(7), ui.Item(11)}, descriptions) {
		t.Fatal("equal order was reported changed")
	}
	if sameAccessibilityOrder([]ui.ID{ui.Item(11), ui.Item(7)}, descriptions) || sameAccessibilityOrder([]ui.ID{ui.Item(7)}, descriptions) {
		t.Fatal("changed order was reported equal")
	}
}

func TestDescribeAccessibilityCarriesStructureAndRange(t *testing.T) {
	hits := []ui.Hit{{
		ID: ui.Item(1), Role: ui.RoleSlider, Label: "Volume", Hint: "Drag", Description: "Output level",
		Level: 2, PosInSet: 1, SetSize: 3, Live: ui.LivePolite, Expandable: true,
		Range: ui.ValueRange{Now: 4, Min: 0, Max: 10, Step: 1, Set: true},
	}}
	got := describeAccessibility(nil, hits, ui.ID{})
	if len(got) != 1 {
		t.Fatalf("descriptions = %d", len(got))
	}
	description := got[0]
	if description.Description != "Drag. Output level" || description.Level != 2 || description.PosInSet != 1 ||
		description.SetSize != 3 || description.Live != "polite" || !description.Expandable || description.Range != hits[0].Range {
		t.Fatalf("description = %+v", description)
	}
}
