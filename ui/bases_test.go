package ui

import (
	"reflect"
	"testing"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
)

var (
	baseBox    = tree.ID(page, "base-box")
	baseSwitch = tree.ID(page, "base-switch")
	baseButton = tree.ID(page, "base-button")
)

// A checkbox built from CheckboxBase, the recipe and the mark helper looks as the shadcn
// checkbox does and declares the same hit, for each state a box can be in: the recipe is
// the one definition of the look, and the Base is the one definition of the behavior.
func TestCheckboxBaseWithTheRecipeIsTheShadcnCheckbox(t *testing.T) {
	states := map[string]struct {
		on, focus, invalid, disabled bool
	}{
		"off": {}, "on": {on: true}, "focus": {focus: true}, "focus on": {on: true, focus: true},
		"invalid": {invalid: true}, "disabled": {disabled: true}, "disabled on": {on: true, disabled: true},
	}
	for themeName, theme := range map[string]gfx.Theme{"light": gfx.LightTheme(), "dark": gfx.DarkTheme()} {
		for name, s := range states {
			t.Run(themeName+"/"+name, func(t *testing.T) {
				state := gfx.State{Checked: s.on, FocusVisible: s.focus, Invalid: s.invalid, Disabled: s.disabled}
				var view View
				flexed := testCanvas()
				flexed.SetInteraction(gfx.ID{}, gfx.ID{}, focusIf(s.focus), true)
				var box Box
				view.Draw(flexed, theme, gfx.Rect{Width: 100, Height: 60}, func(f *Frame) {
					look := components.CheckboxRecipe(theme, state)
					box = f.CheckboxBase(baseBox, s.on).Radius(look.Radius).Shadow(look.Shadow).Background(look.Fill).Border(1, look.Border).
						Ring(look.RingWidth, look.Ring).
						Draw(func(d *DrawContext) { components.PaintCheckboxMark(d.Canvas, d.Rect, state, look.Mark) })
				})
				direct := testCanvas()
				direct.SetInteraction(gfx.ID{}, gfx.ID{}, focusIf(s.focus), true)
				bc := components.BeginCheckbox(direct, theme, baseBox, box.Rect(), s.on).Label("checkbox")
				if s.invalid {
					bc = bc.Invalid()
				}
				if s.disabled {
					bc = bc.Disabled()
				}
				bc.Paint()

				fh, dh := flexed.Hits(), direct.Hits()
				if len(fh) != 1 || len(dh) != 1 || fh[0].Role != dh[0].Role || fh[0].Checked != dh[0].Checked || fh[0].Bounds != dh[0].Bounds {
					t.Errorf("hits differ:\nflex   %+v\ndirect %+v", fh, dh)
				}
				// The box itself paints the same shapes (the halo is a ring outside it here and a
				// disc under it in the component: the same pixels, drawn differently).
				boxOf := func(c *gfx.Canvas) (out []gfx.Rectangle) {
					for _, r := range c.Rectangles() {
						if r.Bounds == box.Rect() {
							out = append(out, r)
						}
					}
					return out
				}
				if f, d := boxOf(flexed), boxOf(direct); len(f) == 0 || !reflect.DeepEqual(withoutOrders(f), withoutOrders(d)) {
					t.Errorf("the box differs:\nflex   %+v\ndirect %+v", f, d)
				}
				if s.on {
					if f, d := len(flexed.Triangles()), len(direct.Triangles()); f < d {
						t.Errorf("the check mark was not painted: %d triangles, the component's %d", f, d)
					}
				}
				if (s.focus || s.invalid) && len(flexed.Triangles()) == len(plainTriangles(theme, s.on)) {
					t.Errorf("no halo was painted for a focused or invalid box")
				}
			})
		}
	}
}

