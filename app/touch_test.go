package app

import (
	"testing"
	"time"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/interaction"
)

// The touch runtime is tested on every platform, macOS included, where a
// window does not install it.
func init() { installTouch() }

// touchFixture is a window with a button and a slider over a page that scrolls,
// which records the clicks, presses and scrolls a finger causes.
type touchFixture struct {
	driver                     *Driver
	clicks, secondary, presses int
	pressed                    bool
	scrolled                   float32
	scrolls                    int
}

const (
	touchButton = 1
	touchSlider = 2
)

func newTouchFixture(t *testing.T) *touchFixture {
	t.Helper()
	f := &touchFixture{}
	config := Config{Title: "touch", Width: 400, Height: 400}
	config.Paint = func(canvas *gfx.Canvas, size gfx.Size) {
		canvas.HitWithOptions(gfx.Item(touchButton), gfx.Rect{X: 20, Y: 20, Width: 200, Height: 60}, "Button", gfx.HitOptions{Role: gfx.RoleButton})
		canvas.HitWithOptions(gfx.Item(touchSlider), gfx.Rect{X: 20, Y: 200, Width: 200, Height: 30}, "Slider", gfx.HitOptions{Role: gfx.RoleSlider})
		f.pressed = canvas.Pressed() == gfx.Item(touchButton)
		if canvas.Clicked(gfx.Item(touchButton)) {
			f.clicks++
		}
	}
	config.OnPointer = func(event gfx.PointerEvent) gfx.EventResult {
		if event.Kind == gfx.PointerDown && event.Device == gfx.PointerTouch {
			f.presses++
			if event.Button == gfx.PointerButtonSecondary {
				f.secondary++
			}
		}
		return gfx.EventResult{}
	}
	config.OnScroll = func(event gfx.ScrollEvent) gfx.EventResult {
		f.scrolls++
		f.scrolled += event.DeltaY
		return gfx.EventResult{Handled: true, Repaint: true}
	}
	driver, err := NewDriver(config, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)
	f.driver = driver
	return f
}

func (f *touchFixture) must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// A finger lifted where it went down is a tap: one click, and no hover left
// behind, as a finger hovers only while it touches.
func TestATapClicksOnceAndLeavesNoHover(t *testing.T) {
	f := newTouchFixture(t)
	f.must(t, f.driver.TouchDown(1, 50, 50))
	if f.presses != 0 {
		t.Fatal("a finger pressed before it was clear it was not a pan")
	}
	f.must(t, f.driver.TouchUp(1, 51, 50))
	if f.clicks != 1 || f.presses != 1 {
		t.Fatalf("a tap clicked %d times after %d presses, want 1 and 1", f.clicks, f.presses)
	}
	if hovered := f.driver.Snapshot().Hovered; !hovered.IsZero() {
		t.Fatalf("a lifted finger left %v hovered", hovered)
	}
}

// A finger held still presses after the tap timeout, so the control shows it is
// pressed while the finger stays, and clicks when it lifts.
func TestAHeldFingerPressesThenClicks(t *testing.T) {
	f := newTouchFixture(t)
	f.must(t, f.driver.TouchDown(1, 50, 50))
	f.must(t, f.driver.Advance(interaction.TouchTapTimeout+time.Millisecond))
	if f.presses != 1 || !f.pressed {
		t.Fatalf("after the tap timeout: %d presses, pressed=%v; want the button pressed", f.presses, f.pressed)
	}
	f.must(t, f.driver.TouchUp(1, 50, 50))
	if f.clicks != 1 {
		t.Fatalf("lifting the held finger clicked %d times", f.clicks)
	}
}

// A finger that moves at once scrolls what is under it and never presses or
// clicks the button it started on; the content follows the finger.
func TestAPanScrollsAndNeverPresses(t *testing.T) {
	f := newTouchFixture(t)
	f.must(t, f.driver.TouchDown(1, 50, 300))
	for y := float32(290); y >= 200; y -= 10 {
		f.must(t, f.driver.TouchMove(1, 50, y))
	}
	f.must(t, f.driver.Advance(time.Second)) // a pan held long does not press either
	f.must(t, f.driver.TouchMove(1, 50, 200))
	f.must(t, f.driver.TouchUp(1, 50, 200))
	if f.presses != 0 || f.clicks != 0 {
		t.Fatalf("a pan pressed %d times and clicked %d", f.presses, f.clicks)
	}
	if f.scrolled < 100-interaction.TouchSlop || f.scrolled > 100 {
		t.Fatalf("a finger moved up 100 scrolled the content %v down, want it to follow the finger", f.scrolled)
	}
}

