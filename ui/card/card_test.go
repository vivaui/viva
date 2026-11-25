package card_test

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/card"
)

func measure(value string, size, weight float32) float32 { return float32(len(value)) * size * 0.55 }

func draw(page *ui.Page, canvas *gfx.Canvas, view func(ui.C)) {
	canvas.BeginFrame()
	canvas.Reset()
	page.Draw(canvas, gfx.Rect{Width: 480, Height: 320}, view)
	canvas.EndFrame()
}

func open(t testing.TB, view func(ui.C)) (*ui.Page, *gfx.Canvas) {
	t.Helper()
	theme := gfx.LightTheme()
	page := ui.NewPage(ui.PageConfig{View: view, Theme: &theme})
	canvas := &gfx.Canvas{}
	canvas.SetMeasurer(measure)
	canvas.SetReduceMotion(true)
	draw(page, canvas, view)
	return page, canvas
}

func textAt(canvas *gfx.Canvas, value string) (gfx.Text, bool) {
	for _, text := range canvas.Texts() {
		if text.Value == value {
			return text, true
		}
	}
	return gfx.Text{}, false
}

// The sections are declared in order, each with the page's own content: header,
// body, then footer, and what Extra declares sits at the end of the header.
func TestSectionsRunDownAndExtraSitsAtTheHeaderEnd(t *testing.T) {
	view := func(c ui.C) {
		card.View(c, card.Spec{
			Title: "Preview", Description: "Keeps nodes.",
			Extra:  func() { ui.Text(c, "extra") },
			Footer: func() { ui.Text(c, "footer") },
		}, func() { ui.Text(c, "body") }).FillWidth()
	}
	_, canvas := open(t, view)
	title, _ := textAt(canvas, "Preview")
	extra, ok := textAt(canvas, "extra")
	body, _ := textAt(canvas, "body")
	footer, ok2 := textAt(canvas, "footer")
	if !ok || !ok2 {
		t.Fatal("the card did not paint its extra and footer")
	}
	if !(title.Bounds.Y < body.Bounds.Y && body.Bounds.Y < footer.Bounds.Y) {
		t.Fatalf("sections are not in order: title %v, body %v, footer %v", title.Bounds.Y, body.Bounds.Y, footer.Bounds.Y)
	}
	if extra.Bounds.X <= title.Bounds.X+title.Bounds.Width {
		t.Fatalf("extra at %v is not after the title ending %v", extra.Bounds.X, title.Bounds.X+title.Bounds.Width)
	}
}

// A card with no header text and no Extra has no header; the small card is
// tighter than the default one.
func TestHeaderIsOmittedAndSmallIsTighter(t *testing.T) {
	height := func(spec card.Spec) float32 {
		var box ui.Box
		view := func(c ui.C) { box = card.View(c, spec, func() { ui.Text(c, "body") }).FillWidth() }
		open(t, view)
		return box.Rect().Height
	}
	plain, headed := height(card.Spec{}), height(card.Spec{Title: "Title"})
	if headed <= plain {
		t.Fatalf("a card with a title is %v high, not above a card without one (%v)", headed, plain)
	}
	if small := height(card.Spec{Size: card.Small}); small >= plain {
		t.Fatalf("a small card is %v high, want it tighter than the default %v", small, plain)
	}
}

// Flat leaves the shadow out.
func TestFlatHasNoShadow(t *testing.T) {
	shadows := func(flat bool) int {
		view := func(c ui.C) { card.View(c, card.Spec{Flat: flat}, func() { ui.Text(c, "body") }).FillWidth() }
		_, canvas := open(t, view)
		return len(canvas.Shadows())
	}
	if shadows(false) == 0 || shadows(true) != 0 {
		t.Fatalf("shadows: raised %d, flat %d; want some and none", shadows(false), shadows(true))
	}
}

func TestWarmPaintAllocatesNothing(t *testing.T) {
	extra := func() {}
	spec := card.Spec{Title: "Preview", Description: "Keeps nodes.", Extra: extra, Footer: extra, HeaderBorder: true, FooterBorder: true}
	var c0 ui.C
	body := func() { ui.Text(c0, "body") }
	view := func(c ui.C) { c0 = c; card.View(c, spec, body).FillWidth() }
	page, canvas := open(t, view)
	if allocations := testing.AllocsPerRun(100, func() { draw(page, canvas, view) }); allocations != 0 {
		t.Fatalf("warm paint allocated %v times, want 0", allocations)
	}
}

func BenchmarkViewPaint(b *testing.B) {
	spec := card.Spec{Title: "Preview", Description: "Keeps nodes."}
	var c0 ui.C
	body := func() { ui.Text(c0, "body") }
	view := func(c ui.C) { c0 = c; card.View(c, spec, body).FillWidth() }
	page, canvas := open(b, view)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		draw(page, canvas, view)
	}
}
