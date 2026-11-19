package base

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/style"
)

const (
	kindContainer   = KindContainer
	kindText        = KindText
	kindSpacer      = KindSpacer
	kindButton      = KindButton
	kindCheckbox    = KindCheckbox
	kindSwitch      = KindSwitch
	kindIcon        = KindIcon
	kindSlot        = KindSlot
	kindInput       = KindInput
	kindInputGroup  = KindInputGroup
	kindTextarea    = KindTextarea
	kindSegmented   = KindSegmented
	kindTable       = KindTable
	kindTabs        = KindTabs
	kindSelect      = KindSelect
	kindProgress    = KindProgress
	kindSpinner     = KindSpinner
	kindSplitter    = KindSplitter
	kindSlider      = KindSlider
	kindImage       = KindImage
	kindSplitButton = KindSplitButton
	kindBadge       = KindBadge
	kindToggle      = KindToggle
	kindPagination  = KindPagination
)

type direction = Direction

const (
	column   = DirectionColumn
	row      = DirectionRow
	StackDir = DirectionStack
	grid     = DirectionGrid
	custom   = DirectionCustom
	flow     = DirectionFlow
)

type dimKind = DimKind

const (
	dimAuto    = DimAuto
	dimPx      = DimPx
	dimFill    = DimFill
	dimPercent = DimPercent
)

type Dimension struct {
	Kind  dimKind
	Value float32
}

const (
	flagClip          = FlagClip
	flagMono          = FlagMono
	flagTextCenter    = FlagTextCenter
	flagDisabled      = FlagDisabled
	flagChecked       = FlagChecked
	flagSmall         = FlagSmall
	flagGrowSet       = FlagGrowSet
	flagShrinkSet     = FlagShrinkSet
	flagHasColor      = FlagHasColor
	flagHasHit        = FlagHasHit
	flagWrap          = FlagWrap
	flagScroll        = FlagScroll
	flagBasis         = FlagBasis
	flagOutline       = FlagOutline
	flagDialog        = FlagDialog
	flagAlertDialog   = FlagAlertDialog
	flagChildren      = FlagChildren
	flagHasRadius     = FlagHasRadius
	flagThemed        = FlagThemed
	flagSetBackground = FlagSetBackground
	flagSetBorder     = FlagSetBorder
	flagSetShadow     = FlagSetShadow
	flagSetPad        = FlagSetPad
	flagSelected      = FlagSelected
	flagOpen          = FlagOpen
	flagVeto          = FlagVeto
	flagPopover       = FlagPopover
	flagDrawer        = FlagDrawer
	flagLabelFor      = FlagLabelFor
	flagMixed         = FlagMixed
)

