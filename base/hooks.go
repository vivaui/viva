package base

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/style"
)

// The styled layer's painting, measurement and construction. Package ui installs
// the hooks a page's features need as it declares them, so an application links
// only the painters it uses; InstallPainters installs all of them for a host
// that drives base directly. The engine below never imports the appearance code.
var (
	PaintSwatch          func(f *Frame, n *Node, r Rect)
	PaintPickerPanel     func(s *ColorPickerState, d *DrawContext, c C, ID gfx.ID, look ColorPickerLook)
	PaintCombo           func(s *ComboState, d *DrawContext)
	PaintAddons          func(f *Frame, n *Node, snapshot InputSnapshot, a *FieldAddons)
	PaintScrollbars      func(f *Frame, n *Node, s *ScrollState)
	PaintTooltip         func(f *Frame)
	PaintTabs            func(f *Frame, n *Node)
	PaintSelect          func(f *Frame, n *Node)
	MeasureSelect        func(f *Frame, n *Node)
	PaintDialog          func(f *Frame, n *Node)
	PaintDrawer          func(f *Frame, n *Node)
	PaintAlertDialog     func(f *Frame, n *Node)
	PaintTableRows       func(f *Frame, t *TableDecl, rows Rect, first, last int, top, contentWidth, rowHeight float32)
	PaintTableHeaderRow  func(f *Frame, t *TableDecl, r Rect, headerHeight, rowHeight float32)
	PaintTableScrollbars func(f *Frame, t *TableDecl, r, body Rect, content gfx.Size, offsetX, offsetY, frozenWidth float32)
	PaintMenu            func(m *MenuItems, surface *gfx.Canvas, palette *Theme, bounds Rect, layer uint16)
	MenuSize             func(m *MenuItems) (width, height float32)
	MenuRowExtent        func(m *MenuItems, i int) float32
	PaintTreeRow         func(s *TreeState, i int, box Box)
	TreeDraw             func(s *TreeState, d *DrawContext)
	EntryLook            func(f *Frame, palette *Theme, state gfx.State) style.TextEntryLook
	ButtonLookFn         func(f *Frame, n *Node, palette *Theme, state gfx.State) style.ButtonLook
	BadgeLookFn          func(f *Frame, n *Node, palette *Theme, state gfx.State) style.ButtonLook
	ToggleLookFn         func(f *Frame, n *Node, palette *Theme, state gfx.State) style.ButtonLook
	// TabsLookFn shares the badge look's signature: a tab strip has no state of its own.
	TabsLookFn     func(f *Frame, n *Node, palette *Theme, state gfx.State) style.ButtonLook
	SelectLookFn   func(f *Frame, n *Node, palette *Theme, state gfx.State, placeholder bool) (style.TextEntryLook, gfx.Color, gfx.Color)
	CheckboxLookFn func(f *Frame, n *Node, palette *Theme, state gfx.State) style.CheckboxLook
	SwitchLookFn   func(f *Frame, n *Node, palette *Theme, state gfx.State, progress float32) style.SwitchLook
	// ControlSurfaceRect gives where a themed control's own surface paints when that
	// is smaller than the element, as a checkbox's box or a switch's track are when
	// the element has a caption; nil uses the element's rectangle.
	ControlSurfaceRect func(f *Frame, n *Node) Rect
	SliderPointer      func(f *Frame, n *Node, s *SliderState)
	BuildDialog        func(f *Frame, spec DialogSpec) Box
	BuildPopover       func(f *Frame, anchor Box, spec PopoverSpec) Box
	DrawerBoxes        func(spec DrawerDeclaration, surface *gfx.Canvas, palette Theme, bounds Rect, layer uint16) DrawerRects
)

// DrawerRects is where a drawer's parts go.
type DrawerRects struct {
	Content, Body, Footer Rect
}

func (f *Frame) paintDialog(n *Node) {
	if PaintDialog != nil {
		PaintDialog(f, n)
	}
}

func (f *Frame) paintAlertDialog(n *Node) {
	if PaintAlertDialog != nil {
		PaintAlertDialog(f, n)
	}
}

func (f *Frame) paintTableRows(t *TableDecl, rows Rect, first, last int, top, contentWidth, rowHeight float32) {
	if PaintTableRows != nil {
		PaintTableRows(f, t, rows, first, last, top, contentWidth, rowHeight)
	}
}

func (f *Frame) paintTableHeaderRow(t *TableDecl, r Rect, headerHeight, rowHeight float32) {
	if PaintTableHeaderRow != nil {
		PaintTableHeaderRow(f, t, r, headerHeight, rowHeight)
	}
}

