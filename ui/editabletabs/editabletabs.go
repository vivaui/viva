// Package editabletabs is the opt-in page control for a strip of tabs the user
// can close and add, as the tabs of an editor or a terminal are: each tab is as
// wide as its title, with a close button, a button after the last tab adds one,
// and a strip longer than its room scrolls. A page keeps the tabs in its model and
// the control beside them:
//
//	m.strip = editabletabs.New()
//	// in the page:
//	m.strip.View(c, "Sessions", &m.active, editabletabs.Spec{Tabs: m.tabs, Addable: true}).FillWidth()
//	if i, ok := m.strip.Closed(); ok {
//		m.closeTab(i)
//	}
//	if m.strip.Added() {
//		m.openTab()
//	}
//
// The control only reports: it never removes or appends a tab, because the tabs
// are the page's. Active is the index of the active tab; a click, an arrow key or
// the close of a tab before it moves it, and the page keeps it in range as tabs
// come and go. What the active tab shows is the page's, declared after the strip.
//
// The strip is one tab stop. Left and Right move to the previous and next tab and
// make it active, Home and End to the first and last, Delete or Backspace closes
// the tab the focus is on, and Enter or Space on the add button adds a tab. A
// middle click closes a tab, as in an editor. A Pinned tab has no close button and
// is not closed by the keys. The wheel scrolls a strip that is longer than its
// room, and the active tab is kept in view.
//
// It is a separate package so an application that does not import it pays
// nothing for it. Keep an EditableTabs in the model: its handlers and draw
// function are made once, so a warm paint allocates nothing. Do not copy an
// EditableTabs after it has been declared.
package editabletabs

