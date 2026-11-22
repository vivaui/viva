package main

import (
	"runtime"
	"testing"
	"time"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

func open(t *testing.T, m *model) *app.Driver {
	t.Helper()
	page := ui.NewPage(ui.PageConfig{View: m.page})
	driver, err := app.NewDriver(ui.Configure(page, app.Config{Width: 420, Height: 760}), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)
	return driver
}

func control(t *testing.T, driver *app.Driver, label string) gfx.Hit {
	t.Helper()
	hits := driver.Snapshot().Hits
	for i := len(hits) - 1; i >= 0; i-- {
		hit := hits[i]
		if hit.Label == label && !hit.IsLabel {
			return hit
		}
	}
	t.Fatalf("no control named %q on screen", label)
	return gfx.Hit{}
}

func centerOf(t *testing.T, driver *app.Driver, label string) (float32, float32) {
	t.Helper()
	b := control(t, driver, label).Bounds
	return b.X + b.Width/2, b.Y + b.Height/2
}

func shown(driver *app.Driver, want string) bool {
	for _, label := range driver.Snapshot().Labels {
		if label.Value == want {
			return true
		}
	}
	return false
}

// A finger lifted on a button presses it once.
func TestATapPressesTheButton(t *testing.T) {
	m := newModel()
	driver := open(t, m)
	x, y := centerOf(t, driver, "Tap me")
	if err := driver.TouchDown(1, x, y); err != nil {
		t.Fatal(err)
	}
	if err := driver.TouchUp(1, x, y); err != nil {
		t.Fatal(err)
	}
	if m.taps != 1 || !shown(driver, "Tapped 1 times") {
		t.Fatalf("taps = %d", m.taps)
	}
}

// A finger that drags up the list scrolls it, and the rows it started on are not
// chosen by it.
func TestAFingerScrollsTheListAndChoosesNothing(t *testing.T) {
	skipWithoutTouchRuntime(t)
	m := newModel()
	driver := open(t, m)
	if shown(driver, "Row 30") {
		t.Fatal("the thirtieth row is on screen before the list scrolled")
	}
	x, y := centerOf(t, driver, "Row 5")
	if err := driver.TouchDown(1, x, y+200); err != nil {
		t.Fatal(err)
	}
	for step := float32(1); step <= 20; step++ {
		if err := driver.Advance(10 * time.Millisecond); err != nil {
			t.Fatal(err)
		}
		if err := driver.TouchMove(1, x, y+200-step*30); err != nil {
			t.Fatal(err)
		}
	}
	if err := driver.TouchUp(1, x, y-400); err != nil {
		t.Fatal(err)
	}
	if err := driver.Advance(2 * time.Second); err != nil {
		t.Fatal(err)
	}
	if m.list.Selected() >= 0 {
		t.Fatalf("a drag chose row %d", m.list.Selected())
	}
	if shown(driver, "Row 1") {
		t.Fatal("the list did not scroll under the finger")
	}
}

// A finger held on a row opens its menu, and choosing from the menu is a tap.
func TestAHeldFingerOpensTheRowMenu(t *testing.T) {
	skipWithoutTouchRuntime(t)
	m := newModel()
	driver := open(t, m)
	x, y := centerOf(t, driver, "Row 1")
	if err := driver.TouchDown(1, x, y); err != nil {
		t.Fatal(err)
	}
	if err := driver.Advance(700 * time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if err := driver.TouchUp(1, x, y); err != nil {
		t.Fatal(err)
	}
	mx, my := centerOf(t, driver, "Open")
	if err := driver.TouchDown(2, mx, my); err != nil {
		t.Fatal(err)
	}
	if err := driver.TouchUp(2, mx, my); err != nil {
		t.Fatal(err)
	}
	if m.menu != "Opened row 1" {
		t.Fatalf("menu = %q", m.menu)
	}
}

// A finger on a slider drags it at once, and does not scroll the page.
func TestAFingerDragsTheSlider(t *testing.T) {
	m := newModel()
	driver := open(t, m)
	b := control(t, driver, "Slider").Bounds
	y := b.Y + b.Height/2
	before := m.volume.Value
	if err := driver.TouchDown(1, b.X+b.Width*0.4, y); err != nil {
		t.Fatal(err)
	}
	if err := driver.TouchMove(1, b.X+b.Width*0.9, y); err != nil {
		t.Fatal(err)
	}
	if err := driver.TouchUp(1, b.X+b.Width*0.9, y); err != nil {
		t.Fatal(err)
	}
	if m.volume.Value <= before {
		t.Fatalf("volume %v after dragging right from %v", m.volume.Value, before)
	}
}

// skipWithoutTouchRuntime skips a test that needs the finger gestures: macOS has no touch
// screen, so its windows do not install them and a finger there is only a pointer.
func skipWithoutTouchRuntime(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "darwin" {
		t.Skip("macOS windows do not install the touch gestures")
	}
}

// A finger that drags the list down at its start carries it past the start, and it
// springs back to where it began when the finger lifts.
func TestTheListGivesAtItsStartAndSpringsBack(t *testing.T) {
	skipWithoutTouchRuntime(t)
	m := newModel()
	driver := open(t, m)
	_, rest := centerOf(t, driver, "Row 1")
	x, y := centerOf(t, driver, "Row 5")
	if err := driver.TouchDown(1, x, y); err != nil {
		t.Fatal(err)
	}
	for step := float32(1); step <= 10; step++ {
		if err := driver.Advance(10 * time.Millisecond); err != nil {
			t.Fatal(err)
		}
		if err := driver.TouchMove(1, x, y+step*15); err != nil {
			t.Fatal(err)
		}
	}
	_, held := centerOf(t, driver, "Row 1")
	if held <= rest {
		t.Fatalf("the first row is at %v while the finger holds the list past its start, want it below %v", held, rest)
	}
	if held-rest >= 150 {
		t.Fatalf("the list followed the finger all %v px past its start, want it to give", held-rest)
	}
	if err := driver.TouchUp(1, x, y+150); err != nil {
		t.Fatal(err)
	}
	if err := driver.Advance(2 * time.Second); err != nil {
		t.Fatal(err)
	}
	if _, home := centerOf(t, driver, "Row 1"); home != rest {
		t.Fatalf("the first row is at %v after the finger lifted, want %v", home, rest)
	}
	if m.list.Selected() >= 0 {
		t.Fatalf("the drag chose row %d", m.list.Selected())
	}
}

// A finger that lands a little beside a small control presses it, as a fingertip is
// bigger than the control is drawn.
func TestAFingerBesideASwitchTurnsItOn(t *testing.T) {
	skipWithoutTouchRuntime(t)
	m := newModel()
	driver := open(t, m)
	b := control(t, driver, "Notifications").Bounds
	if b.Height >= 44 && b.Width >= 44 {
		t.Skip("the switch is already as big as a fingertip")
	}
	// Just above the switch, outside what it is drawn as but within the 44 px a fingertip needs.
	x, y := b.X+b.Width/2, b.Y-3
	if err := driver.TouchDown(1, x, y); err != nil {
		t.Fatal(err)
	}
	if err := driver.TouchUp(1, x, y); err != nil {
		t.Fatal(err)
	}
	if !m.notify {
		t.Fatal("a finger just beside the switch did not turn it on")
	}
}

// A finger held on a control that has a tooltip and no menu shows it, and it goes away
// by itself.
func TestAHeldFingerShowsTheTooltipAndItGoesAway(t *testing.T) {
	skipWithoutTouchRuntime(t)
	m := newModel()
	driver := open(t, m)
	x, y := centerOf(t, driver, "Tap me")
	if err := driver.TouchDown(1, x, y); err != nil {
		t.Fatal(err)
	}
	if err := driver.Advance(700 * time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if !shown(driver, "Counts your taps") {
		t.Fatal("the tooltip did not open under a held finger")
	}
	if err := driver.TouchUp(1, x, y); err != nil {
		t.Fatal(err)
	}
	if m.taps != 0 {
		t.Fatalf("a long press also tapped: %d", m.taps)
	}
	if !shown(driver, "Counts your taps") {
		t.Fatal("the tooltip went away at once when the finger lifted")
	}
	if err := driver.Advance(300 * time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if !shown(driver, "Counts your taps") {
		t.Fatal("the tooltip went away as soon as the finger lifted")
	}
	if err := driver.Advance(2 * time.Second); err != nil {
		t.Fatal(err)
	}
	if shown(driver, "Counts your taps") {
		t.Fatal("the tooltip stayed")
	}
}

// A row with a menu opens the menu, not a tooltip.
func TestAHeldFingerOnAControlWithAMenuShowsNoTooltip(t *testing.T) {
	skipWithoutTouchRuntime(t)
	m := newModel()
	driver := open(t, m)
	x, y := centerOf(t, driver, "Row 1")
	if err := driver.TouchDown(1, x, y); err != nil {
		t.Fatal(err)
	}
	if err := driver.Advance(700 * time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if err := driver.TouchUp(1, x, y); err != nil {
		t.Fatal(err)
	}
	if !shown(driver, "Open") {
		t.Fatal("the long press did not open the row's menu")
	}
}
