package base

import (
	"github.com/vivaui/viva/field"
	"github.com/vivaui/viva/gfx"
)

// FormFieldState is one form field's scratch, kept in the model next to the
// values the field shows; the zero value is ready to use. Keep one for each
// field, as a ListState is kept for each list: it is the layout the field is
// measured, placed and painted in, and it is reused every paint without
// allocating.
type FormFieldState struct {
	// Error is the line under the control, in the destructive color, and Invalid gives
	// the title the invalid look: set them before declaring the field, as BeginField's
	// Invalid and the error it is painted with. For, when set, makes the title the label
	// of the control with that identity: pressing it focuses and clicks the control.
	Error   string
	Invalid bool
	For     gfx.ID
	// Description is a line of help under the control, above Error, in the muted color.
	Description string
	// Message is a success or information line under the control, in the success
	// color: the confirmation of a field whose value is fine, where Error reports
	// one that is not. Error wins: while it is set, Message is not painted. The row
	// is the one the field already reserves for an error, so a message needs no
	// second control and does not change the field's height.
	Message string
	// Required marks the title with the required asterisk and makes the title the
	// control's label when For is set, as components.BeginField.Required does. The
	// platform accessibility contract has no separate required state, so the marker
	// and the label's words carry it.
	Required bool
	// Orientation places the title above the control (field.Vertical, the zero
	// value), beside it (field.Horizontal), or above it while the field is narrow
	// and beside it when it is wide (field.Responsive). A fixed row has room for the
	// orientation it is given.
	Orientation field.Orientation

	Label   string
	Palette *Theme
	layout  field.Layout
	paint   func(*DrawContext)
}

// FormField declares a form field: a title, and the control declared in its
// Children, with the same rhythm as components.BeginField. It is the form row
// an application builds with page elements instead of painting a
// components.BeginField into a slot:
//
//	ui.FormField(c, &m.proxy, m.ProxyRow).FillWidth().Children(func() {
//		ui.Row(c).Fill().Children(func() { ... })
//	})
//
// The field's children are its control: declare one (a Row, a Select, a
// Combobox, a control of the application's own); the field measures the title,
// places the control at the control slot of components.BeginField and paints
// the title around it. It does not bind a value, does not label the control
// (there is no For), and does not clip: a Select's or Combobox's popup may
// leave the field's bounds.
//
// A field with no explicit height is as tall as components.BeginField in the
// same bounds, including the room BeginField reserves for a description and an
// error; in a container that gives it a height (a fixed row, a Fill slot), it
// fills that height and places the same control slot. That is what a form row
// of a fixed height needs: the field does not grow past its row.
//
// Not inside a Memo. state is the field's; declaring two fields with one state
// in a paint is a mistake, as it is for a ListState.
func FormField(c C, state *FormFieldState, Label string) Box {
	p := c.page
	p.ForbidInMemo("FormField")
	if state == nil {
		panic("ui: FormField needs its FormFieldState")
	}
	state.Label = Label
	state.Palette = c.Theme()
	if state.paint == nil {
		state.paint = state.draw
	}
	return p.container(c.f.customLayout(state)).Draw(state.paint)
}

// spec is the field's field.Spec: its title, error, invalid look and the control it labels, the subset
// BeginField.Paint(label, "", "") paints. The shared layout still reserves the
// description and error rows, so the field and a painted BeginField have the
// same height.
func (state *FormFieldState) spec() field.Spec {
	return field.Spec{
		Title: state.Label, Description: state.Description, Error: state.Error,
		Orientation: state.Orientation, Required: state.Required,
		Invalid: state.Invalid, Control: state.For,
	}
}

// MeasureWidth reports no preferred width: a form field takes the width its
// row gives it, so give it a Width, FillWidth or Grow.
func (state *FormFieldState) MeasureWidth(*LayoutContext) float32 { return 0 }

func (state *FormFieldState) PlaceWidth(ctx *LayoutContext, bounds Rect) {
	if ctx.Count() > 1 {
		panic("ui: FormField takes one control child; wrap them in a Row or Column")
	}
	if ctx.Count() == 1 {
		ctx.SetWidth(0, ctx.ContentWidth())
	}
}

