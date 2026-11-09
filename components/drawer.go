package components

import (
	"time"

	"github.com/vivaui/viva/gfx"
)

// DrawerSide selects the viewport edge occupied by a drawer.
type DrawerSide uint8

const (
	DrawerBottom DrawerSide = iota
	DrawerTop
	DrawerRight
	DrawerLeft
)

// DrawerOptions contains caller-owned modal identities and side geometry.
type DrawerOptions struct {
	Layer         uint16
	Scope         gfx.ID
	DismissID     gfx.ID
	CloseID       gfx.ID
	Side          DrawerSide
	ContentExtent float32
	// BodyHeight sizes a top or bottom drawer that has no ContentExtent to its
	// parts: the handle, the header, a body this tall, the footer and the gaps
	// between them, as `h-auto` fits a drawer to its content (never more than
	// 80% of the window). A side drawer fills the window's height and ignores it.
	BodyHeight float32
	// ButtonSize sizes the footer's buttons and the footer with them; the zero
	// value is ButtonSizeDefault.
	ButtonSize ButtonSize
	// InitialFocusID names the control that takes focus when the drawer opens.
	// Zero leaves focus where the runtime puts it, which is nowhere when the
	// opener is outside the new scope.
	InitialFocusID gfx.ID
	// HandleID names the strip along the panel's inner edge, around its handle
	// (see WithHandle), that a swipe begins on: the caller reads the pointer on it
	// with Canvas.PointerOn and feeds DrawerSwipe. Zero leaves the handle a picture.
	HandleID gfx.ID
	// Offset is how far a swipe has moved the panel toward its edge, as
	// DrawerSwipe.Offset reports it: Layout and Begin place every part that much
	// closer to closed, and the scrim fades with it.
	Offset float32
}

// DrawerSwipe is caller-owned drag state for a drawer that is swiped toward
// its edge to dismiss. The caller supplies one axis of the pointer: increasing
// toward the closed edge (down for a bottom drawer, up for a top one, right for
// a right drawer, left for a left one). Move clamps the travel to the open
// extent; End decides whether the release dismisses or snaps to one of the
// caller's snap fractions.
type DrawerSwipe struct {
	dragging bool
	start    float32
	offset   float32
}

// Begin starts a swipe at pointer.
func (swipe *DrawerSwipe) Begin(pointer float32) {
	swipe.dragging, swipe.start, swipe.offset = true, pointer, 0
}

// Move advances the swipe and returns how far it has travelled toward closed,
// clamped to [0, extent].
func (swipe *DrawerSwipe) Move(pointer, extent float32) float32 {
	if !swipe.dragging {
		return swipe.offset
	}
	swipe.offset = min(max(pointer-swipe.start, 0), max(extent, 0))
	return swipe.offset
}

// Offset is the current distance travelled toward closed.
func (swipe *DrawerSwipe) Offset() float32 { return swipe.offset }

// Dragging reports whether a swipe is in progress.
func (swipe *DrawerSwipe) Dragging() bool { return swipe.dragging }

// End finishes the swipe. velocity is the release speed toward closed, in
// pixels per second. It dismisses when the drawer travelled at least half its
// extent or was thrown faster than 600 px/s; otherwise it returns the nearest
// snap fraction (0 is the open edge) times extent. snapPoints are fractions in
// [0,1]; an empty slice snaps to the open or half-open position the drawer is
// already nearest.
func (swipe *DrawerSwipe) End(extent, velocity float32, snapPoints []float32) (offset float32, dismissed bool) {
	swipe.dragging = false
	if extent <= 0 {
		return 0, false
	}
	if swipe.offset >= extent/2 || velocity >= 600 {
		swipe.offset = extent
		return extent, true
	}
	best, bestDistance := float32(0), swipe.offset
	for _, point := range snapPoints {
		candidate := min(max(point, 0), 1) * extent
		if distance := absFloat32(candidate - swipe.offset); distance < bestDistance {
			best, bestDistance = candidate, distance
		}
	}
	swipe.offset = best
	return best, false
}

func absFloat32(value float32) float32 {
	if value < 0 {
		return -value
	}
	return value
}

