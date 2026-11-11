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
		{"scroll-area", "both-axes", []step{{id: scrollAreaHorizontal, want: func(s *State) bool { return s.values[1] == 72 }}}},
		{"segmented-control", "default", []step{{id: segmentedControlView.At(2), want: func(s *State) bool { return s.values[0] == 2 }}}},
		{"select", "default", []step{
			{id: selectTrigger, want: func(s *State) bool { return s.flags&1 != 0 }},
			{id: selectItem.At(1), want: func(s *State) bool { return s.values[0] == 2 && s.flags&1 == 0 }},
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
		{"sonner", "stack", []step{{id: sonnerDismiss.At(1), want: func(s *State) bool { return s.flags&0b010 == 0 && s.flags&0b101 == 0b101 }}}},
		{"table", "selection", []step{
			{id: tableRow.At(1), want: func(s *State) bool { return s.flags&2 != 0 }},
			{id: tableRow.At(1), want: func(s *State) bool { return s.flags&2 == 0 }},
		}},
		{"tabs", "line", []step{{label: "活动", want: func(s *State) bool { return s.values[0] == 1 }}}},
		{"toast", "types", []step{
			{id: toastAction, want: func(s *State) bool { return s.values[0] == 1 }},
			{id: toastDismiss, want: func(s *State) bool { return s.flags&1 == 0 }},
		}},
		{"toggle", "default", []step{{id: toggleItem.At(2), want: func(s *State) bool { return s.flags&4 != 0 }}}},
		{"toggle-group", "single", []step{{id: toggleGroupItem.At(2), want: func(s *State) bool { return s.flags == 4 }}}},
		{"tooltip", "default", []step{{id: tooltipTrigger, want: func(s *State) bool { return s.flags&1 != 0 }}}},
		{"tree", "selection", []step{
			{id: treeLeaf.At(2), want: func(s *State) bool { return s.values[0] == 2 }},
			{id: treeGroupDisclosure, want: func(s *State) bool { return s.flags&1 == 0 }},
		}},
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
