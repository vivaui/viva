package web

import (
	"strconv"

	"github.com/vivaui/viva/gfx"
)

const accessibilityIDPrefix = "viva-canvas-hit-"

type accessibilityDescription struct {
	ID        gfx.ID
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
	Range             gfx.ValueRange
}

func describeAccessibility(destination []accessibilityDescription, hits []gfx.Hit, activeScope gfx.ID) []accessibilityDescription {
	destination = destination[:0]
	// A label is not a node of its own: its words name the control with its
	// identity, as <label for> names an input. Collect them first so the control
	// is described once, with that name, instead of being replaced by a label
	// node and then by itself on every frame.
	var names map[gfx.ID]string
	for index := range hits {
		label := &hits[index]
		if !label.IsLabel || label.ID.IsZero() {
			continue
		}
		if names == nil {
			names = make(map[gfx.ID]string, 4)
		}
		if _, exists := names[label.ID]; !exists {
			names[label.ID] = label.Label
		}
	}
	for _, hit := range hits {
		if hit.Block || hit.MovesWindow || hit.IsLabel {
			continue // an occluder, a move region, or a label of another node
		}
		label := hit.Label
		if name, ok := names[hit.ID]; ok {
			label = name
		}
		destination = append(destination, accessibilityDescription{
			ID:        hit.ID,
			DOMID:     accessibilityDOMID(hit.ID),
			Element:   accessibilityElement(hit.Role, hit.Multiline, hit.Range.Set),
			Role:      accessibilityRole(hit.Role),
			Label:     label,
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

func accessibilityLive(live gfx.LiveRegion) string {
	switch live {
	case gfx.LivePolite:
		return "polite"
	case gfx.LiveAssertive:
		return "assertive"
	}
	return ""
}

func accessibilityElement(role gfx.Role, multiline, hasRange bool) string {
	if role == gfx.RoleCombobox {
		return "input"
	}
	// A slider with a declared range is published as a real range input so a
	// screen reader can change its value through the control it already knows
	// how to operate; the canvas still paints the slider.
	if role == gfx.RoleSlider && hasRange {
		return "input"
	}
	// A terminal is a text service surface too: hidden, but able to receive
	// input-method composition and committed text. Its own cells are painted
	// by the canvas.
	if role == gfx.RoleTerminal {
		return "textarea"
	}
	if role == gfx.RoleTextField {
		if multiline {
			return "textarea"
		}
		return "input"
	}
	return "div"
}

func accessibilityDOMID(id gfx.ID) string {
	bits := id.Bits()
	return accessibilityIDPrefix + strconv.FormatUint(bits>>32, 10) + "-" + strconv.FormatUint(bits&0xffffffff, 10)
}

func accessibilityRole(role gfx.Role) string {
	switch role {
	case gfx.RoleLink:
		return "link"
	case gfx.RoleCheckbox:
		return "checkbox"
	case gfx.RoleRadio:
		return "radio"
	case gfx.RoleTab:
		return "tab"
	case gfx.RoleMenuItem:
		return "menuitem"
	case gfx.RoleCombobox:
		return "combobox"
	case gfx.RoleSwitch:
		return "switch"
	case gfx.RoleTextField, gfx.RoleTerminal:
		return "textbox"
	case gfx.RoleSlider:
		return "slider"
	case gfx.RoleGroup:
		return "group"
	case gfx.RoleMenu:
		return "menu"
	case gfx.RoleMenubar:
		return "menubar"
	case gfx.RoleListBox:
		return "listbox"
	case gfx.RoleOption:
		return "option"
	case gfx.RoleGrid:
		return "grid"
	case gfx.RoleGridCell:
		return "gridcell"
	case gfx.RoleHeading:
		return "heading"
	case gfx.RoleStatus:
		return "status"
	case gfx.RoleSeparator:
		return "separator"
	default:
		return "button"
	}
}

func accessibilityActiveDescendant(descriptions []accessibilityDescription, focused gfx.ID) string {
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

func sameAccessibilityOrder(order []gfx.ID, descriptions []accessibilityDescription) bool {
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
