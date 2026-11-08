package flex

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/ui"
)

// Align places children across a container (Align) or along it (Justify).
// Auto, the zero value, leaves the choice to the default: children stretch across
// a container and start along it.
type Align uint8

const (
	Auto Align = iota
	Start
	Center
	End
	// Baseline lines the children up on their first line of text: in a Row, the
	// labels of a button, a text and a field sit on one line whatever their sizes.
	Baseline
	// Stretch makes a child as wide as the container, for the cross axis, when
	// it has no width of its own. It is the default across.
	Stretch
	// Between puts the free space between the children. It is for Justify.
	Between
)

type kind uint8

const (
	kindContainer kind = iota
	kindText
	kindSpacer
	kindButton
	kindCheckbox
	kindSwitch
	kindIcon
	kindSlot
	kindInput
	kindInputGroup
	kindTextarea
	kindSegmented
	kindTable
	kindTabs
	kindSelect
	kindProgress
	kindSpinner
	kindSplitter
	kindSlider
	kindImage
)

type direction uint8

const (
	column direction = iota
	row
	stack
	grid
)

type dimKind uint8

const (
	dimAuto dimKind = iota
	dimPx
	dimFill
	dimPercent // value is a percentage of the container's content box
)

type dimension struct {
	kind  dimKind
	value float32
}

const (
	flagClip uint32 = 1 << iota
	flagMono
	flagTextCenter
	flagDisabled
	flagChecked
	flagSmall
	flagGrowSet
	flagShrinkSet
	flagHasColor
	flagHasHit
	flagWrap
	flagScroll
	flagBasis
	flagOutline
	flagDialog    // a Dialog's root: painted by components.Dialog around its children
	flagChildren  // Children was called on the element, even if it declared nothing
	flagHasRadius // Radius was called: on a control, its corners are its own, not the theme's
	flagThemed    // a control with the theme's look: what the caller did not set, the recipe does
	flagSetBackground
	flagSetBorder
	flagSetShadow
	flagSetPad
	flagSelected // the element is the chosen one of several (State.Selected)
	flagOpen     // the element shows what it controls (State.Open)
	flagVeto     // a click on the element is not delivered: its identity moved to it under a held press or focus
	flagPopover  // a Popover's root: painted with components.Popover's dismiss, focus scope and Escape around its children
)

// node is one element of the page. Fields are plain values so a node is reused
// without the garbage collector seeing it. Anything with two numbers, one for
// each axis, is indexed by axisX or axisY, so the layout is written once.
type node struct {
	kind  kind
	dir   direction
	flags uint32

	parent, first, last, next int32

	size                 [2]dimension
	minS, maxS           [2]float32 // a max of zero is no max
	grow, shrink         float32
	pad, margin          [4]float32 // top, right, bottom, left
	gap                  float32
	items, self, justify Align
	colSpan              uint8 // the columns a Grid child covers; zero is one
	background           ui.Color
	borderWidth, radius  float32
	gridCol, gridRow     uint16 // where a Grid placed the element
	text                 string
	fontSize, weight     float32
	lineH                float32 // 0 is the size plus seven
	face                 ui.MonoFace
	textDirection        ui.TextDirection // the paragraph direction of a Text; zero takes it from the text
	textEnd              bool             // a Text aligned to the end of its direction
	color                ui.Color
	id                   ui.ID
	label                string
	variant              components.ButtonVariant
	buttonSize           components.ButtonSize
	rowSpan              uint8 // the rows a Grid child covers; zero is one
	outOfFlow            uint8 // absolute positioning: see absolute.go

	maxLines             int32     // a wrapped Text breaks into at most this many lines
	lineFirst, lineCount int32     // its lines, in Frame.lines
	scroll               float32   // how far a Scroll is scrolled
	content              float32   // the natural height of a Scroll's children
	gridMin              float32   // the narrowest cell of a Grid
	basis                float32   // Basis, when flagBasis is set
	aux                  int32     // index into the Frame\'s table for an Input, InputGroup, Textarea or Segmented
	natural              float32   // the width of a Text on one line, before any size is applied; a Progress's fraction (negative: indeterminate)
	ext                  int32     // index into Frame.exts, or 0 for none (exts[0] is a zero value)
	place                placement // a Popup\'s position
	anchor               ui.Rect
	placeGap             float32
	anchorRef            int32 // 1 + the index of the element a Popup hangs off, or 0 when it hangs off anchor
	placeAlign           Align
	baseline             float32 // distance from the top to the first line of text

	pref                [2]float32
	raw                 [2]float32 // pref before Min/Max, the size growth starts from
	growNow, shrinkNow  float32
	base, want, bounded float32 // while flexible lengths are resolved
	frozen              bool
	rect                ui.Rect
}

