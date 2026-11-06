package components

import (
	"strings"
	"testing"
	"time"
	"unsafe"

	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
)

func singleSelectFixture() *SingleSelectState {
	s := &SingleSelectState{AllowCreate: true, MatchCaseInsensitive: true}
	s.SetOptions([]SingleSelectOption{{Value: "dev", Label: "Development"}, {Value: "test", Label: "Testing"}, {Value: "blocked", Disabled: true}})
	return s
}

func BenchmarkSingleSelectPaint(b *testing.B) {
	var tree ui.Tree
	f := tree.Feature("single-select-benchmark")
	o := SingleSelectOptions{InputID: tree.ID(f, "input"), TriggerID: tree.ID(f, "trigger"), ClearID: tree.ID(f, "clear"), DismissID: tree.ID(f, "dismiss"), Scope: tree.ID(f, "scope"), ScrollID: tree.ID(f, "scroll"), OptionIDs: tree.Sub(f, tree.Name("option")), Label: "Group", CreatePrefix: "Create: ", Viewport: ui.Rect{Width: 800, Height: 600}, Layer: 4}
	for _, name := range []string{"closed", "open", "create", "long"} {
		b.Run(name, func(b *testing.B) {
			s := singleSelectFixture()
			s.SetValue("dev")
			s.Open = name != "closed"
			if name == "create" {
				s.Query.Insert("custom group")
			}
			if name == "long" {
				s.SetValue(strings.Repeat("长分组名称", 20))
			}
			var canvas ui.Canvas
			paint := func() {
				canvas.Reset()
				PaintSingleSelect(&canvas, ui.LightTheme(), ui.Rect{Width: 320, Height: 32}, s, o)
			}
			paint()
			b.ReportAllocs()
			for b.Loop() {
				paint()
			}
			b.ReportMetric(float64(unsafe.Sizeof(*s)), "state-B")
		})
	}
}

func TestSingleSelectReplacesClosesAndRetainsCreatedOptions(t *testing.T) {
	s := singleSelectFixture()
	s.Open = true
	if !s.Choose(0) || s.Open || s.Value() != "dev" {
		t.Fatal("selection did not close", s.Value())
	}
	s.Open = true
	if !s.Choose(1) || s.Value() != "test" || s.Open {
		t.Fatal("selection did not replace", s.Value())
	}
	s.Open = true
	if !s.Choose(1) || s.Value() != "test" {
		t.Fatal("reselect toggled value off")
	}
	s.Query.Insert("上海机房")
	if choices := s.Choices(); len(choices) != 1 || !choices[0].Create {
		t.Fatal("missing create row", choices)
	}
	s.Key(ui.KeyEvent{Key: ui.KeyEnter})
	if s.Value() != "上海机房" || s.Query.String() != "" || s.Open {
		t.Fatal("create failed", s.Value())
	}
	s.Choose(0)
	if choices := s.Choices(); len(choices) != 4 || choices[3].Value != "上海机房" {
		t.Fatal("created option was lost", choices)
	}
	s.Clear()
	if s.Value() != "" || s.Query.String() != "" {
		t.Fatal("clear retained a value")
	}
}

func TestSingleSelectExactDisabledAndPendingQuery(t *testing.T) {
	s := singleSelectFixture()
	s.Query.Insert("development")
	if choices := s.Choices(); len(choices) != 1 || choices[0].Create {
		t.Fatal("label shadowed by create", choices)
	}
	if !s.CommitQuery() || s.Value() != "dev" {
		t.Fatal("label did not resolve to stable value")
	}
	s.Query.Insert("BLOCKED")
	if s.CommitQuery() || s.Value() != "dev" {
		t.Fatal("disabled value bypassed")
	}
	s.Query = text.Editor{}
	s.SetValidateCreate(func(value string) bool { return value != "invalid" })
	s.Query.Insert("invalid")
	if s.CommitQuery() {
		t.Fatal("invalid create accepted")
	}
	s.Query = text.Editor{}
	s.Query.Insert(" 新分组 ")
	if !s.CommitQuery() || s.Value() != "新分组" {
		t.Fatal("pending query not trimmed/created")
	}
}

