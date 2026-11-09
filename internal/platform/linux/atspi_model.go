package linux

import (
	"strconv"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform"
)

// The AT-SPI vocabulary this adapter speaks: the numbers an assistive
// technology reads from org.a11y.atspi.Accessible. They are fixed by AT-SPI
// (atspi-constants.h), so a screen reader built against any version agrees.

// AtspiRole values.
const (
	atspiRoleCheckBox     = 7
	atspiRoleColumnHeader = 10
	atspiRoleComboBox     = 11
	atspiRoleFrame        = 23
	atspiRoleImage        = 27
	atspiRoleLabel        = 29
	atspiRoleList         = 31
	atspiRoleListItem     = 32
	atspiRoleMenu         = 33
	atspiRoleMenuBar      = 34
	atspiRoleMenuItem     = 35
	atspiRolePageTab      = 37
	atspiRolePanel        = 39
	atspiRolePassword     = 40
	atspiRolePopupMenu    = 41
	atspiRoleButton       = 43
	atspiRoleRadioButton  = 44
	atspiRoleSeparator    = 50
	atspiRoleSlider       = 51
	atspiRoleStatusBar    = 54
	atspiRoleTable        = 55
	atspiRoleTableCell    = 56
	atspiRoleTerminal     = 60
	atspiRoleToggleButton = 62
	atspiRoleUnknown      = 67
	atspiRoleApplication  = 75
	atspiRoleEntry        = 79
	atspiRoleHeading      = 83
	atspiRoleLink         = 88
)

// AtspiStateType bits: bit n of the 64-bit set is state n.
const (
	atspiStateActive = iota + 1
	atspiStateArmed
	atspiStateBusy
	atspiStateChecked
	atspiStateCollapsed
	atspiStateDefunct
	atspiStateEditable
	atspiStateEnabled
	atspiStateExpandable
	atspiStateExpanded
	atspiStateFocusable
	atspiStateFocused
	atspiStateHasTooltip
	atspiStateHorizontal
	atspiStateIconified
	atspiStateModal
	atspiStateMultiLine
	atspiStateMultiselectable
	atspiStateOpaque
	atspiStatePressed
	atspiStateResizable
	atspiStateSelectable
	atspiStateSelected
	atspiStateSensitive
	atspiStateShowing
	atspiStateSingleLine
	atspiStateStale
	atspiStateTransient
	atspiStateVertical
	atspiStateVisible
	atspiStateManagesDescendants
	atspiStateIndeterminate
	atspiStateRequired
	atspiStateTruncated
	atspiStateAnimated
	atspiStateInvalidEntry
	atspiStateSupportsAutocompletion
	atspiStateSelectableText
	atspiStateIsDefault
	atspiStateVisited
	atspiStateCheckable
	atspiStateHasPopup
	atspiStateReadOnly
)

// AtspiTextGranularity values.
const (
	atspiGranularityChar = iota
	atspiGranularityWord
	atspiGranularitySentence
	atspiGranularityLine
	atspiGranularityParagraph
)

// Interfaces a node implements, by name.
const (
	atspiIfaceAccessible   = "org.a11y.atspi.Accessible"
	atspiIfaceApplication  = "org.a11y.atspi.Application"
	atspiIfaceComponent    = "org.a11y.atspi.Component"
	atspiIfaceAction       = "org.a11y.atspi.Action"
	atspiIfaceText         = "org.a11y.atspi.Text"
	atspiIfaceEditableText = "org.a11y.atspi.EditableText"
	atspiIfaceValue        = "org.a11y.atspi.Value"
)

// atspiRole is the AT-SPI role of a node. A password field is its own role, a
// switch is a toggle button (the role older screen readers know), and a node
// the tree has no word for is a panel.
func atspiRole(node platform.AccessibilityNode) uint32 {
	switch node.Role {
	case platform.A11yRoleApplication:
		return atspiRoleApplication
	case platform.A11yRoleButton:
		if node.Checked || node.Mixed {
			return atspiRoleToggleButton
		}
		return atspiRoleButton
	case platform.A11yRoleText:
		return atspiRoleLabel
	case platform.A11yRoleTextField:
		if node.Secure {
			return atspiRolePassword
		}
		return atspiRoleEntry
	case platform.A11yRoleCheckbox:
		return atspiRoleCheckBox
	case platform.A11yRoleSwitch:
		return atspiRoleToggleButton
	case platform.A11yRoleSlider:
		return atspiRoleSlider
	case platform.A11yRoleImage:
		return atspiRoleImage
	case platform.A11yRoleLink:
		return atspiRoleLink
	case platform.A11yRoleRadio:
		return atspiRoleRadioButton
	case platform.A11yRoleTab:
		return atspiRolePageTab
	case platform.A11yRoleMenuItem:
		return atspiRoleMenuItem
	case platform.A11yRoleCombobox:
		return atspiRoleComboBox
	case platform.A11yRoleMenu:
		return atspiRolePopupMenu
	case platform.A11yRoleMenubar:
		return atspiRoleMenuBar
	case platform.A11yRoleListBox:
		return atspiRoleList
	case platform.A11yRoleOption:
		return atspiRoleListItem
	case platform.A11yRoleGrid:
		return atspiRoleTable
	case platform.A11yRoleGridCell:
		return atspiRoleTableCell
	case platform.A11yRoleHeading:
		return atspiRoleHeading
	case platform.A11yRoleStatus:
		return atspiRoleStatusBar
	case platform.A11yRoleSeparator:
		return atspiRoleSeparator
	}
	return atspiRolePanel
}

