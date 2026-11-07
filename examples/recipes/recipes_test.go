package recipes

import (
	"testing"

	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/ui"
)

var (
	tree = ui.Tree{}
	page = tree.Feature("recipes")
	one  = tree.ID(page, "one")
	two  = tree.ID(page, "two")

	glyph = ui.Icon{ViewBox: 24, Stroke: 2, Cap: ui.CapRound, Commands: []ui.PathCommand{ui.MoveTo(4, 12), ui.LineTo(20, 12)}}
)

var themes = map[string]ui.Theme{"light": ui.LightTheme(), "dark": ui.DarkTheme()}

func canvasWith(hovered, focused ui.ID, focusVisible bool) *ui.Canvas {
	canvas := &ui.Canvas{}
	canvas.SetTree(&tree)
	canvas.SetMeasurer(func(value string, size, weight float32) float32 { return float32(len(value)) * size / 2 })
	canvas.SetInteraction(hovered, ui.ID{}, focused, focusVisible)
	return canvas
}

// draw declares build into a 360 wide page and returns what it painted.
func draw(theme ui.Theme, canvas *ui.Canvas, build func(f *flex.Frame)) {
	var view flex.View
	view.Draw(canvas, theme, ui.Rect{Width: 360, Height: 480}, func(f *flex.Frame) {
		f.Column().Pad(16).Gap(12).Children(func() { build(f) })
	})
}

