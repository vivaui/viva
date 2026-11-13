package components

import (
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
)

func TestCodeEditorDecorationGutterReservesEmptyLane(t *testing.T) {
	for _, custom := range []bool{false, true} {
		fixture := newCodeFixture("one\ntwo\n")
		fixture.snapshot.LineNumbers, fixture.snapshot.MarkerGutter, fixture.snapshot.FoldGutter = true, true, true
		inset := fixture.theme.Metrics.TextInsetX()
		if custom {
			fixture.snapshot.Gutter = &CodeEditorGutter{Inset: 2, Marker: 12, Bookmark: 6, LineNumberPadding: 3, Fold: 7, FoldLast: true}
			inset = 2
		}
		fixture.paint()
		before := fixture.editor.content
		background := gfx.RGB(224, 224, 224)
		decorations := &CodeEditorDecorations{GutterWidth: 9, GutterBackground: background}
		fixture.snapshot.Decorations = decorations
		fixture.snapshot.CurrentLine = true
		fixture.paint()
		if !near(fixture.editor.content.X, before.X+9) || !near(fixture.editor.content.Width, before.Width-9) {
			t.Fatalf("custom=%v: empty lane changed existing lanes: %+v -> %+v", custom, before, fixture.editor.content)
		}
		lane := gfx.Rect{X: fixture.bounds.X + inset, Y: before.Y, Width: 9, Height: before.Height}
		if fills := fixture.fills(background); len(fills) != 1 || fills[0] != lane {
			t.Fatalf("custom=%v: empty background = %+v, want %+v", custom, fills, lane)
		}
		tint := fixture.theme.Foreground
		tint.Alpha *= 0.05
		for _, rect := range fixture.fills(tint) {
			if rect.X < lane.X+lane.Width && rect.X+rect.Width > lane.X {
				t.Fatalf("current-line tint covers the reserved background: %+v", rect)
			}
		}
		decorations.GutterBars = []CodeEditorGutterBar{{Line: 0, Fill: gfx.RGB(255, 128, 0), Stroke: gfx.RGB(255, 128, 0)}}
		fixture.paint()
		withBar := fixture.editor.content
		decorations.GutterBars = nil
		fixture.paint()
		if fixture.editor.content != withBar {
			t.Fatal("adding or removing history shifted the text")
		}
	}
}

func TestCodeEditorDecorationBarsFollowWrappedAndVisibleRows(t *testing.T) {
	fixture := newCodeFixture("abcdefghijklmnop\nhidden\nlast\n")
	fixture.bounds.Width = fixture.widthFor(6)
	fixture.snapshot.Wrap = true
	fixture.snapshot.FoldedLines = []CodeEditorFoldRange{{Header: 0, LastLine: 1}}
	orange, green := gfx.RGB(255, 128, 0), gfx.RGB(0, 160, 0)
	fixture.snapshot.Decorations = &CodeEditorDecorations{
		GutterWidth: 9,
		GutterBars: []CodeEditorGutterBar{
			{Line: 0, Fill: orange, Stroke: orange},
			{Line: 1, Fill: green, Stroke: green},
		},
	}
	fixture.paint()
	rows := 0
	for _, row := range fixture.editor.rows {
		if row.line == 0 {
			rows++
		}
	}
	if rows < 2 || len(fixture.fills(orange)) != rows || len(fixture.fills(green)) != 0 {
		t.Fatalf("bars do not follow visible wraps: rows=%d orange=%v green=%v", rows, fixture.fills(orange), fixture.fills(green))
	}
	for _, bar := range fixture.fills(orange) {
		if !near(bar.Width, 3) || bar.X+bar.Width >= fixture.editor.content.X {
			t.Fatalf("bar did not stay centered in its 9px lane: %+v", bar)
		}
	}
	_, _, height := fixture.cell()
	fixture.bounds.Height = 3*height + 2*codeEditorPadY
	fixture.editor.scrollY = height
	fixture.paint()
	if fills := fixture.fills(orange); len(fills) == 0 || len(fills) >= rows {
		t.Fatalf("scroll did not restrict decorated wraps: %+v", fills)
	}
}

