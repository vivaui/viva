package components

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

func findBarFixture(replacing bool) FindBarSnapshot {
	return FindBarSnapshot{
		Find:        InputSnapshot{ID: gfx.Item(1), Label: "Find", Value: "needle", Placeholder: "Find"},
		Replacement: InputSnapshot{ID: gfx.Item(2), Label: "Replace", Placeholder: "Replace"},
		Replacing:   replacing,
		PreviousID:  gfx.Item(3), NextID: gfx.Item(4), CloseID: gfx.Item(5),
		MatchCaseID: gfx.Item(6), WholeWordID: gfx.Item(7), RegexpID: gfx.Item(8),
		ToggleReplaceID: gfx.Item(9), ReplaceOneID: gfx.Item(10), ReplaceAllID: gfx.Item(11),
		Status: "3 of 12",
		Labels: FindBarLabels{
			Previous: "Previous match", Next: "Next match", Close: "Close find",
			MatchCase: "Match case", WholeWord: "Whole word", Regexp: "Regular expression",
			ToggleReplace: "Toggle replace", ReplaceOne: "Replace", ReplaceAll: "Replace all",
		},
	}
}

func hitsByID(canvas *gfx.Canvas) map[gfx.ID]gfx.Hit {
	out := map[gfx.ID]gfx.Hit{}
	for _, hit := range canvas.Hits() {
		out[hit.ID] = hit
	}
	return out
}

