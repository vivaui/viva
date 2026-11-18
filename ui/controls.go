package ui

// The control layer: how each control kind looks and how large it is. The
// engine keeps the seam (controlPainter, controlMeasurer) and no longer names
// these kinds here, so the engine can move below the styled controls; this file
// moves with them.
import (
	"math"

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

func (f *frame) paintCheckbox(v ControlView) {
	canvas := f.canvas
	side := components.CheckboxExtent(f.theme)
	bounds := Rect{X: v.Rect.X + (v.Rect.Width-side)/2, Y: v.Rect.Y + (v.Rect.Height-side)/2, Width: side, Height: side}
	if v.Text != "" {
		bounds.X = v.Rect.X
		f.paintCaptionView(v, bounds)
	}
	checkbox := components.BeginCheckbox(canvas, f.theme, v.ID, bounds, v.Flags&flagChecked != 0)
	if v.Flags&flagMixed != 0 {
		checkbox = checkbox.Indeterminate()
	}
	if f.styles != nil && f.styles.Checkbox != nil {
		checkbox = checkbox.Recipe(f.styles.Checkbox)
	}
	if v.Label != "" {
		checkbox = checkbox.Label(v.Label)
	}
	if v.Disabled {
		checkbox = checkbox.Disabled()
	}
	checkbox.Paint()
}

func (f *frame) paintSwitch(v ControlView) {
	canvas := f.canvas
	small := v.Flags&flagSmall != 0
	width, height := components.SwitchExtent(f.theme, small)
	bounds := Rect{X: v.Rect.X + (v.Rect.Width-width)/2, Y: v.Rect.Y + (v.Rect.Height-height)/2, Width: width, Height: height}
	if v.Text != "" {
		bounds.X = v.Rect.X
		f.paintCaptionView(v, bounds)
	}
	control := components.BeginSwitch(canvas, f.theme, v.ID, bounds, v.Flags&flagChecked != 0)
	if recipe := v.SwitchRecipe; recipe != nil {
		control = control.Recipe(recipe)
	} else if f.styles != nil && f.styles.Switch != nil {
		control = control.Recipe(f.styles.Switch)
	}
	if small {
		control = control.Small()
	}
	if v.Label != "" {
		control = control.Label(v.Label)
	}
	if v.Disabled {
		control = control.Disabled()
	}
	control.Paint()
}

func (f *frame) paintInputGroup(v ControlView) {
	components.PaintInputGroup(f.canvas, f.theme, v.Rect, widgetsOf(f).groups[v.Aux])
}

func (f *frame) paintSegmented(v ControlView) {
	choice := widgetsOf(f).segmented[v.Aux]
	components.BeginSegmentedControl(f.canvas, f.theme, v.Rect, choice.options, choice.selected).Paint()
}

// measureControls measures the controls: each kind's geometry, from package
// components for the ones that ship with a theme.
func (f *frame) measureControls(n *node) {
	switch n.Kind {
	case kindButton:
		button := components.BeginButton(f.canvas, f.theme, gfx.ID{}, Rect{}).Size(n.ButtonSize).WithIcon(f.exts[n.Ext].icon)
		if d := n.Size[axisY]; d.Kind == dimPx {
			button = button.Height(d.value) // an icon button is as wide as it is tall
		}
		button = button.SizeToFit(n.label)
		n.pref = [2]float32{button.BoundsWidth(), button.BoundsHeight()}
		if n.ButtonSize >= components.ButtonSizeIcon {
			n.pref[axisX] = n.pref[axisY]
		}
	case kindBadge:
		w, h := components.BadgeExtent(f.canvas, f.theme, n.text)
		n.pref = [2]float32{w, h}
	case kindPagination:
		w, h := components.PaginationExtent(f.canvas, f.theme, f.paginationItems(n))
		n.pref = [2]float32{w, h}
	case kindSplitButton:
		button := components.BeginButton(f.canvas, f.theme, gfx.ID{}, Rect{}).Size(n.ButtonSize).SizeToFit(n.label)
		trigger := widgetsOf(f).splitButtons[n.Aux].triggerWidth
		if trigger <= 0 {
			trigger = splitButtonTrigger
		}
		n.pref = [2]float32{button.BoundsWidth() + trigger, button.BoundsHeight()}
	case kindCheckbox:
		w, h := components.CheckboxHitExtent(f.theme)
		n.pref = [2]float32{w, h}
		if n.text != "" {
			n.pref[axisX] = components.CheckboxExtent(f.theme) + captionGap + f.canvas.TextWidthWeight(n.text, f.captionSize(n), gfx.WeightMedium)
		}
	case kindSwitch:
		w, h := components.SwitchHitExtent(f.theme, n.Flags&flagSmall != 0)
		n.pref = [2]float32{w, h}
		if n.text != "" {
			control, _ := components.SwitchExtent(f.theme, n.Flags&flagSmall != 0)
			n.pref[axisX] = control + captionGap + f.canvas.TextWidthWeight(n.text, f.captionSize(n), gfx.WeightMedium)
		}
	case kindSegmented:
		var width float32 = 8
		for _, option := range widgetsOf(f).segmented[n.Aux].options {
			width += f.canvas.TextWidthWeight(option.Label, f.theme.Metrics.FontSizeM, gfx.WeightMedium) + 28
		}
		n.pref = [2]float32{width, f.theme.Metrics.ControlHeight}
	case kindInput:
		n.pref = [2]float32{n.padX(), f.inputLine(n) + n.padY()}
		if n.Flags&flagThemed != 0 {
			n.pref = [2]float32{0, f.theme.Metrics.ControlHeight}
		}
	case kindInputGroup:
		n.pref = [2]float32{0, f.theme.Metrics.ControlHeight}
		if n.Kind == kindInputGroup && widgetsOf(f).groups[n.Aux].Multiline {
			n.pref[axisY] = 64
		}
	case kindTextarea:
		n.pref = [2]float32{n.padX(), f.theme.Metrics.FontSizeM + 7 + n.padY()}
		if n.Flags&flagThemed != 0 {
			n.pref = [2]float32{0, 64}
		}
	case kindTable, kindTabs, kindSelect:
		f.measureWidget(n)
	case kindProgress:
		n.pref = [2]float32{progressWidth, progressHeight}
	case kindSpinner:
		n.pref = [2]float32{16, 16}
	case kindSplitter:
		f.measureSplitter(n)
	case kindSlider:
		f.measureSlider(n)
	}
}

func (f *frame) paintButton(n *node) {
	r := n.rect
	extra := &f.exts[n.Ext]
	button := components.BeginButton(f.canvas, f.theme, n.ID, Rect{X: r.X, Y: r.Y, Width: r.Width}).Size(n.ButtonSize).
		WithIcon(extra.icon).IconTransform(extra.iconTransform)
	if f.styles != nil && f.styles.Button != nil {
		button = button.Recipe(f.styles.Button)
	}
	if extra.buttonRecipe != nil {
		button = button.Recipe(extra.buttonRecipe)
	}
	// A button is as tall as its size says unless the element was given more or less
	// room: a Height, or a Grow along a column. It then fills that room.
	if h := r.Height; h > 0 && math.Abs(float64(h-button.BoundsHeight())) > 0.01 {
		button = button.Height(h)
	}
	if n.Flags&flagDisabled != 0 {
		button = button.Disabled()
	}
	switch n.variant {
	case components.ButtonSecondary:
		button.Secondary(n.label)
	case components.ButtonOutline:
		button.Outline(n.label)
	case components.ButtonGhost:
		button.Ghost(n.label)
	case components.ButtonLink:
		button.Link(n.label)
	case components.ButtonDestructive:
		button.Destructive(n.label)
	case components.ButtonDestructiveOutline:
		button.DestructiveOutline(n.label)
	default:
		button.Default(n.label)
	}
}

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
	f.nodes[b.i].Flags |= flagThemed
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
	b := f.box(kindInput)
	n := &f.nodes[b.i]
	n.Aux = int32(len(widgetsOf(f).inputs))
	n.ID = snapshot.ID
	widgetsOf(f).inputs = append(widgetsOf(f).inputs, snapshot)
	return b
}

