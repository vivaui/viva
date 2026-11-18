package ui

// The control layer: how each control kind looks and how large it is. The
// engine keeps the seam (controlPainter, controlMeasurer) and no longer names
// these kinds here, so the engine can move below the styled controls; this file
// moves with them.
import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
)

// paintControls paints the controls: each kind's look, from package components
// for the ones that ship with a theme, and from the caller's own painter for
// the ones it declared.
func (f *frame) paintControls(n *node, r Rect) {
	if painter := controlPainters[n.Kind]; painter != nil {
		painter(f, n, r)
	}
}

// controlPainters paints one control kind. It is an array, not a map: the kind
// is a small number and the lookup must not hash on the frame's path.
var controlPainters = [...]func(f *frame, n *node, r Rect){
	kindButton:      (*frame).paintButtonControl,
	kindCheckbox:    (*frame).paintCheckboxControl,
	kindSwitch:      (*frame).paintSwitchControl,
	kindInput:       (*frame).paintInputControl,
	kindInputGroup:  (*frame).paintInputGroupControl,
	kindTextarea:    (*frame).paintTextareaControl,
	kindSegmented:   (*frame).paintSegmentedControl,
	kindTable:       (*frame).paintWidgetControl,
	kindTabs:        (*frame).paintWidgetControl,
	kindSelect:      (*frame).paintWidgetControl,
	kindProgress:    (*frame).paintProgressControl,
	kindSpinner:     (*frame).paintSpinnerControl,
	kindSplitter:    (*frame).paintSplitterControl,
	kindSplitButton: (*frame).paintSplitButtonControl,
	kindBadge:       (*frame).paintBadgeControl,
	kindPagination:  (*frame).paintPaginationControl,
	kindSlider:      (*frame).paintSliderControl,
}

func (f *frame) paintButtonControl(n *node, _ Rect)      { f.paintButton(n) }
func (f *frame) paintInputControl(n *node, _ Rect)       { f.paintInput(n) }
func (f *frame) paintTextareaControl(n *node, _ Rect)    { f.paintTextarea(n) }
func (f *frame) paintProgressControl(n *node, _ Rect)    { f.paintProgress(n) }
func (f *frame) paintSpinnerControl(n *node, _ Rect)     { f.paintSpinner(n) }
func (f *frame) paintSplitterControl(n *node, _ Rect)    { f.paintSplitter(n) }
func (f *frame) paintSliderControl(n *node, _ Rect)      { f.paintSlider(n) }
func (f *frame) paintWidgetControl(n *node, _ Rect)      { f.paintWidget(n) }
func (f *frame) paintSplitButtonControl(n *node, r Rect) { f.paintSplitButton(viewOf(f, n)) }
func (f *frame) paintBadgeControl(n *node, _ Rect)       { f.paintBadge(viewOf(f, n)) }
func (f *frame) paintPaginationControl(n *node, _ Rect)  { f.paintPagination(viewOf(f, n)) }

func (f *frame) paintCheckboxControl(n *node, r Rect)   { f.paintCheckbox(viewOf(f, n)) }
func (f *frame) paintSwitchControl(n *node, r Rect)     { f.paintSwitch(viewOf(f, n)) }
func (f *frame) paintInputGroupControl(n *node, r Rect) { f.paintInputGroup(viewOf(f, n)) }
func (f *frame) paintSegmentedControl(n *node, r Rect)  { f.paintSegmented(viewOf(f, n)) }

// paintInput paints the text of an Input or InputBase inside the element's Pad and Border.
// The element's box is already down, so only the content and the hit are left. The hit
// is the whole element, whose looks follow the same identity, while the platform is told
// the text sits in the padded rectangle.

// resetControls starts the control declarations of a paint: ui owns the tables
// and the engine only calls the hook.
var resetControls = func(f *frame) {
	w := widgetsOf(f)
	w.sweepPopoverFocus()
	w.reset()
}

// trimControls drops what a larger page left behind, after the paint.
var trimControls = func(f *frame) {
	w := widgetsOf(f)
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
func (f *frame) Input(snapshot components.InputSnapshot) Box {
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
func (f *frame) InputBase(snapshot components.InputSnapshot) Box {
	b := f.Box(kindInput)
	n := &f.Nodes[b.i]
	n.Aux = int32(len(widgetsOf(f).inputs))
	n.ID = snapshot.ID
	widgetsOf(f).inputs = append(widgetsOf(f).inputs, snapshot)
	return b
}

// InputGroup adds a components.PaintInputGroup (a field with slots beside or above
// it). Give it a Height when it has block slots; without one it is a control tall.
func (f *frame) InputGroup(snapshot components.InputGroupSnapshot) Box {
	b := f.Box(kindInputGroup)
	n := &f.Nodes[b.i]
	n.Aux = int32(len(widgetsOf(f).groups))
	widgetsOf(f).groups = append(widgetsOf(f).groups, snapshot)
	return b
}

// Textarea adds a multi-line text field with the shadcn look, at least 64 pixels tall. It
// is TextareaBase with that look, dressed by components.TextEntryRecipe as Input is: what
// the element sets itself (Background, Border, Radius, Shadow, Pad) replaces the recipe's.
func (f *frame) Textarea(snapshot components.TextareaSnapshot) Box {
	b := f.TextareaBase(snapshot)
	f.Nodes[b.i].Flags |= flagThemed
	return b
}

// TextareaBase adds a multi-line text field without a look, as InputBase is a single-line
// one: the rows, wrapping, selection, composition, caret and scrolling are the textarea's,
// and the fill, border, corners, shadow and padding are the element's. The snapshot's
// InsetTop, InsetBottom and InsetRight still reserve room inside the padded rectangle. It
// is one row tall plus its Pad and Border unless given a Height.
func (f *frame) TextareaBase(snapshot components.TextareaSnapshot) Box {
	b := f.Box(kindTextarea)
	n := &f.Nodes[b.i]
	n.Aux = int32(len(widgetsOf(f).textareas))
	n.ID = snapshot.ID
	widgetsOf(f).textareas = append(widgetsOf(f).textareas, snapshot)
	return b
}

// Segmented adds a components.SegmentedControl choosing among options, selected
// being the ID of the current one. The options slice is kept until End.
func (f *frame) Segmented(options []components.SegmentOption, selected gfx.ID) Box {
	b := f.Box(kindSegmented)
	n := &f.Nodes[b.i]
	n.Aux = int32(len(widgetsOf(f).segmented))
	widgetsOf(f).segmented = append(widgetsOf(f).segmented, segmented{options: options, selected: selected})
	return b
}

// controlBaseline is the label's baseline of a button, for a row that lines
// elements up by it. The engine asks the hook; the control layer answers.
var controlBaseline = func(f *frame, n *node) float32 {
	theme := f.Palette
	if n.HasOwnTheme() {
		theme = f.OwnTheme(n)
	}
	button := components.BeginButton(f.Surface, theme, gfx.ID{}, Rect{}).Size(n.ButtonSize)
	return f.Surface.LabelBaseline(
		gfx.Rect{Height: n.Pref[axisY]},
		gfx.TextStyle{Size: button.LabelFontSize()},
	)
}
