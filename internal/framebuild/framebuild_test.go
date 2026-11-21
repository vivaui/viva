package framebuild

import (
	"reflect"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/scene"
)

func opaqueBitmap(id uint64, version uint32, width, height int) gfx.Bitmap {
	pixels := make([]byte, width*height*4)
	for index := 3; index < len(pixels); index += 4 {
		pixels[index] = 255
	}
	return gfx.Bitmap{Pix: pixels, Width: width, Height: height, Stride: width * 4, ID: id, Version: version}
}

func platformColor(color gfx.Color) platform.Color {
	return platform.Color{
		Red: float64(color.Red), Green: float64(color.Green),
		Blue: float64(color.Blue), Alpha: float64(color.Alpha),
	}
}

// TestBuilderCharacterizesTheFrame paints a deterministic scene and checks the
// frame the builder reconciles: shape geometry, colors and order, the clips and
// layers stamped onto them, and every field of the labels that carry text and
// bitmaps to the platform.
func TestBuilderCharacterizesTheFrame(t *testing.T) {
	sharp := gfx.Color{Red: 1, Alpha: 1}
	round := gfx.Color{Green: 1, Alpha: 1}
	shade := gfx.Color{Blue: 1, Alpha: 0.5}
	cut := gfx.Color{Red: 0.2, Green: 0.4, Blue: 0.6, Alpha: 1}
	layered := gfx.Color{Red: 0.9, Green: 0.8, Alpha: 1}
	quadTint := [4]gfx.Color{
		{Red: 1, Alpha: 1}, {Green: 1, Alpha: 1},
		{Blue: 1, Alpha: 1}, {Red: 1, Green: 1, Alpha: 1},
	}
	textColor := gfx.Color{Red: 0.5, Green: 0.5, Blue: 0.5, Alpha: 1}

	backdrop := opaqueBitmap(7, 1, 4, 4)
	picture := opaqueBitmap(9, 3, 8, 4)
	clip := gfx.Rect{X: 40, Y: 40, Width: 60, Height: 60}
	sharpBounds := gfx.Rect{Width: 30, Height: 30}
	roundBounds := gfx.Rect{X: 10, Y: 10, Width: 20, Height: 20}
	shadowBounds := gfx.Rect{X: 15, Y: 15, Width: 20, Height: 20}
	cutBounds := gfx.Rect{X: 50, Y: 50, Width: 100, Height: 100}
	layerBounds := gfx.Rect{X: 60, Y: 10, Width: 10, Height: 10}
	imageBounds := gfx.Rect{X: 70, Y: 10, Width: 16, Height: 8}
	proportional := gfx.Rect{X: 4, Y: 4, Width: 60, Height: 14}
	runsBounds := gfx.Rect{X: 4, Y: 100, Width: 56, Height: 12}
	layerText := gfx.Rect{X: 4, Y: 120, Width: 60, Height: 14}
	warpBounds := gfx.Rect{X: 4, Y: 140, Width: 60, Height: 14}
	warpQuad := gfx.Quad{
		Points: [4]gfx.Point{{X: 4, Y: 140}, {X: 64, Y: 144}, {X: 6, Y: 154}, {X: 62, Y: 156}},
		Tint:   quadTint,
	}
	runs := []gfx.TextRun{
		{Column: 0, Bytes: 1, Cells: 1, Color: textColor, Underline: true},
		{Column: 1, Bytes: 1, Cells: 1, Color: textColor, Strike: true},
	}

	var canvas gfx.Canvas
	canvas.DrawBackdrop(backdrop, gfx.Rect{X: 0, Y: 0, Width: 4, Height: 4}, gfx.Rect{X: 0, Y: 0, Width: 20, Height: 20})
	canvas.Fill(sharpBounds, sharp)
	canvas.Rounded(roundBounds, 8, round)
	canvas.Shadow(shadowBounds, 4, gfx.ShadowStyle{OffsetY: 2, Blur: 6}, shade)
	canvas.Label("proportional", proportional, gfx.TextStyle{Size: 12, Weight: 400, Color: textColor})
	canvas.Clip(clip)
	canvas.Fill(cutBounds, cut)
	canvas.Label("cut label", gfx.Rect{X: 30, Y: 40, Width: 60, Height: 12}, gfx.TextStyle{Size: 12, Color: textColor})
	canvas.FillQuad(gfx.Quad{
		Points: [4]gfx.Point{{X: 30, Y: 30}, {X: 70, Y: 30}, {X: 30, Y: 70}, {X: 70, Y: 70}},
		Tint:   quadTint,
	})
	canvas.Unclip()
	canvas.LabelRuns("ab", runsBounds, gfx.TextStyle{Size: 10, Mono: true, Color: textColor}, 7, runs)
	canvas.PushLayer(5)
	canvas.Fill(layerBounds, layered)
	canvas.DrawBitmapShaded(picture, gfx.Rect{Width: 8, Height: 4}, imageBounds, 3, 0.25)
	canvas.Label("layered", layerText, gfx.TextStyle{Size: 12, Color: textColor})
	canvas.PopLayer()
	canvas.LabelQuad("warped", warpBounds, warpQuad, gfx.TextStyle{Size: 12, Color: textColor})

	canvasTriangles := append([]gfx.Triangle(nil), canvas.Triangles()...)
	if len(canvasTriangles) != 2 {
		t.Fatalf("canvas painted %d triangles, want 2", len(canvasTriangles))
	}

	builder := New(&canvas)
	frame, err := builder.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	// Shapes: the backdrop shifts every canvas layer one up, taking platform
	// layer zero; the topmost layer stays topmost.
	instances := frame.Rectangles
	wantKinds := []scene.Kind{
		scene.KindRectangle, scene.KindRectangle, scene.KindShadow,
		scene.KindRectangle, scene.KindTriangle, scene.KindTriangle, scene.KindRectangle,
	}
	if len(instances) != len(wantKinds) {
		t.Fatalf("built %d instances, want %d: %+v", len(instances), len(wantKinds), instances)
	}
	for index, want := range wantKinds {
		if instances[index].Kind != want {
			t.Fatalf("instance %d kind = %d, want %d: %+v", index, instances[index].Kind, want, instances)
		}
	}
	if got := instances[0]; got.Bounds != scene.Rect(sharpBounds) || got.Color != scene.Color(sharp) || got.Radius != 0 {
		t.Errorf("sharp rectangle = %+v", got)
	}
	if got := instances[1]; got.Bounds != scene.Rect(roundBounds) || got.Color != scene.Color(round) || got.Radius != 8 {
		t.Errorf("rounded rectangle = %+v", got)
	}
	if got := instances[2]; got.Bounds != scene.Rect(shadowBounds) || got.Color != scene.Color(shade) ||
		got.Radius != 4 || got.Shadow != (scene.Shadow{OffsetY: 2, Blur: 6}) {
		t.Errorf("shadow = %+v", got)
	}
	if got := instances[3]; got.Bounds != scene.Rect(cutBounds) || !got.Clipped || got.Clip != scene.Rect(clip) {
		t.Errorf("clipped rectangle = %+v, want its full bounds and the clip", got)
	}
	for index := range canvasTriangles {
		got := instances[4+index]
		want := SceneTriangle(canvasTriangles[index])
		if got.Points != want.Points || got.Color != want.Color || got.EdgeMask != want.EdgeMask {
			t.Errorf("triangle %d = %+v, want %+v", index, got, want)
		}
		if got.EdgeMask == 0 || !got.Shade.On {
			t.Errorf("triangle %d lost its edge mask or shading: %+v", index, got)
		}
		if !got.Clipped || got.Clip != scene.Rect(clip) {
			t.Errorf("triangle %d did not keep its clip: %+v", index, got)
		}
	}
	if got := instances[6]; got.Bounds != scene.Rect(layerBounds) || got.Color != scene.Color(layered) || got.Layer != 6 {
		t.Errorf("layered rectangle = %+v, want platform layer 6", got)
	}
	for index, instance := range instances[:6] {
		if instance.Layer != 1 {
			t.Errorf("instance %d layer = %d, want 1 above the backdrop", index, instance.Layer)
		}
	}

	// Labels: the backdrop first, then images and text merged by paint order.
	labels := builder.Labels()
	if len(labels) != 7 {
		t.Fatalf("labels = %d, want 7: %+v", len(labels), labels)
	}
	if got := labels[0]; got.BitmapID != backdrop.ID || !got.BitmapUpload || got.Layer != 0 || got.ShapesBefore != 0 ||
		got.BitmapWidth != 4 || got.BitmapHeight != 4 || got.BitmapStride != 16 {
		t.Errorf("backdrop label = %+v", got)
	}
	if got := labels[1]; got.Value != "proportional" || got.Layer != 1 || got.ShapesBefore != 3 ||
		got.FontSize != 12 || got.Weight != 400 || got.Color != platformColor(textColor) || got.Mono {
		t.Errorf("proportional label = %+v", got)
	}
	if got := labels[2]; got.Value != "cut label" || got.Layer != 1 || got.ShapesBefore != 4 ||
		!got.Clipped || got.Clip != clip {
		t.Errorf("clipped label = %+v, want its clip and the shapes before it", got)
	}
	if got := labels[3]; got.Value != "ab" || !got.Mono || got.CellWidth != 7 || got.Layer != 1 || got.ShapesBefore != 6 {
		t.Errorf("runs label = %+v", got)
	} else if len(got.Runs) != 2 || !got.Runs[0].Underline || !got.Runs[1].Strike {
		t.Errorf("runs label lost its decorations: %+v", got.Runs)
	}
	if got := labels[4]; got.BitmapID != picture.ID || got.BitmapVersion != picture.Version || got.Layer != 6 ||
		got.ShapesBefore != 7 || got.BitmapWidth != 8 || got.BitmapHeight != 4 || !got.BitmapUpload ||
		got.ImageRadius != 3 || got.ImageFade != 0.25 {
		t.Errorf("image label = %+v", got)
	}
	if got := labels[5]; got.Value != "layered" || got.Layer != 6 || got.ShapesBefore != 7 {
		t.Errorf("layered label = %+v", got)
	}
	if got := labels[6]; got.Value != "warped" || !got.HasQuad || got.Quad != warpQuad.Points || got.Layer != 1 || got.ShapesBefore != 7 {
		t.Errorf("warped label = %+v", got)
	} else {
		for corner, tint := range warpQuad.Tint {
			if got.Tint[corner] != platformColor(tint) {
				t.Errorf("warped tint %d = %+v, want %+v", corner, got.Tint[corner], platformColor(tint))
			}
		}
	}

	// An unchanged repaint sends no labels, but still describes every instance.
	again, err := builder.Build()
	if err != nil {
		t.Fatalf("unchanged Build: %v", err)
	}
	if again.TextLabels != nil {
		t.Fatalf("unchanged repaint sent labels: %+v", again.TextLabels)
	}
	if !again.Update.Empty() {
		t.Fatalf("unchanged repaint produced renderer work: %+v", again.Update)
	}
	if len(again.Rectangles) != len(wantKinds) {
		t.Fatalf("unchanged repaint returned %d instances, want %d", len(again.Rectangles), len(wantKinds))
	}
	if !reflect.DeepEqual(again.Rectangles, frame.Rectangles) {
		t.Fatalf("unchanged repaint changed the instances")
	}
}