// MeasureHeight is the field's own height: BeginField's title, control and the
// two rows it reserves.
func (state *FormFieldState) MeasureHeight(ctx *LayoutContext) float32 {
	layout := field.Measure(*state.Palette, Rect{Width: ctx.ContentWidth()}, state.spec())
	return layout.Bounds.Height
}

// PlaceHeight places the control at the control slot of the shared field
// layout. The height the container gives is the field's; the control keeps the
// control height BeginField uses.
func (state *FormFieldState) PlaceHeight(ctx *LayoutContext, bounds Rect) {
	if ctx.Count() > 1 {
		panic("ui: FormField takes one control child; wrap them in a Row or Column")
	}
	state.layout = field.Measure(*state.Palette, bounds, state.spec())
	if ctx.Count() == 1 {
		control := state.layout.Control
		ctx.SetX(0, control.X)
		ctx.SetY(0, control.Y)
		ctx.SetHeight(0, control.Height)
	}
}

// draw paints the title the layout placed. It runs after the container's own
// background and border and before the control child, as Draw does.
func (state *FormFieldState) draw(d *DrawContext) {
	if state.layout.Bounds.Width <= 0 && state.layout.Control.Width <= 0 {
		return
	}
	field.Paint(d.Canvas, *d.Theme, state.layout, state.spec())
	state.paintMessage(d)
}

// paintMessage writes Message in the row the shared layout reserves for an error,
// when no error is there to take it. The line is painted rather than declared as
// another element: the field owns this row's height already, and a message is
// text, not a control.
func (state *FormFieldState) paintMessage(d *DrawContext) {
	if state.Message == "" || state.Error != "" {
		return
	}
	metrics := d.Theme.Metrics
	top := state.layout.Control.Y + state.layout.Control.Height
	if description := state.layout.Description; description.Height > 0 && description.Y >= top {
		top = description.Y + description.Height
	}
	bounds := gfx.Rect{
		X: state.layout.Control.X, Y: top + field.Gap(metrics),
		Width: state.layout.Control.Width, Height: field.TextHeight(metrics),
	}
	d.Canvas.Label(state.Message, bounds, gfx.TextStyle{
		Size: metrics.FontSizeM, Weight: gfx.WeightRegular, Color: d.Theme.Success,
	})
}

// Label adds a standalone field label: its words in the field title's type (the
// medium weight at the body size, muted when disabled), with the required marker
// when Required asks for it. It is the title BeginField paints, without the
// control or the rows a form field reserves, and it labels a control when given
// LabelFor, as a Text does:
//
//	email := ui.ID(c)
//	ui.Label(c, "Work email").Required().LabelFor(email)
//	ui.TextFieldID(c, email, &m.email, "Work email", "")
//
// Its width is the words' (plus the marker when Required is called); a later
// Width, FillWidth or Grow may size it as any element. A disabled Label is
// muted and does not forward presses to the control it labels.
func Label(c C, Text string) Box {
	metrics := c.Theme().Metrics
	b := c.f.Slot()
	n := b.N()
	n.Label = Text
	n.Size = [2]Dimension{
		{dimPx, c.Canvas().TextWidthWeight(Text, metrics.FontSizeM, gfx.WeightMedium)},
		{dimPx, field.TitleHeight(metrics)},
	}
	b.DrawOver(paintLabel)
	return b
}

// Required appends the required marker BeginField's title has: an asterisk in the
// danger color after the words. Calling it again changes nothing, and it is only
// for a Label.
func (b Box) Required() Box {
	n := b.N()
	if n.Kind != kindSlot {
		panic("ui: Required is for a Label")
	}
	if n.Flags&flagRequired != 0 {
		return b
	}
	n.Flags |= flagRequired
	if n.Size[AxisX].Kind == dimPx {
		n.Size[AxisX].Value += 3 + b.f.Surface.TextWidthWeight("*", b.f.Palette.Metrics.FontSizeM, gfx.WeightMedium)
	}
	return b
}

// paintLabel paints a Label with the field title's semantics: the shared
// field.Paint draws the words, the marker and the disabled treatment, and the
// element itself carries the LabelFor hit.
func paintLabel(d *DrawContext) {
	n := d.Node
	field.Paint(d.Canvas, *d.Theme, field.Layout{Bounds: d.Rect, Label: d.Rect}, field.Spec{
		Title: n.Label, Required: n.Flags&flagRequired != 0, Disabled: n.Flags&flagDisabled != 0,
	})
}
