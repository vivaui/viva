package carousel_test

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/carousel"
)

var (
	testPrevious = gfx.Item(91)
	testNext     = gfx.Item(92)
)

type model struct {
	view     carousel.Carousel
	page     *ui.Page
	canvas   *gfx.Canvas
	target   gfx.ID
	draws    int
	geometry carousel.Geometry
	keys     int
	scrolls  int
}

func (m *model) wire(orientation carousel.Orientation) {
	m.view.Orientation = orientation
	m.view.ItemCount = 3
	m.view.Index = 1
	m.view.PreviousID, m.view.NextID = testPrevious, testNext
	m.view.PreviousLabel, m.view.NextLabel = "Previous slide", "Next slide"
	m.view.Draw = func(d *ui.DrawContext, geometry carousel.Geometry) {
		m.draws++
		m.geometry = geometry
		d.Canvas.Fill(m.view.ItemBounds(geometry, geometry.Index), d.Theme.Accent)
	}
	m.view.Pointer = func(event gfx.PointerEvent) gfx.EventResult {
		m.target = event.Target
		return gfx.EventResult{Handled: true, Repaint: true}
	}
	m.view.Key = func(event gfx.KeyEvent) gfx.EventResult {
		m.keys++
		m.target = event.Target
		return gfx.EventResult{Handled: true}
	}
	m.view.Scroll = func(event gfx.ScrollEvent) gfx.EventResult {
		m.scrolls++
		m.target = event.Target
		return gfx.EventResult{Handled: true}
	}
}

func (m *model) viewPage(c ui.C) {
	carousel.View(c, "Tour", &m.view).Size(320, 200)
}

func (m *model) open(t testing.TB, orientation carousel.Orientation) {
	t.Helper()
	m.wire(orientation)
	theme := gfx.LightTheme()
	m.page = ui.NewPage(ui.PageConfig{View: m.viewPage, Theme: &theme})
	m.canvas = new(gfx.Canvas)
	draw(m.page, m.canvas, m.viewPage)
}

func draw(page *ui.Page, canvas *gfx.Canvas, view func(ui.C)) {
	canvas.Reset()
	canvas.BeginFrame()
	page.Draw(canvas, gfx.Rect{Width: 400, Height: 300}, view)
	canvas.EndFrame()
}

func drawClick(page *ui.Page, canvas *gfx.Canvas, view func(ui.C), id gfx.ID) {
	canvas.BeginEventPass(id, gfx.ClickInfo{})
	draw(page, canvas, view)
	canvas.EndEventPass()
}

// hitLabelled returns the front-most hit with the label and a non-zero identity.
func hitLabelled(canvas *gfx.Canvas, label string) (gfx.Hit, bool) {
	hits := canvas.Hits()
	for i := len(hits) - 1; i >= 0; i-- {
		if hits[i].Label == label && !hits[i].ID.IsZero() {
			return hits[i], true
		}
	}
	return gfx.Hit{}, false
}

func hit(t *testing.T, canvas *gfx.Canvas, label string) gfx.Hit {
	t.Helper()
	got, ok := hitLabelled(canvas, label)
	if !ok {
		t.Fatalf("no hit labelled %q was painted", label)
	}
	return got
}

