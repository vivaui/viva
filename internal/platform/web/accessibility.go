package web

import (
	"strconv"

	"github.com/vivaui/viva/ui"
)

const accessibilityIDPrefix = "viva-canvas-hit-"

type accessibilityDescription struct {
	ID        ui.ID
	DOMID     string
	Element   string
	Role      string
	Label     string
	Value     string
	Hint      string
	Disabled  bool
	Checked   bool
	Mixed     bool
	Selected  bool
	Expanded  bool
	Multiline bool
	ReadOnly  bool
	Secure    bool
	Hidden    bool
	// Expandable says the control opens and closes, whichever state it is in.
	Expandable bool
	// Description is the Hint and the longer Description of the hit, as one
	// aria-description.
	Description       string
	Level             uint8
	PosInSet, SetSize int32
	Live              string
	Range             ui.ValueRange
}

func describeAccessibility(destination []accessibilityDescription, hits []ui.Hit, activeScope ui.ID) []accessibilityDescription {
	destination = destination[:0]
	for _, hit := range hits {
		if hit.Block || hit.MovesWindow {
			continue // an occluder or a move region, not a target
		}
		destination = append(destination, accessibilityDescription{
			ID:        hit.ID,
			DOMID:     accessibilityDOMID(hit.ID),
			Element:   accessibilityElement(hit.Role, hit.Multiline, hit.Range.Set),
			Role:      accessibilityRole(hit.Role),
			Label:     hit.Label,
			Value:     hit.Value,
			Hint:      hit.Hint,
			Disabled:  hit.Disabled,
			Checked:   hit.Checked,
			Mixed:     hit.Mixed,
			Selected:  hit.Selected,
			Expanded:  hit.Expanded,
			Multiline: hit.Multiline,
			ReadOnly:  hit.ReadOnly,
			Secure:    hit.Secure,
			Hidden:    !activeScope.IsZero() && hit.Scope != activeScope,

			Expandable:  hit.Expandable,
			Description: accessibilityDescribedBy(hit.Hint, hit.Description),
			Level:       hit.Level,
			PosInSet:    hit.PosInSet,
			SetSize:     hit.SetSize,
			Live:        accessibilityLive(hit.Live),
			Range:       hit.Range,
		})
	}
	return destination
}

// accessibilityDescribedBy joins the short hint and the longer description a
// control has into the one aria-description the DOM offers.
func accessibilityDescribedBy(hint, description string) string {
	switch {
	case description == "":
		return hint
	case hint == "":
		return description
	}
	return hint + ". " + description
}

func accessibilityLive(live ui.LiveRegion) string {
	switch live {
	case ui.LivePolite:
		return "polite"
	case ui.LiveAssertive:
		return "assertive"
	}
	return ""
}

func accessibilityElement(role ui.Role, multiline, hasRange bool) string {
	if role == ui.RoleCombobox {
		return "input"
	}
	// A slider with a declared range is published as a real range input so a
	// screen reader can change its value through the control it already knows
	// how to operate; the canvas still paints the slider.
	if role == ui.RoleSlider && hasRange {
		return "input"
	}
	// A terminal is a text service surface too: hidden, but able to receive
	// input-method composition and committed text. Its own cells are painted
	// by the canvas.
	if role == ui.RoleTerminal {
		return "textarea"
	}
	if role == ui.RoleTextField {
		if multiline {
			return "textarea"
		}
		return "input"
	}
	return "div"
}

func accessibilityDOMID(id ui.ID) string {
	bits := id.Bits()
	return accessibilityIDPrefix + strconv.FormatUint(bits>>32, 10) + "-" + strconv.FormatUint(bits&0xffffffff, 10)
}

func accessibilityRole(role ui.Role) string {
	switch role {
	case ui.RoleLink:
		return "link"
	case ui.RoleCheckbox:
		return "checkbox"
	case ui.RoleRadio:
		return "radio"
	case ui.RoleTab:
		return "tab"
	case ui.RoleMenuItem:
		return "menuitem"
	case ui.RoleCombobox:
		return "combobox"
	case ui.RoleSwitch:
		return "switch"
	case ui.RoleTextField, ui.RoleTerminal:
		return "textbox"
	case ui.RoleSlider:
		return "slider"
	case ui.RoleGroup:
		return "group"
	case ui.RoleMenu:
		return "menu"
	case ui.RoleMenubar:
		return "menubar"
	case ui.RoleListBox:
		return "listbox"
	case ui.RoleOption:
		return "option"
	case ui.RoleGrid:
		return "grid"
	case ui.RoleGridCell:
		return "gridcell"
	case ui.RoleHeading:
		return "heading"
	case ui.RoleStatus:
		return "status"
	case ui.RoleSeparator:
		return "separator"
	default:
		return "button"
	}
}

func accessibilityActiveDescendant(descriptions []accessibilityDescription, focused ui.ID) string {
	if focused.IsZero() {
		return ""
	}
	for _, description := range descriptions {
		if description.ID == focused && !description.Hidden && !description.Disabled {
			return description.DOMID
		}
	}
	return ""
}

func sameAccessibilityOrder(order []ui.ID, descriptions []accessibilityDescription) bool {
	if len(order) != len(descriptions) {
		return false
	}
	for index, description := range descriptions {
		if order[index] != description.ID {
			return false
		}
	}
	return true
}
