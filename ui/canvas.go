// Package ui defines lightweight, platform-neutral paint primitives.
package ui

import (
	"math"

	"github.com/vivaui/viva/internal/textmetrics"
)

// Color is a linear RGBA color.
type Color struct {
	Red   float32
	Green float32
	Blue  float32
	Alpha float32
}

// Rect is a logical-pixel rectangle measured from the top-left.
type Rect struct {
	X      float32
	Y      float32
	Width  float32
	Height float32
}

// Size is a width and height in logical pixels.
type Size struct {
	Width  float32
	Height float32
}

// Point is one logical-pixel coordinate measured from the top-left.
type Point struct {
	X float32
	Y float32
}

// Rectangle is one solid paint primitive. ClipOf/HasClip capture the active
// clip intersection at paint time; renderers intersect Bounds with it.
type Rectangle struct {
	Bounds  Rect
	Color   Color
	Radius  float32
	Layer   uint16
	ClipOf  Rect
	HasClip bool
}

// ShadowStyle is one soft drop-shadow layer, the arguments of a CSS box-shadow: the
// shadow is the casting rectangle moved by OffsetX and OffsetY, grown by Spread
// on every side (shrunk when negative), and blurred over a Blur-wide band either
// side of its edge.
type ShadowStyle struct {
	OffsetX float32
	OffsetY float32
	Blur    float32
	Spread  float32
}

// Extent is how far the shadow reaches beyond the rectangle that casts it, in
// every direction.
func (style ShadowStyle) Extent() float32 {
	if style.Blur <= 0 {
		return 0
	}
	return style.Blur + max(style.Spread, 0) + max(style.OffsetX, -style.OffsetX, style.OffsetY, -style.OffsetY)
}

// Shadow is one soft shadow primitive: the shadow that the rounded rectangle
// Bounds and Radius casts, placed and blurred by Style and painted in Color. The
// casting rectangle is cut out of the shadow, so a translucent surface drawn over
// it is not darkened. ClipOf/HasClip capture the active clip like Rectangle's.
type Shadow struct {
	Bounds  Rect
	Radius  float32
	Color   Color
	Style   ShadowStyle
	Layer   uint16
	ClipOf  Rect
	HasClip bool
	// RectsBefore and TrianglesBefore are how many of the canvas's rectangles and
	// triangles were painted before this shadow. Shadows, rectangles and triangles
	// live in separate lists; these two counts interleave them back into paint
	// order, as Triangle.RectsBefore does for triangles. Neither decreases along
	// Shadows().
	RectsBefore     uint32
	TrianglesBefore uint32
}

// ShadowLevel is a step of the elevation scale shadcn takes from Tailwind CSS: the
// stronger the level, the higher a surface floats above the page.
type ShadowLevel uint8

const (
	// ShadowNone casts no shadow.
	ShadowNone ShadowLevel = iota
	// ShadowXS is shadow-xs, for controls that sit on the page: inputs, outline
	// buttons, switches and select triggers.
	ShadowXS
	// ShadowSM is shadow-sm, for a raised part of a control: the selected tab, a
	// slider thumb.
	ShadowSM
	// ShadowMD is shadow-md, for a surface opened over the page: popovers, menus
	// and select lists.
	ShadowMD
	// ShadowLG is shadow-lg, for a surface that covers part of the page: sheets and
	// submenus.
	ShadowLG
)

// shadowLayers are the box-shadow layers of each level, black at the given
// alpha, in the order they are painted (the last one nearest the surface).
var shadowLayers = [...][2]struct {
	style ShadowStyle
	alpha float32
}{
	ShadowXS: {{ShadowStyle{OffsetY: 1, Blur: 2}, 0.05}},
	ShadowSM: {{ShadowStyle{OffsetY: 1, Blur: 3}, 0.10}, {ShadowStyle{OffsetY: 1, Blur: 2, Spread: -1}, 0.10}},
	ShadowMD: {{ShadowStyle{OffsetY: 4, Blur: 6, Spread: -1}, 0.10}, {ShadowStyle{OffsetY: 2, Blur: 4, Spread: -2}, 0.10}},
	ShadowLG: {{ShadowStyle{OffsetY: 10, Blur: 15, Spread: -3}, 0.10}, {ShadowStyle{OffsetY: 4, Blur: 6, Spread: -4}, 0.10}},
}

// Triangle is one solid vector-mesh primitive. Paths are tessellated into
// triangles so native backends need no SVG parser or general geometry engine.
type Triangle struct {
	Points [3]Point
	// Color is the triangle's color, or, when Shaded, that of Points[0].
	Color Color
	// Shade holds the colors of Points[1] and Points[2] when Shaded. A shaded
	// triangle varies linearly from one vertex color to the next across its face,
	// straight (not premultiplied) RGBA as SVG gradients do; a gradient is a set of
	// them. Unshaded triangles ignore Shade.
	Shade  [2]Color
	Shaded bool
	Layer  uint16
	// EdgeMask marks the triangle edges that lie on the vector shape's outer
	// boundary. Bit 0 is Points[0]→Points[1], bit 1 is Points[1]→Points[2],
	// and bit 2 is Points[2]→Points[0]. Renderers feather only these edges;
	// shared tessellation edges stay fully covered.
	EdgeMask uint8
	ClipOf   Rect
	HasClip  bool
	// RectsBefore is how many of the canvas's rectangles were painted before
	// this triangle. Rectangles and triangles live in separate lists; this is
	// what interleaves them back into paint order, so an icon drawn after a
	// highlight stays above it and one drawn before stays below. It never
	// decreases along Triangles().
	RectsBefore uint32
}

// TextStyle describes one system-font line.
type TextStyle struct {
	Size   float32
	Weight float32
	Color  Color
	// Mono selects the system fixed-pitch font instead of the proportional
	// one. Every ASCII character then advances by Canvas.CellSize(Size).
	Mono     bool
	MonoFace MonoFace
	// Direction is the paragraph direction of the line. The zero value takes it from
	// the first strong character of the text (UAX #9), which is what the platform's own
	// engine does. Set it for a line of a wrapped paragraph whose own first strong
	// character is not the paragraph's, so that every line of a Hebrew paragraph is
	// right-aligned and ordered right to left, including the one that starts with an
	// English name. It matters only to right-to-left text and neutral characters.
	Direction TextDirection
}

// TextDirection is the direction of a paragraph of text.
type TextDirection uint8

const (
	// DirectionAuto takes the direction of the first strong character of the text.
	DirectionAuto TextDirection = iota
	// DirectionLTR is left to right.
	DirectionLTR
	// DirectionRTL is right to left.
	DirectionRTL
)

// TextDirectionOf is the paragraph direction value has when no direction is set: right
// to left when its first strong character is Hebrew, Arabic or another right-to-left
// letter (UAX #9, rules P2 and P3), left to right otherwise. A paragraph that wraps takes
// it from its whole text and gives it to each line in TextStyle.Direction.
func TextDirectionOf(value string) TextDirection {
	if textmetrics.RightToLeft(value) {
		return DirectionRTL
	}
	return DirectionLTR
}

// MonoFace chooses a system-provided fixed-pitch font. The zero value is
// the platform's modern system mono. MonoClassic requests a traditional
// terminal face and falls back to MonoSystem when unavailable. Higher values
// are installed families named by MonoFamily, with the families that draw
// what they lack when made by MonoStack.
type MonoFace uint16

const (
	MonoSystem MonoFace = iota
	MonoClassic
)

// TextRun colors and places a stretch of a LabelRuns line: the Bytes bytes of
// the label's text that follow the previous run, drawn starting at cell Column
// and spanning Cells columns. Runs must cover the text in order and should
// split before every character that is not ASCII, so each such character sits
// on its own cell; a run holding a single character is centered within its
// Cells. Decorations are drawn across the run's whole cell span.
//
// Block Elements (U+2580 to U+259F: halves, eighths, quadrants, shades) are not drawn from
// a font: the renderer fills their exact fraction of the cell, so neighbouring cells meet
// without a seam; see BlockRectangles.
//
// Ligatures asks the fixed-pitch renderer to shape the run as one line instead
// of placing one glyph per cell, so a font's programming ligatures appear. The
// shaped run is fitted to the run's full cell span; it therefore needs ASCII
// text and at least two cells. Other runs keep the one-glyph-per-cell rule.
type TextRun struct {
	Column    int32
	Bytes     int32
	Cells     int32
	Color     Color
	Weight    float32
	Italic    bool
	Underline bool
	Strike    bool
	Ligatures bool
}

