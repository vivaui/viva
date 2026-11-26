// Package rate is the opt-in page control for a star rating: a row of stars
// that shows a score and, unless it is read only, lets the user choose one. A
// page keeps a Rate in its model beside the score it edits:
//
//	m.rate = rate.New().AllowHalf()
//	// in the page:
//	m.rate.View(c, "Rating", &m.score)
//	if m.rate.Changed() {
//		// the user chose a new score; m.score already holds it
//	}
//
// The score is a number of stars from zero to Count, a whole number or, with
// AllowHalf, a multiple of one half. A click on a star chooses it, on the left
// half of a star chooses the half with AllowHalf, and a click on the score
// already chosen clears it unless KeepScore is set. While the pointer is over
// the stars they show the score a click would choose.
//
// The stars are one tab stop. The arrow keys change the score by one star, or
// one half with AllowHalf, and Home and End choose none and all of the stars. A
// ReadOnly rating shows the score and takes no input.
//
// It is a separate package so an application that does not import it pays
// nothing for it. Keep a Rate in the model: its handlers and draw function are
// made once, so a warm paint allocates nothing. Do not copy a Rate after it has
// been declared.
package rate

import (
	"math"
	"strconv"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// DefaultColor is the color of a chosen star unless Color gives another.
var DefaultColor = gfx.RGB(245, 183, 0)

// starPath is Lucide's star, as a closed outline.
var starPath = []gfx.PathCommand{
	gfx.MoveTo(12, 2), gfx.LineTo(15.09, 8.26), gfx.LineTo(22, 9.27), gfx.LineTo(17, 14.14),
	gfx.LineTo(18.18, 21.02), gfx.LineTo(12, 17.77), gfx.LineTo(5.82, 21.02), gfx.LineTo(7, 14.14),
	gfx.LineTo(2, 9.27), gfx.LineTo(8.91, 8.26), gfx.ClosePath(),
}

var (
	solidStar   = gfx.Icon{ViewBox: 24, Commands: starPath}
	outlineStar = gfx.Icon{ViewBox: 24, Stroke: 1.5, Cap: gfx.CapRound, Join: gfx.JoinRound, Commands: starPath}
)

// names are the accessible names of the stars, made once.
var names [16]string

func init() {
	for i := range names {
		names[i] = strconv.Itoa(i + 1)
	}
}

func name(i int) string {
	if i < len(names) {
		return names[i]
	}
	return strconv.Itoa(i + 1)
}

// Rate is one rating control. Configure it with the chainable methods; they may
// be called before each View.
type Rate struct {
	handlers ui.Handlers
	drawFn   func(*ui.DrawContext)

	count    int
	half     bool
	keep     bool
	readOnly bool
	size     float32
	color    gfx.Color

	value    *float32
	origin   gfx.Rect // where the control was drawn last
	hover    float32  // the score the pointer would choose; zero when it is elsewhere
	changed  bool     // the score changed since the last View
	reported bool     // the score changed since the paint before this one
}

// New returns a rating of five stars.
func New() *Rate {
	r := &Rate{count: 5, size: 20}
	r.handlers.Key = r.key
	r.handlers.Pointer = r.pointer
	r.drawFn = r.draw
	return r
}

// Count is the number of stars; five by default.
func (r *Rate) Count(count int) *Rate {
	r.count = max(count, 1)
	return r
}

// AllowHalf lets the score be a multiple of one half.
func (r *Rate) AllowHalf() *Rate {
	r.half = true
	return r
}

// KeepScore makes a click on the chosen score keep it, instead of clearing it.
func (r *Rate) KeepScore() *Rate {
	r.keep = true
	return r
}

// ReadOnly shows the score and takes no input.
func (r *Rate) ReadOnly() *Rate {
	r.readOnly = true
	return r
}

// Editable clears ReadOnly, for a page that enables editing later.
func (r *Rate) Editable() *Rate {
	r.readOnly = false
	return r
}

// Size is the width and height of a star in pixels; 20 by default.
func (r *Rate) Size(size float32) *Rate {
	r.size = max(size, 8)
	return r
}

// Color is the color of the chosen stars; DefaultColor by default.
func (r *Rate) Color(color gfx.Color) *Rate {
	r.color = color
	return r
}

// Changed reports whether the user chose a new score since the last paint.
func (r *Rate) Changed() bool { return r.reported }

const gap = 4

// step is the smallest change of the score.
func (r *Rate) step() float32 {
	if r.half {
		return 0.5
	}
	return 1
}

// View declares the rating at c, showing and editing *score. label names it for
// assistive technology. Declare the view on its own, not inside a Memo.
func (r *Rate) View(c ui.C, label string, score *float32) ui.Box {
	if r == nil {
		panic("rate: View needs its Rate")
	}
	if score == nil {
		panic("rate: View needs the score")
	}
	r.reported, r.changed = r.changed, false
	r.value = score
	*score = r.clamp(*score)
	r.handlers.Role = gfx.RoleGroup
	if r.readOnly {
		r.handlers.Role = gfx.RoleStatus
	}
	width := float32(r.count)*r.size + float32(r.count-1)*gap
	return ui.Control(c, label, &r.handlers).Size(width, r.size).Shrink(0).Draw(r.drawFn)
}

// clamp is score as one the rating can show: from zero to Count, in whole
// stars or halves.
func (r *Rate) clamp(score float32) float32 {
	if score != score { // NaN
		return 0
	}
	step := r.step()
	score = float32(math.Round(float64(score/step))) * step
	return min(max(score, 0), float32(r.count))
}

// choose makes score the chosen one and reports whether it changed.
func (r *Rate) choose(score float32) bool {
	score = r.clamp(score)
	if score == *r.value {
		return false
	}
	*r.value, r.changed = score, true
	return true
}

// pointedScore is the score a click at x on star i would choose.
func (r *Rate) pointedScore(i int, x float32) float32 {
	score := float32(i + 1)
	if r.half && x < r.starRect(r.origin, i).X+r.size/2 {
		score -= 0.5
	}
	return score
}

// starRect is the rectangle of star i within origin, the control's rectangle.
func (r *Rate) starRect(origin gfx.Rect, i int) gfx.Rect {
	return gfx.Rect{X: origin.X + float32(i)*(r.size+gap), Y: origin.Y, Width: r.size, Height: r.size}
}

// focusPart is the star the keyboard rests on: the one that holds the score, or
// the first when there is none.
func (r *Rate) focusPart(score float32) int {
	return min(max(int(math.Ceil(float64(score)))-1, 0), r.count-1)
}

func (r *Rate) pointer(event gfx.PointerEvent) gfx.EventResult {
	if r.readOnly || r.value == nil {
		return gfx.EventResult{}
	}
	i, onStar := event.Target.In(r.handlers.Parts())
	switch event.Kind {
	case gfx.PointerExit:
		if r.hover == 0 {
			return gfx.EventResult{}
		}
		r.hover = 0
		return gfx.EventResult{Repaint: true}
	case gfx.PointerMove:
		if !onStar {
			return gfx.EventResult{}
		}
		hover := r.pointedScore(i, event.X)
		if hover == r.hover {
			return gfx.EventResult{}
		}
		r.hover = hover
		return gfx.EventResult{Repaint: true}
	case gfx.PointerDown:
		if !onStar || event.Button != gfx.PointerButtonPrimary {
			return gfx.EventResult{}
		}
		score := r.pointedScore(i, event.X)
		if score == *r.value && !r.keep {
			score = 0
		}
		r.choose(score)
		r.hover = 0
		return gfx.EventResult{Handled: true, Repaint: true, Focus: r.handlers.Parts().At(r.focusPart(*r.value)), FocusVisible: false}
	}
	return gfx.EventResult{}
}

func (r *Rate) key(event gfx.KeyEvent) gfx.EventResult {
	if r.readOnly || r.value == nil || event.Text != "" {
		return gfx.EventResult{}
	}
	score := *r.value
	switch event.Key {
	case gfx.KeyLeft, gfx.KeyDown:
		score -= r.step()
	case gfx.KeyRight, gfx.KeyUp:
		score += r.step()
	case gfx.KeyHome:
		score = 0
	case gfx.KeyEnd:
		score = float32(r.count)
	default:
		return gfx.EventResult{}
	}
	r.choose(score)
	return gfx.EventResult{Handled: true, Repaint: true, Focus: r.handlers.Parts().At(r.focusPart(*r.value)), FocusVisible: true}
}

// draw paints the stars and declares each as a part of the control. A star is
// painted empty, then filled over the score the pointer or the user's choice
// shows, with the half star clipped to its left half.
func (r *Rate) draw(d *ui.DrawContext) {
	if r.value == nil {
		return
	}
	r.origin = d.Rect
	canvas, theme := d.Canvas, d.Theme
	parts := r.handlers.Parts()
	shown := *r.value
	if r.hover > 0 && !r.readOnly {
		shown = r.hover
	}
	color := r.color
	if color == (gfx.Color{}) {
		color = DefaultColor
	}
	rest := r.focusPart(*r.value)
	for i := range r.count {
		star := r.starRect(d.Rect, i)
		canvas.DrawIcon(solidStar, star, theme.Border)
		switch filled := min(max(shown-float32(i), 0), 1); {
		case filled >= 1:
			canvas.DrawIcon(solidStar, star, color)
		case filled > 0:
			canvas.Clip(gfx.Rect{X: star.X, Y: star.Y, Width: star.Width * filled, Height: star.Height})
			canvas.DrawIcon(solidStar, star, color)
			canvas.Unclip()
		}
		if r.readOnly {
			continue
		}
		if canvas.FocusVisible() == parts.At(i) {
			ring(canvas, theme, gfx.Rect{X: star.X - 2, Y: star.Y - 2, Width: star.Width + 4, Height: star.Height + 4})
		}
		// The stars are one radio group and one tab stop: the one that holds the
		// score is checked and takes the focus, and the arrow keys move it.
		canvas.HitWithOptions(parts.At(i), star, name(i), gfx.HitOptions{
			Role: gfx.RoleRadio, Checked: *r.value > 0 && i == rest, SkipFocus: i != rest, SkipTab: i != rest,
			PosInSet: int32(i + 1), SetSize: int32(r.count),
		})
	}
}

// ring paints the focus ring around a star.
func ring(canvas *gfx.Canvas, theme *gfx.Theme, b gfx.Rect) {
	const width = float32(2)
	canvas.Fill(gfx.Rect{X: b.X, Y: b.Y, Width: b.Width, Height: width}, theme.Ring)
	canvas.Fill(gfx.Rect{X: b.X, Y: b.Y + b.Height - width, Width: b.Width, Height: width}, theme.Ring)
	canvas.Fill(gfx.Rect{X: b.X, Y: b.Y, Width: width, Height: b.Height}, theme.Ring)
	canvas.Fill(gfx.Rect{X: b.X + b.Width - width, Y: b.Y, Width: width, Height: b.Height}, theme.Ring)
}
