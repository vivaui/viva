// Package hovercard is the opt-in page hover card: a non-modal preview
// that appears beside the anchor the pointer rests on, stays open while the
// pointer moves into it, and closes after the pointer leaves.
//
// It is a page control, not a canvas recipe. The anchor is an element the
// caller already declared, the title, description and body are page children,
// and the surface is painted by components.BeginHoverCard. The view keeps its
// hover timing in page-owned state, so the declaration can be a fresh value on
// every paint:
//
//	anchor := ui.Button(c, "@viva-framework").Outline().Width(180)
//	hovercard.View(c, "@viva-framework", &hovercard.Card{
//		Anchor:      anchor,
//		Title:       "Viva Framework",
//		Description: "Go-first, GPU-native desktop UI.",
//		Body: func() {
//			ui.Text(c, "Native, retained and allocation-aware").Muted()
//		},
//	})
//
// The zero value opens after DefaultOpenDelay and closes after
// DefaultCloseDelay; Delays selects another timing, including zero. The view
// asks the canvas for a frame only while a delay is running, so an open idle
// card schedules no work.
//
// It is a separate package so an application that does not import it pays
// nothing for it.
package hovercard

import (
	"time"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// Side is the edge of the anchor the card opens on. It aliases the component's
// vocabulary so a page that declares a card reads through this package alone.
type Side = components.HoverCardSide

// The anchor edges a card opens on.
const (
	Bottom = components.HoverCardBottom
	Top    = components.HoverCardTop
	Right  = components.HoverCardRight
	Left   = components.HoverCardLeft
)

// Align positions the card along the anchor's cross axis.
type Align = components.HoverCardAlign

// The alignments along the anchor's cross axis.
const (
	Center = components.HoverCardAlignCenter
	Start  = components.HoverCardAlignStart
	End    = components.HoverCardAlignEnd
)

// The default timing and size of a card.
const (
	// DefaultOpenDelay is how long the pointer rests on the anchor before the
	// card opens.
	DefaultOpenDelay = 700 * time.Millisecond
	// DefaultCloseDelay is how long the card waits after the pointer leaves the
	// anchor and the card before it closes, so moving from one to the other
	// keeps it open.
	DefaultCloseDelay = 300 * time.Millisecond
	// DefaultWidth is the card's width when Width is not set.
	DefaultWidth = float32(256)
	// DefaultSideOffset is the gap between the anchor and the card.
	DefaultSideOffset = float32(4)
	// surfaceMargin is the space the card keeps from the viewport's edges.
	surfaceMargin = float32(8)
)

// Card is one page hover card. Declare it where the anchor is declared;
// the view keeps the hover state in the page, so a fresh value each paint is
// the normal call site. Keep Open in the application's model when the
// application needs to read or set the open state itself.
type Card struct {
	// Anchor is the element the card describes, declared before View in the
	// same paint. The card uses that element's laid-out geometry.
	Anchor ui.Box
	// Title and Description are the card's heading text. They are page children
	// and stay visible in the declaration.
	Title, Description string
	// Body declares the card's content, as any container's children. It is nil
	// for a card with only a heading.
	Body func()
	// Open, when non-nil, is the caller's copy of the card's open state. View
	// writes it when hover opens and closes the card, so an application that
	// wants to read the state holds only this value. The hover timing stays in
	// the view; the value is not a second source of truth.
	Open *bool
	// OnOpenChange is called when the view opens or closes the card because of
	// the pointer. It is not called for a caller's own write to Open. It runs
	// during the paint, so it must not declare elements.
	OnOpenChange func(open bool)

	side      Side
	align     Align
	width     float32
	openDelay time.Duration
	closeWait time.Duration
	delays    bool
	disabled  bool
}

// Side selects the anchor edge the card opens on.
func (card *Card) Side(side Side) *Card { card.side = side; return card }

// Align lines the card up with the anchor along its cross axis.
func (card *Card) Align(align Align) *Card { card.align = align; return card }

// Width fixes the card's width; zero keeps DefaultWidth.
func (card *Card) Width(width float32) *Card { card.width = width; return card }

// Delays selects how long the pointer rests before the card opens, and how long
// it waits after leaving before the card closes. Either may be zero.
func (card *Card) Delays(open, close time.Duration) *Card {
	card.openDelay, card.closeWait, card.delays = open, close, true
	return card
}

// Disabled suppresses the card for a disabled anchor. Disabled() is Disabled(true).
func (card *Card) Disabled(disabled ...bool) *Card {
	card.disabled = len(disabled) == 0 || disabled[0]
	return card
}

// hoverState is the page-owned state of one declaration. It holds the timing,
// the last card rectangle and the stable declaimers, so a warm paint allocates
// nothing for the package.
type hoverState struct {
	open         bool
	pendingOpen  bool
	pendingClose bool
	since        time.Duration
	contentRect  gfx.Rect

	c           ui.C
	title       string
	description string
	body        func()
	children    func()
	paintCard   func(*ui.DrawContext)
}

// View declares the hover card at c and returns the anchor element. label names
// the anchor for assistive technology; the card announces Title when it has
// one. The anchor must be declared in the same paint and is left as declared
// unless it has no identity, in which case View gives it one for hover
// detection.
func View(c ui.C, label string, card *Card) ui.Box {
	if card == nil {
		panic("hovercard: View needs a Card")
	}
	anchor := card.Anchor
	if !anchor.Valid() {
		panic("hovercard: Card.Anchor must be declared before View")
	}
	canvas := c.Canvas()
	state := ui.State[hoverState](c, "hovercard")
	if state.children == nil {
		// One declaimer for the life of this declaration: a warm paint reuses
		// the same functions, as the other page views do.
		state.children = func() { state.declareChildren() }
		state.paintCard = func(d *ui.DrawContext) { state.paintSurface(d) }
	}

	anchorID := anchor.N().ID
	contentID := ui.ID(c)
	if anchorID.IsZero() {
		anchorID = ui.ID(c)
		anchor = anchor.Hit(anchorID, label)
	}

	if card.disabled {
		state.pendingOpen, state.pendingClose = false, false
		return anchor
	}

	page := c.GetPage()
	pointerIn, pointer := page.GetPointerIn(), page.GetPointer()
	onAnchor := canvas.Hovered() == anchorID || pointerIn && hoveredWithin(canvas, anchorID, pointer)
	open := state.isOpen(card)
	onContent := open && (canvas.Hovered() == contentID || pointerIn && contains(state.contentRect, pointer))
	state.settle(canvas, card, canvas.Now(), onAnchor, onContent)

	if !state.isOpen(card) {
		return anchor
	}
	state.declare(c, card, label, contentID, anchor)
	return anchor
}

// isOpen is the card's open state: the caller's when it owns one, else the
// view's own.
func (state *hoverState) isOpen(card *Card) bool {
	if card.Open != nil {
		return *card.Open
	}
	return state.open
}

// setOpen changes the open state and reports it to the caller.
func (state *hoverState) setOpen(card *Card, open bool) {
	if state.isOpen(card) == open {
		return
	}
	state.open = open
	if card.Open != nil {
		*card.Open = open
	}
	if card.OnOpenChange != nil {
		card.OnOpenChange(open)
	}
}

// settle advances the hover timing. A remaining wait asks the canvas for one
// frame at the deadline and for nothing while idle.
func (state *hoverState) settle(canvas *gfx.Canvas, card *Card, now time.Duration, onAnchor, onContent bool) {
	open := state.isOpen(card)
	if onAnchor || open && onContent {
		state.pendingClose = false
		if open {
			state.pendingOpen = false
			return
		}
		if !state.pendingOpen {
			state.pendingOpen, state.since = true, now
		}
		if wait := card.openWait() - (now - state.since); wait > 0 {
			canvas.RequestFrameIn(wait)
			return
		}
		state.pendingOpen = false
		state.setOpen(card, true)
		return
	}
	state.pendingOpen = false
	if !open {
		state.pendingClose = false
		return
	}
	if !state.pendingClose {
		state.pendingClose, state.since = true, now
	}
	if wait := card.closeDelay() - (now - state.since); wait > 0 {
		canvas.RequestFrameIn(wait)
		return
	}
	state.pendingClose = false
	state.setOpen(card, false)
	state.contentRect = gfx.Rect{}
}

func (card *Card) openWait() time.Duration {
	if !card.delays {
		return DefaultOpenDelay
	}
	return max(card.openDelay, 0)
}

func (card *Card) closeDelay() time.Duration {
	if !card.delays {
		return DefaultCloseDelay
	}
	return max(card.closeWait, 0)
}

// declare opens the page popup the card lives in. The popup places its children
// from the caller's anchor; the card's surface is painted in the popup's place
// by the component.
func (state *hoverState) declare(c ui.C, card *Card, label string, contentID gfx.ID, anchor ui.Box) {
	frame := c.GetF()
	metrics := c.Theme().Metrics
	popup := frame.Popup()
	align := ui.Center
	switch card.align {
	case Start:
		align = ui.Start
	case End:
		align = ui.End
	}
	switch card.side {
	case Top:
		popup.AboveBox(anchor, DefaultSideOffset, align)
	case Right:
		popup.BesideBox(anchor, DefaultSideOffset, align)
	case Left:
		popup.BesideLeftBox(anchor, DefaultSideOffset, align)
	default:
		popup.BelowBox(anchor, DefaultSideOffset, align)
	}
	width := card.width
	if width <= 0 {
		width = DefaultWidth
	}
	popup.Width(width).Pad(metrics.PadSurface).Gap(metrics.GapItem)
	popup.Within(c.Bounds().Inset(gfx.All(surfaceMargin)))
	name := label
	if card.Title != "" {
		name = card.Title
	}
	popup.HitWith(contentID, name, gfx.HitOptions{Role: gfx.RoleGroup, SkipFocus: true})
	state.c, state.title, state.description, state.body = c, card.Title, card.Description, card.Body
	popup.Draw(state.paintCard)
	popup.Children(state.children)
}

// paintSurface paints the card surface through components.BeginHoverCard at the
// popup's settled rectangle. A local viewport makes the component's placement
// resolve to exactly that rectangle; the caller's anchor already placed the
// popup, so only the component's surface paint is used here.
func (state *hoverState) paintSurface(d *ui.DrawContext) {
	state.contentRect = d.Rect
	if d.Rect.Width <= 0 || d.Rect.Height <= 0 {
		return
	}
	pad := float32(8)
	viewport := gfx.Rect{
		X: d.Rect.X - pad, Y: d.Rect.Y - pad,
		Width: d.Rect.Width + 2*pad, Height: d.Rect.Height + 2*pad,
	}
	components.BeginHoverCard(d.Canvas, *d.Theme, viewport, gfx.Rect{X: viewport.X, Y: viewport.Y}, components.HoverCardOptions{
		ContentSize: gfx.Size{Width: d.Rect.Width, Height: d.Rect.Height},
	}).Paint("", "")
}

// declareChildren declares the card's content as page children, in the popup
// the view opened.
func (state *hoverState) declareChildren() {
	c := state.c
	if state.title != "" {
		ui.Text(c, state.title).Weight(gfx.WeightMedium).Color(c.Theme().PopoverForeground)
	}
	if state.description != "" {
		ui.Wrap(c, state.description).Muted()
	}
	if state.body != nil {
		state.body()
	}
}

func contains(r gfx.Rect, p gfx.Point) bool {
	return r.Width > 0 && r.Height > 0 && p.X >= r.X && p.Y >= r.Y && p.X < r.X+r.Width && p.Y < r.Y+r.Height
}

// hoveredWithin reports whether point is inside the region the current paint
// declared for id. It covers an anchor whose own hits were split among its
// children, where Canvas.Hovered names a child rather than the anchor.
func hoveredWithin(canvas *gfx.Canvas, id gfx.ID, point gfx.Point) bool {
	bounds, ok := canvas.HitBounds(id)
	return ok && contains(bounds, point)
}