// Text is one immutable single-line label clipped like rectangles. A text
// with RunCount > 0 is a fixed-pitch grid line (see LabelRuns); read its runs
// with Canvas.TextRuns.
type Text struct {
	Value     string
	Bounds    Rect
	Style     TextStyle
	CellWidth float32
	RunFirst  int32
	RunCount  int32
	Layer     uint16
	ClipOf    Rect
	HasClip   bool
}

// Bitmap is immutable premultiplied BGRA pixel data. Pixels are stored top
// row first; Stride is bytes between rows and must be at least Width*4. The
// caller owns Pix and must keep it unchanged while a canvas can retain it.
// Native renderers upload the pixels lazily and identify unchanged content by
// ID and Version, so mutating Pix requires incrementing Version.
type Bitmap struct {
	Pix           []byte
	Width, Height int
	Stride        int
	ID            uint64
	Version       uint32
}

// Image is one bitmap draw. Source is measured in bitmap pixels. ClipOf and
// HasClip capture the active clip intersection at paint time.
type Image struct {
	Bitmap  Bitmap
	Source  Rect
	Bounds  Rect
	Layer   uint16
	ClipOf  Rect
	HasClip bool
	// Backdrop marks an image drawn beneath every other primitive of the frame,
	// whatever layer they are on. See Canvas.DrawBackdrop.
	Backdrop bool
	// DevicePixels marks an image whose Bounds and Source are whole device pixels
	// and whole bitmap pixels, drawn one bitmap pixel to one device pixel. The
	// painter keeps it so, at a fractional display scale too, instead of rounding
	// it to logical pixels. See Surface.DrawPixels.
	DevicePixels bool
}

// Role describes the platform-neutral semantic purpose of an interactive hit.
type Role uint8

const (
	RoleButton Role = iota + 1
	RoleLink
	RoleCheckbox
	RoleRadio
	RoleTab
	RoleMenuItem
	RoleCombobox
	RoleSwitch
	RoleTextField
	RoleSlider
	// RoleTerminal is a character-cell terminal. While it has focus the
	// keyboard is raw (see KeyEvent) and the application's text input is a
	// preedit surface at the cursor (see app.TextInput.Terminal).
	RoleTerminal
	// RoleGroup names a non-actionable region that belongs together, such as a
	// scroll viewport or a hover card's body. It can take focus but does not
	// activate with Enter or Space.
	RoleGroup
	// The popup container roles below belong to one surface, not to a control.
	// They are declared with Canvas.Container.
	RoleMenu
	RoleMenubar
	RoleListBox
	RoleOption
	RoleGrid
	RoleGridCell
	RoleHeading
	RoleStatus
	RoleSeparator
)

// LiveRegion says how eagerly assistive technology announces a change to the
// content of a region, as aria-live does. The zero value announces nothing.
type LiveRegion uint8

const (
	LiveOff LiveRegion = iota
	// LivePolite announces a change when the user is idle.
	LivePolite
	// LiveAssertive announces a change at once.
	LiveAssertive
)

// ValueRange is the numeric value of a slider, a progress bar or a spinner:
// where it is, the bounds it moves between and the smallest step. Set is false
// for a control with no number to report.
type ValueRange struct {
	Now, Min, Max, Step float64
	Set                 bool
}

// PointerCursor selects the pointer shape while a hit is hovered or dragged.
// The zero value is the platform's ordinary arrow.
type PointerCursor uint8

const (
	PointerCursorDefault PointerCursor = iota
	PointerCursorText
	PointerCursorHand
	PointerCursorCrosshair
	PointerCursorWait
	PointerCursorResizeHorizontal
	PointerCursorResizeVertical
	PointerCursorResizeNWSE
	PointerCursorResizeNESW
	PointerCursorNotAllowed
)

// HitOptions describes interaction and accessibility state without a dynamic
// property bag. Zero values represent an enabled focusable button.
type HitOptions struct {
	Role     Role
	Cursor   PointerCursor
	Value    string
	Hint     string
	Disabled bool
	// FocusableWhenDisabled keeps a disabled control in the Tab order and
	// focusable, as Base UI's accordion, collapsible and tab triggers do; it
	// never activates. Pointer presses still skip it.
	FocusableWhenDisabled bool
	Checked               bool
	// Mixed marks a checkbox in the indeterminate state (aria-checked="mixed").
	Mixed    bool
	Selected bool
	Expanded bool
	// Expandable marks a control that opens and closes something (a disclosure,
	// an accordion trigger, a menu with a submenu), so assistive technology
	// offers expand and collapse for it whichever state it is in now. A
	// combobox is always expandable.
	Expandable bool
	Multiline  bool
	ReadOnly   bool
	// Secure marks a password field. Its Value is always dropped so no
	// accessibility, mirroring, or reload path can expose it, and the runtime
	// masks what the platform text service draws.
	Secure bool
	// Description is a longer explanation than Hint, announced after the name
	// (aria-description).
	Description string
	// Level is the heading level, 1 to 9, or the nesting depth of a tree item.
	Level uint8
	// PosInSet and SetSize place an item among its siblings, counting from 1
	// (aria-posinset and aria-setsize). Zero means not stated.
	PosInSet, SetSize int32
	// Live makes the region announce changes to its content.
	Live LiveRegion
	// Range is the numeric value of a slider or a progress bar.
	Range     ValueRange
	SkipFocus bool
	// SkipTab keeps a focusable hit out of Tab traversal while leaving it
	// reachable by arrow keys, an explicit Focus, and labels. A roving group
	// (tabs, radios, a calendar grid) gives its selected item the tab stop and
	// marks every other item SkipTab.
	SkipTab bool
	// Backdrop marks a hit that only catches clicks away from an overlay, such
	// as the window-wide dismiss area behind a dialog or a menu. It is not a
	// control: a press on it dismisses the overlay as it would anywhere else,
	// including over a move region of a custom titlebar, which still moves the
	// window once the pointer drags (see MoveRegion).
	Backdrop bool
	// PassSecondary, on a Backdrop, makes a secondary press the way a native
	// menu takes it: the press dismisses the overlay as a click on the backdrop
	// would, and once the overlay is gone it is delivered again to whatever lies
	// under the pointer. A menu thereby moves to the next control that has one
	// in a single right click. Other buttons only dismiss. A dialog leaves it
	// off: it keeps the page behind it out of reach.
	PassSecondary bool
	// PassPrimary, on a Backdrop, is PassSecondary for a non-modal popover: a
	// primary press dismisses the overlay and is then delivered again to the
	// control beneath, so the first click outside both closes the popup and does
	// what the user aimed at, as a web popover does.
	PassPrimary bool
	// label makes the hit a label of its ID's control (see Canvas.HitLabel).
	label bool
	// Autofocus gives the hit the keyboard in the paint where it first
	// appears, that is, when the previous paint did not declare its ID. Use it
	// for a page that should be ready to type into as soon as it is shown,
	// such as a terminal. A hit that stays painted keeps whatever focus the
	// user gave it.
	Autofocus bool
	// TextBounds, TextSize and TextColor describe the text a text field
	// paints: where it sits, how large it is and what color. A native editor
	// that takes over the field draws exactly there, so focusing the field
	// does not move or resize its text. The runtime uses them wherever the
	// application's text input leaves its own bounds, size or color unset.
	TextBounds Rect
	TextSize   float32
	TextColor  Color
	// HideNativeOverlays asks the platform text service to keep its caret and
	// selection invisible while preserving native editing, IME and clipboard
	// behavior. Components that paint their own caret and selection set this
	// so the two presentations do not overlap.
	HideNativeOverlays bool
	// NavigationKeys asks Up, Down, PageUp and PageDown to reach the
	// application as raw ui.KeyEvents while the field is focused, instead
	// of being consumed by the text system or moving its caret. Use it
	// for a search field, combobox or command palette with a dropdown.
	// Cursor placement, selection, Home/End, word navigation and
	// input-method composition remain native.
	// Honored by the macOS text system and app.Driver; the other platform
	// backends do not consult it yet.
	NavigationKeys bool
	// Placement is set by a text field that lays its text out in rows itself
	// (a wrapping Textarea). The runtime then puts the caret where a press or
	// drag lands according to it, rather than the platform editor, which knows
	// one line only, and moves input-method windows to CaretBounds.
	Placement TextPlacement
}

// TextPlacement is what a text field that lays its own rows out tells the
// runtime about them, painted by the last frame. Offsets are byte offsets into
// the field's Value, which must be the text it paints.
type TextPlacement interface {
	// OffsetAt is the offset of the caret position nearest to a window point.
	// ok is false when the field has painted nothing to place a caret in.
	OffsetAt(point Point) (offset int, ok bool)
	// CaretBounds is where the caret was painted, in window coordinates, and
	// false while it is out of view.
	CaretBounds() (Rect, bool)
}