// The page declares the view, the component lays out the content, the Draw
// callback paints the selected slide inside the viewport, and the controls keep
// the caller's identities while the control routes input to its callbacks.
func TestViewPaintsGeometryAndRoutesInput(t *testing.T) {
	m := new(model)
	m.open(t, carousel.Horizontal)

	geometry := m.view.Geometry()
	if m.draws != 1 {
		t.Fatalf("Draw called %d times, want 1", m.draws)
	}
	if geometry.Viewport.Width != 320 || geometry.Viewport.Height != 200 {
		t.Fatalf("viewport = %+v, want 320x200", geometry.Viewport)
	}
	if geometry.Content.Width != 960 || geometry.Offset != 320 || geometry.Index != 1 {
		t.Fatalf("content = %+v offset %g index %d, want 960, 320, 1", geometry.Content, geometry.Offset, geometry.Index)
	}
	if first := m.view.ItemBounds(geometry, 0); first.Width != 320 || first.X != geometry.Content.X {
		t.Fatalf("first slide = %+v, want one 320-wide viewport at the content origin", first)
	}
	if second := m.view.ItemBounds(geometry, 1); second.X != geometry.Content.X+320 {
		t.Fatalf("second slide = %+v, want it one viewport along", second)
	}

	control := hit(t, m.canvas, "Tour")
	previous := hit(t, m.canvas, "Previous slide")
	next := hit(t, m.canvas, "Next slide")
	if previous.ID != testPrevious || next.ID != testNext {
		t.Fatalf("navigation hits = %v %v, want the caller's %v %v", previous.ID, next.ID, testPrevious, testNext)
	}
	if previous.Role != gfx.RoleButton || previous.Disabled || !previous.Focusable {
		t.Fatalf("previous hit = %+v, want an enabled button", previous)
	}

	if result := m.page.Pointer(gfx.PointerEvent{
		Kind: gfx.PointerDown, Button: gfx.PointerButtonPrimary, Target: control.ID,
		X: control.Bounds.X + 4, Y: control.Bounds.Y + 4,
	}); !result.Handled || m.target != control.ID {
		t.Fatalf("pointer Target = %v (result %+v), want %v", m.target, result, control.ID)
	}
	if result := m.page.Key(gfx.KeyEvent{Key: gfx.KeyLeft, Target: control.ID}); !result.Handled || m.keys != 1 || m.target != control.ID {
		t.Fatalf("key Target = %v (result %+v after %d calls)", m.target, result, m.keys)
	}
	if result := m.page.Wheel(gfx.ScrollEvent{Target: control.ID, DeltaY: 1}); !result.Handled || m.scrolls != 1 || m.target != control.ID {
		t.Fatalf("wheel Target = %v (result %+v after %d calls)", m.target, result, m.scrolls)
	}
}

// A click on a navigation control is read with Clicked after View, and a
// different identity is not reported.
func TestViewReportsClicksForCallerIdentities(t *testing.T) {
	m := new(model)
	m.open(t, carousel.Horizontal)

	drawClick(m.page, m.canvas, m.viewPage, testNext)
	if !m.view.Clicked(testNext) {
		t.Fatal("Clicked did not report the clicked next control")
	}
	if m.view.Clicked(testPrevious) {
		t.Fatal("Clicked reported an identity that was not clicked")
	}
	draw(m.page, m.canvas, m.viewPage)
	if m.view.Clicked(testNext) {
		t.Fatal("Clicked reported a click in an ordinary paint")
	}
}

// The vertical orientation places the controls above and below the viewport and
// divides the content extent by height.
func TestVerticalOrientation(t *testing.T) {
	m := new(model)
	m.open(t, carousel.Vertical)
	m.view.Index = 0
	draw(m.page, m.canvas, m.viewPage)
	geometry := m.view.Geometry()
	if geometry.Viewport.Height != 200 || geometry.Content.Height != 600 {
		t.Fatalf("vertical content = %+v viewport %+v, want 600 high", geometry.Content, geometry.Viewport)
	}
	if geometry.PreviousButton.Y >= geometry.Viewport.Y || geometry.NextButton.Y <= geometry.Viewport.Y+geometry.Viewport.Height {
		t.Fatalf("vertical controls = %+v %+v, want them above and below", geometry.PreviousButton, geometry.NextButton)
	}
	if item := m.view.ItemBounds(geometry, 2); item.Height != 200 || item.Y != geometry.Content.Y+400 {
		t.Fatalf("third vertical slide = %+v, want one viewport down the axis", item)
	}
}

// Disabled keeps both control bounds and semantics while making them inert.
func TestDisabledControls(t *testing.T) {
	m := new(model)
	m.open(t, carousel.Horizontal)
	m.view.Disabled = true
	m.view.Index = 0
	draw(m.page, m.canvas, m.viewPage)
	geometry := m.view.Geometry()
	if geometry.CanPrevious || geometry.CanNext {
		t.Fatalf("disabled geometry = %+v, want both directions off", geometry)
	}
	next := hit(t, m.canvas, "Next slide")
	if next.ID != testNext || !next.Disabled || next.Focusable {
		t.Fatalf("disabled next hit = %+v, want inert with its caller identity", next)
	}
}

// A warm repaint allocates nothing: the wrapper keeps its part scope and the
// component keeps its own layout caches.
func TestWarmPaintAllocatesNothing(t *testing.T) {
	m := new(model)
	m.open(t, carousel.Horizontal)
	if allocations := testing.AllocsPerRun(100, func() { draw(m.page, m.canvas, m.viewPage) }); allocations != 0 {
		t.Fatalf("warm paint allocated %v times, want 0", allocations)
	}
}

func BenchmarkViewPaint(b *testing.B) {
	m := new(model)
	m.open(b, carousel.Horizontal)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		draw(m.page, m.canvas, m.viewPage)
	}
}
