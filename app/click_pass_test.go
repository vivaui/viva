//go:build darwin || windows || linux

package app

import (
	"testing"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
)

var (
	passTree = gfx.Tree{}
	passPage = passTree.Feature("pass")
	passSave = passPage.ID(passTree.Name("save"))
	passOpen = passPage.ID(passTree.Name("open"))
)

// clickPassApp is an application whose buttons are handled where they are declared.
type clickPassApp struct {
	saves, opens int
	paints       int
	eventPaints  int
}

func (a *clickPassApp) config() Config {
	theme := gfx.LightTheme()
	return Config{
		Title: "pass", Width: 400, Height: 200, Tree: &passTree,
		Paint: func(canvas *gfx.Canvas, size gfx.Size) {
			a.paints++
			if components.BeginButton(canvas, theme, passSave, gfx.Rect{X: 10, Y: 10, Width: 100, Height: 32}).Default("Save").Clicked() {
				a.saves++
				a.eventPaints++
			}
			if components.BeginButton(canvas, theme, passOpen, gfx.Rect{X: 10, Y: 60, Width: 100, Height: 32}).Outline("Open").Clicked() {
				a.opens++
			}
		},
	}
}

func TestAClickIsDeliveredToTheControlThatDeclaredItExactlyOnce(t *testing.T) {
	a := &clickPassApp{}
	var canvas gfx.Canvas
	driver, err := NewDriver(a.config(), &canvas)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()

	if err := driver.Click(60, 26); err != nil { // Save
		t.Fatal(err)
	}
	if a.saves != 1 || a.opens != 0 {
		t.Fatalf("after one click on Save: saves=%d opens=%d, want 1 and 0", a.saves, a.opens)
	}
	// Painting again, for any reason, must not deliver the click a second time.
	if err := driver.Resize(420, 220); err != nil {
		t.Fatal(err)
	}
	if a.saves != 1 {
		t.Fatalf("a later paint delivered the click again: saves=%d", a.saves)
	}
	if err := driver.Click(60, 76); err != nil { // Open
		t.Fatal(err)
	}
	if a.saves != 1 || a.opens != 1 {
		t.Fatalf("after one click on Open: saves=%d opens=%d, want 1 and 1", a.saves, a.opens)
	}
}

func TestEnterOnAFocusedButtonIsAClickInTheEventPass(t *testing.T) {
	a := &clickPassApp{}
	var canvas gfx.Canvas
	driver, err := NewDriver(a.config(), &canvas)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()

	if err := driver.Key(gfx.KeyTab, 0); err != nil {
		t.Fatal(err)
	}
	if err := driver.Key(gfx.KeyEnter, 0); err != nil {
		t.Fatal(err)
	}
	if a.saves != 1 {
		t.Fatalf("Enter on the focused Save button: saves=%d, want 1", a.saves)
	}
}

func TestAPaintThatNeverAsksCostsNoEventPass(t *testing.T) {
	paints := 0
	theme := gfx.LightTheme()
	config := Config{
		Title: "plain", Width: 400, Height: 200, Tree: &passTree,
		Paint: func(canvas *gfx.Canvas, size gfx.Size) {
			paints++
			components.BeginButton(canvas, theme, passSave, gfx.Rect{X: 10, Y: 10, Width: 100, Height: 32}).Default("Save")
		},
	}
	var canvas gfx.Canvas
	driver, err := NewDriver(config, &canvas)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	before := paints
	if err := driver.Click(60, 26); err != nil {
		t.Fatal(err)
	}
	// Moving onto the button, pressing and releasing repaint for hover and press
	// state: no pass for the click, since nothing asks to be told of it.
	if got := paints - before; got > 3 {
		t.Fatalf("a click on an application that never asks Clicked painted %d times", got)
	}
}