// Hit is one pointer interaction region in back-to-front paint order. The zero
// ID is reserved for "no target". Label is the accessibility name of the region.
type Hit struct {
	ID       ID
	Bounds   Rect
	Label    string
	Value    string
	Hint     string
	Scope    ID
	Layer    uint16
	Role     Role
	Cursor   PointerCursor
	Disabled bool
	// FocusableDisabled is HitOptions.FocusableWhenDisabled.
	FocusableDisabled bool
	Checked           bool
	Mixed             bool
	Selected          bool
	Expanded          bool
	// Expandable is HitOptions.Expandable.
	Expandable bool
	Multiline  bool
	ReadOnly   bool
	Secure     bool
	Focusable  bool
	// Description, Level, PosInSet, SetSize, Live and Range are the
	// HitOptions fields of the same names.
	Description       string
	Level             uint8
	PosInSet, SetSize int32
	Live              LiveRegion
	Range             ValueRange
	// SkipTab is HitOptions.SkipTab.
	SkipTab bool
	// Autofocus is HitOptions.Autofocus.
	Autofocus bool
	// Block marks an opaque region rather than a target: it has no ID and no
	// accessibility node, and presses inside it reach nothing declared before it.
	Block bool
	// Container marks a semantic region with no target: a menu, listbox, grid,
	// heading or status area. It is not hit-testable and declares no control.
	Container bool
	// IsLabel marks a label of the control the ID names (see Canvas.HitLabel):
	// a press on it is a press on the control, it is no tab stop and no node of
	// its own, and its Label text names the control for assistive technology.
	IsLabel bool
	// Backdrop is HitOptions.Backdrop.
	Backdrop bool
	// MovesWindow marks a region of a custom-titlebar window that moves the
	// window when pressed (see Canvas.MoveRegion). It has no ID and no
	// accessibility node.
	MovesWindow bool
	// PassSecondary is HitOptions.PassSecondary.
	PassSecondary bool
	// PassPrimary is HitOptions.PassPrimary.
	PassPrimary bool
	// TextBounds, TextSize and TextColor are the text geometry declared through
	// HitOptions.
	TextBounds Rect
	TextSize   float32
	TextColor  Color
	// HideNativeOverlays is HitOptions.HideNativeOverlays.
	HideNativeOverlays bool
	// NavigationKeys is HitOptions.NavigationKeys.
	NavigationKeys bool
	// Placement is HitOptions.Placement.
	Placement TextPlacement
}

// Clip is one clipping rectangle restricting the primitives painted after it
// until the next PopClip with the same depth.
type Clip struct {
	Bounds Rect
}

// Canvas retains paint primitives and hit regions in deterministic
// back-to-front order.
type Canvas struct {
	rectangles    []Rectangle
	triangles     []Triangle
	shadows       []Shadow
	images        []Image
	texts         []Text
	runs          []TextRun
	hits          []Hit
	clips         []Clip
	focusScopes   []ID
	layers        []uint16
	pathPoints    []Point
	contourEnds   []int32
	contourClosed []bool
	// iconPartCenters is where each contour of an icon in motion turns and grows,
	// rebuilt from the icon's own contour count on every painted frame.
	iconPartCenters []Point
	// iconContours holds the contours of an icon whose parts move, so a moving
	// glyph does not allocate the slice on every frame.
	iconContours []Icon
	pathBuilder  PathBuilder
	// gradientSource holds the flat triangles a path gradient recolors.
	gradientSource []Triangle
	// path and triangulation scratch survives Reset: reused every paint.
	strokeSideA     []Point
	strokeSideB     []Point
	strokeRing      []Point
	ringPoints      []Point
	ringEnds        []int32
	earcut          earcutScratch
	rings           []ringCacheEntry
	parentScratch   []int32
	consumedScratch []bool
	areasScratch    []float64
	edgeDir         []Point
	edgeNormal      []Point
	measurer        func(value string, size, weight float32) float32
	cellMeasurer    func(size float32, face MonoFace) (width, height float32)
	engine          TextEngine
	// spans is what an engine writes a selection into. The engine is an interface, so
	// a caller's own buffer passed to it would escape to the heap on every paint.
	spans    []TextSpan
	cellSize cellSizeMemo
	// fits remembers the lines FitLine cut; a different measurement empties it.
	fits         []fitEntry
	hovered      ID
	pressed      ID
	focused      ID
	scrollX      float32
	scrollY      float32
	trappedScope ID
	escapeTarget ID
	// The focus-scope depth each overlay claim was made at. A claim from a
	// deeper scope (a popup inside a dialog) wins over an outer one however
	// late the outer overlay finishes painting; equal depths let the later
	// claim win, which is painting order for sibling overlays.
	trapDepth, escapeDepth, initialDepth int
	// clicked is the control whose click the event pass is delivering; listens
	// records that the last paint asked about clicks at all.
	clicked                     ID
	eventPass, listensForClicks bool
	afterPaint                  []func()
	// pointer is the pointer event the event pass is delivering; consumed records
	// that a control took it; listensForPointer that the last paint asked.
	pointer           PointerEvent
	hasPointer        bool
	consumed          bool
	listensForPointer bool
	initialFocus      ID
	// focusRequest asks the runtime to focus one hit even without a trap, as a
	// non-modal popover does. The runtime honors a given request once, so
	// painting it every frame never pulls focus back after the user leaves.
	focusRequest   ID
	tree           *Tree
	iconCache      *iconCache
	iconPaintCache *iconPaintCache
	pixelScale     float32 // survives Reset; zero is referenceScale
	focusVisible   bool
	// motion survives Reset: transitions span many paint passes.
	motion motionState
}

// SetMeasurer replaces the canvas's text measurement with a width function, in logical
// pixels, for a font size and weight; nil restores it. It takes precedence over the
// TextEngine and the reference metrics for every text query. An application does not
// need it: a backend installs the platform's engine (SetTextEngine). It exists for tests
// and tools that want exact, simple widths, such as one em per character.
func (canvas *Canvas) SetMeasurer(measure func(value string, size, weight float32) float32) {
	canvas.measurer = measure
	clear(canvas.fits)
}

// SetCellMeasurer replaces the reference fixed-pitch cell (0.6 of the size wide,
// 1.18 high) with a function returning the advance of one character cell and the
// height of one line at a size; nil restores it. Like SetMeasurer it is for tests
// and tools.
func (canvas *Canvas) SetCellMeasurer(measure func(size float32, face MonoFace) (width, height float32)) {
	canvas.cellMeasurer = measure
	canvas.cellSize = cellSizeMemo{}
}

// CellSize is the size of one character cell of the fixed-pitch font at size,
// in logical pixels. The height is a whole number of pixels so rows of cells
// tile without seams.
func (canvas *Canvas) CellSize(size float32) (width, height float32) {
	return canvas.MonoCellSize(size, MonoSystem)
}

// MonoCellSize is the cell size of face at size: 0.6 of the size wide and 1.18 high
// on every platform, whatever the advance and line box of the font that draws the
// characters (the platforms place each character on this grid, so a terminal has the
// same columns and rows in the same window everywhere).
func (canvas *Canvas) MonoCellSize(size float32, face MonoFace) (width, height float32) {
	if size <= 0 {
		return 0, 0
	}
	if !validMonoFace(face) {
		face = MonoSystem
	}
	if memo := canvas.cellSize; memo.size == size && memo.face == face {
		return memo.width, memo.height
	}
	width, height = size*textmetrics.MonoAdvance, size*textmetrics.MonoLine
	if canvas.cellMeasurer != nil {
		if w, h := canvas.cellMeasurer(size, face); w > 0 && h > 0 {
			// A font's own line box says nothing about how tall a row should be.
			width, height = w, min(h, size*textmetrics.MonoLine)
		}
	}
	height = float32(int32(height + 0.999))
	canvas.cellSize = cellSizeMemo{size: size, width: width, height: height, face: face}
	return width, height
}

// cellSizeMemo remembers the last CellSize answer: a terminal asks every paint
// for the same size.
type cellSizeMemo struct {
	size, width, height float32
	face                MonoFace
}

// TextWidth measures one regular-weight line. Text drawn in another weight is measured
// with TextWidthWeight.
func (canvas *Canvas) TextWidth(value string, size float32) float32 {
	return canvas.TextWidthWeight(value, size, WeightRegular)
}

