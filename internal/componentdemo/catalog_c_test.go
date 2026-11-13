package componentdemo

import (
	"fmt"
	"slices"
	"testing"

	"github.com/vivaui/viva/gfx"
)

type groupCPaintSnapshot struct {
	rectangles []gfx.Rectangle
	texts      []gfx.Text
	hits       []gfx.Hit
	scope      gfx.ID
	escape     gfx.ID
}

func TestGroupCCatalogOrder(t *testing.T) {
	expected := [...]string{
		"column-resize", "navigation-menu", "pagination", "popover", "progress", "questionnaire",
		"radio-group", "resizable", "scroll-area", "segmented-control", "select", "separator",
		"sheet", "sidebar", "single-select", "skeleton", "slider", "sonner", "spinner", "table", "tabs",
		"textarea", "toast", "toggle", "toggle-group", "tooltip", "tree", "tree-navigation", "typography",
	}
	if len(groupCComponents) != len(expected) {
		t.Fatalf("group C has %d components, want %d", len(groupCComponents), len(expected))
	}
	for index, component := range groupCComponents {
		if component.ID != expected[index] {
			t.Fatalf("component %d = %q, want %q", index, component.ID, expected[index])
		}
		if component.Title == "" || component.Category == "" || component.Summary == "" || component.paint == nil {
			t.Fatalf("incomplete metadata for %q: %+v", component.ID, component)
		}
		if len(component.Examples) == 0 {
			t.Fatalf("component %q has no examples", component.ID)
		}
		seen := make([]string, 0, len(component.Examples))
		for _, example := range component.Examples {
			if example.ID == "" || example.Title == "" || example.Description == "" {
				t.Fatalf("incomplete example metadata for %q: %+v", component.ID, example)
			}
			if slices.Contains(seen, example.ID) {
				t.Fatalf("duplicate example %q for %q", example.ID, component.ID)
			}
			seen = append(seen, example.ID)
		}
	}
}

func TestGroupCPaintLightDarkVisibleAndDeterministic(t *testing.T) {
	sizes := [...]gfx.Size{{Width: 320, Height: 340}, {Width: 760, Height: 380}}
	for _, component := range groupCComponents {
		component := component
		for _, example := range component.Examples {
			example := example
			for _, dark := range []bool{false, true} {
				for _, size := range sizes {
					name := fmt.Sprintf("%s/%s/dark=%t/%dx%d", component.ID, example.ID, dark, int(size.Width), int(size.Height))
					t.Run(name, func(t *testing.T) {
						state := bareState(component.ID, example.ID)
						state.dark = dark
						if component.initialize != nil {
							component.initialize(state)
						}
						// The Page caches the appearance it paints with, so the
						// state after a paint holds it too. Paint once before the
						// comparison: the first paint makes the Page.
						state.theme = state.themeFor()
						var canvas gfx.Canvas
						canvas.SetMeasurer(measureTestText)
						paintGroupCFrame(&canvas, &component, state, size, dark)
						before := *state
						paintGroupCFrame(&canvas, &component, state, size, dark)
						if len(canvas.Rectangles()) == 0 && len(canvas.Texts()) == 0 {
							t.Fatal("paint produced no visible commands")
						}
						if canvas.Animating() {
							t.Fatal("static reduced-motion paint remained animated")
						}
						if *state != before {
							t.Fatal("paint mutated component state")
						}
						for _, hit := range canvas.Hits() {
							// Passive popup blockers and semantic containers do not
							// dispatch actions and intentionally have no control ID.
							if hit.ID.IsZero() && !hit.Block && !hit.Container && !hit.IsLabel {
								t.Fatal("interactive hit used reserved ID 0")
							}
						}
						expected := groupCSnapshot(&canvas)
						paintGroupCFrame(&canvas, &component, state, size, dark)
						assertSameGroupCPaint(t, expected, &canvas)
					})
				}
			}
		}
	}
}

func paintGroupCFrame(canvas *gfx.Canvas, component *Component, state *State, size gfx.Size, dark bool) {
	theme := gfx.LightTheme()
	if dark {
		theme = gfx.DarkTheme()
	}
	canvas.SetReduceMotion(true)
	canvas.BeginFrame()
	canvas.Reset()
	component.paint(state, canvas, theme, size)
	canvas.EndFrame()
}

func groupCSnapshot(canvas *gfx.Canvas) groupCPaintSnapshot {
	return groupCPaintSnapshot{
		rectangles: slices.Clone(canvas.Rectangles()),
		texts:      slices.Clone(canvas.Texts()),
		hits:       slices.Clone(canvas.Hits()),
		scope:      canvas.TrappedFocusScope(),
		escape:     canvas.EscapeTarget(),
	}
}

func assertSameGroupCPaint(t testing.TB, expected groupCPaintSnapshot, canvas *gfx.Canvas) {
	t.Helper()
	if !slices.Equal(expected.rectangles, canvas.Rectangles()) ||
		!slices.Equal(expected.texts, canvas.Texts()) ||
		!slices.Equal(expected.hits, canvas.Hits()) ||
		expected.scope != canvas.TrappedFocusScope() || expected.escape != canvas.EscapeTarget() {
		t.Fatal("identical state produced different paint or interaction streams")
	}
}

func TestTreeNavigationPreviewMovesFocusAndHidesCollapsedChildren(t *testing.T) {
	state := newTestState(t, "tree-navigation", "default")
	root, child := treeNavigationRows.At(0), treeNavigationRows.At(1)
	var canvas gfx.Canvas
	paintAt(&canvas, state, testSize)
	if hasText(&canvas, "Gateway") {
		t.Fatal("collapsed preview paints a child")
	}
	step := func(target gfx.ID, key gfx.Key, want gfx.ID) {
		t.Helper()
		result := state.Key(gfx.KeyEvent{Target: target, Key: key})
		if !result.Handled || !result.Repaint || !result.FocusVisible || result.Focus != want {
			t.Fatalf("key %v: result=%+v, want focus %v and repaint", key, result, want)
		}
		paintAt(&canvas, state, testSize)
		if !findHit(t, &canvas, want).Selected {
			t.Fatalf("key %v: the focused row is not selected", key)
		}
	}
	step(root, gfx.KeyRight, root)
	step(root, gfx.KeyRight, child)
	step(child, gfx.KeyLeft, root)
	step(root, gfx.KeyLeft, root)
	if hasText(&canvas, "Gateway") {
		t.Fatal("collapsing from the keyboard did not remove children")
	}
}
