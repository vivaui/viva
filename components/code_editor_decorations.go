package components

import (
	"math"
	"sort"
	"unicode/utf8"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
)

// CodeEditorDecoration is the optional drawing extension of a CodeEditor.
// Set it to *CodeEditorDecorations; its private methods keep layout in the
// framework and let clients without decorations omit their drawing code.
// A nil *CodeEditorDecorations draws nothing; other embedded wrappers are not
// part of this interface's supported API.
type CodeEditorDecoration interface {
	codeEditorGutterWidth() float32
	codeEditorHasIndicators() bool
	codeEditorGutter(codeDecorationPaint, gfx.Rect, int)
	codeEditorIndicators(codeDecorationPaint, int)
}

// Pass only existing editor/canvas state by value across the optional dispatch.
// Passing *codePaint would make every paint escape, including undecorated ones.
type codeDecorationPaint struct {
	canvas                      *gfx.Canvas
	editor                      *CodeEditor
	size, cellWidth, lineHeight float32
	face                        gfx.MonoFace
}

// CodeEditorDecorations adds caller-owned gutter bars and byte indicators.
// Buffers stay immutable for Paint; the editor does not retain them.
type CodeEditorDecorations struct {
	// GutterWidth reserves a leading lane in logical pixels, even without bars.
	// Nonpositive or nonfinite widths omit the lane. Existing gutter lanes remain.
	GutterWidth float32
	// GutterBackground is independent of the current-line highlight.
	GutterBackground gfx.Color
	// GutterBars are sorted by logical line; same-line bars paint in caller order.
	GutterBars []CodeEditorGutterBar
	// IndicatorLayers paint in caller order, above text and below carets.
	IndicatorLayers []CodeEditorIndicatorLayer
}

// CodeEditorGutterBar paints a centered bar on every visible row of Line.
// Joins omit the end caps toward adjacent logical lines; wrapped rows connect.
type CodeEditorGutterBar struct {
	Line                  int
	Fill, Stroke          gfx.Color
	JoinBefore, JoinAfter bool
}

// CodeEditorIndicatorStyle chooses an underline or a point at a byte position.
type CodeEditorIndicatorStyle uint8

const (
	// CodeEditorUnderline paints a square two-pixel strip below a nonempty span.
	CodeEditorUnderline CodeEditorIndicatorStyle = iota
	// CodeEditorPointBelow paints an upward-pointing triangle below an empty span.
	CodeEditorPointBelow
	// CodeEditorPointAbove paints a downward-pointing triangle above an empty span.
	CodeEditorPointAbove
)

// CodeEditorIndicatorLayer paints sorted disjoint UTF-8 byte ranges of Value.
// Point styles require strictly increasing empty spans. Invalid styles or ranges
// reject the whole layer without normalizing or mutating its caller's buffer.
type CodeEditorIndicatorLayer struct {
	Spans []text.Span
	Color gfx.Color
	Style CodeEditorIndicatorStyle
}

func (decorations *CodeEditorDecorations) codeEditorGutterWidth() float32 {
	if decorations != nil && decorations.GutterWidth > 0 && decorations.GutterWidth <= math.MaxFloat32 {
		return decorations.GutterWidth
	}
	return 0
}

func (decorations *CodeEditorDecorations) codeEditorHasIndicators() bool {
	return decorations != nil && len(decorations.IndicatorLayers) != 0
}

func (decorations *CodeEditorDecorations) codeEditorGutter(paint codeDecorationPaint, lane gfx.Rect, firstRow int) {
	if decorations != nil {
		paint.gutterDecorations(decorations, lane, firstRow)
	}
}

func (decorations *CodeEditorDecorations) codeEditorIndicators(paint codeDecorationPaint, firstRow int) {
	if decorations != nil {
		paint.indicatorLayers(decorations.IndicatorLayers, firstRow)
	}
}

func decorationColor(color gfx.Color, disabled bool) gfx.Color {
	if disabled {
		color.Alpha *= 0.5
	}
	return color
}

func (paint *codeDecorationPaint) gutterDecorations(decorations *CodeEditorDecorations, lane gfx.Rect, firstRow int) {
	canvas, editor := paint.canvas, paint.editor
	canvas.Clip(lane)
	defer canvas.Unclip()
	canvas.Fill(lane, decorationColor(decorations.GutterBackground, editor.disabled))
	bars := decorations.GutterBars
	previous := -1
	for _, bar := range bars {
		if bar.Line < previous || bar.Line < 0 || bar.Line >= editor.lineCount {
			return
		}
		previous = bar.Line
	}
	if firstRow >= len(editor.rows) || len(bars) == 0 {
		return
	}
	barWidth := float32(math.Ceil(float64(lane.Width / 3)))
	x := float32(math.Floor(float64(lane.X+lane.Width/2))) - float32(math.Floor(float64(barWidth/2)))
	first := sort.Search(len(bars), func(index int) bool { return bars[index].Line >= int(editor.rows[firstRow].line) })
	for index := firstRow; index < len(editor.rows); index++ {
		y := editor.content.Y + float32(index)*paint.lineHeight - editor.scrollY
		if y >= lane.Y+lane.Height {
			break
		}
		row := editor.rows[index]
		for first < len(bars) && bars[first].Line < int(row.line) {
			first++
		}
		for at := first; at < len(bars) && bars[at].Line == int(row.line); at++ {
			bar := bars[at]
			before := !editor.firstOfLine(index) || bar.JoinBefore
			after := editor.softWrapped(index) || bar.JoinAfter
			paint.gutterBar(gfx.Rect{X: x, Y: y, Width: barWidth, Height: paint.lineHeight}, bar, before, after)
		}
	}
}