// DrawerLayout exposes the drawer's content, handle, and body slots.
type DrawerLayout struct {
	Content gfx.Rect
	Handle  gfx.Rect
	Header  gfx.Rect
	Body    gfx.Rect
	Footer  gfx.Rect
	Close   gfx.Rect
}

// Drawer is a modal edge surface. Drag state and snap-point policy remain
// caller-owned until pointer capture is available in the runtime.
type Drawer struct {
	canvas     *gfx.Canvas
	theme      gfx.Theme
	viewport   gfx.Rect
	options    DrawerOptions
	disabled   bool
	showHandle bool
}

// BeginDrawer creates a drawer declaration.
func BeginDrawer(canvas *gfx.Canvas, theme gfx.Theme, viewport gfx.Rect, options DrawerOptions) Drawer {
	return Drawer{canvas: canvas, theme: theme, viewport: viewport, options: options}
}

// WithHandle enables the explicit swipe-handle visual used by Base UI.
func (drawer Drawer) WithHandle() Drawer {
	drawer.showHandle = true
	return drawer
}

// Actions paints the footer's buttons against its right edge, in the order
// given, so the last one sits at the corner (see ButtonRow). A disabled drawer
// makes every action inert. Call it between Begin and End.
func (drawer Drawer) Actions(layout DrawerLayout, actions ...ButtonAction) {
	overlayActions(drawer.canvas, drawer.theme, layout.Footer, drawer.options.ButtonSize, drawer.disabled, actions)
}

// Disabled keeps dismiss and close semantics visible while making them inert.
func (drawer Drawer) Disabled() Drawer {
	drawer.disabled = true
	return drawer
}

// Paint paints a complete drawer and closes its modal layer.
func (drawer Drawer) Paint(title, description string) DrawerLayout {
	layout := drawer.Begin(title, description)
	drawer.End()
	return layout
}

// Begin paints the drawer and leaves its layer/scope active for body UI.
func (drawer Drawer) Begin(title, description string) DrawerLayout {
	if drawer.canvas == nil || drawer.viewport.Width <= 0 || drawer.viewport.Height <= 0 {
		return DrawerLayout{}
	}
	drawer.canvas.PushLayer(drawer.options.Layer)
	if !drawer.options.Scope.IsZero() {
		drawer.canvas.PushFocusScope(drawer.options.Scope)
	}
	key := overlayKey(drawer.options.DismissID, drawer.options.CloseID, drawer.options.Scope)
	metrics := drawer.theme.Metrics
	var scratch overlayLines
	layout, lines := drawer.layout(&scratch, description)
	content := layout.Content
	// The scrim fades as a swipe takes the panel toward its edge.
	drawer.canvas.BeginPresence(key, overlayBackdropPart, scrimPresence(drawer.theme, drawerDuration, gfx.EaseCSS), centerOf(drawer.viewport))
	scrim := overlayBackdrop(drawer.theme)
	if extent := drawer.extent(content); drawer.options.Offset > 0 && extent > 0 {
		scrim.Alpha *= max(1-drawer.options.Offset/extent, 0)
	}
	drawer.canvas.Fill(drawer.viewport, scrim)
	drawer.canvas.EndPresence()
	if !drawer.options.DismissID.IsZero() {
		drawer.canvas.HitWithOptions(drawer.options.DismissID, drawer.viewport, "Dismiss drawer", gfx.HitOptions{Role: gfx.RoleButton, Disabled: drawer.disabled, SkipFocus: true, Backdrop: true})
	}
	look := dialogRecipe(&drawer.theme)
	radius := look.Radius
	drawer.canvas.BeginPresence(key, overlaySurfacePart, drawer.presence(content), centerOf(content))
	paintPanel(drawer.canvas, content, look)
	drawer.squareViewportEdge(content, radius)
	// The panel is opaque to pointers: pressing it must not reach the dismiss
	// target behind it.
	drawer.canvas.Block(content)
	if drawer.showHandle {
		drawer.canvas.Rounded(layout.Handle, min(layout.Handle.Width, layout.Handle.Height)/2, drawer.theme.Muted)
		if id := drawer.options.HandleID; !id.IsZero() {
			drawer.canvas.HitWithOptions(id, drawer.handleStrip(content), "Drag to dismiss", gfx.HitOptions{
				Role: gfx.RoleButton, Disabled: drawer.disabled, SkipFocus: true,
			})
		}
	}
	inner := layout.Header
	titleHeight := overlayLineHeight(metrics)
	drawer.canvas.Label(title, gfx.Rect{X: inner.X, Y: inner.Y, Width: inner.Width, Height: titleHeight}, gfx.TextStyle{Size: overlayTitleSize(metrics), Weight: gfx.WeightMedium, Color: drawer.theme.Foreground})
	overlayParagraph(drawer.canvas, lines, gfx.Rect{X: inner.X, Y: inner.Y + titleHeight + drawerTitleGap, Width: inner.Width}, overlayLineHeight(metrics), gfx.TextStyle{
		Size: metrics.FontSizeM, Weight: gfx.WeightRegular, Color: drawer.theme.MutedForeground,
	}, false)
	if !drawer.options.CloseID.IsZero() {
		drawer.paintClose(layout.Close)
	}
	return layout
}

