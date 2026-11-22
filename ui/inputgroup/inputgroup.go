// Package inputgroup is the opt-in page control for the bordered input
// groups of package components. A page composes one with a caller-owned
// ui.Field and chained options:
//
//	group := inputgroup.New(&m.domain).
//		Placeholder("project-name").
//		Prefix("https://").
//		Action(m.clearID, m.crossIcon, "Clear domain", m.clearDomain)
//	group.View(c, "Project domain").FillWidth()
//	if group.ActionClicked() {
//		// the action ran too, if it had a callback
//	}
//
// The field is the page's own ui.Field: its value, focus, caret, selection,
// composition and Secure/Invalid/Disabled state are painted through the
// component, and the platform text service edits the same editor a
// ui.TextField would. Prefix, Suffix, Icon, Action, Button and the block
// addons are chained options; there is no second constructor for a size or a
// visual variant. Small changes the button sizes, Spinner replaces the end
// action with a loading indicator.
//
// View declares a ui.Control around the component's text-field hit. By
// default the field's hit is the control's own identity, so a page configured
// with ui.Configure gives its platform the field's editor. FieldID names the
// hit explicitly for an embedding runner that resolves text by an application
// identity.
//
// It is a separate package so an application that does not import it pays
// nothing for it. Keep a Group in the model beside its Field: its handlers,
// text provider and draw function are made once, so a warm paint allocates
// nothing. Do not copy a Group after it has been declared.
package inputgroup

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/window"
)

// Group is one input group's caller-owned field and its page control.
// Configure it with the chainable methods before each View.
type Group struct {
	field       *ui.Field
	placeholder string
	fieldID     gfx.ID

	inlineStart components.InputGroupSlot
	inlineEnd   components.InputGroupSlot
	blockStart  components.InputGroupSlot
	blockEnd    components.InputGroupSlot

	small     bool
	multiline bool
	readOnly  bool
	actionID  gfx.ID
	action    func()

	// label is the accessible name of the paint being declared.
	label string

	handlers     ui.Handlers
	textProvider func(window.TextInputContext) *window.TextInput
	drawFn       func(*ui.DrawContext)
	prepared     bool
	clicked      gfx.ID
	hadClick     bool
}

// New returns an input group over field, an application-owned ui.Field. The
// field is required; bind it to the editor the page's text service edits, as a
// ui.TextField's field is bound.
func New(field *ui.Field) *Group {
	if field == nil {
		panic("inputgroup: New needs its ui.Field")
	}
	return &Group{field: field}
}

// Placeholder is the text shown while the field is empty.
func (g *Group) Placeholder(text string) *Group {
	g.placeholder = text
	return g
}

// FieldID gives the field's hit an explicit identity instead of the control's
// own. It is for a runner that resolves the platform text service by an
// application identity, such as a component preview.
func (g *Group) FieldID(id gfx.ID) *Group {
	g.fieldID = id
	return g
}

// Prefix puts a separated text addon before the field.
func (g *Group) Prefix(text string) *Group {
	g.inlineStart = components.InputGroupSlot{
		Kind: components.InputGroupSlotText, Text: text, Separated: true,
	}
	return g
}

// Suffix puts a separated text addon after the field.
func (g *Group) Suffix(text string) *Group {
	g.inlineEnd = components.InputGroupSlot{
		Kind: components.InputGroupSlotText, Text: text, Separated: true,
	}
	g.actionID, g.action = gfx.ID{}, nil
	return g
}

// Icon puts a decorative icon before the field.
func (g *Group) Icon(icon gfx.Icon) *Group {
	g.inlineStart = components.InputGroupSlot{Kind: components.InputGroupSlotIcon, Icon: icon}
	return g
}

// Action puts an icon button after the field. id is the caller-owned hit
// identity; onClick runs after the click is read, and may be nil.
func (g *Group) Action(id gfx.ID, icon gfx.Icon, label string, onClick func()) *Group {
	g.inlineEnd = components.InputGroupSlot{
		Kind: components.InputGroupSlotButton, ID: id, Icon: icon, Label: label,
		ButtonSize: g.iconButtonSize(),
	}
	g.actionID, g.action = id, onClick
	return g
}

// Button puts a text button after the field. id is the caller-owned hit
// identity; onClick runs after the click is read, and may be nil.
func (g *Group) Button(id gfx.ID, label string, onClick func()) *Group {
	g.inlineEnd = components.InputGroupSlot{
		Kind: components.InputGroupSlotButton, ID: id, Text: label,
		ButtonSize: g.textButtonSize(),
	}
	g.actionID, g.action = id, onClick
	return g
}

// Small selects the small button sizes for the action and button slots.
func (g *Group) Small() *Group {
	g.small = true
	if g.inlineEnd.Kind == components.InputGroupSlotButton {
		g.inlineEnd.ButtonSize = g.buttonSizeFor(g.inlineEnd)
	}
	return g
}

// DisableAction paints the end action or button inert without changing the
// field, as the invalid state of a form does with an invitation button.
func (g *Group) DisableAction() *Group {
	g.inlineEnd.Disabled = true
	return g
}

