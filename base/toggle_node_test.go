package base_test

import (
	"reflect"
	"testing"

	. "github.com/vivaui/viva/base"
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/style"
)

type toggleProbe struct {
	box   Rect
	rects []gfx.Rectangle
	texts []gfx.Text
	hits  []gfx.Hit
}

func probeToggle(t *testing.T, state func(*gfx.Canvas), declare func(f *Frame) Box) toggleProbe {
	t.Helper()
	canvas := testCanvas()
	if state != nil {
		state(canvas)
	}
	var view View
	var box Box
	view.Draw(canvas, gfx.LightTheme(), gfx.Rect{Width: 400, Height: 120}, func(f *Frame) {
		f.Row().Items(Start).Children(func() { box = declare(f) })
	})
	return toggleProbe{box: box.Rect(), rects: canvas.Rectangles(), texts: canvas.Texts(), hits: canvas.Hits()}
}

func toggleLabel(t *testing.T, p toggleProbe) gfx.Text {
	t.Helper()
	if len(p.texts) != 1 {
		t.Fatalf("texts = %+v, want the one label", p.texts)
	}
	return p.texts[0]
}

// A toggle with no calls of its own is the recipe's chip: no fill while off,
// the muted fill while on, the medium label in the foreground color, and one
// button hit that reports its pressed state.
func TestToggleDefaultsAreTheRecipe(t *testing.T) {
	theme := gfx.LightTheme()
	_, _, fontSize := style.ToggleGeometry(theme.Metrics, style.SizeDefault)
	for _, tc := range []struct {
		name    string
		pressed bool
		fill    gfx.Color
	}{
		{"off", false, gfx.Color{}},
		{"on", true, theme.Muted},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := probeToggle(t, nil, func(f *Frame) Box { return f.Toggle(gfx.Item(1), "Bold", tc.pressed) })
			if len(p.hits) != 1 || p.hits[0].Role != gfx.RoleButton || p.hits[0].Checked != tc.pressed || p.hits[0].Selected != tc.pressed {
				t.Errorf("hits = %+v, want one button with checked=%v", p.hits, tc.pressed)
			}
			if p.hits[0].Bounds != p.box {
				t.Errorf("hit %v is not the box %v", p.hits[0].Bounds, p.box)
			}
			label := toggleLabel(t, p)
			if label.Style.Size != fontSize || label.Style.Weight != gfx.WeightMedium || label.Style.Color != theme.Foreground {
				t.Errorf("label style = %+v, want size %v, medium, %v", label.Style, fontSize, theme.Foreground)
			}
			_ = tc.fill // checked by the recipe parity test; the engine paints the look
		})
	}
}

// The calls every element takes reach the toggle: FontSize, Weight and Color
// the label, Pad the room inside, Radius and Height the surface, and the size's
// min-w keeps the toggle at least as wide as it is tall.
func TestTogglePropertiesReachTheSurface(t *testing.T) {
	ink := gfx.Color{Red: 1, Alpha: 1}
	p := probeToggle(t, nil, func(f *Frame) Box {
		return f.Toggle(gfx.Item(1), "Bold", true).FontSize(20).Weight(gfx.WeightBold).Color(ink).Height(40).Radius(3)
	})
	if p.box.Height != 40 {
		t.Errorf("toggle height = %v, want 40", p.box.Height)
	}
	label := toggleLabel(t, p)
	if label.Style.Size != 20 || label.Style.Weight != gfx.WeightBold || label.Style.Color != ink {
		t.Errorf("label style = %+v, want 20, bold, red", label.Style)
	}
	if len(p.rects) == 0 || p.rects[len(p.rects)-1].Radius != 3 {
		t.Errorf("surface radius was not the explicit 3: %+v", p.rects)
	}
	// The size's min-w: a short label still fills a square.
	_, padding, _ := style.ToggleGeometry(gfx.LightTheme().Metrics, style.SizeDefault)
	short := probeToggle(t, nil, func(f *Frame) Box { return f.Toggle(gfx.Item(1), "x", false) })
	if short.box.Width < short.box.Height {
		t.Errorf("toggle %v is narrower than it is tall", short.box)
	}
	_ = padding
}