// A finger lifted while moving flings: the content goes on scrolling, slows
// down and stops, asking for frames only while it moves.
func TestALiftedMovingFingerFlingsAndStops(t *testing.T) {
	f := newTouchFixture(t)
	f.must(t, f.driver.TouchDown(1, 50, 380))
	y := float32(380)
	for range 10 {
		f.must(t, f.driver.Advance(10*time.Millisecond))
		y -= 20 // 2000 logical pixels a second
		f.must(t, f.driver.TouchMove(1, 50, y))
	}
	f.must(t, f.driver.TouchUp(1, 50, y))
	panned, steps := f.scrolled, f.scrolls
	f.must(t, f.driver.Advance(200*time.Millisecond))
	if f.scrolled <= panned || f.scrolls <= steps {
		t.Fatal("the content stopped when the finger lifted")
	}
	f.must(t, f.driver.Advance(10*time.Second))
	settled, settledSteps := f.scrolled, f.scrolls
	if f.driver.Transitioning() {
		t.Fatal("a fling still asks for frames after it stopped")
	}
	f.must(t, f.driver.Advance(time.Second))
	if f.scrolled != settled || f.scrolls != settledSteps {
		t.Fatal("the fling never stopped")
	}
	if f.scrolled-panned > 2000 {
		t.Fatalf("a fling at 2000/s went %v further, more than a second at full speed", f.scrolled-panned)
	}
}

// A finger held still past the long press is a secondary press, as a right
// click: what opens context menus. The primary press it began as clicks nothing.
func TestALongPressIsASecondaryPress(t *testing.T) {
	f := newTouchFixture(t)
	var events []gfx.PointerEvent
	record := f.driver.painter.onPointer
	f.driver.painter.onPointer = func(event gfx.PointerEvent) gfx.EventResult {
		events = append(events, event)
		return record(event)
	}
	f.must(t, f.driver.TouchDown(1, 50, 50))
	f.must(t, f.driver.Advance(interaction.TouchLongPress+50*time.Millisecond))
	if f.secondary != 1 || f.pressed {
		t.Fatalf("a long press made %d secondary presses and left the button pressed=%v, want 1 and released", f.secondary, f.pressed)
	}
	// The application sees the primary press end before the secondary one
	// begins: a drag it was following does not go on under the menu.
	ended := false
	for _, event := range events {
		if event.Kind == gfx.PointerExit {
			ended = true
		}
		if event.Kind == gfx.PointerDown && event.Button == gfx.PointerButtonSecondary && !ended {
			t.Fatalf("the secondary press began while the primary one was held: %+v", events)
		}
	}
	f.must(t, f.driver.TouchUp(1, 50, 50))
	if f.clicks != 0 {
		t.Fatalf("a long press also clicked %d times", f.clicks)
	}
}

// A slider takes a moving finger as a drag: it presses at once instead of
// waiting to see whether the finger pans.
func TestASliderPressesAtOnce(t *testing.T) {
	f := newTouchFixture(t)
	f.must(t, f.driver.TouchDown(1, 50, 210))
	if f.presses != 1 {
		t.Fatalf("a finger on a slider pressed %d times at once, want 1", f.presses)
	}
	f.must(t, f.driver.TouchMove(1, 120, 210))
	f.must(t, f.driver.TouchUp(1, 120, 210))
	if f.scrolls != 0 {
		t.Fatal("dragging a slider scrolled the page")
	}
}

// Two quick taps in one place are a double tap, counted as a double click.
func TestTwoQuickTapsAreADoubleTap(t *testing.T) {
	var counts []int
	f := newTouchFixture(t)
	f.driver.painter.onPointer = func(event gfx.PointerEvent) gfx.EventResult {
		if event.Kind == gfx.PointerDown {
			counts = append(counts, event.ClickCount)
		}
		return gfx.EventResult{}
	}
	f.must(t, f.driver.TouchDown(1, 50, 50))
	f.must(t, f.driver.TouchUp(1, 50, 50))
	f.must(t, f.driver.Advance(100*time.Millisecond))
	f.must(t, f.driver.TouchDown(1, 52, 51))
	f.must(t, f.driver.TouchUp(1, 52, 51))
	if len(counts) != 2 || counts[0] != 1 || counts[1] != 2 {
		t.Fatalf("two quick taps counted %v, want [1 2]", counts)
	}
}

// A fling's frames allocate nothing.
func TestAFlingStepDoesNotAllocate(t *testing.T) {
	f := newTouchFixture(t)
	f.must(t, f.driver.TouchDown(1, 50, 380))
	y := float32(380)
	for range 10 {
		f.must(t, f.driver.Advance(10*time.Millisecond))
		y -= 30
		f.must(t, f.driver.TouchMove(1, 50, y))
	}
	f.must(t, f.driver.TouchUp(1, 50, y))
	f.must(t, f.driver.Advance(50*time.Millisecond))
	if allocations := testing.AllocsPerRun(20, func() { _ = f.driver.Advance(frameInterval) }); allocations != 0 {
		t.Fatalf("a fling frame allocated %v times", allocations)
	}
}