// node is one element of the page. Fields are plain values so a node is reused
// without the garbage collector seeing it. Anything with two numbers, one for
// each axis, is indexed by axisX or axisY, so the layout is written once.
type Node struct {
	Kind  Kind
	Dir   direction
	Flags uint32

	Parent, First, Last, Next int32

	Size                 [2]Dimension
	MinS, MaxS           [2]float32 // a max of zero is no max
	Grow, Shrink         float32
	Pad, Margin          [4]float32 // top, right, bottom, left
	Gap                  float32
	Items, Self, Justify Align
	ColSpan              uint8 // the columns a Grid child covers; zero is one
	Background           Color
	BorderWidth, Radius  float32
	GridCol, GridRow     uint16 // where a Grid placed the element; gridRow is also the line a Flow put it on
	Text                 string
	FontSize, Weight     float32
	LineH                float32 // 0 is the size plus seven
	Face                 gfx.MonoFace
	TextDirection        gfx.TextDirection // the paragraph direction of a Text; zero takes it from the text
	TextEnd              bool              // a Text aligned to the end of its direction
	Color                Color
	ID                   gfx.ID
	Label                string
	Variant              style.Variant
	ButtonSize           style.Size
	RowSpan              uint8 // the rows a Grid child covers; zero is one
	OutOfFlow            uint8 // absolute positioning: see absolute.go

	MaxLines             int32     // a wrapped Text breaks into at most this many lines
	LineFirst, LineCount int32     // its lines, in frame.lines
	Scroll               float32   // how far a Scroll is scrolled
	Content              float32   // the natural height of a Scroll's children
	GridMin              float32   // the narrowest cell of a Grid; a Flow's LineGap, negative for Gap
	Basis                float32   // Basis, when flagBasis is set
	Aux                  int32     // index into the frame\'s table for an Input, InputGroup, Textarea or Segmented
	Natural              float32   // the width of a Text on one line, before any size is applied; a Progress's fraction (negative: indeterminate)
	Ext                  int32     // index into frame.exts, or 0 for none (exts[0] is a zero value)
	Place                placement // a Popup\'s position
	Anchor               Rect
	PlaceGap             float32
	AnchorRef            int32 // 1 + the index of the element a Popup hangs off, or 0 when it hangs off anchor
	PlaceAlign           Align
	PlaceShift           float32 // moves a popup along its anchor from where Align put it
	Baseline             float32 // distance from the top to the first line of text

	Pref                [2]float32
	Raw                 [2]float32 // pref before Min/Max, the size growth starts from
	GrowNow, ShrinkNow  float32
	Base, Want, Bounded float32 // while flexible lengths are resolved
	Frozen              bool
	Rect                Rect
	Band                uint16 // the compositing band the subtree paints on (Overlay); zero is the page's
	FocusScope          gfx.ID // the focus scope the subtree paints in, when it is not the page's
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
	AxisX = 0
	AxisY = 1
)

// extra holds what few elements need, so that the common element stays small: the
// hit options of an interactive container, an icon, the border's color, the
// shadow's level, the ring, and the indexes of its gradient and state looks.
type Extra struct {
	hit           gfx.HitOptions
	icon          gfx.Icon
	iconTransform gfx.IconTransform                                          // moves the icon inside its box without moving the box
	buttonRecipe  func(gfx.Theme, style.Variant, gfx.State) style.ButtonLook // ButtonRecipe: this button's own look
	switchRecipe  func(gfx.Theme, gfx.State, float32) style.SwitchLook       // SwitchRecipe: this switch's own look
	border        Color
	shadow        gfx.ShadowLevel
	ring          Color   // Ring: a line around the outside of the element
	ringWidth     float32 // its width; zero is none
	borderDashed  bool
	fade          float32          // 1 less the opacity, so that the zero value is opaque
	gradient      int32            // 1 + the index of the element's gradient in frame.gradients, or 0 for none
	States        int32            // 1 + the index of the element's Hover, Pressed and Focus looks in frame.states, or 0 for none
	scrollState   *ScrollState     // the state a ScrollArea reports its extent to
	maxPct        [2]float32       // MaxWidthPercent and MaxHeightPercent; zero is none
	hitOut        [4]float32       // how far the hit region reaches beyond the rectangle: top, right, bottom, left
	hitExact      bool             // a styled control targets its laid-out rectangle, not its enlarged desktop target
	edges         [4]float32       // an absolute element's distance from its container's top, right, bottom, left
	within        Rect             // what a Popup is kept inside, when not the frame; empty for the frame
	moves         bool             // the element's rectangle moves a custom-titlebar window (MovesWindow)
	scrollManual  bool             // ScrollAt: the application owns the offset and wheel input
	nativeView    gfx.NativeViewID // the native view that fills the element (NativeView)
	bind          int32            // 1 + the index of the element's binding in frame.binds, or 0 (see bind.go)
	memo          int32            // 1 + the index the element's cache has with whoever declared it (memoFn), or 0
	Draws         int32            // 1 + the index of the element's Draw and DrawOver callbacks in frame.draws, or 0 for none
	tooltip       string           // what Tooltip says over the element; empty for none
	tooltipSide   uint8            // 1 + the TooltipSide TooltipOn opens the element's tip on; 0 is below
	addons        int32            // 1 + the index of the field's icon and button in frame.addons, or 0
	focusGroup    FocusAxis        // FocusGroup: the arrow keys that move the focus between the controls inside; zero is none
	focusOrder    FocusOrder       // FocusOrder: how the group chooses the next member; nil is its axis
	layout        Layout           // CustomLayout: who measures and places the children
	swatchFlush   bool             // a ColorSwatch fills its border with its color
	swatchSkipTab bool             // a ColorSwatch takes no Tab stop (TabStop)
	swatch        bool             // the element is a ColorSwatch, painted by paintSwatch
	swatchValue   Color            // its color and whether it is the chosen one of its set
	swatchChosen  bool
	picker        *ColorPickerState // a ColorPicker's state, painted by paintColorPicker
	pickerLook    ColorPickerLook   // how it looks
}

