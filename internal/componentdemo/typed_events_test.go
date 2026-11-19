package componentdemo

import (
	"testing"
	"time"

	"github.com/vivaui/viva/gfx"
)

func TestTypedEventDispatchDefaultsToUnhandled(t *testing.T) {
	state := newTestState(t, "button", "default")
	if result := state.Pointer(gfx.PointerEvent{Kind: gfx.PointerDown, Target: buttonPress.At(0)}); result != (gfx.EventResult{}) {
		t.Fatalf("pointer result = %+v", result)
	}
	if result := state.Key(gfx.KeyEvent{Key: gfx.KeyRight, Target: buttonPress.At(0)}); result != (gfx.EventResult{}) {
		t.Fatalf("key result = %+v", result)
	}
	if result := state.Scroll(gfx.ScrollEvent{DeltaY: 40, Target: buttonPress.At(0)}); result != (gfx.EventResult{}) {
		t.Fatalf("scroll result = %+v", result)
	}
}

// paintAt paints state at size on canvas, so the demo knows where its control is; a
// pointer event carries coordinates, and the demo reads them against that placement.
func paintAt(canvas *gfx.Canvas, state *State, size gfx.Size) {
	canvas.SetMeasurer(measureTestText)
	paintFrame(canvas, state, size, time.Second, true)
}

func TestSliderPointerCoordinatesUpdateCallerState(t *testing.T) {
	state := newTestState(t, "slider", "default")
	var canvas gfx.Canvas
	paintAt(&canvas, state, testSize)
	track := state.slider.placed
	right, middle := track.X+track.Width, track.X+track.Width/2
	result := state.Pointer(gfx.PointerEvent{
		Kind: gfx.PointerDown, Button: gfx.PointerButtonPrimary,
		X: right, Y: track.Y, Target: sliderPrimary,
	})
	if !result.Handled || !result.Repaint || result.Focus != sliderPrimary || state.values[0] != 100 {
		t.Fatalf("down result=%+v value=%d", result, state.values[0])
	}
	result = state.Pointer(gfx.PointerEvent{
		Kind: gfx.PointerMove, X: middle, Y: track.Y, Target: sliderPrimary, Captured: true,
	})
	if !result.Handled || !result.Repaint || state.values[0] != 50 {
		t.Fatalf("captured move result=%+v value=%d", result, state.values[0])
	}
	result = state.Pointer(gfx.PointerEvent{Kind: gfx.PointerUp, Target: sliderPrimary, Captured: true})
	if !result.Handled || result.Repaint {
		t.Fatalf("up result=%+v", result)
	}
	click(&canvas, state, sliderPrimary)
	if state.values[0] != 60 {
		t.Fatalf("click value=%d", state.values[0])
	}
}

func TestSliderKeyboardUpdatesCallerState(t *testing.T) {
	state := newTestState(t, "slider", "default")
	result := state.Key(gfx.KeyEvent{Key: gfx.KeyUp, Target: sliderPrimary})
	if !result.Handled || !result.Repaint || state.values[0] != 50 {
		t.Fatalf("up result=%+v value=%d", result, state.values[0])
	}
	result = state.Key(gfx.KeyEvent{Key: gfx.KeyHome, Target: sliderPrimary})
	if !result.Handled || !result.Repaint || state.values[0] != 0 {
		t.Fatalf("home result=%+v value=%d", result, state.values[0])
	}
	if result := state.Key(gfx.KeyEvent{Key: gfx.KeyUp, Target: sliderDisabled}); result != (gfx.EventResult{}) {
		t.Fatalf("disabled slider result=%+v", result)
	}
}

func TestRangeSliderKeyboardKeepsThumbsApart(t *testing.T) {
	state := newTestState(t, "slider", "range")
	result := state.Key(gfx.KeyEvent{Key: gfx.KeyEnd, Target: sliderLower})
	if !result.Handled || !result.Repaint || state.values[0] != 70 {
		t.Fatalf("end lower result=%+v values=%v", result, state.values[:2])
	}
	result = state.Key(gfx.KeyEvent{Key: gfx.KeyUp, Target: sliderUpper})
	if !result.Handled || !result.Repaint || state.values[1] != 80 || state.slider.thumbs[1].Value != 80 {
		t.Fatalf("up upper result=%+v values=%v thumbs=%+v", result, state.values[:2], state.slider.thumbs)
	}
}

