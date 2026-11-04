package components

import (
	"testing"

	"github.com/vivaui/viva/ui"
)

func findBarFixture(replacing bool) FindBarSnapshot {
	return FindBarSnapshot{
		Find:        InputSnapshot{ID: ui.Item(1), Label: "Find", Value: "needle", Placeholder: "Find"},
		Replacement: InputSnapshot{ID: ui.Item(2), Label: "Replace", Placeholder: "Replace"},
		Replacing:   replacing,
		PreviousID:  ui.Item(3), NextID: ui.Item(4), CloseID: ui.Item(5),
		MatchCaseID: ui.Item(6), WholeWordID: ui.Item(7), RegexpID: ui.Item(8),
		ToggleReplaceID: ui.Item(9), ReplaceOneID: ui.Item(10), ReplaceAllID: ui.Item(11),
		Status: "3 of 12",
		Labels: FindBarLabels{
			Previous: "Previous match", Next: "Next match", Close: "Close find",
			MatchCase: "Match case", WholeWord: "Whole word", Regexp: "Regular expression",
			ToggleReplace: "Toggle replace", ReplaceOne: "Replace", ReplaceAll: "Replace all",
		},
	}
}

func hitsByID(canvas *ui.Canvas) map[ui.ID]ui.Hit {
	out := map[ui.ID]ui.Hit{}
	for _, hit := range canvas.Hits() {
		out[hit.ID] = hit
	}
	return out
}

func TestFindBarDeclaresEveryControlWithinItsBounds(t *testing.T) {
	theme := ui.LightTheme()
	for _, replacing := range []bool{false, true} {
		for _, width := range []float32{240, 360, 480, 720} {
			var canvas ui.Canvas
			bounds := ui.Rect{X: 30, Y: 40, Width: width, Height: 5}
			layout := PaintFindBar(&canvas, theme, bounds, findBarFixture(replacing))
			if want := FindBarHeight(theme, replacing); layout.Bounds.Height != want || layout.Bounds.Y != 40 {
				t.Fatalf("replacing=%v width=%v: bar bounds %+v, want height %v from the top", replacing, width, layout.Bounds, want)
			}
			hits := hitsByID(&canvas)
			wantIDs := []ui.ID{ui.Item(1), ui.Item(3), ui.Item(4), ui.Item(5), ui.Item(6), ui.Item(7), ui.Item(8), ui.Item(9)}
			if replacing {
				wantIDs = append(wantIDs, ui.Item(2), ui.Item(10), ui.Item(11))
			}
			if len(hits) != len(wantIDs) {
				t.Fatalf("replacing=%v width=%v: %d hits, want %d", replacing, width, len(hits), len(wantIDs))
			}
			var rects []ui.Rect
			for _, id := range wantIDs {
				hit, ok := hits[id]
				if !ok {
					t.Fatalf("replacing=%v width=%v: no hit %v", replacing, width, id)
				}
				b := hit.Bounds
				if b.X < layout.Bounds.X || b.Y < layout.Bounds.Y || b.X+b.Width > layout.Bounds.X+layout.Bounds.Width+0.01 || b.Y+b.Height > layout.Bounds.Y+layout.Bounds.Height+0.01 {
					t.Errorf("replacing=%v width=%v: %v at %+v leaves the bar %+v", replacing, width, id, b, layout.Bounds)
				}
				for _, other := range rects {
					if b.X < other.X+other.Width-0.01 && other.X < b.X+b.Width-0.01 && b.Y < other.Y+other.Height-0.01 && other.Y < b.Y+b.Height-0.01 {
						t.Errorf("replacing=%v width=%v: %v at %+v overlaps %+v", replacing, width, id, b, other)
					}
				}
				rects = append(rects, b)
			}
		}
	}
}

