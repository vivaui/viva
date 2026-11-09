package ui

import (
	"math"
	"strings"
	"testing"

	"github.com/vivaui/viva/gfx"
)

var splitID = tree.ID(page, "split")

// A determinate bar is filled to its fraction and never asks for another frame; an
// indeterminate bar and a spinner ask only while some of them is on screen.
func TestProgressAndSpinnerAskForFramesOnlyWhileTheyAreOnScreen(t *testing.T) {
	theme := gfx.LightTheme()
	frames := func(scrolled float32, reduce bool, declare func(f *Frame)) bool {
		var view View
		canvas := testCanvas()
		canvas.SetReduceMotion(reduce)
		view.Draw(canvas, theme, gfx.Rect{Width: 300, Height: 100}, func(f *Frame) {
			f.Column().Fill().Children(func() {
				f.Scroll(scrolled).Height(100).Children(func() {
					f.Slot().Height(400)
					declare(f)
				})
			})
		})
		_, ok := canvas.NextFrame()
		return ok
	}
	bar := func(f *Frame) { f.Progress(0.5).Height(8) }
	indeterminate := func(f *Frame) { f.Progress(0).Indeterminate() }
	spinner := func(f *Frame) { f.Spinner() }

	if frames(400, false, bar) || frames(0, false, bar) {
		t.Error("a determinate progress bar asked for a frame")
	}
	for name, declare := range map[string]func(*Frame){"indeterminate bar": indeterminate, "spinner": spinner} {
		if !frames(400, false, declare) {
			t.Errorf("a %s that is on screen asked for no frame", name)
		}
		if frames(0, false, declare) {
			t.Errorf("a %s scrolled out of view asked for a frame", name)
		}
		if frames(400, true, declare) {
			t.Errorf("a %s asked for a frame although the system wants reduced motion", name)
		}
	}
}

func TestProgressIsFilledToItsFractionAndSizedLikeABar(t *testing.T) {
	var view View
	canvas := testCanvas()
	var column, row Box
	view.Draw(canvas, gfx.LightTheme(), gfx.Rect{Width: 300, Height: 100}, func(f *Frame) {
		f.Column().Pad(10).Gap(8).Children(func() {
			column = f.Progress(0.25)
			f.Row().Children(func() { row = f.Progress(2) }) // more than full is full
		})
	})
	near(t, "in a column", column.Rect(), gfx.Rect{X: 10, Y: 10, Width: 280, Height: 6})
	near(t, "in a row", row.Rect(), gfx.Rect{X: 10, Y: 24, Width: 160, Height: 6})
	rects := canvas.Rectangles()
	if len(rects) < 4 {
		t.Fatalf("%d rectangles painted, want a track and a fill for each bar", len(rects))
	}
	// The fill is the full pill clipped to the fraction.
	if got := rects[1].ClipOf.Width; got < 69.9 || got > 70.1 {
		t.Errorf("a quarter-full bar of 280 has a fill %v wide, want 70", got)
	}
	if got := rects[3].ClipOf.Width; got != 160 {
		t.Errorf("an over-full bar has a fill %v wide, want the track's 160", got)
	}
}

// splitPage is two panes either side of a splitter, in a row 400 wide.
func splitPage(view *View, state *SplitterState, width float32) func(*gfx.Canvas, gfx.Size) {
	return func(canvas *gfx.Canvas, size gfx.Size) {
		view.Draw(canvas, gfx.LightTheme(), gfx.Rect{Width: width, Height: 100}, func(f *Frame) {
			f.Row().Fill().Children(func() {
				f.Column().Width(state.Pos).Children(func() { f.Text("left") })
				f.Splitter(splitID, state).Mins(100, 150)
				f.Column().Grow(1).Children(func() { f.Text("right") })
			})
		})
	}
}

func splitterAt(t *testing.T, canvas *gfx.Canvas) gfx.Rect {
	t.Helper()
	for _, hit := range canvas.Hits() {
		if hit.ID == splitID {
			if hit.Cursor != gfx.PointerCursorResizeHorizontal {
				t.Errorf("the splitter's cursor is %v, want the horizontal resize shape", hit.Cursor)
			}
			return hit.Bounds
		}
	}
	t.Fatal("no hit region for the splitter")
	return gfx.Rect{}
}

