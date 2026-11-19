package marker_test

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/marker"
)

var testIcon = gfx.Icon{ViewBox: 24, Stroke: 2, Cap: gfx.CapRound, Join: gfx.JoinRound, Commands: []gfx.PathCommand{
	gfx.MoveTo(4, 12), gfx.LineTo(20, 12),
}}

func draw(page *ui.Page, canvas *gfx.Canvas, view func(ui.C)) {
	canvas.Reset()
	canvas.BeginFrame()
	page.Draw(canvas, gfx.Rect{Width: 360, Height: 240}, view)
	canvas.EndFrame()
}

func open(t testing.TB, view func(ui.C)) (*ui.Page, *gfx.Canvas) {
	t.Helper()
	canvas := new(gfx.Canvas)
	canvas.SetReduceMotion(true)
	theme := gfx.LightTheme()
	page := ui.NewPage(ui.PageConfig{View: view, Theme: &theme})
	draw(page, canvas, view)
	return page, canvas
}

func hasText(canvas *gfx.Canvas, value string) bool {
	for _, text := range canvas.Texts() {
		if text.Value == value {
			return true
		}
	}
	return false
}

func textBounds(canvas *gfx.Canvas, value string) (gfx.Rect, bool) {
	for _, text := range canvas.Texts() {
		if text.Value == value {
			return text.Bounds, true
		}
	}
	return gfx.Rect{}, false
}

// A default marker paints its icon and label in the muted metadata color.
func TestViewPaintsIconAndLabel(t *testing.T) {
	theme := gfx.LightTheme()
	badge := marker.New()
	view := func(c ui.C) {
		badge.Reset().Icon(testIcon).Label("Updated 2 minutes ago")
		badge.View(c, "Updated 2 minutes ago")
	}
	_, canvas := open(t, view)
	if !hasText(canvas, "Updated 2 minutes ago") {
		t.Fatal("the marker did not paint its label")
	}
	if len(canvas.Triangles()) == 0 {
		t.Fatal("the marker did not paint its icon")
	}
	for _, text := range canvas.Texts() {
		if text.Value == "Updated 2 minutes ago" && text.Style.Color != theme.MutedForeground {
			t.Fatalf("marker label color = %+v, want muted foreground", text.Style.Color)
		}
	}
}

// The separator variant centers the label between two rules.
func TestSeparatorPaintsRulesOnBothSides(t *testing.T) {
	badge := marker.New()
	view := func(c ui.C) {
		badge.Reset().Separator().Label("October 2026")
		badge.View(c, "October 2026")
	}
	_, canvas := open(t, view)
	bounds, ok := textBounds(canvas, "October 2026")
	if !ok {
		t.Fatal("the separator did not paint its label")
	}
	// Label bounds span the rest of the row; the rules sit outside the glyphs.
	glyphEnd := bounds.X + canvas.TextWidth("October 2026", gfx.LightTheme().Metrics.FontSizeM)
	var left, right int
	for _, rectangle := range canvas.Rectangles() {
		if rectangle.Bounds.Height > 1 {
			continue
		}
		if rectangle.Bounds.X+rectangle.Bounds.Width <= bounds.X {
			left++
		}
		if rectangle.Bounds.X >= glyphEnd {
			right++
		}
	}
	if left == 0 || right == 0 {
		t.Fatalf("separator rules = %d left, %d right; want one on each side; text=%+v rects=%+v", left, right, bounds, canvas.Rectangles())
	}
}

// The border variant paints a rule below the row.
func TestBorderPaintsARuleBelowTheLabel(t *testing.T) {
	badge := marker.New()
	view := func(c ui.C) {
		badge.Reset().Border().Icon(testIcon).Label("Release channel")
		badge.View(c, "Release channel")
	}
	_, canvas := open(t, view)
	bounds, ok := textBounds(canvas, "Release channel")
	if !ok {
		t.Fatal("the border marker did not paint its label")
	}
	for _, rectangle := range canvas.Rectangles() {
		if rectangle.Bounds.Y >= bounds.Y+bounds.Height && rectangle.Bounds.Height <= 1 {
			return
		}
	}
	t.Fatalf("no rule was painted below the label at %+v", bounds)
}

// Progress paints the caller's spinner frame in the icon slot.
func TestProgressPaintsSpinnerAndLabel(t *testing.T) {
	badge := marker.New()
	view := func(c ui.C) {
		badge.Reset().Progress(2).Label("Thinking")
		badge.View(c, "Thinking")
	}
	_, canvas := open(t, view)
	if !hasText(canvas, "Thinking") {
		t.Fatal("the progress marker did not paint its label")
	}
	if len(canvas.Rectangles()) == 0 {
		t.Fatal("the progress marker did not paint the spinner")
	}
	if canvas.Animating() {
		t.Fatal("an unchanged progress marker requested another frame")
	}
}

func TestWarmPaintAllocatesNothing(t *testing.T) {
	var plain, separator, border, progress marker.Marker
	view := func(c ui.C) {
		plain.Reset().Icon(testIcon).Label("Default")
		plain.View(c, "Default")
		separator.Reset().Separator().Label("October 2026")
		separator.View(c, "October 2026")
		border.Reset().Border().Icon(testIcon).Label("Release channel")
		border.View(c, "Release channel")
		progress.Reset().Progress(2).Label("Thinking")
		progress.View(c, "Thinking")
	}
	page, canvas := open(t, view)
	if allocations := testing.AllocsPerRun(100, func() { draw(page, canvas, view) }); allocations != 0 {
		t.Fatalf("warm paint allocated %v times, want 0", allocations)
	}
}

func BenchmarkViewPaint(b *testing.B) {
	var plain, separator, border, progress marker.Marker
	view := func(c ui.C) {
		plain.Reset().Icon(testIcon).Label("Default")
		plain.View(c, "Default")
		separator.Reset().Separator().Label("October 2026")
		separator.View(c, "October 2026")
		border.Reset().Border().Icon(testIcon).Label("Release channel")
		border.View(c, "Release channel")
		progress.Reset().Progress(2).Label("Thinking")
		progress.View(c, "Thinking")
	}
	page, canvas := open(b, view)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		draw(page, canvas, view)
	}
}