func TestSingleSelectKeyboardAndComposition(t *testing.T) {
	s := singleSelectFixture()
	s.SetValue("dev")
	s.Query.SetMarkedText("机房", text.Range{End: 2})
	s.Key(ui.KeyEvent{Key: ui.KeyEnter})
	if s.Value() != "dev" {
		t.Fatal("IME confirmation selected")
	}
	s.Query = text.Editor{}
	s.Query.Insert("custom")
	s.Key(ui.KeyEvent{Key: ui.KeyEnter, Repeat: true})
	if s.Value() != "dev" {
		t.Fatal("repeated Enter created")
	}
	s.Key(ui.KeyEvent{Key: ui.KeyEscape})
	if s.Value() != "dev" || s.Query.String() != "" || s.Open {
		t.Fatal("Escape lost committed value")
	}
	s.Key(ui.KeyEvent{Key: ui.KeyDown})
	s.Key(ui.KeyEvent{Key: ui.KeyDown})
	s.Key(ui.KeyEvent{Key: ui.KeyEnter})
	if s.Value() != "test" || s.Open {
		t.Fatal("keyboard selection failed", s.Value())
	}
	s.Open = true
	if result := s.Key(ui.KeyEvent{Key: ui.KeyTab, Modifiers: ui.ModifierShift}); !result.ContinueTraversal || s.Open {
		t.Fatal("Tab trapped focus")
	}
}

func TestSingleSelectChoiceCacheDoesNotAllocate(t *testing.T) {
	s := singleSelectFixture()
	s.Query.Insert("De")
	s.Choices()
	if count := testing.AllocsPerRun(100, func() { s.Choices() }); count != 0 {
		t.Fatalf("stable choices allocate %g times", count)
	}
}

func TestSingleSelectKeepsItsNativeInputAndPopupInBounds(t *testing.T) {
	var tree ui.Tree
	f := tree.Feature("single-select-layout")
	o := SingleSelectOptions{InputID: tree.ID(f, "input"), TriggerID: tree.ID(f, "trigger"), ClearID: tree.ID(f, "clear"), DismissID: tree.ID(f, "dismiss"), Scope: tree.ID(f, "scope"), OptionIDs: tree.Sub(f, tree.Name("option")), Label: "Group", Viewport: ui.Rect{Width: 800, Height: 600}, Layer: 4}
	for _, theme := range []ui.Theme{ui.LightTheme(), ui.DarkTheme()} {
		for _, width := range []float32{180, 320, 600} {
			s := singleSelectFixture()
			s.SetValue("dev")
			s.Open = true
			var canvas ui.Canvas
			layout := PaintSingleSelect(&canvas, theme, ui.Rect{X: 20, Y: 540, Width: width}, s, o)
			if layout.Input.Width <= 0 || layout.Input.X < layout.Bounds.X || layout.Input.X+layout.Input.Width > layout.Bounds.X+layout.Bounds.Width {
				t.Fatal("native query escaped the control", layout)
			}
			if layout.Popup.Y < 0 || layout.Popup.Y+layout.Popup.Height > 540 {
				t.Fatal("popup did not fit above the field", layout)
			}
			if canvas.TrappedFocusScope() != o.Scope || canvas.InitialFocus() != o.InputID {
				t.Fatal("popup lost its native input focus scope")
			}
			inputs := 0
			for _, hit := range canvas.Hits() {
				if hit.ID == o.InputID {
					inputs++
					if hit.Role != ui.RoleCombobox || hit.TextBounds != layout.Input || hit.Value != "Development" || !hit.Expanded {
						t.Fatal("native input geometry or selected-value semantics were lost", hit)
					}
				}
			}
			if inputs != 1 {
				t.Fatal("duplicate native inputs", inputs)
			}
		}
	}
}

func TestSingleSelectDismissDropsTheQueryAndKeepsTheValue(t *testing.T) {
	s := singleSelectFixture()
	s.SetValue("dev")
	s.Open = true
	s.Query.Insert("cust")
	s.Key(ui.KeyEvent{Key: ui.KeyTab})
	if s.Open || s.Query.String() != "" || s.Value() != "dev" {
		t.Fatalf("Tab left open=%v query=%q value=%q", s.Open, s.Query.String(), s.Value())
	}
	s.Open = true
	s.Query.Insert("cust")
	s.Dismiss()
	if s.Open || s.Query.String() != "" || s.Value() != "dev" {
		t.Fatalf("Dismiss left open=%v query=%q value=%q", s.Open, s.Query.String(), s.Value())
	}
}

