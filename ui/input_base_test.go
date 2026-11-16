package ui

import (
	"reflect"
	"testing"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
)

var (
	baseField = tree.ID(page, "base-field")
	baseOther = tree.ID(page, "base-other")
)

func baseSnapshot(id gfx.ID) components.InputSnapshot {
	return components.InputSnapshot{ID: id, Label: "Cell", Value: "Gopher", Invalid: true}
}

// An InputBase is dressed by its element and by nothing else: the shadcn surface, with
// its destructive border for an invalid field, is not painted, and the element's own
// fill and border are.
func TestInputBaseHasTheLookOfItsElementAndNotShadcns(t *testing.T) {
	theme := gfx.LightTheme()
	fill, line := gfx.Color{Red: .9, Green: .9, Blue: .2, Alpha: 1}, gfx.Color{Red: .1, Green: .2, Blue: .8, Alpha: 1}
	var view View
	canvas := testCanvas()
	view.Draw(canvas, theme, gfx.Rect{Width: 200, Height: 60}, func(f *Frame) {
		f.InputBase(baseSnapshot(baseField)).Background(fill).Border(2, line).Radius(0)
	})
	for _, rect := range canvas.Rectangles() {
		if rect.Color == theme.Danger || rect.Color == theme.Input {
			t.Fatalf("the shadcn surface was painted: %+v", rect)
		}
	}
	if len(canvas.Rectangles()) == 0 {
		t.Fatal("the element painted nothing")
	}
}

// The text sits inside the element's Pad and Border, whatever they are, and the pointer
// hits the whole element: the invariants a cell editor relies on so its value does not
// move when it opens.
func TestInputBaseTextSitsInsideThePadAndBorderAndTheHitIsTheWholeElement(t *testing.T) {
	for _, tc := range []struct {
		name   string
		pad    float32
		border float32
	}{{"bare", 0, 0}, {"padded", 8, 0}, {"framed", 8, 2}, {"wide", 12, 3}} {
		t.Run(tc.name, func(t *testing.T) {
			var view View
			canvas := testCanvas()
			var box Box
			view.Draw(canvas, gfx.LightTheme(), gfx.Rect{Width: 240, Height: 80}, func(f *Frame) {
				box = f.InputBase(baseSnapshot(baseField)).Pad(tc.pad).Border(tc.border, gfx.Color{Alpha: 1}).Height(40)
			})
			rect := box.Rect()
			hits := canvas.Hits()
			if len(hits) != 1 {
				t.Fatalf("hits = %+v", hits)
			}
			hit := hits[0]
			inset := tc.pad + tc.border
			if hit.Bounds != rect {
				t.Errorf("hit bounds = %+v, want the whole element %+v", hit.Bounds, rect)
			}
			content := gfx.Rect{X: rect.X + inset, Y: rect.Y + inset, Width: rect.Width - 2*inset, Height: rect.Height - 2*inset}
			// The platform is told the line the text is drawn on, which is centered in the
			// padded rectangle and never leaves it.
			text := hitText(canvas, &hit).Bounds
			if text.X != content.X || text.Width != content.Width {
				t.Errorf("text bounds %+v are not the padded rectangle %+v across", text, content)
			}
			if text.Y < content.Y || text.Y+text.Height > content.Y+content.Height {
				t.Errorf("text bounds %+v leave the padded rectangle %+v", text, content)
			}
			if above, below := text.Y-content.Y, content.Y+content.Height-text.Y-text.Height; above != below {
				t.Errorf("text bounds %+v are not centered in %+v: %v above, %v below", text, content, above, below)
			}
		})
	}
}