func TestFindBarFieldTakesWhatTheButtonsLeaveAndTheStatusGivesWayFirst(t *testing.T) {
	theme := ui.LightTheme()
	var wide, narrow ui.Canvas
	wideLayout := PaintFindBar(&wide, theme, ui.Rect{Width: 600}, findBarFixture(false))
	minimum := FindBarMinWidth(findBarFixture(false))
	narrowLayout := PaintFindBar(&narrow, theme, ui.Rect{Width: minimum}, findBarFixture(false))
	if wideLayout.Status.Width <= 0 {
		t.Fatal("a wide bar shows no count")
	}
	if narrowLayout.Status.Width != 0 {
		t.Fatalf("a narrow bar keeps its count (%v wide) and squeezes the field to %v", narrowLayout.Status.Width, narrowLayout.Find.Width)
	}
	if narrowLayout.Find.Width < findBarMinField-0.01 {
		t.Fatalf("at the minimum width the field is %v wide, under its minimum %v", narrowLayout.Find.Width, findBarMinField)
	}
	if wideLayout.Find.Width <= narrowLayout.Find.Width {
		t.Fatal("the field does not grow with the bar")
	}
	found := false
	for _, text := range wide.Texts() {
		found = found || text.Value == "3 of 12"
	}
	if !found {
		t.Fatal("the count is not painted")
	}
}

func TestFindBarOptionsAreCheckedStatesAndNamed(t *testing.T) {
	theme := ui.LightTheme()
	snapshot := findBarFixture(false)
	snapshot.MatchCase, snapshot.Regexp = true, true
	var canvas ui.Canvas
	PaintFindBar(&canvas, theme, ui.Rect{Width: 480}, snapshot)
	hits := hitsByID(&canvas)
	for id, want := range map[ui.ID]bool{ui.Item(6): true, ui.Item(7): false, ui.Item(8): true} {
		hit := hits[id]
		if hit.Checked != want || hit.Selected != want || hit.Role != ui.RoleButton {
			t.Errorf("option %v: checked %v selected %v role %v, want checked %v", id, hit.Checked, hit.Selected, hit.Role, want)
		}
	}
	if hits[ui.Item(6)].Label != "Match case" || hits[ui.Item(7)].Label != "Whole word" || hits[ui.Item(8)].Label != "Regular expression" {
		t.Errorf("option names: %q %q %q", hits[ui.Item(6)].Label, hits[ui.Item(7)].Label, hits[ui.Item(8)].Label)
	}
	if hits[ui.Item(3)].Label != "Previous match" || hits[ui.Item(5)].Label != "Close find" {
		t.Errorf("button names: %q %q", hits[ui.Item(3)].Label, hits[ui.Item(5)].Label)
	}
	if hits[ui.Item(1)].Role != ui.RoleTextField || hits[ui.Item(1)].Value != "needle" {
		t.Errorf("find field hit = %+v", hits[ui.Item(1)])
	}
}

func TestFindBarLeavesOutButtonsWithoutAnID(t *testing.T) {
	theme := ui.LightTheme()
	snapshot := findBarFixture(true)
	snapshot.WholeWordID, snapshot.RegexpID, snapshot.ToggleReplaceID, snapshot.ReplaceAllID = ui.ID{}, ui.ID{}, ui.ID{}, ui.ID{}
	var canvas ui.Canvas
	layout := PaintFindBar(&canvas, theme, ui.Rect{Width: 480}, snapshot)
	hits := hitsByID(&canvas)
	for _, id := range []ui.ID{ui.Item(7), ui.Item(8), ui.Item(9), ui.Item(11)} {
		if _, ok := hits[id]; ok {
			t.Errorf("%v was left out and still declared", id)
		}
	}
	if layout.ToggleReplace != (ui.Rect{}) {
		t.Errorf("toggle rect %+v for a bar without the toggle", layout.ToggleReplace)
	}
}

func TestFindBarDisabledDimsSteppingAndReplacing(t *testing.T) {
	theme := ui.LightTheme()
	snapshot := findBarFixture(true)
	snapshot.Disabled, snapshot.NoResults, snapshot.Status = true, true, "No results"
	var canvas ui.Canvas
	PaintFindBar(&canvas, theme, ui.Rect{Width: 520}, snapshot)
	hits := hitsByID(&canvas)
	for _, id := range []ui.ID{ui.Item(3), ui.Item(4), ui.Item(10), ui.Item(11)} {
		if !hits[id].Disabled {
			t.Errorf("%v accepts a click though there is nothing to step to", id)
		}
	}
	if hits[ui.Item(5)].Disabled || hits[ui.Item(6)].Disabled {
		t.Error("closing the bar or changing an option must stay possible")
	}
	for _, text := range canvas.Texts() {
		if text.Value == "No results" && text.Style.Color != theme.Danger {
			t.Errorf("the empty result is %+v, want the danger color", text.Style.Color)
		}
	}
}