// TestBuilderRestagesOnlyChangedBitmaps checks that a changed tile carries its
// pixels again while an unchanged frame does not.
func TestBuilderRestagesOnlyChangedBitmaps(t *testing.T) {
	bitmap := opaqueBitmap(11, 1, 4, 4)
	tile := gfx.Rect{X: 4, Y: 4, Width: 8, Height: 8}
	paint := func(canvas *gfx.Canvas) {
		canvas.DrawBitmap(bitmap, tile)
	}
	uploads := func(labels []platform.TextLabel) (count int) {
		for _, label := range labels {
			if label.BitmapUpload {
				count++
			}
		}
		return count
	}

	var canvas gfx.Canvas
	paint(&canvas)
	builder := New(&canvas)
	if _, err := builder.Build(); err != nil {
		t.Fatalf("Build: %v", err)
	}
	if got := uploads(builder.Labels()); got != 1 {
		t.Fatalf("first frame uploaded %d bitmaps, want 1", got)
	}

	if frame, err := builder.Build(); err != nil {
		t.Fatalf("unchanged Build: %v", err)
	} else if frame.TextLabels != nil {
		t.Fatalf("unchanged frame sent labels: %+v", frame.TextLabels)
	}

	bitmap.Version++
	canvas.Reset()
	paint(&canvas)
	frame, err := builder.Build()
	if err != nil {
		t.Fatalf("changed Build: %v", err)
	}
	if frame.TextLabels == nil || len(frame.TextLabels) != 1 {
		t.Fatalf("changed bitmap sent no labels: %+v", frame.TextLabels)
	}
	if !frame.TextLabels[0].BitmapUpload || frame.TextLabels[0].BitmapVersion != 2 {
		t.Fatalf("changed bitmap label = %+v, want its pixels and version 2", frame.TextLabels[0])
	}
}