// Without a Height the element is a line of text and its Pad and Border tall, so a
// padded field never clips its own text.
func TestInputBaseDefaultHeightGrowsWithPadAndBorder(t *testing.T) {
	height := func(pad, border float32) float32 {
		var view View
		var box Box
		view.Draw(testCanvas(), gfx.LightTheme(), gfx.Rect{Width: 240, Height: 200}, func(f *Frame) {
			f.Row().Items(Start).Children(func() {
				box = f.InputBase(baseSnapshot(baseField)).Pad(pad).Border(border, gfx.Color{Alpha: 1}).Width(100)
			})
		})
		return box.Rect().Height
	}
	bare := height(0, 0)
	if bare <= 0 {
		t.Fatalf("a bare field is %v tall", bare)
	}
	if got := height(8, 2); got != bare+2*(8+2) {
		t.Errorf("height with a pad of 8 and a border of 2 = %v, want the bare %v plus 20", got, bare)
	}
}

// FontSize and Color style the text, and nothing else of the theme moves with them.
func TestInputBaseFontSizeAndColorStyleTheText(t *testing.T) {
	theme := gfx.LightTheme()
	ink := gfx.Color{Red: .7, Green: .1, Blue: .1, Alpha: 1}
	var view View
	canvas := testCanvas()
	view.Draw(canvas, theme, gfx.Rect{Width: 240, Height: 80}, func(f *Frame) {
		f.InputBase(baseSnapshot(baseField)).FontSize(20).Color(ink)
	})
	hits := canvas.Hits()
	hit := hits[0]
	text := hitText(canvas, &hit)
	if text.Size != 20 {
		t.Errorf("text size = %v, want 20", text.Size)
	}
	if text.Color != ink {
		t.Errorf("text color = %+v, want %+v", text.Color, ink)
	}
	found := false
	for _, text := range canvas.Texts() {
		if text.Value == "Gopher" {
			found = text.Style.Size == 20 && text.Style.Color == ink
		}
	}
	if !found {
		t.Errorf("texts = %+v, want the value drawn at 20 in the ink color", canvas.Texts())
	}
}

// The looks of an element follow the field's identity, so the focused field shows its
// Focus look and the other does not: the state a custom frame is made of.
func TestInputBaseFocusLookFollowsTheField(t *testing.T) {
	idle, active := gfx.Color{Red: .5, Green: .5, Blue: .5, Alpha: 1}, gfx.Color{Red: .1, Green: .3, Blue: .9, Alpha: 1}
	draw := func(focus gfx.ID) *gfx.Canvas {
		var view View
		canvas := testCanvas()
		canvas.SetInteraction(gfx.ID{}, gfx.ID{}, focus, true)
		view.Draw(canvas, gfx.LightTheme(), gfx.Rect{Width: 240, Height: 120}, func(f *Frame) {
			f.Column().Gap(8).Children(func() {
				for _, id := range []gfx.ID{baseField, baseOther} {
					f.InputBase(baseSnapshot(id)).Height(32).Background(idle).Focus(Look{}.Background(active))
				}
			})
		})
		return canvas
	}
	count := func(c *gfx.Canvas, color gfx.Color) (n int) {
		for _, rect := range c.Rectangles() {
			if rect.Color == color {
				n++
			}
		}
		return n
	}
	none, first := draw(gfx.ID{}), draw(baseField)
	if count(none, active) != 0 {
		t.Errorf("an unfocused page shows the focus look")
	}
	if count(first, active) == 0 {
		t.Errorf("the focused field does not show its focus look: %+v", first.Rectangles())
	}
	if count(first, idle) == 0 {
		t.Errorf("the other field lost its resting fill: %+v", first.Rectangles())
	}
}

func TestInputBaseIsAllocationFreeAfterWarmup(t *testing.T) {
	var view View
	canvas := testCanvas()
	draw := func() {
		canvas.Reset()
		view.Draw(canvas, gfx.LightTheme(), gfx.Rect{Width: 240, Height: 80}, func(f *Frame) {
			f.InputBase(baseSnapshot(baseField)).Pad(8).Border(2, gfx.Color{Alpha: 1}).Radius(0)
		})
	}
	draw()
	if allocs := testing.AllocsPerRun(100, draw); allocs != 0 {
		t.Fatalf("allocs = %v, want 0", allocs)
	}
}

