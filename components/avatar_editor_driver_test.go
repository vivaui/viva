package components_test

import (
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

type avatarEditorDriverModel struct {
	options components.AvatarEditorOptions
	layout  components.AvatarEditorLayout
}

func (model *avatarEditorDriverModel) page(c ui.C) {
	ui.Slot(c).Fill().Draw(func(d *ui.DrawContext) {
		model.layout = components.BeginAvatarEditor(d.Canvas, *d.Theme, d.Rect, model.options).Paint()
		model.options.State = model.layout.State
	})
}

func TestAvatarEditorDriverInteraction(t *testing.T) {
	state := components.AvatarEditorState{Zoom: 2, CenterX: .5, CenterY: .5}
	ids := components.AvatarEditorIDs{
		Canvas: gfx.Item(4), Zoom: gfx.Item(5), ZoomOut: gfx.Item(6), ZoomIn: gfx.Item(7),
		Reset: gfx.Item(8),
	}
	pixels := make([]byte, 160*100*4)
	model := &avatarEditorDriverModel{options: components.AvatarEditorOptions{
		IDs: ids, State: state, AspectRatio: 1, Circular: true,
		Bitmap: gfx.Bitmap{Pix: pixels, Width: 160, Height: 100, Stride: 160 * 4, ID: 1, Version: 1},
		Labels: components.AvatarEditorLabels{
			Hint: "Drag and zoom", Canvas: "Avatar crop",
			Zoom: "Zoom", ZoomOut: "Zoom out", ZoomIn: "Zoom in", Reset: "Reset",
		},
	}}
	page := ui.NewPage(ui.PageConfig{View: model.page, Theme: themePointer(gfx.LightTheme())})
	driver, err := app.NewDriver(page.Config(app.Config{
		Width: 900, Height: 680,
		OnPointer: func(event gfx.PointerEvent) gfx.EventResult {
			state, result := model.layout.Pointer(event, model.options.State)
			model.options.State = state
			return result
		},
		OnScroll: func(event gfx.ScrollEvent) gfx.EventResult {
			state, result := model.layout.Scroll(event, model.options.State)
			model.options.State = state
			return result
		},
		OnKey: func(event gfx.KeyEvent) gfx.EventResult {
			state, result := model.layout.Key(event, model.options.State)
			model.options.State = state
			return result
		},
	}), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)

	canvasHit := driverAvatarHit(t, driver, ids.Canvas)
	x := canvasHit.Bounds.X + canvasHit.Bounds.Width/2
	y := canvasHit.Bounds.Y + canvasHit.Bounds.Height/2
	if err := driver.Press(x-60, y+25, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	if err := driver.Move(x+35, y-45); err != nil {
		t.Fatal(err)
	}
	if err := driver.Release(x+35, y-45, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	if model.options.State.CenterX == .5 || model.options.State.CenterY == .5 {
		t.Fatalf("driver drag did not update the crop: %+v", model.options.State)
	}

	beforeZoom := model.options.State.Zoom
	if err := driver.Scroll(gfx.ScrollEvent{X: x, Y: y, DeltaY: -1}); err != nil {
		t.Fatal(err)
	}
	if model.options.State.Zoom <= beforeZoom {
		t.Fatalf("driver scroll did not zoom in: %+v", model.options.State)
	}

	if err := driver.Key(gfx.KeyHome, 0); err != nil {
		t.Fatal(err)
	}
	if model.options.State.Zoom != 1 || model.options.State.CenterX != .5 || model.options.State.CenterY != .5 {
		t.Fatalf("driver key did not reset: %+v", model.options.State)
	}

	zoomIn := driverAvatarHit(t, driver, ids.ZoomIn)
	if err := driver.Click(zoomIn.Bounds.X+zoomIn.Bounds.Width/2, zoomIn.Bounds.Y+zoomIn.Bounds.Height/2); err != nil {
		t.Fatal(err)
	}
	if model.options.State.Zoom <= 1 {
		t.Fatalf("zoom button did not update state: %+v", model.options.State)
	}
	if model.layout.Crop.Width >= float32(model.options.Bitmap.Height) {
		t.Fatalf("zoom button returned an unzoomed crop: %+v", model.layout.Crop)
	}
}

func themePointer(theme gfx.Theme) *gfx.Theme { return &theme }

func driverAvatarHit(t *testing.T, driver *app.Driver, id gfx.ID) gfx.Hit {
	t.Helper()
	for _, hit := range driver.Snapshot().Hits {
		if hit.ID == id {
			return hit
		}
	}
	t.Fatalf("missing avatar editor hit %v", id)
	return gfx.Hit{}
}