func TestCodeEditorIndicatorSpansUseTextGeometry(t *testing.T) {
	fixture := newCodeFixture("\t\u732bab\nlast")
	orange, green := gfx.RGB(255, 128, 0), gfx.RGB(0, 160, 0)
	fixture.snapshot.Decorations = &CodeEditorDecorations{IndicatorLayers: []CodeEditorIndicatorLayer{
		{Spans: []text.Span{{Start: 1, End: 4}}, Color: orange, Style: CodeEditorUnderline},
		{Spans: []text.Span{{Start: 4, End: 4}}, Color: green, Style: CodeEditorPointBelow},
	}}
	fixture.paint()
	cell, _, line := fixture.cell()
	fills := fixture.fills(orange)
	if len(fills) != 1 || !near(fills[0].X, fixture.editor.content.X+4*cell+1) || !near(fills[0].Width, 2*cell-2) || !near(fills[0].Y, fixture.editor.content.Y+line-2) || fills[0].Height != 2 {
		t.Fatalf("Unicode/tab underline = %+v", fills)
	}
	points := decorationTriangles(fixture, green)
	if len(points) != 1 || !near(points[0].Points[2].X, float32(math.Round(float64(fixture.editor.content.X+6*cell)))+0.5) || points[0].Points[2].Y >= points[0].Points[0].Y {
		t.Fatalf("deletion is not anchored at its byte position: %+v", points)
	}
	if fixture.snapshot.Value != "\t\u732bab\nlast" || fixture.snapshot.Caret != 0 {
		t.Fatal("decorations mutated the source or caret")
	}
}

func decorationTriangles(fixture *codeFixture, color gfx.Color) []gfx.Triangle {
	var result []gfx.Triangle
	for _, triangle := range fixture.canvas.Triangles() {
		if triangle.Color == color {
			result = append(result, triangle)
		}
	}
	return result
}

func TestCodeEditorIndicatorPointsIgnoreCaretAffinityAndHiddenLines(t *testing.T) {
	fixture := newCodeFixture("abcd\nhidden\nlast\n")
	fixture.bounds.Width = fixture.widthFor(2)
	fixture.bounds.Height = 600
	fixture.snapshot.Wrap = true
	fixture.snapshot.Caret = 2
	fixture.snapshot.FoldedLines = []CodeEditorFoldRange{{Header: 0, LastLine: 1}}
	color := gfx.RGB(200, 40, 100)
	fixture.snapshot.Decorations = &CodeEditorDecorations{IndicatorLayers: []CodeEditorIndicatorLayer{
		{Spans: []text.Span{{Start: 2, End: 2}, {Start: 6, End: 6}, {Start: 17, End: 17}}, Color: color, Style: CodeEditorPointAbove},
	}}
	fixture.paint()
	before := decorationTriangles(fixture, color)
	if len(before) != 2 {
		t.Fatalf("expected a downstream wrap point and final EOF point, not a hidden-line point: %+v", before)
	}
	_, _, height := fixture.cell()
	if !near(before[0].Points[0].Y, fixture.editor.content.Y+height-0.5) {
		t.Fatalf("soft-wrap point did not belong downstream: %+v", before[0])
	}
	fixture.editor.setUpstream(2, true)
	fixture.paint()
	after := decorationTriangles(fixture, color)
	for index := range before {
		if before[index].Points != after[index].Points {
			t.Fatalf("caret affinity moved a point: %+v -> %+v", before[index], after[index])
		}
	}
}

