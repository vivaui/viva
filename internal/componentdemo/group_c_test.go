package componentdemo

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

// Each step is a click on a control the demo declares, and what the caller-owned
// state must be after it.
func TestGroupCClicksUpdateCallerState(t *testing.T) {
	type step struct {
		id   gfx.ID
		want func(*State) bool
	}
	tests := []struct {
		component, example string
		steps              []step
	}{
		{"navigation-menu", "default", []step{
			{navigationMenuTrigger, func(s *State) bool { return s.flags&1 != 0 }},
			{navigationMenuAPI, func(s *State) bool { return s.values[0] == 3 && s.flags&1 == 0 }},
		}},
		{"pagination", "default", []step{
			{paginationNext, func(s *State) bool { return s.values[0] == 4 }},
			{paginationPage.At(4), func(s *State) bool { return s.values[0] == 8 }},
			{paginationPrevious, func(s *State) bool { return s.values[0] == 7 }},
		}},
		{"popover", "custom-body", []step{
			{popoverTrigger, func(s *State) bool { return s.flags&1 != 0 }},
			{popoverAction, func(s *State) bool { return s.flags&2 != 0 }},
			{popoverDismiss, func(s *State) bool { return s.flags&1 == 0 }},
		}},
		{"questionnaire", "multiple-choice", []step{
			{questionnaireChoice.At(1), func(s *State) bool { return s.flags&2 != 0 }},
			{questionnaireNext, func(s *State) bool { return s.values[1] == 1 }},
		}},
		{"radio-group", "vertical", []step{{radioGroupItem.At(0), func(s *State) bool { return s.values[0] == 0 }}}},
		{"scroll-area", "both-axes", []step{{scrollAreaHorizontal, func(s *State) bool { return s.values[1] == 72 }}}},
		{"segmented-control", "default", []step{{segmentedControlView.At(2), func(s *State) bool { return s.values[0] == 2 }}}},
		{"select", "default", []step{
			{selectTrigger, func(s *State) bool { return s.flags&1 != 0 }},
			{selectItem.At(1), func(s *State) bool { return s.values[0] == 2 && s.flags&1 == 0 }},
		}},
		{"sheet", "right", []step{
			{sheetTrigger, func(s *State) bool { return s.flags&1 != 0 }},
			{sheetSave, func(s *State) bool { return s.flags&1 == 0 }},
		}},
		{"sidebar", "collapsed", []step{
			{sidebarItem.At(2), func(s *State) bool { return s.values[0] == 2 }},
			{sidebarCollapse, func(s *State) bool { return s.flags&1 != 0 }},
		}},
		{"slider", "default", []step{{sliderPrimary, func(s *State) bool { return s.values[0] == 50 }}}},
		{"sonner", "stack", []step{{sonnerDismiss.At(1), func(s *State) bool { return s.flags&0b010 == 0 && s.flags&0b101 == 0b101 }}}},
		{"table", "selection", []step{
			{tableRow.At(1), func(s *State) bool { return s.flags&2 != 0 }},
			{tableRow.At(1), func(s *State) bool { return s.flags&2 == 0 }},
		}},
		{"tabs", "line", []step{{tabsItem.At(1), func(s *State) bool { return s.values[0] == 1 }}}},
		{"toast", "types", []step{
			{toastAction, func(s *State) bool { return s.values[0] == 1 }},
			{toastDismiss, func(s *State) bool { return s.flags&1 == 0 }},
		}},
		{"toggle", "default", []step{{toggleItem.At(2), func(s *State) bool { return s.flags&4 != 0 }}}},
		{"toggle-group", "single", []step{{toggleGroupItem.At(2), func(s *State) bool { return s.flags == 4 }}}},
		{"tooltip", "default", []step{{tooltipTrigger, func(s *State) bool { return s.flags&1 != 0 }}}},
		{"tree", "selection", []step{
			{treeLeaf.At(2), func(s *State) bool { return s.values[0] == 2 }},
			{treeGroupDisclosure, func(s *State) bool { return s.flags&1 == 0 }},
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
				if step.id != popoverDismiss { // the backdrop has no hit of its own to find
					findHit(t, &canvas, step.id) // the demo declares the control it reads the click on
				}
				canvas.BeginEventPass(step.id, gfx.ClickInfo{})
				paintFrame(&canvas, state, size, 0, true)
				canvas.EndEventPass()
				if !step.want(state) {
					t.Fatalf("step %d: click on %v left flags=%b values=%v", index, step.id, state.flags, state.values)
				}
			}
		})
	}
}