// atspiRoleName is the untranslated name of a role, what GetRoleName answers.
func atspiRoleName(role uint32) string {
	switch role {
	case atspiRoleCheckBox:
		return "check box"
	case atspiRoleComboBox:
		return "combo box"
	case atspiRoleFrame:
		return "frame"
	case atspiRoleImage:
		return "image"
	case atspiRoleLabel:
		return "label"
	case atspiRoleList:
		return "list"
	case atspiRoleListItem:
		return "list item"
	case atspiRoleMenu:
		return "menu"
	case atspiRoleMenuBar:
		return "menu bar"
	case atspiRoleMenuItem:
		return "menu item"
	case atspiRolePageTab:
		return "page tab"
	case atspiRolePanel:
		return "panel"
	case atspiRolePassword:
		return "password text"
	case atspiRolePopupMenu:
		return "popup menu"
	case atspiRoleButton:
		return "push button"
	case atspiRoleRadioButton:
		return "radio button"
	case atspiRoleSeparator:
		return "separator"
	case atspiRoleSlider:
		return "slider"
	case atspiRoleStatusBar:
		return "status bar"
	case atspiRoleTable:
		return "table"
	case atspiRoleTableCell:
		return "table cell"
	case atspiRoleTerminal:
		return "terminal"
	case atspiRoleToggleButton:
		return "toggle button"
	case atspiRoleApplication:
		return "application"
	case atspiRoleEntry:
		return "text entry"
	case atspiRoleHeading:
		return "heading"
	case atspiRoleLink:
		return "link"
	}
	return "unknown"
}

// atspiStates is the state set of a node as the two 32-bit words GetState
// answers with. A node that is on screen is showing, visible, enabled and
// sensitive; the rest follow what the node says of itself.
func atspiStates(node platform.AccessibilityNode) [2]uint32 {
	var bits uint64
	set := func(on bool, state int) {
		if on {
			bits |= 1 << state
		}
	}
	set(true, atspiStateVisible)
	set(true, atspiStateShowing)
	set(!node.Disabled, atspiStateEnabled)
	set(!node.Disabled, atspiStateSensitive)
	set(node.Focusable, atspiStateFocusable)
	set(node.Focused, atspiStateFocused)
	patterns := node.Patterns()
	set(patterns.Has(platform.PatternToggle) || node.Role == platform.A11yRoleRadio || node.Checked, atspiStateCheckable)
	set(node.Checked || (node.Role == platform.A11yRoleRadio && node.Checked), atspiStateChecked)
	set(node.Mixed, atspiStateIndeterminate)
	set(patterns.Has(platform.PatternSelectionItem), atspiStateSelectable)
	set(node.Selected, atspiStateSelected)
	if patterns.Has(platform.PatternExpandCollapse) {
		set(true, atspiStateExpandable)
		set(node.Expanded, atspiStateExpanded)
		set(!node.Expanded, atspiStateCollapsed)
	}
	if patterns.Has(platform.PatternText) || node.Role == platform.A11yRoleCombobox {
		set(!node.ReadOnly && !node.Disabled, atspiStateEditable)
		set(node.ReadOnly, atspiStateReadOnly)
		set(node.Multiline, atspiStateMultiLine)
		set(!node.Multiline, atspiStateSingleLine)
	}
	set(node.Role == platform.A11yRoleCombobox, atspiStateHasPopup)
	return [2]uint32{uint32(bits), uint32(bits >> 32)}
}

// atspiFrameStates is the state set of a window.
func atspiFrameStates(active bool) [2]uint32 {
	var bits uint64
	for _, state := range []int{atspiStateEnabled, atspiStateSensitive, atspiStateShowing, atspiStateVisible, atspiStateResizable, atspiStateOpaque} {
		bits |= 1 << state
	}
	if active {
		bits |= 1 << atspiStateActive
	}
	return [2]uint32{uint32(bits), uint32(bits >> 32)}
}

// atspiInterfaces lists the interfaces a node implements, which a client asks
// for to know what it may call.
func atspiInterfaces(node platform.AccessibilityNode, actionable bool) []string {
	interfaces := []string{atspiIfaceAccessible, atspiIfaceComponent}
	patterns := node.Patterns()
	if actionable && len(atspiActions(node)) != 0 {
		interfaces = append(interfaces, atspiIfaceAction)
	}
	if patterns.Has(platform.PatternText) || node.Role == platform.A11yRoleCombobox {
		interfaces = append(interfaces, atspiIfaceText)
		if actionable && !node.ReadOnly {
			interfaces = append(interfaces, atspiIfaceEditableText)
		}
	}
	if node.Range.Set {
		interfaces = append(interfaces, atspiIfaceValue)
	}
	return interfaces
}