func TestFindBarSteadyPaintAllocatesNothing(t *testing.T) {
	theme := ui.LightTheme()
	snapshot := findBarFixture(true)
	var canvas ui.Canvas
	paint := func() {
		canvas.Reset()
		PaintFindBar(&canvas, theme, ui.Rect{Width: 520}, snapshot)
	}
	paint()
	if allocations := testing.AllocsPerRun(100, paint); allocations != 0 {
		t.Fatalf("steady find bar allocations: %g", allocations)
	}
}

func TestFindBarCommandsForKeys(t *testing.T) {
	control, meta := ui.ModifierControl, ui.ModifierMeta
	cases := []struct {
		name  string
		event ui.KeyEvent
		inBar bool
		want  FindBarCommand
		ok    bool
	}{
		{"Ctrl+F", ui.KeyEvent{Modifiers: control, Text: "f"}, false, FindBarOpen, true},
		{"Cmd+F", ui.KeyEvent{Modifiers: meta, Text: "f"}, false, FindBarOpen, true},
		{"Cmd+Shift+F is not find", ui.KeyEvent{Modifiers: meta | ui.ModifierShift, Text: "F"}, false, 0, false},
		{"Ctrl+H is the application's", ui.KeyEvent{Modifiers: control, Text: "h"}, false, 0, false},
		{"Cmd+Option+F", ui.KeyEvent{Modifiers: meta | ui.ModifierAlt, Text: "ƒ"}, false, FindBarOpenReplace, true},
		{"F3", ui.KeyEvent{Key: ui.KeyF3}, false, FindBarNext, true},
		{"Shift+F3", ui.KeyEvent{Key: ui.KeyF3, Modifiers: ui.ModifierShift}, false, FindBarPrevious, true},
		{"Enter in the bar", ui.KeyEvent{Key: ui.KeyEnter}, true, FindBarNext, true},
		{"Shift+Enter in the bar", ui.KeyEvent{Key: ui.KeyEnter, Modifiers: ui.ModifierShift}, true, FindBarPrevious, true},
		{"Enter in the editor is a line break", ui.KeyEvent{Key: ui.KeyEnter}, false, 0, false},
		{"Escape in the bar", ui.KeyEvent{Key: ui.KeyEscape}, true, FindBarCloseCommand, true},
		{"Escape in the editor", ui.KeyEvent{Key: ui.KeyEscape}, false, 0, false},
		{"a release", ui.KeyEvent{Key: ui.KeyF3, Release: true}, false, 0, false},
	}
	for _, c := range cases {
		got, ok := FindBarCommandForKey(c.event, c.inBar)
		if ok != c.ok || ok && got != c.want {
			t.Errorf("%s: %v, %v; want %v, %v", c.name, got, ok, c.want, c.ok)
		}
	}
}

func TestGoToLineBarIsOneFieldWithinItsBounds(t *testing.T) {
	theme := ui.LightTheme()
	var canvas ui.Canvas
	field := PaintGoToLine(&canvas, theme, ui.Rect{X: 10, Y: 20, Width: 500, Height: 3}, GoToLineSnapshot{
		Input: InputSnapshot{ID: ui.Item(1), Label: "Go to line", Placeholder: "Line 1–40"}, Invalid: true,
	})
	hits := canvas.Hits()
	if len(hits) != 1 || hits[0].ID != ui.Item(1) || hits[0].Role != ui.RoleTextField {
		t.Fatalf("hits = %+v", hits)
	}
	if hits[0].Bounds != field {
		t.Fatalf("the hit %+v is not the field %+v", hits[0].Bounds, field)
	}
	if field.X < 10 || field.Y < 20 || field.X+field.Width > 10+goToLineWidth || field.Y+field.Height > 20+GoToLineHeight(theme) {
		t.Fatalf("field %+v leaves the bar", field)
	}
	var narrow ui.Canvas
	if field := PaintGoToLine(&narrow, theme, ui.Rect{Width: 120}, GoToLineSnapshot{Input: InputSnapshot{ID: ui.Item(1)}}); field.Width > 120 {
		t.Fatalf("a narrow bar's field is %v wide", field.Width)
	}
}
