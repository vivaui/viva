// Package multiselect is the opt-in page control for a multi-select: removable
// chips, a native query input and a filtered popup, painted by
// components.PaintMultiSelect and driven by base.MultiSelectState.
//
// It is a separate package so an application that does not import it pays
// nothing for it: the control's types and methods are not reachable from the
// page engine or package ui.
//
//	picker := multiselect.New(&m.tags)
//	...
//	picker.View(c, multiselect.Spec{
//		Label: "Tags", Placeholder: "Add a tag",
//		Options: tagOptions, Removes: tagRemoves,
//	}).FillWidth()
package multiselect

import (
	"github.com/vivaui/viva/base"
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/window"
)

// State is the selection, options and query of one picker; Option is one
// selectable value. They are the engine's types, re-exported so an application
// that only uses this package does not import package base.
type (
	State  = base.MultiSelectState
	Option = base.MultiSelectOption
)

// Spec supplies the control's words and the two scopes of its popup, from the
// same gfx.Tree the page names its elements in:
// Options.At(i) names choice i and Removes.At(i) names the remove button of
// value i. Make them once in the application's tree; an empty scope paints the
// rows but gives them no identities.
type Spec struct {
	Label, Placeholder, EmptyLabel, CreatePrefix string
	MaxRows                                      int
	Invalid                                      bool
	Options, Removes                             gfx.Scope
}

// Picker is one MultiSelect's page control. Keep it in the model beside the
// state; one Picker belongs to one state and one place on a page. Its handlers
// and draw function are made once, so a warm paint allocates nothing.
type Picker struct {
	State *base.MultiSelectState

	spec     Spec
	handlers ui.Handlers
	drawFn   func(*ui.DrawContext)
	input    window.TextInput
}

// New returns a picker for state.
func New(state *base.MultiSelectState) *Picker {
	if state == nil {
		panic("multiselect: New needs its MultiSelectState")
	}
	p := &Picker{State: state}
	p.handlers.Role = gfx.RoleCombobox
	p.handlers.Key = p.key
	p.handlers.Pointer = p.pointer
	p.handlers.Scroll = p.scroll
	p.handlers.Text = p.text
	p.drawFn = p.draw
	p.input.Editor = &state.Query
	return p
}

// View declares the picker at c. Give it a width: it is as tall as the theme's
// control, unless a Height follows. Declare it on its own, not inside a Memo.
func (p *Picker) View(c ui.C, spec Spec) ui.Box {
	if p.State == nil || p.drawFn == nil {
		panic("multiselect: use New to make a Picker")
	}
	p.spec = spec
	return ui.Control(c, spec.Label, &p.handlers).
		Height(c.Theme().Metrics.ControlHeight).
		Draw(p.drawFn)
}

// Wheel scrolls the open popup. A page whose wheel handler goes elsewhere
// first calls it while Open, then falls back to the page:
//
//	if m.tags.Open && m.tagsPicker.Wheel(e) { return }
func (p *Picker) Wheel(event gfx.ScrollEvent) bool {
	if p.State == nil || !p.State.Open {
		return false
	}
	return p.State.Wheel(event)
}

func (p *Picker) key(event gfx.KeyEvent) gfx.EventResult {
	return p.State.Key(event)
}

func (p *Picker) pointer(event gfx.PointerEvent) gfx.EventResult {
	return p.State.Pointer(event)
}

func (p *Picker) scroll(event gfx.ScrollEvent) gfx.EventResult {
	if p.Wheel(event) {
		return gfx.EventResult{Handled: true, Repaint: true}
	}
	return gfx.EventResult{}
}

func (p *Picker) text(window.TextInputContext) *window.TextInput {
	return &p.input
}

func (p *Picker) draw(d *ui.DrawContext) {
	parts := p.handlers.Parts()
	if parts.IsZero() {
		return
	}
	options := components.MultiSelectOptions{
		InputID: d.Node.ID, TriggerID: parts.At(0), DismissID: parts.At(1),
		Scope: parts.At(2), ScrollID: parts.At(3),
		OptionIDs: p.spec.Options, RemoveIDs: p.spec.Removes,
		Label: p.spec.Label, Placeholder: p.spec.Placeholder,
		EmptyLabel: p.spec.EmptyLabel, CreatePrefix: p.spec.CreatePrefix,
		Viewport: d.Frame.GetBounds(), Layer: d.Frame.WidgetLayer() + 2,
		Disabled: d.State.Disabled, Invalid: p.spec.Invalid, MaxRows: p.spec.MaxRows,
	}
	components.PaintMultiSelect(d.Canvas, *d.Theme, d.Rect, p.State, options)
	p.pick(d.Canvas, options)
}

// pick applies this paint's presses. The input itself is the control's own hit,
// so the platform's text service finds its editor through Handlers.Text; the
// trigger, the dismiss target and the popup's rows are parts.
func (p *Picker) pick(canvas *gfx.Canvas, options components.MultiSelectOptions) {
	if canvas == nil {
		return
	}
	if canvas.Clicked(options.TriggerID) {
		p.State.Open = !p.State.Open
		return
	}
	if canvas.Clicked(options.DismissID) {
		p.State.Open = false
		return
	}
	if index, ok := canvas.ClickedIn(options.RemoveIDs); ok {
		p.State.Remove(index)
		return
	}
	if index, ok := canvas.ClickedIn(options.OptionIDs); ok {
		p.State.Choose(index)
	}
}