// TextWidthWeight measures one line at a weight (TextStyle.Weight): with the platform's
// TextEngine when the backend installed one, the same shaped line that is
// drawn, so the width is the glyphs' width; with the reference metrics
// (internal/textmetrics) otherwise, the same on every platform whatever fonts
// are installed. A heavier weight is wider; measure a label at the weight it is drawn
// with.
func (canvas *Canvas) TextWidthWeight(value string, size, weight float32) float32 {
	if value == "" {
		return 0
	}
	if canvas.measurer != nil {
		if width := canvas.measurer(value, size, weight); width > 0 {
			return width
		}
	}
	if canvas.engine != nil {
		if width := canvas.engine.Width(value, size, weight); width > 0 {
			return width
		}
	}
	return textmetrics.Width(value, size, weight)
}

// Reset discards all retained primitives while keeping the backing arrays
// for the next paint pass.
func (canvas *Canvas) Reset() {
	canvas.afterPaint = nil
	canvas.rectangles = canvas.rectangles[:0]
	canvas.triangles = canvas.triangles[:0]
	canvas.shadows = canvas.shadows[:0]
	canvas.images = canvas.images[:0]
	canvas.texts = canvas.texts[:0]
	canvas.runs = canvas.runs[:0]
	canvas.hits = canvas.hits[:0]
	canvas.clips = canvas.clips[:0]
	canvas.focusScopes = canvas.focusScopes[:0]
	canvas.layers = canvas.layers[:0]
	canvas.pathPoints = canvas.pathPoints[:0]
	canvas.contourEnds = canvas.contourEnds[:0]
	canvas.contourClosed = canvas.contourClosed[:0]
	canvas.iconPartCenters = canvas.iconPartCenters[:0]
	canvas.iconContours = canvas.iconContours[:0]
	canvas.motion.open = canvas.motion.open[:0]
	canvas.trappedScope = ID{}
	canvas.escapeTarget = ID{}
	canvas.initialFocus = ID{}
	canvas.focusRequest = ID{}
	canvas.trapDepth, canvas.escapeDepth, canvas.initialDepth = -1, -1, -1
	canvas.listensForClicks = false
	canvas.listensForPointer = false
}

// Trim releases backing arrays whose capacity has outgrown their content.
// Reset keeps the arrays for the next paint, which is right for a steady page
// but wrong after a one-off large one: without Trim, a page with 100,000 hits
// would keep that capacity for the life of the application. The painter calls
// it after publishing a frame. An application that owns a Canvas directly may
// call it after painting, once the previous frame has been handed to the
// platform; do not call it right after Reset, when the slices are empty, or a
// large steady page would reallocate every frame. It keeps the current
// contents, so it is safe between frames.
func (canvas *Canvas) Trim() {
	canvas.rectangles = trimCapacity(canvas.rectangles)
	canvas.triangles = trimCapacity(canvas.triangles)
	canvas.shadows = trimCapacity(canvas.shadows)
	canvas.images = trimCapacity(canvas.images)
	canvas.texts = trimCapacity(canvas.texts)
	canvas.runs = trimCapacity(canvas.runs)
	canvas.hits = trimCapacity(canvas.hits)
	canvas.clips = trimCapacity(canvas.clips)
	canvas.focusScopes = trimCapacity(canvas.focusScopes)
	canvas.layers = trimCapacity(canvas.layers)
	canvas.pathPoints = trimCapacity(canvas.pathPoints)
	canvas.contourEnds = trimCapacity(canvas.contourEnds)
	canvas.contourClosed = trimCapacity(canvas.contourClosed)
	canvas.pathBuilder.commands = trimCapacity(canvas.pathBuilder.commands)
	canvas.gradientSource = trimCapacity(canvas.gradientSource)
	canvas.strokeSideA = trimCapacity(canvas.strokeSideA)
	canvas.strokeSideB = trimCapacity(canvas.strokeSideB)
	canvas.strokeRing = trimCapacity(canvas.strokeRing)
	canvas.ringPoints = trimCapacity(canvas.ringPoints)
	canvas.ringEnds = trimCapacity(canvas.ringEnds)
	canvas.parentScratch = trimCapacity(canvas.parentScratch)
	canvas.consumedScratch = trimCapacity(canvas.consumedScratch)
	canvas.areasScratch = trimCapacity(canvas.areasScratch)
	canvas.edgeDir = trimCapacity(canvas.edgeDir)
	canvas.edgeNormal = trimCapacity(canvas.edgeNormal)
	canvas.spans = trimCapacity(canvas.spans)
	canvas.earcut.points = trimCapacity(canvas.earcut.points)
	canvas.earcut.ends = trimCapacity(canvas.earcut.ends)
	canvas.earcut.nodes = trimCapacity(canvas.earcut.nodes)
	canvas.earcut.triangles = trimCapacity(canvas.earcut.triangles)
	canvas.earcut.holes = trimCapacity(canvas.earcut.holes)
}

// trimCapacity keeps a slice's content but gives back its backing array when
// it is far larger than the content. The thresholds keep a steady page from
// reallocating every frame; only a peak that has already passed is released.
func trimCapacity[T any](items []T) []T {
	const minCapacity = 4096
	const ratio = 4
	if cap(items) <= minCapacity || len(items)*ratio >= cap(items) {
		return items
	}
	next := make([]T, len(items), max(len(items)*2, minCapacity/64))
	copy(next, items)
	return next
}

// DrawBitmap appends a complete bitmap draw. Invalid or empty bitmaps are
// ignored. Pixels are sampled linearly into bounds.
func (canvas *Canvas) DrawBitmap(bitmap Bitmap, bounds Rect) {
	canvas.DrawBitmapRegion(bitmap, Rect{Width: float32(bitmap.Width), Height: float32(bitmap.Height)}, bounds)
}

// DrawBitmapRegion appends a source region of bitmap sampled into bounds.
// Source is clipped to the bitmap without allocating or copying pixels.
func (canvas *Canvas) DrawBitmapRegion(bitmap Bitmap, source, bounds Rect) {
	if bitmap.Width <= 0 || bitmap.Height <= 0 || bitmap.Stride < bitmap.Width*4 || bounds.Width <= 0 || bounds.Height <= 0 {
		return
	}
	required := (bitmap.Height-1)*bitmap.Stride + bitmap.Width*4
	if required < 0 || len(bitmap.Pix) < required {
		return
	}
	left := max(source.X, 0)
	top := max(source.Y, 0)
	right := min(source.X+source.Width, float32(bitmap.Width))
	bottom := min(source.Y+source.Height, float32(bitmap.Height))
	if left >= right || top >= bottom {
		return
	}
	image := Image{
		Bitmap: bitmap,
		Source: Rect{X: left, Y: top, Width: right - left, Height: bottom - top},
		Bounds: bounds,
		Layer:  canvas.currentLayer(),
	}
	image.ClipOf, image.HasClip = canvas.clipSnapshot()
	canvas.images = append(canvas.images, image)
}

// DrawBackdrop appends a source region of bitmap sampled into bounds beneath
// everything else the frame paints: every rectangle, text and image of every
// layer, so that translucent surfaces let it show through. It is not clipped and
// belongs to no layer. At most one backdrop is meaningful; with several, they
// stack in the order given. Invalid or empty bitmaps are ignored.
func (canvas *Canvas) DrawBackdrop(bitmap Bitmap, source, bounds Rect) {
	before := len(canvas.images)
	canvas.DrawBitmapRegion(bitmap, source, bounds)
	if len(canvas.images) == before {
		return
	}
	image := &canvas.images[before]
	image.Layer, image.ClipOf, image.HasClip, image.Backdrop = 0, Rect{}, false, true
}

// Fill appends a solid rectangle. On a canvas that knows the display scale (SetPixelScale) its
// edges land on whole device pixels, so a panel or a divider is not a blur of two.
func (canvas *Canvas) Fill(bounds Rect, color Color) {
	canvas.appendRectangle(canvas.snapRect(bounds, 0), color, 0)
}

// Rounded appends a solid rounded rectangle, its edges snapped as Fill's are.
func (canvas *Canvas) Rounded(bounds Rect, radius float32, color Color) {
	canvas.appendRectangle(canvas.snapRect(bounds, radius), color, radius)
}

func (canvas *Canvas) appendRectangle(bounds Rect, color Color, radius float32) {
	rectangle := Rectangle{Bounds: bounds, Color: color, Radius: radius, Layer: canvas.currentLayer()}
	rectangle.ClipOf, rectangle.HasClip = canvas.clipSnapshot()
	canvas.rectangles = append(canvas.rectangles, rectangle)
}

