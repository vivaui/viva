package app

import (
	"os"
	"runtime"
	"testing"

	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/ui"
)

// controlsFixture is a painter that draws window buttons whatever the platform
// it runs on, with actions that count what they are asked to do.
type controlsFixture struct {
	painter                  *painter
	canvas                   *ui.Canvas
	minimizes, toggles, shut int
	maximized                bool
	clicks                   []ui.ID
}

func newControlsFixture(t testing.TB, mutate func(*Config)) *controlsFixture {
	t.Helper()
	fixture := &controlsFixture{canvas: &ui.Canvas{}}
	config := Config{Title: "controls", Width: 600, Height: 300, WindowMode: WindowCustomTitlebar, TitlebarHeight: 36}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		canvas.Fill(ui.Rect{Width: size.Width, Height: 36}, ui.Color{Red: 0.1, Green: 0.1, Blue: 0.12, Alpha: 1})
		canvas.HitWithLabel(ui.Item(1), ui.Rect{X: 10, Y: 4, Width: 80, Height: 28}, "Tab")
		canvas.HitWithLabel(ui.Item(2), ui.Rect{X: 10, Y: 100, Width: 80, Height: 28}, "Content")
	}
	if mutate != nil {
		mutate(&config)
	}
	onClick(&config, func(id ui.ID) { fixture.clicks = append(fixture.clicks, id) })
	normalized, err := config.normalize()
	if err != nil {
		t.Fatal(err)
	}
	painter, err := newPainter(fixture.canvas, &normalized)
	if err != nil {
		t.Fatal(err)
	}
	painter.controls = newWindowControls(&normalized)
	actions := painter.windowActions()
	actions.Minimize = func() { fixture.minimizes++ }
	actions.ToggleMaximize = func() { fixture.toggles++; fixture.maximized = !fixture.maximized }
	actions.RequestClose = func() { fixture.shut++ }
	actions.Maximized = func() bool { return fixture.maximized }
	if _, err := painter.repaint(); err != nil {
		t.Fatal(err)
	}
	fixture.painter = painter
	return fixture
}

func (fixture *controlsFixture) click(t testing.TB, x, y float32) {
	t.Helper()
	for _, kind := range []platform.PointerEventKind{platform.PointerMove, platform.PointerDown, platform.PointerUp} {
		if _, err := fixture.painter.pointer(platform.PointerEvent{Kind: kind, Button: ui.PointerButtonPrimary, X: x, Y: y}); err != nil {
			t.Fatal(err)
		}
	}
}

func (fixture *controlsFixture) hit(button int) (ui.Hit, bool) {
	for _, hit := range fixture.canvas.Hits() {
		if hit.ID == fixture.painter.controls.ids[button] {
			return hit, true
		}
	}
	return ui.Hit{}, false
}

func TestWindowButtonsSitAtTheTrailingEndOfTheStripAndAreAsTallAsIt(t *testing.T) {
	fixture := newControlsFixture(t, nil)
	for index, want := range []struct {
		id   int
		name string
	}{{windowControlMinimize, "Minimize"}, {windowControlMaximize, "Maximize"}, {windowControlClose, "Close"}} {
		hit, ok := fixture.hit(want.id)
		if !ok {
			t.Fatalf("button %q has no hit", want.name)
		}
		wantX := float32(600 - (3-index)*WindowControlWidth)
		if hit.Label != want.name || hit.Role != ui.RoleButton || hit.Bounds != (ui.Rect{X: wantX, Width: WindowControlWidth, Height: 36}) {
			t.Fatalf("%s = %+v", want.name, hit)
		}
		if hit.Layer != windowControlsLayer {
			t.Fatalf("%s is on layer %d, want the top one", want.name, hit.Layer)
		}
	}
	if leading, trailing := TitlebarInsets(); trailing != 0 && trailing != 3*WindowControlWidth || leading != 0 && leading != 70 {
		t.Fatalf("insets = %v, %v", leading, trailing)
	}
}