// frame is one page being declared. It is handed to the function given to
// view.Draw; its methods add elements to the container being filled.
//
// frame is the engine under Page: a page declares with C and Box, and the page
// gives the frame the declarations. Tests and previews that need the engine
// paint through a View; applications do not reach a frame.
type Frame struct {
	Surface     *gfx.Canvas
	Palette     Theme
	styles      *Styles // the page's recipe overrides; nil keeps the theme's
	Nodes       []Node
	addons      []FieldAddons      // the icons and buttons inside fields, by 1 + extra.addons
	Images      []gfx.Bitmap       // what each Image element draws, indexed by its aux
	Exts        []Extra            // exts[0] is never written: an element without extras points at it
	Floats      []int32            // the Popups, which belong to no container
	States      []stateLooks       // what Hover, Pressed and Focus declared, by element
	Draws       []drawHooks        // what Draw and DrawOver declared, by element
	memoDepth   int                // how many Memos of a page are recording what is declared now
	page        *Page              // the page declaring this frame; nil for a frame of the application's
	tooltips    bool               // an element was given a Tooltip: End paints the one the pointer rests on
	tooltip     any                // the tooltip state the layer above owns: see tooltipOf
	drawContext DrawContext        // what a Draw callback is handed, reused for the frame
	pickerCtx   Ctx                // the declaration context a ColorPicker's painter is handed, reused for the frame
	Gradients   []gradient         // the gradients of the elements that have one
	Stops       []gfx.GradientStop // their stops, in the order declared
	paintStops  []gfx.GradientStop // the stops of the gradient being painted, faded
	Opacity     float32            // the product of the opacities of the elements being painted
	popupLayer  uint16             // the canvas layer the Popups are painted on
	nativeMark  int                // how many native views the canvas had when the page began
	Binds       []bindDecl         // the bound controls, see bind.go
	// The paint of a Popover, the settling of bound values and the paint of a Caption
	// are reached through these, set by the call that declares one, so an application
	// that declares none does not link them.
	paintPopoverFn func(f *Frame, i int32, n *Node)
	groupFn        func(f *Frame, i int32, axis FocusAxis, from gfx.Mark)          // set by the first FocusGroup, like paintDrawerFn
	paintDrawerFn  func(f *Frame, n *Node)                                         // set by the first Drawer, so a frame without one does not link it
	PaintAddonsFn  func(f *Frame, n *Node, snapshot InputSnapshot, a *FieldAddons) // set by the first field Icon or Action
	paintTooltipFn func(f *Frame)                                                  // set by the first Tooltip, so a frame without one does not link it
	paintBarsFn    func(f *Frame, n *Node, s *ScrollState)                         // set by the first Scrollbars, so a frame without one does not link them
	paintSwatchFn  func(f *Frame, n *Node, r Rect)                                 // set by the first ColorSwatch, so one closure is not made for each paint
	paintPickerFn  func(f *Frame, n *Node)                                         // set by the first ColorPicker, for the same reason
	settleFn       func(f *Frame, i int32) *bindDecl                               // i < 0 settles them all
	captionFn      func(f *Frame, n *Node, control Rect)
	// memoFn paints, or begins (before) and ends the paint of, an element a cache
	// declared; it reports whether the element is painted and its paint is done.
	memoFn     func(f *Frame, i int32, before bool) bool
	Lines      []string
	controls   any                 // the layer above's control declarations: see widgetsOf
	Ellipses   []gfx.EllipsisCache // the cut-off ends of wrapped texts, kept between frames
	Wraps      []wrapCache         // the full line sets of wrapped texts, kept between frames
	wrapCursor int
	scratch    scratch       // memory the layout borrows, see scratch.go
	layoutCtx  LayoutContext // the context a CustomLayout is handed, reused
	Scrolls    scrollStore   // the ScrollStates of the page's ScrollAreas, by identity
	Parent     int32
	bounds     Rect
	gen        uint32 // the paint this frame is declaring; a Box names it
	shiftedX   bool   // a CustomLayout moved children after their width was solved
	done       bool
}

