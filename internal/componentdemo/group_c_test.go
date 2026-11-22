package componentdemo

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

// Each step is a click on a control the demo declares, and what the caller-owned
// state must be after it.
func TestGroupCClicksUpdateCallerState(t *testing.T) {
	type step struct {
		id    gfx.ID
		label string // when set, the step finds its control by accessibility label
		want  func(*State) bool
	}
	tests := []struct {
		component, example string
		steps              []step
	}{
		{"navigation-menu", "default", []step{
			{id: navigationMenuTrigger, want: func(s *State) bool { return s.flags&1 != 0 }},
			{id: navigationMenuAPI, want: func(s *State) bool { return s.values[0] == 3 && s.flags&1 == 0 }},
		}},
		{"pagination", "default", []step{
			{id: paginationNext, want: func(s *State) bool { return s.values[0] == 4 }},
			{id: paginationPage.At(4), want: func(s *State) bool { return s.values[0] == 8 }},
			{id: paginationPrevious, want: func(s *State) bool { return s.values[0] == 7 }},
		}},
		{"popover", "custom-body", []step{
			{id: popoverTrigger, want: func(s *State) bool { return s.flags&1 != 0 }},
			{id: popoverAction, want: func(s *State) bool { return s.flags&2 != 0 }},
			{id: popoverDismiss, want: func(s *State) bool { return s.flags&1 == 0 }},
		}},
		{"questionnaire", "multiple-choice", []step{
			{id: questionnaireChoice.At(1), want: func(s *State) bool { return s.flags&2 != 0 }},
			{id: questionnaireNext, want: func(s *State) bool { return s.values[1] == 1 }},
		}},
		{"radio-group", "vertical", []step{{id: radioGroupItem.At(0), want: func(s *State) bool { return s.values[0] == 0 }}}},
		{"segmented-control", "default", []step{{label: "Timeline", want: func(s *State) bool { return s.values[0] == 2 }}}},
		{"select", "default", []step{
			{label: "Renderer", want: func(s *State) bool { return s.values[0] == 0 }},
			{label: "Direct3D", want: func(s *State) bool { return s.values[0] == 2 }},
		}},
		{"sheet", "right", []step{
			{id: sheetTrigger, want: func(s *State) bool { return s.flags&1 != 0 }},
			{id: sheetSave, want: func(s *State) bool { return s.flags&1 == 0 }},
		}},
		{"sidebar", "collapsed", []step{
			{id: sidebarItem.At(2), want: func(s *State) bool { return s.values[0] == 2 }},
			{id: sidebarCollapse, want: func(s *State) bool { return s.flags&1 != 0 }},
		}},
		{"slider", "default", []step{{id: sliderPrimary, want: func(s *State) bool { return s.values[0] == 50 }}}},
		{"sonner", "stack", []step{{label: "Close toast", want: func(s *State) bool { return s.flags == 0b011 }}}},
		{"table", "selection", []step{
			{label: "桌面构建", want: func(s *State) bool { return s.flags&2 != 0 }},
			{label: "性能审计", want: func(s *State) bool { return s.flags&4 != 0 }},
		}},
		{"tabs", "line", []step{{label: "活动", want: func(s *State) bool { return s.values[0] == 1 }}}},
		{"toast", "types", []step{
			{label: "切换", want: func(s *State) bool { return s.values[0] == 1 }},
			{label: "Close toast", want: func(s *State) bool { return s.flags == 0 }},
		}},
		{"toggle", "default", []step{{id: toggleItem.At(2), want: func(s *State) bool { return s.flags&4 != 0 }}}},
		{"toggle-group", "single", []step{{id: toggleGroupItem.At(2), want: func(s *State) bool { return s.flags == 4 }}}},
	}
	for _, test := range tests {
		t.Run(test.component+"/"+test.example, func(t *testing.T) {
			state := newTestState(t, test.component, test.example)
			var canvas gfx.Canvas
			canvas.SetMeasurer(measureTestText)
			size := testSize
			if test.component == "pagination" {
				size = gfx.Size{Width: 760, Height: 380} // the narrow stage shows only the current page
			}
			for index, step := range test.steps {
				paintFrame(&canvas, state, size, 0, true)
				target := step.id
				if step.label != "" {
					target = findHitByLabel(t, &canvas, step.label)
				}
				if target != popoverDismiss { // the backdrop has no hit of its own to find
					findHit(t, &canvas, target) // the demo declares the control it reads the click on
				}
				canvas.BeginEventPass(target, gfx.ClickInfo{})
				paintFrame(&canvas, state, size, 0, true)
				canvas.EndEventPass()
				if !step.want(state) {
					t.Fatalf("step %d: click on %v left flags=%b values=%v", index, target, state.flags, state.values)
				}
			}
		})
	}
	// The page Tree owns its row identities: choose a leaf by label, then collapse
	// the branch through the disclosure identity the state reports.
	t.Run("tree/selection", func(t *testing.T) {
		state := newTestState(t, "tree", "selection")
		var canvas gfx.Canvas
		canvas.SetMeasurer(measureTestText)
		paintFrame(&canvas, state, testSize, 0, true)
		click(&canvas, state, findHitByLabel(t, &canvas, "Worker"))
		if state.values[0] != 2 {
			t.Fatalf("choosing Worker left values[0]=%d", state.values[0])
		}
		paintFrame(&canvas, state, testSize, 0, true)
		click(&canvas, state, treeDemoFor(state).state.DisclosureID(0))
		if state.flags&1 != 0 {
			t.Fatalf("collapsing Production left flags=%b", state.flags)
		}
	})
	// The page Scroll owns its offset: a sideways wheel over the area moves it and
	// the demo mirrors the settled offset back into its values.
	t.Run("scroll-area/both-axes", func(t *testing.T) {
		state := newTestState(t, "scroll-area", "both-axes")
		var canvas gfx.Canvas
		canvas.SetMeasurer(measureTestText)
		paintFrame(&canvas, state, testSize, 0, true)
		demo := scrollAreaDemoFor(state)
		if demo.scroller == nil || demo.bounds.Width <= 0 {
			t.Fatal("the scroll area was not declared")
		}
		result := state.Scroll(gfx.ScrollEvent{
			X: demo.bounds.X + demo.bounds.Width/2, Y: demo.bounds.Y + demo.bounds.Height/2,
			DeltaX: 72, Precise: true, Target: scrollAreaViewport,
		})
		if !result.Handled || !result.Repaint || demo.scroller.OffsetX != 72 {
			t.Fatalf("wheel result=%+v offsetX=%.0f", result, demo.scroller.OffsetX)
		}
		paintFrame(&canvas, state, testSize, 0, true)
		if state.values[1] != 72 {
			t.Fatalf("mirrored offsetX=%d, want 72", state.values[1])
		}
	})
}

// findHitByLabel finds the control of the canvas with the accessibility label,
// for a demo whose Page names a control by its place instead of a stable tree ID.
func findHitByLabel(t testing.TB, canvas *gfx.Canvas, label string) gfx.ID {
	t.Helper()
	for _, hit := range canvas.Hits() {
		if hit.Label == label && !hit.IsLabel && !hit.Backdrop {
			return hit.ID
		}
	}
	t.Fatalf("missing hit labelled %q", label)
	return gfx.ID{}
}
