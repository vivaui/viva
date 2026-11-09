package platform

import "github.com/vivaui/viva/gfx"

// Accessibility node roles, in the numbering of internal/semantic.Role, which
// is what AccessibilityNode.Role carries.
const (
	A11yRoleApplication uint8 = iota + 1
	A11yRoleGroup
	A11yRoleButton
	A11yRoleText
	A11yRoleTextField
	A11yRoleCheckbox
	A11yRoleSlider
	A11yRoleImage
	A11yRoleLink
	A11yRoleRadio
	A11yRoleTab
	A11yRoleMenuItem
	A11yRoleCombobox
	A11yRoleSwitch
	A11yRoleMenu
	A11yRoleMenubar
	A11yRoleListBox
	A11yRoleOption
	A11yRoleGrid
	A11yRoleGridCell
	A11yRoleHeading
	A11yRoleStatus
	A11yRoleSeparator
)

// AccessibilityAction is something assistive technology asks a node to do.
type AccessibilityAction uint8

const (
	// AccessibilityPress is the node's default action: click a button or a
	// link, toggle a checkbox or a switch, choose a radio, a tab, an option or
	// a menu item, open or close a disclosure.
	AccessibilityPress AccessibilityAction = iota + 1
	// AccessibilityFocus gives the node the keyboard.
	AccessibilityFocus
	// AccessibilityIncrement and AccessibilityDecrement move a slider by one
	// step.
	AccessibilityIncrement
	AccessibilityDecrement
	// AccessibilitySetNumber moves a slider to Number.
	AccessibilitySetNumber
	// AccessibilitySetText replaces the text of a text field with Text.
	AccessibilitySetText
	// AccessibilitySetSelection selects the span Start..End (code points) of a
	// text field's text, a caret when they are equal.
	AccessibilitySetSelection
)

// AccessibilityRequest is one request from assistive technology.
type AccessibilityRequest struct {
	Node   gfx.ID
	Action AccessibilityAction
	// Number is the value of AccessibilitySetNumber.
	Number float64
	// Text is the value of AccessibilitySetText.
	Text string
	// Start and End are the span of AccessibilitySetSelection.
	Start, End int32
}

// AccessibilityHandler carries out one request. Each repaint it causes is
// handed to present, in order, so a request that takes several steps (a slider
// moved by many keys) shows each one; the adapter presents a frame as it does
// any other. The bool is false when the node is gone or cannot do what was
// asked.
type AccessibilityHandler func(request AccessibilityRequest, present func(Frame) error) (bool, error)

// AccessibilityPatterns are the kinds of behavior a node offers to assistive
// technology, named for the UI Automation control patterns they map to and
// used by every adapter to decide what a node can do.
type AccessibilityPatterns uint16

const (
	// PatternInvoke is a single action: a button, a link, a menu item.
	PatternInvoke AccessibilityPatterns = 1 << iota
	// PatternToggle is a two or three state control: a checkbox, a switch.
	PatternToggle
	// PatternValue is a text value that can be read and written.
	PatternValue
	// PatternRangeValue is a number between bounds: a slider.
	PatternRangeValue
	// PatternSelectionItem is an item that is chosen among siblings: a tab,
	// a radio, an option.
	PatternSelectionItem
	// PatternSelection is the container of such items: a list.
	PatternSelection
	// PatternExpandCollapse is a control that opens and closes.
	PatternExpandCollapse
	// PatternText is a body of text with a caret and a selection.
	PatternText
)

// Has reports whether every pattern in want is offered.
func (patterns AccessibilityPatterns) Has(want AccessibilityPatterns) bool {
	return patterns&want == want
}

// Patterns reports what the node can do. The answer follows from the role and
// the state the node already carries, so every platform offers the same set.
func (node AccessibilityNode) Patterns() AccessibilityPatterns {
	var patterns AccessibilityPatterns
	switch node.Role {
	case A11yRoleButton, A11yRoleLink, A11yRoleMenuItem:
		patterns |= PatternInvoke
	case A11yRoleCheckbox, A11yRoleSwitch:
		patterns |= PatternToggle
	case A11yRoleRadio, A11yRoleTab, A11yRoleOption:
		patterns |= PatternSelectionItem
	case A11yRoleGridCell:
		patterns |= PatternInvoke | PatternSelectionItem
	case A11yRoleCombobox:
		patterns |= PatternExpandCollapse | PatternValue
	case A11yRoleTextField:
		patterns |= PatternValue | PatternText
	case A11yRoleSlider:
		if node.Range.Set {
			patterns |= PatternRangeValue
		}
		patterns |= PatternValue
	case A11yRoleListBox:
		patterns |= PatternSelection
	}
	if node.Expandable {
		patterns |= PatternExpandCollapse
	}
	return patterns
}