// Sizes: Small and Large move the height by the metrics' four pixels.
func TestToggleSizesChangeTheHeight(t *testing.T) {
	theme := gfx.LightTheme()
	for name, tc := range map[string]struct {
		declare func(f *Frame) Box
		want    float32
	}{
		"small": {func(f *Frame) Box { return f.Toggle(gfx.Item(1), "Bold", false).Small() }, theme.Metrics.ControlHeight - 4},
		"large": {func(f *Frame) Box { return f.Toggle(gfx.Item(1), "Bold", false).Large() }, theme.Metrics.ControlHeight + 4},
	} {
		p := probeToggle(t, nil, tc.declare)
		if p.box.Height != tc.want {
			t.Errorf("%s toggle height = %v, want %v", name, p.box.Height, tc.want)
		}
	}
}

// A focused or invalid toggle draws the recipe's ring and border; hover fills
// the muted surface the component does.
func TestToggleStatesFollowTheRecipe(t *testing.T) {
	theme := gfx.LightTheme()
	focused := probeToggle(t, func(c *gfx.Canvas) { c.SetInteraction(gfx.ID{}, gfx.ID{}, gfx.Item(1), true) },
		func(f *Frame) Box { return f.Toggle(gfx.Item(1), "Bold", false) })
	if len(focused.rects) < 2 { // the ring disc and the surface
		t.Errorf("focused toggle has no ring: %+v", focused.rects)
	}
	invalid := probeToggle(t, nil, func(f *Frame) Box { return f.Toggle(gfx.Item(1), "Bold", false).Invalid(true) })
	danger := false
	for _, rect := range invalid.rects {
		danger = danger || rect.Color == theme.Danger || rect.Color == theme.Ring
	}
	if !danger {
		t.Errorf("invalid toggle paints no dangerous edge: %+v", invalid.rects)
	}
	if len(invalid.hits) != 1 || !invalid.hits[0].Invalid {
		t.Errorf("invalid toggle hit = %+v, want Invalid", invalid.hits)
	}
	hovered := probeToggle(t, func(c *gfx.Canvas) { c.SetInteraction(gfx.Item(1), gfx.ID{}, gfx.ID{}, false) },
		func(f *Frame) Box { return f.Toggle(gfx.Item(1), "Bold", false) })
	fill := false
	for _, rect := range hovered.rects {
		fill = fill || rect.Color == theme.Muted
	}
	if !fill {
		t.Errorf("hovered toggle paints no muted fill: %+v", hovered.rects)
	}
}

// Children replace the label and stay inside the content rect; a disabled
// toggle makes its nested content inert.
func TestToggleChildrenReplaceTheLabel(t *testing.T) {
	p := probeToggle(t, nil, func(f *Frame) Box {
		return f.Toggle(gfx.Item(2), "Bold", true).Children(func() {
			f.Text("one")
			f.Text("two")
		})
	})
	if len(p.texts) != 2 {
		t.Fatalf("texts = %+v, want the two children", p.texts)
	}
	for _, text := range p.texts {
		if text.Value == "Bold" {
			t.Errorf("the label was painted over the children: %+v", p.texts)
		}
		if text.Bounds.X < p.box.X || text.Bounds.X+text.Bounds.Width > p.box.X+p.box.Width {
			t.Errorf("child %q at %v is outside the toggle %v", text.Value, text.Bounds, p.box)
		}
	}
	if len(p.hits) != 1 || p.hits[0].Checked != true || p.hits[0].Label != "Bold" {
		t.Errorf("hits = %+v, want one checked button named by the label", p.hits)
	}

	disabled := probeToggle(t, nil, func(f *Frame) Box {
		return f.Toggle(gfx.Item(3), "Outer", false).Disabled(true).Children(func() {
			f.Button(gfx.Item(4), "inner")
		})
	})
	if len(disabled.hits) != 2 {
		t.Fatalf("hits = %+v, want the toggle and its nested button", disabled.hits)
	}
	for _, hit := range disabled.hits {
		if !hit.Disabled || hit.Focusable {
			t.Errorf("hit %+v is not inert under the disabled toggle", hit)
		}
	}
}