func BenchmarkInputBase(b *testing.B) {
	var view View
	canvas := testCanvas()
	b.ReportAllocs()
	for range b.N {
		canvas.Reset()
		view.Draw(canvas, gfx.LightTheme(), gfx.Rect{Width: 240, Height: 80}, func(f *Frame) {
			f.InputBase(baseSnapshot(baseField)).Pad(8).Border(2, gfx.Color{Alpha: 1}).Radius(0)
		})
	}
}

// An Input is an InputBase dressed by the recipe, and nothing more: for every theme and state
// it paints what components.PaintInput paints, shape for shape, and declares the same hit.
// This is what lets the shadcn field be restyled with the calls any element takes without a
// second copy of its look.
func TestInputIsTheBaseDressedByTheRecipeAndPaintsAsTheComponentDoes(t *testing.T) {
	themes := map[string]gfx.Theme{"light": gfx.LightTheme(), "dark": gfx.DarkTheme()}
	translucent := gfx.LightTheme()
	translucent.Background.Alpha = .1
	themes["translucent"] = translucent
	snapshots := map[string]components.InputSnapshot{
		"rest":        {ID: baseField, Label: "Name", Value: "Gopher"},
		"empty":       {ID: baseField, Label: "Name", Placeholder: "Your name"},
		"focused":     {ID: baseField, Label: "Name", Value: "Gopher", Focused: true, ShowCaret: true, Caret: 3, SelectionStart: 1, SelectionEnd: 3},
		"invalid":     {ID: baseField, Label: "Name", Value: "Gopher", Invalid: true},
		"disabled":    {ID: baseField, Label: "Name", Value: "Gopher", Disabled: true},
		"secure":      {ID: baseField, Label: "Name", Value: "hunter2", Secure: true},
		"composition": {ID: baseField, Label: "Name", Value: "Gopher", Focused: true, CompositionStart: 2, CompositionEnd: 4},
		"platform":    {ID: baseField, Label: "Name", Value: "Gopher", Focused: true, PlatformEditing: true},
		"aligned":     {ID: baseField, Label: "Name", Value: "Gopher", Alignment: gfx.AlignEnd},
	}
	for themeName, theme := range themes {
		for name, snapshot := range snapshots {
			t.Run(themeName+"/"+name, func(t *testing.T) {
				var view View
				flexed := testCanvas()
				var box Box
				view.Draw(flexed, theme, gfx.Rect{Width: 300, Height: 100}, func(f *Frame) {
					box = f.Input(snapshot)
				})
				direct := testCanvas()
				components.PaintInput(direct, theme, box.Rect(), snapshot)
				sameVisible(t, flexed, direct)
				fh, dh := flexed.Hits(), direct.Hits()
				if len(fh) != 1 || len(dh) != 1 || fh[0].Role != dh[0].Role || fh[0].Bounds != dh[0].Bounds ||
					hitText(flexed, &fh[0]).Bounds != hitText(direct, &dh[0]).Bounds || fh[0].Value != dh[0].Value || fh[0].Disabled != dh[0].Disabled {
					t.Errorf("hits differ:\nflex   %+v\ndirect %+v", fh, dh)
				}
			})
		}
	}
}

// withoutOrders clears paint order from a copy: two independent paints of the
// same shapes may number their calls differently, and the order is not what
// these comparisons are about.
func withoutOrders(in []gfx.Rectangle) []gfx.Rectangle {
	out := append([]gfx.Rectangle(nil), in...)
	for index := range out {
		out[index].Order = 0
	}
	return out
}

func withoutShadowOrders(in []gfx.Shadow) []gfx.Shadow {
	out := append([]gfx.Shadow(nil), in...)
	for index := range out {
		out[index].Order = 0
	}
	return out
}

func withoutTextOrders(in []gfx.Text) []gfx.Text {
	out := append([]gfx.Text(nil), in...)
	for index := range out {
		out[index].Order = 0
	}
	return out
}

