package recipes

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var (
	tree = gfx.Tree{}
	root = tree.Feature("recipes")

	glyph = gfx.Icon{ViewBox: 24, Stroke: 2, Cap: gfx.CapRound, Commands: []gfx.PathCommand{gfx.MoveTo(4, 12), gfx.LineTo(20, 12)}}
)

var themes = map[string]gfx.Theme{"light": gfx.LightTheme(), "dark": gfx.DarkTheme()}

func canvasWith(hovered, focused gfx.ID, focusVisible bool) *gfx.Canvas {
	canvas := &gfx.Canvas{}
	canvas.SetTree(&tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 { return float32(len(value)) * size / 2 })
	canvas.SetInteraction(hovered, gfx.ID{}, focused, focusVisible)
	return canvas
}

func newPage(theme gfx.Theme) *ui.Page {
	return ui.NewPage(ui.PageConfig{Tree: &tree, Root: "recipes", Theme: &theme})
}

// draw declares build into a 360 wide page and returns what it painted.
func draw(page *ui.Page, theme gfx.Theme, canvas *gfx.Canvas, build func(c ui.C)) {
	canvas.Reset()
	canvas.BeginFrame()
	page.Draw(canvas, gfx.Rect{Width: 360, Height: 480}, func(c ui.C) {
		ui.Column(c).Pad(16).Gap(12).Children(func() { build(c) })
	})
	canvas.EndFrame()
}

// hitOf finds the control labelled label in the canvas, for the tests that set
// the pointer or focus on it before drawing again.
func hitOf(t *testing.T, canvas *gfx.Canvas, label string) gfx.ID {
	t.Helper()
	for _, hit := range canvas.Hits() {
		if hit.Label == label && !hit.IsLabel && !hit.Backdrop {
			return hit.ID
		}
	}
	t.Fatalf("no control labelled %q", label)
	return gfx.ID{}
}

// Every recipe paints something and stays inside the room it is given, in both
// themes: a recipe that overflows its column or paints nothing is broken
// whatever it looks like.
func TestEveryRecipePaintsInsideItsRoom(t *testing.T) {
	recipes := map[string]func(c ui.C, theme gfx.Theme) ui.Box{
		"settings row": func(c ui.C, theme gfx.Theme) ui.Box {
			return SettingsRow(c, theme, "Notifications", "Tell me when a long transfer finishes, even while the window is closed.", new(bool))
		},
		"list row": func(c ui.C, theme gfx.Theme) ui.Box {
			return ListRow(c, theme, glyph, glyph, "production-database-primary-replica-eu-west-1", "ssh", false)
		},
		"avatar": func(c ui.C, theme gfx.Theme) ui.Box { return Avatar(c, theme, 40, "AB", true) },
		"badge":  func(c ui.C, theme gfx.Theme) ui.Box { return Badge(c, theme, ToneDanger, "Failed") },
		"stat": func(c ui.C, theme gfx.Theme) ui.Box {
			return StatCard(c, theme, "1,284", "Active sessions", gfx.RGB(99, 102, 241), gfx.RGB(236, 72, 153))
		},
		"hero": func(c ui.C, theme gfx.Theme) ui.Box {
			return Hero(c, theme, "Welcome back", "Pick up where you left off on any of your hosts.", "Open")
		},
		"icon button": func(c ui.C, theme gfx.Theme) ui.Box { return IconButton(c, theme, glyph, "Search", false) },
		"empty state": func(c ui.C, theme gfx.Theme) ui.Box {
			return EmptyState(c, theme, glyph, "No hosts yet", "Add a host to connect to it from here.", "Add host")
		},
		"section": func(c ui.C, theme gfx.Theme) ui.Box {
			return Section(c, theme, "General", true, func() { ui.Text(c, "content") })
		},
	}
	for name, recipe := range recipes {
		for themeName, theme := range themes {
			t.Run(name+"/"+themeName, func(t *testing.T) {
				canvas := canvasWith(gfx.ID{}, gfx.ID{}, false)
				page := newPage(theme)
				var box ui.Box
				draw(page, theme, canvas, func(c ui.C) { box = recipe(c, theme) })
				if len(canvas.Rectangles())+len(canvas.Triangles())+len(canvas.Texts()) == 0 {
					t.Fatal("painted nothing")
				}
				if r := box.Rect(); r.X < 16-0.01 || r.Right() > 344+0.01 {
					t.Errorf("spans %v to %v, want inside the column's 16 to 344", r.X, r.Right())
				}
			})
		}
	}
}

func TestPencilOfAListRowAppearsWithTheRowAndOnlyThen(t *testing.T) {
	theme := gfx.LightTheme()
	page := newPage(theme)
	canvas := canvasWith(gfx.ID{}, gfx.ID{}, false)
	draw(page, theme, canvas, func(c ui.C) {
		ListRow(c, theme, glyph, glyph, "host", "ssh", false)
	})
	row := hitOf(t, canvas, "host")
	triangles := func(hovered, focused gfx.ID, visible bool) int {
		canvas := canvasWith(hovered, focused, visible)
		draw(page, theme, canvas, func(c ui.C) {
			ListRow(c, theme, glyph, glyph, "host", "ssh", false)
		})
		return len(canvas.Triangles())
	}
	rest := triangles(gfx.ID{}, gfx.ID{}, false)
	if hovered := triangles(row, gfx.ID{}, false); hovered <= rest {
		t.Errorf("hovering the row painted %d triangles, at rest %d: the pencil did not appear", hovered, rest)
	}
	if focused := triangles(gfx.ID{}, row, true); focused <= rest {
		t.Errorf("focusing the row painted %d triangles, at rest %d: the pencil did not appear", focused, rest)
	}
	if other := triangles(tree.ID(root, "other"), gfx.ID{}, false); other != rest {
		t.Errorf("hovering another element painted %d triangles, want the %d of rest", other, rest)
	}
}