func TestCodeEditorMalformedIndicatorLayerIsRejected(t *testing.T) {
	color := gfx.RGB(200, 50, 80)
	for _, layer := range []CodeEditorIndicatorLayer{
		{Spans: []text.Span{{Start: -1, End: 1}}, Style: CodeEditorUnderline},
		{Spans: []text.Span{{Start: 2, End: 1}}, Style: CodeEditorUnderline},
		{Spans: []text.Span{{Start: 0, End: 99}}, Style: CodeEditorUnderline},
		{Spans: []text.Span{{Start: 1, End: 3}}, Style: CodeEditorUnderline},
		{Spans: []text.Span{{Start: 0, End: 0}}, Style: CodeEditorUnderline},
		{Spans: []text.Span{{Start: 0, End: 3}}, Style: CodeEditorPointAbove},
		{Spans: []text.Span{{Start: 3, End: 4}, {Start: 0, End: 3}}, Style: CodeEditorUnderline},
		{Spans: []text.Span{{Start: 0, End: 4}, {Start: 3, End: 4}}, Style: CodeEditorUnderline},
		{Spans: []text.Span{{Start: 0, End: 0}, {Start: 0, End: 0}}, Style: CodeEditorPointBelow},
		{Spans: []text.Span{{Start: 0, End: 3}}, Style: CodeEditorIndicatorStyle(255)},
	} {
		fixture := newCodeFixture("\u732ba")
		layer.Color = color
		before := append([]text.Span(nil), layer.Spans...)
		fixture.snapshot.Decorations = &CodeEditorDecorations{IndicatorLayers: []CodeEditorIndicatorLayer{layer}}
		fixture.paint()
		if len(fixture.fills(color)) != 0 || len(decorationTriangles(fixture, color)) != 0 {
			t.Fatalf("painted malformed layer: %+v", layer)
		}
		if !reflect.DeepEqual(layer.Spans, before) {
			t.Fatal("validation modified the caller's spans")
		}
	}
}

func TestCodeEditorDecorationOrderKeepsCaretsAboveCrossRowPoints(t *testing.T) {
	fixture := newCodeFixture("one\ntwo")
	first, second, caret := gfx.RGB(0, 160, 0), gfx.RGB(255, 128, 0), gfx.RGB(1, 30, 200)
	fixture.snapshot.Caret, fixture.snapshot.CaretColor = 4, caret
	fixture.snapshot.CompositionStart, fixture.snapshot.CompositionEnd = 0, 2
	fixture.snapshot.Decorations = &CodeEditorDecorations{IndicatorLayers: []CodeEditorIndicatorLayer{
		{Spans: []text.Span{{Start: 3, End: 3}}, Color: first, Style: CodeEditorPointBelow},
		{Spans: []text.Span{{Start: 4, End: 4}}, Color: second, Style: CodeEditorPointAbove},
	}}
	fixture.paint()
	var lastText, firstPoint, secondPoint, compositionOrder, caretOrder uint32
	for _, label := range fixture.canvas.Texts() {
		lastText = max(lastText, label.Order)
	}
	for _, triangle := range fixture.canvas.Triangles() {
		if triangle.Color == first {
			firstPoint = triangle.Order
		}
		if triangle.Color == second {
			secondPoint = triangle.Order
		}
	}
	for _, rect := range fixture.canvas.Rectangles() {
		if rect.Color == caret {
			caretOrder = rect.Order
		}
		if rect.Color == fixture.theme.Foreground && rect.Bounds.Height == 1 {
			compositionOrder = rect.Order
		}
	}
	if firstPoint <= lastText || secondPoint <= firstPoint || compositionOrder <= secondPoint || caretOrder <= secondPoint {
		t.Fatalf("base text / indicator layers / composition / caret order = %d / %d,%d / %d / %d", lastText, firstPoint, secondPoint, compositionOrder, caretOrder)
	}
}

func TestCodeEditorDecorationWidthRejectsNonfiniteValues(t *testing.T) {
	for _, width := range []float32{0, -1, float32(math.NaN()), float32(math.Inf(1))} {
		fixture := newCodeFixture(strings.Repeat("x", 30))
		fixture.paint()
		before := fixture.editor.content
		fixture.snapshot.Decorations = &CodeEditorDecorations{GutterWidth: width}
		fixture.paint()
		if fixture.editor.content != before {
			t.Fatalf("invalid gutter width %v changed layout: %+v -> %+v", width, before, fixture.editor.content)
		}
	}
}