// Shadow appends one soft shadow layer cast by the rounded rectangle bounds. Draw
// it before the surface it belongs to: the rectangle itself is cut out of the
// shadow, so only the part that falls outside it is painted, and a shadow
// without Blur paints nothing. The shadow reaches style.Extent beyond bounds, and
// the canvas clip cuts it like any other shape.
func (canvas *Canvas) Shadow(bounds Rect, radius float32, style ShadowStyle, color Color) {
	if style.Blur <= 0 || color.Alpha <= 0 || bounds.Width <= 0 || bounds.Height <= 0 {
		return
	}
	if canvas.pixelScale > 0 {
		// The edge the shadow is cut out along is the edge of the surface drawn over it.
		bounds = canvas.snapBox(bounds, radius)
	}
	shadow := Shadow{
		Bounds: bounds, Radius: radius, Color: color, Style: style, Layer: canvas.currentLayer(),
		RectsBefore: uint32(len(canvas.rectangles)), TrianglesBefore: uint32(len(canvas.triangles)),
	}
	shadow.ClipOf, shadow.HasClip = canvas.clipSnapshot()
	canvas.shadows = append(canvas.shadows, shadow)
}

// Elevate appends the shadow of a level of the elevation scale, one or two
// layers cast by the rounded rectangle bounds. Draw it before the surface.
func (canvas *Canvas) Elevate(bounds Rect, radius float32, level ShadowLevel) {
	if level == ShadowNone || int(level) >= len(shadowLayers) {
		return
	}
	for _, layer := range shadowLayers[level] {
		if layer.alpha > 0 {
			canvas.Shadow(bounds, radius, layer.style, Color{Alpha: layer.alpha})
		}
	}
}

// Border appends an outer border and inset fill.
func (canvas *Canvas) Border(bounds Rect, width float32, border, fill Color) {
	canvas.RoundedBorder(bounds, 0, width, border, fill)
}

// RoundedBorder appends a rounded border and inset fill. On a canvas that knows the display scale
// the outer edge lands on whole device pixels and the border is a whole number of them, the same
// on all four sides and at least one: a 1 point border at 150% is one pixel, not a pixel and a
// half blurred over two, as a browser draws it.
func (canvas *Canvas) RoundedBorder(bounds Rect, radius, width float32, border, fill Color) {
	if canvas.pixelScale > 0 {
		bounds = canvas.snapBox(bounds, radius)
		width = canvas.borderWidth(width)
	}
	canvas.appendRectangle(bounds, border, radius)
	if bounds.Width > width*2 && bounds.Height > width*2 {
		canvas.appendRectangle(Rect{
			X: bounds.X + width, Y: bounds.Y + width,
			Width: bounds.Width - width*2, Height: bounds.Height - width*2,
		}, fill, max(radius-width, 0))
	}
}

// hairline is the thickness, in logical pixels, up to which a rectangle is a line: a border, a
// divider, a focus ring. Its thickness is kept whatever its position.
const hairline = 2

// snapRect puts the edges of a rectangle on whole device pixels. A square-cornered rectangle
// rounds both edges, so neighbours that meet at a logical edge meet at a device edge. A rounded
// one rounds its start and its size, since two rounded edges could make a circle an ellipse a pixel
// wider than high. A hairline rounds its start and keeps a thickness of whole pixels
// (borderWidth): rounding both edges would make it one pixel at one position and two at the next.
func (canvas *Canvas) snapRect(bounds Rect, radius float32) Rect {
	if canvas.pixelScale <= 0 || bounds.Width <= 0 || bounds.Height <= 0 {
		return bounds
	}
	bounds.X, bounds.Width = canvas.snapSpan(bounds.X, bounds.Width, radius > 0, true)
	bounds.Y, bounds.Height = canvas.snapSpan(bounds.Y, bounds.Height, radius > 0, true)
	return bounds
}

// snapBox is snapRect for the outer edge of a bordered box, which is a box whatever its size: it
// has no hairline rule.
func (canvas *Canvas) snapBox(bounds Rect, radius float32) Rect {
	if bounds.Width <= 0 || bounds.Height <= 0 {
		return bounds
	}
	bounds.X, bounds.Width = canvas.snapSpan(bounds.X, bounds.Width, radius > 0, false)
	bounds.Y, bounds.Height = canvas.snapSpan(bounds.Y, bounds.Height, radius > 0, false)
	return bounds
}

func (canvas *Canvas) snapSpan(origin, length float32, keepSize, allowHairline bool) (float32, float32) {
	switch {
	case allowHairline && length <= hairline:
		return canvas.roundToDevice(origin), canvas.borderWidth(length)
	case keepSize:
		return canvas.roundToDevice(origin), max(canvas.roundToDevice(length), 1/canvas.pixelScale)
	}
	start, end := canvas.roundToDevice(origin), canvas.roundToDevice(origin+length)
	return start, end - start
}

func (canvas *Canvas) roundToDevice(value float32) float32 {
	return float32(math.Floor(float64(value)*float64(canvas.pixelScale)+0.5)) / canvas.pixelScale
}

// borderWidth is a thickness in logical pixels made whole device pixels, rounded down and at
// least one, as browsers do: 1 point is 1 pixel at 125% and 150%, 2 points are 3 at 150%.
func (canvas *Canvas) borderWidth(width float32) float32 {
	return float32(math.Max(math.Floor(float64(width)*float64(canvas.pixelScale)+1e-3), 1)) / canvas.pixelScale
}

// Alignment selects horizontal label placement within its bounds.
type Alignment uint8

const (
	// AlignLeft puts the line at the left edge, whatever its direction.
	AlignLeft Alignment = iota
	// AlignCenter centers the line.
	AlignCenter
	// AlignRight puts the line at the right edge, whatever its direction.
	AlignRight
	// AlignStart puts the line where its text begins: the left edge for left-to-right
	// text, the right edge for right-to-left text (TextStyle.Direction, else the first
	// strong character). It is what Label does.
	AlignStart
	// AlignEnd is the other end: the right edge for left-to-right text, the left edge
	// for right-to-left text.
	AlignEnd
)

// Label appends an immutable system-font line, aligned to the start of its text
// (AlignStart): the left edge, or the right edge for Hebrew, Arabic and other
// right-to-left text.
func (canvas *Canvas) Label(value string, bounds Rect, style TextStyle) {
	canvas.LabelAligned(value, bounds, style, AlignStart)
}

// LabelCenter appends a horizontally centered system-font line.
func (canvas *Canvas) LabelCenter(value string, bounds Rect, style TextStyle) {
	canvas.LabelAligned(value, bounds, style, AlignCenter)
}

// LabelAligned appends a system-font line with explicit horizontal placement.
// Placement uses the layout width of TextWidthWeight: the platform draws the line
// from there with its own advances, so a drawn line is off-center by half of the
// difference between the two (a pixel or so). A right-to-left line is
// drawn in the visual order UAX #9 gives it.
//
// A TextStyle.Direction that differs from the one the text has by itself is carried to
// the platform as an invisible LRM or RLM in front of the line, which the engines take
// as the paragraph direction too; the text of such a line is then not the caller's
// string, and costs one allocation.
func (canvas *Canvas) LabelAligned(value string, bounds Rect, style TextStyle, alignment Alignment) {
	value, bounds.X = canvas.placeLabel(value, bounds, style, alignment)
	text := Text{Value: value, Bounds: bounds, Style: style, Layer: canvas.currentLayer()}
	text.ClipOf, text.HasClip = canvas.clipSnapshot()
	canvas.texts = append(canvas.texts, text)
}

// LabelOrigin is the x at which LabelAligned starts the line value in bounds: the left end
// of the line as drawn. The caret, selection and composition of a text field are placed
// from it with TextCaret and TextSelection.
func (canvas *Canvas) LabelOrigin(value string, bounds Rect, style TextStyle, alignment Alignment) float32 {
	_, x := canvas.placeLabel(value, bounds, style, alignment)
	return x
}

// placeLabel resolves the direction and alignment of a line: the text the platform is
// given (value, with the direction mark a forced direction needs) and the left edge of
// its label.
func (canvas *Canvas) placeLabel(value string, bounds Rect, style TextStyle, alignment Alignment) (string, float32) {
	rtl := textmetrics.RightToLeft(value)
	switch style.Direction {
	case DirectionLTR, DirectionRTL:
		forced := style.Direction == DirectionRTL
		if forced != rtl {
			rtl = forced
			if forced {
				value = "\u200f" + value
			} else {
				value = "\u200e" + value
			}
		}
	}
	switch alignment {
	case AlignStart:
		alignment = AlignLeft
		if rtl {
			alignment = AlignRight
		}
	case AlignEnd:
		alignment = AlignRight
		if rtl {
			alignment = AlignLeft
		}
	}
	if alignment == AlignCenter || alignment == AlignRight {
		inset := bounds.Width - canvas.TextWidthWeight(value, style.Size, style.Weight)
		if alignment == AlignCenter {
			inset /= 2
		}
		if inset < 0 {
			inset = 0
		}
		// Move the origin only; keep the full bounds width so the label
		// texture size stays stable across measured-width changes.
		bounds.X += inset
	}
	return value, bounds.X
}