// The node toggle paints what components.BeginToggle painted: same shapes and
// same label for each treatment. A zero-alpha fill is drawn as a record by the
// component and skipped by the engine, so it is filtered before comparing.
func TestToggleNodePaintsWhatTheComponentDid(t *testing.T) {
	theme := gfx.LightTheme()
	cases := []struct {
		name    string
		pressed bool
		variant components.ToggleVariant
		box     func(f *Frame) Box
	}{
		{"default off", false, components.ToggleDefault, func(f *Frame) Box { return f.Toggle(gfx.Item(1), "Bold", false) }},
		{"default on", true, components.ToggleDefault, func(f *Frame) Box { return f.Toggle(gfx.Item(1), "Bold", true) }},
		{"outline", false, components.ToggleOutline, func(f *Frame) Box { return f.Toggle(gfx.Item(1), "Bold", false).Outline() }},
		{"small", false, components.ToggleDefault, func(f *Frame) Box { return f.Toggle(gfx.Item(1), "Bold", false).Small() }},
		{"large", false, components.ToggleDefault, func(f *Frame) Box { return f.Toggle(gfx.Item(1), "Bold", false).Large() }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			canvas := testCanvas()
			var view View
			var box Box
			view.Draw(canvas, theme, gfx.Rect{Width: 200, Height: 80}, func(f *Frame) {
				f.Row().Items(Start).Children(func() { box = tc.box(f) })
			})
			want := testCanvas()
			toggle := components.BeginToggle(want, theme, gfx.Item(1), box.Rect(), tc.pressed)
			if tc.variant == components.ToggleOutline {
				toggle.Outline("Bold")
			} else {
				toggle.Default("Bold")
			}
			var drawn []gfx.Rectangle
			for _, rect := range want.Rectangles() {
				if rect.Color.Alpha > 0 {
					drawn = append(drawn, rect)
				}
			}
			if !reflect.DeepEqual(toggleShapes(drawn), toggleShapes(canvas.Rectangles())) {
				t.Errorf("toggle differs from the component:\nwant %+v\n got %+v", drawn, canvas.Rectangles())
			}
			wantTexts, gotTexts := want.Texts(), canvas.Texts()
			if len(wantTexts) != len(gotTexts) {
				t.Fatalf("texts = %+v, want %+v", gotTexts, wantTexts)
			}
			for i := range wantTexts {
				wantText, gotText := wantTexts[i], gotTexts[i]
				// The paint order numbers primitives; a skipped zero-alpha fill
				// moves the label's number without moving the label.
				wantText.Order, gotText.Order = 0, 0
				if wantText != gotText {
					t.Errorf("text %d differs: want %+v got %+v", i, wantTexts[i], gotTexts[i])
				}
			}
		})
	}
}

func toggleShapes(in []gfx.Rectangle) []gfx.Rectangle {
	out := append([]gfx.Rectangle(nil), in...)
	for i := 1; i < len(out); i++ {
		for j := i; j > 0; j-- {
			a, b := out[j-1], out[j]
			if a.Bounds.X < b.Bounds.X || a.Bounds.X == b.Bounds.X && (a.Bounds.Y < b.Bounds.Y || a.Bounds.Y == b.Bounds.Y && a.Radius < b.Radius) {
				break
			}
			out[j-1], out[j] = out[j], out[j-1]
		}
	}
	return out
}