func TestPointerGeometryUsesResponsiveViewport(t *testing.T) {
	narrow := gfx.Size{Width: 320, Height: 340}
	slider := newTestState(t, "slider", "default")
	var canvas gfx.Canvas
	paintAt(&canvas, slider, narrow)
	if track := slider.slider.placed; track.X+track.Width > narrow.Width {
		t.Fatalf("slider track %+v is wider than the window", track)
	}
	track := slider.slider.placed
	result := slider.Pointer(gfx.PointerEvent{
		Kind: gfx.PointerDown, Button: gfx.PointerButtonPrimary, X: track.X + track.Width,
		Viewport: narrow, Target: sliderPrimary,
	})
	if !result.Handled || slider.values[0] != 100 {
		t.Fatalf("responsive slider result=%+v value=%d", result, slider.values[0])
	}

	resizable := newTestState(t, "resizable", "horizontal")
	paintAt(&canvas, resizable, narrow)
	if position := resizableDemoFor(resizable).state.Pos; position > narrow.Width {
		t.Fatalf("responsive splitter position %.0f is wider than the window", position)
	}
}

func TestRangeSliderPointerCannotCrossOtherThumb(t *testing.T) {
	state := newTestState(t, "slider", "range")
	var canvas gfx.Canvas
	paintAt(&canvas, state, testSize)
	track := state.slider.placed
	result := state.Pointer(gfx.PointerEvent{Kind: gfx.PointerDown, Button: gfx.PointerButtonPrimary, X: track.X + track.Width, Target: sliderLower})
	if !result.Handled || state.values[0] != state.values[1] || state.slider.thumbs[0].Value != float32(state.values[0]) {
		t.Fatalf("lower crossing result=%+v values=%v thumbs=%+v", result, state.values[:2], state.slider.thumbs)
	}
	click(&canvas, state, sliderUpper)
	if state.values[0] != 40 || state.values[1] != 90 || state.slider.thumbs[1].Value != 90 {
		t.Fatalf("range preset values=%v thumbs=%+v", state.values[:2], state.slider.thumbs)
	}
}

func TestResizableUsesCapturedPointerDrag(t *testing.T) {
	state := newTestState(t, "resizable", "horizontal")
	var canvas gfx.Canvas
	paintAt(&canvas, state, testSize)
	demo := resizableDemoFor(state)
	split := findHit(t, &canvas, resizableHandle).Bounds
	center := split.X + split.Width/2
	pointer := func(event gfx.PointerEvent) {
		canvas.BeginPointerPass(event)
		paintAt(&canvas, state, testSize)
		canvas.EndPointerPass()
	}
	// A move without a press does not start a drag.
	pointer(gfx.PointerEvent{Kind: gfx.PointerMove, X: center + 40, Viewport: testSize, Target: resizableHandle})
	if demo.state.Dragging() || demo.state.Pos != 260 {
		t.Fatalf("uncaptured move dragged=%t position=%.0f", demo.state.Dragging(), demo.state.Pos)
	}
	pointer(gfx.PointerEvent{Kind: gfx.PointerDown, Button: gfx.PointerButtonPrimary, X: center, Viewport: testSize, Target: resizableHandle})
	if !demo.state.Dragging() {
		t.Fatal("a press on the divider did not start a drag")
	}
	pointer(gfx.PointerEvent{Kind: gfx.PointerMove, X: center + 40, Viewport: testSize, Target: resizableHandle, Captured: true})
	if demo.state.Pos <= 260 {
		t.Fatalf("captured move left position=%.0f", demo.state.Pos)
	}
	pointer(gfx.PointerEvent{Kind: gfx.PointerUp, X: center + 40, Viewport: testSize, Target: resizableHandle, Captured: true})
	if demo.state.Dragging() {
		t.Fatal("the drag did not stop on release")
	}
}

