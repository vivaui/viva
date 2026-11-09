package componentdemo

import (
	"slices"
	"testing"
	"time"

	"github.com/vivaui/viva/gfx"
)

type groupAPaintSnapshot struct {
	rectangles []gfx.Rectangle
	texts      []gfx.Text
	hits       []gfx.Hit
	scope      gfx.ID
	escape     gfx.ID
}

func newGroupAState(component Component, exampleID string) *State {
	state := bareState(component.ID, exampleID)
	if component.initialize != nil {
		component.initialize(state)
	}
	return state
}

func paintGroupAFrame(canvas *gfx.Canvas, component Component, state *State, theme gfx.Theme, size gfx.Size) {
	canvas.SetReduceMotion(true)
	canvas.BeginFrame()
	canvas.Reset()
	component.paint(state, canvas, theme, size)
	canvas.EndFrame()
}

func snapshotGroupAPaint(canvas *gfx.Canvas) groupAPaintSnapshot {
	return groupAPaintSnapshot{
		rectangles: slices.Clone(canvas.Rectangles()),
		texts:      slices.Clone(canvas.Texts()),
		hits:       slices.Clone(canvas.Hits()),
		scope:      canvas.TrappedFocusScope(),
		escape:     canvas.EscapeTarget(),
	}
}

func TestGroupACatalogOrder(t *testing.T) {
	want := []string{
		"accordion", "alert", "alert-dialog", "aspect-ratio", "attachment",
		"avatar", "breadcrumb", "bubble", "button-group", "calendar", "card",
		"carousel", "chart", "chart-area", "chart-bar", "chart-downsample", "collapsible", "combobox",
	}
	if len(groupAComponents) != len(want) {
		t.Fatalf("group A has %d components, want %d", len(groupAComponents), len(want))
	}
	for index, component := range groupAComponents {
		if component.ID != want[index] {
			t.Fatalf("group A component %d = %q, want %q", index, component.ID, want[index])
		}
		if component.paint == nil || len(component.Examples) == 0 {
			t.Fatalf("component %q has no paint function or examples", component.ID)
		}
		if component.Title == "" || component.Category == "" || component.Summary == "" {
			t.Fatalf("component %q has incomplete metadata", component.ID)
		}
	}
}

func TestGroupAPaintDeterminism(t *testing.T) {
	sizes := []gfx.Size{{Width: 320, Height: 340}, {Width: 760, Height: 380}}
	themes := []struct {
		name  string
		theme gfx.Theme
	}{
		{name: "light", theme: gfx.LightTheme()},
		{name: "dark", theme: gfx.DarkTheme()},
	}
	for _, component := range groupAComponents {
		for _, example := range component.Examples {
			for _, theme := range themes {
				for _, size := range sizes {
					name := component.ID + "/" + example.ID + "/" + theme.name
					if size.Width > 320 {
						name += "/wide"
					} else {
						name += "/narrow"
					}
					t.Run(name, func(t *testing.T) {
						state := newGroupAState(component, example.ID)
						before := *state
						var canvas gfx.Canvas
						paintGroupAFrame(&canvas, component, state, theme.theme, size)
						if len(canvas.Rectangles()) == 0 && len(canvas.Texts()) == 0 {
							t.Fatal("paint produced no visible commands")
						}
						if *state != before {
							t.Fatal("paint mutated component state")
						}
						if canvas.Animating() {
							t.Fatal("static reduced-motion paint requested another frame")
						}
						for _, hit := range canvas.Hits() {
							if !hit.Block && !hit.Container && hit.ID.IsZero() {
								t.Fatal("interactive command used reserved ID 0")
							}
						}
						first := snapshotGroupAPaint(&canvas)
						paintGroupAFrame(&canvas, component, state, theme.theme, size)
						if !slices.Equal(first.rectangles, canvas.Rectangles()) ||
							!slices.Equal(first.texts, canvas.Texts()) ||
							!slices.Equal(first.hits, canvas.Hits()) ||
							first.scope != canvas.TrappedFocusScope() || first.escape != canvas.EscapeTarget() {
							t.Fatal("identical state produced different paint or interaction streams")
						}
					})
				}
			}
		}
	}
}

func TestGroupAStaticWarmPaintAllocations(t *testing.T) {
	for _, component := range groupAComponents {
		for _, example := range component.Examples {
			t.Run(component.ID+"/"+example.ID, func(t *testing.T) {
				state := newGroupAState(component, example.ID)
				var canvas gfx.Canvas
				paint := func() {
					paintGroupAFrame(&canvas, component, state, gfx.LightTheme(), gfx.Size{Width: 760, Height: 380})
				}
				paint()
				if allocations := testing.AllocsPerRun(100, paint); allocations != 0 {
					t.Fatalf("static warm paint allocated %v times", allocations)
				}
			})
		}
	}
}

// Group A reads its clicks while it paints, and each control changes the state
// the demo says it does.
func TestGroupAClicksChangeCallerState(t *testing.T) {
	cases := []struct {
		component, example string
		target             gfx.ID
		changed            func(*State) bool
	}{
		{"accordion", "default", accordionItem.At(1), func(s *State) bool { return s.flags == 3 }},
		{"accordion", "disabled", accordionItem.At(1), func(s *State) bool { return s.flags == 1 }},
		{"alert-dialog", "default", alertDialogAction, func(s *State) bool { return s.flags&alertDialogOpenFlag == 0 }},
		{"alert-dialog", "media", alertDialogCancel, func(s *State) bool { return s.flags&alertDialogOpenFlag == 0 }},
		{"attachment", "default", attachmentItem.At(0), func(s *State) bool { return s.values[0] == 1 }},
		{"breadcrumb", "default", breadcrumbLink.At(0), func(s *State) bool { return s.values[0] == 1 }},
		{"bubble", "conversation", bubbleReaction, func(s *State) bool { return s.flags == 1 }},
		{"button-group", "horizontal", buttonGroupItem.At(2), func(s *State) bool { return s.values[0] == 2 }},
		{"calendar", "month", calendarNext, func(s *State) bool { return s.values[0] == 1 }},
		{"carousel", "horizontal", carouselNext, func(s *State) bool { return s.values[0] == 1 }},
		{"carousel", "vertical", carouselNext, func(s *State) bool { return s.values[0] == 1 }},
		{"collapsible", "default", collapsibleTrigger, func(s *State) bool { return s.flags == 0 }},
		{"combobox", "default", comboboxOption.At(1), func(s *State) bool { return s.values[0] == 2 && s.flags&comboboxOpenFlag == 0 }},
		{"combobox", "default", comboboxInput, func(s *State) bool { return s.flags&comboboxOpenFlag == 0 }},
	}
	for _, test := range cases {
		t.Run(test.component+"/"+test.example, func(t *testing.T) {
			state := newTestState(t, test.component, test.example)
			var canvas gfx.Canvas
			paintFrame(&canvas, state, testSize, time.Second, true)
			findHit(t, &canvas, test.target)
			click(&canvas, state, test.target)
			if !test.changed(state) {
				t.Fatalf("clicking %v did not change the state: flags %b values %v", test.target, state.flags, state.values)
			}
		})
	}
}