import (
	"math"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var (
	closeIcon = gfx.Icon{ViewBox: 24, Stroke: 2.5, Cap: gfx.CapRound, Join: gfx.JoinRound, Commands: []gfx.PathCommand{
		gfx.MoveTo(18, 6), gfx.LineTo(6, 18), gfx.MoveTo(6, 6), gfx.LineTo(18, 18),
	}}
	plusIcon = gfx.Icon{ViewBox: 24, Stroke: 2, Cap: gfx.CapRound, Join: gfx.JoinRound, Commands: []gfx.PathCommand{
		gfx.MoveTo(5, 12), gfx.LineTo(19, 12), gfx.MoveTo(12, 5), gfx.LineTo(12, 19),
	}}
)

// Tab is one tab of the strip.
type Tab struct {
	// Title names the tab.
	Title string
	// Icon is drawn before the title. A zero icon draws none.
	Icon gfx.Icon
	// Pinned takes away the close button, and the keys that close.
	Pinned bool
	// Disabled dims the tab and takes no press.
	Disabled bool
}

// Spec declares the strip.
type Spec struct {
	// Tabs are the tabs, in order. Keep the slice in the model: a slice made in each
	// paint allocates in each paint.
	Tabs []Tab
	// Addable adds the button after the last tab.
	Addable bool
	// CloseLabel and AddLabel are the accessible names of the buttons, "Close" and
	// "Add tab" when empty.
	CloseLabel, AddLabel string
}

// Layout constants, in pixels.
const (
	height  = 36
	padX    = 12
	gap     = 6
	closeSz = 18
	addSz   = 28
)

// EditableTabs is one strip and what the user did to it.
type EditableTabs struct {
	handlers ui.Handlers
	drawFn   func(*ui.DrawContext)

	spec    Spec
	active  *int
	offset  float32 // how far the strip is scrolled
	revealI int     // the tab to bring into view at the next draw, or -1
	seen    int     // the active tab as of the last View: a change the page makes is revealed too
	focus   int     // the tab the keyboard rests on

	hoverPart int // the part under the pointer, or -1

	closed          int // 1 + the tab closed since the last View, or 0
	closedNow       int
	added, addedNow bool
	width           float32 // the strip's width as last drawn
}

// New returns a strip.
func New() *EditableTabs {
	t := &EditableTabs{revealI: -1, seen: -1, hoverPart: -1}
	t.handlers.Role = gfx.RoleGroup
	t.handlers.Key = t.key
	t.handlers.Pointer = t.pointer
	t.handlers.Scroll = t.scroll
	t.drawFn = t.draw
	return t
}

// Closed reports which tab the user closed in the paint that delivered it.
func (t *EditableTabs) Closed() (int, bool) { return t.closed - 1, t.closed > 0 }

// Added reports whether the user pressed the add button in the paint that delivered it.
func (t *EditableTabs) Added() bool { return t.added }

// parts: tab i is 3i, its close button 3i+1, and the add button 3n.
func tabPart(i int) int   { return 3 * i }
func closePart(i int) int { return 3*i + 1 }

func (t *EditableTabs) closable(i int) bool {
	return i >= 0 && i < len(t.spec.Tabs) && !t.spec.Tabs[i].Pinned && !t.spec.Tabs[i].Disabled
}

// View declares the strip at c, showing *active as the active tab. label names the
// strip for assistive technology. The strip is one line high; give it FillWidth.
// Declare the view on its own, not inside a Memo.
func (t *EditableTabs) View(c ui.C, label string, active *int, spec Spec) ui.Box {
	if t == nil {
		panic("editabletabs: View needs its EditableTabs")
	}
	if active == nil {
		panic("editabletabs: View needs the active tab")
	}
	t.closed, t.added = t.closedNow, t.addedNow
	t.closedNow, t.addedNow = 0, false
	if spec.CloseLabel == "" {
		spec.CloseLabel = "Close"
	}
	if spec.AddLabel == "" {
		spec.AddLabel = "Add tab"
	}
	t.spec, t.active = spec, active
	if len(spec.Tabs) > 0 {
		*active = min(max(*active, 0), len(spec.Tabs)-1)
	} else {
		*active = 0
	}
	t.focus = *active
	if *active != t.seen {
		t.seen, t.revealI = *active, *active
	}
	return ui.Control(c, label, &t.handlers).Height(height).Shrink(0).Draw(t.drawFn)
}

func (t *EditableTabs) focusInside() bool {
	return t.focus >= 0 && t.focus < len(t.spec.Tabs) && t.hoverPart >= -1
}

// widths are the x of each tab's left edge and the right edge of the last, in the
// strip's own coordinates before scrolling; add is where the add button starts.
func (t *EditableTabs) widthOf(canvas *gfx.Canvas, theme *gfx.Theme, tab Tab) float32 {
	w := canvas.TextWidthWeight(tab.Title, theme.Metrics.FontSizeM, gfx.WeightMedium) + 2*padX
	if tab.Icon.ViewBox > 0 {
		w += theme.Metrics.FontSizeM + gap
	}
	if !tab.Pinned {
		w += closeSz + gap/2
	}
	// Whole pixels, so the edges of neighbors are the same numbers however far the
	// strip is scrolled: tabs share an edge and do not overlap by rounding.
	return float32(math.Ceil(float64(w)))
}

// layout calls fn with the rectangle of each tab, scrolled, and returns the total
// width of the tabs and the add button.
func (t *EditableTabs) layout(canvas *gfx.Canvas, theme *gfx.Theme, origin gfx.Rect, fn func(i int, r gfx.Rect)) float32 {
	x := float32(0)
	for i, tab := range t.spec.Tabs {
		w := t.widthOf(canvas, theme, tab)
		if fn != nil {
			fn(i, gfx.Rect{X: origin.X + x - t.offset, Y: origin.Y, Width: w, Height: origin.Height})
		}
		x += w
	}
	if t.spec.Addable {
		x += gap + addSz
	}
	return x
}

func (t *EditableTabs) limit(total float32) float32 { return max(total-t.width, 0) }

func (t *EditableTabs) scroll(event gfx.ScrollEvent) gfx.EventResult {
	delta := event.DeltaX
	if math.Abs(float64(event.DeltaY)) > math.Abs(float64(event.DeltaX)) {
		delta = event.DeltaY
	}
	if event.Pages != 0 {
		delta = event.StepXIn(1, t.width)
		if math.Abs(float64(event.DeltaY)) > math.Abs(float64(event.DeltaX)) {
			delta = event.StepYIn(1, t.width)
		}
	}
	if delta == 0 {
		return gfx.EventResult{}
	}
	before := t.offset
	t.offset = max(t.offset-delta, 0)
	return gfx.EventResult{Handled: t.offset != before, Repaint: t.offset != before}
}

func (t *EditableTabs) closeTab(i int) gfx.EventResult {
	if !t.closable(i) {
		return gfx.EventResult{}
	}
	t.closedNow = i + 1
	return gfx.EventResult{Handled: true, Repaint: true}
}

func (t *EditableTabs) pointer(event gfx.PointerEvent) gfx.EventResult {
	if t.active == nil {
		return gfx.EventResult{}
	}
	parts := t.handlers.Parts()
	part, on := event.Target.In(parts)
	switch event.Kind {
	case gfx.PointerExit:
		if t.hoverPart == -1 {
			return gfx.EventResult{}
		}
		t.hoverPart = -1
		return gfx.EventResult{Repaint: true}
	case gfx.PointerMove:
		hover := -1
		if on {
			hover = part
		}
		if hover == t.hoverPart {
			return gfx.EventResult{}
		}
		t.hoverPart = hover
		return gfx.EventResult{Repaint: true}
	case gfx.PointerDown:
		if !on {
			return gfx.EventResult{}
		}
		n := len(t.spec.Tabs)
		switch {
		case part == 3*n && t.spec.Addable:
			if event.Button == gfx.PointerButtonPrimary {
				t.addedNow = true
				return gfx.EventResult{Handled: true, Repaint: true}
			}
		case part%3 == 1:
			if event.Button == gfx.PointerButtonPrimary {
				return t.closeTab(part / 3)
			}
		case part%3 == 0 && part/3 < n && !t.spec.Tabs[part/3].Disabled:
			i := part / 3
			if event.Button == gfx.PointerButtonMiddle {
				return t.closeTab(i)
			}
			if event.Button == gfx.PointerButtonPrimary {
				*t.active, t.focus = i, i
				t.revealI = i
				return gfx.EventResult{Handled: true, Repaint: true, Focus: parts.At(tabPart(i))}
			}
		}
	}
	return gfx.EventResult{}
}

// step is the nearest enabled tab from i in a direction, or i.
func (t *EditableTabs) step(i, dir int) int {
	for j := i + dir; j >= 0 && j < len(t.spec.Tabs); j += dir {
		if !t.spec.Tabs[j].Disabled {
			return j
		}
	}
	return i
}

func (t *EditableTabs) key(event gfx.KeyEvent) gfx.EventResult {
	if t.active == nil || event.Release || event.Text != "" || event.Modifiers != 0 {
		return gfx.EventResult{}
	}
	parts := t.handlers.Parts()
	part, on := event.Target.In(parts)
	n := len(t.spec.Tabs)
	if on && part == 3*n && t.spec.Addable {
		if event.Key == gfx.KeyEnter || event.Key == gfx.KeySpace {
			t.addedNow = true
			return gfx.EventResult{Handled: true, Repaint: true}
		}
		if event.Key == gfx.KeyLeft && n > 0 {
			return gfx.EventResult{Handled: true, Repaint: true, Focus: parts.At(tabPart(n - 1)), FocusVisible: true}
		}
		return gfx.EventResult{}
	}
	if n == 0 {
		return gfx.EventResult{}
	}
	i := min(max(t.focus, 0), n-1)
	switch event.Key {
	case gfx.KeyLeft:
		i = t.step(i, -1)
	case gfx.KeyRight:
		if j := t.step(i, 1); j != i {
			i = j
		} else if t.spec.Addable {
			return gfx.EventResult{Handled: true, Repaint: true, Focus: parts.At(3 * n), FocusVisible: true}
		}
	case gfx.KeyHome:
		i = t.step(-1, 1)
	case gfx.KeyEnd:
		i = t.step(n, -1)
	case gfx.KeyDelete, gfx.KeyBackspace:
		return t.closeTab(i)
	default:
		return gfx.EventResult{}
	}
	*t.active, t.focus, t.revealI = i, i, i
	return gfx.EventResult{Handled: true, Repaint: true, Focus: parts.At(tabPart(i)), FocusVisible: true}
}

// draw paints the strip and declares each tab, close button and the add button as
// a part of the control.
func (t *EditableTabs) draw(d *ui.DrawContext) {
	if t.active == nil {
		return
	}
	canvas, theme := d.Canvas, d.Theme
	metrics := theme.Metrics
	parts := t.handlers.Parts()
	t.width = d.Rect.Width
	total := t.layout(canvas, theme, d.Rect, nil)
	if t.revealI >= 0 && t.revealI < len(t.spec.Tabs) {
		var left, right float32
		x := float32(0)
		for i, tab := range t.spec.Tabs {
			w := t.widthOf(canvas, theme, tab)
			if i == t.revealI {
				left, right = x, x+w
			}
			x += w
		}
		switch {
		case left < t.offset:
			t.offset = left
		case right > t.offset+d.Rect.Width:
			t.offset = right - d.Rect.Width
		}
		t.revealI = -1
	}
	t.offset = float32(math.Round(float64(min(max(t.offset, 0), t.limit(total)))))
	// The rule under the strip, and the clip that keeps scrolled tabs inside it.
	canvas.Fill(gfx.Rect{X: d.Rect.X, Y: d.Rect.Y + d.Rect.Height - 1, Width: d.Rect.Width, Height: 1}, theme.Border)
	canvas.Clip(d.Rect)
	defer canvas.Unclip()
	size := metrics.FontSizeM
	n := len(t.spec.Tabs)
	rest := min(max(t.focus, 0), max(n-1, 0))
	end := d.Rect.X
	t.layout(canvas, theme, d.Rect, func(i int, r gfx.Rect) {
		tab := t.spec.Tabs[i]
		end = r.X + r.Width
		selected := i == *t.active
		hovered := !tab.Disabled && t.hoverPart == tabPart(i)
		ink := theme.MutedForeground
		switch {
		case selected:
			ink = theme.Foreground
			canvas.Fill(gfx.Rect{X: r.X, Y: r.Y + r.Height - 2, Width: r.Width, Height: 2}, theme.Primary)
		case hovered:
			ink = theme.Foreground
			canvas.Fill(gfx.Rect{X: r.X, Y: r.Y, Width: r.Width, Height: r.Height - 1}, theme.Hover)
		}
		if tab.Disabled {
			ink.Alpha *= 0.5
		}
		x := r.X + padX
		if tab.Icon.ViewBox > 0 {
			canvas.DrawIcon(tab.Icon, gfx.Rect{X: x, Y: r.Y + (r.Height-size)/2, Width: size, Height: size}, ink)
			x += size + gap
		}
		labelWidth := r.Width - (x - r.X) - padX
		if !tab.Pinned {
			labelWidth -= closeSz + gap/2
		}
		canvas.Label(tab.Title, gfx.Rect{X: x, Y: r.Y, Width: max(labelWidth, 0), Height: r.Height}, gfx.TextStyle{Size: size, Weight: gfx.WeightMedium, Color: ink})
		if canvas.FocusVisible() == parts.At(tabPart(i)) {
			canvas.RoundedBorder(gfx.Rect{X: r.X + 2, Y: r.Y + 2, Width: r.Width - 4, Height: r.Height - 5}, metrics.RadiusControl, 2, theme.Ring, gfx.Color{})
		}
		canvas.HitWithOptions(parts.At(tabPart(i)), r, tab.Title, gfx.HitOptions{
			Role: gfx.RoleTab, Selected: selected, Disabled: tab.Disabled,
			SkipFocus: tab.Disabled, SkipTab: i != rest,
		})
		if tab.Pinned {
			return
		}
		button := gfx.Rect{X: r.X + r.Width - padX/2 - closeSz, Y: r.Y + (r.Height-closeSz)/2, Width: closeSz, Height: closeSz}
		if t.hoverPart == closePart(i) {
			canvas.Rounded(button, metrics.RadiusControl, theme.Hover)
		}
		show := ink
		if selected || hovered || t.hoverPart == closePart(i) {
			show = theme.Foreground
		}
		canvas.DrawIcon(closeIcon, gfx.Rect{X: button.X + 4, Y: button.Y + 4, Width: closeSz - 8, Height: closeSz - 8}, show)
		canvas.HitWithOptions(parts.At(closePart(i)), button, t.spec.CloseLabel, gfx.HitOptions{
			Role: gfx.RoleButton, Description: tab.Title, Disabled: tab.Disabled, SkipTab: true, SkipFocus: true,
		})
	})
	if t.spec.Addable {
		button := gfx.Rect{X: end + gap, Y: d.Rect.Y + (d.Rect.Height-addSz)/2, Width: addSz, Height: addSz}
		add := parts.At(3 * n)
		if canvas.Hovered() == add {
			canvas.Rounded(button, metrics.RadiusControl, theme.Hover)
		}
		canvas.DrawIcon(plusIcon, gfx.Rect{X: button.X + 7, Y: button.Y + 7, Width: addSz - 14, Height: addSz - 14}, theme.MutedForeground)
		if canvas.FocusVisible() == add {
			canvas.RoundedBorder(button, metrics.RadiusControl, 2, theme.Ring, gfx.Color{})
		}
		canvas.HitWithOptions(add, button, t.spec.AddLabel, gfx.HitOptions{Role: gfx.RoleButton, SkipTab: true})
	}
}