// Dragging the bar moves the first pane by what the pointer moved, even when the
// pointer leaves the strip, and the limits hold at both ends.
func TestSplitterDragMovesTheFirstPaneWithinItsLimits(t *testing.T) {
	var view View
	state := SplitterState{Pos: 200}
	paint := splitPage(&view, &state, 400)
	canvas := testCanvas()
	size := gfx.Size{Width: 400, Height: 100}
	paint(canvas, size)
	bar := splitterAt(t, canvas)
	x, y := bar.X+bar.Width/2, float32(50)

	deliver := func(kind gfx.PointerEventKind, px float32) {
		t.Helper()
		if !canvas.DeliverPointer(gfx.PointerEvent{Kind: kind, Button: gfx.PointerButtonPrimary, X: px, Y: y, Target: splitID, Captured: kind != gfx.PointerDown}, size, paint) {
			t.Fatalf("pointer %v was not consumed by the splitter", kind)
		}
	}
	deliver(gfx.PointerDown, x)
	if !state.Dragging() {
		t.Fatal("a press on the bar did not start a drag")
	}
	deliver(gfx.PointerMove, x-60)
	if state.Pos != 140 {
		t.Errorf("after moving 60 left the first pane is %v, want 140", state.Pos)
	}
	deliver(gfx.PointerMove, x-500)
	if state.Pos != 100 {
		t.Errorf("dragged far left the first pane is %v, want its minimum 100", state.Pos)
	}
	deliver(gfx.PointerMove, x+500)
	if want := float32(400 - splitterThickness - 150); state.Pos != want {
		t.Errorf("dragged far right the first pane is %v, want %v (the second keeps its minimum)", state.Pos, want)
	}
	deliver(gfx.PointerUp, x+500)
	if state.Dragging() {
		t.Error("the release did not end the drag")
	}

	canvas.Reset()
	paint(canvas, size)
	if got := splitterAt(t, canvas).X; got != state.Pos {
		t.Errorf("the bar is at %v after the drag, want it right after the first pane at %v", got, state.Pos)
	}
}

// Ratio gives the first pane its initial share of the container; once it is
// placed, the application can set Pos and a drag keeps the pixel position.
func TestSplitterRatioPlacesTheFirstPaneOnTheFirstLayout(t *testing.T) {
	var view View
	state := SplitterState{Ratio: 0.6}
	canvas := testCanvas()
	splitPage(&view, &state, 400)(canvas, gfx.Size{Width: 400, Height: 100})
	want := float32(0.6 * (400 - splitterThickness))
	if math.Abs(float64(state.Pos-want)) > 0.01 {
		t.Fatalf("the first pane is %v, want %v", state.Pos, want)
	}
	// The share survives a resize.
	canvas.Reset()
	splitPage(&view, &state, 500)(canvas, gfx.Size{Width: 500, Height: 100})
	want = float32(0.6 * (500 - splitterThickness))
	if math.Abs(float64(state.Pos-want)) > 0.01 {
		t.Fatalf("after resizing the first pane is %v, want %v", state.Pos, want)
	}
	// A state that holds pixels clears the share.
	state.Ratio = 0
	state.Pos = 180
	canvas.Reset()
	splitPage(&view, &state, 400)(canvas, gfx.Size{Width: 400, Height: 100})
	if state.Pos != 180 {
		t.Fatalf("a set position was overwritten: %v", state.Pos)
	}
}

func TestSplitterKeepsItsPositionInRangeWhenTheWindowShrinksAndTakesArrowKeys(t *testing.T) {
	var view View
	state := SplitterState{Pos: 300}
	canvas := testCanvas()
	size := gfx.Size{}
	splitPage(&view, &state, 500)(canvas, size)
	splitPage(&view, &state, 400)(canvas, size)
	if want := float32(400 - splitterThickness - 150); state.Pos != want {
		t.Errorf("after the window shrank the first pane is %v, want %v", state.Pos, want)
	}
	if _, asked := canvas.NextFrame(); !asked {
		t.Error("moving the position asked for no frame, so the panes would lag behind it")
	}
	if got := state.Key(gfx.KeyEvent{Key: gfx.KeyLeft, Target: omit}); got.Handled {
		t.Error("an arrow key for another control moved the splitter")
	}
	state.Key(gfx.KeyEvent{Key: gfx.KeyLeft, Target: splitID})
	if want := float32(400-splitterThickness-150) - splitterStep; state.Pos != want {
		t.Errorf("Left moved the first pane to %v, want %v", state.Pos, want)
	}
	state.Key(gfx.KeyEvent{Key: gfx.KeyHome, Target: splitID})
	if state.Pos != 100 {
		t.Errorf("Home put the first pane at %v, want its minimum", state.Pos)
	}
}

func TestASplitterInAColumnResizesVertically(t *testing.T) {
	var view View
	state := SplitterState{Pos: 30}
	canvas := testCanvas()
	var bar Box
	view.Draw(canvas, gfx.LightTheme(), gfx.Rect{Width: 200, Height: 200}, func(f *Frame) {
		f.Column().Fill().Children(func() {
			f.Row().Height(state.Pos)
			bar = f.Splitter(splitID, &state).Mins(10, 10)
			f.Row().Grow(1)
		})
	})
	near(t, "bar", bar.Rect(), gfx.Rect{X: 0, Y: 30, Width: 200, Height: splitterThickness})
	for _, hit := range canvas.Hits() {
		if hit.ID == splitID && hit.Cursor != gfx.PointerCursorResizeVertical {
			t.Errorf("cursor %v, want the vertical resize shape", hit.Cursor)
		}
	}
}