func TestCodeEditorDecorationBarLayersAndJoins(t *testing.T) {
	fixture := newCodeFixture("one\ntwo\n")
	fill, stroke, upper := gfx.RGB(210, 210, 210), gfx.RGB(255, 128, 0), gfx.RGB(0, 160, 0)
	bar := CodeEditorGutterBar{Line: 0, Fill: fill, Stroke: stroke}
	fixture.snapshot.Decorations = &CodeEditorDecorations{GutterWidth: 9, GutterBars: []CodeEditorGutterBar{bar}}
	fixture.paint()
	closed := fixture.fills(stroke)
	if len(closed) != 4 || len(fixture.fills(fill)) != 1 {
		t.Fatalf("outlined bar has no distinct caps/sides/fill: %+v", closed)
	}
	fixture.snapshot.Decorations.(*CodeEditorDecorations).GutterBars[0].JoinBefore = true
	fixture.paint()
	if joined := fixture.fills(stroke); len(joined) != 3 {
		t.Fatalf("logical join retained an end cap: %+v", joined)
	}
	fixture.snapshot.Decorations.(*CodeEditorDecorations).GutterBars = []CodeEditorGutterBar{
		{Line: 0, Fill: stroke, Stroke: stroke},
		{Line: 0, Fill: upper, Stroke: upper},
	}
	fixture.paint()
	if a, b := fixture.fills(stroke), fixture.fills(upper); len(a) != 1 || len(b) != 1 || a[0] != b[0] {
		t.Fatalf("same-line edition layers do not share bar geometry: %v / %v", a, b)
	}
	var first, second uint32
	for _, rect := range fixture.canvas.Rectangles() {
		if rect.Color == stroke {
			first = rect.Order
		}
		if rect.Color == upper {
			second = rect.Order
		}
	}
	if first == 0 || second <= first {
		t.Fatalf("bar caller order changed: %d, %d", first, second)
	}
}

func TestCodeEditorDecorationHorizontalScrollAndDisabledColors(t *testing.T) {
	fixture := newCodeFixture(strings.Repeat("a", 100))
	fixture.bounds.Width = fixture.widthFor(12)
	fixture.snapshot.HorizontalScrollbarID = gfx.Item(9)
	barColor, indicatorColor := gfx.RGB(0, 160, 0), gfx.RGB(255, 128, 0)
	fixture.snapshot.Decorations = &CodeEditorDecorations{
		GutterWidth: 9, GutterBars: []CodeEditorGutterBar{{Line: 0, Fill: barColor, Stroke: barColor}},
		IndicatorLayers: []CodeEditorIndicatorLayer{{Spans: []text.Span{{Start: 4, End: 8}}, Color: indicatorColor}},
	}
	fixture.paint()
	beforeBar, beforeIndicator := fixture.fills(barColor), fixture.fills(indicatorColor)
	if len(beforeBar) != 1 || len(beforeIndicator) != 1 {
		t.Fatal("fixture did not draw a bar and underline")
	}
	cell, _, _ := fixture.cell()
	fixture.editor.scrollX = 2 * cell
	fixture.paint()
	afterBar, afterIndicator := fixture.fills(barColor), fixture.fills(indicatorColor)
	if len(afterBar) != 1 || afterBar[0] != beforeBar[0] || len(afterIndicator) != 1 || !near(afterIndicator[0].X, beforeIndicator[0].X-2*cell) {
		t.Fatalf("horizontal scroll moved the wrong presentation: bars=%v/%v indicators=%v/%v", beforeBar, afterBar, beforeIndicator, afterIndicator)
	}
	fixture.snapshot.Disabled = true
	fixture.paint()
	barColor.Alpha, indicatorColor.Alpha = 0.5, 0.5
	if len(fixture.fills(barColor)) != 1 || len(fixture.fills(indicatorColor)) != 1 || fixture.snapshot.Decorations.(*CodeEditorDecorations).IndicatorLayers[0].Color.Alpha != 1 {
		t.Fatal("disabled decorations were not uniformly dimmed without caller mutation")
	}
}

