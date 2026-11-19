package base

// The control layer: how each control kind looks and how large it is. The
// engine keeps the seam (controlPainter, controlMeasurer) and no longer names
// these kinds here, so the engine can move below the styled controls; this file
// moves with them.
import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/style"
)

// resetControls starts the control declarations of a paint: ui owns the tables
// and the engine only calls the hook.
var resetControls = func(f *Frame) {
	w := WidgetsOf(f)
	w.sweepPopoverFocus()
	w.reset()
}

// trimControls drops what a larger page left behind, after the paint.
var trimControls = func(f *Frame) {
	w := WidgetsOf(f)
	w.tables = trimFlexCapacity(w.tables)
	w.tabs = trimFlexCapacity(w.tabs)
	w.selects = trimFlexCapacity(w.selects)
	w.rows = trimFlexCapacity(w.rows)
	w.dialogs = trimFlexCapacity(w.dialogs)
	w.popovers = trimFlexCapacity(w.popovers)
	w.popoverFocus = trimFlexCapacity(w.popoverFocus)
	w.splits = trimFlexCapacity(w.splits)
	w.sliders = trimFlexCapacity(w.sliders)
	w.cells = trimFlexCapacity(w.cells)
	w.rects = trimFlexCapacity(w.rects)
	w.inputs = trimFlexCapacity(w.inputs)
	w.groups = trimFlexCapacity(w.groups)
	w.textareas = trimFlexCapacity(w.textareas)
	w.segmented = trimFlexCapacity(w.segmented)
	w.choices = trimFlexCapacity(w.choices)
}

// Input adds a single-line text field with the shadcn look, the height of a control and
// as wide as its container unless given a width. It is InputBase with that look: the
// recipe (components.TextEntryRecipe) fills in the fill, border, corners, shadow and the
// padding of the text, and whatever the element sets itself (Background, Border, Radius,
// Shadow, Pad) replaces the recipe's, so the field is restyled with the calls any element
// takes rather than with a variant:
//
//	f.Input(snapshot).Radius(0).Border(2, theme.Primary)
//
// The snapshot is copied, so the caller may reuse it; its strings must stay valid until End.
func (f *Frame) Input(snapshot InputSnapshot) Box {
	b := f.InputBase(snapshot)
	f.Nodes[b.i].Flags |= flagThemed
	return b
}

// InputBase adds a single-line text field without a look: no fill, border, shadow or
// padding of its own, and no focus color. What it looks like is the element's, with the
// calls any element takes (Background, Border, Radius, Shadow, Pad, Ring and the Hover,
// Pressed and Focus looks, which follow the snapshot's ID); what it does is Input's:
// the value, placeholder, selection, input-method composition, caret and masking are
// painted by the same code and the platform edits it. The text sits inside the element's
// Pad and Border. FontSize, Weight, Color and Mono (with MonoFace) style the text (with the
// snapshot's own Text for what the element leaves unsaid): one value for the glyphs, the caret, the
// selection and what the platform is told. A Mono field is painted by the component, since the
// platform's text service draws one proportional face: it still edits through the platform, whose
// own text and overlays are hidden, and a FieldBase gives the component the caret, the selection and
// the composition from its editor. Radius rounds the element's own box.
//
// It is as tall as a line of its text (LineHeight, or the size plus seven) plus its Pad and
// Border unless given a Height, and as wide as its container unless given a Width. Invalid has no look here: the caller draws
// it, from the snapshot it passed.
//
//	f.InputBase(snapshot).Pad(8).Border(2, theme.Primary).Radius(0).Background(theme.Background)
func (f *Frame) InputBase(snapshot InputSnapshot) Box {
	b := f.Box(kindInput)
	n := &f.Nodes[b.i]
	n.Aux = int32(len(WidgetsOf(f).inputs))
	n.ID = snapshot.ID
	WidgetsOf(f).inputs = append(WidgetsOf(f).inputs, snapshot)
	return b
}

// InputGroup adds a components.PaintInputGroup (a field with slots beside or above
// it). Give it a Height when it has block slots; without one it is a control tall.
func (f *Frame) InputGroup(snapshot InputGroupSnapshot) Box {
	b := f.Box(kindInputGroup)
	n := &f.Nodes[b.i]
	n.Aux = int32(len(WidgetsOf(f).groups))
	WidgetsOf(f).groups = append(WidgetsOf(f).groups, snapshot)
	return b
}

// Textarea adds a multi-line text field with the shadcn look, at least 64 pixels tall. It
// is TextareaBase with that look, dressed by components.TextEntryRecipe as Input is: what
// the element sets itself (Background, Border, Radius, Shadow, Pad) replaces the recipe's.
func (f *Frame) Textarea(snapshot TextareaSnapshot) Box {
	b := f.TextareaBase(snapshot)
	f.Nodes[b.i].Flags |= flagThemed
	return b
}

// TextareaBase adds a multi-line text field without a look, as InputBase is a single-line
// one: the rows, wrapping, selection, composition, caret and scrolling are the textarea's,
// and the fill, border, corners, shadow and padding are the element's. The snapshot's
// InsetTop, InsetBottom and InsetRight still reserve room inside the padded rectangle. It
// is one row tall plus its Pad and Border unless given a Height.
func (f *Frame) TextareaBase(snapshot TextareaSnapshot) Box {
	b := f.Box(kindTextarea)
	n := &f.Nodes[b.i]
	n.Aux = int32(len(WidgetsOf(f).textareas))
	n.ID = snapshot.ID
	WidgetsOf(f).textareas = append(WidgetsOf(f).textareas, snapshot)
	return b
}

// Segmented adds a components.SegmentedControl choosing among options, selected
// being the ID of the current one. The options slice is kept until End. An option's
// own Disabled field marks it unavailable; DisabledOptions marks options of the
// control the page declared.
func (f *Frame) Segmented(options []style.SegmentOption, selected gfx.ID) Box {
	b := f.Box(kindSegmented)
	n := &f.Nodes[b.i]
	n.Aux = int32(len(WidgetsOf(f).segmented))
	WidgetsOf(f).segmented = append(WidgetsOf(f).segmented, segmented{options: options, selected: selected})
	return b
}

// Choice adds a segmented control choosing among labels, selected being the index of
// the current one (negative for none). Its options are identified by scope.At(0),
// scope.At(1), ...: declare a scope for it once (tree.Sub(page, tree.Name("theme"))),
// and read the click with Box.ChosenIndex. Mark options unavailable with
// DisabledOptions, one entry per label.
//
//	if i, ok := f.Choice(themeScope, themeLabels, m.theme).ChosenIndex(); ok { m.theme = i }

func (f *Frame) Choice(scope gfx.Scope, labels []string, selected int) Box {
	first := len(WidgetsOf(f).choices)
	for index, label := range labels {
		WidgetsOf(f).choices = append(WidgetsOf(f).choices, style.SegmentOption{ID: scope.At(index), Label: label})
	}
	options := WidgetsOf(f).choices[first:len(WidgetsOf(f).choices):len(WidgetsOf(f).choices)]
	chosen := gfx.ID{}
	if selected >= 0 && selected < len(labels) {
		chosen = scope.At(selected)
	}
	return f.Segmented(options, chosen)
}