func TestScrollAreaHandlesOnlyTargetedScroll(t *testing.T) {
	state := newTestState(t, "scroll-area", "both-axes")
	var canvas gfx.Canvas
	paintAt(&canvas, state, testSize)
	demo := scrollAreaDemoFor(state)
	if demo.scroller == nil || demo.bounds.Width <= 0 {
		t.Fatal("the scroll area was not declared")
	}
	if result := state.Scroll(gfx.ScrollEvent{DeltaX: 40, DeltaY: 60, Target: gfx.Item(999)}); result != (gfx.EventResult{}) {
		t.Fatalf("wheel outside the area result=%+v", result)
	}
	inside := func(event gfx.ScrollEvent) gfx.ScrollEvent {
		event.X, event.Y = demo.bounds.X+demo.bounds.Width/2, demo.bounds.Y+demo.bounds.Height/2
		return event
	}
	result := state.Scroll(inside(gfx.ScrollEvent{DeltaX: 40, DeltaY: 60, Target: scrollAreaViewport, Precise: true}))
	if !result.Handled || !result.Repaint || demo.scroller.OffsetX != 40 || demo.scroller.Offset != 60 {
		t.Fatalf("wheel inside result=%+v offsets=(%.0f,%.0f)", result, demo.scroller.OffsetX, demo.scroller.Offset)
	}
	result = state.Scroll(inside(gfx.ScrollEvent{DeltaX: 500, DeltaY: 500, Target: scrollAreaVertical}))
	if !result.Handled || !result.Repaint || demo.scroller.OffsetX != demo.scroller.MaxX() || demo.scroller.Offset != demo.scroller.Max() {
		t.Fatalf("clamped result=%+v offsets=(%.0f,%.0f) max=(%.0f,%.0f)", result, demo.scroller.OffsetX, demo.scroller.Offset, demo.scroller.MaxX(), demo.scroller.Max())
	}
	result = state.Scroll(inside(gfx.ScrollEvent{DeltaX: 10, DeltaY: 10, Target: scrollAreaViewport}))
	if result.Repaint {
		t.Fatalf("bounded result=%+v", result)
	}
}

func TestGroupKeyboardHelpersUpdateFocusAndSelection(t *testing.T) {
	tests := []struct {
		name, component, example string
		target                   gfx.ID
		key                      gfx.Key
		wantFocus                gfx.ID
		wantValue                int32
		wantFlags                uint64
		checkFlags               bool
	}{
		// The standard tabs examples are ui.Tabs: their arrow keys are the Page's
		// roving focus (ui/roving_test.go). The icons example paints the strip
		// from explicit IDs, so its keys stay in the demo.
		{name: "tabs icons", component: "tabs", example: "icons", target: tabsItem.At(0), key: gfx.KeyRight, wantFocus: tabsItem.At(1), wantValue: 0},
		{name: "radio skips disabled", component: "radio-group", example: "states", target: radioGroupItem.At(1), key: gfx.KeyDown, wantFocus: radioGroupItem.At(0), wantValue: 0},
		{name: "segment skips disabled", component: "segmented-control", example: "states", target: segmentedControlView.At(1), key: gfx.KeyRight, wantFocus: segmentedControlView.At(0), wantValue: 0},
		{name: "toggle vertical skips disabled", component: "toggle-group", example: "vertical", target: toggleGroupItem.At(1), key: gfx.KeyDown, wantFocus: toggleGroupItem.At(0), wantFlags: 1, checkFlags: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			state := newTestState(t, test.component, test.example)
			result := state.Key(gfx.KeyEvent{Key: test.key, Target: test.target})
			if !result.Handled || !result.FocusVisible || result.Focus != test.wantFocus {
				t.Fatalf("result=%+v", result)
			}
			if test.checkFlags {
				if state.flags != test.wantFlags {
					t.Fatalf("flags=%d want %d", state.flags, test.wantFlags)
				}
			} else if state.values[0] != test.wantValue {
				t.Fatalf("value=%d want %d", state.values[0], test.wantValue)
			}
		})
	}

	multiple := newTestState(t, "toggle-group", "multiple")
	before := multiple.flags
	result := multiple.Key(gfx.KeyEvent{Key: gfx.KeyRight, Target: toggleGroupItem.At(0)})
	if !result.Handled || result.Focus != toggleGroupItem.At(1) || result.Repaint || multiple.flags != before {
		t.Fatalf("multiple result=%+v flags=%d", result, multiple.flags)
	}
}

func TestStandardTabsRouteKeysThroughThePage(t *testing.T) {
	for _, example := range []string{"default", "line", "vertical"} {
		t.Run(example, func(t *testing.T) {
			state := newTestState(t, "tabs", example)
			var canvas gfx.Canvas
			paintAt(&canvas, state, testSize)
			labels := state.tabsLabels()
			first := findHitByLabel(t, &canvas, labels[0])
			second := findHitByLabel(t, &canvas, labels[1])
			click(&canvas, state, first)
			key := gfx.KeyRight
			if example == "vertical" {
				key = gfx.KeyDown
			}
			result := state.Key(gfx.KeyEvent{Key: key, Target: first})
			if !result.Handled || !result.FocusVisible || result.Focus != second {
				t.Fatalf("key result = %+v, want focus %v", result, second)
			}
		})
	}
}

