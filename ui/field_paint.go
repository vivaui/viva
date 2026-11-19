package ui

import (
	"github.com/vivaui/viva/base"
	"github.com/vivaui/viva/field"
)

// Label adds a standalone field label: the field title's words and type, with
// Required adding the danger-colored marker and Disabled muting it. LabelFor
// makes it the label of a control, as it does for a Text.
//
//	email := ui.ID(c)
//	ui.Label(c, "Work email").Required().LabelFor(email)
//	ui.TextFieldID(c, email, &m.email, "Work email", "")
func Label(c C, Text string) Box { useField(); return base.Label(c, Text) }

// FieldOrientation is a FormField's arrangement: the title above the control,
// beside it, or above it while the field is narrow and beside it when it is
// wide. It is field.Orientation, kept here so a page names it without a second
// import.
type FieldOrientation = field.Orientation

const (
	// FieldVertical puts the title above the control; it is the zero value.
	FieldVertical = field.Vertical
	// FieldHorizontal puts the title in a column beside the control.
	FieldHorizontal = field.Horizontal
	// FieldResponsive is FieldVertical in a field narrower than 360 pixels and
	// FieldHorizontal in a wider one.
	FieldResponsive = field.Responsive
)