// atspiAction is one named thing a node can do.
type atspiAction struct {
	Name, Description string
}

// atspiActions are the actions of a node, the first being its default.
func atspiActions(node platform.AccessibilityNode) []atspiAction {
	if !node.Operable() {
		return nil
	}
	patterns := node.Patterns()
	var actions []atspiAction
	switch {
	case patterns.Has(platform.PatternToggle):
		actions = append(actions, atspiAction{"toggle", "Toggle the control"})
	case patterns.Has(platform.PatternSelectionItem):
		actions = append(actions, atspiAction{"select", "Choose the item"})
	case patterns.Has(platform.PatternExpandCollapse) && node.Role != platform.A11yRoleButton && node.Role != platform.A11yRoleMenuItem:
		if node.Expanded {
			actions = append(actions, atspiAction{"collapse", "Close the control"})
		} else {
			actions = append(actions, atspiAction{"expand", "Open the control"})
		}
	case patterns.Has(platform.PatternInvoke):
		actions = append(actions, atspiAction{"click", "Press the control"})
	}
	if node.Focusable {
		actions = append(actions, atspiAction{"focus", "Move the keyboard to the control"})
	}
	return actions
}

// atspiAttributes are the name/value pairs of GetAttributes, in order.
func atspiAttributes(node platform.AccessibilityNode) [][2]string {
	attributes := [][2]string{{"toolkit", "viva"}}
	if node.Role == platform.A11yRoleHeading && node.Level > 0 {
		attributes = append(attributes, [2]string{"level", strconv.Itoa(int(node.Level))})
	}
	if node.PosInSet > 0 {
		attributes = append(attributes, [2]string{"posinset", strconv.Itoa(int(node.PosInSet))})
	}
	if node.SetSize > 0 {
		attributes = append(attributes, [2]string{"setsize", strconv.Itoa(int(node.SetSize))})
	}
	switch node.Live {
	case gfx.LivePolite:
		attributes = append(attributes, [2]string{"live", "polite"}, [2]string{"container-live", "polite"})
	case gfx.LiveAssertive:
		attributes = append(attributes, [2]string{"live", "assertive"}, [2]string{"container-live", "assertive"})
	}
	if node.Role == platform.A11yRoleHeading {
		attributes = append(attributes, [2]string{"xml-roles", "heading"})
	}
	return attributes
}

// atspiTextUnit maps a granularity to our text unit. A sentence is not
// something the text layer knows, so it is the line.
func atspiTextUnit(granularity uint32) platform.AccessibilityTextUnit {
	switch granularity {
	case atspiGranularityChar:
		return platform.TextUnitCharacter
	case atspiGranularityWord:
		return platform.TextUnitWord
	case atspiGranularityParagraph:
		return platform.TextUnitParagraph
	}
	return platform.TextUnitLine
}

// atspiTextAt is the unit of text at offset: the string and its span.
func atspiTextAt(text []rune, offset int, granularity uint32) (string, int, int) {
	offset = max(0, min(offset, len(text)))
	if offset == len(text) {
		return "", len(text), len(text)
	}
	unit := atspiTextUnit(granularity)
	start, end := platform.TextUnitStart(text, offset, unit), platform.TextUnitEnd(text, offset, unit)
	return string(text[start:end]), start, end
}

// atspiTextBefore is the unit just before the one at offset.
func atspiTextBefore(text []rune, offset int, granularity uint32) (string, int, int) {
	offset = max(0, min(offset, len(text)))
	unit := atspiTextUnit(granularity)
	start := platform.TextUnitStart(text, offset, unit)
	if start == 0 {
		return "", 0, 0
	}
	return atspiTextAt(text, start-1, granularity)
}

// atspiTextAfter is the unit just after the one at offset.
func atspiTextAfter(text []rune, offset int, granularity uint32) (string, int, int) {
	offset = max(0, min(offset, len(text)))
	end := platform.TextUnitEnd(text, offset, atspiTextUnit(granularity))
	if end >= len(text) {
		return "", len(text), len(text)
	}
	return atspiTextAt(text, end, granularity)
}

// atspiTextEdit describes the change from one text to another as the single
// span that was removed and the text put in its place, found by trimming what
// the two share at both ends.
func atspiTextEdit(old, next []rune) (start, removed, added int) {
	for start < len(old) && start < len(next) && old[start] == next[start] {
		start++
	}
	oldEnd, nextEnd := len(old), len(next)
	for oldEnd > start && nextEnd > start && old[oldEnd-1] == next[nextEnd-1] {
		oldEnd--
		nextEnd--
	}
	return start, oldEnd - start, nextEnd - start
}
