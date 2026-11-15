package app

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/semantic"
)

// AccessNode is what assistive technology sees of one control in a frame: the
// same information the platform adapters send to the system, without the
// platform. Driver.Accessibility returns the last frame's tree.
type AccessNode struct {
	// ID names the control, as the hit it was made from does.
	ID gfx.ID
	// Role is the control's role; zero when it is a text without one.
	Role gfx.Role
	// Bounds is where the control was drawn, in logical pixels.
	Bounds gfx.Rect

	// Label is the accessible name, Value the control's value and Hint the
	// short instruction usually read after the name.
	Label string
	Value string
	Hint  string
	// Description is a longer explanation than Hint.
	Description string

	Focused   bool
	Disabled  bool
	Checked   bool
	Mixed     bool
	Selected  bool
	Expanded  bool
	Multiline bool
	ReadOnly  bool
	Secure    bool
	Focusable bool
	// Expandable marks a control that opens and closes something, whatever its
	// state now.
	Expandable bool

	// Level is a heading's level or a tree item's depth; zero when not stated.
	Level uint8
	// PosInSet and SetSize place the node among its siblings, counting from 1.
	PosInSet, SetSize int32
	// Live marks a region that announces changes to its content.
	Live gfx.LiveRegion
	// Range is a slider's or progress bar's number.
	Range gfx.ValueRange
	// Selection is the selected span of a focused text field's Value.
	Selection AccessSelection
}

// AccessSelection is a span of a text node's Value in Unicode code points:
// Start and End are the anchor and the caret, so End < Start for a selection
// made backwards. Valid is false when the node has no caret.
type AccessSelection struct {
	Start, End int32
	Valid      bool
}

// Accessibility returns the accessibility tree of the last painted frame, in
// reading order. It is safe to call from the goroutine that drives the Driver
// (the main thread in a test), as Snapshot is.
func (driver *Driver) Accessibility() []AccessNode {
	if driver == nil || driver.painter == nil {
		return nil
	}
	nodes := driver.painter.accessibility
	if len(nodes) == 0 {
		return nil
	}
	out := make([]AccessNode, len(nodes))
	for index, node := range nodes {
		out[index] = AccessNode{
			ID: node.ID, Role: accessRole(semantic.Role(node.Role)),
			Bounds: gfx.Rect{X: node.X, Y: node.Y, Width: node.Width, Height: node.Height},
			Label:  node.Label, Value: node.Value, Hint: node.Hint, Description: node.Description,
			Focused: node.Focused, Disabled: node.Disabled, Checked: node.Checked, Mixed: node.Mixed,
			Selected: node.Selected, Expanded: node.Expanded, Multiline: node.Multiline,
			ReadOnly: node.ReadOnly, Secure: node.Secure, Focusable: node.Focusable,
			Expandable: node.Expandable, Level: node.Level, PosInSet: node.PosInSet, SetSize: node.SetSize,
			Live: node.Live, Range: node.Range,
			Selection: AccessSelection{Start: node.Selection.Start, End: node.Selection.End, Valid: node.Selection.Valid},
		}
	}
	return out
}

// accessRole maps the semantic role the platform adapters receive back to the
// gfx.Role the application declared. Roles that have no gfx counterpart (a
// text, an image, the application itself) are zero.
func accessRole(role semantic.Role) gfx.Role {
	switch role {
	case semantic.RoleButton:
		return gfx.RoleButton
	case semantic.RoleLink:
		return gfx.RoleLink
	case semantic.RoleCheckbox:
		return gfx.RoleCheckbox
	case semantic.RoleRadio:
		return gfx.RoleRadio
	case semantic.RoleTab:
		return gfx.RoleTab
	case semantic.RoleMenuItem:
		return gfx.RoleMenuItem
	case semantic.RoleCombobox:
		return gfx.RoleCombobox
	case semantic.RoleSwitch:
		return gfx.RoleSwitch
	case semantic.RoleTextField:
		return gfx.RoleTextField
	case semantic.RoleSlider:
		return gfx.RoleSlider
	case semantic.RoleGroup:
		return gfx.RoleGroup
	case semantic.RoleMenu:
		return gfx.RoleMenu
	case semantic.RoleMenubar:
		return gfx.RoleMenubar
	case semantic.RoleListBox:
		return gfx.RoleListBox
	case semantic.RoleOption:
		return gfx.RoleOption
	case semantic.RoleGrid:
		return gfx.RoleGrid
	case semantic.RoleGridCell:
		return gfx.RoleGridCell
	case semantic.RoleHeading:
		return gfx.RoleHeading
	case semantic.RoleStatus:
		return gfx.RoleStatus
	case semantic.RoleSeparator:
		return gfx.RoleSeparator
	}
	return 0
}