// Layout is where the parts of the drawer go for its viewport, without painting
// anything: Begin paints at exactly these rectangles. The description wraps to
// the drawer's width, so the header grows with its lines; without a canvas to
// measure them it counts as one line.
func (drawer Drawer) Layout(description string) DrawerLayout {
	var scratch overlayLines
	layout, _ := drawer.layout(&scratch, description)
	return layout
}

func (drawer Drawer) layout(scratch *overlayLines, description string) (DrawerLayout, []string) {
	if drawer.viewport.Width <= 0 || drawer.viewport.Height <= 0 {
		return DrawerLayout{}, nil
	}
	metrics := drawer.theme.Metrics
	padding := metrics.PadSurface
	lines := overlayWrap(drawer.canvas, scratch, description, metrics.FontSizeM, gfx.WeightRegular, drawer.contentWidth()-2*padding)
	headerHeight := drawerHeaderHeight(metrics, len(lines))
	content := drawer.contentBounds(headerHeight)
	inner := content.Inset(gfx.All(padding))
	layout := DrawerLayout{Content: content}
	if drawer.showHandle {
		layout.Handle = drawer.handleBounds(content)
		if drawer.options.Side == DrawerTop || drawer.options.Side == DrawerBottom {
			inner.Y += drawerHandleReach(metrics)
			inner.Height = max(0, inner.Height-drawerHandleReach(metrics))
		} else {
			inner.X += drawerHandleReach(metrics)
			inner.Width = max(0, inner.Width-drawerHandleReach(metrics))
		}
	}
	footerHeight := dialogFooterHeight(metrics, drawer.options.ButtonSize)
	layout.Header = gfx.Rect{X: inner.X, Y: inner.Y, Width: inner.Width, Height: headerHeight}
	layout.Footer = gfx.Rect{X: inner.X, Y: inner.Y + inner.Height - footerHeight, Width: inner.Width, Height: footerHeight}
	layout.Body = gfx.Rect{X: inner.X, Y: layout.Header.Y + layout.Header.Height + metrics.GapItem, Width: inner.Width, Height: max(0, layout.Footer.Y-layout.Header.Y-layout.Header.Height-metrics.GapItem*2)}
	if !drawer.options.CloseID.IsZero() {
		closeSize := metrics.ControlHeight
		layout.Close = gfx.Rect{X: content.X + content.Width - padding - closeSize, Y: content.Y + padding, Width: closeSize, Height: closeSize}
	}
	return layout, lines
}

// The swipe handle is 100x6 (`h-1.5 w-[100px]`) and sits 16px (`mt-4`) in from the
// drawer's edge; the header starts a padding (16px) under it.
const (
	drawerTitleGap   = float32(6)
	drawerHandleLong = float32(100)
	drawerHandleThin = float32(6)
)