// sameVisible fails unless two canvases painted the same shapes and texts. Fully transparent
// fills paint nothing and are not compared.
func sameVisible(t *testing.T, got, want *gfx.Canvas) {
	t.Helper()
	opaque := func(in []gfx.Rectangle) (out []gfx.Rectangle) {
		for _, r := range in {
			if r.Color.Alpha > 0 {
				out = append(out, r)
			}
		}
		return out
	}
	if g, w := opaque(got.Rectangles()), opaque(want.Rectangles()); !reflect.DeepEqual(withoutOrders(g), withoutOrders(w)) {
		t.Errorf("rectangles differ:\ngot  %+v\nwant %+v", g, w)
	}
	if g, w := got.Shadows(), want.Shadows(); !reflect.DeepEqual(withoutShadowOrders(g), withoutShadowOrders(w)) {
		t.Errorf("shadows differ:\ngot  %+v\nwant %+v", g, w)
	}
	if g, w := got.Texts(), want.Texts(); !reflect.DeepEqual(withoutTextOrders(g), withoutTextOrders(w)) {
		t.Errorf("texts differ:\ngot  %+v\nwant %+v", g, w)
	}
	if g, w := len(got.Triangles()), len(want.Triangles()); g != w {
		t.Errorf("triangles: got %d, want %d", g, w)
	}
}

// What the element sets itself replaces the recipe's value, field by field, and leaves the
// rest of the recipe alone: the shadcn field with square corners and a thick border keeps
// its focus color unless the caller took it over.
func TestInputKeepsTheRecipeForWhatTheCallerDidNotSet(t *testing.T) {
	theme := gfx.LightTheme()
	paint := func(focused bool, style func(Box) Box) *gfx.Canvas {
		var view View
		canvas := testCanvas()
		snapshot := components.InputSnapshot{ID: baseField, Label: "x", Value: "v", Focused: focused}
		view.Draw(canvas, theme, gfx.Rect{Width: 200, Height: 60}, func(f *Frame) { style(f.Input(snapshot)) })
		return canvas
	}
	square := paint(false, func(b Box) Box { return b.Radius(0) })
	if got := square.Rectangles()[0].Radius; got != 0 {
		t.Errorf("radius = %v, want the caller's square corner", got)
	}
	plain := paint(false, func(b Box) Box { return b })
	if got := plain.Rectangles()[0].Radius; got != theme.Metrics.RadiusControl {
		t.Errorf("radius = %v, want the recipe's %v", got, theme.Metrics.RadiusControl)
	}
	// A caller-set border takes the color, so the recipe's focus color is not painted over it.
	mine := gfx.Color{Red: .2, Green: .6, Blue: .2, Alpha: 1}
	framed := paint(true, func(b Box) Box { return b.Border(2, mine) })
	found := false
	for _, r := range framed.Rectangles() {
		if r.Color == mine {
			found = true
		}
		if r.Color == theme.Primary {
			t.Errorf("the recipe's focus border was painted over the caller's: %+v", r)
		}
	}
	if !found && len(framed.Triangles()) == 0 {
		t.Errorf("the caller's border was not painted: %+v", framed.Rectangles())
	}
	// A recipe field the caller did not set still follows the state: the focus border.
	focused := paint(true, func(b Box) Box { return b.Radius(0) })
	if focused.Rectangles()[0].Color != theme.Primary.Over(theme.Background) {
		t.Errorf("focused border = %+v, want the recipe's accent", focused.Rectangles()[0].Color)
	}
}

