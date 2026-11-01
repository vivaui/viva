//go:build js && wasm

package web

import (
	"syscall/js"

	"github.com/vivaui/viva/internal/bidi"
	"github.com/vivaui/viva/ui"
)

// render repaints the canvas vocabulary into the 2D context. Each layer is
// drawn back to front as rectangles (with the shadows recorded among them),
// then triangles, then text.
func (runtime *runtime) render() {
	context := runtime.context
	clock := &runtime.clock
	clock.begin()
	runtime.paths.beginFrame()
	if runtime.painter.dirty {
		runtime.painter.dirty = false
		runtime.a11y.sync(runtime.painter.accessibility, runtime.painter.focused)
		id, input := runtime.painter.activeTextInput()
		runtime.a11y.text.sync(id, input, runtime.textInputChanged)
	}
	clock.lap(&clock.a11y)
	context.Call("save")
	context.Call("scale", runtime.dpr, runtime.dpr)
	context.Set("fillStyle", cssColor(runtime.painter.clear))
	context.Call("fillRect", 0, 0, runtime.painter.width, runtime.painter.height)
	for _, layer := range runtime.painter.layers {
		runtime.drawRectangles(layer)
		clock.lap(&clock.rectangles)
		runtime.drawTriangles(layer)
		clock.lap(&clock.triangles)
		runtime.drawTexts(layer)
		clock.lap(&clock.texts)
	}
	context.Call("restore")
	runtime.paths.endFrame()
	clock.end(runtime.painter, runtime.paths.built, runtime.paths.reused)
}

// drawRectangles paints one layer's rectangles. A shadow is painted where it
// was recorded among them, and shadows recorded after the last rectangle come last.
func (runtime *runtime) drawRectangles(layer uint16) {
	context := runtime.context
	shadows := runtime.painter.shadows
	drawShadowsBefore := func(rectangles int) {
		for len(shadows) > 0 && int(shadows[0].RectsBefore) <= rectangles {
			if shadow := shadows[0]; shadow.Layer == layer {
				drawShadow(context, shadow, runtime.dpr)
			}
			shadows = shadows[1:]
		}
	}
	for number, rectangle := range runtime.painter.rectangles {
		if rectangle.Layer != layer {
			continue
		}
		drawShadowsBefore(number)
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
	drawShadowsBefore(len(runtime.painter.rectangles))
}

// drawTriangles paints one layer's triangles. Consecutive solid triangles of one
// color and clip (an icon, the strokes of a glyph) are one cached path and one
// fill; a shaded triangle is drawn alone with its gradient.
func (runtime *runtime) drawTriangles(layer uint16) {
	triangles := runtime.painter.triangles
	for start := 0; start < len(triangles); {
		first := triangles[start]
		if first.Layer != layer {
			start++
			continue
		}
		end := start + 1
		if !first.Shaded {
			for end < len(triangles) && sameRun(first, triangles[end]) {
				end++
			}
		}
		if first.Shaded {
			runtime.drawShadedTriangle(first)
		} else {
			runtime.drawSolidRun(triangles[start:end])
		}
		start = end
	}
}

// sameRun reports whether next continues the run that first began.
func sameRun(first, next ui.Triangle) bool {
	return next.Layer == first.Layer && !next.Shaded && next.Color == first.Color &&
		next.HasClip == first.HasClip && next.ClipOf == first.ClipOf
}

func (runtime *runtime) drawSolidRun(run []ui.Triangle) {
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

func (runtime *runtime) drawShadedTriangle(triangle ui.Triangle) {
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

// drawTexts paints one layer's text lines. A line that cannot overflow its box
// (no outer clip, left to right, narrower and no taller than the box) is drawn
// directly with only the style that changed since the last line; any other
// line is clipped to its box inside its own saved state.
func (runtime *runtime) drawTexts(layer uint16) {
	context := runtime.context
	context.Call("save")
	defer context.Call("restore")
	context.Set("textBaseline", "alphabetic")
	context.Set("textAlign", "left")
	context.Set("direction", "ltr")
	var style textStyleState
	for _, text := range runtime.painter.texts {
		if text.Layer != layer {
			continue
		}
		if text.RunCount > 0 {
			runtime.drawRuns(text)
			continue
		}
		metrics := runtime.metrics.baseline(text.Style.Size)
		baseline := (text.Bounds.Height-metrics.Ascent-metrics.Descent)*0.5 + metrics.Ascent
		y := text.Bounds.Y + baseline
		// The line is drawn whole, in the font and direction the text engine measured it
		// with. The base direction of a right-to-left paragraph puts the
		// punctuation around it on the side the paragraph puts it.
		rtl := bidi.BaseDirection(text.Value) == bidi.RTL
		if !text.HasClip && !rtl && metrics.Ascent+metrics.Descent <= text.Bounds.Height &&
			runtime.metrics.width(text.Value, text.Style.Size, text.Style.Weight)+fitMargin <= text.Bounds.Width {
			style.apply(context, text.Style)
			context.Call("fillText", text.Value, text.Bounds.X, y)
			continue
		}
		context.Call("save")
		if text.HasClip {
			clipRect(context, text.ClipOf)
		}
		clipRect(context, text.Bounds)
		context.Set("fillStyle", cssColor(text.Style.Color))
		context.Set("font", cssFont(text.Style.Size, text.Style.Weight))
		if rtl {
			context.Set("direction", "rtl")
		}
		context.Call("fillText", text.Value, text.Bounds.X, y)
		context.Call("restore")
	}
}

// fitMargin is the room, in pixels, a line must leave in its box to skip the clip:
// glyphs overhang their advance a little, and a clip would cut that.
const fitMargin = 2

// textStyleState is the fill and font a context currently holds, so a run of lines
// in one style sets them once. The zero value matches nothing.
type textStyleState struct {
	set    bool
	color  ui.Color
	size   float32
	weight float32
}

func (state *textStyleState) apply(context js.Value, style ui.TextStyle) {
	if state.set && state.color == style.Color && state.size == style.Size && state.weight == style.Weight {
		return
	}
	if !state.set || state.color != style.Color {
		context.Set("fillStyle", cssColor(style.Color))
	}
	if !state.set || state.size != style.Size || state.weight != style.Weight {
		context.Set("font", cssFont(style.Size, style.Weight))
	}
	*state = textStyleState{true, style.Color, style.Size, style.Weight}
}