func TestSingleSelectPopupStaysInsideAShortViewport(t *testing.T) {
	var tree ui.Tree
	f := tree.Feature("single-select-short-viewport")
	o := SingleSelectOptions{InputID: tree.ID(f, "input"), TriggerID: tree.ID(f, "trigger"), ClearID: tree.ID(f, "clear"), DismissID: tree.ID(f, "dismiss"), Scope: tree.ID(f, "scope"), OptionIDs: tree.Sub(f, tree.Name("option")), Label: "Group", Layer: 4}
	for _, test := range []struct {
		name     string
		viewport ui.Rect
		field    ui.Rect
	}{
		{"below", ui.Rect{Width: 800, Height: 100}, ui.Rect{X: 20, Y: 40, Width: 240}},
		{"near the bottom", ui.Rect{Width: 800, Height: 90}, ui.Rect{X: 20, Y: 20, Width: 240}},
		{"tiny", ui.Rect{Width: 800, Height: 24}, ui.Rect{X: 20, Y: 2, Width: 240}},
	} {
		t.Run(test.name, func(t *testing.T) {
			s := singleSelectFixture()
			s.SetValue("dev")
			s.Open = true
			var canvas ui.Canvas
			o.Viewport = test.viewport
			layout := PaintSingleSelect(&canvas, ui.DarkTheme(), test.field, s, o)
			if layout.Popup.Y < test.viewport.Y || layout.Popup.Height < 1 ||
				layout.Popup.Y+layout.Popup.Height > test.viewport.Y+test.viewport.Height {
				t.Fatalf("popup %+v escaped viewport %+v", layout.Popup, test.viewport)
			}
		})
	}
}

// Hovering a row is a pointer move onto the cursor: the row under the pointer
// becomes the row the highlight and Enter name. A pointer that stays where it
// was never takes the cursor back from the arrow keys.
func TestSingleSelectPointerMovesTheCursorWhenItCrossesRows(t *testing.T) {
	var tree ui.Tree
	f := tree.Feature("single-select-pointer")
	o := SingleSelectOptions{InputID: tree.ID(f, "input"), TriggerID: tree.ID(f, "trigger"), ClearID: tree.ID(f, "clear"), DismissID: tree.ID(f, "dismiss"), Scope: tree.ID(f, "scope"), ScrollID: tree.ID(f, "scroll"), OptionIDs: tree.Sub(f, tree.Name("option")), Label: "分组", CreatePrefix: "创建：", Viewport: ui.Rect{Width: 800, Height: 600}, Layer: 4}
	s := singleSelectFixture()
	s.Open = true
	var canvas ui.Canvas
	paint := func(hovered ui.ID) {
		canvas.Reset()
		canvas.SetInteraction(hovered, ui.ID{}, o.InputID, false)
		PaintSingleSelect(&canvas, ui.LightTheme(), ui.Rect{Width: 320, Height: 32}, s, o)
	}

	paint(o.OptionIDs.At(0))
	s.Key(ui.KeyEvent{Key: ui.KeyDown})
	if s.Active != 1 {
		t.Fatalf("arrow key did not move the cursor: %d", s.Active)
	}
	paint(o.OptionIDs.At(0)) // the pointer has not moved since the key press
	if s.Active != 1 {
		t.Fatalf("a resting pointer took the cursor back to %d", s.Active)
	}
	paint(o.OptionIDs.At(2)) // a disabled row is not a cursor target
	if s.Active != 1 {
		t.Fatalf("hovering the disabled row moved the cursor to %d", s.Active)
	}
	if !s.Key(ui.KeyEvent{Key: ui.KeyEnter}).Handled || s.Value() != "test" {
		t.Fatalf("Enter after hovering left the value %q", s.Value())
	}
}