// InputGroup adds a components.PaintInputGroup (a field with slots beside or above
// it). Give it a Height when it has block slots; without one it is a control tall.
func (f *frame) InputGroup(snapshot components.InputGroupSnapshot) Box {
	b := f.box(kindInputGroup)
	n := &f.nodes[b.i]
	n.Aux = int32(len(widgetsOf(f).groups))
	widgetsOf(f).groups = append(widgetsOf(f).groups, snapshot)
	return b
}

// Textarea adds a multi-line text field with the shadcn look, at least 64 pixels tall. It
// is TextareaBase with that look, dressed by components.TextEntryRecipe as Input is: what
// the element sets itself (Background, Border, Radius, Shadow, Pad) replaces the recipe's.
func (f *frame) Textarea(snapshot components.TextareaSnapshot) Box {
	b := f.TextareaBase(snapshot)
	f.nodes[b.i].Flags |= flagThemed
	return b
}

// TextareaBase adds a multi-line text field without a look, as InputBase is a single-line
// one: the rows, wrapping, selection, composition, caret and scrolling are the textarea's,
// and the fill, border, corners, shadow and padding are the element's. The snapshot's
// InsetTop, InsetBottom and InsetRight still reserve room inside the padded rectangle. It
// is one row tall plus its Pad and Border unless given a Height.
func (f *frame) TextareaBase(snapshot components.TextareaSnapshot) Box {
	b := f.box(kindTextarea)
	n := &f.nodes[b.i]
	n.Aux = int32(len(widgetsOf(f).textareas))
	n.ID = snapshot.ID
	widgetsOf(f).textareas = append(widgetsOf(f).textareas, snapshot)
	return b
}