// placement says where a Popup goes.
type placement uint8

const (
	placeNone       placement = iota
	placeAt                   // its top left corner at the anchor's
	placeBelow                // under the anchor, flipping above it when there is no room
	placeAbove                // over the anchor, flipping below it when there is no room
	placeBeside               // to the right of the anchor, flipping to its left when there is no room
	placeBesideLeft           // to the left of the anchor, flipping to its right when there is no room
)

const (
	axisX = 0
	axisY = 1
)

// extra holds what few elements need, so that the common element stays small: the
// hit options of an interactive container, an icon, the border's color, the
// shadow's level, the ring, and the indexes of its gradient and state looks.
type extra struct {
	hit           ui.HitOptions
	icon          ui.Icon
	iconTransform ui.IconTransform // moves the icon inside its box without moving the box
	border        ui.Color
	shadow        ui.ShadowLevel
	ring          ui.Color        // Ring: a line around the outside of the element
	ringWidth     float32         // its width; zero is none
	fade          float32         // 1 less the opacity, so that the zero value is opaque
	gradient      int32           // 1 + the index of the element's gradient in Frame.gradients, or 0 for none
	states        int32           // 1 + the index of the element's Hover, Pressed and Focus looks in Frame.states, or 0 for none
	scrollState   *ScrollState    // the state a ScrollArea reports its extent to
	maxPct        [2]float32      // MaxWidthPercent and MaxHeightPercent; zero is none
	hitOut        [4]float32      // how far the hit region reaches beyond the rectangle: top, right, bottom, left
	edges         [4]float32      // an absolute element's distance from its container's top, right, bottom, left
	within        ui.Rect         // what a Popup is kept inside, when not the frame; empty for the frame
	moves         bool            // the element's rectangle moves a custom-titlebar window (MovesWindow)
	nativeView    ui.NativeViewID // the native view that fills the element (NativeView)
	bind          int32           // 1 + the index of the element's binding in Frame.binds, or 0 (see bind.go)
	draws         int32           // 1 + the index of the element's Draw and DrawOver callbacks in Frame.draws, or 0 for none
}

