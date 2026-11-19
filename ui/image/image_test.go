package image_test

import (
	"math"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/image"
)

var testBitmap = gfx.Bitmap{
	Pix: make([]byte, 40*20*4), Width: 40, Height: 20, Stride: 40 * 4, ID: 7, Version: 1,
}

type model struct {
	view    image.Image
	page    *ui.Page
	canvas  *gfx.Canvas
	pointer int
	key     int
	scroll  int
	target  gfx.ID
}

func (m *model) viewPage(c ui.C) {
	image.View(c, "Preview", &m.view).Size(200, 120)
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

func open(t testing.TB, m *model) {
	t.Helper()
	m.view.Reset().Bitmap(testBitmap).Fit()
	m.view.Pointer = func(event gfx.PointerEvent) gfx.EventResult {
		m.pointer++
		m.target = event.Target
		return gfx.EventResult{Handled: true, Repaint: true}
	}
	m.view.Key = func(event gfx.KeyEvent) gfx.EventResult {
		m.key++
		m.target = event.Target
		return gfx.EventResult{Handled: true}
	}
	m.view.Scroll = func(event gfx.ScrollEvent) gfx.EventResult {
		m.scroll++
		m.target = event.Target
		return gfx.EventResult{Handled: true}
	}
	theme := gfx.LightTheme()
	m.page = ui.NewPage(ui.PageConfig{View: m.viewPage, Theme: &theme})
	m.canvas = new(gfx.Canvas)
	draw(m.page, m.canvas, m.viewPage)
}

func hit(t *testing.T, canvas *gfx.Canvas, label string) gfx.Hit {
	t.Helper()
	hits := canvas.Hits()
	for index := len(hits) - 1; index >= 0; index-- {
		if hits[index].Label == label && !hits[index].ID.IsZero() {
			return hits[index]
		}
	}
	t.Fatalf("no hit labelled %q was painted", label)
	return gfx.Hit{}
}

func paintedImage(t *testing.T, canvas *gfx.Canvas) gfx.Image {
	t.Helper()
	images := canvas.Images()
	if len(images) != 1 {
		t.Fatalf("painted %d images, want 1", len(images))
	}
	return images[0]
}

func closeTo(got, want float32) bool { return math.Abs(float64(got-want)) < .001 }

// Fit centers the whole bitmap in the caller's viewport and keeps its
// proportions; the page names the view for assistive technology.
func TestFitCentersWholeBitmap(t *testing.T) {
	m := new(model)
	open(t, m)

	control := hit(t, m.canvas, "Preview")
	if control.Role != gfx.RoleGroup || !control.Focusable {
		t.Fatalf("control = %+v, want a focusable group", control)
	}
	painted := paintedImage(t, m.canvas)
	if !closeTo(painted.Bounds.X, 0) || !closeTo(painted.Bounds.Y, 10) ||
		!closeTo(painted.Bounds.Width, 200) || !closeTo(painted.Bounds.Height, 100) {
		t.Fatalf("fit bounds = %+v, want 200x100 centered at y=10", painted.Bounds)
	}
	if painted.Bitmap.ID != testBitmap.ID {
		t.Fatalf("painted bitmap ID = %d, want %d", painted.Bitmap.ID, testBitmap.ID)
	}
}

// Actual maps one bitmap pixel to one device pixel, independent of the display
// scale, and keeps the chained pan while the component clamps it.
func TestActualSizeAndChainedOptions(t *testing.T) {
	m := new(model)
	open(t, m)
	m.canvas.SetPixelScale(2)
	m.view.Actual()
	draw(m.page, m.canvas, m.viewPage)

	painted := paintedImage(t, m.canvas)
	if !closeTo(painted.Bounds.Width*2, float32(testBitmap.Width)) ||
		!closeTo(painted.Bounds.Height*2, float32(testBitmap.Height)) {
		t.Fatalf("actual bounds = %+v at 2x, want 40x20 device pixels", painted.Bounds)
	}

	m.view.Zoom(16).Pan(gfx.Point{X: 1e6, Y: -1e6})
	draw(m.page, m.canvas, m.viewPage)
	painted = paintedImage(t, m.canvas)
	if !closeTo(painted.Bounds.Width, 320) || !closeTo(painted.Bounds.Height, 160) {
		t.Fatalf("zoomed bounds = %+v, want 320x160", painted.Bounds)
	}
	if !closeTo(painted.Bounds.X, 0) || !closeTo(painted.Bounds.Y, -40) {
		t.Fatalf("panned bounds = %+v, want the clamps at x=0 y=-40", painted.Bounds)
	}

	m.view.Fit()
	draw(m.page, m.canvas, m.viewPage)
	if painted = paintedImage(t, m.canvas); !closeTo(painted.Bounds.Width, 200) {
		t.Fatalf("Fit after Zoom bounds = %+v, want the fitted width", painted.Bounds)
	}
}

// The wrapper routes the control's pointer, key and wheel events to the
// caller's callbacks, which a page uses to drag and zoom.
func TestInteractionCallbacksReceiveEvents(t *testing.T) {
	m := new(model)
	open(t, m)
	control := hit(t, m.canvas, "Preview")

	if result := m.page.Pointer(gfx.PointerEvent{
		Kind: gfx.PointerDown, Button: gfx.PointerButtonPrimary, Target: control.ID,
		X: control.Bounds.X + 2, Y: control.Bounds.Y + 2,
	}); !result.Handled || m.pointer != 1 || m.target != control.ID {
		t.Fatalf("pointer = %+v after %d calls, target %v; want %v", result, m.pointer, m.target, control.ID)
	}
	if result := m.page.Key(gfx.KeyEvent{Key: gfx.KeyLeft, Target: control.ID}); !result.Handled || m.key != 1 || m.target != control.ID {
		t.Fatalf("key = %+v after %d calls, target %v; want %v", result, m.key, m.target, control.ID)
	}
	if result := m.page.Wheel(gfx.ScrollEvent{Target: control.ID, DeltaY: 1}); !result.Handled || m.scroll != 1 || m.target != control.ID {
		t.Fatalf("wheel = %+v after %d calls, target %v; want %v", result, m.scroll, m.target, control.ID)
	}
}

// A click on the view is reported after View and not in an ordinary paint.
func TestClickedReportsTheEventPass(t *testing.T) {
	m := new(model)
	open(t, m)
	control := hit(t, m.canvas, "Preview")
	drawClick(m.page, m.canvas, m.viewPage, control.ID)
	if !m.view.Clicked() {
		t.Fatal("Clicked did not report the event pass's click")
	}
	draw(m.page, m.canvas, m.viewPage)
	if m.view.Clicked() {
		t.Fatal("Clicked reported a click in an ordinary paint")
	}
}

// An empty bitmap paints the viewport surface but no image command.
func TestEmptyBitmapPaintsNoImage(t *testing.T) {
	m := new(model)
	open(t, m)
	m.view.Reset()
	draw(m.page, m.canvas, m.viewPage)
	if images := m.canvas.Images(); len(images) != 0 {
		t.Fatalf("empty bitmap painted %d images", len(images))
	}
}

// A warm repaint allocates nothing: the handlers and draw function are made
// once and the component keeps its own layout.
func TestWarmPaintAllocatesNothing(t *testing.T) {
	m := new(model)
	open(t, m)
	if allocations := testing.AllocsPerRun(100, func() { draw(m.page, m.canvas, m.viewPage) }); allocations != 0 {
		t.Fatalf("warm paint allocated %v times, want 0", allocations)
	}
}

func BenchmarkViewPaint(b *testing.B) {
	m := new(model)
	open(b, m)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		draw(m.page, m.canvas, m.viewPage)
	}
}