// Operable reports whether a request to act on the node can succeed: it has an
// ID to be found by and is not disabled.
func (node AccessibilityNode) Operable() bool {
	return !node.ID.IsZero() && !node.Disabled
}

// IsChosen reports whether a selection item is the one chosen. A radio keeps
// its state in Checked, a tab or an option in Selected.
func (node AccessibilityNode) IsChosen() bool {
	return node.Selected || (node.Role == A11yRoleRadio && node.Checked)
}

// AccessibilityContainer returns the index of the node that holds nodes[index]:
// the nearest earlier list, grid, menu or menu bar whose bounds contain the
// center of the node. The tree is flat in paint order, which lays a container
// before what it holds. It returns -1 for a node at the top level.
func AccessibilityContainer(nodes []AccessibilityNode, index int) int {
	if index < 0 || index >= len(nodes) {
		return -1
	}
	node := nodes[index]
	centerX, centerY := node.X+node.Width/2, node.Y+node.Height/2
	for candidate := index - 1; candidate >= 0; candidate-- {
		switch nodes[candidate].Role {
		case A11yRoleListBox, A11yRoleGrid, A11yRoleMenu, A11yRoleMenubar:
		default:
			continue
		}
		box := nodes[candidate]
		if centerX >= box.X && centerX < box.X+box.Width && centerY >= box.Y && centerY < box.Y+box.Height {
			return candidate
		}
	}
	return -1
}

// AccessibilityParents fills destination with the container index of every
// node (see AccessibilityContainer), -1 for the top level.
func AccessibilityParents(destination []int32, nodes []AccessibilityNode) []int32 {
	destination = destination[:0]
	for index := range nodes {
		destination = append(destination, int32(AccessibilityContainer(nodes, index)))
	}
	return destination
}

// AccessibilityIndexOf returns the position of the node with the given ID, or
// -1. A zero ID never matches.
func AccessibilityIndexOf(nodes []AccessibilityNode, id gfx.ID) int {
	if id.IsZero() {
		return -1
	}
	for index := range nodes {
		if nodes[index].ID == id {
			return index
		}
	}
	return -1
}

// AccessibilityChange is what differs between two versions of a node, as the
// events an adapter should raise for it.
type AccessibilityChange uint32

const (
	ChangedName AccessibilityChange = 1 << iota
	ChangedRole
	ChangedFocus
	ChangedEnabled
	ChangedValue
	ChangedHint
	ChangedDescription
	ChangedReadOnly
	ChangedPassword
	ChangedExpand
	ChangedSelected
	ChangedToggle
	ChangedRange
	ChangedLevel
	ChangedPosition
	ChangedLive
	ChangedSelection
	ChangedBounds
)

// DiffAccessibility returns the properties that differ between old and next.
func DiffAccessibility(old, next AccessibilityNode) AccessibilityChange {
	var changes AccessibilityChange
	set := func(differs bool, change AccessibilityChange) {
		if differs {
			changes |= change
		}
	}
	set(old.Label != next.Label, ChangedName)
	set(old.Role != next.Role, ChangedRole)
	set(old.Focused != next.Focused, ChangedFocus)
	set(old.Disabled != next.Disabled, ChangedEnabled)
	set(old.Value != next.Value, ChangedValue)
	set(old.Hint != next.Hint, ChangedHint)
	set(old.Description != next.Description, ChangedDescription)
	set(old.ReadOnly != next.ReadOnly, ChangedReadOnly)
	set(old.Secure != next.Secure, ChangedPassword)
	set(old.Expanded != next.Expanded, ChangedExpand)
	set(old.Selected != next.Selected, ChangedSelected)
	set(old.Checked != next.Checked || old.Mixed != next.Mixed, ChangedToggle)
	set(old.Range != next.Range, ChangedRange)
	set(old.Level != next.Level, ChangedLevel)
	set(old.PosInSet != next.PosInSet || old.SetSize != next.SetSize, ChangedPosition)
	set(old.Live != next.Live, ChangedLive)
	set(old.Selection != next.Selection, ChangedSelection)
	set(old.X != next.X || old.Y != next.Y || old.Width != next.Width || old.Height != next.Height, ChangedBounds)
	return changes
}