// Frame is one page being declared. It is handed to the function given to
// View.Draw; its methods add elements to the container being filled.
type Frame struct {
	canvas      *ui.Canvas
	theme       ui.Theme
	nodes       []node
	inputs      []components.InputSnapshot
	groups      []components.InputGroupSnapshot
	textareas   []components.TextareaSnapshot
	segmented   []segmented
	choices     []components.SegmentOption // the options Choice builds, shared by the frame
	images      []ui.Bitmap                // what each Image element draws, indexed by its aux
	exts        []extra                    // exts[0] is never written: an element without extras points at it
	floats      []int32                    // the Popups, which belong to no container
	states      []stateLooks               // what Hover, Pressed and Focus declared, by element
	draws       []drawHooks                // what Draw and DrawOver declared, by element
	drawContext DrawContext                // what a Draw callback is handed, reused for the frame
	gradients   []gradient                 // the gradients of the elements that have one
	stops       []ui.GradientStop          // their stops, in the order declared
	paintStops  []ui.GradientStop          // the stops of the gradient being painted, faded
	opacity     float32                    // the product of the opacities of the elements being painted
	popupLayer  uint16                     // the canvas layer the Popups are painted on
	nativeMark  int                        // how many native views the canvas had when the page began
	binds       []bindDecl                 // the bound controls, see bind.go
	// The paint of a Popover, the settling of bound values and the paint of a Caption
	// are reached through these, set by the call that declares one, so an application
	// that declares none does not link them.
	paintPopoverFn func(f *Frame, i int32, n *node)
	settleFn       func(f *Frame, i int32) *bindDecl // i < 0 settles them all
	captionFn      func(f *Frame, n *node, control ui.Rect)
	lines          []string
	widgets        widgets            // Table, Tabs, Select, Dialog and Splitter declarations
	ellipses       []ui.EllipsisCache // the cut-off ends of wrapped texts, kept between frames
	wraps          []wrapCache        // the full line sets of wrapped texts, kept between frames
	wrapCursor     int
	scratch        scratch     // memory the layout borrows, see scratch.go
	scrolls        scrollStore // the ScrollStates of the page's ScrollAreas, by identity
	parent         int32
	bounds         ui.Rect
	done           bool
}

// View keeps the memory a page is laid out in between frames. The zero value is
// ready to use; keep one for each page that is painted every frame.
type View struct{ frame Frame }

// Draw declares, lays out and paints one page inside bounds. build runs once.
func (view *View) Draw(canvas *ui.Canvas, theme ui.Theme, bounds ui.Rect, build func(*Frame)) {
	f := view.Begin(canvas, theme, bounds)
	build(f)
	f.End()
}

// Begin starts a page for callers that need the rectangles it lays out (Box.Rect)
// before painting something of their own; End finishes it. Draw is Begin, build
// and End.
func (view *View) Begin(canvas *ui.Canvas, theme ui.Theme, bounds ui.Rect) *Frame {
	f := &view.frame
	f.canvas, f.theme, f.bounds, f.done = canvas, theme, bounds, false
	f.nodes = f.nodes[:0]
	f.lines = f.lines[:0]
	f.exts = append(f.exts[:0], extra{})
	f.inputs, f.groups, f.textareas, f.segmented, f.choices = f.inputs[:0], f.groups[:0], f.textareas[:0], f.segmented[:0], f.choices[:0]
	f.images = f.images[:0]
	clear(f.binds) // a binding holds pointers into the model: not past the frame
	f.binds = f.binds[:0]
	f.floats = f.floats[:0]
	f.states, f.gradients, f.stops = f.states[:0], f.gradients[:0], f.stops[:0]
	clear(f.draws) // a callback holds what it captured: not past the frame it was declared in
	f.draws = f.draws[:0]
	f.opacity = 1
	f.popupLayer = 0
	f.nativeMark = 0
	if canvas != nil {
		f.nativeMark = len(canvas.NativeViews())
	}
	f.scratch.reset()
	f.widgets.reset()
	f.scrolls.epoch++
	f.parent = -1
	root := f.add(kindContainer)
	n := &f.nodes[root]
	n.dir = column
	n.size = [2]dimension{{dimPx, bounds.Width}, {dimPx, bounds.Height}}
	f.parent = root
	return f
}

