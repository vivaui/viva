package components

import (
	"fmt"
	"testing"
	"time"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
)

type hierarchyActionFixture struct {
	menu   HierarchyMenuState
	option HierarchyMenuOptions
	driver *app.Driver
	result HierarchyMenuKeyResult
}

func newHierarchyActionFixture(t *testing.T, values []string, width, rows int, configure func(*HierarchyMenuOptions)) *hierarchyActionFixture {
	t.Helper()
	f := &hierarchyActionFixture{}
	var tree gfx.Tree
	feature := tree.Feature("hierarchy-actions")
	f.option = HierarchyMenuOptions{
		OptionIDs: tree.Sub(feature, tree.Name("rows")), ScrollID: tree.ID(feature, "scroll"),
		Scope: tree.ID(feature, "popup"), Layer: 4, Overlay: true, DismissID: tree.ID(feature, "dismiss"),
		Viewport: gfx.Rect{Width: float32(width), Height: 600}, MaxRows: rows, Typeahead: true,
		Leading:  []HierarchyMenuAction{{ID: tree.ID(feature, "none"), Label: "No group"}},
		Trailing: []HierarchyMenuAction{{ID: tree.ID(feature, "new"), Label: "New group"}},
	}
	if configure != nil {
		configure(&f.option)
	}
	f.menu.Build("/", "", nil, len(values), func(i int) HierarchyMenuItem { return HierarchyMenuItem{Value: values[i]} })
	driver, err := app.NewDriver(app.Config{Width: width, Height: 600, Paint: func(c *gfx.Canvas, _ gfx.Size) {
		if index, ok := c.ClickedIn(f.option.OptionIDs); ok {
			value, picked := f.menu.Choose(index)
			f.result = HierarchyMenuKeyResult{Chosen: value, Picked: picked}
		}
		for _, actions := range [][]HierarchyMenuAction{f.option.Leading, f.option.Trailing} {
			for _, action := range actions {
				if c.Clicked(action.ID) {
					f.result = HierarchyMenuKeyResult{ActionID: action.ID}
				}
			}
		}
		PaintHierarchyMenu(c, gfx.LightTheme(), gfx.Rect{X: 20, Y: 20, Width: 160, Height: 26}, &f.menu, f.option)
	}, OnKey: func(event gfx.KeyEvent) gfx.EventResult {
		result := f.menu.Key(event, true)
		if result.Picked || !result.ActionID.IsZero() {
			f.result = result
		}
		return result.EventResult
	}, OnScroll: func(event gfx.ScrollEvent) gfx.EventResult {
		return gfx.EventResult{Handled: true, Repaint: f.menu.Wheel(event)}
	}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	f.driver = driver
	t.Cleanup(driver.Close)
	return f
}

func (f *hierarchyActionFixture) key(t *testing.T, key gfx.Key) {
	t.Helper()
	if err := f.driver.Key(key, 0); err != nil {
		t.Fatal(err)
	}
}

func (f *hierarchyActionFixture) wantAction(t *testing.T, id gfx.ID) {
	t.Helper()
	if f.result.ActionID != id || f.result.Picked || f.result.Chosen != "" {
		t.Fatalf("action result %+v, want command %v without a path selection", f.result, id)
	}
}

func (f *hierarchyActionFixture) wantValue(t *testing.T, value string) {
	t.Helper()
	if !f.result.Picked || f.result.Chosen != value || !f.result.ActionID.IsZero() {
		t.Fatalf("path result %+v, want %q without a command", f.result, value)
	}
}

func TestHierarchyMenuRootActionsAreIndependentOfSameNamedPaths(t *testing.T) {
	f := newHierarchyActionFixture(t, []string{"No group", "New group", "Parent/Leaf"}, 900, 7, nil)
	none, create := f.option.Leading[0].ID, f.option.Trailing[0].ID
	hierarchyTestClick(t, f.driver, none)
	f.wantAction(t, none)
	hierarchyTestClick(t, f.driver, f.option.OptionIDs.At(0))
	f.wantValue(t, "No group")
	hierarchyTestClick(t, f.driver, create)
	f.wantAction(t, create)
	hierarchyTestClick(t, f.driver, f.option.OptionIDs.At(1))
	f.wantValue(t, "New group")
	f.key(t, gfx.KeyHome)
	f.key(t, gfx.KeyEnter)
	f.wantAction(t, none)
	f.key(t, gfx.KeyEnd)
	f.key(t, gfx.KeyEnter)
	f.wantAction(t, create)
	// Hovering a real parent may open the cascade, but it remains selectable.
	hierarchyTestClick(t, f.driver, f.option.OptionIDs.At(2))
	f.wantValue(t, "Parent")
	hierarchyTestClick(t, f.driver, f.option.OptionIDs.At(1<<16))
	f.wantValue(t, "Parent/Leaf")
	for _, id := range []gfx.ID{none, create} {
		count := 0
		for _, hit := range f.driver.Snapshot().Hits {
			if hit.ID == id {
				count++
				if hit.Role != gfx.RoleMenuItem || hit.Value != "" {
					t.Fatalf("command became a path option: %+v", hit)
				}
			}
		}
		if count != 1 {
			t.Fatalf("root command appears in %d cascade panels", count)
		}
	}
}

func TestHierarchyMenuRootActionsShareKeyboardScrollAndTypeahead(t *testing.T) {
	values := make([]string, 24)
	for i := range values {
		values[i] = fmt.Sprintf("Group %02d", i)
	}
	f := newHierarchyActionFixture(t, values, 900, 3, nil)
	none, create := f.option.Leading[0].ID, f.option.Trailing[0].ID
	f.key(t, gfx.KeyHome)
	f.key(t, gfx.KeyPageDown)
	f.key(t, gfx.KeyEnter)
	f.wantValue(t, "Group 02")
	f.key(t, gfx.KeyPageUp)
	f.key(t, gfx.KeyEnter)
	f.wantAction(t, none)
	f.key(t, gfx.KeyEnd)
	last := hierarchyTestHit(t, f.driver, create)
	if last.Height != 32 || f.menu.scrolls[0].Offset == 0 {
		t.Fatalf("End did not reveal the complete final command: %+v", last)
	}
	f.key(t, gfx.KeyEnter)
	f.wantAction(t, create)
	f.key(t, gfx.KeyHome)
	// Driver.Type is text-editor input. A native menu sends printable key
	// events, which we feed to the same Key contract instead.
	for _, letter := range "Group 23" {
		if result := f.menu.Key(gfx.KeyEvent{Text: string(letter)}, true); !result.Handled {
			t.Fatalf("typeahead did not handle %q", letter)
		}
	}
	f.key(t, gfx.KeyEnter)
	f.wantValue(t, "Group 23")
	// Shift produces ordinary uppercase text, without enabling modified arrows.
	f.key(t, gfx.KeyHome)
	if result := f.menu.Key(gfx.KeyEvent{Text: "N", Modifiers: gfx.ModifierShift}, true); !result.Handled {
		t.Fatal("Shift+N did not participate in typeahead")
	}
	result := f.menu.Key(gfx.KeyEvent{Key: gfx.KeyEnter}, true)
	if result.ActionID != create || result.Picked {
		t.Fatalf("uppercase typeahead chose %+v, want the New group command", result)
	}
}

func TestHierarchyMenuActionsSkipDisabledAndIgnoreKeyReleases(t *testing.T) {
	f := newHierarchyActionFixture(t, []string{"No group", "New group"}, 900, 7, func(o *HierarchyMenuOptions) {
		o.Leading[0].Disabled, o.Trailing[0].Disabled = true, true
	})
	f.key(t, gfx.KeyHome)
	f.key(t, gfx.KeyEnter)
	f.wantValue(t, "No group")
	f.key(t, gfx.KeyEnd)
	f.key(t, gfx.KeyEnter)
	f.wantValue(t, "New group")
	for _, event := range []gfx.KeyEvent{
		{Key: gfx.KeyHome, Release: true},
		{Key: gfx.KeyRight, Release: true},
		{Key: gfx.KeyEnter, Release: true},
		{Text: "N", Release: true},
		{Key: gfx.KeyHome, Modifiers: gfx.ModifierShift},
		{Text: "N", Modifiers: gfx.ModifierControl},
	} {
		if result := f.menu.Key(event, true); result.Handled || result.Picked || !result.ActionID.IsZero() {
			t.Fatalf("key %+v acted: %+v", event, result)
		}
	}
	f.key(t, gfx.KeyEnter)
	f.wantValue(t, "New group")
}

func TestHierarchyMenuSpaceContinuesTypeaheadBeforeActivating(t *testing.T) {
	f := newHierarchyActionFixture(t, []string{"New office"}, 500, 7, nil)
	for _, letter := range "New" {
		f.menu.Key(gfx.KeyEvent{Text: string(letter)}, true)
	}
	// Platforms can report Space as a named key with no printable Text.
	result := f.menu.Key(gfx.KeyEvent{Key: gfx.KeySpace}, true)
	if !result.Handled || result.Picked || !result.ActionID.IsZero() {
		t.Fatalf("Space executed the row during typeahead: %+v", result)
	}
	for _, letter := range "group" {
		f.menu.Key(gfx.KeyEvent{Text: string(letter)}, true)
	}
	f.key(t, gfx.KeyEnter)
	f.wantAction(t, f.option.Trailing[0].ID)
	// Navigation starts a new session, so bare Space now activates the row.
	f.key(t, gfx.KeyHome)
	f.key(t, gfx.KeySpace)
	f.wantAction(t, f.option.Leading[0].ID)
}

func TestHierarchyMenuCompactRootActionsReturnWithTheRoot(t *testing.T) {
	f := newHierarchyActionFixture(t, []string{"A/B/C/D/E/F/G/H"}, 320, 7, nil)
	none, create := f.option.Leading[0].ID, f.option.Trailing[0].ID
	hierarchyTestClick(t, f.driver, f.option.OptionIDs.At(hierarchyOpenLevel<<16))
	if f.result.Picked || !f.result.ActionID.IsZero() || f.menu.Depth() != 1 {
		t.Fatalf("child navigation acted as selection: %+v depth=%d", f.result, f.menu.Depth())
	}
	for _, hit := range f.driver.Snapshot().Hits {
		if hit.ID == none || hit.ID == create {
			t.Fatalf("root command leaked into compact child: %+v", hit)
		}
	}
	f.key(t, gfx.KeyHome)
	f.key(t, gfx.KeyEnter)
	f.wantValue(t, "A/B")
	hierarchyTestClick(t, f.driver, f.option.OptionIDs.At(hierarchyBackIndex))
	f.key(t, gfx.KeyHome)
	f.key(t, gfx.KeyEnter)
	f.wantAction(t, none)
	f.key(t, gfx.KeyEnd)
	f.key(t, gfx.KeySpace)
	f.wantAction(t, create)
}

func TestHierarchyMenuEmptyRootCanActivateItsTrailingAction(t *testing.T) {
	f := newHierarchyActionFixture(t, nil, 500, 7, func(o *HierarchyMenuOptions) {
		o.Leading = nil
		o.Trailing[0].Label = "Create a group / project"
	})
	create := f.option.Trailing[0].ID
	f.key(t, gfx.KeyEnter)
	f.wantAction(t, create)
	if got := hierarchyTestHit(t, f.driver, create); got.Width <= 160 {
		t.Fatalf("root did not size for the command label: %+v", got)
	}
	for _, text := range f.driver.Snapshot().Texts {
		if text.Value != "Create a group / project" {
			t.Fatalf("command was split or replaced by an empty-list label: %+v", text)
		}
	}
}

func TestHierarchyMenuLeavesEditableTextToTheCaller(t *testing.T) {
	f := newHierarchyActionFixture(t, []string{"Alpha", "Beta"}, 500, 7, func(o *HierarchyMenuOptions) {
		o.Typeahead = false
	})
	for _, event := range []gfx.KeyEvent{{Text: "B"}, {Text: "B", Modifiers: gfx.ModifierShift}, {Key: gfx.KeySpace, Text: " "}} {
		if result := f.menu.Key(event, true); result.Handled {
			t.Fatalf("editable input's text %+v was consumed: %+v", event, result)
		}
	}
}

func TestHierarchyMenuRebuildForgetsInputAndDragState(t *testing.T) {
	values := make([]string, 20)
	for i := range values {
		values[i] = fmt.Sprintf("Group %02d", i)
	}
	f := newHierarchyActionFixture(t, values, 900, 3, nil)
	f.key(t, gfx.KeyEnd)
	bar := hierarchyTestHit(t, f.driver, f.option.ScrollID)
	result := f.menu.Pointer(gfx.PointerEvent{Kind: gfx.PointerDown, Button: gfx.PointerButtonPrimary, Target: f.option.ScrollID, X: bar.X + 2, Y: bar.Y + 2})
	if !result.Handled || !f.menu.scrolls[0].Dragging() {
		t.Fatal("fixture did not start a scrollbar drag")
	}
	f.menu.Key(gfx.KeyEvent{Text: "G"}, true)
	if !f.menu.typeahead.Typing(time.Now()) {
		t.Fatal("fixture did not start typeahead")
	}
	f.menu.Build("/", "", nil, 1, func(int) HierarchyMenuItem { return HierarchyMenuItem{Value: "Fresh"} })
	if f.menu.scrolls[0].Dragging() || f.menu.scrolls[0].Offset != 0 || f.menu.typeahead.Typing(time.Now()) || f.menu.Depth() != 0 {
		t.Fatal("reopened menu retained drag, scroll, typeahead or a child")
	}
	var canvas gfx.Canvas
	PaintHierarchyMenu(&canvas, gfx.LightTheme(), gfx.Rect{X: 20, Y: 20, Width: 160, Height: 26}, &f.menu, f.option)
	if result := f.menu.Key(gfx.KeyEvent{Key: gfx.KeyEnter}, true); result.ActionID != f.option.Leading[0].ID {
		t.Fatalf("reopened menu did not start on its root action: %+v", result)
	}
}
