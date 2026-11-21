package base_test

import (
	"testing"

	. "github.com/vivaui/viva/base"
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
)

type choiceProbe struct {
	box    Rect
	rects  []gfx.Rectangle
	texts  []gfx.Text
	hits   []gfx.Hit
	marked bool
}

func probeChoice(t *testing.T, declare func(f *Frame) Box) choiceProbe {
	t.Helper()
	canvas := testCanvas()
	var view View
	var box Box
	view.Draw(canvas, gfx.LightTheme(), gfx.Rect{Width: 400, Height: 200}, func(f *Frame) {
		f.Column().Pad(6).Children(func() { box = declare(f) })
	})
	return choiceProbe{
		box: box.Rect(), rects: canvas.Rectangles(), texts: canvas.Texts(), hits: canvas.Hits(),
		marked: len(canvas.Triangles()) > 0,
	}
}

func checkboxBoxRect(t *testing.T, p choiceProbe) gfx.Rect {
	t.Helper()
	side := components.CheckboxExtent(gfx.LightTheme())
	for _, rect := range p.rects {
		// The box's surface is the square of the checkbox's extent at the start of
		// the element when it has a caption.
		if rect.Bounds.Width == side && rect.Bounds.Height == side {
			return rect.Bounds
		}
	}
	t.Fatalf("no %v checkbox box in %+v", side, p.rects)
	return gfx.Rect{}
}

// A themed Checkbox's box, caption and mark come from the recipe and the
// element: the box surface from CheckboxRecipe, the caption text in the
// element's style, and the mark still the control's graphic.
func TestCheckboxDefaultsAreTheRecipe(t *testing.T) {
	theme := gfx.LightTheme()
	look := components.CheckboxRecipe(theme, gfx.State{Checked: true})
	p := probeChoice(t, func(f *Frame) Box { return f.Checkbox(gfx.Item(1), true).Caption("Sync") })
	box := checkboxBoxRect(t, p)
	if !p.marked {
		t.Error("the checkbox painted no mark")
	}
	found := false
	for _, rect := range p.rects {
		if rect.Bounds == box {
			found = true
			if rect.Color != look.Fill || rect.Radius != look.Radius {
				t.Errorf("box = %v radius %v, want %v radius %v", rect.Color, rect.Radius, look.Fill, look.Radius)
			}
		}
	}
	if !found {
		t.Errorf("no box surface at %v: %+v", box, p.rects)
	}
	if len(p.texts) != 1 || p.texts[0].Value != "Sync" {
		t.Fatalf("texts = %+v, want the caption", p.texts)
	}
	if p.texts[0].Style.Size != theme.Metrics.FontSizeM || p.texts[0].Style.Weight != gfx.WeightMedium {
		t.Errorf("caption style = %+v, want body size and medium", p.texts[0].Style)
	}
	if p.texts[0].Bounds.X < box.X+box.Width {
		t.Errorf("caption %v overlaps the box %v", p.texts[0].Bounds, box)
	}
	hit := hitByIDWithRole(p.hits, gfx.RoleCheckbox)
	if hit.ID != gfx.Item(1) || !hit.Checked || hit.Role != gfx.RoleCheckbox {
		t.Errorf("hit = %+v, want a checked checkbox", hit)
	}
}

func hitByIDWithRole(hits []gfx.Hit, role gfx.Role) gfx.Hit {
	for _, hit := range hits {
		if hit.Role == role {
			return hit
		}
	}
	return gfx.Hit{}
}

// FontSize, Weight and Color style the caption; the box's own surface takes
// Background and Radius.
func TestCheckboxPropertiesReachTheCaptionAndBox(t *testing.T) {
	ink := gfx.Color{Red: 1, Alpha: 1}
	fill := gfx.Color{Blue: 1, Alpha: 1}
	p := probeChoice(t, func(f *Frame) Box {
		return f.Checkbox(gfx.Item(1), false).Caption("Sync").FontSize(18).Weight(gfx.WeightBold).Color(ink).Background(fill).Radius(3)
	})
	if len(p.texts) != 1 {
		t.Fatalf("texts = %+v", p.texts)
	}
	if got := p.texts[0].Style; got.Size != 18 || got.Weight != gfx.WeightBold || got.Color != ink {
		t.Errorf("caption style = %+v, want 18, bold, red", got)
	}
	found := false
	for _, rect := range p.rects {
		if rect.Color == fill {
			found = true
		}
	}
	if !found {
		t.Errorf("no box fill %v: %+v", fill, p.rects)
	}
	outer := float32(0)
	for _, rect := range p.rects {
		if rect.Radius > outer {
			outer = rect.Radius
		}
	}
	if outer != 3 {
		t.Errorf("box radius = %v, want the explicit 3", outer)
	}
}