func TestCodeEditorIndicatorsPreserveZeroWidthRepresentations(t *testing.T) {
	fixture := newCodeFixture("e\u0301x")
	fixture.snapshot.Representations = []CodeRepresentation{{Character: 'x', Label: ""}}
	underline, point := gfx.RGB(255, 128, 0), gfx.RGB(0, 160, 0)
	fixture.snapshot.Decorations = &CodeEditorDecorations{IndicatorLayers: []CodeEditorIndicatorLayer{
		{Spans: []text.Span{{Start: 1, End: 3}, {Start: 3, End: 4}}, Color: underline},
		{Spans: []text.Span{{Start: 1, End: 1}, {Start: 3, End: 3}}, Color: point, Style: CodeEditorPointBelow},
	}}
	fixture.paint()
	triangles := decorationTriangles(fixture, point)
	if len(fixture.fills(underline)) != 0 || len(triangles) != 2 || triangles[0].Points != triangles[1].Points {
		t.Fatalf("zero-width source ranges invented cells or moved points: underlines=%v points=%v", fixture.fills(underline), triangles)
	}
}

func TestCodeEditorDecorationsPaintWithoutSteadyAllocations(t *testing.T) {
	fixture := newCodeFixture("one\ntwo\nthree\n")
	color := gfx.RGB(0, 160, 0)
	fixture.snapshot.Decorations = &CodeEditorDecorations{
		GutterWidth: 9, GutterBars: []CodeEditorGutterBar{{Line: 1, Fill: color, Stroke: color}},
		IndicatorLayers: []CodeEditorIndicatorLayer{
			{Spans: []text.Span{{Start: 4, End: 7}}, Color: color},
			{Spans: []text.Span{{Start: 1, End: 1}, {Start: 8, End: 8}}, Color: color, Style: CodeEditorPointAbove},
		},
	}
	fixture.paint()
	if allocations := testing.AllocsPerRun(100, fixture.paint); allocations != 0 {
		t.Fatalf("unchanged decorated paint allocated %g times", allocations)
	}
}

func TestCodeEditorTypedNilDecorationKeepsDefaultDrawing(t *testing.T) {
	fixture := newCodeFixture("one\ntwo\n")
	fixture.paint()
	content := fixture.editor.content
	var decorations *CodeEditorDecorations
	fixture.snapshot.Decorations = decorations
	fixture.paint()
	if fixture.editor.content != content {
		t.Fatal("a typed nil decoration changed the default layout")
	}
	if allocations := testing.AllocsPerRun(100, fixture.paint); allocations != 0 {
		t.Fatalf("typed nil decoration allocated %g times", allocations)
	}
}

func TestCodeEditorOversizedDecorationCannotCoverAdjacentEditor(t *testing.T) {
	var canvas gfx.Canvas
	var left, right CodeEditor
	theme := gfx.LightTheme()
	right.Paint(&canvas, theme, gfx.Rect{X: 100, Width: 100, Height: 200}, CodeEditorSnapshot{Value: "right"})
	background, bar := gfx.RGB(234, 56, 78), gfx.RGB(88, 99, 11)
	left.Paint(&canvas, theme, gfx.Rect{Width: 100, Height: 200}, CodeEditorSnapshot{
		Value: "left", LineNumbers: true,
		Decorations: &CodeEditorDecorations{
			GutterWidth: 200, GutterBackground: background,
			GutterBars: []CodeEditorGutterBar{{Line: 0, Fill: bar, Stroke: bar}},
		},
	})
	for _, rect := range canvas.Rectangles() {
		if rect.Color != background && rect.Color != bar {
			continue
		}
		if !rect.HasClip || min(rect.Bounds.X+rect.Bounds.Width, rect.ClipOf.X+rect.ClipOf.Width) > 100 {
			t.Fatalf("left decoration can paint over the right editor: bounds=%+v clip=%+v", rect.Bounds, rect.ClipOf)
		}
	}
	for _, label := range canvas.Texts() {
		if label.Value == "1" && (!label.HasClip || label.ClipOf.X+label.ClipOf.Width > 100) {
			t.Fatalf("oversized decoration exposed a neighboring gutter label: %+v", label)
		}
	}
}

