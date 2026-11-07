package componentdemo

import (
	"testing"
	"time"

	"github.com/vivaui/viva/ui"
)

func TestTypedEventDispatchDefaultsToUnhandled(t *testing.T) {
	state := newTestState(t, "button", "default")
	if result := state.Pointer(ui.PointerEvent{Kind: ui.PointerDown, Target: buttonPress.At(0)}); result != (ui.EventResult{}) {
		t.Fatalf("pointer result = %+v", result)
	}
	if result := state.Key(ui.KeyEvent{Key: ui.KeyRight, Target: buttonPress.At(0)}); result != (ui.EventResult{}) {
		t.Fatalf("key result = %+v", result)
	}
	if result := state.Scroll(ui.ScrollEvent{DeltaY: 40, Target: buttonPress.At(0)}); result != (ui.EventResult{}) {
		t.Fatalf("scroll result = %+v", result)
	}
}

// paintAt paints state at size on canvas, so the demo knows where its control is; a
// pointer event carries coordinates, and the demo reads them against that placement.
func paintAt(canvas *ui.Canvas, state *State, size ui.Size) {
	canvas.SetMeasurer(measureTestText)
	paintFrame(canvas, state, size, time.Second, true)
}

func TestSliderPointerCoordinatesUpdateCallerState(t *testing.T) {
	state := newTestState(t, "slider", "default")
	var canvas ui.Canvas
	paintAt(&canvas, state, testSize)
	track := state.slider.placed
	right, middle := track.X+track.Width, track.X+track.Width/2
	result := state.Pointer(ui.PointerEvent{
		Kind: ui.PointerDown, Button: ui.PointerButtonPrimary,
		X: right, Y: track.Y, Target: sliderPrimary,
	})
	if !result.Handled || !result.Repaint || result.Focus != sliderPrimary || state.values[0] != 100 {
		t.Fatalf("down result=%+v value=%d", result, state.values[0])
	}
	result = state.Pointer(ui.PointerEvent{
		Kind: ui.PointerMove, X: middle, Y: track.Y, Target: sliderPrimary, Captured: true,
	})
	if !result.Handled || !result.Repaint || state.values[0] != 50 {
		t.Fatalf("captured move result=%+v value=%d", result, state.values[0])
	}
	result = state.Pointer(ui.PointerEvent{Kind: ui.PointerUp, Target: sliderPrimary, Captured: true})
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
	result := state.Key(ui.KeyEvent{Key: ui.KeyUp, Target: sliderPrimary})
	if !result.Handled || !result.Repaint || state.values[0] != 50 {
		t.Fatalf("up result=%+v value=%d", result, state.values[0])
	}
	result = state.Key(ui.KeyEvent{Key: ui.KeyHome, Target: sliderPrimary})
	if !result.Handled || !result.Repaint || state.values[0] != 0 {
		t.Fatalf("home result=%+v value=%d", result, state.values[0])
	}
	if result := state.Key(ui.KeyEvent{Key: ui.KeyUp, Target: sliderDisabled}); result != (ui.EventResult{}) {
		t.Fatalf("disabled slider result=%+v", result)
	}
}

func TestRangeSliderKeyboardKeepsThumbsApart(t *testing.T) {
	state := newTestState(t, "slider", "range")
	result := state.Key(ui.KeyEvent{Key: ui.KeyEnd, Target: sliderLower})
	if !result.Handled || !result.Repaint || state.values[0] != 70 {
		t.Fatalf("end lower result=%+v values=%v", result, state.values[:2])
	}
	result = state.Key(ui.KeyEvent{Key: ui.KeyUp, Target: sliderUpper})
	if !result.Handled || !result.Repaint || state.values[1] != 80 || state.slider.thumbs[1].Value != 80 {
		t.Fatalf("up upper result=%+v values=%v thumbs=%+v", result, state.values[:2], state.slider.thumbs)
	}
}

func TestPointerGeometryUsesResponsiveViewport(t *testing.T) {
	narrow := ui.Size{Width: 320, Height: 340}
	slider := newTestState(t, "slider", "default")
	var canvas ui.Canvas
	paintAt(&canvas, slider, narrow)
	if track := slider.slider.placed; track.X+track.Width > narrow.Width {
		t.Fatalf("slider track %+v is wider than the window", track)
	}
	track := slider.slider.placed
	result := slider.Pointer(ui.PointerEvent{
		Kind: ui.PointerDown, Button: ui.PointerButtonPrimary, X: track.X + track.Width,
		Viewport: narrow, Target: sliderPrimary,
	})
	if !result.Handled || slider.values[0] != 100 {
		t.Fatalf("responsive slider result=%+v value=%d", result, slider.values[0])
	}

	resizable := newTestState(t, "resizable", "horizontal")
	paintAt(&canvas, resizable, narrow)
	split := resizable.slider.placed
	result = resizable.Pointer(ui.PointerEvent{
		Kind: ui.PointerDown, Button: ui.PointerButtonPrimary, X: split.X + split.Width,
		Viewport: narrow, Target: resizableHandle,
	})
	if !result.Handled || resizable.values[0] != 80 {
		t.Fatalf("responsive resizable result=%+v value=%d", result, resizable.values[0])
	}
}