// view keeps the memory a page is laid out in between frames. It is the engine
// under Page; tests and previews that need the engine paint through a View. An
// application uses a Page, not a view.
type View struct{ Frame *Frame }

// Draw declares, lays out and paints one page inside bounds. build runs once.
func (View *View) Draw(Surface *gfx.Canvas, Palette Theme, bounds Rect, build func(*Frame)) {
	f := View.Begin(Surface, Palette, bounds)
	build(f)
	f.End()
}

// Begin starts a page for callers that need the rectangles it lays out (Box.Rect)
// before painting something of their own; End finishes it. Draw is Begin, build
// and End.
func (View *View) Begin(Surface *gfx.Canvas, Palette Theme, bounds Rect) *Frame {
	if View.Frame == nil {
		View.Frame = new(Frame)
	}
	f := View.Frame
	if f.gen == ^uint32(0) {
		// A generation is about to wrap: retire this frame, with the Boxes that
		// name it, and declare the next paint in a copy. The copy shares the
		// arrays, so no memory is duplicated; one allocation happens every 2³²
		// Begins, not per paint.
		next := *f
		next.gen = 0
		f.gen = 0 // retire this frame: a Box of it has generation MaxUint32
		View.Frame = &next
		f = &next
	}
	f.gen++
	f.Surface, f.Palette, f.bounds, f.done = Surface, Palette, bounds, false
	f.Nodes = f.Nodes[:0]
	f.Lines = f.Lines[:0]
	f.Exts = append(f.Exts[:0], Extra{})
	f.Images = f.Images[:0]
	f.addons = f.addons[:0]
	clear(f.Binds) // a binding holds pointers into the model: not past the frame
	f.Binds = f.Binds[:0]
	f.Floats = f.Floats[:0]
	f.States, f.Gradients, f.Stops = f.States[:0], f.Gradients[:0], f.Stops[:0]
	clear(f.Draws) // a callback holds what it captured: not past the frame it was declared in
	f.Draws = f.Draws[:0]
	f.Opacity = 1
	f.popupLayer = 0
	f.memoDepth = 0
	f.page = nil
	f.tooltips = false
	f.nativeMark = 0
	if Surface != nil {
		f.nativeMark = len(Surface.NativeViews())
	}
	f.scratch.reset()
	resetControls(f)
	f.Scrolls.epoch++
	f.Parent = -1
	root := f.Add(kindContainer)
	n := &f.Nodes[root]
	n.Dir = column
	n.Size = [2]Dimension{{dimPx, bounds.Width}, {dimPx, bounds.Height}}
	f.Parent = root
	return f
}