func (paint *codeDecorationPaint) gutterBar(bounds gfx.Rect, bar CodeEditorGutterBar, before, after bool) {
	canvas := paint.canvas
	fill := decorationColor(bar.Fill, paint.editor.disabled)
	stroke := decorationColor(bar.Stroke, paint.editor.disabled)
	if fill == stroke || stroke.Alpha == 0 {
		canvas.Fill(bounds, fill)
		return
	}
	// Keep fill and outline disjoint so translucent bars have uniform opacity.
	border := min(float32(1), bounds.Width/2, bounds.Height/2)
	top, bottom := float32(0), float32(0)
	if !before {
		top = border
	}
	if !after {
		bottom = border
	}
	canvas.Fill(gfx.Rect{X: bounds.X, Y: bounds.Y, Width: border, Height: bounds.Height}, stroke)
	canvas.Fill(gfx.Rect{X: bounds.X + bounds.Width - border, Y: bounds.Y, Width: border, Height: bounds.Height}, stroke)
	inner := gfx.Rect{X: bounds.X + border, Y: bounds.Y + top, Width: max(0, bounds.Width-2*border), Height: max(0, bounds.Height-top-bottom)}
	canvas.Fill(inner, fill)
	if top > 0 {
		canvas.Fill(gfx.Rect{X: inner.X, Y: bounds.Y, Width: inner.Width, Height: top}, stroke)
	}
	if bottom > 0 {
		canvas.Fill(gfx.Rect{X: inner.X, Y: bounds.Y + bounds.Height - bottom, Width: inner.Width, Height: bottom}, stroke)
	}
}

func validIndicatorLayer(value string, layer CodeEditorIndicatorLayer) bool {
	if layer.Style > CodeEditorPointAbove {
		return false
	}
	point := layer.Style != CodeEditorUnderline
	lastStart, lastEnd := -1, -1
	for _, span := range layer.Spans {
		if span.Start < 0 || span.End < span.Start || span.End > len(value) ||
			span.Start < len(value) && !utf8.RuneStart(value[span.Start]) ||
			span.End < len(value) && !utf8.RuneStart(value[span.End]) {
			return false
		}
		if point {
			if span.Start != span.End || span.Start <= lastStart {
				return false
			}
		} else if span.Start == span.End || span.Start < lastEnd {
			return false
		}
		lastStart, lastEnd = span.Start, span.End
	}
	return true
}

func (paint *codeDecorationPaint) indicatorLayers(layers []CodeEditorIndicatorLayer, firstRow int) {
	editor := paint.editor
	baseline := paint.canvas.MonoBaseline(paint.size, paint.face, paint.lineHeight)
	pointHeight := float32(3)
	if baseline > 0 {
		pointHeight = float32(math.Floor(float64(max(float32(3), paint.lineHeight-baseline))))
	}
	for _, layer := range layers {
		if layer.Color.Alpha == 0 || !validIndicatorLayer(editor.text, layer) {
			continue
		}
		color := decorationColor(layer.Color, editor.disabled)
		point := layer.Style != CodeEditorUnderline
		for index := firstRow; index < len(editor.rows); index++ {
			y := editor.content.Y + float32(index)*paint.lineHeight - editor.scrollY
			if y >= editor.content.Y+editor.content.Height {
				break
			}
			row := editor.rows[index]
			start, end := int(row.start), int(row.end)
			first := sort.Search(len(layer.Spans), func(at int) bool {
				if point {
					return layer.Spans[at].Start >= start
				}
				return layer.Spans[at].End > start
			})
			walker := columnWalker{editor: editor, row: row, at: start}
			left := editor.content.X - editor.scrollX
			for _, span := range layer.Spans[first:] {
				if span.Start > end || span.Start == end && (!point || editor.softWrapped(index)) {
					break
				}
				if point {
					x := left + float32(walker.column(span.Start))*paint.cellWidth
					bottom := y + paint.lineHeight
					if baseline > 0 {
						bottom = y + max(paint.lineHeight, baseline+3)
					}
					paint.indicatorPoint(x, y, bottom, pointHeight, color, layer.Style)
					continue
				}
				a, b := max(start, span.Start), min(end, span.End)
				if a >= b {
					continue
				}
				x0 := left + float32(walker.column(a))*paint.cellWidth + 1
				x1 := left + float32(walker.column(b))*paint.cellWidth - 1
				x0, x1 = max(x0, editor.content.X), min(x1, editor.content.X+editor.content.Width)
				if x1 > x0 {
					paint.canvas.Fill(gfx.Rect{X: x0, Y: y + paint.lineHeight - 2, Width: x1 - x0, Height: 2}, color)
				}
			}
		}
	}
}

func (paint *codeDecorationPaint) indicatorPoint(x, top, bottom, height float32, color gfx.Color, style CodeEditorIndicatorStyle) {
	x = float32(math.Round(float64(x))) + 0.5
	base := float32(math.Ceil(float64(bottom))) + 0.5
	tip := base - height
	if style == CodeEditorPointAbove {
		base = float32(math.Floor(float64(top))) - 0.5
		tip = base + height
	}
	paint.canvas.AppendTriangle(gfx.Triangle{
		Points: [3]gfx.Point{{X: x - height, Y: base}, {X: x + height, Y: base}, {X: x, Y: tip}},
		Color:  color, EdgeMask: 7,
	})
}
