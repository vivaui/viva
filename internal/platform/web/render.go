//go:build js && wasm

package web

import (
	"syscall/js"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/bidi"
)

// render repaints the canvas vocabulary into the 2D context. Each layer is
// drawn back to front, and within a layer every primitive is drawn in the order
// the application painted it: rectangles, triangles, shadows and text are kept
// in separate lists but each carries the paint order of the frame.
func (runtime *runtime) render() {
	context := runtime.context
	clock := &runtime.clock
	clock.begin()
	runtime.paths.beginFrame()
	if runtime.painter.dirty {
		runtime.painter.dirty = false
		runtime.a11y.sync(runtime.painter.accessibility, runtime.painter.focus.Focused())
		id, input := runtime.painter.activeTextInput()
		runtime.a11y.text.sync(id, input, runtime.textInputChanged)
	}
	clock.lap(&clock.a11y)
	context.Call("save")
	context.Call("scale", runtime.dpr, runtime.dpr)
	context.Set("fillStyle", cssColor(runtime.painter.clear))
	context.Call("fillRect", 0, 0, runtime.painter.width, runtime.painter.height)
	for _, layer := range runtime.painter.layers {
		runtime.drawLayer(layer)
	}
	context.Call("restore")
	runtime.paths.endFrame()
	clock.end(runtime.painter, runtime.paths.built, runtime.paths.reused)
	if delay, ok := runtime.canvas.NextFrame(); ok {
		runtime.scheduleFrame(delay)
	}
}

// drawLayer paints one layer's primitives in paint order. Each list is in
// paint order, so the layer is a merge of four sorted streams; a label painted
// between two shapes of the same style ends a triangle run.
func (runtime *runtime) drawLayer(layer uint16) {
	context := runtime.context
	clock := &runtime.clock
	rectangles := runtime.painter.rectangles
	shadows := runtime.painter.shadows
	triangles := runtime.painter.triangles
	texts := runtime.painter.texts
	ri, si, ti, xi := 0, 0, 0, 0
	for {
		for ri < len(rectangles) && rectangles[ri].Layer != layer {
			ri++
		}
		for si < len(shadows) && shadows[si].Layer != layer {
			si++
		}
		for ti < len(triangles) && triangles[ti].Layer != layer {
			ti++
		}
		for xi < len(texts) && texts[xi].Layer != layer {
			xi++
		}
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
		if xi < len(texts) && texts[xi].Order < best {
			kind = 4
		}
		switch kind {
		case 1:
			drawRectangle(context, rectangles[ri])
			ri++
			clock.lap(&clock.rectangles)
		case 2:
			first := triangles[ti]
			if first.Shaded {
				runtime.drawShadedTriangle(first)
				ti++
			} else {
				start := ti
				ti++
				for ti < len(triangles) && sameRun(first, triangles[ti]) {
					due := si < len(shadows) && shadows[si].Order < triangles[ti].Order ||
						ri < len(rectangles) && rectangles[ri].Order < triangles[ti].Order ||
						xi < len(texts) && texts[xi].Order < triangles[ti].Order
					if due {
						break
					}
					ti++
				}
				runtime.drawSolidRun(triangles[start:ti])
			}
			clock.lap(&clock.triangles)
		case 3:
			drawShadow(context, shadows[si], runtime.dpr)
			si++
			clock.lap(&clock.rectangles)
		case 4:
			runtime.drawText(texts[xi])
			xi++
			clock.lap(&clock.texts)
		default:
			return
		}
	}
}

// drawRectangle paints one rectangle.
func drawRectangle(context js.Value, rectangle gfx.Rectangle) {
	if rectangle.HasClip {
		context.Call("save")
		clipRect(context, rectangle.ClipOf)
	}
	context.Set("fillStyle", cssColor(rectangle.Color))
	if rectangle.Radius > 0 {
		traceRoundedRect(context, rectangle.Bounds, rectangle.Radius)
		context.Call("fill")
	} else {
		context.Call("fillRect", rectangle.Bounds.X, rectangle.Bounds.Y, rectangle.Bounds.Width, rectangle.Bounds.Height)
	}
	if rectangle.HasClip {
		context.Call("restore")
	}
}

// sameRun reports whether next continues the run that first began.
func sameRun(first, next gfx.Triangle) bool {
	return next.Layer == first.Layer && !next.Shaded && next.Color == first.Color &&
		next.HasClip == first.HasClip && next.ClipOf == first.ClipOf
}