func TestClickingAWindowButtonActsAndNeverReachesTheApplication(t *testing.T) {
	fixture := newControlsFixture(t, nil)
	fixture.click(t, 600-3*WindowControlWidth+20, 18)
	fixture.click(t, 600-2*WindowControlWidth+20, 18)
	fixture.click(t, 600-WindowControlWidth+20, 18)
	if fixture.minimizes != 1 || fixture.toggles != 1 || fixture.shut != 1 {
		t.Fatalf("minimize %d, maximize %d, close %d; want one each", fixture.minimizes, fixture.toggles, fixture.shut)
	}
	if len(fixture.clicks) != 0 {
		t.Fatalf("the application's OnClick saw %v", fixture.clicks)
	}
	// The application's own hits still work next to them.
	fixture.click(t, 20, 18)
	fixture.click(t, 20, 110)
	if len(fixture.clicks) != 2 || fixture.clicks[0] != ui.Item(1) || fixture.clicks[1] != ui.Item(2) {
		t.Fatalf("application clicks = %v", fixture.clicks)
	}
	// Pressing a button and letting go elsewhere is not a click.
	before := fixture.minimizes
	x := float32(600 - 3*WindowControlWidth + 20)
	for _, event := range []platform.PointerEvent{
		{Kind: platform.PointerMove, X: x, Y: 18}, {Kind: platform.PointerDown, Button: ui.PointerButtonPrimary, X: x, Y: 18},
		{Kind: platform.PointerMove, X: 200, Y: 200}, {Kind: platform.PointerUp, Button: ui.PointerButtonPrimary, X: 200, Y: 200},
	} {
		if _, err := fixture.painter.pointer(event); err != nil {
			t.Fatal(err)
		}
	}
	if fixture.minimizes != before {
		t.Fatal("releasing the pointer away from the button still minimized")
	}
}

func TestMaximizeButtonShowsTheOppositeActionAndFollowsTheWindowState(t *testing.T) {
	fixture := newControlsFixture(t, nil)
	if hit, _ := fixture.hit(windowControlMaximize); hit.Label != "Maximize" {
		t.Fatalf("label = %q", hit.Label)
	}
	idle := len(fixture.canvas.Triangles()) + len(fixture.canvas.Rectangles())
	fixture.maximized = true
	if _, err := fixture.painter.repaint(); err != nil {
		t.Fatal(err)
	}
	if hit, _ := fixture.hit(windowControlMaximize); hit.Label != "Restore" {
		t.Fatalf("label = %q after the window maximized", hit.Label)
	}
	if got := len(fixture.canvas.Triangles()) + len(fixture.canvas.Rectangles()); got == idle {
		t.Fatal("the glyph did not change")
	}
}

func TestWindowButtonsPaintHoverPressedAndTheCloseColor(t *testing.T) {
	fixture := newControlsFixture(t, nil)
	close := float32(600 - WindowControlWidth + 20)
	fill := func() (ui.Color, bool) {
		for _, rectangle := range fixture.canvas.Rectangles() {
			if rectangle.Bounds.X >= 600-WindowControlWidth && (rectangle.Bounds.Width == WindowControlWidth || rectangle.Radius > 0) {
				return rectangle.Color, true
			}
		}
		return ui.Color{}, false
	}
	if _, ok := fill(); ok {
		t.Fatal("a resting button has a background")
	}
	fixture.painter.pointer(platform.PointerEvent{Kind: platform.PointerMove, X: close, Y: 18})
	hover, ok := fill()
	if !ok || hover.Red < 0.7 || hover.Alpha != 1 {
		t.Fatalf("hovered close background = %+v, %t, want the red one", hover, ok)
	}
	fixture.painter.pointer(platform.PointerEvent{Kind: platform.PointerDown, Button: ui.PointerButtonPrimary, X: close, Y: 18})
	if pressed, _ := fill(); pressed.Alpha >= hover.Alpha {
		t.Fatalf("pressed close %+v is not fainter than hover %+v", pressed, hover)
	}
}

func TestDesktopThemeDetectionAndStyling(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("desktop theme detection reads XDG_CURRENT_DESKTOP only on Linux")
	}
	orig := os.Getenv("XDG_CURRENT_DESKTOP")
	defer os.Setenv("XDG_CURRENT_DESKTOP", orig)

	os.Setenv("XDG_CURRENT_DESKTOP", "KDE")
	if detectDesktopTheme() != desktopBreeze {
		t.Errorf("expected desktopBreeze for KDE, got %v", detectDesktopTheme())
	}

	os.Setenv("XDG_CURRENT_DESKTOP", "GNOME")
	if detectDesktopTheme() != desktopAdwaita {
		t.Errorf("expected desktopAdwaita for GNOME, got %v", detectDesktopTheme())
	}

	os.Setenv("XDG_CURRENT_DESKTOP", "XFCE")
	if detectDesktopTheme() != desktopDefault {
		t.Errorf("expected desktopDefault for XFCE, got %v", detectDesktopTheme())
	}
}