// drawerHandleReach is how far the handle pushes the content in: its margin, its
// thickness and the header's own padding, less the padding the content already
// has.
func drawerHandleReach(metrics gfx.Metrics) float32 {
	return metrics.PadSurface + drawerHandleThin
}

// drawerHeaderHeight is the title and, under it, the description's lines.
func drawerHeaderHeight(metrics gfx.Metrics, descriptionLines int) float32 {
	height := overlayLineHeight(metrics)
	if descriptionLines > 0 {
		height += drawerTitleGap + float32(descriptionLines)*overlayLineHeight(metrics)
	}
	return height
}

// presence is how the panel comes and goes: it slides in from beyond its edge
// without fading, over Base UI's 450ms (its closed transform is the panel's own
// size plus 2px). EaseCSS starts and lands softly; the emphasized curve moved a
// sixth of the panel in the first 60 Hz frame, which reads as a jolt rather than
// a slide.
func (drawer Drawer) presence(content gfx.Rect) gfx.Presence {
	if !moving(drawer.theme) {
		return gfx.Presence{}
	}
	slide := gfx.Point{Y: content.Height + 2}
	switch drawer.options.Side {
	case DrawerTop:
		slide = gfx.Point{Y: -(content.Height + 2)}
	case DrawerRight:
		slide = gfx.Point{X: content.Width + 2}
	case DrawerLeft:
		slide = gfx.Point{X: -(content.Width + 2)}
	}
	return gfx.Presence{Motion: gfx.Motion{Duration: drawerDuration, Easing: gfx.EaseCSS}, SlideIn: slide, SlideOut: slide}
}

// End closes the layer and installs modal focus and Escape routing. It also ends
// what the drawer animates in and out.
func (drawer Drawer) End() {
	if drawer.canvas == nil || drawer.viewport.Width <= 0 || drawer.viewport.Height <= 0 {
		return
	}
	drawer.canvas.EndPresence()
	// Claim while the drawer's own scope is still pushed, so a popup painted
	// deeper inside it keeps its own trap and Escape target.
	if !drawer.options.Scope.IsZero() {
		drawer.canvas.TrapFocus(drawer.options.Scope)
		if !drawer.disabled {
			drawer.canvas.SetInitialFocus(drawer.options.InitialFocusID)
		}
	}
	escapeTarget := drawer.options.DismissID
	if escapeTarget.IsZero() {
		escapeTarget = drawer.options.CloseID
	}
	if drawer.disabled {
		escapeTarget = gfx.ID{}
	}
	drawer.canvas.SetEscapeTarget(escapeTarget)
	if !drawer.options.Scope.IsZero() {
		drawer.canvas.PopFocusScope()
	}
	drawer.canvas.PopLayer()
}

// contentWidth is the width the drawer's text lays out in: the window's for a top
// or bottom drawer, the panel's for a side drawer.
func (drawer Drawer) contentWidth() float32 {
	if drawer.options.Side == DrawerTop || drawer.options.Side == DrawerBottom {
		return drawer.viewport.Width
	}
	extent := drawer.options.ContentExtent
	if extent <= 0 {
		extent = min(drawer.viewport.Width*0.75, 384)
	}
	return min(extent, drawer.viewport.Width)
}

func (drawer Drawer) contentBounds(headerHeight float32) gfx.Rect {
	extent := drawer.options.ContentExtent
	if drawer.options.Side == DrawerTop || drawer.options.Side == DrawerBottom {
		if extent <= 0 {
			metrics := drawer.theme.Metrics
			extent = 2*metrics.PadSurface + headerHeight + dialogFooterHeight(metrics, drawer.options.ButtonSize) + max(drawer.options.BodyHeight, 0) + 2*metrics.GapItem
			if drawer.showHandle {
				extent += drawerHandleReach(metrics)
			}
		}
		extent = min(extent, drawer.viewport.Height*0.80)
		y := drawer.viewport.Y
		if drawer.options.Side == DrawerBottom {
			y = drawer.viewport.Y + drawer.viewport.Height - extent
		}
		return gfx.Rect{X: drawer.viewport.X, Y: y, Width: drawer.viewport.Width, Height: extent}.Offset(drawer.swipeShift())
	}
	extent = drawer.contentWidth()
	x := drawer.viewport.X
	if drawer.options.Side == DrawerRight {
		x = drawer.viewport.X + drawer.viewport.Width - extent
	}
	return gfx.Rect{X: x, Y: drawer.viewport.Y, Width: extent, Height: drawer.viewport.Height}.Offset(drawer.swipeShift())
}