func (f *Frame) paintTableScrollbars(t *TableDecl, r, body Rect, content gfx.Size, offsetX, offsetY, frozenWidth float32) {
	if PaintTableScrollbars != nil {
		PaintTableScrollbars(f, t, r, body, content, offsetX, offsetY, frozenWidth)
	}
}

func (f *Frame) paintTabs(n *Node) {
	if PaintTabs != nil {
		PaintTabs(f, n)
	}
}

func (f *Frame) paintSelect(n *Node) {
	if PaintSelect != nil {
		PaintSelect(f, n)
	}
}

func (f *Frame) measureSelect(n *Node) {
	if MeasureSelect != nil {
		MeasureSelect(f, n)
	}
}

func (f *Frame) paintScrollbars(n *Node, s *ScrollState) {
	if PaintScrollbars != nil {
		PaintScrollbars(f, n, s)
	}
}

func (f *Frame) paintTooltip() {
	if PaintTooltip != nil {
		PaintTooltip(f)
	}
}

func (f *Frame) paintAddons(n *Node, snapshot InputSnapshot, a *FieldAddons) {
	if PaintAddons != nil {
		PaintAddons(f, n, snapshot, a)
	}
}

func (f *Frame) paintSwatch(n *Node, r Rect) {
	if PaintSwatch != nil {
		PaintSwatch(f, n, r)
	}
}

func (s *ColorPickerState) paint(d *DrawContext, c C, ID gfx.ID, look ColorPickerLook) {
	if PaintPickerPanel != nil {
		PaintPickerPanel(s, d, c, ID, look)
	}
}

func (s *ComboState) draw(d *DrawContext) {
	if PaintCombo != nil {
		PaintCombo(s, d)
	}
}

func (m *MenuItems) paintAt(surface *gfx.Canvas, palette *Theme, bounds Rect, layer uint16) {
	if PaintMenu != nil {
		PaintMenu(m, surface, palette, bounds, layer)
	}
}

func (f *Frame) Dialog(spec DialogSpec) Box {
	if BuildDialog != nil {
		return BuildDialog(f, spec)
	}
	return Box{}
}

func (f *Frame) Popover(anchor Box, spec PopoverSpec) Box {
	if BuildPopover != nil {
		return BuildPopover(f, anchor, spec)
	}
	return Box{}
}

// buttonLook is the look the theme's recipe gives the button element n in a state: the
// element's own ButtonRecipe, else the page's Styles.Button, else the theme's.
func (f *Frame) buttonLook(n *Node, palette *Theme, state gfx.State) style.ButtonLook {
	if ButtonLookFn != nil {
		return ButtonLookFn(f, n, palette, state)
	}
	return style.ButtonLook{}
}

// tabsLook is the look of the strip behind a tab list: its fill and radius.
func (f *Frame) tabsLook(n *Node, palette *Theme) style.ButtonLook {
	if TabsLookFn != nil {
		return TabsLookFn(f, n, palette, gfx.State{})
	}
	return style.ButtonLook{}
}

// toggleLook is the look the theme's recipe gives the toggle element n in a state.
func (f *Frame) toggleLook(n *Node, palette *Theme, state gfx.State) style.ButtonLook {
	if ToggleLookFn != nil {
		return ToggleLookFn(f, n, palette, state)
	}
	return style.ButtonLook{}
}

// badgeLook is the look the theme's recipe gives the badge element n in a state.
func (f *Frame) badgeLook(n *Node, palette *Theme, state gfx.State) style.ButtonLook {
	if BadgeLookFn != nil {
		return BadgeLookFn(f, n, palette, state)
	}
	return style.ButtonLook{}
}

func (f *Frame) TextEntryLook(palette *Theme, state gfx.State) style.TextEntryLook {
	if EntryLook != nil {
		return EntryLook(f, palette, state)
	}
	return style.TextEntryLook{}
}

func (m *MenuItems) rowExtent(i int) float32 {
	if MenuRowExtent != nil {
		return MenuRowExtent(m, i)
	}
	return 0
}

func (s *TreeState) row(i int, box Box) {
	if PaintTreeRow != nil {
		PaintTreeRow(s, i, box)
	}
}

func (s *TreeState) draw(d *DrawContext) {
	if TreeDraw != nil {
		TreeDraw(s, d)
	}
}

func (d *DrawerDeclaration) Boxes(surface *gfx.Canvas, palette Theme, bounds Rect, layer uint16) DrawerRects {
	if DrawerBoxes != nil {
		return DrawerBoxes(*d, surface, palette, bounds, layer)
	}
	return DrawerRects{}
}