// TextSpan is a horizontal extent of a line.
type TextSpan struct{ X, Width float32 }

// TextCaret is the x, from the left end of the line value, of the caret at byte offset: in
// left-to-right text the width of what precedes it; in right-to-left text, or at a
// change of direction, where UAX #9 displays that boundary (the caret of Hebrew moves from
// the right).
func (canvas *Canvas) TextCaret(value string, size, weight float32, offset int) float32 {
	offset = clampOffset(value, offset)
	if canvas.measurer != nil {
		return canvas.TextWidthWeight(value[:offset], size, weight)
	}
	if canvas.engine != nil {
		return canvas.engine.Caret(value, size, weight, offset)
	}
	return textmetrics.Caret(value, size, weight, offset)
}

// TextSelection appends the extents of the selection [start, end) of value, measured from
// the left end of the line: one span, or several where the selection crosses a change of
// direction (the highlight of a selection in mixed Hebrew and English is not contiguous).
func (canvas *Canvas) TextSelection(dst []TextSpan, value string, size, weight float32, start, end int) []TextSpan {
	start, end = clampOffset(value, start), clampOffset(value, end)
	if start > end {
		start, end = end, start
	}
	if canvas.measurer != nil {
		return append(dst, TextSpan{X: canvas.TextWidthWeight(value[:start], size, weight), Width: canvas.TextWidthWeight(value[start:end], size, weight)})
	}
	if canvas.engine != nil {
		canvas.spans = canvas.engine.Selection(canvas.spans[:0], value, size, weight, start, end)
		return append(dst, canvas.spans...)
	}
	return referenceEngine{}.Selection(dst, value, size, weight, start, end)
}

// TextOffsetAt is the byte offset of the character boundary nearest to x, measured from the
// left end of the line value: where a click puts the caret.
func (canvas *Canvas) TextOffsetAt(value string, size, weight, x float32) int {
	if canvas.measurer != nil {
		return offsetAtByPrefix(value, x, func(prefix string) float32 { return canvas.TextWidthWeight(prefix, size, weight) })
	}
	if canvas.engine != nil {
		return clampOffset(value, canvas.engine.OffsetAt(value, size, weight, x))
	}
	return textmetrics.OffsetAt(value, size, weight, x)
}

// LabelRuns appends one fixed-pitch line: value drawn in the system fixed-pitch
// font with every character on a grid of cellWidth, each run of it in its own
// color and weight. A terminal row is one such line, however many colors it
// holds, so the number and order of labels stays stable while its content
// changes. Bounds is the whole line; its width should be columns*cellWidth.
// runs must cover value (see TextRun); the canvas copies them, so the caller
// may reuse the slice as soon as the call returns.
func (canvas *Canvas) LabelRuns(value string, bounds Rect, style TextStyle, cellWidth float32, runs []TextRun) {
	style.Mono = true
	text := Text{
		Value: value, Bounds: bounds, Style: style, CellWidth: cellWidth,
		RunFirst: int32(len(canvas.runs)), RunCount: int32(len(runs)), Layer: canvas.currentLayer(),
	}
	canvas.runs = append(canvas.runs, runs...)
	text.ClipOf, text.HasClip = canvas.clipSnapshot()
	canvas.texts = append(canvas.texts, text)
}

// AllTextRuns returns every run of the current paint pass; Text.RunFirst and
// Text.RunCount index into it. The slice is valid until the canvas is reset.
func (canvas *Canvas) AllTextRuns() []TextRun { return canvas.runs }

// TextRuns returns the runs of a text produced by LabelRuns. The slice is valid
// until the canvas is reset.
func (canvas *Canvas) TextRuns(text Text) []TextRun {
	if text.RunCount == 0 || int(text.RunFirst+text.RunCount) > len(canvas.runs) {
		return nil
	}
	return canvas.runs[text.RunFirst : text.RunFirst+text.RunCount : text.RunFirst+text.RunCount]
}

// clipSnapshot returns the intersection of the active clip stack and whether
// any clip is active.
func (canvas *Canvas) clipSnapshot() (Rect, bool) {
	if len(canvas.clips) == 0 {
		return Rect{}, false
	}
	intersection := canvas.clips[0].Bounds
	for _, clip := range canvas.clips[1:] {
		intersection = intersectCanvasRect(intersection, clip.Bounds)
	}
	return intersection, true
}

func intersectCanvasRect(a, b Rect) Rect {
	left := max32v(a.X, b.X)
	top := max32v(a.Y, b.Y)
	right := min32v(a.X+a.Width, b.X+b.Width)
	bottom := min32v(a.Y+a.Height, b.Y+b.Height)
	if right <= left || bottom <= top {
		return Rect{}
	}
	return Rect{X: left, Y: top, Width: right - left, Height: bottom - top}
}