func TestRangeSliderPointerCannotCrossOtherThumb(t *testing.T) {
	state := newTestState(t, "slider", "range")
	var canvas ui.Canvas
	paintAt(&canvas, state, testSize)
	track := state.slider.placed
	result := state.Pointer(ui.PointerEvent{Kind: ui.PointerDown, Button: ui.PointerButtonPrimary, X: track.X + track.Width, Target: sliderLower})
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
	var canvas ui.Canvas
	paintAt(&canvas, state, testSize)
	split := state.slider.placed
	at := func(fraction float32) float32 { return split.X + split.Width*fraction }
	result := state.Pointer(ui.PointerEvent{Kind: ui.PointerMove, X: at(0.7), Target: resizableHandle})
	if result != (ui.EventResult{}) || state.values[0] != 50 {
		t.Fatalf("uncaptured move result=%+v value=%d", result, state.values[0])
	}
	result = state.Pointer(ui.PointerEvent{Kind: ui.PointerDown, Button: ui.PointerButtonPrimary, X: at(0.7), Target: resizableHandle})
	if !result.Handled || !result.Repaint || result.Focus != resizableHandle || state.values[0] != 70 {
		t.Fatalf("down result=%+v value=%d", result, state.values[0])
	}
	result = state.Pointer(ui.PointerEvent{Kind: ui.PointerMove, X: at(0.2), Target: resizableHandle, Captured: true})
	if !result.Handled || !result.Repaint || state.values[0] != 20 {
		t.Fatalf("captured move result=%+v value=%d", result, state.values[0])
	}
	click(&canvas, state, resizableHandle)
	if state.values[0] != 30 {
		t.Fatalf("click value=%d", state.values[0])
	}
}

func TestScrollAreaHandlesOnlyTargetedScroll(t *testing.T) {
	state := newTestState(t, "scroll-area", "both-axes")
	if result := state.Scroll(ui.ScrollEvent{DeltaX: 40, DeltaY: 60, Target: ui.Item(999)}); result != (ui.EventResult{}) {
		t.Fatalf("untargeted result=%+v", result)
	}
	result := state.Scroll(ui.ScrollEvent{DeltaX: 40, DeltaY: 60, Target: scrollAreaViewport, Precise: true})
	if !result.Handled || !result.Repaint || state.values[1] != 40 || state.values[0] != 60 {
		t.Fatalf("targeted result=%+v offsets=(%d,%d)", result, state.values[1], state.values[0])
	}
	result = state.Scroll(ui.ScrollEvent{DeltaX: 500, DeltaY: 500, Target: scrollAreaVertical})
	if !result.Handled || !result.Repaint || state.values[1] != 180 || state.values[0] != 180 {
		t.Fatalf("clamped result=%+v offsets=(%d,%d)", result, state.values[1], state.values[0])
	}
	result = state.Scroll(ui.ScrollEvent{DeltaX: 10, DeltaY: 10, Target: scrollAreaViewport})
	if !result.Handled || result.Repaint {
		t.Fatalf("bounded result=%+v", result)
	}
	var canvas ui.Canvas
	paintAt(&canvas, state, testSize)
	click(&canvas, state, scrollAreaVertical)
	if state.values[0] != 0 {
		t.Fatalf("vertical click offset=%d", state.values[0])
	}
}