// A mixed box reports mixed, an invalid one paints the recipe's danger ring.
func TestCheckboxMixedAndInvalid(t *testing.T) {
	theme := gfx.LightTheme()
	p := probeChoice(t, func(f *Frame) Box { return f.Checkbox(gfx.Item(1), false).Mixed(true) })
	hit := hitByIDWithRole(p.hits, gfx.RoleCheckbox)
	if !hit.Mixed || hit.Checked {
		t.Errorf("mixed hit = %+v, want mixed and not checked", hit)
	}
	look := components.CheckboxRecipe(theme, gfx.State{Invalid: true})
	invalid := probeChoice(t, func(f *Frame) Box {
		return f.Checkbox(gfx.Item(1), false).Invalid(true)
	})
	// The ring is the recipe's invalid ring: a translucent, red-dominant halo
	// around the box.
	ring := false
	for _, rect := range invalid.rects {
		if rect.Color.Red > rect.Color.Green && rect.Color.Alpha > 0 && rect.Color.Alpha < 1 {
			ring = true
		}
	}
	_ = look
	if !ring {
		t.Errorf("invalid checkbox paints no danger ring: %+v", invalid.rects)
	}
	if hit := hitByIDWithRole(invalid.hits, gfx.RoleCheckbox); !hit.Invalid {
		t.Errorf("invalid checkbox hit = %+v, want Invalid", hit)
	}
}

// A themed Switch's track is the recipe's surface (sized by explicit Width and
// Height), its thumb stays the control's graphic, and its caption is the
// element's label.
func TestSwitchDefaultsAreTheRecipe(t *testing.T) {
	theme := gfx.LightTheme()
	look := components.SwitchRecipe(theme, gfx.State{}, 1)
	p := probeChoice(t, func(f *Frame) Box { return f.Switch(gfx.Item(2), true).Caption("Running") })
	if len(p.texts) != 1 || p.texts[0].Value != "Running" {
		t.Fatalf("texts = %+v, want the caption", p.texts)
	}
	var track gfx.Rectangle
	for _, rect := range p.rects {
		if rect.Color == look.Track {
			track = rect
		}
	}
	if track.Bounds.Width == 0 {
		t.Fatalf("no track with %v: %+v", look.Track, p.rects)
	}
	if track.Radius != track.Bounds.Height/2 {
		t.Errorf("track %v radius %v, want a pill of its own half height", track.Bounds, track.Radius)
	}
	thumb := false
	for _, rect := range p.rects {
		if rect.Color == look.Thumb && rect.Bounds.Width < track.Bounds.Height {
			thumb = true
		}
	}
	if !thumb {
		t.Errorf("no thumb %v inside the track: %+v", look.Thumb, p.rects)
	}
	hit := hitByIDWithRole(p.hits, gfx.RoleSwitch)
	if !hit.Checked || hit.Role != gfx.RoleSwitch {
		t.Errorf("hit = %+v, want a checked switch", hit)
	}

	wide := probeChoice(t, func(f *Frame) Box {
		return f.Switch(gfx.Item(3), false).Caption("Running").Width(80).Height(40)
	})
	var wideTrack gfx.Rectangle
	for _, rect := range wide.rects {
		if rect.Color == components.SwitchRecipe(theme, gfx.State{}, 0).Track {
			wideTrack = rect
		}
	}
	if wideTrack.Bounds.Width != 80 || wideTrack.Bounds.Height != 40 {
		t.Errorf("explicit switch track = %v, want 80x40", wideTrack.Bounds)
	}
}

// An explicit pixel Size sizes a Checkbox's box itself, not only its element,
// and a non-square box draws a mark sized for its shorter side.
func TestCheckboxExplicitSizeSizesTheBox(t *testing.T) {
	p := probeChoice(t, func(f *Frame) Box { return f.Checkbox(gfx.Item(1), true).Size(22, 24) })
	var box gfx.Rectangle
	for _, rect := range p.rects {
		if rect.Bounds.Width == 22 && rect.Bounds.Height == 24 {
			box = rect
		}
	}
	if box.Bounds.Width == 0 {
		t.Fatalf("no 22x24 box in %+v", p.rects)
	}
	if !p.marked {
		t.Error("the explicit box painted no mark")
	}
}

// FillWidth and Percent sizes size a Switch's track from what the layout gave
// the element, keeping the caption's room out of the track.
func TestSwitchStretchedTrack(t *testing.T) {
	theme := gfx.LightTheme()
	off := components.SwitchRecipe(theme, gfx.State{}, 0)
	caption := "Running"
	canvas := testCanvas()
	var view View
	var box Box
	view.Draw(canvas, theme, gfx.Rect{Width: 400, Height: 120}, func(f *Frame) {
		f.Column().Width(400).Children(func() {
			f.Column().Width(400).Children(func() {
				box = f.Switch(gfx.Item(1), false).Caption(caption).FillWidth()
			})
		})
	})
	width := canvas.TextWidthWeight(caption, theme.Metrics.FontSizeM, gfx.WeightMedium)
	want := box.Rect().Width - width - CaptionGap
	track := gfx.Rectangle{}
	for _, rect := range canvas.Rectangles() {
		if rect.Color == off.Track && rect.Bounds.Height > 0 {
			track = rect
		}
	}
	if track.Bounds.Width == 0 {
		t.Fatalf("no track with %v in %+v", off.Track, canvas.Rectangles())
	}
	if diff := track.Bounds.Width - want; diff > 0.01 || diff < -0.01 {
		t.Errorf("track width = %v, want the element width less the caption and gap (%v)", track.Bounds.Width, want)
	}
	if track.Radius != track.Bounds.Height/2 {
		t.Errorf("track radius = %v, want half its height %v", track.Radius, track.Bounds.Height/2)
	}
}