func max32v(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

func min32v(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

// Hit declares a pointer interaction region in paint order. Later hits win
// over earlier ones at the same point.
func (canvas *Canvas) Hit(id ID, bounds Rect) {
	canvas.HitWithLabel(id, bounds, "")
}

// HitWithLabel declares an interaction region with an accessibility name.
func (canvas *Canvas) HitWithLabel(id ID, bounds Rect, label string) {
	canvas.HitWithOptions(id, bounds, label, HitOptions{Role: RoleButton})
}

// HitWithOptions declares an interaction region with typed semantic state.
// The current focus scope and compositing layer are captured at declaration time.
func (canvas *Canvas) HitWithOptions(id ID, bounds Rect, label string, options HitOptions) {
	if clip, clipped := canvas.clipSnapshot(); clipped {
		bounds = intersectCanvasRect(bounds, clip)
		if bounds.Width <= 0 || bounds.Height <= 0 {
			return
		}
	}
	role := options.Role
	if role == 0 {
		role = RoleButton
	}
	focusable := (!options.Disabled || options.FocusableWhenDisabled) && !options.SkipFocus
	if options.Secure {
		options.Value = ""
	}
	canvas.hits = append(canvas.hits, Hit{
		ID: id, Bounds: bounds, Label: label,
		Value: options.Value, Hint: options.Hint,
		Scope: canvas.currentFocusScope(), Layer: canvas.currentLayer(), Role: role, Cursor: options.Cursor,
		Disabled: options.Disabled, FocusableDisabled: options.Disabled && options.FocusableWhenDisabled,
		Checked: options.Checked, Mixed: options.Mixed,
		Selected: options.Selected, Expanded: options.Expanded, Expandable: options.Expandable || options.Expanded,
		Multiline: options.Multiline, ReadOnly: options.ReadOnly, Secure: options.Secure,
		Description: options.Description, Level: options.Level, PosInSet: options.PosInSet, SetSize: options.SetSize,
		Live: options.Live, Range: options.Range,
		Focusable: focusable, SkipTab: focusable && options.SkipTab, Autofocus: options.Autofocus && focusable, IsLabel: options.label, Backdrop: options.Backdrop,
		PassSecondary: options.Backdrop && options.PassSecondary, PassPrimary: options.Backdrop && options.PassPrimary,
		TextBounds: options.TextBounds, TextSize: options.TextSize, TextColor: options.TextColor,
		HideNativeOverlays: options.HideNativeOverlays,
		NavigationKeys:     options.NavigationKeys,
		Placement:          options.Placement,
	})
}

// HitLabel declares bounds as a label of the control with that identity, as an
// HTML `<label for>` is: pressing it focuses the control and clicks it (a
// checkbox or a switch toggles, a text field takes the keyboard, and Clicked
// reports the control), and it is hovered and pressed as the control is. It is
// not a tab stop and no node of its own; text, the label's words, names the
// control for assistive technology in place of the control's own label. A
// disabled label does nothing, as a disabled control does. Declare it over the
// text that is painted, after or before the control: only the control's own hit
// is focusable, so the keyboard visits it once.
//
//	canvas.HitLabel(subscribe, titleBounds, "Subscribe to updates", false)
func (canvas *Canvas) HitLabel(control ID, bounds Rect, text string, disabled bool) {
	if control.IsZero() {
		return
	}
	canvas.HitWithOptions(control, bounds, text, HitOptions{Role: RoleButton, Disabled: disabled, SkipFocus: true, label: true})
}

// Container declares a semantic region that is not a control: a menu,
// menubar, listbox, grid, heading or status area. It has no target, takes no
// press, and appears in the accessibility tree with the given role and label.
func (canvas *Canvas) Container(bounds Rect, role Role, label string) {
	canvas.ContainerWithOptions(bounds, role, label, HitOptions{})
}

// ContainerWithOptions is Container with the accessibility detail of a region:
// the Level of a heading, the Live setting of a status area, a Description.
// Other options have no meaning for a region that takes no press.
func (canvas *Canvas) ContainerWithOptions(bounds Rect, role Role, label string, options HitOptions) {
	if clip, clipped := canvas.clipSnapshot(); clipped {
		bounds = intersectCanvasRect(bounds, clip)
	}
	if bounds.Width <= 0 || bounds.Height <= 0 || role == 0 {
		return
	}
	canvas.hits = append(canvas.hits, Hit{
		Bounds: bounds, Label: label, Scope: canvas.currentFocusScope(), Layer: canvas.currentLayer(),
		Role: role, Container: true,
		Description: options.Description, Level: options.Level, PosInSet: options.PosInSet, SetSize: options.SetSize,
		Live: options.Live, Range: options.Range,
	})
}

// Block declares an opaque region, such as the surface of a dialog or menu.
// Pointer presses inside it hit nothing declared before it, so a full-window
// dismiss target underneath a panel is not triggered by clicking the panel's
// padding. Hits declared after Block, like the panel's own buttons, still win.
func (canvas *Canvas) Block(bounds Rect) {
	if clip, clipped := canvas.clipSnapshot(); clipped {
		bounds = intersectCanvasRect(bounds, clip)
	}
	if bounds.Width <= 0 || bounds.Height <= 0 {
		return
	}
	canvas.hits = append(canvas.hits, Hit{Bounds: bounds, Scope: canvas.currentFocusScope(), Layer: canvas.currentLayer(), Block: true})
}

// MoveRegion declares a place a custom-titlebar window is moved from: pressing
// and dragging it moves the window, and a double click zooms it. It is inert as
// Block is, with no ID, hover, focus or accessibility node, and it lies where it
// was declared in paint order: a control declared after it, such as a button of a
// tab bar laid over it, takes its own presses, and so does the surface of a dialog
// above it. An overlay's backdrop above it still receives a press (to dismiss the
// overlay), and the window then moves once the pointer has dragged. A window with
// no move region is moved from nowhere.
func (canvas *Canvas) MoveRegion(bounds Rect) {
	if clip, clipped := canvas.clipSnapshot(); clipped {
		bounds = intersectCanvasRect(bounds, clip)
	}
	if bounds.Width <= 0 || bounds.Height <= 0 {
		return
	}
	canvas.hits = append(canvas.hits, Hit{Bounds: bounds, Scope: canvas.currentFocusScope(), Layer: canvas.currentLayer(), MovesWindow: true})
}

// Hits returns the retained hit regions. The result is invalidated by the
// next Reset.
func (canvas *Canvas) Hits() []Hit {
	return canvas.hits
}

// PushFocusScope assigns subsequent hits to a stable component scope.
func (canvas *Canvas) PushFocusScope(scope ID) {
	if !scope.IsZero() {
		canvas.focusScopes = append(canvas.focusScopes, scope)
	}
}

// PopFocusScope closes the innermost focus scope.
func (canvas *Canvas) PopFocusScope() {
	if len(canvas.focusScopes) > 0 {
		canvas.focusScopes = canvas.focusScopes[:len(canvas.focusScopes)-1]
	}
}

// TrapFocus restricts hit testing and keyboard traversal to scope. A scope
// trapped from a deeper focus scope than the last one wins, so a popup painted
// inside a dialog keeps the trap even though the dialog finishes painting after
// it; two scopes at the same depth let the later one win, which is painting
// order for sibling overlays.
func (canvas *Canvas) TrapFocus(scope ID) {
	if depth := len(canvas.focusScopes); depth >= canvas.trapDepth {
		canvas.trappedScope, canvas.trapDepth = scope, depth
	}
}

// TrappedFocusScope returns the top-most active focus trap.
func (canvas *Canvas) TrappedFocusScope() ID {
	return canvas.trappedScope
}

// RequestFocus asks the runtime to focus id even when no focus trap is active,
// as a non-modal popover does. The runtime applies one request once; painting
// the same request every frame does not pull focus back after the user leaves.
func (canvas *Canvas) RequestFocus(id ID) {
	canvas.focusRequest = id
}

// FocusRequest returns the non-modal focus request, or zero.
func (canvas *Canvas) FocusRequest() ID { return canvas.focusRequest }

// SetInitialFocus names the hit that receives keyboard focus when the trapped
// scope opens, which is how a modal moves focus inside itself. It has no
// effect while the scope stays open, so painting it every frame never steals
// focus back from the user.
func (canvas *Canvas) SetInitialFocus(id ID) {
	if depth := len(canvas.focusScopes); depth >= canvas.initialDepth {
		canvas.initialFocus, canvas.initialDepth = id, depth
	}
}

// InitialFocus returns the hit named by SetInitialFocus, or zero.
func (canvas *Canvas) InitialFocus() ID {
	return canvas.initialFocus
}

// SetEscapeTarget routes Escape to one explicit application-owned action ID.
// The deepest claim wins, as TrapFocus does, so an open popup owns Escape even
// when the dialog around it finishes painting later.
func (canvas *Canvas) SetEscapeTarget(id ID) {
	if depth := len(canvas.focusScopes); depth >= canvas.escapeDepth {
		canvas.escapeTarget, canvas.escapeDepth = id, depth
	}
}

// EscapeTarget returns the application-owned dismiss action ID.
func (canvas *Canvas) EscapeTarget() ID {
	return canvas.escapeTarget
}

func (canvas *Canvas) currentFocusScope() ID {
	if len(canvas.focusScopes) == 0 {
		return ID{}
	}
	return canvas.focusScopes[len(canvas.focusScopes)-1]
}

// PushLayer assigns subsequent paint primitives to a higher compositing layer.
// Layer zero is normal content; overlays should use monotonically increasing
// stable IDs and paint after their owning content.
func (canvas *Canvas) PushLayer(layer uint16) {
	canvas.layers = append(canvas.layers, layer)
}

// PopLayer closes the innermost compositing layer.
func (canvas *Canvas) PopLayer() {
	if len(canvas.layers) > 0 {
		canvas.layers = canvas.layers[:len(canvas.layers)-1]
	}
}

func (canvas *Canvas) currentLayer() uint16 {
	if len(canvas.layers) == 0 {
		return 0
	}
	return canvas.layers[len(canvas.layers)-1]
}

// Clip restricts painting to bounds until the matching Unclip call. Clips
// nest; the effective region is the intersection with enclosing clips.
func (canvas *Canvas) Clip(bounds Rect) {
	canvas.clips = append(canvas.clips, Clip{Bounds: bounds})
}

// Unclip closes the innermost open clip.
func (canvas *Canvas) Unclip() {
	if len(canvas.clips) > 0 {
		canvas.clips = canvas.clips[:len(canvas.clips)-1]
	}
}

// Clips returns the retained clip stack. The result is invalidated by the
// next Reset.
func (canvas *Canvas) Clips() []Clip {
	return canvas.clips
}

// SetInteraction records the runtime pointer and focus state before a paint
// pass so components can render hover, pressed, and focus feedback. It is
// runtime-facing; application paint code should only read it.
func (canvas *Canvas) SetInteraction(hovered, pressed, focused ID, focusVisible bool) {
	canvas.hovered = hovered
	canvas.pressed = pressed
	canvas.focused = focused
	canvas.focusVisible = focusVisible
}

// Hovered returns the hit ID currently under the pointer, or the zero ID.
func (canvas *Canvas) Hovered() ID {
	return canvas.hovered
}

// Pressed returns the hit ID currently held down, or the zero ID.
func (canvas *Canvas) Pressed() ID {
	return canvas.pressed
}

// Focused returns the hit ID holding keyboard focus, or the zero ID.
func (canvas *Canvas) Focused() ID {
	return canvas.focused
}

func (canvas *Canvas) FocusVisible() ID {
	if canvas.focusVisible {
		return canvas.focused
	}
	return ID{}
}

// Tree returns the identity tree this canvas names its places in: the one the
// application gave it with SetTree, or else one of its own. Embedded canvases
// share their parent's.
func (canvas *Canvas) Tree() *Tree {
	if canvas.tree == nil {
		canvas.tree = new(Tree)
	}
	return canvas.tree
}

// SetTree makes tree the identity tree of this canvas. An application that
// declares its identities in package-level variables owns the tree they are
// made in and gives it to every window's canvas; call it before the first paint.
func (canvas *Canvas) SetTree(tree *Tree) {
	canvas.tree = tree
}

// SetScroll records the active scroll offset before a paint pass. Positive
// values mean the viewport moved right/down, revealing content below.
func (canvas *Canvas) SetScroll(x, y float32) {
	canvas.scrollX = x
	canvas.scrollY = y
}

// ScrollX returns the horizontal scroll offset of the active scroll region.
func (canvas *Canvas) ScrollX() float32 {
	return canvas.scrollX
}

// ScrollY returns the vertical scroll offset of the active scroll region.
func (canvas *Canvas) ScrollY() float32 {
	return canvas.scrollY
}

// Rectangles returns the retained rectangle paint list.
func (canvas *Canvas) Rectangles() []Rectangle {
	return canvas.rectangles
}

// Triangles returns the retained vector-mesh paint list.
func (canvas *Canvas) Triangles() []Triangle {
	return canvas.triangles
}

// Shadows returns the soft shadows painted so far. Each carries how many
// rectangles and triangles came before it, which is how they are merged with
// Rectangles and Triangles into paint order. The result is invalidated by Reset.
func (canvas *Canvas) Shadows() []Shadow {
	return canvas.shadows
}

// Images returns the retained bitmap paint list.
func (canvas *Canvas) Images() []Image {
	return canvas.images
}

// Texts returns the retained text paint list.
func (canvas *Canvas) Texts() []Text {
	return canvas.texts
}

// Clicked reports whether id is the control whose click this paint is delivering.
// It is true for one control, in one pass: when a click completes, the runtime
// paints once more in the event pass, and only there may a paint change the
// application's state. Every other paint is read-only and Clicked is false in it.
//
//	if components.BeginButton(canvas, theme, save, bounds).Default("Save").Clicked() {
//		model.save() // runs once per click
//	}
//
// Asking costs a comparison. A paint that never asks makes the event pass
// unnecessary, and the runtime skips it.
func (canvas *Canvas) Clicked(id ID) bool {
	canvas.listensForClicks = true
	return canvas.eventPass && !id.IsZero() && canvas.clicked == id
}

// ClickedByKey is Clicked for a click that came from the keyboard: Enter (or Space) on the
// focused control, not a press and release of the pointer. The pointer's click is delivered
// while the control is still held down (Pressed is its ID); the keyboard's is not. A text
// field uses it to tell "Enter was pressed in me" from "I was clicked".
func (canvas *Canvas) ClickedByKey(id ID) bool {
	return canvas.Clicked(id) && canvas.pressed != id
}

// ClickedIn reports which item of the list scope this paint is delivering a click
// on, and false in every other paint or when the click was elsewhere. It is
// Clicked for a whole list at once, for the component that numbers its own rows.
func (canvas *Canvas) ClickedIn(list Scope) (int, bool) {
	canvas.listensForClicks = true
	if !canvas.eventPass {
		return 0, false
	}
	return canvas.clicked.In(list)
}

// ClickTarget returns the control whose click this paint is delivering, and false
// in every ordinary paint. It is Clicked for a central dispatcher: an application
// that keeps one handler for all of its controls reads the target once, at the
// start of its paint, before anything is drawn, and hands it on:
//
//	if id, ok := canvas.ClickTarget(); ok {
//		model.Click(id) // runs once per click, before this paint draws
//	}
//
// A component or a screen that owns its controls asks Clicked(id) of each one
// instead; use ClickTarget only where a single switch over identities is the
// application's design (and in the development reload host, which forwards the
// event pass to the process that runs the application). A handler that may block
// or open a native dialog is run through AfterPaint. Asking marks the paint
// as listening for clicks, exactly as Clicked does, so the runtime schedules the
// event pass for the clicks that follow.
func (canvas *Canvas) ClickTarget() (ID, bool) {
	canvas.listensForClicks = true
	return canvas.clicked, canvas.eventPass
}

// AfterPaint defers fn until the event pass has returned: the runtime calls it on
// the UI thread, outside any paint, and repaints afterwards, so the repaint shows
// what fn changed. It only takes effect in the event pass of a click and is
// ignored in an ordinary paint, so a read-only paint never schedules work. Use it
// when the handler of a click may block or open a native dialog, which must not
// run while a paint is in progress:
//
//	if id, ok := canvas.ClickTarget(); ok {
//		canvas.AfterPaint(func() { model.Click(id) })
//	}
//
// The functions run in the order they were deferred, once. A function that calls
// AfterPaint itself is ignored, because it no longer runs inside a paint. Nothing
// is allocated until a function is deferred.
func (canvas *Canvas) AfterPaint(fn func()) {
	if canvas.eventPass && fn != nil {
		canvas.afterPaint = append(canvas.afterPaint, fn)
	}
}

// RunAfterPaint runs and clears the functions deferred by AfterPaint. The runtime
// calls it once the event pass has ended; an application never does.
func (canvas *Canvas) RunAfterPaint() {
	fns := canvas.afterPaint
	canvas.afterPaint = nil
	for _, fn := range fns {
		fn()
	}
}

// ListensForClicks reports whether the last paint asked Clicked about anything:
// whether a click needs the event pass.
func (canvas *Canvas) ListensForClicks() bool { return canvas.listensForClicks }

// BeginEventPass makes the next paint the event pass of a click on id: Clicked(id)
// is true in it. Pair it with EndEventPass around that one paint. The runtime does
// this; an application only calls Clicked.
func (canvas *Canvas) BeginEventPass(id ID) {
	canvas.clicked, canvas.eventPass = id, true
}

// EndEventPass returns the canvas to ordinary, read-only painting.
func (canvas *Canvas) EndEventPass() {
	canvas.clicked, canvas.eventPass = ID{}, false
}

// PointerOn returns the pointer event this paint is delivering to id, and false in
// every other paint or when the event was aimed elsewhere. It is how a control
// reads a right click, a double click or a drag where it is declared:
//
//	if event, ok := canvas.PointerOn(row); ok && event.Button == ui.PointerButtonSecondary {
//		menu.OpenAt(event.X, event.Y)
//		canvas.Consume()
//	}
//
// Like Clicked, it is answered only in the event pass, and a paint that never asks
// makes the pass unnecessary. Pointer moves that no press captured are not
// delivered this way: hover is Hovered.
func (canvas *Canvas) PointerOn(id ID) (PointerEvent, bool) {
	canvas.listensForPointer = true
	if !canvas.hasPointer || id.IsZero() || canvas.pointer.Target != id {
		return PointerEvent{}, false
	}
	return canvas.pointer, true
}

// PointerIn is PointerOn for a whole list: the item number the event was aimed at.
func (canvas *Canvas) PointerIn(list Scope) (int, PointerEvent, bool) {
	canvas.listensForPointer = true
	if !canvas.hasPointer {
		return 0, PointerEvent{}, false
	}
	index, ok := canvas.pointer.Target.In(list)
	if !ok {
		return 0, PointerEvent{}, false
	}
	return index, canvas.pointer, true
}

// Consume says the control took the pointer event being delivered: the runtime
// does not also treat it as a click, and repaints. It does nothing outside the
// event pass.
func (canvas *Canvas) Consume() {
	if canvas.hasPointer {
		canvas.consumed = true
	}
}

// ListensForPointer reports whether the last paint asked PointerOn or PointerIn:
// whether a pointer event needs the event pass.
func (canvas *Canvas) ListensForPointer() bool { return canvas.listensForPointer }

// BeginPointerPass makes the next paint the event pass of a pointer event.
func (canvas *Canvas) BeginPointerPass(event PointerEvent) {
	canvas.pointer, canvas.hasPointer, canvas.consumed = event, true, false
}

// EndPointerPass ends the pointer event pass and reports whether a control
// consumed the event.
func (canvas *Canvas) EndPointerPass() bool {
	consumed := canvas.consumed
	canvas.pointer, canvas.hasPointer, canvas.consumed = PointerEvent{}, false, false
	return consumed
}

// DeliverPointer runs paint as the event pass of a pointer event and reports
// whether a control consumed it. It is what the runtime does for a window, for a
// test or a tool that drives a paint function without one:
//
//	if canvas.DeliverPointer(event, size, state.Paint) { ... }
//
// It does nothing, and reports false, when the last paint never asked PointerOn or
// PointerIn.
func (canvas *Canvas) DeliverPointer(event PointerEvent, size Size, paint func(*Canvas, Size)) bool {
	if paint == nil || !canvas.listensForPointer {
		return false
	}
	canvas.BeginPointerPass(event)
	canvas.Reset()
	canvas.BeginFrame()
	paint(canvas, size)
	canvas.EndFrame()
	return canvas.EndPointerPass()
}