func (runtime *runtime) drawSolidRun(run []gfx.Triangle) {
	context := runtime.context
	first := run[0]
	context.Call("save")
	if first.HasClip {
		clipRect(context, first.ClipOf)
	}
	context.Call("translate", first.Points[0].X, first.Points[0].Y)
	context.Set("fillStyle", cssColor(first.Color))
	context.Call("fill", runtime.paths.path(run))
	context.Call("restore")
}

func (runtime *runtime) drawShadedTriangle(triangle gfx.Triangle) {
	context := runtime.context
	if triangle.HasClip {
		context.Call("save")
		clipRect(context, triangle.ClipOf)
	}
	context.Set("fillStyle", triangleFill(context, triangle))
	context.Call("beginPath")
	context.Call("moveTo", triangle.Points[0].X, triangle.Points[0].Y)
	context.Call("lineTo", triangle.Points[1].X, triangle.Points[1].Y)
	context.Call("lineTo", triangle.Points[2].X, triangle.Points[2].Y)
	context.Call("closePath")
	context.Call("fill")
	if triangle.HasClip {
		context.Call("restore")
	}
}

// drawText paints one text line. A line that cannot overflow its box (no outer
// clip, left to right, narrower and no taller than the box) is drawn directly;
// any other line is clipped to its box inside its own saved state.
func (runtime *runtime) drawText(text gfx.Text) {
	context := runtime.context
	context.Call("save")
	defer context.Call("restore")
	context.Set("textBaseline", "alphabetic")
	context.Set("textAlign", "left")
	context.Set("direction", "ltr")
	if text.RunCount > 0 {
		runtime.drawRuns(text)
		return
	}
	var style textStyleState
	metrics := runtime.metrics.baseline(text.Style.Size)
	baseline := runtime.canvas.LabelBaseline(gfx.Rect{Height: text.Bounds.Height}, gfx.TextStyle{Size: text.Style.Size})
	y := text.Bounds.Y + baseline
	// The line is drawn whole, in the font and direction the text engine measured it
	// with. The base direction of a right-to-left paragraph puts the
	// punctuation around it on the side the paragraph puts it.
	rtl := bidi.BaseDirection(text.Value) == bidi.RTL
	// Italic text takes the clipped path below: its width is not the regular face's that
	// the fit test here measures, and the path draws it in the font the engine measured.
	if !text.HasClip && !rtl && !text.Style.Italic && metrics.Ascent+metrics.Descent <= text.Bounds.Height &&
		runtime.metrics.width(text.Value, text.Style.Size, text.Style.Weight)+fitMargin <= text.Bounds.Width {
		style.apply(context, text.Style)
		context.Call("fillText", text.Value, text.Bounds.X, y)
		return
	}
	context.Call("save")
	if text.HasClip {
		clipRect(context, text.ClipOf)
	}
	clipRect(context, text.Bounds)
	context.Set("fillStyle", cssColor(text.Style.Color))
	context.Set("font", cssFontStyle(text.Style.Size, text.Style.Weight, text.Style.Italic))
	if rtl {
		context.Set("direction", "rtl")
	}
	context.Call("fillText", text.Value, text.Bounds.X, y)
	context.Call("restore")
}

// fitMargin is the room, in pixels, a line must leave in its box to skip the clip:
// glyphs overhang their advance a little, and a clip would cut that.
const fitMargin = 2

// textStyleState is the fill and font a context currently holds, so a run of lines
// in one style sets them once. The zero value matches nothing.
type textStyleState struct {
	set    bool
	color  gfx.Color
	size   float32
	weight float32
	italic bool
}

func (state *textStyleState) apply(context js.Value, style gfx.TextStyle) {
	if state.set && state.color == style.Color && state.size == style.Size && state.weight == style.Weight && state.italic == style.Italic {
		return
	}
	if !state.set || state.color != style.Color {
		context.Set("fillStyle", cssColor(style.Color))
	}
	if !state.set || state.size != style.Size || state.weight != style.Weight || state.italic != style.Italic {
		context.Set("font", cssFontStyle(style.Size, style.Weight, style.Italic))
	}
	*state = textStyleState{set: true, color: style.Color, size: style.Size, weight: style.Weight, italic: style.Italic}
}