// A Textarea is a TextareaBase dressed by the recipe: it paints what components.PaintTextarea
// does, and a Base paints nothing of the shadcn surface.
func TestTextareaIsTheBaseDressedByTheRecipeAndPaintsAsTheComponentDoes(t *testing.T) {
	translucent := gfx.LightTheme()
	translucent.Background.Alpha = .1
	themes := map[string]gfx.Theme{"light": gfx.LightTheme(), "dark": gfx.DarkTheme(), "translucent": translucent}
	snapshots := map[string]components.TextareaSnapshot{
		"rest":     {ID: baseField, Label: "Notes", Value: "one\ntwo"},
		"empty":    {ID: baseField, Label: "Notes", Placeholder: "Write here"},
		"focused":  {ID: baseField, Label: "Notes", Value: "one\ntwo", Focused: true, ShowCaret: true, Caret: 4, SelectionStart: 0, SelectionEnd: 2},
		"invalid":  {ID: baseField, Label: "Notes", Value: "one", Invalid: true},
		"disabled": {ID: baseField, Label: "Notes", Value: "one", Disabled: true},
		"insets":   {ID: baseField, Label: "Notes", Value: "one\ntwo", InsetTop: 10, InsetBottom: 6, InsetRight: 12},
	}
	for themeName, theme := range themes {
		for name, snapshot := range snapshots {
			t.Run(themeName+"/"+name, func(t *testing.T) {
				var view View
				flexed := testCanvas()
				var box Box
				view.Draw(flexed, theme, gfx.Rect{Width: 300, Height: 200}, func(f *Frame) { box = f.Textarea(snapshot) })
				direct := testCanvas()
				components.PaintTextarea(direct, theme, box.Rect(), snapshot)
				sameVisible(t, flexed, direct)
				fh, dh := flexed.Hits(), direct.Hits()
				if len(fh) != 1 || len(dh) != 1 || fh[0].Bounds != dh[0].Bounds || hitText(flexed, &fh[0]).Bounds != hitText(direct, &dh[0]).Bounds || !fh[0].Multiline {
					t.Errorf("hits differ:\nflex   %+v\ndirect %+v", fh, dh)
				}
			})
		}
	}
}

func TestTextareaBasePaintsNoSurfaceOfItsOwn(t *testing.T) {
	theme := gfx.LightTheme()
	var view View
	canvas := testCanvas()
	view.Draw(canvas, theme, gfx.Rect{Width: 200, Height: 120}, func(f *Frame) {
		f.TextareaBase(components.TextareaSnapshot{ID: baseField, Label: "n", Value: "x", Invalid: true}).Height(80)
	})
	for _, rect := range canvas.Rectangles() {
		if rect.Color == theme.Danger || rect.Color == theme.Input {
			t.Fatalf("the shadcn surface was painted: %+v", rect)
		}
	}
	if len(canvas.Shadows()) != 0 {
		t.Fatalf("a shadow was painted: %+v", canvas.Shadows())
	}
}

// The case that asked for all this (a table's one-cell editor): a square frame two pixels
// thick in the accent color around an opaque cell, and the text exactly where the cell's own
// text was, so it does not jump when the editor opens. Declared with a FieldBase and the calls
// any element takes: no copy of the field's behavior, no hand-painted edges.
func TestFieldBaseCanBeACellEditorWhoseTextDoesNotMove(t *testing.T) {
	theme := gfx.LightTheme()
	const inset = float32(8) // where the cell's text sits from the cell's edge
	var cell Field
	var view View
	canvas := testCanvas()
	var box Box
	view.Draw(canvas, theme, gfx.Rect{Width: 300, Height: 100}, func(f *Frame) {
		f.Row().Items(Start).Children(func() {
			box = f.FieldBase(&cell, baseField, "Cell", "").Size(120, 28).
				Pad(inset-2).Border(2, theme.Primary).Radius(0).Background(theme.Background)
		})
	})
	hits := canvas.Hits()
	if len(hits) != 1 {
		t.Fatalf("hits = %+v", hits)
	}
	if got, want := hitText(canvas, &hits[0]).Bounds.X, box.Rect().X+inset; got != want {
		t.Errorf("the text starts at x=%v, want the cell's own %v: it would jump when the editor opens", got, want)
	}
	if hits[0].Bounds != box.Rect() {
		t.Errorf("hit = %+v, want the whole cell %+v", hits[0].Bounds, box.Rect())
	}
	for _, r := range canvas.Rectangles() {
		if r.Radius != 0 {
			t.Errorf("a rounded corner was painted: %+v", r)
		}
		if r.Color == theme.Input || r.Color == theme.Danger {
			t.Errorf("the shadcn surface was painted: %+v", r)
		}
	}
	if len(canvas.Shadows()) != 0 {
		t.Errorf("a shadow was painted: %+v", canvas.Shadows())
	}
}