func TestGroupKeyboardHelpersUpdateFocusAndSelection(t *testing.T) {
	tests := []struct {
		name, component, example string
		target                   ui.ID
		key                      ui.Key
		wantFocus                ui.ID
		wantValue                int32
		wantFlags                uint64
		checkFlags               bool
	}{
		{name: "tabs", component: "tabs", example: "default", target: tabsItem.At(0), key: ui.KeyRight, wantFocus: tabsItem.At(1), wantValue: 0},
		{name: "tabs vertical", component: "tabs", example: "vertical", target: tabsItem.At(0), key: ui.KeyDown, wantFocus: tabsItem.At(1), wantValue: 0},
		{name: "radio skips disabled", component: "radio-group", example: "states", target: radioGroupItem.At(1), key: ui.KeyDown, wantFocus: radioGroupItem.At(0), wantValue: 0},
		{name: "segment skips disabled", component: "segmented-control", example: "states", target: segmentedControlView.At(1), key: ui.KeyRight, wantFocus: segmentedControlView.At(0), wantValue: 0},
		{name: "toggle vertical skips disabled", component: "toggle-group", example: "vertical", target: toggleGroupItem.At(1), key: ui.KeyDown, wantFocus: toggleGroupItem.At(0), wantFlags: 1, checkFlags: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			state := newTestState(t, test.component, test.example)
			result := state.Key(ui.KeyEvent{Key: test.key, Target: test.target})
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
	result := multiple.Key(ui.KeyEvent{Key: ui.KeyRight, Target: toggleGroupItem.At(0)})
	if !result.Handled || result.Focus != toggleGroupItem.At(1) || result.Repaint || multiple.flags != before {
		t.Fatalf("multiple result=%+v flags=%d", result, multiple.flags)
	}
}

func TestContextMenuOpensAtSecondaryPointer(t *testing.T) {
	state := newTestState(t, "context-menu", "default")
	state.flags &^= contextMenuOpenMask
	if result := state.Pointer(ui.PointerEvent{Kind: ui.PointerDown, Button: ui.PointerButtonPrimary, Target: contextMenuSurface}); result != (ui.EventResult{}) {
		t.Fatalf("primary result=%+v", result)
	}
	result := state.Pointer(ui.PointerEvent{
		Kind: ui.PointerDown, Button: ui.PointerButtonSecondary,
		X: 420, Y: 180, Target: contextMenuSurface,
	})
	if !result.Handled || !result.Repaint || state.flags&contextMenuOpenMask == 0 || state.points[0] != 420 || state.points[1] != 180 {
		t.Fatalf("secondary result=%+v flags=%d point=%v", result, state.flags, state.points[:2])
	}
	var canvas ui.Canvas
	click(&canvas, state, contextMenuToggle)
	if state.flags&1 == 0 || state.flags&contextMenuOpenMask == 0 {
		t.Fatalf("toggle click flags=%d", state.flags)
	}
}

func TestTooltipAndHoverCardUseSynchronousEnterLeave(t *testing.T) {
	tooltip := newTestState(t, "tooltip", "default")
	result := tooltip.Pointer(ui.PointerEvent{Kind: ui.PointerMove, Target: tooltipTrigger})
	if !result.Handled || !result.Repaint || tooltip.flags&2 == 0 {
		t.Fatalf("tooltip enter result=%+v flags=%d", result, tooltip.flags)
	}
	result = tooltip.Pointer(ui.PointerEvent{Kind: ui.PointerExit})
	if !result.Handled || !result.Repaint || tooltip.flags&2 != 0 {
		t.Fatalf("tooltip leave result=%+v flags=%d", result, tooltip.flags)
	}
	var canvas ui.Canvas
	click(&canvas, tooltip, tooltipTrigger)
	tooltip.Pointer(ui.PointerEvent{Kind: ui.PointerMove, Target: tooltipTrigger})
	tooltip.Pointer(ui.PointerEvent{Kind: ui.PointerExit})
	if tooltip.flags&1 == 0 {
		t.Fatal("pointer leave cleared click-owned tooltip state")
	}

	hover := newTestState(t, "hover-card", "default")
	result = hover.Pointer(ui.PointerEvent{Kind: ui.PointerMove, Target: hoverCardAnchor})
	if !result.Handled || !result.Repaint || hover.flags&1 == 0 {
		t.Fatalf("hover enter result=%+v flags=%d", result, hover.flags)
	}
	result = hover.Pointer(ui.PointerEvent{Kind: ui.PointerMove, Target: ui.ID{}})
	if !result.Handled || !result.Repaint || hover.flags&1 != 0 {
		t.Fatalf("hover leave result=%+v flags=%d", result, hover.flags)
	}

	disabled := newTestState(t, "hover-card", "disabled")
	if result = disabled.Pointer(ui.PointerEvent{Kind: ui.PointerMove, Target: hoverCardAnchor}); result != (ui.EventResult{}) || disabled.flags != 0 {
		t.Fatalf("disabled result=%+v flags=%d", result, disabled.flags)
	}
}

func TestTypedDemoEventsAllocateZero(t *testing.T) {
	slider := newTestState(t, "slider", "default")
	resizable := newTestState(t, "resizable", "horizontal")
	scroll := newTestState(t, "scroll-area", "both-axes")
	tabs := newTestState(t, "tabs", "default")
	if allocations := testing.AllocsPerRun(100, func() {
		slider.values[0] = 40
		slider.Pointer(ui.PointerEvent{Kind: ui.PointerMove, X: 620, Target: sliderPrimary, Captured: true})
		resizable.values[0] = 50
		resizable.Pointer(ui.PointerEvent{Kind: ui.PointerMove, X: 500, Target: resizableHandle, Captured: true})
		scroll.values[0], scroll.values[1] = 0, 0
		scroll.Scroll(ui.ScrollEvent{DeltaX: 10, DeltaY: 10, Target: scrollAreaViewport})
		tabs.values[0] = 0
		tabs.Key(ui.KeyEvent{Key: ui.KeyRight, Target: tabsItem.At(0)})
	}); allocations != 0 {
		t.Fatalf("typed demo events allocated %v times", allocations)
	}
}
