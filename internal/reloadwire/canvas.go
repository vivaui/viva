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
	// The primitive lists are separate but every primitive carries the paint
	// order of the frame it came from. Replaying them in that order makes the
	// canvas hand out the same relative order again.
	rectangles, triangles, shadows := snapshot.Rectangles, snapshot.Triangles, snapshot.Shadows
	images, texts := snapshot.Images, snapshot.Texts
	ri, ti, si, ii, xi := 0, 0, 0, 0, 0
	for ri < len(rectangles) || ti < len(triangles) || si < len(shadows) || ii < len(images) || xi < len(texts) {
		kind, best := 0, ^uint32(0)
		if ri < len(rectangles) && rectangles[ri].Order < best {
			kind, best = 1, rectangles[ri].Order
		}
		if ti < len(triangles) && triangles[ti].Order < best {
			kind, best = 2, triangles[ti].Order
		}
		if si < len(shadows) && shadows[si].Order < best {
			kind, best = 3, shadows[si].Order
		}
		if ii < len(images) && images[ii].Order < best {
			kind, best = 4, images[ii].Order
		}
		if xi < len(texts) && texts[xi].Order < best {
			kind = 5
		}
		switch kind {
		case 1:
			rectangle := rectangles[ri]
			ri++
			pushLayerAndClip(canvas, rectangle.Layer, rectangle.ClipOf, rectangle.HasClip)
			canvas.Rounded(rectangle.Bounds, rectangle.Radius, rectangle.Color)
			popLayerAndClip(canvas, rectangle.Layer, rectangle.HasClip)
		case 2:
			triangle := triangles[ti]
			ti++
			pushLayerAndClip(canvas, triangle.Layer, triangle.ClipOf, triangle.HasClip)
			canvas.AppendTriangle(triangle)
			popLayerAndClip(canvas, triangle.Layer, triangle.HasClip)
		case 3:
			shadow := shadows[si]
			si++
			pushLayerAndClip(canvas, shadow.Layer, shadow.ClipOf, shadow.HasClip)
			canvas.Shadow(shadow.Bounds, shadow.Radius, shadow.Style, shadow.Color)
			popLayerAndClip(canvas, shadow.Layer, shadow.HasClip)
		case 4:
			image := images[ii]
			ii++
			pushLayerAndClip(canvas, image.Layer, image.ClipOf, image.HasClip)
			canvas.DrawBitmapRegion(image.Bitmap, image.Source, image.Bounds)
			popLayerAndClip(canvas, image.Layer, image.HasClip)
		case 5:
			text := texts[xi]
			xi++
			pushLayerAndClip(canvas, text.Layer, text.ClipOf, text.HasClip)
			if text.RunCount > 0 || text.CellWidth > 0 {
				canvas.LabelRunsWithIdentity(text.Identity, text.Value, text.Bounds, text.Style, text.CellWidth, snapshot.Runs[text.RunFirst:text.RunFirst+text.RunCount])
			} else if text.HasQuad {
				// Bounds and Quad are already placed; resolving AlignStart again
				// would shift right-to-left text a second time.
				canvas.LabelQuadAligned(text.Value, text.Bounds, gfx.Quad{Points: text.Quad, Tint: text.Tint}, text.Style, gfx.AlignLeft)
			} else {
				canvas.LabelAligned(text.Value, text.Bounds, text.Style, gfx.AlignLeft)
			}
			popLayerAndClip(canvas, text.Layer, text.HasClip)
		}
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