// BenchmarkInputPage is a page of eight Inputs and a Textarea: the cost of a themed text field is
// its recipe resolved at paint time on top of the base, and it allocates nothing.
func BenchmarkInputPage(b *testing.B) {
	var view View
	canvas := testCanvas()
	snapshot := components.InputSnapshot{ID: baseField, Label: "n", Value: "Gopher", Focused: true, ShowCaret: true, Caret: 3}
	b.ReportAllocs()
	for range b.N {
		canvas.Reset()
		view.Draw(canvas, gfx.LightTheme(), gfx.Rect{Width: 400, Height: 300}, func(f *Frame) {
			f.Column().Gap(8).Pad(8).Children(func() {
				for range 8 {
					f.Input(snapshot)
				}
				f.Textarea(components.TextareaSnapshot{ID: baseOther, Label: "t", Value: "a\nb"})
			})
		})
	}
}

// What the element says of its text is the field's text style, the one value the glyphs, the
// caret and the platform follow; the default height of a base field is one line of that text.
func TestInputBaseTakesItsTextStyleFromTheElement(t *testing.T) {
	theme := gfx.LightTheme()
	ink := gfx.Color{Red: .2, Green: .3, Blue: .9, Alpha: 1}
	var view View
	canvas := testCanvas()
	var box Box
	view.Draw(canvas, theme, gfx.Rect{Width: 240, Height: 80}, func(f *Frame) {
		f.Row().Items(Start).Children(func() {
			box = f.InputBase(components.InputSnapshot{ID: baseField, Label: "Type", Value: "text", Focused: true, ShowCaret: true, Caret: 2}).
				Width(120).FontSize(11).Weight(gfx.WeightMedium).Color(ink)
		})
	})
	hits := canvas.Hits()
	hit := hits[0]
	text := hitText(canvas, &hit)
	if text.Size != 11 || text.Weight != gfx.WeightMedium || text.Color != ink {
		t.Errorf("the platform is told size %v, weight %v, color %+v, want the element's 11, medium and ink", text.Size, text.Weight, text.Color)
	}
	for _, text := range canvas.Texts() {
		if text.Value == "text" && (text.Style.Size != 11 || text.Style.Weight != gfx.WeightMedium || text.Style.Color != ink) {
			t.Errorf("the glyphs are painted in %+v", text.Style)
		}
	}
	if got := box.Rect().Height; got != 11+7 {
		t.Errorf("an 11 point base field is %v tall, want one line of its text, 18", got)
	}
}

// The element's style wins over the snapshot's, and the snapshot's stands where the element says
// nothing: a model that carries the style with the value needs no calls on the element.
func TestInputBaseElementStyleOverridesTheSnapshotsAndTheSnapshotsStandsAlone(t *testing.T) {
	theme := gfx.LightTheme()
	snapshot := components.InputSnapshot{ID: baseField, Label: "x", Value: "v", Text: components.InputTextStyle{Size: 12, Weight: gfx.WeightBold}}
	textOf := func(style func(Box) Box) gfx.HitText {
		var view View
		canvas := testCanvas()
		view.Draw(canvas, theme, gfx.Rect{Width: 240, Height: 80}, func(f *Frame) { style(f.InputBase(snapshot).Width(100)) })
		hits := canvas.Hits()
		return hitText(canvas, &hits[0])
	}
	if text := textOf(func(b Box) Box { return b }); text.Size != 12 || text.Weight != gfx.WeightBold {
		t.Errorf("the snapshot's own style was lost: size %v weight %v", text.Size, text.Weight)
	}
	if text := textOf(func(b Box) Box { return b.FontSize(16) }); text.Size != 16 || text.Weight != gfx.WeightBold {
		t.Errorf("the element's size should win and the snapshot's weight stay: size %v weight %v", text.Size, text.Weight)
	}
}

