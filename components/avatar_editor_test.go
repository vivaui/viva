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
	state := AvatarEditorState{}
	InitializeAvatarEditorState(&state)
	return AvatarEditorOptions{
		IDs:         ids,
		Labels:      AvatarEditorLabels{Hint: "Drag and zoom", Canvas: "Avatar crop"},
		Bitmap:      avatarEditorTestBitmap(120, 80),
		State:       state,
		AspectRatio: 1,
		Circular:    true,
	}
}

func avatarEditorTestBounds() gfx.Rect {
	return gfx.Rect{X: 24, Y: 20, Width: 456, Height: 500}
}

func TestAvatarEditorCropMath(t *testing.T) {
	bitmap := avatarEditorTestBitmap(120, 80)
	state := AvatarEditorState{}
	InitializeAvatarEditorState(&state)
	crop := avatarEditorCrop(bitmap, state, 1, 1, 8)
	if !avatarEditorRectNear(crop, gfx.Rect{X: 20, Y: 0, Width: 80, Height: 80}) {
		t.Fatalf("initial square crop = %+v", crop)
	}

	state.Zoom = 2
	crop = avatarEditorCrop(bitmap, state, 1, 1, 8)
	if !avatarEditorRectNear(crop, gfx.Rect{X: 40, Y: 20, Width: 40, Height: 40}) {
		t.Fatalf("zoomed square crop = %+v", crop)
	}

	state.Zoom, state.CenterX, state.CenterY = 1, .99, -.5
	state = normalizeAvatarEditorCenter(state, gfx.Size{Width: 120, Height: 80}, 1, 1, 8)
	crop = avatarEditorCrop(bitmap, state, 1, 1, 8)
	if !avatarEditorRectNear(crop, gfx.Rect{X: 40, Y: 0, Width: 80, Height: 80}) {
		t.Fatalf("clamped crop = %+v", crop)
	}

	state = AvatarEditorState{}
	InitializeAvatarEditorState(&state)
	crop = avatarEditorCrop(bitmap, state, 16.0/9.0, 1, 8)
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
			options.State.Zoom = 2
			options.Backdrop = gfx.Color{Red: .12, Green: .23, Blue: .34, Alpha: 1}
			bounds := avatarEditorTestBounds()
			layout := BeginAvatarEditor(&canvas, theme, bounds, options).Paint()
			if !layout.Ready || layout.Stage.Width <= 0 || layout.Stage.Height <= 0 || layout.Crop.Width != layout.Crop.Height {
				t.Fatalf("scale %v: incomplete layout %+v", scale, layout)
			}
			if layout.Content != bounds || layout.Stage.X < bounds.X || layout.Stage.X+layout.Stage.Width > bounds.X+bounds.Width {
				t.Fatalf("scale %v: crop surface escaped its bounds: %+v", scale, layout)
			}
			if layout.ZoomTrack.X < bounds.X || layout.ZoomTrack.X+layout.ZoomTrack.Width > bounds.X+bounds.Width ||
				layout.ZoomTrack.Y+layout.ZoomTrack.Height > bounds.Y+bounds.Height {
				t.Fatalf("scale %v: zoom controls escaped their bounds: %+v", scale, layout.ZoomTrack)
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
			if !result.Handled || state.Gesture != avatarEditorGestureCanvas {
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
			if !result.Handled || state.Gesture != avatarEditorGestureNone {
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
		}
	}
}

func TestAvatarEditorLoadingAndAllocations(t *testing.T) {
	options := avatarEditorTestOptions()
	options.Loading = true
	options.Bitmap = gfx.Bitmap{}
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
	bounds := gfx.Rect{X: 7, Y: 11, Width: 120, Height: 180}
	var canvas gfx.Canvas
	layout := BeginAvatarEditor(&canvas, gfx.LightTheme(), bounds, options).Paint()
	if layout.Stage.X < bounds.X || layout.Stage.X+layout.Stage.Width > bounds.X+bounds.Width ||
		layout.Stage.Y+layout.Stage.Height > bounds.Y+bounds.Height {
		t.Fatalf("constrained stage escaped bounds: %+v", layout.Stage)
	}
	if layout.ZoomTrack.X < bounds.X || layout.ZoomTrack.X+layout.ZoomTrack.Width > bounds.X+bounds.Width ||
		layout.ZoomTrack.Y+layout.ZoomTrack.Height > bounds.Y+bounds.Height {
		t.Fatalf("constrained controls escaped bounds: %+v", layout.ZoomTrack)
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
