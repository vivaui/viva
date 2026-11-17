package components

import (
	"math"
	"testing"

	"github.com/vivaui/viva/gfx"
)

func avatarEditorTestBitmap(width, height int) gfx.Bitmap {
	pixels := make([]byte, width*height*4)
	for i := range pixels {
		pixels[i] = byte(i*31 + 17)
	}
	return gfx.Bitmap{Pix: pixels, Width: width, Height: height, Stride: width * 4, ID: 91, Version: 2}
}

func avatarEditorTestOptions() AvatarEditorOptions {
	ids := AvatarEditorIDs{
		Canvas: gfx.Item(4), Zoom: gfx.Item(5), ZoomOut: gfx.Item(6), ZoomIn: gfx.Item(7),
		Reset: gfx.Item(8),
	}
	return AvatarEditorOptions{
		IDs:         ids,
		Labels:      AvatarEditorLabels{Hint: "Drag and zoom", Canvas: "Avatar crop"},
		Bitmap:      avatarEditorTestBitmap(120, 80),
		AspectRatio: 1,
		Circular:    true,
	}
}

func avatarEditorTestBounds() gfx.Rect {
	return gfx.Rect{X: 24, Y: 20, Width: 456, Height: 500}
}

func TestAvatarEditorCropMath(t *testing.T) {
	bitmap := avatarEditorTestBitmap(120, 80)
	geometry := newAvatarCropGeometry(bitmap, 1, 1, 8)
	state := geometry.normalize(AvatarEditorState{})
	crop := geometry.crop(state)
	if !avatarEditorRectNear(crop, gfx.Rect{X: 20, Y: 0, Width: 80, Height: 80}) {
		t.Fatalf("initial square crop = %+v", crop)
	}

	state.Zoom = 2
	crop = geometry.crop(state)
	if !avatarEditorRectNear(crop, gfx.Rect{X: 40, Y: 20, Width: 40, Height: 40}) {
		t.Fatalf("zoomed square crop = %+v", crop)
	}

	state.Zoom, state.CenterX, state.CenterY = 1, .99, -.5
	crop = geometry.crop(geometry.normalize(state))
	if !avatarEditorRectNear(crop, gfx.Rect{X: 40, Y: 0, Width: 80, Height: 80}) {
		t.Fatalf("clamped crop = %+v", crop)
	}

	geometry = newAvatarCropGeometry(bitmap, 16.0/9.0, 1, 8)
	crop = geometry.crop(geometry.normalize(AvatarEditorState{}))
	if math.Abs(float64(crop.Width/crop.Height-16.0/9.0)) > 0.0001 ||
		crop.X < 0 || crop.Y < 0 || crop.X+crop.Width > 120.001 || crop.Y+crop.Height > 80.001 {
		t.Fatalf("widescreen crop = %+v", crop)
	}
}