// A container given Hit and nothing that shows or sizes it is a mistake the guide
// warns about: it fails at once with the fix in the message, while an empty target
// that was given a size or a background is fine.
func TestAHitWithoutChildrenAndWithoutSizeFailsLoudly(t *testing.T) {
	draw := func(declare func(f *Frame)) (message string) {
		defer func() {
			if r := recover(); r != nil {
				message, _ = r.(string)
			}
		}()
		var view View
		view.Draw(testCanvas(), gfx.LightTheme(), gfx.Rect{Width: 200, Height: 100}, func(f *Frame) {
			f.Column().Children(func() { declare(f) })
		})
		return ""
	}
	message := draw(func(f *Frame) { f.Row().Pad(8).Hit(rowID, "Inbox").Clicked() })
	if !strings.Contains(message, "Children") || !strings.Contains(message, "Inbox") {
		t.Errorf("message %q does not name the label and the fix", message)
	}
	for name, declare := range map[string]func(f *Frame){
		"with children":          func(f *Frame) { f.Row().Hit(rowID, "r").Children(func() { f.Text("x") }) },
		"with an empty Children": func(f *Frame) { f.Row().Hit(rowID, "r").Children(func() {}) },
		"with a height":          func(f *Frame) { f.Row().Height(6).Hit(rowID, "r") },
		"filling":                func(f *Frame) { f.Row().Fill().Hit(rowID, "r") },
		"with a background":      func(f *Frame) { f.Row().Background(gfx.LightTheme().Hover).Hit(rowID, "r") },
		"a slot":                 func(f *Frame) { f.Slot().Height(5).Hit(rowID, "r") },
	} {
		if message := draw(declare); message != "" {
			t.Errorf("%s: %s", name, message)
		}
	}
}

func TestLeavesAndSplittersAllocateNothingOnceWarm(t *testing.T) {
	var view View
	canvas := testCanvas()
	theme := gfx.LightTheme()
	state := SplitterState{Pos: 120}
	run := func() {
		canvas.Reset()
		view.Draw(canvas, theme, gfx.Rect{Width: 600, Height: 300}, func(f *Frame) {
			f.Column().Fill().Pad(8).Gap(8).Children(func() {
				f.Progress(0.3)
				f.Progress(0).Indeterminate().Height(8)
				f.Row().Gap(8).Items(Center).Children(func() { f.Spinner(); f.Text("Loading") })
				f.Row().Grow(1).Children(func() {
					f.Column().Width(state.Pos)
					f.Splitter(splitID, &state)
					f.Column().Grow(1)
				})
			})
		})
	}
	run()
	if allocs := testing.AllocsPerRun(200, run); allocs != 0 {
		t.Fatalf("a warm page with progress, spinner and splitter allocates %v times a frame, want 0", allocs)
	}
}

func BenchmarkProgressSpinnerSplitter(b *testing.B) {
	var view View
	canvas := testCanvas()
	theme := gfx.LightTheme()
	state := SplitterState{Pos: 120}
	b.ReportAllocs()
	for b.Loop() {
		canvas.Reset()
		view.Draw(canvas, theme, gfx.Rect{Width: 600, Height: 300}, func(f *Frame) {
			f.Column().Fill().Pad(8).Gap(8).Children(func() {
				f.Progress(0.3)
				f.Spinner()
				f.Row().Grow(1).Children(func() {
					f.Column().Width(state.Pos)
					f.Splitter(splitID, &state)
					f.Column().Grow(1)
				})
			})
		})
	}
}

// Disabled takes the condition, so a stepper's limit button is one call; false leaves the
// control as it was and can undo an earlier true.
func TestDisabledTakesTheCondition(t *testing.T) {
	theme := gfx.LightTheme()
	paint := func(declare func(Box) Box) *gfx.Canvas {
		canvas := testCanvas()
		var view View
		view.Draw(canvas, theme, gfx.Rect{Width: 200, Height: 100}, func(f *Frame) {
			f.Row().Children(func() { declare(f.Button(save, "Minus").Outline()) })
		})
		return canvas
	}
	plain := paint(func(b Box) Box { return b })
	sameCanvas(t, "Disabled(false)", paint(func(b Box) Box { return b.Disabled(false) }), plain)
	sameCanvas(t, "Disabled(true) then false", paint(func(b Box) Box { return b.Disabled(true).Disabled(false) }), plain)
	off := paint(func(b Box) Box { return b.Disabled(true) })
	if off.Texts()[0].Style.Color == plain.Texts()[0].Style.Color && off.Rectangles()[0].Color == plain.Rectangles()[0].Color {
		t.Error("a disabled button looks like an enabled one")
	}
}