// End lays the page out and paints it. Rects are valid afterwards, until the
// next Begin.
func (f *Frame) End() {
	if f.done {
		return
	}
	if f.settleFn != nil && len(f.binds) > 0 {
		f.settleFn(f, -1)
	}
	f.done = true
	f.checkHits()
	f.applyRecipes()
	root := &f.nodes[0]
	f.measure(axisX)
	root.rect.X, root.rect.Width = f.bounds.X, f.bounds.Width
	f.solve(0, axisX)
	for _, index := range f.floats {
		f.placePopup(&f.nodes[index], axisX)
		f.solve(index, axisX)
	}
	f.measure(axisY)
	root.rect.Y, root.rect.Height = f.bounds.Y, f.bounds.Height
	f.solve(0, axisY)
	for _, index := range f.floats {
		f.placePopup(&f.nodes[index], axisY)
		f.solve(index, axisY)
	}
	f.reportScrolls()
	f.scrolls.sweep()
	f.reportSplitters()
	f.paint(0)
	f.raiseOverNativeViews()
	if f.popupLayer != 0 {
		f.canvas.PushLayer(f.popupLayer)
	}
	for _, index := range f.floats {
		f.paint(index)
	}
	if f.popupLayer != 0 {
		f.canvas.PopLayer()
	}
	f.trim()
}

// trim releases the backing arrays of a large page after it has been replaced
// by a smaller one. It keeps every element, so the Box results of this frame
// (Rect, Clicked, TabClicked and so on) stay valid.
func (f *Frame) trim() {
	// Cache entries past the wrapped texts this frame declared belong to a
	// larger page that is gone. Drop them before trimming the outer slices so
	// their strings do not stay reachable.
	if f.wrapCursor < len(f.wraps) {
		f.wraps = f.wraps[:f.wrapCursor]
	}
	if f.wrapCursor < len(f.ellipses) {
		f.ellipses = f.ellipses[:f.wrapCursor]
	}
	f.nodes = trimFlexCapacity(f.nodes)
	f.inputs = trimFlexCapacity(f.inputs)
	f.groups = trimFlexCapacity(f.groups)
	f.textareas = trimFlexCapacity(f.textareas)
	f.segmented = trimFlexCapacity(f.segmented)
	f.choices = trimFlexCapacity(f.choices)
	f.images = trimFlexCapacity(f.images)
	f.exts = trimFlexCapacity(f.exts)
	f.draws = trimFlexCapacity(f.draws)
	f.binds = trimFlexCapacity(f.binds)
	f.floats = trimFlexCapacity(f.floats)
	f.lines = trimFlexCapacity(f.lines)
	f.ellipses = trimFlexCapacity(f.ellipses)
	f.wraps = trimFlexCapacity(f.wraps)
	f.widgets.tables = trimFlexCapacity(f.widgets.tables)
	f.widgets.tabs = trimFlexCapacity(f.widgets.tabs)
	f.widgets.selects = trimFlexCapacity(f.widgets.selects)
	f.widgets.rows = trimFlexCapacity(f.widgets.rows)
	f.widgets.dialogs = trimFlexCapacity(f.widgets.dialogs)
	f.widgets.popovers = trimFlexCapacity(f.widgets.popovers)
	f.widgets.splits = trimFlexCapacity(f.widgets.splits)
	f.widgets.sliders = trimFlexCapacity(f.widgets.sliders)
	f.widgets.cells = trimFlexCapacity(f.widgets.cells)
	f.widgets.rects = trimFlexCapacity(f.widgets.rects)
	f.scratch.abs = trimFlexCapacity(f.scratch.abs)
	f.scratch.rows = trimFlexCapacity(f.scratch.rows)
	f.scratch.rowAt = trimFlexCapacity(f.scratch.rowAt)
	f.scratch.cells = trimFlexCapacity(f.scratch.cells)
}

// Canvas and Theme give the declaration what it was started with.
func (f *Frame) Canvas() *ui.Canvas { return f.canvas }
func (f *Frame) Theme() ui.Theme    { return f.theme }

// Bounds is the rectangle this frame lays out in, as the caller gave it to Draw
// or Begin. Custom painting (a tooltip, a highlight) uses it to stay inside the
// region it belongs to: the frame's bounds are known before End, unlike a
// child's rectangle.
func (f *Frame) Bounds() ui.Rect { return f.bounds }