func TestContextMenuOpensAtSecondaryPointer(t *testing.T) {
	state := newTestState(t, "context-menu", "default")
	var canvas gfx.Canvas
	paintFrame(&canvas, state, testSize, time.Second, true)
	area := findHitByLabel(t, &canvas, "上下文菜单区域")
	deliver := func(button gfx.PointerButton, kind gfx.PointerEventKind) {
		canvas.DeliverPointer(gfx.PointerEvent{
			Kind: kind, Button: button, X: 120, Y: 180, Target: area,
		}, testSize, state.Paint)
		paintFrame(&canvas, state, testSize, time.Second, true)
	}
	deliver(gfx.PointerButtonPrimary, gfx.PointerDown)
	if hasText(&canvas, "Show details") {
		t.Fatal("a primary press opened the context menu")
	}
	deliver(gfx.PointerButtonSecondary, gfx.PointerDown)
	if !hasText(&canvas, "Show details") {
		t.Fatal("a secondary press did not open the context menu")
	}
	row := findHitByLabel(t, &canvas, "Show details")
	click(&canvas, state, row)
	if state.flags&1 == 0 {
		t.Fatalf("toggle click flags=%d", state.flags)
	}
}

func TestTooltipAndHoverCardUseSynchronousEnterLeave(t *testing.T) {
	tooltip := newTestState(t, "tooltip", "default")
	var canvas gfx.Canvas
	paintFrame(&canvas, tooltip, testSize, time.Second, true)
	findHit(t, &canvas, tooltipTrigger)
	canvas.SetInteraction(tooltipTrigger, gfx.ID{}, gfx.ID{}, false)
	paintFrame(&canvas, tooltip, testSize, time.Second, true)
	if !hasText(&canvas, "原生同步提示，无后台延时器") {
		t.Fatal("hovering the trigger did not show its tooltip")
	}
	canvas.SetInteraction(gfx.ID{}, gfx.ID{}, gfx.ID{}, false)
	paintFrame(&canvas, tooltip, testSize, time.Second, true)
	if hasText(&canvas, "原生同步提示，无后台延时器") {
		t.Fatal("moving the pointer away did not hide the tooltip")
	}

	hover := newTestState(t, "hover-card", "default")
	result := hover.Pointer(gfx.PointerEvent{Kind: gfx.PointerMove, Target: hoverCardAnchor})
	if !result.Handled || !result.Repaint || hover.flags&1 == 0 {
		t.Fatalf("hover enter result=%+v flags=%d", result, hover.flags)
	}
	result = hover.Pointer(gfx.PointerEvent{Kind: gfx.PointerMove, Target: gfx.ID{}})
	if !result.Handled || !result.Repaint || hover.flags&1 != 0 {
		t.Fatalf("hover leave result=%+v flags=%d", result, hover.flags)
	}

	disabled := newTestState(t, "hover-card", "disabled")
	if result = disabled.Pointer(gfx.PointerEvent{Kind: gfx.PointerMove, Target: hoverCardAnchor}); result != (gfx.EventResult{}) || disabled.flags != 0 {
		t.Fatalf("disabled result=%+v flags=%d", result, disabled.flags)
	}
}

func TestTypedDemoEventsAllocateZero(t *testing.T) {
	slider := newTestState(t, "slider", "default")
	resizable := newTestState(t, "resizable", "horizontal")
	scroll := newTestState(t, "scroll-area", "both-axes")
	if allocations := testing.AllocsPerRun(100, func() {
		slider.values[0] = 40
		slider.Pointer(gfx.PointerEvent{Kind: gfx.PointerMove, X: 620, Target: sliderPrimary, Captured: true})
		resizable.values[0] = 50
		resizable.Pointer(gfx.PointerEvent{Kind: gfx.PointerMove, X: 500, Target: resizableHandle, Captured: true})
		scroll.values[0], scroll.values[1] = 0, 0
		scroll.Scroll(gfx.ScrollEvent{DeltaX: 10, DeltaY: 10, Target: scrollAreaViewport})
	}); allocations != 0 {
		t.Fatalf("typed demo events allocated %v times", allocations)
	}
}

func TestListKeysUseTheEmbeddedPage(t *testing.T) {
	state := newTestState(t, "list", "selection")
	var canvas gfx.Canvas
	paintAt(&canvas, state, testSize)
	row := findHitByLabel(t, &canvas, "Report.pdf")
	click(&canvas, state, row)
	if selected := state.list.list.Selected(); selected < 0 || state.list.rows[selected].name != "Report.pdf" {
		t.Fatalf("click selected %d", selected)
	}
	result := state.Key(gfx.KeyEvent{Key: gfx.KeyDown, Target: row})
	if !result.Handled {
		t.Fatalf("key result = %+v", result)
	}
	selected := state.list.list.Selected()
	if selected < 0 || state.list.rows[selected].name != "Notes.md" {
		t.Fatalf("Down selected %d", selected)
	}
}