// End lays the page out and paints it. Rects are valid afterwards, until the
// next Begin.
func (f *Frame) End() {
	if f.done {
		return
	}
	if f.settleFn != nil && len(f.Binds) > 0 {
		f.settleFn(f, -1)
	}
	f.done = true
	f.checkHits()
	f.applyRecipes()
	root := &f.Nodes[0]
	f.measure(AxisX)
	root.Rect.X, root.Rect.Width = f.bounds.X, f.bounds.Width
	f.solve(0, AxisX)
	for _, index := range f.Floats {
		f.placePopup(&f.Nodes[index], AxisX)
		f.solve(index, AxisX)
	}
	f.measure(AxisY)
	root.Rect.Y, root.Rect.Height = f.bounds.Y, f.bounds.Height
	f.solve(0, AxisY)
	if f.shiftedX {
		// A CustomLayout moved children horizontally after their width was solved.
		// Their subtrees moved with them, and a popup that hangs off one is placed
		// again from the anchor's final rectangle before its height is placed.
		for _, index := range f.Floats {
			f.placePopup(&f.Nodes[index], AxisX)
			f.solve(index, AxisX)
		}
		f.shiftedX = false
	}
	for _, index := range f.Floats {
		f.placePopup(&f.Nodes[index], AxisY)
		f.solve(index, AxisY)
	}
	f.reportScrolls()
	f.Scrolls.sweep()
	f.reportSplitters()
	f.PaintChild(0)
	f.raiseOverNativeViews()
	if f.popupLayer != 0 {
		f.Surface.PushLayer(f.popupLayer)
	}
	for _, index := range f.Floats {
		f.PaintChild(index)
	}
	if f.popupLayer != 0 {
		f.Surface.PopLayer()
	}
	if f.tooltips {
		f.paintTooltipFn(f)
	} else if f.paintTooltipFn != nil {
		f.tooltip = nil
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
	if f.wrapCursor < len(f.Wraps) {
		f.Wraps = f.Wraps[:f.wrapCursor]
	}
	if f.wrapCursor < len(f.Ellipses) {
		f.Ellipses = f.Ellipses[:f.wrapCursor]
	}
	f.layoutCtx.children = trimFlexCapacity(f.layoutCtx.children)
	f.Nodes = trimFlexCapacity(f.Nodes)
	f.Images = trimFlexCapacity(f.Images)
	f.Exts = trimFlexCapacity(f.Exts)
	f.Draws = trimFlexCapacity(f.Draws)
	f.Binds = trimFlexCapacity(f.Binds)
	f.Floats = trimFlexCapacity(f.Floats)
	f.Lines = trimFlexCapacity(f.Lines)
	f.Ellipses = trimFlexCapacity(f.Ellipses)
	f.Wraps = trimFlexCapacity(f.Wraps)
	trimControls(f)
	f.scratch.abs = trimFlexCapacity(f.scratch.abs)
	f.scratch.rows = trimFlexCapacity(f.scratch.rows)
	f.scratch.rowAt = trimFlexCapacity(f.scratch.rowAt)
	f.scratch.cells = trimFlexCapacity(f.scratch.cells)
}

// Canvas and Theme give the declaration what it was started with.
func (f *Frame) Canvas() *gfx.Canvas { return f.Surface }
func (f *Frame) Theme() Theme        { return f.Palette }

// Bounds is the rectangle this frame lays out in, as the caller gave it to Draw
// or Begin. Custom painting (a tooltip, a highlight) uses it to stay inside the
// region it belongs to: the frame's bounds are known before End, unlike a
// child's rectangle.
func (f *Frame) Bounds() Rect { return f.bounds }

func (f *Frame) Add(k Kind) int32 {
	if f.done {
		panic("ui: an element was declared after the page was laid out (in a Draw callback, or after End): declare elements in the build function")
	}
	index := int32(len(f.Nodes))
	f.Nodes = append(f.Nodes, Node{Kind: k, Parent: f.Parent, First: -1, Last: -1, Next: -1, Items: Auto})
	if f.Parent >= 0 {
		parent := &f.Nodes[f.Parent]
		if parent.Last < 0 {
			parent.First = index
		} else {
			f.Nodes[parent.Last].Next = index
		}
		f.Nodes[f.Parent].Last = index
	}
	return index
}

func (f *Frame) Box(k Kind) Box { return Box{f: f, i: f.Add(k), gen: f.gen} }

// Row opens a container that lays its children out left to right.
func (f *Frame) Row() Box {
	b := f.Box(kindContainer)
	f.Nodes[b.i].Dir = row
	return b
}

// Column opens a container that lays its children out top to bottom.
func (f *Frame) Column() Box { return f.Box(kindContainer) }

// Stack opens a container that lays its children over one another, each in the
// whole content area unless it has a size of its own.
func (f *Frame) Stack() Box {
	b := f.Box(kindContainer)
	f.Nodes[b.i].Dir = StackDir
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
	saved := f.Parent
	f.Parent = -1
	b := f.Box(kindContainer)
	f.Parent = saved
	f.Floats = append(f.Floats, b.i)
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
	b := f.Box(kindContainer)
	n := &f.Nodes[b.i]
	n.Dir = grid
	n.GridMin = max(minCell, 1)
	return b
}

// Flow opens a row that wraps: it lays its children out left to right and starts a
// new line below when the next one does not fit, as CSS's flex-wrap does. Each line
// shares its room as a Row does, so Grow, Shrink and Justify apply to every line
// alone; a line is as tall as its tallest child, and Items lines the children up
// within it. Gap is the space between children and, unless LineGap says otherwise,
// between lines. A child wider than the whole width has a line to itself; text and
// containers give way to it as they do in a Row, so a text's Ellipsis or Wrap
// decides how it fits.
func (f *Frame) Flow() Box {
	b := f.Box(kindContainer)
	n := &f.Nodes[b.i]
	n.Dir = flow
	n.GridMin = -1
	return b
}

// customLayout declares a container whose children a Layout measures and places.
func (f *Frame) customLayout(layout Layout) Box {
	b := f.Box(kindContainer)
	n := &f.Nodes[b.i]
	n.Dir = custom
	f.Extra(b.i).layout = layout
	return b
}

// Scroll opens a column whose children may be taller than it is: it clips them
// and shows them offset pixels up. The caller owns the offset (wheel events and
// scrollbars are the application's, as with components.ScrollArea); after End,
// Box.Content says how tall the children are, so the offset can be clamped to
// Content - Rect().Height. Give it a size, Fill or Grow, or it is as tall as its
// children.
func (f *Frame) Scroll(offset float32) Box {
	b := f.Box(kindContainer)
	n := &f.Nodes[b.i]
	n.Flags |= flagScroll | flagClip
	n.Scroll = max(offset, 0)
	return b
}

// Paragraph adds text that breaks into lines to fit the width it is given, at most
// maxLines of them (the last ends with an ellipsis), so its height follows its
// width. It is what Text is not: a Text is one line and is clipped. Zero or less
// takes as many lines as the width needs, as Box.MaxLines.
func (f *Frame) Paragraph(Value string, MaxLines int) Box {
	return f.Text(Value).MaxLines(MaxLines)
}

// Text adds one line of text, in the theme's body size and foreground color.
func (f *Frame) Text(Value string) Box {
	b := f.Box(kindText)
	n := &f.Nodes[b.i]
	n.Text = Value
	n.FontSize = f.Palette.Metrics.FontSizeM
	n.Color = f.Palette.Foreground
	n.Shrink = 1
	return b
}

// Spacer adds empty space that grows to take what the other children leave.
func (f *Frame) Spacer() Box {
	b := f.Box(kindSpacer)
	f.Nodes[b.i].Grow = 1
	f.Nodes[b.i].Flags |= flagGrowSet
	return b
}

// Divider adds a one pixel line in the theme's border color across the
// container: horizontal in a column, vertical in a row or a flow.
func (f *Frame) Divider() Box {
	b := f.Box(kindContainer)
	n := &f.Nodes[b.i]
	n.Background = f.Palette.Border
	if f.Parent >= 0 && (f.Nodes[f.Parent].Dir == row || f.Nodes[f.Parent].Dir == flow) {
		n.Size[AxisX] = Dimension{dimPx, 1}
	} else {
		n.Size[AxisY] = Dimension{dimPx, 1}
	}
	return b
}

// Icon adds an icon, 16 pixels square unless given a size, in the foreground
// color unless given a Color.
func (f *Frame) Icon(icon gfx.Icon) Box {
	b := f.Box(kindIcon)
	n := &f.Nodes[b.i]
	f.Extra(b.i).icon = icon
	n.Color = f.Palette.Foreground
	return b
}

// Image adds a bitmap, 16 pixels square unless given a size, scaled to fill the
// element's rectangle. It is the way to show a picture the application draws or
// asks the system for, such as app.FileIcon, among text and icons; the bitmap
// is not copied, so its pixels must stay unchanged while the page is on screen
// (see gfx.Bitmap). An empty or invalid bitmap leaves the room blank.
func (f *Frame) Image(bitmap gfx.Bitmap) Box {
	b := f.Box(kindImage)
	f.Nodes[b.i].Aux = int32(len(f.Images))
	f.Images = append(f.Images, bitmap)
	return b
}

// Button adds a button with the official look; Outline, Ghost and the other
// variants choose another. Clicked reports a click on it.
func (f *Frame) Button(ID gfx.ID, Label string) Box {
	b := f.Box(kindButton)
	n := &f.Nodes[b.i]
	n.ID, n.Label = ID, Label
	n.Flags |= flagHasHit
	return b
}

// Checkbox adds a checkbox in the given state; Clicked reports a click on it.
func (f *Frame) Checkbox(ID gfx.ID, checked bool) Box {
	b := f.Box(kindCheckbox)
	n := &f.Nodes[b.i]
	n.ID = ID
	if checked {
		n.Flags |= flagChecked
	}
	n.Flags |= flagHasHit
	return b
}

// Switch adds a two-state switch in the given state; Clicked reports a click.
func (f *Frame) Switch(ID gfx.ID, on bool) Box {
	b := f.Box(kindSwitch)
	n := &f.Nodes[b.i]
	n.ID = ID
	if on {
		n.Flags |= flagChecked
	}
	n.Flags |= flagHasHit
	return b
}

// Toggle adds a two-state button in the given state; Clicked reports a click.
func (f *Frame) Toggle(ID gfx.ID, Label string, pressed bool) Box {
	b := f.Box(kindToggle)
	n := &f.Nodes[b.i]
	n.ID, n.Label = ID, Label
	if pressed {
		n.Flags |= flagChecked | flagSelected
	}
	n.Flags |= flagHasHit
	return b
}

// Slot reserves room for something the caller paints itself: Rect gives where
// once End has laid the page out. Give it a size, or let it grow.
func (f *Frame) Slot() Box { return f.Box(kindSlot) }

// extra returns the extras of element i, giving it a set of its own if it has none.
func (f *Frame) Extra(i int32) *Extra {
	n := &f.Nodes[i]
	if n.Ext == 0 {
		n.Ext = int32(len(f.Exts))
		f.Exts = append(f.Exts, Extra{})
	}
	return &f.Exts[n.Ext]
}

// checkHits stops a page that declared the one shape that cannot be what was meant: a
// container given Hit, never given Children, and with nothing else that makes it
// show or take room (no size, background, border or place of its own). Such a row is
// as tall as its padding, paints nothing and is not the clickable row of text its
// author had in mind: Hit names a region for the pointer and the accessibility tree,
// it does not draw its label. Declare the content inside Children before Clicked, or
// give an empty target a size, a background or Absolute.
func (f *Frame) checkHits() {
	for i := range f.Nodes {
		n := &f.Nodes[i]
		if n.Kind != kindContainer || n.Flags&flagHasHit == 0 || n.Flags&flagChildren != 0 || n.First >= 0 {
			continue
		}
		if n.Size[AxisX].Kind != dimAuto || n.Size[AxisY].Kind != dimAuto || n.Background.Alpha > 0 || n.BorderWidth > 0 || n.OutOfFlow != 0 || n.MinS != [2]float32{} {
			continue
		}
		panic("ui: a Row, Column or Stack given Hit(" + n.Label + ") has no Children and no size, background or border, so it is empty: " +
			"declare what it shows with Children(func() { ... }) before Clicked(), e.g. f.Row().Hit(id, name).Children(func() { f.Text(name) }).Clicked()")
	}
}