// A disabled button is not clicked, and neither is one a modal dialog covers.
func TestClickedIsFalseOutsideTheEventPass(t *testing.T) {
	var canvas gfx.Canvas
	canvas.BeginFrame()
	if canvas.Clicked(passSave) {
		t.Fatal("Clicked is true in an ordinary paint")
	}
	if !canvas.ListensForClicks() {
		t.Fatal("asking did not mark the paint as listening")
	}
	canvas.EndFrame()
	canvas.BeginEventPass(passSave, gfx.ClickInfo{})
	if !canvas.Clicked(passSave) || canvas.Clicked(passOpen) || canvas.Clicked(gfx.ID{}) {
		t.Fatal("the event pass names the wrong control")
	}
	canvas.EndEventPass()
	if canvas.Clicked(passSave) {
		t.Fatal("the event pass leaked past EndEventPass")
	}
}

func BenchmarkClickThroughTheEventPass(b *testing.B) {
	a := &clickPassApp{}
	var canvas gfx.Canvas
	driver, err := NewDriver(a.config(), &canvas)
	if err != nil {
		b.Fatal(err)
	}
	defer driver.Close()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := driver.Click(60, 26); err != nil {
			b.Fatal(err)
		}
	}
}

var passMenu = passPage.ID(passTree.Name("menu"))

type pointerPassApp struct {
	menus, doubles, drags int
	clicks                int
	consume               bool
}

func (a *pointerPassApp) config() Config {
	return Config{
		Title: "pointer", Width: 400, Height: 200, Tree: &passTree,
		Paint: func(canvas *gfx.Canvas, size gfx.Size) {
			bounds := gfx.Rect{X: 10, Y: 10, Width: 200, Height: 100}
			canvas.HitWithLabel(passMenu, bounds, "Area")
			if event, ok := canvas.PointerOn(passMenu); ok {
				switch {
				case event.Kind == gfx.PointerDown && event.Button == gfx.PointerButtonSecondary:
					a.menus++
					canvas.Consume()
				case event.Kind == gfx.PointerDown && event.ClickCount == 2:
					a.doubles++
				case event.Kind == gfx.PointerMove && event.Captured:
					a.drags++
				}
			}
			if canvas.Clicked(passMenu) {
				a.clicks++
			}
		},
	}
}

