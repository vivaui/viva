//go:build darwin || windows || linux

package headless

import (
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/render"
	"github.com/vivaui/viva/internal/scene"
	"github.com/vivaui/viva/text"
)

// scenery turns a snapshot into the renderer-facing types the window path uses.
func scenery(snapshot app.Snapshot) ([]render.Rectangle, []platform.TextLabel, *platform.TextInput) {
	rectangles := make([]render.Rectangle, len(snapshot.Primitives))
	for index, primitive := range snapshot.Primitives {
		rectangle := render.Rectangle{
			Kind:   scene.KindRectangle,
			Bounds: scene.Rect{X: primitive.Bounds.X, Y: primitive.Bounds.Y, Width: primitive.Bounds.Width, Height: primitive.Bounds.Height},
			Color:  scene.Color{Red: primitive.Color.Red, Green: primitive.Color.Green, Blue: primitive.Color.Blue, Alpha: primitive.Color.Alpha},
			Radius: primitive.Radius, Layer: primitive.Layer,
		}
		if primitive.Clipped {
			rectangle.Clipped = true
			rectangle.Clip = scene.Rect{X: primitive.Clip.X, Y: primitive.Clip.Y, Width: primitive.Clip.Width, Height: primitive.Clip.Height}
		}
		if primitive.Kind == app.ShadowPrimitive {
			rectangle.Kind = scene.KindShadow
			rectangle.Shadow = scene.Shadow(primitive.Shadow)
		}
		if primitive.Kind == app.TrianglePrimitive {
			rectangle.Kind = scene.KindTriangle
			rectangle.EdgeMask = primitive.EdgeMask
			if primitive.Shaded {
				rectangle.Shade = scene.Shade{On: true, Colors: [2]scene.Color{scene.Color(primitive.Shade[0]), scene.Color(primitive.Shade[1])}}
			}
			for point := range rectangle.Points {
				rectangle.Points[point] = scene.Point{X: primitive.Points[point].X, Y: primitive.Points[point].Y}
			}
		}
		rectangles[index] = rectangle
	}
	labels := make([]platform.TextLabel, len(snapshot.Labels))
	for index, label := range snapshot.Labels {
		labels[index] = platform.TextLabel{
			Value: label.Value, X: label.Bounds.X, Y: label.Bounds.Y, Width: label.Bounds.Width, Height: label.Bounds.Height,
			OriginX: label.OriginX, OriginY: label.OriginY,
			FontSize: label.FontSize, Weight: label.Weight, Layer: label.Layer,
			Mono: label.Mono, MonoFace: label.MonoFace, CellWidth: label.CellWidth, Runs: label.Runs,
			Clip: label.Clip, Clipped: label.Clipped,
			Bitmap: label.Bitmap, BitmapWidth: label.BitmapWidth, BitmapHeight: label.BitmapHeight,
			BitmapStride: label.BitmapStride, BitmapID: label.BitmapID, BitmapVersion: label.BitmapVersion,
			BitmapUpload: len(label.Bitmap) > 0,
			Color:        platform.Color{Red: float64(label.Color.Red), Green: float64(label.Color.Green), Blue: float64(label.Color.Blue), Alpha: float64(label.Color.Alpha)},
		}
	}
	var input *platform.TextInput
	if editor := snapshot.Editor; editor != nil {
		input = &platform.TextInput{
			X: editor.Bounds.X, Y: editor.Bounds.Y, Width: editor.Bounds.Width, Height: editor.Bounds.Height,
			FontSize: editor.FontSize, Layer: editor.Layer, Multiline: editor.Multiline, Secure: editor.Secure,
			Terminal: editor.Terminal, MonoFace: editor.MonoFace,
			Color: platform.Color{Red: float64(editor.Color.Red), Green: float64(editor.Color.Green), Blue: float64(editor.Color.Blue), Alpha: float64(editor.Color.Alpha)},
			State: platform.TextState{
				Value:       editor.Value,
				Selection:   text.Range{Start: editor.Selection[0], End: editor.Selection[1]},
				Composition: text.Range{Start: editor.Composition[0], End: editor.Composition[1]},
				Composing:   editor.Composing,
			},
			// Rendering never edits; the handler only has to exist.
			OnEvent: func(platform.TextEvent) (platform.TextState, error) { return platform.TextState{}, nil },
		}
	}
	return rectangles, labels, input
}