// swipeShift is how far Offset moves the panel, toward its edge.
func (drawer Drawer) swipeShift() (dx, dy float32) {
	offset := max(drawer.options.Offset, 0)
	switch drawer.options.Side {
	case DrawerTop:
		return 0, -offset
	case DrawerRight:
		return offset, 0
	case DrawerLeft:
		return -offset, 0
	}
	return 0, offset
}

func (drawer Drawer) squareViewportEdge(content gfx.Rect, radius float32) {
	switch drawer.options.Side {
	case DrawerTop:
		drawer.canvas.Fill(gfx.Rect{X: content.X, Y: content.Y, Width: content.Width, Height: radius}, drawer.theme.Popover)
	case DrawerRight:
		drawer.canvas.Fill(gfx.Rect{X: content.X + content.Width - radius, Y: content.Y, Width: radius, Height: content.Height}, drawer.theme.Popover)
	case DrawerLeft:
		drawer.canvas.Fill(gfx.Rect{X: content.X, Y: content.Y, Width: radius, Height: content.Height}, drawer.theme.Popover)
	default:
		drawer.canvas.Fill(gfx.Rect{X: content.X, Y: content.Y + content.Height - radius, Width: content.Width, Height: radius}, drawer.theme.Popover)
	}
}

// extent is how far the panel reaches from its edge: how far a swipe takes it to
// close it.
func (drawer Drawer) extent(content gfx.Rect) float32 {
	if drawer.options.Side == DrawerTop || drawer.options.Side == DrawerBottom {
		return content.Height
	}
	return content.Width
}

// handleStrip is where a swipe begins: the band across the panel that holds its
// handle (see handleBounds), with the handle's margin on both sides of it.
func (drawer Drawer) handleStrip(content gfx.Rect) gfx.Rect {
	reach := drawer.theme.Metrics.PadSurface + drawerHandleReach(drawer.theme.Metrics)
	if drawer.options.Side == DrawerLeft || drawer.options.Side == DrawerRight {
		return gfx.Rect{X: content.X, Y: content.Y, Width: reach, Height: content.Height}
	}
	return gfx.Rect{X: content.X, Y: content.Y, Width: content.Width, Height: reach}
}

func (drawer Drawer) handleBounds(content gfx.Rect) gfx.Rect {
	margin := drawer.theme.Metrics.PadSurface
	if drawer.options.Side == DrawerLeft || drawer.options.Side == DrawerRight {
		return gfx.Rect{X: content.X + margin, Y: content.Y + (content.Height-drawerHandleLong)/2, Width: drawerHandleThin, Height: drawerHandleLong}
	}
	return gfx.Rect{X: content.X + (content.Width-drawerHandleLong)/2, Y: content.Y + margin, Width: drawerHandleLong, Height: drawerHandleThin}
}

func (drawer Drawer) paintClose(bounds gfx.Rect) {
	fill := drawer.theme.Background
	fill.Alpha = 0
	foreground := drawer.theme.MutedForeground
	if !drawer.disabled && drawer.canvas.Hovered() == drawer.options.CloseID {
		fill = drawer.theme.Accent
		foreground = drawer.theme.AccentForeground
	}
	if drawer.disabled {
		foreground.Alpha *= 0.50
	}
	drawer.canvas.Rounded(bounds, drawer.theme.Metrics.RadiusControl, fill)
	paintCross(drawer.canvas, bounds, 16, foreground)
	drawer.canvas.HitWithOptions(drawer.options.CloseID, bounds, "Close drawer", gfx.HitOptions{Role: gfx.RoleButton, Disabled: drawer.disabled})
}

// drawerDuration is the 450ms of Base UI's drawer transition.
const drawerDuration = 450 * time.Millisecond
