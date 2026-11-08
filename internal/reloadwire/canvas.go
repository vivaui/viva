package reloadwire

import (
	"time"

	"github.com/vivaui/viva/ui"
)

func CaptureCanvas(canvas *ui.Canvas) CanvasSnapshot {
	var delayMs uint32
	if delay, ok := canvas.NextFrame(); ok && delay > 0 {
		delayMs = uint32(max((delay+time.Millisecond-1)/time.Millisecond, 1)) // never rounded down to "now"
	}
	return CanvasSnapshot{
		Rectangles:   append([]ui.Rectangle(nil), canvas.Rectangles()...),
		Shadows:      append([]ui.Shadow(nil), canvas.Shadows()...),
		Images:       append([]ui.Image(nil), canvas.Images()...),
		Texts:        append([]ui.Text(nil), canvas.Texts()...),
		Runs:         append([]ui.TextRun(nil), canvas.AllTextRuns()...),
		Hits:         append([]ui.Hit(nil), canvas.Hits()...),
		TrappedScope: canvas.TrappedFocusScope(),
		EscapeTarget: canvas.EscapeTarget(),
		InitialFocus: canvas.InitialFocus(),
		Animating:    canvas.Animating(),
		FrameDelayMs: delayMs,
	}
}

func ApplyCanvas(canvas *ui.Canvas, snapshot CanvasSnapshot) {
	canvas.Reset()
	// A shadow goes back among the rectangles where it was painted. Triangles are
	// not carried, so only the rectangles count.
	shadows := snapshot.Shadows
	replayShadows := func(rectangles int) {
		for len(shadows) > 0 && int(shadows[0].RectsBefore) <= rectangles {
			shadow := shadows[0]
			shadows = shadows[1:]
			pushLayerAndClip(canvas, shadow.Layer, shadow.ClipOf, shadow.HasClip)
			canvas.Shadow(shadow.Bounds, shadow.Radius, shadow.Style, shadow.Color)
			popLayerAndClip(canvas, shadow.Layer, shadow.HasClip)
		}
	}
	for index, rectangle := range snapshot.Rectangles {
		replayShadows(index)
		pushLayerAndClip(canvas, rectangle.Layer, rectangle.ClipOf, rectangle.HasClip)
		canvas.Rounded(rectangle.Bounds, rectangle.Radius, rectangle.Color)
		popLayerAndClip(canvas, rectangle.Layer, rectangle.HasClip)
	}
	replayShadows(len(snapshot.Rectangles))
	for _, image := range snapshot.Images {
		pushLayerAndClip(canvas, image.Layer, image.ClipOf, image.HasClip)
		canvas.DrawBitmapRegion(image.Bitmap, image.Source, image.Bounds)
		popLayerAndClip(canvas, image.Layer, image.HasClip)
	}
	for _, text := range snapshot.Texts {
		pushLayerAndClip(canvas, text.Layer, text.ClipOf, text.HasClip)
		if text.RunCount > 0 || text.CellWidth > 0 {
			canvas.LabelRunsWithIdentity(text.Identity, text.Value, text.Bounds, text.Style, text.CellWidth, snapshot.Runs[text.RunFirst:text.RunFirst+text.RunCount])
		} else {
			canvas.Label(text.Value, text.Bounds, text.Style)
		}
		popLayerAndClip(canvas, text.Layer, text.HasClip)
	}
	for _, hit := range snapshot.Hits {
		if hit.Layer != 0 {
			canvas.PushLayer(hit.Layer)
		}
		if !hit.Scope.IsZero() {
			canvas.PushFocusScope(hit.Scope)
		}
		if hit.Block {
			canvas.Block(hit.Bounds)
		} else if hit.MovesWindow {
			canvas.MoveRegion(hit.Bounds)
		} else {
			canvas.HitWithOptions(hit.ID, hit.Bounds, hit.Label, ui.HitOptions{
				Role: hit.Role, Value: hit.Value, Hint: hit.Hint, Disabled: hit.Disabled,
				Checked: hit.Checked, Selected: hit.Selected, Expanded: hit.Expanded,
				Multiline: hit.Multiline, ReadOnly: hit.ReadOnly, Secure: hit.Secure, SkipFocus: !hit.Focusable, Autofocus: hit.Autofocus,
				TextBounds: hit.TextBounds, TextSize: hit.TextSize, TextColor: hit.TextColor, TextWeight: hit.TextWeight,
			})
		}
		if !hit.Scope.IsZero() {
			canvas.PopFocusScope()
		}
		if hit.Layer != 0 {
			canvas.PopLayer()
		}
	}
	canvas.TrapFocus(snapshot.TrappedScope)
	canvas.SetEscapeTarget(snapshot.EscapeTarget)
	canvas.SetInitialFocus(snapshot.InitialFocus)
	if snapshot.Animating {
		canvas.RequestFrame()
	} else if snapshot.FrameDelayMs > 0 {
		canvas.RequestFrameIn(time.Duration(snapshot.FrameDelayMs) * time.Millisecond)
	}
}

func pushLayerAndClip(canvas *ui.Canvas, layer uint16, clip ui.Rect, hasClip bool) {
	if layer != 0 {
		canvas.PushLayer(layer)
	}
	if hasClip {
		canvas.Clip(clip)
	}
}

func popLayerAndClip(canvas *ui.Canvas, layer uint16, hasClip bool) {
	if hasClip {
		canvas.Unclip()
	}
	if layer != 0 {
		canvas.PopLayer()
	}
}