// plainTriangles is what a box in no special state paints as strokes: the mark of a
// checked one, nothing for an unchecked one.
func plainTriangles(theme gfx.Theme, on bool) []gfx.Triangle {
	var view View
	canvas := testCanvas()
	view.Draw(canvas, theme, gfx.Rect{Width: 100, Height: 60}, func(f *Frame) {
		state := gfx.State{Checked: on}
		look := components.CheckboxRecipe(theme, state)
		f.CheckboxBase(baseBox, on).Radius(look.Radius).Background(look.Fill).Border(1, look.Border).
			Draw(func(d *DrawContext) { components.PaintCheckboxMark(d.Canvas, d.Rect, state, look.Mark) })
	})
	return canvas.Triangles()
}

func focusIf(on bool) gfx.ID {
	if on {
		return baseBox
	}
	return gfx.ID{}
}

func TestCheckboxBaseIsACheckboxToAssistiveTechnologyAndReportsItsState(t *testing.T) {
	var view View
	canvas := testCanvas()
	var box Box
	view.Draw(canvas, gfx.LightTheme(), gfx.Rect{Width: 100, Height: 60}, func(f *Frame) {
		box = f.CheckboxBase(baseBox, true).Label("Accept")
	})
	hits := canvas.Hits()
	if len(hits) != 1 || hits[0].Role != gfx.RoleCheckbox || !hits[0].Checked || hits[0].Label != "Accept" {
		t.Fatalf("hits = %+v", hits)
	}
	side := components.CheckboxExtent(gfx.LightTheme())
	if r := box.Rect(); r.Width != side || r.Height != side {
		t.Errorf("size = %v x %v, want the box's %v", r.Width, r.Height, side)
	}
	if hits[0].Bounds.Width <= side || hits[0].Bounds.Height <= side {
		t.Errorf("the pointer region %+v does not reach beyond the box", hits[0].Bounds)
	}
}

func TestSwitchBaseIsASwitchToAssistiveTechnology(t *testing.T) {
	theme := gfx.LightTheme()
	var view View
	canvas := testCanvas()
	var box Box
	view.Draw(canvas, theme, gfx.Rect{Width: 100, Height: 60}, func(f *Frame) {
		box = f.SwitchBase(baseSwitch, true).Label("Wi-Fi")
	})
	hits := canvas.Hits()
	if len(hits) != 1 || hits[0].Role != gfx.RoleSwitch || !hits[0].Checked || hits[0].Label != "Wi-Fi" {
		t.Fatalf("hits = %+v", hits)
	}
	width, height := components.SwitchExtent(theme, false)
	if r := box.Rect(); r.Width != width || r.Height != height {
		t.Errorf("size = %v x %v, want the track's %v x %v", r.Width, r.Height, width, height)
	}
	hitWidth, hitHeight := components.SwitchHitExtent(theme, false)
	if hits[0].Bounds.Width != hitWidth || hits[0].Bounds.Height != hitHeight {
		t.Errorf("pointer region %v x %v, want the control's %v x %v", hits[0].Bounds.Width, hits[0].Bounds.Height, hitWidth, hitHeight)
	}
}

// A ButtonBase has no look and is a button: it centers what it holds and is clicked.
func TestButtonBaseIsAButtonWithoutALook(t *testing.T) {
	theme := gfx.LightTheme()
	var view View
	canvas := testCanvas()
	var box Box
	var child Box
	view.Draw(canvas, theme, gfx.Rect{Width: 200, Height: 80}, func(f *Frame) {
		f.Row().Items(Start).Children(func() {
			box = f.ButtonBase(baseButton, "Add").Size(100, 40).Children(func() { child = f.Text("Add") })
		})
	})
	hits := canvas.Hits()
	if len(hits) != 1 || hits[0].Role != gfx.RoleButton || hits[0].Label != "Add" || !hits[0].Focusable {
		t.Fatalf("hits = %+v", hits)
	}
	if len(canvas.Rectangles()) != 0 || len(canvas.Shadows()) != 0 {
		t.Errorf("a ButtonBase painted a look: %+v", canvas.Rectangles())
	}
	b, c := box.Rect(), child.Rect()
	if left, right := c.X-b.X, b.X+b.Width-c.X-c.Width; left != right {
		t.Errorf("the label is not centered across: %v left, %v right", left, right)
	}
}