// Moving the pointer down a row at a time leaves one tinted row behind on every
// paint: the canvas stream never carries the previous row's tint, so a repaint
// of only the changed row cannot leave stale pixels on screen.
func TestSingleSelectPaintTintsOneRowPerHoverStep(t *testing.T) {
	theme := ui.LightTheme()
	var tree ui.Tree
	f := tree.Feature("single-select-hover-sweep")
	o := SingleSelectOptions{InputID: tree.ID(f, "input"), TriggerID: tree.ID(f, "trigger"), ClearID: tree.ID(f, "clear"), DismissID: tree.ID(f, "dismiss"), Scope: tree.ID(f, "scope"), ScrollID: tree.ID(f, "scroll"), OptionIDs: tree.Sub(f, tree.Name("option")), Label: "分组", Viewport: ui.Rect{Width: 800, Height: 600}, Layer: 4}
	s := singleSelectFixture()
	s.Open = true
	var canvas ui.Canvas
	tint := func(row int) int {
		canvas.Reset()
		canvas.SetInteraction(o.OptionIDs.At(row), ui.ID{}, o.InputID, false)
		PaintSingleSelect(&canvas, theme, ui.Rect{Width: 320, Height: 32}, s, o)
		tinted := 0
		for _, rectangle := range canvas.Rectangles() {
			if rectangle.Color == theme.Accent && rectangle.Radius == theme.Metrics.RadiusItem() {
				tinted++
			}
		}
		return tinted
	}
	for _, row := range []int{0, 1, 0} {
		if got := tint(row); got != 1 {
			t.Fatalf("hovering row %d tinted %d rows", row, got)
		}
	}
}

// Options that share a long prefix differ only at the end, so a popup as
// narrow as its field would show them as identical ellipsized rows.
func TestSingleSelectPopupGrowsToTheLongestLabelWithinTheViewport(t *testing.T) {
	var tree ui.Tree
	f := tree.Feature("single-select-wide-labels")
	o := SingleSelectOptions{InputID: tree.ID(f, "input"), TriggerID: tree.ID(f, "trigger"), DismissID: tree.ID(f, "dismiss"), Scope: tree.ID(f, "scope"), OptionIDs: tree.Sub(f, tree.Name("option")), Label: "Group", Layer: 4}
	long := strings.Repeat("新疆维吾尔自治区 / ", 8)
	for _, test := range []struct {
		name     string
		viewport ui.Rect
		field    ui.Rect
	}{
		{"roomy", ui.Rect{Width: 1600, Height: 600}, ui.Rect{X: 20, Y: 40, Width: 240}},
		{"field near the right edge", ui.Rect{Width: 500, Height: 600}, ui.Rect{X: 300, Y: 40, Width: 180}},
		{"viewport narrower than the label", ui.Rect{Width: 320, Height: 600}, ui.Rect{X: 20, Y: 40, Width: 240}},
	} {
		t.Run(test.name, func(t *testing.T) {
			s := singleSelectFixture()
			s.SetOptions([]SingleSelectOption{{Value: "a", Label: "short"}, {Value: "b", Label: long + "甲"}})
			s.Open = true
			var canvas ui.Canvas
			o.Viewport = test.viewport
			theme := ui.DarkTheme()
			layout := PaintSingleSelect(&canvas, theme, test.field, s, o)
			if layout.Popup.Width < test.field.Width {
				t.Fatalf("popup %+v narrower than field %+v", layout.Popup, test.field)
			}
			if layout.Popup.X < test.viewport.X || layout.Popup.X+layout.Popup.Width > test.viewport.X+test.viewport.Width {
				t.Fatalf("popup %+v escaped viewport %+v", layout.Popup, test.viewport)
			}
			// Past the viewport the popup stops at its edge and truncates; with
			// room it must show the whole label.
			fits := canvas.TextWidthWeight(long+"甲", theme.Metrics.FontSizeM, ui.WeightRegular)+38 <= layout.Popup.Width-8
			if test.name == "roomy" && !fits {
				t.Fatalf("popup %+v truncates a label that fits viewport %+v", layout.Popup, test.viewport)
			}
			if test.name != "roomy" && layout.Popup.Width < test.viewport.Width-2 {
				t.Fatalf("popup %+v stopped short of viewport %+v", layout.Popup, test.viewport)
			}
		})
	}
}

func hierarchicalSelectFixture() (*SingleSelectState, SingleSelectOptions) {
	var tree ui.Tree
	f := tree.Feature("single-select-tree")
	o := SingleSelectOptions{InputID: tree.ID(f, "input"), TriggerID: tree.ID(f, "trigger"), ClearID: tree.ID(f, "clear"), DismissID: tree.ID(f, "dismiss"), Scope: tree.ID(f, "scope"), ScrollID: tree.ID(f, "scroll"), OptionIDs: tree.Sub(f, tree.Name("option")), Label: "Group", CreatePrefix: "Create: ", Viewport: ui.Rect{Width: 800, Height: 600}, Layer: 4}
	s := &SingleSelectState{AllowCreate: true, MatchCaseInsensitive: true, Separator: "/"}
	s.SetOptions([]SingleSelectOption{{Value: "School/Gansu/Lanzhou"}, {Value: "School/Gansu/Tianshui"}, {Value: "School/Xinjiang"}, {Value: "Office"}})
	return s, o
}