func (f *Frame) add(k kind) int32 {
	if f.done {
		panic("flex: an element was declared after the page was laid out (in a Draw callback, or after End): declare elements in the build function")
	}
	index := int32(len(f.nodes))
	f.nodes = append(f.nodes, node{kind: k, parent: f.parent, first: -1, last: -1, next: -1, items: Auto})
	if f.parent >= 0 {
		parent := &f.nodes[f.parent]
		if parent.last < 0 {
			parent.first = index
		} else {
			f.nodes[parent.last].next = index
		}
		f.nodes[f.parent].last = index
	}
	return index
}

func (f *Frame) box(k kind) Box { return Box{f: f, i: f.add(k)} }

// Row opens a container that lays its children out left to right.
func (f *Frame) Row() Box {
	b := f.box(kindContainer)
	f.nodes[b.i].dir = row
	return b
}

// Column opens a container that lays its children out top to bottom.
func (f *Frame) Column() Box { return f.box(kindContainer) }

// Stack opens a container that lays its children over one another, each in the
// whole content area unless it has a size of its own.
func (f *Frame) Stack() Box {
	b := f.box(kindContainer)
	f.nodes[b.i].dir = stack
	return b
}

// Popup opens a container that belongs to no other: it takes no room in the one it
// was declared in, is placed by At, Below or Above (give it one of them), is as big
// as its content unless it has a size, is kept inside the frame's bounds, and is
// painted last, over everything. It is for menus, tooltips and select lists that hang
// off a rectangle or a pointer position; children are declared with Children as in any
// container, and the popup's own layer is the caller's business (wrap End in
// canvas.PushLayer if it must sit above components painted later).
func (f *Frame) Popup() Box {
	saved := f.parent
	f.parent = -1
	b := f.box(kindContainer)
	f.parent = saved
	f.floats = append(f.floats, b.i)
	return b
}

// PopupLayer paints the page's popups on a canvas layer of their own instead of the page's. A
// native view (Canvas.NativeView) hides what is painted on its layer and below, so popups that
// must stand over one go on a higher layer than the view's.
func (f *Frame) PopupLayer(layer uint16) { f.popupLayer = layer }

// Grid opens a container that lays its children in equal cells, as many columns
// as fit when each is at least minCell wide, wrapping onto new rows; Gap is the
// space between cells both ways. A row is as tall as its tallest cell.
func (f *Frame) Grid(minCell float32) Box {
	b := f.box(kindContainer)
	n := &f.nodes[b.i]
	n.dir = grid
	n.gridMin = max(minCell, 1)
	return b
}

// Scroll opens a column whose children may be taller than it is: it clips them
// and shows them offset pixels up. The caller owns the offset (wheel events and
// scrollbars are the application's, as with components.ScrollArea); after End,
// Box.Content says how tall the children are, so the offset can be clamped to
// Content - Rect().Height. Give it a size, Fill or Grow, or it is as tall as its
// children.
func (f *Frame) Scroll(offset float32) Box {
	b := f.box(kindContainer)
	n := &f.nodes[b.i]
	n.flags |= flagScroll | flagClip
	n.scroll = max(offset, 0)
	return b
}

// Paragraph adds text that breaks into lines to fit the width it is given, at most
// maxLines of them (the last ends with an ellipsis), so its height follows its
// width. It is what Text is not: a Text is one line and is clipped.
func (f *Frame) Paragraph(value string, maxLines int) Box {
	b := f.Text(value)
	n := &f.nodes[b.i]
	n.flags |= flagWrap
	n.maxLines = int32(max(maxLines, 1))
	return b
}

// Text adds one line of text, in the theme's body size and foreground color.
func (f *Frame) Text(value string) Box {
	b := f.box(kindText)
	n := &f.nodes[b.i]
	n.text = value
	n.fontSize = f.theme.Metrics.FontSizeM
	n.color = f.theme.Foreground
	n.shrink = 1
	return b
}

