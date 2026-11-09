package components

import (
	"testing"
	"time"

	"github.com/vivaui/viva/gfx"
)

// overlayFrame paints one frame of a dialog (when open) and a dropdown menu (when
// menuOpen) on a canvas with a clock.
func overlayFrame(canvas *gfx.Canvas, now *time.Duration, at time.Duration, open, menuOpen bool) {
	*now = at
	canvas.Reset()
	canvas.BeginFrame()
	viewport := gfx.Rect{Width: 800, Height: 600}
	theme := gfx.DarkTheme()
	if open {
		dialog := BeginDialog(canvas, theme, viewport, DialogOptions{
			Layer: 1, Scope: gfx.Item(1), DismissID: gfx.Item(2), CloseID: gfx.Item(3), ContentSize: gfx.Size{Width: 400, Height: 200},
		})
		dialog.Begin("Delete host?", "This cannot be undone.")
		dialog.End()
	}
	if menuOpen {
		menu := BeginDropdownMenu(canvas, theme, gfx.Rect{X: 20, Y: 20, Width: 160, Height: 80}, DropdownMenuOptions{Layer: 2, Scope: gfx.Item(4), DismissID: gfx.Item(5)})
		menu.AddItem(DropdownMenuItem{ID: gfx.Item(6), Label: "Open"})
		menu.End()
	}
	canvas.EndFrame()
}

func TestClosedDialogAndMenuFadeOutWithoutTakingInput(t *testing.T) {
	var canvas gfx.Canvas
	var now time.Duration
	canvas.SetClock(func() time.Duration { return now })

	overlayFrame(&canvas, &now, 0, true, true)
	if !canvas.Animating() || canvas.Rectangles()[0].Color.Alpha != 0 {
		t.Fatalf("opening: animating %v, first scrim alpha %v", canvas.Animating(), canvas.Rectangles()[0].Color.Alpha)
	}
	overlayFrame(&canvas, &now, time.Second, true, true)
	if canvas.Animating() {
		t.Fatal("still animating long after both overlays came in")
	}
	if got := canvas.Rectangles()[0].Color.Alpha; got != overlayBackdrop(gfx.DarkTheme()).Alpha {
		t.Fatalf("scrim at rest has alpha %v", got)
	}
	rest := len(canvas.Rectangles())

	// The application stops painting both: they stay on screen for the exit, with no hit regions and no
	// modal state, and the canvas asks for frames while they do.
	overlayFrame(&canvas, &now, time.Second, false, false)
	overlayFrame(&canvas, &now, time.Second+50*time.Millisecond, false, false)
	if !canvas.Animating() || len(canvas.Rectangles()) < rest {
		t.Fatalf("leaving: animating %v, %d rectangles (want at least the %d they had; a border fades as its sides)", canvas.Animating(), len(canvas.Rectangles()), rest)
	}
	if len(canvas.Hits()) != 0 || !canvas.TrappedFocusScope().IsZero() || !canvas.EscapeTarget().IsZero() {
		t.Fatalf("a leaving overlay holds input: %d hits, trap %v", len(canvas.Hits()), canvas.TrappedFocusScope())
	}
	if got := canvas.Rectangles()[0].Color.Alpha; got <= 0 || got >= overlayBackdrop(gfx.DarkTheme()).Alpha {
		t.Fatalf("scrim halfway out has alpha %v", got)
	}
	overlayFrame(&canvas, &now, time.Second+time.Second, false, false)
	if canvas.Animating() || len(canvas.Rectangles()) != 0 {
		t.Fatalf("after the exit: animating %v, %d rectangles", canvas.Animating(), len(canvas.Rectangles()))
	}
	if _, ok := canvas.NextFrame(); ok {
		t.Fatal("an idle canvas asked for a frame")
	}
}

func TestOverlaysAreFinalWithoutAClockOrUnderReducedMotion(t *testing.T) {
	var plain gfx.Canvas
	var now time.Duration
	// No clock: BeginFrame is a no-op for motion.
	plain.BeginFrame()
	dialog := BeginDialog(&plain, gfx.DarkTheme(), gfx.Rect{Width: 800, Height: 600}, DialogOptions{DismissID: gfx.Item(2), ContentSize: gfx.Size{Width: 400, Height: 200}})
	dialog.Paint("Title", "")
	plain.EndFrame()
	if plain.Rectangles()[0].Color.Alpha != overlayBackdrop(gfx.DarkTheme()).Alpha || plain.Animating() {
		t.Fatalf("no clock: scrim alpha %v animating %v", plain.Rectangles()[0].Color.Alpha, plain.Animating())
	}

	var reduced gfx.Canvas
	reduced.SetClock(func() time.Duration { return now })
	reduced.SetReduceMotion(true)
	overlayFrame(&reduced, &now, 0, true, false)
	if reduced.Rectangles()[0].Color.Alpha != overlayBackdrop(gfx.DarkTheme()).Alpha || reduced.Animating() {
		t.Fatalf("reduced motion: scrim alpha %v animating %v", reduced.Rectangles()[0].Color.Alpha, reduced.Animating())
	}
	overlayFrame(&reduced, &now, 10*time.Millisecond, false, false)
	if len(reduced.Rectangles()) != 0 || reduced.Animating() {
		t.Fatalf("reduced motion: a closed dialog left %d rectangles", len(reduced.Rectangles()))
	}
}

func TestContextMenuNeitherEntersNorLeaves(t *testing.T) {
	var canvas gfx.Canvas
	var now time.Duration
	canvas.SetClock(func() time.Duration { return now })
	paint := func(at time.Duration, open bool) {
		now = at
		canvas.Reset()
		canvas.BeginFrame()
		if open {
			menu := BeginContextMenu(&canvas, gfx.DarkTheme(), gfx.Rect{X: 20, Y: 20, Width: 160, Height: 80},
				ContextMenuOptions{Layer: 2, Scope: gfx.Item(4), DismissID: gfx.Item(5)})
			menu.AddItem(ContextMenuItem{ID: gfx.Item(6), Label: "Open"})
			menu.End()
		}
		canvas.EndFrame()
	}

	paint(0, true)
	if canvas.Animating() {
		t.Fatal("a context menu animates in")
	}
	paint(time.Millisecond, false)
	if canvas.Animating() || len(canvas.Rectangles()) != 0 {
		t.Fatalf("a context menu animates out: animating %v, %d rectangles", canvas.Animating(), len(canvas.Rectangles()))
	}
}
