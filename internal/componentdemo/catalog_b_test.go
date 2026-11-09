package componentdemo

import (
	"fmt"
	"testing"
	"time"

	"github.com/vivaui/viva/gfx"
)

func TestGroupBCatalogOrder(t *testing.T) {
	expected := [...]string{
		"command", "context-menu", "drawer", "dropdown-menu", "empty",
		"field", "form", "hover-card", "icon-hover", "input-group", "input-otp",
		"item", "kbd", "label", "list", "markdown", "marker", "menubar", "message",
		"message-scroller", "metric-card", "multi-select", "native-select",
	}
	if len(groupBComponents) != len(expected) {
		t.Fatalf("group B contains %d components, want %d", len(groupBComponents), len(expected))
	}
	for index, id := range expected {
		component := groupBComponents[index]
		if component.ID != id {
			t.Fatalf("group B component %d = %q, want %q", index, component.ID, id)
		}
		if component.Title == "" || component.Category == "" || component.Summary == "" || component.paint == nil {
			t.Fatalf("group B component %q has incomplete metadata", component.ID)
		}
		if len(component.Examples) == 0 {
			t.Fatalf("group B component %q has no examples", component.ID)
		}
	}
}

func TestGroupBPaintsThemesResponsivelyAndDeterministically(t *testing.T) {
	sizes := [...]gfx.Size{{Width: 320, Height: 340}, {Width: 760, Height: 380}}
	for componentIndex := range groupBComponents {
		component := &groupBComponents[componentIndex]
		for _, example := range component.Examples {
			for _, dark := range [...]bool{false, true} {
				for _, size := range sizes {
					name := fmt.Sprintf("%s/%s/dark=%t/%.0fx%.0f", component.ID, example.ID, dark, size.Width, size.Height)
					t.Run(name, func(t *testing.T) {
						state := newGroupBState(component, example.ID)
						var canvas gfx.Canvas
						// A virtualized list lays its first frame out from estimates; the
						// steady frames are the ones that must not differ.
						paintGroupBFrame(&canvas, state, component, size, dark)
						paintGroupBFrame(&canvas, state, component, size, dark)
						if len(canvas.Texts()) == 0 && len(canvas.Rectangles()) <= 1 {
							t.Fatal("example emitted no visible component commands")
						}
						expected := snapshot(&canvas)
						paintGroupBFrame(&canvas, state, component, size, dark)
						assertSamePaint(t, expected, &canvas)
						if canvas.Animating() {
							t.Fatal("static example requested another frame")
						}
					})
				}
			}
		}
	}
}

func TestGroupBStaticWarmPaintAllocations(t *testing.T) {
	for componentIndex := range groupBComponents {
		component := &groupBComponents[componentIndex]
		for _, example := range component.Examples {
			t.Run(component.ID+"/"+example.ID, func(t *testing.T) {
				state := newGroupBState(component, example.ID)
				var canvas gfx.Canvas
				paint := func() {
					paintGroupBFrame(&canvas, state, component, gfx.Size{Width: 760, Height: 380}, false)
				}
				paint()
				if allocations := testing.AllocsPerRun(100, paint); allocations != 0 {
					t.Fatalf("static warm paint allocated %v times", allocations)
				}
			})
		}
	}
}

func newGroupBState(component *Component, exampleID string) *State {
	state := bareState(component.ID, exampleID)
	if component.initialize != nil {
		component.initialize(state)
	}
	return state
}

func paintGroupBFrame(canvas *gfx.Canvas, state *State, component *Component, size gfx.Size, dark bool) {
	canvas.SetReduceMotion(true)
	canvas.SetClock(func() time.Duration { return time.Second })
	canvas.BeginFrame()
	canvas.Reset()
	theme := gfx.LightTheme()
	if dark {
		theme = gfx.DarkTheme()
	}
	viewport := gfx.Rect{Width: size.Width, Height: size.Height}
	canvas.Clip(viewport)
	canvas.Fill(viewport, theme.Background)
	component.paint(state, canvas, theme, size)
	canvas.Unclip()
	canvas.EndFrame()
}

// The group B controls change their demo's state through Clicked, in the event pass.
func TestGroupBClicksChangeState(t *testing.T) {
	var canvas gfx.Canvas
	for _, test := range []struct {
		component, example string
		id                 gfx.ID
		changed            func(*State) bool
	}{
		{"command", "default", commandItem.At(2), func(s *State) bool { return s.values[0] == 2 }},
		{"dropdown-menu", "default", dropdownMenuToggle, func(s *State) bool { return s.flags&1 != 0 }},
		{"item", "default", itemRow.At(0), func(s *State) bool { return s.flags&1 != 0 }},
		{"native-select", "default", nativeSelectTrigger.At(0), func(s *State) bool { return s.flags&1 != 0 }},
		{"message-scroller", "default", messageScrollerEnd, func(s *State) bool { return s.values[0] == 72 }},
		{"menubar", "default", menubarFile, func(s *State) bool { return s.flags&1 != 0 }},
		{"input-otp", "default", inputOTPField, func(s *State) bool { return s.values[0] == 4 }},
		{"input-group", "default", inputGroupClear, func(s *State) bool { return s.flags&1 == 0 }},
		{"drawer", "default", drawerClose, func(s *State) bool { return s.flags&1 == 0 }},
	} {
		state := newTestState(t, test.component, test.example)
		paintFrame(&canvas, state, testSize, time.Second, true)
		click(&canvas, state, test.id)
		if !test.changed(state) {
			t.Errorf("%s: clicking its control did not change the state", test.component)
		}
	}
}
