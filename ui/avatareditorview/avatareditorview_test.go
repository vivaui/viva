package avatareditorview_test

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/avatareditorview"
)

var (
	testCanvas  = gfx.Item(4)
	testZoom    = gfx.Item(5)
	testZoomOut = gfx.Item(6)
	testZoomIn  = gfx.Item(7)
	testReset   = gfx.Item(8)
)

func testIDs() avatareditorview.IDs {
	return avatareditorview.IDs{
		Canvas: testCanvas, Zoom: testZoom, ZoomOut: testZoomOut, ZoomIn: testZoomIn,
		Reset: testReset,
	}
}

func testBitmap() gfx.Bitmap {
	const width, height = 160, 100
	pixels := make([]byte, width*height*4)
	for index := 3; index < len(pixels); index += 4 {
		pixels[index] = 255
	}
	return gfx.Bitmap{Pix: pixels, Width: width, Height: height, Stride: width * 4, ID: 1, Version: 1}
}

type model struct {
	view    avatareditorview.Editor
	page    *ui.Page
	canvas  *gfx.Canvas
	target  gfx.ID
	keys    int
	scrolls int
}

func (m *model) wire() {
	m.view.Bitmap = testBitmap()
	m.view.IDs = testIDs()
	m.view.AspectRatio = 1
	m.view.Circular = true
	m.view.Labels = avatareditorview.Labels{
		Hint: "Drag and zoom", Canvas: "Avatar crop", Zoom: "Zoom",
		ZoomOut: "Zoom out", ZoomIn: "Zoom in", Reset: "Reset",
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
	avatareditorview.View(c, "Editor", &m.view).Size(360, 260)
}

func (m *model) open(t testing.TB) {
	t.Helper()
	theme := gfx.LightTheme()
	m.page = ui.NewPage(ui.PageConfig{View: m.viewPage, Theme: &theme})
	m.canvas = new(gfx.Canvas)
	draw(m.page, m.canvas, m.viewPage)
}

func draw(page *ui.Page, canvas *gfx.Canvas, view func(ui.C)) {
	canvas.Reset()
	canvas.BeginFrame()
	page.Draw(canvas, gfx.Rect{Width: 400, Height: 320}, view)
	canvas.EndFrame()
}

func drawClick(page *ui.Page, canvas *gfx.Canvas, view func(ui.C), id gfx.ID) {
	canvas.BeginEventPass(id, gfx.ClickInfo{})
	draw(page, canvas, view)
	canvas.EndEventPass()
}

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

// The page declares the view, the component paints the caller's image and crop
// state, and the wrapper gives every control an identity inside the page's
// control so pointer, key and wheel events reach it.
func TestViewPaintsAndRoutesInput(t *testing.T) {
	m := new(model)
	m.wire()
	m.open(t)

	if layout := m.view.Layout(); !layout.Ready || layout.Crop.Width <= 0 || layout.Stage.Width <= 0 {
		t.Fatalf("layout = %+v, want a ready crop and stage", layout)
	}
	canvas := hit(t, m.canvas, "Avatar crop")
	if canvas.Role != gfx.RoleGroup {
		t.Fatalf("canvas role = %v, want a group", canvas.Role)
	}
	if zoomIn := hit(t, m.canvas, "Zoom in"); zoomIn.ID.IsZero() {
		t.Fatal("zoom in was painted without an identity")
	}

	x := canvas.Bounds.X + canvas.Bounds.Width/2
	y := canvas.Bounds.Y + canvas.Bounds.Height/2
	if result := m.page.Pointer(gfx.PointerEvent{
		Kind: gfx.PointerDown, Button: gfx.PointerButtonPrimary, Target: canvas.ID, X: x, Y: y,
	}); !result.Handled || !result.Repaint || m.target != testCanvas {
		t.Fatalf("pointer down Target = %v (result %+v), want the caller's %v", m.target, result, testCanvas)
	}
	if result := m.page.Pointer(gfx.PointerEvent{
		Kind: gfx.PointerMove, Captured: true, Target: canvas.ID, X: x + 40, Y: y - 25,
	}); !result.Handled || m.target != testCanvas {
		t.Fatalf("pointer move Target = %v (result %+v)", m.target, result)
	}
	if m.view.State.CenterX == .5 && m.view.State.CenterY == .5 {
		t.Fatalf("drag did not update the crop: %+v", m.view.State)
	}
	if result := m.page.Pointer(gfx.PointerEvent{
		Kind: gfx.PointerUp, Captured: true, Target: canvas.ID, X: x + 40, Y: y - 25,
	}); !result.Handled {
		t.Fatalf("pointer up result = %+v", result)
	}

	before := m.view.State.Zoom
	if result := m.page.Wheel(gfx.ScrollEvent{Target: canvas.ID, X: x, Y: y, DeltaY: -1}); !result.Handled || m.scrolls != 1 {
		t.Fatalf("wheel result = %+v after %d calls", result, m.scrolls)
	}
	if m.view.State.Zoom <= before || m.target != testCanvas {
		t.Fatalf("wheel did not zoom around the pointer: %+v target %v", m.view.State, m.target)
	}

	if result := m.page.Key(gfx.KeyEvent{Key: gfx.KeyHome, Target: canvas.ID}); !result.Handled || m.keys != 1 {
		t.Fatalf("key result = %+v after %d calls", result, m.keys)
	}
	if m.view.State.Zoom != 1 || m.view.State.CenterX != .5 || m.view.State.CenterY != .5 {
		t.Fatalf("Home did not reset the crop: %+v", m.view.State)
	}
}

// A click on a caller identity is read with Clicked after View, and the zoom
// control keeps working.
func TestClickedTranslatesCallerIdentities(t *testing.T) {
	m := new(model)
	m.wire()
	m.open(t)
	zoomIn := hit(t, m.canvas, "Zoom in")
	drawClick(m.page, m.canvas, m.viewPage, zoomIn.ID)
	if !m.view.Clicked(testZoomIn) {
		t.Fatal("Clicked did not report the clicked zoom control")
	}
	if m.view.Clicked(testZoomOut) || m.view.Clicked(testReset) {
		t.Fatal("Clicked reported an identity that was not clicked")
	}
	if m.view.State.Zoom <= 1 {
		t.Fatalf("zoom click did not update the crop: %+v", m.view.State)
	}
	draw(m.page, m.canvas, m.viewPage)
	if m.view.Clicked(testZoomIn) {
		t.Fatal("Clicked reported a click in an ordinary paint")
	}
}

// A zero identity stays omitted: the control is neither painted nor focusable,
// while the other controls keep their behavior.
func TestZeroIdentityIsOmitted(t *testing.T) {
	var view avatareditorview.Editor
	view.Bitmap = testBitmap()
	view.IDs = avatareditorview.IDs{Canvas: testCanvas, ZoomIn: testZoomIn}
	view.Labels = avatareditorview.Labels{Canvas: "Avatar crop", Reset: "Reset"}
	viewPage := func(c ui.C) { avatareditorview.View(c, "Editor", &view).Size(360, 260) }
	theme := gfx.LightTheme()
	page := ui.NewPage(ui.PageConfig{View: viewPage, Theme: &theme})
	canvas := new(gfx.Canvas)
	draw(page, canvas, viewPage)
	hit(t, canvas, "Avatar crop")
	hit(t, canvas, "Zoom in")
	if _, ok := hitLabelled(canvas, "Reset"); ok {
		t.Fatal("a control with the zero identity was painted")
	}
	if _, ok := hitLabelled(canvas, "Zoom out"); ok {
		t.Fatal("an omitted zoom control was painted")
	}
}

// A warm repaint allocates nothing: the wrapper keeps its handlers and draw
// function and the component keeps its own caches.
func TestWarmPaintAllocatesNothing(t *testing.T) {
	m := new(model)
	m.wire()
	m.open(t)
	if allocations := testing.AllocsPerRun(100, func() { draw(m.page, m.canvas, m.viewPage) }); allocations != 0 {
		t.Fatalf("warm paint allocated %v times, want 0", allocations)
	}
}

func BenchmarkViewPaint(b *testing.B) {
	m := new(model)
	m.wire()
	m.open(b)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		draw(m.page, m.canvas, m.viewPage)
	}
}