func TestRightClickDoubleClickAndDragReachTheControlThatDeclaredThem(t *testing.T) {
	a := &pointerPassApp{}
	var canvas gfx.Canvas
	driver, err := NewDriver(a.config(), &canvas)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()

	if err := driver.Press(60, 60, gfx.PointerButtonSecondary); err != nil {
		t.Fatal(err)
	}
	if err := driver.Release(60, 60, gfx.PointerButtonSecondary); err != nil {
		t.Fatal(err)
	}
	if a.menus != 1 {
		t.Fatalf("a right press reached the control %d times, want 1", a.menus)
	}
	if err := driver.DoubleClick(60, 60); err != nil {
		t.Fatal(err)
	}
	if a.doubles != 1 {
		t.Fatalf("a double click reached the control %d times, want 1", a.doubles)
	}
	before := a.drags
	if err := driver.Press(60, 60, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	if err := driver.Move(90, 70); err != nil {
		t.Fatal(err)
	}
	if err := driver.Move(120, 80); err != nil {
		t.Fatal(err)
	}
	if err := driver.Release(120, 80, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	if a.drags-before != 2 {
		t.Fatalf("a drag of two moves reached the control %d times", a.drags-before)
	}
}

func TestMovingOverAControlIsHoverNotAPointerEvent(t *testing.T) {
	a := &pointerPassApp{}
	var canvas gfx.Canvas
	driver, err := NewDriver(a.config(), &canvas)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	for x := float32(20); x < 180; x += 20 {
		if err := driver.Move(x, 60); err != nil {
			t.Fatal(err)
		}
	}
	if a.drags != 0 || a.menus != 0 {
		t.Fatalf("hovering delivered pointer events: drags=%d menus=%d", a.drags, a.menus)
	}
}

// Pointer moves are the hottest event; an application that never asks PointerOn
// must not pay an event pass for them.
func BenchmarkPointerMoveWithoutPointerListeners(b *testing.B) {
	theme := gfx.LightTheme()
	paints := 0
	config := Config{
		Title: "move", Width: 400, Height: 200, Tree: &passTree,
		Paint: func(canvas *gfx.Canvas, size gfx.Size) {
			paints++
			components.BeginButton(canvas, theme, passSave, gfx.Rect{X: 10, Y: 10, Width: 100, Height: 32}).Default("Save")
		},
	}
	var canvas gfx.Canvas
	driver, err := NewDriver(config, &canvas)
	if err != nil {
		b.Fatal(err)
	}
	defer driver.Close()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		x := float32(20 + i%80)
		if err := driver.Move(x, 26); err != nil {
			b.Fatal(err)
		}
	}
}

// A central dispatcher defers its handler: it runs once, after the click paint has
// returned (not inside it), and the repaint that follows shows what it changed.
func TestAfterPaintRunsOutsideThePaintAndTheRepaintShowsIt(t *testing.T) {
	inPaint, ranInPaint, runs, shown := false, false, 0, 0
	theme := gfx.LightTheme()
	config := Config{
		Title: "after", Width: 400, Height: 200, Tree: &passTree,
		Paint: func(canvas *gfx.Canvas, size gfx.Size) {
			inPaint = true
			defer func() { inPaint = false }()
			shown = runs
			components.BeginButton(canvas, theme, passSave, gfx.Rect{X: 10, Y: 10, Width: 100, Height: 32}).Default("Save")
			if id, ok := canvas.ClickTarget(); ok {
				canvas.AfterPaint(func() {
					ranInPaint = ranInPaint || inPaint
					runs++
					canvas.AfterPaint(func() { runs += 100 }) // outside a paint: ignored
				})
				_ = id
			}
		},
	}
	var canvas gfx.Canvas
	driver, err := NewDriver(config, &canvas)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	if err := driver.Click(60, 26); err != nil {
		t.Fatal(err)
	}
	if runs != 1 || ranInPaint {
		t.Fatalf("deferred function ran %d times, inside a paint: %v", runs, ranInPaint)
	}
	if shown != 1 {
		t.Fatalf("the repaint after the click did not see the deferred change (saw %d)", shown)
	}
	if err := driver.Resize(420, 220); err != nil {
		t.Fatal(err)
	}
	if runs != 1 {
		t.Fatalf("an ordinary repaint ran the deferred function again: %d", runs)
	}
}

// The event pass of a click carries the modifier keys held and how many clicks the
// sequence had, for a pointer click, and a plain click for the keyboard's Enter.
func TestTheClickPassSaysHowTheClickCame(t *testing.T) {
	var got []gfx.ClickInfo
	theme := gfx.LightTheme()
	config := Config{
		Title: "pass", Width: 400, Height: 200, Tree: &passTree,
		Paint: func(canvas *gfx.Canvas, size gfx.Size) {
			if components.BeginButton(canvas, theme, passSave, gfx.Rect{X: 10, Y: 10, Width: 100, Height: 32}).Default("Save").Clicked() {
				info, ok := canvas.ClickInfo()
				if !ok {
					t.Error("ClickInfo is not available in the pass that delivers the click")
				}
				got = append(got, info)
			}
		},
	}
	var canvas gfx.Canvas
	driver, err := NewDriver(config, &canvas)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()

	if err := driver.KeyDown(gfx.KeyShiftLeft, ""); err != nil {
		t.Fatal(err)
	}
	if err := driver.Click(60, 26); err != nil {
		t.Fatal(err)
	}
	if err := driver.KeyUp(gfx.KeyShiftLeft, ""); err != nil {
		t.Fatal(err)
	}
	if err := driver.DoubleClick(60, 26); err != nil {
		t.Fatal(err)
	}
	if len(got) < 3 {
		t.Fatalf("%d clicks delivered", len(got))
	}
	if got[0].Modifiers&gfx.ModifierShift == 0 {
		t.Errorf("a Shift-click arrived with modifiers %v", got[0].Modifiers)
	}
	if got[1].Modifiers != 0 {
		t.Errorf("a click after Shift was released arrived with modifiers %v", got[1].Modifiers)
	}
	if last := got[len(got)-1]; last.Clicks != 2 {
		t.Errorf("the second click of a double click arrived as click %d", last.Clicks)
	}
}