func TestInteractiveDrawsItsRingOnlyForVisibleKeyboardFocus(t *testing.T) {
	theme := gfx.LightTheme()
	page := newPage(theme)
	canvas := canvasWith(gfx.ID{}, gfx.ID{}, false)
	draw(page, theme, canvas, func(c ui.C) {
		row := ui.Row(c).Size(80, 32).Radius(8).Hit(ui.ID(c), "x")
		Interactive(row, theme).Children(func() { ui.Text(c, "x") })
	})
	target := hitOf(t, canvas, "x")
	ringed := func(focusVisible bool) bool {
		canvas := canvasWith(gfx.ID{}, target, focusVisible)
		draw(page, theme, canvas, func(c ui.C) {
			row := ui.Row(c).Size(80, 32).Radius(8).Hit(ui.ID(c), "x")
			Interactive(row, theme).Children(func() { ui.Text(c, "x") })
		})
		for _, triangle := range canvas.Triangles() {
			if triangle.Color == ring(theme) {
				return true
			}
		}
		return false
	}
	if !ringed(true) {
		t.Error("a keyboard-focused element has no ring")
	}
	if ringed(false) {
		t.Error("a clicked element has a ring")
	}
}

func TestAvatarDotTakesNoRoom(t *testing.T) {
	theme := gfx.LightTheme()
	page := newPage(theme)
	var with, without ui.Box
	draw(page, theme, canvasWith(gfx.ID{}, gfx.ID{}, false), func(c ui.C) {
		with = Avatar(c, theme, 40, "AB", true)
		without = Avatar(c, theme, 40, "AB", false)
	})
	if with.Rect().Width != 40 || with.Rect().Height != 40 || without.Rect().Height != 40 {
		t.Errorf("avatars are %+v and %+v, want 40 square", with.Rect(), without.Rect())
	}
	if gap := without.Rect().Y - with.Rect().Bottom(); gap != 12 {
		t.Errorf("the dot moved the next element: gap %v, want the column's 12", gap)
	}
}

func TestDisabledIconButtonIsInert(t *testing.T) {
	theme := gfx.LightTheme()
	page := newPage(theme)
	canvas := canvasWith(gfx.ID{}, gfx.ID{}, false)
	draw(page, theme, canvas, func(c ui.C) { IconButton(c, theme, glyph, "Search", true) })
	target := hitOf(t, canvas, "Search")
	for _, hit := range canvas.Hits() {
		if hit.ID == target {
			if !hit.Disabled {
				t.Error("a disabled icon button is not disabled")
			}
			return
		}
	}
	t.Error("a disabled icon button has no hit region to name it")
}

func TestASwitchedOffSectionFadesTheControlsInIt(t *testing.T) {
	theme := gfx.LightTheme()
	page := newPage(theme)
	fill := func(enabled bool) float32 {
		canvas := canvasWith(gfx.ID{}, gfx.ID{}, false)
		var save ui.Box
		draw(page, theme, canvas, func(c ui.C) {
			Section(c, theme, "General", enabled, func() { save = ui.Button(c, "Save") })
		})
		for _, rectangle := range canvas.Rectangles() {
			if rectangle.Bounds.X == save.Rect().X && rectangle.Bounds.Y == save.Rect().Y {
				return rectangle.Color.Alpha
			}
		}
		t.Fatal("the button painted no fill")
		return 0
	}
	if on, off := fill(true), fill(false); !(off > 0 && off < on) {
		t.Errorf("the button's fill has alpha %v when enabled and %v when switched off, want less", on, off)
	}
}

func TestRecipesReportClicks(t *testing.T) {
	theme := gfx.LightTheme()
	page := newPage(theme)
	canvas := canvasWith(gfx.ID{}, gfx.ID{}, false)
	draw(page, theme, canvas, func(c ui.C) {
		if Hero(c, theme, "t", "x", "Open").Clicked() {
			t.Error("a click was reported with none delivered")
		}
	})
}

func TestAWarmRecipePageAllocatesNothing(t *testing.T) {
	theme := gfx.LightTheme()
	page := newPage(theme)
	var on bool
	canvas := canvasWith(gfx.ID{}, gfx.ID{}, false)
	run := func() {
		canvas.Reset()
		canvas.BeginFrame()
		page.Draw(canvas, gfx.Rect{Width: 360, Height: 800}, func(c ui.C) {
			ui.Column(c).Pad(16).Gap(12).Children(func() {
				SettingsRow(c, theme, "Notifications", "Tell me when a transfer finishes.", &on)
				ListRow(c, theme, glyph, glyph, "host", "ssh", true)
				Avatar(c, theme, 40, "AB", true)
				Badge(c, theme, ToneAccent, "New")
				StatCard(c, theme, "12", "Sessions", gfx.RGB(1, 2, 3), gfx.RGB(4, 5, 6))
				Hero(c, theme, "Hello", "A line of text.", "Open")
				IconButton(c, theme, glyph, "Search", false)
				Section(c, theme, "General", false, func() { ui.Text(c, "content") })
			})
		})
		canvas.EndFrame()
	}
	run()
	if allocs := testing.AllocsPerRun(100, run); allocs != 0 {
		t.Fatalf("a warm page of recipes allocates %v times a frame, want 0", allocs)
	}
}