func TestAvatarEditorPaintAndInput(t *testing.T) {
	for _, theme := range []gfx.Theme{gfx.LightTheme(), gfx.DarkTheme()} {
		for _, scale := range []float32{1, 1.25, 1.5, 2} {
			var canvas gfx.Canvas
			canvas.SetPixelScale(scale)
			options := avatarEditorTestOptions()
			options.IDs.Reset = gfx.ID{}
			options.State = AvatarEditorState{Zoom: 2, CenterX: .5, CenterY: .5}
			options.Backdrop = gfx.Color{Red: .12, Green: .23, Blue: .34, Alpha: 1}
			bounds := avatarEditorTestBounds()
			layout := BeginAvatarEditor(&canvas, theme, bounds, options).Paint()
			if !layout.Ready || layout.Stage.Width <= 0 || layout.Stage.Height <= 0 || layout.Crop.Width != layout.Crop.Height {
				t.Fatalf("scale %v: incomplete layout %+v", scale, layout)
			}
			if layout.Stage.X < bounds.X || layout.Stage.X+layout.Stage.Width > bounds.X+bounds.Width {
				t.Fatalf("scale %v: crop surface escaped its bounds: %+v", scale, layout)
			}
			if !canvas.TrappedFocusScope().IsZero() || !canvas.EscapeTarget().IsZero() {
				t.Fatal("crop surface installed dialog routing")
			}
			if texts := canvas.Texts(); len(texts) != 0 {
				t.Fatalf("scale %v: crop surface painted dialog chrome text: %+v", scale, texts)
			}
			backdropFound := false
			for _, triangle := range canvas.Triangles() {
				backdropFound = backdropFound || triangle.Color == options.Backdrop
			}
			if !backdropFound {
				t.Fatalf("scale %v: circular mask did not use the caller's backdrop", scale)
			}
			if images := canvas.Images(); len(images) != 1 {
				t.Fatalf("scale %v: circular preview used %d bitmap draws, want 1", scale, len(images))
			}
			canvasHits, zoomHits := 0, 0
			for _, hit := range canvas.Hits() {
				switch hit.ID {
				case options.IDs.Canvas:
					canvasHits++
					if hit.Cursor != gfx.PointerCursorHand {
						t.Fatalf("crop cursor = %v", hit.Cursor)
					}
				case options.IDs.Zoom:
					zoomHits++
					if hit.Role != gfx.RoleSlider || !hit.Range.Set {
						t.Fatalf("zoom hit = %+v", hit)
					}
				}
			}
			if canvasHits != 1 || zoomHits != 1 {
				t.Fatalf("scale %v: crop hits %d, zoom hits %d", scale, canvasHits, zoomHits)
			}

			state := layout.State
			startX := layout.Stage.X + layout.Stage.Width/2
			startY := layout.Stage.Y + layout.Stage.Height/2
			state, result := layout.Pointer(gfx.PointerEvent{
				Kind: gfx.PointerDown, Button: gfx.PointerButtonPrimary,
				Target: options.IDs.Canvas, X: startX, Y: startY,
			}, state)
			if !result.Handled {
				t.Fatal("crop drag did not begin")
			}
			state, result = layout.Pointer(gfx.PointerEvent{
				Kind: gfx.PointerMove, Button: gfx.PointerButtonPrimary, Captured: true,
				Target: options.IDs.Canvas, X: startX - 80, Y: startY + 30,
			}, state)
			if !result.Handled || state.CenterX <= .5 || state.CenterY >= .5 {
				t.Fatalf("crop drag state = %+v", state)
			}
			state, result = layout.Pointer(gfx.PointerEvent{
				Kind: gfx.PointerUp, Button: gfx.PointerButtonPrimary,
				Target: options.IDs.Canvas, X: startX - 80, Y: startY + 30,
			}, state)
			if !result.Handled || state.gesture != avatarEditorGestureNone {
				t.Fatal("crop drag did not end")
			}

			beforeZoom := state.Zoom
			state, result = layout.Scroll(gfx.ScrollEvent{
				Target: options.IDs.Canvas, X: startX + 40, Y: startY - 20, DeltaY: -1,
			}, state)
			if !result.Handled || state.Zoom <= beforeZoom {
				t.Fatalf("wheel zoom = %+v, %+v", state, result)
			}

			beforeX := state.CenterX
			state, result = layout.Key(gfx.KeyEvent{Target: options.IDs.Canvas, Key: gfx.KeyLeft}, state)
			if !result.Handled || state.CenterX >= beforeX {
				t.Fatalf("keyboard pan = %+v, %+v", state, result)
			}
			state, result = layout.Key(gfx.KeyEvent{Target: options.IDs.Canvas, Key: gfx.KeyHome}, state)
			if !result.Handled || state.Zoom != 1 || state.CenterX != .5 || state.CenterY != .5 {
				t.Fatalf("keyboard reset = %+v", state)
			}
			if _, result = layout.Key(gfx.KeyEvent{Target: options.IDs.Canvas, Key: gfx.KeyEnter}, state); result.Handled {
				t.Fatal("crop input claimed the application's submit key")
			}
			if _, result = layout.Key(gfx.KeyEvent{Target: options.IDs.Canvas, Key: gfx.KeyLeft, Modifiers: gfx.ModifierControl}, state); result.Handled {
				t.Fatal("crop input claimed the application's keyboard shortcut")
			}
		}
	}
}

func TestAvatarEditorLoadingAndAllocations(t *testing.T) {
	options := avatarEditorTestOptions()
	options.Loading = true
	var canvas gfx.Canvas
	paint := func() {
		canvas.Reset()
		BeginAvatarEditor(&canvas, gfx.LightTheme(), avatarEditorTestBounds(), options).Paint()
	}
	paint()
	for _, hit := range canvas.Hits() {
		if hit.ID == options.IDs.Canvas && !hit.Disabled || hit.ID == options.IDs.Zoom && !hit.Disabled {
			t.Fatalf("loading editor enabled %+v", hit)
		}
	}
	if allocations := testing.AllocsPerRun(100, paint); allocations != 0 {
		t.Fatalf("editor paint allocations = %v", allocations)
	}

	options = avatarEditorTestOptions()
	paint = func() {
		canvas.Reset()
		BeginAvatarEditor(&canvas, gfx.LightTheme(), avatarEditorTestBounds(), options).Paint()
	}
	paint()
	if allocations := testing.AllocsPerRun(100, paint); allocations != 0 {
		t.Fatalf("ready editor paint allocations = %v", allocations)
	}
}