// Segmented adds a components.SegmentedControl choosing among options, selected
// being the ID of the current one. The options slice is kept until End.
func (f *frame) Segmented(options []components.SegmentOption, selected gfx.ID) Box {
	b := f.box(kindSegmented)
	n := &f.nodes[b.i]
	n.Aux = int32(len(widgetsOf(f).segmented))
	widgetsOf(f).segmented = append(widgetsOf(f).segmented, segmented{options: options, selected: selected})
	return b
}

// Choice adds a segmented control choosing among labels, selected being the index of
// the current one (negative for none). Its options are identified by scope.At(0),
// scope.At(1), ...: declare a scope for it once (tree.Sub(page, tree.Name("theme"))),
// and read the click with Box.ChosenIndex.
//
//	if i, ok := f.Choice(themeScope, themeLabels, m.theme).ChosenIndex(); ok { m.theme = i }
func (f *frame) Choice(scope gfx.Scope, labels []string, selected int) Box {
	first := len(widgetsOf(f).choices)
	for index, label := range labels {
		widgetsOf(f).choices = append(widgetsOf(f).choices, components.SegmentOption{ID: scope.At(index), Label: label})
	}
	options := widgetsOf(f).choices[first:len(widgetsOf(f).choices):len(widgetsOf(f).choices)]
	chosen := gfx.ID{}
	if selected >= 0 && selected < len(labels) {
		chosen = scope.At(selected)
	}
	return f.Segmented(options, chosen)
}

func (f *frame) paintInput(n *node) {
	// What the element says of its text (FontSize, Weight, Color) is the field's text style,
	// over the snapshot's own: the one value the glyphs, the caret and the platform follow.
	// What it says of its alignment is the same, but a field is start-aligned unless it says
	// otherwise, so the snapshot's own declaration is left alone only by an alignment it
	// declares. What the snapshot leaves at AlignLeft the element's TextEnd() takes.
	snapshot := widgetsOf(f).inputs[n.Aux]
	if n.TextEnd {
		if snapshot.Alignment == gfx.AlignLeft || snapshot.Alignment == gfx.AlignStart {
			snapshot.Alignment = gfx.AlignEnd
		}
	}
	if n.FontSize > 0 {
		snapshot.Text.Size = n.FontSize
	}
	if n.Weight > 0 {
		snapshot.Text.Weight = n.Weight
	}
	if n.Flags&flagHasColor != 0 {
		snapshot.Text.Color = n.Color
	}
	if n.Flags&flagMono != 0 {
		snapshot.Text.Mono, snapshot.Text.MonoFace = true, n.Face
	}
	if a := f.exts[n.Ext].addons; a != 0 {
		f.paintAddonsFn(f, n, snapshot, &f.addons[a-1])
		return
	}
	components.PaintInputContent(f.canvas, f.theme, n.rect, f.contentRect(n), snapshot)
}

// paintTextarea paints the rows of a Textarea or TextareaBase inside the element's Pad and
// Border, as paintInput does for a line.
func (f *frame) paintTextarea(n *node) {
	if n.Flags&flagHasColor != 0 {
		theme := f.theme
		theme.Foreground = n.Color
		components.PaintTextareaContent(f.canvas, theme, n.rect, f.contentRect(n), widgetsOf(f).textareas[n.Aux])
		return
	}
	components.PaintTextareaContent(f.canvas, f.theme, n.rect, f.contentRect(n), widgetsOf(f).textareas[n.Aux])
}

// controlBaseline is the label's baseline of a button, for a row that lines
// elements up by it. The engine asks the hook; the control layer answers.
var controlBaseline = func(f *frame, n *node) float32 {
	theme := f.theme
	if n.hasOwnTheme() {
		theme = f.ownTheme(n)
	}
	button := components.BeginButton(f.canvas, theme, gfx.ID{}, Rect{}).Size(n.ButtonSize)
	return f.canvas.LabelBaseline(
		gfx.Rect{Height: n.pref[axisY]},
		gfx.TextStyle{Size: button.LabelFontSize()},
	)
}