func TestWindowControlsStyleIsReadAtEveryPaintAndDefaultsFollowTheClearColor(t *testing.T) {
	dark := true
	fixture := newControlsFixture(t, func(config *Config) {
		config.WindowControls = func() WindowControlsStyle {
			if dark {
				return WindowControlsStyle{Foreground: ui.Color{Red: 1, Green: 0, Blue: 0, Alpha: 1}}
			}
			return WindowControlsStyle{Foreground: ui.Color{Red: 0, Green: 0, Blue: 1, Alpha: 1}}
		}
	})
	glyphColor := func() ui.Color {
		for _, triangle := range fixture.canvas.Triangles() {
			if triangle.Points[0].X >= 600-3*WindowControlWidth {
				return triangle.Color
			}
		}
		for _, rectangle := range fixture.canvas.Rectangles() {
			if rectangle.Bounds.X >= 600-3*WindowControlWidth && rectangle.Bounds.Width < WindowControlWidth {
				return rectangle.Color
			}
		}
		return ui.Color{}
	}
	if got := glyphColor(); got.Red != 1 {
		t.Fatalf("glyph color = %+v, want the first style", got)
	}
	dark = false
	if _, err := fixture.painter.repaint(); err != nil {
		t.Fatal(err)
	}
	if got := glyphColor(); got.Blue != 1 {
		t.Fatalf("glyph color = %+v after the style changed", got)
	}

	light := (WindowControlsStyle{}).withDefaults(Color{Red: 1, Green: 1, Blue: 1, Alpha: 1})
	night := (WindowControlsStyle{}).withDefaults(Color{Red: 0.05, Green: 0.06, Blue: 0.09, Alpha: 1})
	if light.Foreground.Red >= 0.5 || night.Foreground.Red <= 0.5 {
		t.Fatalf("default ink on light %+v and on dark %+v", light.Foreground, night.Foreground)
	}
	named := (WindowControlsStyle{Close: ui.Color{Red: 1, Alpha: 1}, Restore: "还原"}).withDefaults(Color{})
	if named.Close.Red != 1 || named.Close.Green != 0 || named.Restore != "还原" || named.Minimize != "Minimize" {
		t.Fatalf("explicit fields were overwritten or defaults missing: %+v", named)
	}
}

// A dialog traps focus in its own scope and blocks what lies under its scrim;
// the buttons must stay usable, and the dialog's content must still be too.
func TestWindowButtonsStayUsableUnderAModalDialog(t *testing.T) {
	fixture := newControlsFixture(t, func(config *Config) {
		paint := config.Paint
		config.Paint = func(canvas *ui.Canvas, size ui.Size) {
			paint(canvas, size)
			canvas.PushLayer(600)
			canvas.PushFocusScope(ui.Item(9))
			canvas.Block(ui.Rect{Width: size.Width, Height: size.Height})
			canvas.HitWithLabel(ui.Item(9), ui.Rect{X: 200, Y: 100, Width: 100, Height: 30}, "OK")
			canvas.PopFocusScope()
			canvas.TrapFocus(ui.Item(9))
			canvas.PopLayer()
		}
	})
	fixture.click(t, 600-WindowControlWidth+20, 18)
	if fixture.shut != 1 {
		t.Fatal("the close button did not work under a modal dialog")
	}
	fixture.click(t, 20, 18) // the application's tab, under the scrim
	if len(fixture.clicks) != 0 {
		t.Fatalf("a click under the scrim reached the application: %v", fixture.clicks)
	}
	fixture.click(t, 250, 110)
	if len(fixture.clicks) != 1 || fixture.clicks[0] != ui.Item(9) {
		t.Fatalf("the dialog's own button stopped working: %v", fixture.clicks)
	}
}

func TestWindowButtonsDoNothingUntilTheAdapterFillsItsActions(t *testing.T) {
	fixture := newControlsFixture(t, nil)
	*fixture.painter.controls.actions = platform.WindowActions{}
	fixture.click(t, 600-WindowControlWidth+20, 18)
	fixture.painter.controls.actions = nil
	fixture.click(t, 600-WindowControlWidth+20, 18)
	if fixture.shut != 0 {
		t.Fatal("a button acted with no action behind it")
	}
}