func TestAvatarEditorFitsConstrainedBounds(t *testing.T) {
	options := avatarEditorTestOptions()
	options.MaxStageSize = 100
	var canvas gfx.Canvas
	for _, bounds := range []gfx.Rect{
		{X: 7, Y: 11, Width: 120, Height: 180},
		{X: 7, Y: 11, Width: 60, Height: 100},
		{X: 7, Y: 11, Width: 12, Height: 32},
		{X: 7, Y: 11, Width: 120, Height: 6},
	} {
		canvas.Reset()
		layout := BeginAvatarEditor(&canvas, gfx.LightTheme(), bounds, options).Paint()
		if !layout.Ready || layout.Stage.Width > 100 || layout.Stage.Height > 100 {
			t.Fatalf("bounds %+v: invalid crop layout %+v", bounds, layout)
		}
		slider := false
		for _, hit := range canvas.Hits() {
			if hit.Bounds.X < bounds.X || hit.Bounds.X+hit.Bounds.Width > bounds.X+bounds.Width ||
				hit.Bounds.Y < bounds.Y || hit.Bounds.Y+hit.Bounds.Height > bounds.Y+bounds.Height {
				t.Fatalf("bounds %+v: hit escaped its container: %+v", bounds, hit)
			}
			slider = slider || hit.ID == options.IDs.Zoom
		}
		if slider != (bounds.Width >= 60 && bounds.Height >= 100) {
			t.Fatalf("bounds %+v: slider present = %v", bounds, slider)
		}
	}
}

func TestAvatarEditorZoomLimitsAndStatus(t *testing.T) {
	options := avatarEditorTestOptions()
	options.IDs.Reset = gfx.ID{}
	options.MinZoom = .5
	options.MaxZoom = float32(math.Inf(1))
	options.AspectRatio = float32(math.NaN())
	var canvas gfx.Canvas
	paint := func() AvatarEditorLayout {
		canvas.Reset()
		return BeginAvatarEditor(&canvas, gfx.LightTheme(), avatarEditorTestBounds(), options).Paint()
	}
	layout := paint()
	if !layout.Ready || layout.State.Zoom != 1 || !avatarEditorRectNear(layout.Crop, gfx.Rect{X: 20, Width: 80, Height: 80}) {
		t.Fatalf("invalid limits must fall back to a crop inside the source: %+v", layout)
	}
	options.MinZoom, options.MaxZoom = 2, 2
	layout = paint()
	if !layout.Ready || layout.State.Zoom != 2 || layout.Crop.Width != 40 {
		t.Fatalf("fixed zoom = %+v", layout)
	}
	for _, hit := range canvas.Hits() {
		if hit.ID == options.IDs.Zoom && !hit.Disabled {
			t.Fatal("fixed zoom slider can receive focus or input")
		}
	}
	_, result := layout.Scroll(gfx.ScrollEvent{Target: options.IDs.Canvas, DeltaY: -1}, layout.State)
	if result.Handled {
		t.Fatal("fixed zoom stole the containing page's wheel input")
	}
	options.Error = "Cannot open the new image"
	options.Loading = true
	layout = paint()
	if layout.Ready || layout.Crop != (gfx.Rect{}) || len(canvas.Images()) != 0 {
		t.Fatal("the loading or error state offered the previous image as a crop")
	}
	if texts := canvas.Texts(); len(texts) != 1 || texts[0].Value != options.Error {
		t.Fatalf("error must be shown before the loading message: %+v", texts)
	}
}

func TestAvatarEditorPaintsTheCropReturnedAfterZoomClick(t *testing.T) {
	var canvas gfx.Canvas
	options := avatarEditorTestOptions()
	canvas.BeginEventPass(options.IDs.ZoomIn, gfx.ClickInfo{})
	defer canvas.EndEventPass()
	layout := BeginAvatarEditor(&canvas, gfx.LightTheme(), avatarEditorTestBounds(), options).Paint()
	images := canvas.Images()
	if !layout.Ready || layout.State.Zoom <= 1 || len(images) != 1 || !avatarEditorRectNear(images[0].Source, layout.Crop) {
		t.Fatalf("preview and returned source crop disagree after a click: %+v", layout)
	}
}

func BenchmarkAvatarEditor(b *testing.B) {
	for _, circular := range []bool{false, true} {
		name := "square"
		if circular {
			name = "circular"
		}
		b.Run(name, func(b *testing.B) {
			options := avatarEditorTestOptions()
			options.Circular = circular
			var canvas gfx.Canvas
			paint := func() {
				canvas.Reset()
				BeginAvatarEditor(&canvas, gfx.LightTheme(), avatarEditorTestBounds(), options).Paint()
			}
			paint()
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				paint()
			}
		})
	}
}

func avatarEditorRectNear(a, b gfx.Rect) bool {
	const epsilon = .0001
	return math.Abs(float64(a.X-b.X)) < epsilon &&
		math.Abs(float64(a.Y-b.Y)) < epsilon &&
		math.Abs(float64(a.Width-b.Width)) < epsilon &&
		math.Abs(float64(a.Height-b.Height)) < epsilon
}