// Every recipe paints something and stays inside the room it is given, in both
// themes: a recipe that overflows its column or paints nothing is broken
// whatever it looks like.
func TestEveryRecipePaintsInsideItsRoom(t *testing.T) {
	recipes := map[string]func(f *flex.Frame, theme ui.Theme) flex.Box{
		"settings row": func(f *flex.Frame, theme ui.Theme) flex.Box {
			return SettingsRow(f, theme, one, "Notifications", "Tell me when a long transfer finishes, even while the window is closed.", true)
		},
		"list row": func(f *flex.Frame, theme ui.Theme) flex.Box {
			return ListRow(f, theme, one, glyph, glyph, "production-database-primary-replica-eu-west-1", "ssh", false)
		},
		"avatar": func(f *flex.Frame, theme ui.Theme) flex.Box { return Avatar(f, theme, 40, "AB", true) },
		"badge":  func(f *flex.Frame, theme ui.Theme) flex.Box { return Badge(f, theme, ToneDanger, "Failed") },
		"stat": func(f *flex.Frame, theme ui.Theme) flex.Box {
			return StatCard(f, theme, "1,284", "Active sessions", ui.RGB(99, 102, 241), ui.RGB(236, 72, 153))
		},
		"hero": func(f *flex.Frame, theme ui.Theme) flex.Box {
			return Hero(f, theme, one, "Welcome back", "Pick up where you left off on any of your hosts.", "Open")
		},
		"icon button": func(f *flex.Frame, theme ui.Theme) flex.Box { return IconButton(f, theme, one, glyph, "Search", false) },
		"empty state": func(f *flex.Frame, theme ui.Theme) flex.Box {
			return EmptyState(f, theme, one, glyph, "No hosts yet", "Add a host to connect to it from here.", "Add host")
		},
		"section": func(f *flex.Frame, theme ui.Theme) flex.Box {
			return Section(f, theme, "General", true, func() { f.Text("content") })
		},
	}
	for name, recipe := range recipes {
		for themeName, theme := range themes {
			t.Run(name+"/"+themeName, func(t *testing.T) {
				canvas := canvasWith(ui.ID{}, ui.ID{}, false)
				var box flex.Box
				draw(theme, canvas, func(f *flex.Frame) { box = recipe(f, theme) })
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
	theme := ui.LightTheme()
	triangles := func(hovered, focused ui.ID, visible bool) int {
		canvas := canvasWith(hovered, focused, visible)
		draw(theme, canvas, func(f *flex.Frame) {
			ListRow(f, theme, one, ui.Icon{ViewBox: 24}, glyph, "host", "ssh", false)
		})
		return len(canvas.Triangles())
	}
	rest := triangles(ui.ID{}, ui.ID{}, false)
	if hovered := triangles(one, ui.ID{}, false); hovered <= rest {
		t.Errorf("hovering the row painted %d triangles, at rest %d: the pencil did not appear", hovered, rest)
	}
	if focused := triangles(ui.ID{}, one, true); focused <= rest {
		t.Errorf("focusing the row painted %d triangles, at rest %d: the pencil did not appear", focused, rest)
	}
	if other := triangles(two, ui.ID{}, false); other != rest {
		t.Errorf("hovering another element painted %d triangles, want the %d of rest", other, rest)
	}
}

func TestInteractiveDrawsItsRingOnlyForVisibleKeyboardFocus(t *testing.T) {
	theme := ui.LightTheme()
	ringed := func(focusVisible bool) bool {
		canvas := canvasWith(ui.ID{}, one, focusVisible)
		draw(theme, canvas, func(f *flex.Frame) {
			Interactive(f.Row().Size(80, 32).Radius(8), theme, one, "x").Children(func() { f.Text("x") })
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
	theme := ui.LightTheme()
	var with, without flex.Box
	draw(theme, canvasWith(ui.ID{}, ui.ID{}, false), func(f *flex.Frame) {
		with = Avatar(f, theme, 40, "AB", true)
		without = Avatar(f, theme, 40, "AB", false)
	})
	if with.Rect().Width != 40 || with.Rect().Height != 40 || without.Rect().Height != 40 {
		t.Errorf("avatars are %+v and %+v, want 40 square", with.Rect(), without.Rect())
	}
	if gap := without.Rect().Y - with.Rect().Bottom(); gap != 12 {
		t.Errorf("the dot moved the next element: gap %v, want the column's 12", gap)
	}
}

func TestDisabledIconButtonIsInert(t *testing.T) {
	theme := ui.LightTheme()
	canvas := canvasWith(one, ui.ID{}, false)
	draw(theme, canvas, func(f *flex.Frame) { IconButton(f, theme, one, glyph, "Search", true) })
	found := false
	for _, hit := range canvas.Hits() {
		if hit.ID == one {
			found = true
			if !hit.Disabled {
				t.Error("a disabled icon button is not disabled")
			}
		}
	}
	if !found {
		t.Error("a disabled icon button has no hit region to name it")
	}
}

func TestASwitchedOffSectionFadesTheControlsInIt(t *testing.T) {
	theme := ui.LightTheme()
	fill := func(enabled bool) float32 {
		canvas := canvasWith(ui.ID{}, ui.ID{}, false)
		var save flex.Box
		draw(theme, canvas, func(f *flex.Frame) {
			Section(f, theme, "General", enabled, func() { save = f.Button(two, "Save") })
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
	theme := ui.LightTheme()
	canvas := canvasWith(ui.ID{}, ui.ID{}, false)
	draw(theme, canvas, func(f *flex.Frame) {
		if Hero(f, theme, one, "t", "x", "Open").Clicked() {
			t.Error("a click was reported with none delivered")
		}
	})
}

func TestAWarmRecipePageAllocatesNothing(t *testing.T) {
	theme := ui.LightTheme()
	canvas := canvasWith(one, ui.ID{}, false)
	var view flex.View
	run := func() {
		canvas.Reset()
		view.Draw(canvas, theme, ui.Rect{Width: 360, Height: 800}, func(f *flex.Frame) {
			f.Column().Pad(16).Gap(12).Children(func() {
				SettingsRow(f, theme, one, "Notifications", "Tell me when a transfer finishes.", true)
				ListRow(f, theme, two, glyph, glyph, "host", "ssh", true)
				Avatar(f, theme, 40, "AB", true)
				Badge(f, theme, ToneAccent, "New")
				StatCard(f, theme, "12", "Sessions", ui.RGB(1, 2, 3), ui.RGB(4, 5, 6))
				Hero(f, theme, one, "Hello", "A line of text.", "Open")
				IconButton(f, theme, two, glyph, "Search", false)
				Section(f, theme, "General", false, func() { f.Text("content") })
			})
		})
	}
	run()
	if allocs := testing.AllocsPerRun(100, run); allocs != 0 {
		t.Fatalf("a warm page of recipes allocates %v times a frame, want 0", allocs)
	}
}
