//go:build darwin || windows || linux

package ui

import (
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
)

// A FieldBase is a Field in everything but looks: a click focuses it, typed text reaches
// its editor, and Enter is Submitted, with the element's own frame around it.
func TestFieldBaseTakesTypedTextAndSubmitsThroughTheDriver(t *testing.T) {
	var cell Field
	var view View
	cellID := tree.ID(page, "cell-base")
	var submitted []string
	config := app.Config{
		Tree: &tree, Title: "cell", Width: 300, Height: 120,
		Paint: func(canvas *gfx.Canvas, size gfx.Size) {
			view.Draw(canvas, gfx.LightTheme(), gfx.Rect{Width: size.Width, Height: size.Height}, func(f *Frame) {
				f.Column().Fill().Pad(12).Children(func() {
					if f.FieldBase(&cell, cellID, "Cell", "").Pad(8).Border(2, gfx.LightTheme().Primary).Radius(0).Submitted() {
						submitted = append(submitted, cell.Text())
					}
				})
			})
		},
		TextInput: Fields(&cell),
	}
	var canvas gfx.Canvas
	driver, err := app.NewDriver(config, &canvas)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	var bounds gfx.Rect
	found := false
	for _, h := range driver.Snapshot().Hits {
		if h.ID == cellID {
			bounds, found = h.Bounds, true
		}
	}
	if !found {
		t.Fatal("the cell is not on screen")
	}
	if err := driver.Click(bounds.X+bounds.Width/2, bounds.Y+bounds.Height/2); err != nil {
		t.Fatal(err)
	}
	if err := driver.Type("Ada"); err != nil {
		t.Fatal(err)
	}
	if err := driver.Key(gfx.KeyEnter, 0); err != nil {
		t.Fatal(err)
	}
	if cell.Text() != "Ada" || len(submitted) != 1 || submitted[0] != "Ada" {
		t.Fatalf("text %q, submitted %q, want Ada once", cell.Text(), submitted)
	}
}

// A FieldBase set in a fixed-pitch face is painted by the component, since the platform's text
// service draws one proportional face: typing still reaches the editor through the platform, whose
// own text and overlays are hidden, and what is on screen is the glyphs in the mono face, the
// caret on a cell boundary and the selection over whole cells, from the editor's own state.
func TestMonoFieldBaseIsPaintedByTheComponentFromTheEditorsState(t *testing.T) {
	var field Field
	var view View
	id := tree.ID(page, "cell-mono")
	config := app.Config{
		Tree: &tree, Title: "mono", Width: 300, Height: 100,
		Paint: func(canvas *gfx.Canvas, size gfx.Size) {
			view.Draw(canvas, gfx.LightTheme(), gfx.Rect{Width: size.Width, Height: size.Height}, func(f *Frame) {
				f.Column().Pad(12).Children(func() {
					f.FieldBase(&field, id, "Type", "").Height(28).FontSize(11).Weight(gfx.WeightMedium).MonoFace(gfx.MonoClassic)
				})
			})
		},
		TextInput: Fields(&field),
	}
	var canvas gfx.Canvas
	driver, err := app.NewDriver(config, &canvas)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	var bounds gfx.Rect
	for _, hit := range driver.Snapshot().Hits {
		if hit.ID == id {
			bounds = hit.Bounds
		}
	}
	if err := driver.Click(bounds.X+bounds.Width/2, bounds.Y+bounds.Height/2); err != nil {
		t.Fatal(err)
	}
	if err := driver.Type("int4 not"); err != nil {
		t.Fatal(err)
	}
	if field.Text() != "int4 not" {
		t.Fatalf("typing reached the editor as %q", field.Text())
	}
	snapshot := driver.Snapshot()
	var line gfx.Text
	for _, text := range snapshot.Texts {
		if text.Value == "int4 not" {
			line = text
		}
	}
	if !line.Style.Mono || line.Style.MonoFace != gfx.MonoClassic || line.Style.Size != 11 || line.Style.Weight != gfx.WeightMedium {
		t.Fatalf("the typed text is painted in %+v, want 11 point medium classic mono by the component", line.Style)
	}
	var hit gfx.Hit
	for _, h := range snapshot.Hits {
		if h.ID == id {
			hit = h
		}
	}
	if !hit.HideNativeOverlays || hit.TextColor != (gfx.Color{}) || hit.TextSize != 11 || hit.TextWeight != gfx.WeightMedium {
		t.Errorf("the platform is told %+v", hit)
	}
	cell, _ := canvas.MonoCellSize(11, gfx.MonoClassic)
	caretX := func() (float32, bool) {
		for _, p := range driver.Snapshot().Primitives {
			if p.Bounds.Width == 1 && p.Bounds.Height > 1 && p.Bounds.X >= line.Bounds.X {
				return p.Bounds.X, true
			}
		}
		return 0, false
	}
	if x, ok := caretX(); !ok || x != line.Bounds.X+8*cell {
		t.Errorf("the caret is at x=%v (found %v), want eight cells after the text's start %v", x, ok, line.Bounds.X+8*cell)
	}
}