func TestFindBarDeclaresEveryControlWithinItsBounds(t *testing.T) {
	theme := gfx.LightTheme()
	for _, replacing := range []bool{false, true} {
		for _, width := range []float32{240, 360, 480, 720} {
			var canvas gfx.Canvas
			bounds := gfx.Rect{X: 30, Y: 40, Width: width, Height: 5}
			layout := PaintFindBar(&canvas, theme, bounds, findBarFixture(replacing))
			if want := FindBarHeight(theme, replacing); layout.Bounds.Height != want || layout.Bounds.Y != 40 {
				t.Fatalf("replacing=%v width=%v: bar bounds %+v, want height %v from the top", replacing, width, layout.Bounds, want)
			}
			hits := hitsByID(&canvas)
			wantIDs := []gfx.ID{gfx.Item(1), gfx.Item(3), gfx.Item(4), gfx.Item(5), gfx.Item(6), gfx.Item(7), gfx.Item(8), gfx.Item(9)}
			if replacing {
				wantIDs = append(wantIDs, gfx.Item(2), gfx.Item(10), gfx.Item(11))
			}
			if len(hits) != len(wantIDs) {
				t.Fatalf("replacing=%v width=%v: %d hits, want %d", replacing, width, len(hits), len(wantIDs))
			}
			var rects []gfx.Rect
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
	theme := gfx.LightTheme()
	var wide, narrow gfx.Canvas
	wideLayout := PaintFindBar(&wide, theme, gfx.Rect{Width: 600}, findBarFixture(false))
	minimum := FindBarMinWidth(findBarFixture(false))
	narrowLayout := PaintFindBar(&narrow, theme, gfx.Rect{Width: minimum}, findBarFixture(false))
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
	theme := gfx.LightTheme()
	snapshot := findBarFixture(false)
	snapshot.MatchCase, snapshot.Regexp = true, true
	var canvas gfx.Canvas
	PaintFindBar(&canvas, theme, gfx.Rect{Width: 480}, snapshot)
	hits := hitsByID(&canvas)
	for id, want := range map[gfx.ID]bool{gfx.Item(6): true, gfx.Item(7): false, gfx.Item(8): true} {
		hit := hits[id]
		if hit.Checked != want || hit.Selected != want || hit.Role != gfx.RoleButton {
			t.Errorf("option %v: checked %v selected %v role %v, want checked %v", id, hit.Checked, hit.Selected, hit.Role, want)
		}
	}
	if hits[gfx.Item(6)].Label != "Match case" || hits[gfx.Item(7)].Label != "Whole word" || hits[gfx.Item(8)].Label != "Regular expression" {
		t.Errorf("option names: %q %q %q", hits[gfx.Item(6)].Label, hits[gfx.Item(7)].Label, hits[gfx.Item(8)].Label)
	}
	if hits[gfx.Item(3)].Label != "Previous match" || hits[gfx.Item(5)].Label != "Close find" {
		t.Errorf("button names: %q %q", hits[gfx.Item(3)].Label, hits[gfx.Item(5)].Label)
	}
	if hits[gfx.Item(1)].Role != gfx.RoleTextField || hits[gfx.Item(1)].Value != "needle" {
		t.Errorf("find field hit = %+v", hits[gfx.Item(1)])
	}
}

func TestFindBarLeavesOutButtonsWithoutAnID(t *testing.T) {
	theme := gfx.LightTheme()
	snapshot := findBarFixture(true)
	snapshot.WholeWordID, snapshot.RegexpID, snapshot.ToggleReplaceID, snapshot.ReplaceAllID = gfx.ID{}, gfx.ID{}, gfx.ID{}, gfx.ID{}
	var canvas gfx.Canvas
	layout := PaintFindBar(&canvas, theme, gfx.Rect{Width: 480}, snapshot)
	hits := hitsByID(&canvas)
	for _, id := range []gfx.ID{gfx.Item(7), gfx.Item(8), gfx.Item(9), gfx.Item(11)} {
		if _, ok := hits[id]; ok {
			t.Errorf("%v was left out and still declared", id)
		}
	}
	if layout.ToggleReplace != (gfx.Rect{}) {
		t.Errorf("toggle rect %+v for a bar without the toggle", layout.ToggleReplace)
	}
}

func TestFindBarDisabledDimsSteppingAndReplacing(t *testing.T) {
	theme := gfx.LightTheme()
	snapshot := findBarFixture(true)
	snapshot.Disabled, snapshot.NoResults, snapshot.Status = true, true, "No results"
	var canvas gfx.Canvas
	PaintFindBar(&canvas, theme, gfx.Rect{Width: 520}, snapshot)
	hits := hitsByID(&canvas)
	for _, id := range []gfx.ID{gfx.Item(3), gfx.Item(4), gfx.Item(10), gfx.Item(11)} {
		if !hits[id].Disabled {
			t.Errorf("%v accepts a click though there is nothing to step to", id)
		}
	}
	if hits[gfx.Item(5)].Disabled || hits[gfx.Item(6)].Disabled {
		t.Error("closing the bar or changing an option must stay possible")
	}
	for _, text := range canvas.Texts() {
		if text.Value == "No results" && text.Style.Color != theme.Danger {
			t.Errorf("the empty result is %+v, want the danger color", text.Style.Color)
		}
	}
}

func TestFindBarSteadyPaintAllocatesNothing(t *testing.T) {
	theme := gfx.LightTheme()
	snapshot := findBarFixture(true)
	var canvas gfx.Canvas
	paint := func() {
		canvas.Reset()
		PaintFindBar(&canvas, theme, gfx.Rect{Width: 520}, snapshot)
	}
	paint()
	if allocations := testing.AllocsPerRun(100, paint); allocations != 0 {
		t.Fatalf("steady find bar allocations: %g", allocations)
	}
}

func TestFindBarCommandsForKeys(t *testing.T) {
	control, meta := gfx.ModifierControl, gfx.ModifierMeta
	cases := []struct {
		name  string
		event gfx.KeyEvent
		inBar bool
		want  FindBarCommand
		ok    bool
	}{
		{"Ctrl+F", gfx.KeyEvent{Modifiers: control, Text: "f"}, false, FindBarOpen, true},
		{"Cmd+F", gfx.KeyEvent{Modifiers: meta, Text: "f"}, false, FindBarOpen, true},
		{"Cmd+Shift+F is not find", gfx.KeyEvent{Modifiers: meta | gfx.ModifierShift, Text: "F"}, false, 0, false},
		{"Ctrl+H is the application's", gfx.KeyEvent{Modifiers: control, Text: "h"}, false, 0, false},
		{"Cmd+Option+F", gfx.KeyEvent{Modifiers: meta | gfx.ModifierAlt, Text: "ƒ"}, false, FindBarOpenReplace, true},
		{"F3", gfx.KeyEvent{Key: gfx.KeyF3}, false, FindBarNext, true},
		{"Shift+F3", gfx.KeyEvent{Key: gfx.KeyF3, Modifiers: gfx.ModifierShift}, false, FindBarPrevious, true},
		{"Enter in the bar", gfx.KeyEvent{Key: gfx.KeyEnter}, true, FindBarNext, true},
		{"Shift+Enter in the bar", gfx.KeyEvent{Key: gfx.KeyEnter, Modifiers: gfx.ModifierShift}, true, FindBarPrevious, true},
		{"Enter in the editor is a line break", gfx.KeyEvent{Key: gfx.KeyEnter}, false, 0, false},
		{"Escape in the bar", gfx.KeyEvent{Key: gfx.KeyEscape}, true, FindBarCloseCommand, true},
		{"Escape in the editor", gfx.KeyEvent{Key: gfx.KeyEscape}, false, 0, false},
		{"a release", gfx.KeyEvent{Key: gfx.KeyF3, Release: true}, false, 0, false},
	}
	for _, c := range cases {
		got, ok := FindBarCommandForKey(c.event, c.inBar)
		if ok != c.ok || ok && got != c.want {
			t.Errorf("%s: %v, %v; want %v, %v", c.name, got, ok, c.want, c.ok)
		}
	}
}

func TestGoToLineBarIsOneFieldWithinItsBounds(t *testing.T) {
	theme := gfx.LightTheme()
	var canvas gfx.Canvas
	field := PaintGoToLine(&canvas, theme, gfx.Rect{X: 10, Y: 20, Width: 500, Height: 3}, GoToLineSnapshot{
		Input: InputSnapshot{ID: gfx.Item(1), Label: "Go to line", Placeholder: "Line 1–40"}, Invalid: true,
	})
	hits := canvas.Hits()
	if len(hits) != 1 || hits[0].ID != gfx.Item(1) || hits[0].Role != gfx.RoleTextField {
		t.Fatalf("hits = %+v", hits)
	}
	if hits[0].Bounds != field {
		t.Fatalf("the hit %+v is not the field %+v", hits[0].Bounds, field)
	}
	if field.X < 10 || field.Y < 20 || field.X+field.Width > 10+goToLineWidth || field.Y+field.Height > 20+GoToLineHeight(theme) {
		t.Fatalf("field %+v leaves the bar", field)
	}
	var narrow gfx.Canvas
	if field := PaintGoToLine(&narrow, theme, gfx.Rect{Width: 120}, GoToLineSnapshot{Input: InputSnapshot{ID: gfx.Item(1)}}); field.Width > 120 {
		t.Fatalf("a narrow bar's field is %v wide", field.Width)
	}
}