// The memo hands a line that arrives unchanged back as it replaced it, and a
// line whose runs changed — a recolour keeps the value — gets a replacement
// built for the new runs, not the remembered one.
func TestReplaceCellGlyphsMemoFollowsItsInput(t *testing.T) {
	builder := New(&gfx.Canvas{})
	value := "┌──┐ ok"
	runs := []gfx.TextRun{{Bytes: int32(len(value)), Cells: 6, Color: gfx.Color{Alpha: 1}}}
	firstValue, firstRuns := builder.replaceCellGlyphs(value, runs)
	if firstValue == value {
		t.Fatalf("a line with cell glyphs came back unchanged: %q", firstValue)
	}
	againValue, againRuns := builder.replaceCellGlyphs(value, runs)
	if againValue != firstValue || &againRuns[0] != &firstRuns[0] {
		t.Fatal("the same line was replaced again instead of remembered")
	}

	recolored := []gfx.TextRun{{Bytes: int32(len(value)), Cells: 6, Color: gfx.Color{Red: 1, Alpha: 1}}}
	recValue, recRuns := builder.replaceCellGlyphs(value, recolored)
	if recRuns[0].Color != recolored[0].Color {
		t.Fatalf("a recoloured line got the remembered colour %+v", recRuns[0].Color)
	}
	if recValue != firstValue {
		t.Fatalf("the replacement text changed with the colour: %q and %q", recValue, firstValue)
	}

	plain := "plain line"
	plainRuns := []gfx.TextRun{{Bytes: int32(len(plain)), Cells: 10, Color: gfx.Color{Alpha: 1}}}
	outValue, outRuns := builder.replaceCellGlyphs(plain, plainRuns)
	if outValue != plain || &outRuns[0] != &plainRuns[0] {
		t.Fatal("a line without cell glyphs was copied or remembered")
	}
}