// Spacer adds empty space that grows to take what the other children leave.
func (f *Frame) Spacer() Box {
	b := f.box(kindSpacer)
	f.nodes[b.i].grow = 1
	f.nodes[b.i].flags |= flagGrowSet
	return b
}

// Divider adds a one pixel line in the theme's border color across the
// container: horizontal in a column, vertical in a row.
func (f *Frame) Divider() Box {
	b := f.box(kindContainer)
	n := &f.nodes[b.i]
	n.background = f.theme.Border
	if f.parent >= 0 && f.nodes[f.parent].dir == row {
		n.size[axisX] = dimension{dimPx, 1}
	} else {
		n.size[axisY] = dimension{dimPx, 1}
	}
	return b
}

// Icon adds an icon, 16 pixels square unless given a size, in the foreground
// color unless given a Color.
func (f *Frame) Icon(icon ui.Icon) Box {
	b := f.box(kindIcon)
	n := &f.nodes[b.i]
	f.extra(b.i).icon = icon
	n.color = f.theme.Foreground
	return b
}

// Image adds a bitmap, 16 pixels square unless given a size, scaled to fill the
// element's rectangle. It is the way to show a picture the application draws or
// asks the system for, such as app.FileIcon, among text and icons; the bitmap
// is not copied, so its pixels must stay unchanged while the page is on screen
// (see ui.Bitmap). An empty or invalid bitmap leaves the room blank.
func (f *Frame) Image(bitmap ui.Bitmap) Box {
	b := f.box(kindImage)
	f.nodes[b.i].aux = int32(len(f.images))
	f.images = append(f.images, bitmap)
	return b
}

// Button adds a button with the official look; Outline, Ghost and the other
// variants choose another. Clicked reports a click on it.
func (f *Frame) Button(id ui.ID, label string) Box {
	b := f.box(kindButton)
	n := &f.nodes[b.i]
	n.id, n.label = id, label
	n.flags |= flagHasHit
	return b
}

// Checkbox adds a checkbox in the given state; Clicked reports a click on it.
func (f *Frame) Checkbox(id ui.ID, checked bool) Box {
	b := f.box(kindCheckbox)
	n := &f.nodes[b.i]
	n.id = id
	if checked {
		n.flags |= flagChecked
	}
	n.flags |= flagHasHit
	return b
}

// Switch adds a two-state switch in the given state; Clicked reports a click.
func (f *Frame) Switch(id ui.ID, on bool) Box {
	b := f.box(kindSwitch)
	n := &f.nodes[b.i]
	n.id = id
	if on {
		n.flags |= flagChecked
	}
	n.flags |= flagHasHit
	return b
}

// Slot reserves room for something the caller paints itself: Rect gives where
// once End has laid the page out. Give it a size, or let it grow.
func (f *Frame) Slot() Box { return f.box(kindSlot) }

// extra returns the extras of element i, giving it a set of its own if it has none.
func (f *Frame) extra(i int32) *extra {
	n := &f.nodes[i]
	if n.ext == 0 {
		n.ext = int32(len(f.exts))
		f.exts = append(f.exts, extra{})
	}
	return &f.exts[n.ext]
}

