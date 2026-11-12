package reloadwire

import (
	"time"

	"github.com/vivaui/viva/gfx"
)

func CaptureCanvas(canvas *gfx.Canvas) CanvasSnapshot {
	var delayMs uint32
	if delay, ok := canvas.NextFrame(); ok && delay > 0 {
		delayMs = uint32(max((delay+time.Millisecond-1)/time.Millisecond, 1)) // never rounded down to "now"
	}
	hits := canvas.Hits()
	hitTexts := make([]gfx.HitText, len(hits))
	for index := range hits {
		hitTexts[index], _ = canvas.HitText(&hits[index])
	}
	return CanvasSnapshot{
		Rectangles:   append([]gfx.Rectangle(nil), canvas.Rectangles()...),
		Triangles:    append([]gfx.Triangle(nil), canvas.Triangles()...),
		Shadows:      append([]gfx.Shadow(nil), canvas.Shadows()...),
		Images:       append([]gfx.Image(nil), canvas.Images()...),
		Texts:        append([]gfx.Text(nil), canvas.Texts()...),
		Runs:         append([]gfx.TextRun(nil), canvas.AllTextRuns()...),
		Hits:         append([]gfx.Hit(nil), hits...),
		HitTexts:     hitTexts,
		TrappedScope: canvas.TrappedFocusScope(),
		EscapeTarget: canvas.EscapeTarget(),
		InitialFocus: canvas.InitialFocus(),
		Animating:    canvas.Animating(),
		FrameDelayMs: delayMs,
	}
}

func ApplyCanvas(canvas *gfx.Canvas, snapshot CanvasSnapshot) {
	canvas.Reset()
	// Rectangles, triangles and shadows live in separate lists. A triangle
	// records how many rectangles came before it, and a shadow how many of both,
	// which is what interleaves them back into paint order.
	triangles := snapshot.Triangles
	shadows := snapshot.Shadows
	trianglesDone := 0
	replayPainting := func(rectangles int) {
		for {
			if len(shadows) > 0 &&
				int(shadows[0].RectsBefore) <= rectangles &&
				int(shadows[0].TrianglesBefore) <= trianglesDone {
				shadow := shadows[0]
				shadows = shadows[1:]
				pushLayerAndClip(canvas, shadow.Layer, shadow.ClipOf, shadow.HasClip)
				canvas.Shadow(shadow.Bounds, shadow.Radius, shadow.Style, shadow.Color)
				popLayerAndClip(canvas, shadow.Layer, shadow.HasClip)
				continue
			}
			if len(triangles) > 0 && int(triangles[0].RectsBefore) <= rectangles {
				triangle := triangles[0]
				triangles = triangles[1:]
				trianglesDone++
				pushLayerAndClip(canvas, triangle.Layer, triangle.ClipOf, triangle.HasClip)
				canvas.AppendTriangle(triangle)
				popLayerAndClip(canvas, triangle.Layer, triangle.HasClip)
				continue
			}
			break
		}
	}
	for index, rectangle := range snapshot.Rectangles {
		replayPainting(index)
		pushLayerAndClip(canvas, rectangle.Layer, rectangle.ClipOf, rectangle.HasClip)
		canvas.Rounded(rectangle.Bounds, rectangle.Radius, rectangle.Color)
		popLayerAndClip(canvas, rectangle.Layer, rectangle.HasClip)
	}
	replayPainting(len(snapshot.Rectangles))
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
	for index, hit := range snapshot.Hits {
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
			var text gfx.HitText
			if index < len(snapshot.HitTexts) {
				text = snapshot.HitTexts[index]
			}
			canvas.HitWithOptions(hit.ID, hit.Bounds, hit.Label, gfx.HitOptions{
				Role: hit.Role, Value: hit.Value, Hint: hit.Hint, Disabled: hit.Disabled,
				Checked: hit.Checked, Selected: hit.Selected, Expanded: hit.Expanded,
				Multiline: hit.Multiline, ReadOnly: hit.ReadOnly, Secure: hit.Secure, SkipFocus: !hit.Focusable, Autofocus: hit.Autofocus,
				TextBounds: text.Bounds, TextSize: text.Size, TextColor: text.Color, TextWeight: text.Weight,
				Placement: text.Placement,
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

func pushLayerAndClip(canvas *gfx.Canvas, layer uint16, clip gfx.Rect, hasClip bool) {
	if layer != 0 {
		canvas.PushLayer(layer)
	}
	if hasClip {
		canvas.Clip(clip)
	}
}

func popLayerAndClip(canvas *gfx.Canvas, layer uint16, hasClip bool) {
	if hasClip {
		canvas.Unclip()
	}
	if layer != 0 {
		canvas.PopLayer()
	}
}