// A parent group is a value of its own, and the row of a nested group names
// the whole path, so two siblings with a long shared prefix stay distinct.
func TestSingleSelectTreeChoosesParentsAndLeavesByFullPath(t *testing.T) {
	s, _ := hierarchicalSelectFixture()
	s.Open = true
	if !s.Choose(0) || s.Value() != "School" {
		t.Fatalf("choosing the first-level row left %q", s.Value())
	}
	s.Open = true
	// Level 2 under School/Gansu (the open trail) is chosen by level<<16|row.
	s.Choices()
	s.tree.trail[0], s.tree.trail[1], s.tree.depth = 1, 2, 2
	if !s.Choose(2<<16|1) || s.Value() != "School/Gansu/Tianshui" {
		t.Fatalf("choosing a third-level row left %q", s.Value())
	}
}

func TestSingleSelectTreeKeyboardMovesBetweenLevels(t *testing.T) {
	s, _ := hierarchicalSelectFixture()
	s.Open = true
	s.Key(ui.KeyEvent{Key: ui.KeyRight}) // School
	s.Key(ui.KeyEvent{Key: ui.KeyRight}) // Gansu
	if s.tree.depth != 2 || s.tree.level != 2 {
		t.Fatalf("Right twice: depth %d level %d, want 2 and 2", s.tree.depth, s.tree.level)
	}
	s.Key(ui.KeyEvent{Key: ui.KeyDown})
	s.Key(ui.KeyEvent{Key: ui.KeyLeft})
	if s.tree.depth != 1 || s.tree.level != 1 {
		t.Fatalf("Left: depth %d level %d, want 1 and 1", s.tree.depth, s.tree.level)
	}
	s.Key(ui.KeyEvent{Key: ui.KeyRight})
	s.Key(ui.KeyEvent{Key: ui.KeyDown})
	s.Key(ui.KeyEvent{Key: ui.KeyEnter})
	if s.Value() != "School/Gansu/Tianshui" || s.Open {
		t.Fatalf("Enter chose %q (open %v)", s.Value(), s.Open)
	}
	s.Open = true
	s.Key(ui.KeyEvent{Key: ui.KeyEscape})
	if !s.Open || s.tree.depth != 1 {
		t.Fatalf("Escape closed the list (open %v, depth %d) instead of the last fly-out", s.Open, s.tree.depth)
	}
}

// Typing leaves the tree for the flat list of full paths, and clearing the text
// returns to the tree.
func TestSingleSelectTreeFallsBackToFlatWhileSearching(t *testing.T) {
	s, _ := hierarchicalSelectFixture()
	s.Open = true
	if !s.treeActive() {
		t.Fatal("tree not active without a query")
	}
	s.Query.Insert("tian")
	if s.treeActive() || len(s.Choices()) < 2 || s.Choices()[1].Value != "School/Gansu/Tianshui" {
		t.Fatalf("a query did not give the flat list of full paths: %+v", s.Choices())
	}
	s.Query = text.Editor{}
	if !s.treeActive() {
		t.Fatal("clearing the query did not return to the tree")
	}
}