func TestBasesAreAllocationFreeAfterWarmup(t *testing.T) {
	var view View
	canvas := testCanvas()
	theme := gfx.LightTheme()
	look := components.CheckboxRecipe(theme, gfx.State{Checked: true})
	mark := func(d *DrawContext) { components.PaintCheckboxMark(d.Canvas, d.Rect, d.State, look.Mark) }
	draw := func() {
		canvas.Reset()
		view.Draw(canvas, theme, gfx.Rect{Width: 200, Height: 100}, func(f *Frame) {
			f.Row().Gap(8).Children(func() {
				f.CheckboxBase(baseBox, true).Radius(look.Radius).Background(look.Fill).Border(1, look.Border).Draw(mark)
				f.SwitchBase(baseSwitch, true)
				f.ButtonBase(baseButton, "ok").Pad(4).Children(func() { f.Text("ok") })
			})
		})
	}
	draw()
	if allocs := testing.AllocsPerRun(100, draw); allocs != 0 {
		t.Fatalf("allocs = %v, want 0", allocs)
	}
}

// Each item Base is the control it names to assistive technology, reports its state, and
// reaches the drawing callback as the matching gfx.State: a strip of tabs, a segmented control
// or a select is made of them without a copy of what they do.
func TestItemBasesAreTheControlsTheyNameAndReportTheirState(t *testing.T) {
	theme := gfx.LightTheme()
	type item struct {
		name  string
		role  gfx.Role
		build func(f *Frame, id gfx.ID) Box
		check func(gfx.State) bool
		hit   func(gfx.Hit) bool
	}
	items := []item{
		{"toggle", gfx.RoleButton, func(f *Frame, id gfx.ID) Box { return f.ToggleBase(id, "Bold", true) },
			func(s gfx.State) bool { return s.Checked && s.Selected }, func(h gfx.Hit) bool { return h.Checked && h.Selected }},
		{"tab", gfx.RoleTab, func(f *Frame, id gfx.ID) Box { return f.TabBase(id, "General", true) },
			func(s gfx.State) bool { return s.Selected && !s.Checked }, func(h gfx.Hit) bool { return h.Selected }},
		{"radio", gfx.RoleRadio, func(f *Frame, id gfx.ID) Box { return f.RadioBase(id, "List", true) },
			func(s gfx.State) bool { return s.Checked && s.Selected }, func(h gfx.Hit) bool { return h.Checked && h.Selected }},
		{"option", gfx.RoleOption, func(f *Frame, id gfx.ID) Box { return f.OptionBase(id, "Small", true) },
			func(s gfx.State) bool { return s.Selected }, func(h gfx.Hit) bool { return h.Selected }},
		{"combobox", gfx.RoleCombobox, func(f *Frame, id gfx.ID) Box { return f.ComboboxBase(id, "Size", "Small", true) },
			func(s gfx.State) bool { return s.Open }, func(h gfx.Hit) bool { return h.Expanded && h.Expandable && h.Value == "Small" }},
	}
	for _, it := range items {
		t.Run(it.name, func(t *testing.T) {
			var view View
			canvas := testCanvas()
			var seen gfx.State
			view.Draw(canvas, theme, gfx.Rect{Width: 200, Height: 80}, func(f *Frame) {
				f.Row().Items(Start).Children(func() {
					it.build(f, baseButton).Size(80, 28).Draw(func(d *DrawContext) { seen = d.State })
				})
			})
			hits := canvas.Hits()
			if len(hits) != 1 || hits[0].Role != it.role || !it.hit(hits[0]) || !hits[0].Focusable {
				t.Errorf("hit = %+v, want a focusable %v in its state", hits, it.role)
			}
			if !it.check(seen) {
				t.Errorf("the drawing callback saw %+v", seen)
			}
			if len(canvas.Rectangles()) != 0 {
				t.Errorf("an item Base painted a look: %+v", canvas.Rectangles())
			}
		})
	}
}