func TestInputBaseLineHeightIsTheElementsAndThePadComesOnTop(t *testing.T) {
	height := func(style func(Box) Box) float32 {
		var view View
		var box Box
		view.Draw(testCanvas(), gfx.LightTheme(), gfx.Rect{Width: 240, Height: 200}, func(f *Frame) {
			f.Row().Items(Start).Children(func() {
				box = style(f.InputBase(components.InputSnapshot{ID: baseField, Label: "x"}).Width(100))
			})
		})
		return box.Rect().Height
	}
	if got := height(func(b Box) Box { return b.LineHeight(24) }); got != 24 {
		t.Errorf("height with LineHeight(24) = %v, want 24", got)
	}
	if got := height(func(b Box) Box { return b.LineHeight(24).Pad(4) }); got != 32 {
		t.Errorf("height with LineHeight(24) and a pad of 4 = %v, want 32", got)
	}
	if got := height(func(b Box) Box { return b.FontSize(20) }); got != 27 {
		t.Errorf("height of a 20 point field = %v, want the size plus seven, 27", got)
	}
}

// TextEnd right-aligns the value of an Input: the element's declaration reaches where the
// glyph line is painted, and the hit tells the platform the same end. What the snapshot
// declared itself is not written over.
func TestInputTextEndRightAlignsTheValueAndTheEditorGeometry(t *testing.T) {
	theme := gfx.LightTheme()
	snapshot := components.InputSnapshot{ID: baseField, Label: "Amount", Value: "42", Text: components.InputTextStyle{Size: 14}}
	paint := func(style func(Box) Box) *gfx.Canvas {
		var view View
		canvas := testCanvas()
		view.Draw(canvas, theme, gfx.Rect{Width: 240, Height: 80}, func(f *Frame) {
			f.Row().Items(Start).Children(func() {
				style(f.InputBase(snapshot).Width(200))
			})
		})
		return canvas
	}
	lineOf := func(canvas *gfx.Canvas) gfx.Text {
		for _, text := range canvas.Texts() {
			if text.Value == "42" {
				return text
			}
		}
		t.Fatalf("no value painted: %+v", canvas.Texts())
		return gfx.Text{}
	}
	plain, ended := paint(func(b Box) Box { return b }), paint(func(b Box) Box { return b.TextEnd() })
	// The element's TextEnd() reaches the painted line.
	if lineOf(plain).Bounds.X == lineOf(ended).Bounds.X {
		t.Fatalf("the value did not move: %+v and %+v", lineOf(plain).Bounds, lineOf(ended).Bounds)
	}
	if lineOf(ended).Bounds.X <= lineOf(plain).Bounds.X {
		t.Errorf("the value starts at %v, want it right of the start-aligned %v", lineOf(ended).Bounds.X, lineOf(plain).Bounds.X)
	}
	// What the hit tells the platform is the same edge.
	if text := hitText(plain, &plain.Hits()[0]); text.Alignment != gfx.AlignStart {
		t.Errorf("a field without TextEnd promises alignment %v, want AlignStart", text.Alignment)
	}
	if text := hitText(ended, &ended.Hits()[0]); text.Alignment != gfx.AlignEnd {
		t.Errorf("the platform is told alignment %v, want AlignEnd", text.Alignment)
	}
	// The element does not write over a snapshot that declared its own.
	own := components.InputSnapshot{ID: baseField, Label: "Amount", Value: "42", Alignment: gfx.AlignCenter}
	var view View
	centered := testCanvas()
	view.Draw(centered, theme, gfx.Rect{Width: 240, Height: 80}, func(f *Frame) {
		f.Row().Items(Start).Children(func() {
			f.InputBase(own).Width(200).TextEnd()
		})
	})
	if text := hitText(centered, &centered.Hits()[0]); text.Alignment != gfx.AlignCenter {
		t.Errorf("the snapshot's own alignment was written over: %v, want the declared AlignCenter", text.Alignment)
	}
}