func TestSingleSelectTreeOpensAtOnceAndSwitchesAfterARest(t *testing.T) {
	s, o := hierarchicalSelectFixture()
	s.Open = true
	var canvas ui.Canvas
	var now time.Duration
	canvas.SetClock(func() time.Duration { return now })
	paint := func(hovered ui.ID) ui.Rect {
		canvas.Reset()
		canvas.BeginFrame()
		defer canvas.EndFrame()
		canvas.SetInteraction(hovered, ui.ID{}, o.InputID, false)
		return PaintSingleSelect(&canvas, ui.LightTheme(), ui.Rect{X: 20, Y: 20, Width: 240, Height: 32}, s, o).Popup
	}
	one := paint(ui.ID{})
	both := paint(o.OptionIDs.At(0))
	if s.tree.depth != 1 || both.Width <= one.Width {
		t.Fatalf("depth %d popup %+v -> %+v: the fly-out did not open at once", s.tree.depth, one, both)
	}
	if panel := s.tree.panels[1]; panel.X < s.tree.panels[0].X || panel.X+panel.Width > 800 || panel.Y < 0 || panel.Y+panel.Height > 600 {
		t.Fatalf("fly-out %+v outside the viewport", panel)
	}
	// Another row of the first level takes the fly-out away only after resting.
	paint(o.OptionIDs.At(1))
	if s.tree.depth != 1 {
		t.Fatal("the fly-out closed the moment the pointer crossed another row")
	}
	if _, ok := canvas.NextFrame(); !ok {
		t.Fatal("no frame was requested to finish the switch delay")
	}
	now += hierarchySwitchDelay + time.Millisecond
	paint(o.OptionIDs.At(1))
	if s.tree.depth != 0 {
		t.Fatalf("depth %d after resting on a row with no children", s.tree.depth)
	}
}

func TestSingleSelectTreeBeyondSupportedDepthIsFlat(t *testing.T) {
	s, _ := hierarchicalSelectFixture()
	s.SetOptions([]SingleSelectOption{{Value: strings.Repeat("a/", hierarchyMaxLevels) + "z"}})
	if s.treeActive() {
		t.Fatal("a value nested past the supported depth must not use the tree")
	}
}

func BenchmarkSingleSelectTreePaint(b *testing.B) {
	s, o := hierarchicalSelectFixture()
	s.SetValue("School/Gansu/Tianshui")
	s.Open = true
	var canvas ui.Canvas
	paint := func() {
		canvas.Reset()
		PaintSingleSelect(&canvas, ui.LightTheme(), ui.Rect{X: 20, Y: 20, Width: 240, Height: 32}, s, o)
	}
	paint()
	b.ReportAllocs()
	for b.Loop() {
		paint()
	}
}

// The menu stands alone, anchored under any control: it chooses by path, owns no
// open flag, and its overlay reports a press outside it.
func TestHierarchyMenuStandsAloneUnderAnAnchor(t *testing.T) {
	var tree ui.Tree
	f := tree.Feature("hierarchy-menu")
	o := HierarchyMenuOptions{OptionIDs: tree.Sub(f, tree.Name("option")), ScrollID: tree.ID(f, "scroll"), Scope: tree.ID(f, "scope"), Layer: 4, Label: "Groups", Viewport: ui.Rect{Width: 800, Height: 600}, Overlay: true, DismissID: tree.ID(f, "dismiss")}
	values := []string{"Gansu/Lanzhou", "Gansu/Tianshui", "Xinjiang"}
	var m HierarchyMenuState
	if !m.Build("/", "", nil, len(values), func(i int) HierarchyMenuItem { return HierarchyMenuItem{Value: values[i]} }) {
		t.Fatal("Build refused ordinary values")
	}
	var canvas ui.Canvas
	canvas.Reset()
	popup := PaintHierarchyMenu(&canvas, ui.LightTheme(), ui.Rect{X: 40, Y: 20, Width: 100, Height: 26}, &m, o)
	if popup.Width < 100 || popup.Y < 46 {
		t.Fatalf("popup %+v is not under its anchor", popup)
	}
	if result := m.Key(ui.KeyEvent{Key: ui.KeyDown}, false); !result.Opened {
		t.Fatal("Down on a hidden menu did not ask to show it")
	}
	m.Key(ui.KeyEvent{Key: ui.KeyRight}, true)
	m.Key(ui.KeyEvent{Key: ui.KeyDown}, true)
	if result := m.Key(ui.KeyEvent{Key: ui.KeyEnter}, true); !result.Picked || result.Chosen != "Gansu/Tianshui" {
		t.Fatalf("Enter picked %q (%v)", result.Chosen, result.Picked)
	}
	if m.Key(ui.KeyEvent{Key: ui.KeyEscape}, true); m.Depth() != 0 {
		t.Fatalf("Escape left %d fly-outs open", m.Depth())
	}
	if m.Build("/", "", nil, 1, func(int) HierarchyMenuItem {
		return HierarchyMenuItem{Value: strings.Repeat("a/", hierarchyMaxLevels) + "z"}
	}) {
		t.Fatal("Build accepted a value nested past the supported depth")
	}
}