// segmented is what a Segmented control is painted from.
type segmented struct {
	options  []components.SegmentOption
	selected ui.ID
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
func (f *Frame) Input(snapshot components.InputSnapshot) Box {
	b := f.InputBase(snapshot)
	f.nodes[b.i].flags |= flagThemed
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
func (f *Frame) InputBase(snapshot components.InputSnapshot) Box {
	b := f.box(kindInput)
	n := &f.nodes[b.i]
	n.aux = int32(len(f.inputs))
	n.id = snapshot.ID
	f.inputs = append(f.inputs, snapshot)
	return b
}

// InputGroup adds a components.PaintInputGroup (a field with slots beside or above
// it). Give it a Height when it has block slots; without one it is a control tall.
func (f *Frame) InputGroup(snapshot components.InputGroupSnapshot) Box {
	b := f.box(kindInputGroup)
	n := &f.nodes[b.i]
	n.aux = int32(len(f.groups))
	f.groups = append(f.groups, snapshot)
	return b
}

// Textarea adds a multi-line text field with the shadcn look, at least 64 pixels tall. It
// is TextareaBase with that look, dressed by components.TextEntryRecipe as Input is: what
// the element sets itself (Background, Border, Radius, Shadow, Pad) replaces the recipe's.
func (f *Frame) Textarea(snapshot components.TextareaSnapshot) Box {
	b := f.TextareaBase(snapshot)
	f.nodes[b.i].flags |= flagThemed
	return b
}

// TextareaBase adds a multi-line text field without a look, as InputBase is a single-line
// one: the rows, wrapping, selection, composition, caret and scrolling are the textarea's,
// and the fill, border, corners, shadow and padding are the element's. The snapshot's
// InsetTop, InsetBottom and InsetRight still reserve room inside the padded rectangle. It
// is one row tall plus its Pad and Border unless given a Height.
func (f *Frame) TextareaBase(snapshot components.TextareaSnapshot) Box {
	b := f.box(kindTextarea)
	n := &f.nodes[b.i]
	n.aux = int32(len(f.textareas))
	n.id = snapshot.ID
	f.textareas = append(f.textareas, snapshot)
	return b
}

// Segmented adds a components.SegmentedControl choosing among options, selected
// being the ID of the current one. The options slice is kept until End.
func (f *Frame) Segmented(options []components.SegmentOption, selected ui.ID) Box {
	b := f.box(kindSegmented)
	n := &f.nodes[b.i]
	n.aux = int32(len(f.segmented))
	f.segmented = append(f.segmented, segmented{options: options, selected: selected})
	return b
}

// Choice adds a segmented control choosing among labels, selected being the index of
// the current one (negative for none). Its options are identified by scope.At(0),
// scope.At(1), ...: declare a scope for it once (tree.Sub(page, tree.Name("theme"))),
// and read the click with Box.ChosenIndex.
//
//	if i, ok := f.Choice(themeScope, themeLabels, m.theme).ChosenIndex(); ok { m.theme = i }
func (f *Frame) Choice(scope ui.Scope, labels []string, selected int) Box {
	first := len(f.choices)
	for index, label := range labels {
		f.choices = append(f.choices, components.SegmentOption{ID: scope.At(index), Label: label})
	}
	options := f.choices[first:len(f.choices):len(f.choices)]
	chosen := ui.ID{}
	if selected >= 0 && selected < len(labels) {
		chosen = scope.At(selected)
	}
	return f.Segmented(options, chosen)
}

// checkHits stops a page that declared the one shape that cannot be what was meant: a
// container given Hit, never given Children, and with nothing else that makes it
// show or take room (no size, background, border or place of its own). Such a row is
// as tall as its padding, paints nothing and is not the clickable row of text its
// author had in mind: Hit names a region for the pointer and the accessibility tree,
// it does not draw its label. Declare the content inside Children before Clicked, or
// give an empty target a size, a background or Absolute.
func (f *Frame) checkHits() {
	for i := range f.nodes {
		n := &f.nodes[i]
		if n.kind != kindContainer || n.flags&flagHasHit == 0 || n.flags&flagChildren != 0 || n.first >= 0 {
			continue
		}
		if n.size[axisX].kind != dimAuto || n.size[axisY].kind != dimAuto || n.background.Alpha > 0 || n.borderWidth > 0 || n.outOfFlow != 0 || n.minS != [2]float32{} {
			continue
		}
		panic("flex: a Row, Column or Stack given Hit(" + n.label + ") has no Children and no size, background or border, so it is empty: " +
			"declare what it shows with Children(func() { ... }) before Clicked(), e.g. f.Row().Hit(id, name).Children(func() { f.Text(name) }).Clicked()")
	}
}