func TestWindowActionsExistOnlyWhereVivaDrawsTheButtons(t *testing.T) {
	var canvas ui.Canvas
	painter := newTestPainter(t, &canvas, Config{Title: "plain", Width: 100, Height: 100, WindowMode: WindowCustomTitlebar})
	if (painter.windowActions() != nil) != controlsDrawnByViva {
		t.Fatalf("actions %v, controlsDrawnByViva %t", painter.windowActions(), controlsDrawnByViva)
	}
	painter = newTestPainter(t, &canvas, Config{Title: "titled", Width: 100, Height: 100})
	if painter.windowActions() != nil {
		t.Fatal("an ordinary window got window-button actions")
	}
}

func TestSteadyStatePaintOfTheWindowButtonsDoesNotAllocate(t *testing.T) {
	fixture := newControlsFixture(t, nil)
	if _, err := fixture.painter.repaint(); err != nil {
		t.Fatal(err)
	}
	allocations := testing.AllocsPerRun(200, func() {
		fixture.canvas.Reset()
		fixture.painter.controls.paint(fixture.canvas, ui.Size{Width: 600, Height: 300})
	})
	if allocations != 0 {
		t.Fatalf("painting the buttons allocated %.1f objects", allocations)
	}
}

func BenchmarkWindowControlsPaint(b *testing.B) {
	fixture := newControlsFixture(b, nil)
	b.ReportAllocs()
	for b.Loop() {
		fixture.canvas.Reset()
		fixture.painter.controls.paint(fixture.canvas, ui.Size{Width: 600, Height: 300})
	}
}

// A window whose frame Viva draws has resize edges; the pointer over one shows
// the cursor that says so, and leaving it restores the usual one.
func TestPointerOverAWindowEdgeShowsTheResizeCursor(t *testing.T) {
	fixture := newControlsFixture(t, nil)
	var cursors []Cursor
	fixture.painter.setCursor = func(cursor Cursor) error { cursors = append(cursors, cursor); return nil }
	move := func(x, y float32) {
		t.Helper()
		if _, err := fixture.painter.pointer(platform.PointerEvent{Kind: platform.PointerMove, X: x, Y: y}); err != nil {
			t.Fatal(err)
		}
	}
	last := func() Cursor {
		if len(cursors) == 0 {
			return CursorArrow
		}
		return cursors[len(cursors)-1]
	}
	for _, test := range []struct {
		name string
		x, y float32
		want Cursor
	}{
		{"left", 2, 150, CursorResizeHorizontal},
		{"right", 597, 150, CursorResizeHorizontal},
		{"bottom", 300, 297, CursorResizeVertical},
		{"bottom right", 597, 297, CursorResizeNWSE},
		{"bottom left", 2, 297, CursorResizeNESW},
		{"top edge of the empty strip", 300, 2, CursorResizeVertical},
		{"interior", 300, 150, CursorArrow},
	} {
		move(test.x, test.y)
		if last() != test.want {
			t.Errorf("%s: cursor = %d, want %d", test.name, last(), test.want)
		}
	}

	// The top edge yields to a control: the close button keeps its own cursor.
	move(600-20, 2)
	if last() == CursorResizeVertical {
		t.Error("the top edge over a window button showed the resize cursor")
	}

	// The top edge over a titlebar move region must still show resize cursor.
	fixture.canvas.MoveRegion(ui.Rect{X: 0, Y: 0, Width: 600, Height: 40})
	fixture.painter.paintCanvas()
	fixture.painter.syncRegions()
	move(300, 2)
	if last() != CursorResizeVertical {
		t.Errorf("the top edge over a titlebar move region showed cursor %d, want CursorResizeVertical", last())
	}

	// Leaving the window clears it.
	move(2, 150)
	fixture.painter.pointer(platform.PointerEvent{Kind: platform.PointerExit})
	if last() != CursorArrow {
		t.Errorf("after the pointer left, cursor = %d", last())
	}

	// A maximized window has no edges.
	fixture.maximized = true
	move(2, 150)
	if last() == CursorResizeHorizontal {
		t.Error("a maximized window showed a resize cursor")
	}
}

func TestMovingAlongAnEdgeDoesNotRepaint(t *testing.T) {
	fixture := newControlsFixture(t, nil)
	fixture.painter.setCursor = func(Cursor) error { return nil }
	fixture.painter.pointer(platform.PointerEvent{Kind: platform.PointerMove, X: 2, Y: 120})
	frame, err := fixture.painter.pointer(platform.PointerEvent{Kind: platform.PointerMove, X: 2, Y: 140})
	if err != nil {
		t.Fatal(err)
	}
	if len(frame.Rectangles) != 0 || len(frame.TextLabels) != 0 {
		t.Fatalf("a move along the edge produced a frame: %+v", frame)
	}
}