func TestCodeEditorOversizedDecorationClipsFoldPaintingAndHits(t *testing.T) {
	for _, width := range []float32{80, 120} {
		var canvas gfx.Canvas
		var editor CodeEditor
		fold := gfx.Item(1)
		editor.Paint(&canvas, gfx.LightTheme(), gfx.Rect{Width: 100, Height: 200}, CodeEditorSnapshot{
			Value:       "left",
			FoldMarkers: []CodeEditorFoldMarker{{Line: 0, ID: fold}},
			Decorations: &CodeEditorDecorations{GutterWidth: width},
		})
		if len(canvas.Triangles()) == 0 {
			t.Fatal("fixture did not draw the fold chevron")
		}
		for _, triangle := range canvas.Triangles() {
			if !triangle.HasClip || triangle.ClipOf.Width > 0 && triangle.ClipOf.X+triangle.ClipOf.Width > 100 {
				t.Errorf("width=%v: fold chevron escaped editor: %+v", width, triangle)
			}
		}
		for _, hit := range canvas.Hits() {
			if hit.ID == fold && hit.Bounds.X+hit.Bounds.Width > 100 {
				t.Errorf("width=%v: fold hit escaped editor: %+v", width, hit.Bounds)
			}
		}
		if len(canvas.Clips()) != 0 {
			t.Fatal("editor left an unbalanced clip")
		}
	}
}

func TestCodeEditorOversizedDecorationHidesCaretOutsideEditor(t *testing.T) {
	for _, value := range []string{"", "left"} {
		fixture := newCodeFixture(value)
		fixture.bounds.Width = 100
		fixture.snapshot.Wrap = true
		fixture.snapshot.Decorations = &CodeEditorDecorations{GutterWidth: 120}
		fixture.paint()
		if caret, visible := fixture.editor.CaretBounds(); visible {
			t.Errorf("value=%q: an invisible text area exposed an IME caret outside the editor: %+v", value, caret)
		}
	}
}

type decorationFontMetrics struct {
	gfx.TextEngine
	metrics gfx.MonoCellMetrics
}

func (engine *decorationFontMetrics) MonoCell(float32, gfx.MonoFace) gfx.MonoCellMetrics {
	return engine.metrics
}

func TestCodeEditorIndicatorPointsUseMeasuredBaseline(t *testing.T) {
	for _, test := range []struct {
		size             float32
		metrics          gfx.MonoCellMetrics
		lineHeight, base float32
		pointHeight      float32
	}{
		{20, gfx.MonoCellMetrics{Advance: 10, Ascent: 14, Descent: 6}, 30, 19, 11},
		{24, gfx.MonoCellMetrics{Advance: 12, Ascent: 34, Descent: 2}, 36, 34, 3},
	} {
		fixture := newCodeFixture("ab")
		fixture.snapshot.FontSize = test.size
		fixture.canvas.SetTextEngine(&decorationFontMetrics{TextEngine: gfx.ReferenceTextEngine(), metrics: test.metrics})
		below, above := gfx.RGB(0, 160, 0), gfx.RGB(255, 128, 0)
		fixture.snapshot.Decorations = &CodeEditorDecorations{IndicatorLayers: []CodeEditorIndicatorLayer{
			{Spans: []text.Span{{Start: 1, End: 1}}, Color: below, Style: CodeEditorPointBelow},
			{Spans: []text.Span{{Start: 1, End: 1}}, Color: above, Style: CodeEditorPointAbove},
		}}
		fixture.paint()
		if actual := fixture.canvas.MonoBaseline(test.size, gfx.MonoSystem, test.lineHeight); actual != test.base {
			t.Fatalf("baseline fixture = %v, want %v", actual, test.base)
		}
		bottomPoints, topPoints := decorationTriangles(fixture, below), decorationTriangles(fixture, above)
		if len(bottomPoints) != 1 || len(topPoints) != 1 {
			t.Fatal("measured-font point was not drawn")
		}
		bottom := fixture.editor.content.Y + max(test.lineHeight, test.base+3)
		if bottomPoints[0].Points[0].Y != bottom+0.5 || bottomPoints[0].Points[0].Y-bottomPoints[0].Points[2].Y != test.pointHeight || topPoints[0].Points[2].Y-topPoints[0].Points[0].Y != test.pointHeight {
			t.Fatalf("point ignored font descent/minimum height: bottom=%+v top=%+v", bottomPoints[0], topPoints[0])
		}
	}
}