// Spinner replaces the end action with a loading indicator at phase. Advancing
// the phase is the application's job.
func (g *Group) Spinner(phase components.SpinnerPhase) *Group {
	g.inlineEnd = components.InputGroupSlot{Kind: components.InputGroupSlotSpinner, Phase: phase}
	g.actionID, g.action = gfx.ID{}, nil
	return g
}

// Multiline makes the field several rows tall, as a textarea is.
func (g *Group) Multiline() *Group {
	g.multiline = true
	return g
}

// ReadOnly paints the field as not editable without taking it out of the tab
// order or the pointer's reach.
func (g *Group) ReadOnly() *Group {
	g.readOnly = true
	return g
}

// Editable clears ReadOnly, for a page that enables editing after the first
// paint.
func (g *Group) Editable() *Group {
	g.readOnly = false
	return g
}

// BlockStart puts a full-width text addon above the field.
func (g *Group) BlockStart(text string) *Group {
	g.blockStart = components.InputGroupSlot{
		Kind: components.InputGroupSlotText, Text: text, Separated: true,
	}
	return g
}

// BlockEnd puts a full-width text addon below the field.
func (g *Group) BlockEnd(text string) *Group {
	g.blockEnd = components.InputGroupSlot{Kind: components.InputGroupSlotText, Text: text}
	return g
}

// BlockKbd puts a full-width keyboard badge below the field.
func (g *Group) BlockKbd(text string) *Group {
	g.blockEnd = components.InputGroupSlot{Kind: components.InputGroupSlotKbd, Text: text}
	return g
}

// View declares the group at c. label names the field for assistive
// technology. The box is one control row high, plus any block addons; give it a
// width.
//
// Declare the view on its own, not inside a Memo.
func (g *Group) View(c ui.C, label string) ui.Box {
	if g == nil {
		panic("inputgroup: View needs its Group")
	}
	g.prepare()
	g.label = label
	return ui.Control(c, label, &g.handlers).Height(g.height(*c.Theme())).Draw(g.drawFn)
}

// ActionClicked reports whether the paint just made delivered a click on the
// group's action. It is false in ordinary paints or when there is no action.
func (g *Group) ActionClicked() bool {
	return g.hadClick && !g.actionID.IsZero() && g.clicked == g.actionID
}

func (g *Group) prepare() {
	if g.prepared {
		return
	}
	g.prepared = true
	g.handlers.Role = gfx.RoleGroup
	g.handlers.Text = g.text
	g.textProvider = ui.Fields(g.field)
	g.drawFn = g.draw
}

func (g *Group) text(context window.TextInputContext) *window.TextInput {
	if g.textProvider == nil {
		return nil
	}
	return g.textProvider(context)
}

func (g *Group) iconButtonSize() components.InputGroupButtonSize {
	if g.small {
		return components.InputGroupButtonIconSM
	}
	return components.InputGroupButtonIconXS
}

func (g *Group) textButtonSize() components.InputGroupButtonSize {
	if g.small {
		return components.InputGroupButtonSM
	}
	return components.InputGroupButtonXS
}

func (g *Group) buttonSizeFor(slot components.InputGroupSlot) components.InputGroupButtonSize {
	if slot.Text == "" && (len(slot.Icon.Commands) != 0 || len(slot.Icon.Layers) != 0) {
		return g.iconButtonSize()
	}
	return g.textButtonSize()
}

func (g *Group) height(theme gfx.Theme) float32 {
	height := theme.Metrics.ControlHeight
	if g.multiline {
		height = 64
	}
	if g.blockStart.Kind != components.InputGroupSlotNone {
		height += 32
	}
	if g.blockEnd.Kind != components.InputGroupSlotNone {
		height += 32
	}
	return height
}

func (g *Group) draw(d *ui.DrawContext) {
	id := g.fieldID
	if id.IsZero() {
		id = d.Node.ID
	}
	g.clicked, g.hadClick = d.Canvas.ClickTarget()
	input := g.field.Snapshot(d.Frame, id, g.label, g.placeholder)
	snapshot := components.InputGroupSnapshot{
		ID: id, Label: g.label, Value: input.Value, Placeholder: g.placeholder,
		Disabled: input.Disabled, Invalid: input.Invalid, Secure: input.Secure,
		Focused: input.Focused, Multiline: g.multiline, ReadOnly: g.readOnly,
		HideEditorText: true, ShowCaret: input.ShowCaret, Caret: input.Caret,
		SelectionStart: input.SelectionStart, SelectionEnd: input.SelectionEnd,
		CompositionStart: input.CompositionStart, CompositionEnd: input.CompositionEnd,
		Blink:       input.Blink,
		InlineStart: g.inlineStart, InlineEnd: g.inlineEnd,
		BlockStart: g.blockStart, BlockEnd: g.blockEnd,
	}
	components.PaintInputGroup(d.Canvas, *d.Theme, d.Rect, snapshot)
	if g.hadClick && !g.actionID.IsZero() && g.clicked == g.actionID {
		d.Canvas.RequestFocus(id)
		if g.action != nil {
			g.action()
		}
	}
}
