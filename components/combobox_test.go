package components

import (
	"slices"
	"testing"

	"github.com/vivaui/viva/ui"
)

func TestComboboxCapturesEditableOverlayContract(t *testing.T) {
	var canvas ui.Canvas
	combobox := BeginCombobox(&canvas, ui.LightTheme(), ui.Rect{X: 10, Y: 12, Width: 180}, ComboboxOptions{
		InputID: ui.Item(1), ClearID: ui.Item(2), Label: "Framework", Value: "React", Query: "Re", Open: true,
		ShowTrigger: true, ShowClear: true, Content: ui.Rect{X: 10, Y: 50, Width: 180, Height: 116},
		Layer: 9, Scope: ui.Item(55), DismissID: ui.Item(77),
	})
	if combobox.InputBounds().Height != 32 || combobox.ContentBounds().Width != 208 {
		t.Fatalf("geometry input=%+v content=%+v", combobox.InputBounds(), combobox.ContentBounds())
	}
	if canvas.TrappedFocusScope() != ui.Item(55) || canvas.EscapeTarget() != ui.Item(77) {
		t.Fatalf("overlay contract scope=%d escape=%d", canvas.TrappedFocusScope(), canvas.EscapeTarget())
	}
	combobox.AddLabel("Libraries")
	selected := combobox.AddItem(ComboboxItem{ID: ui.Item(3), Label: "React", Value: "react", Selected: true})
	combobox.AddItem(ComboboxItem{ID: ui.Item(4), Label: "Unavailable", Disabled: true})
	if selected != (ui.Rect{X: 14, Y: 82, Width: 200, Height: 32}) {
		t.Fatalf("selected bounds = %+v", selected)
	}
	hits := nonBlockHits(canvas.Hits())
	if len(hits) != 4 {
		t.Fatalf("hits = %+v", hits)
	}
	if input := hits[0]; input.Role != ui.RoleCombobox || input.Value != "React" || !input.Expanded {
		t.Fatalf("input hit = %+v", input)
	}
	if clear := hits[1]; clear.Role != ui.RoleButton || clear.ID != ui.Item(2) {
		t.Fatalf("clear hit = %+v", clear)
	}
	if item := hits[2]; item.Role != ui.RoleOption || item.Scope != ui.Item(55) || !item.Selected {
		t.Fatalf("selected item = %+v", item)
	}
	if disabled := hits[3]; !disabled.Disabled || disabled.Focusable {
		t.Fatalf("disabled item = %+v", disabled)
	}
	for _, rectangle := range canvas.Rectangles()[2:] {
		if rectangle.Layer != 9 {
			t.Fatalf("popup rectangle layer = %+v", rectangle)
		}
	}
}

func TestComboboxPaintIsDeterministicAndAllocationFreeAfterWarmup(t *testing.T) {
	var canvas ui.Canvas
	paint := func() {
		canvas.Reset()
		combobox := BeginCombobox(&canvas, ui.DarkTheme(), ui.Rect{X: 2, Y: 3, Width: 160}, ComboboxOptions{
			InputID: ui.Item(1), Label: "Language", Placeholder: "Select language", Open: true, ShowTrigger: true,
			Content: ui.Rect{X: 2, Y: 41, Width: 160, Height: 108}, Layer: 4, Scope: ui.Item(6), DismissID: ui.Item(8),
		})
		combobox.AddLabel("Popular")
		combobox.AddItem(ComboboxItem{ID: ui.Item(2), Label: "Go", Value: "go"})
		combobox.AddSeparator()
		combobox.PaintChip(ui.Rect{X: 2, Y: 154, Width: 70, Height: 24}, ComboboxChip{RemoveID: ui.Item(3), Label: "Go", Value: "go"})
	}
	paint()
	rectangles := slices.Clone(canvas.Rectangles())
	texts := slices.Clone(canvas.Texts())
	hits := slices.Clone(canvas.Hits())
	paint()
	if !slices.Equal(rectangles, canvas.Rectangles()) || !slices.Equal(texts, canvas.Texts()) || !slices.Equal(hits, canvas.Hits()) {
		t.Fatal("identical combobox snapshots produced different streams")
	}
	if allocations := testing.AllocsPerRun(100, paint); allocations != 0 {
		t.Fatalf("combobox paint allocations = %v", allocations)
	}
}

// A selected row is told apart by its check; only the row the caller marked
// highlighted is tinted, so a long multi-select list does not read as a wall of
// highlights and the pointer cannot tint a second row beside the one Enter names.
func TestComboboxTintsOnlyTheHighlightedRowNotTheSelectedOne(t *testing.T) {
	theme := ui.LightTheme()
	paint := func(hovered, highlighted ui.ID) []ui.Rectangle {
		var canvas ui.Canvas
		canvas.SetInteraction(hovered, ui.ID{}, ui.ID{}, false)
		combobox := BeginCombobox(&canvas, theme, ui.Rect{Width: 180}, ComboboxOptions{
			InputID: ui.Item(1), Label: "Framework", Open: true, Content: ui.Rect{Y: 40, Width: 200, Height: 100}, Layer: 3,
		})
		combobox.AddItem(ComboboxItem{ID: ui.Item(3), Label: "React", Selected: true, Highlighted: highlighted == ui.Item(3)})
		combobox.AddItem(ComboboxItem{ID: ui.Item(4), Label: "Vue", Highlighted: highlighted == ui.Item(4)})
		return canvas.Rectangles()
	}
	selectedOnly := paint(ui.ID{}, ui.ID{})
	if highlighted := paint(ui.ID{}, ui.Item(4)); len(highlighted) != len(selectedOnly)+1 {
		t.Fatalf("the highlighted row added %d rectangles, want its tint alone", len(highlighted)-len(selectedOnly))
	}
	if hovered := paint(ui.Item(4), ui.ID{}); len(hovered) != len(selectedOnly) {
		t.Fatalf("hovering a row the caller did not highlight added %d rectangles", len(hovered)-len(selectedOnly))
	}
}

func TestComboboxEmptyRowIsOneLineHigh(t *testing.T) {
	var canvas ui.Canvas
	combobox := BeginCombobox(&canvas, ui.LightTheme(), ui.Rect{Width: 180}, ComboboxOptions{
		InputID: ui.Item(1), Label: "Framework", Open: true, Content: ui.Rect{Y: 40, Width: 200, Height: 100},
	})
	if empty := combobox.AddEmpty("No results"); empty.Height >= 56 || empty.Height < 20 {
		t.Fatalf("empty row = %+v, want a short row of one line with padding", empty)
	}
}

func TestComboboxViewportDismissesOnOutsidePress(t *testing.T) {
	var canvas ui.Canvas
	BeginCombobox(&canvas, ui.LightTheme(), ui.Rect{Width: 180}, ComboboxOptions{
		InputID: ui.Item(1), Label: "Framework", Open: true, Content: ui.Rect{Y: 40, Width: 200, Height: 100},
		DismissID: ui.Item(9), Viewport: ui.Rect{Width: 800, Height: 600},
	})
	dismiss := false
	for _, hit := range canvas.Hits() {
		if hit.ID == ui.Item(9) && hit.Bounds == (ui.Rect{Width: 800, Height: 600}) {
			dismiss = true
		}
	}
	if !dismiss {
		t.Fatalf("no dismiss hit over the viewport: %+v", canvas.Hits())
	}
}

func TestComboboxChipsWrapAndGrowTheContainer(t *testing.T) {
	theme := ui.LightTheme()
	chips := []ComboboxChip{
		{RemoveID: ui.Item(11), Label: "React"}, {RemoveID: ui.Item(12), Label: "Vue"},
		{RemoveID: ui.Item(13), Label: "Svelte"}, {RemoveID: ui.Item(14), Label: "Angular"},
	}
	var scratch [8]ui.Rect
	paint := func(width float32, chips []ComboboxChip) (ComboboxChipsLayout, *ui.Canvas) {
		var canvas ui.Canvas
		layout := PaintComboboxChips(&canvas, theme, ui.Rect{X: 5, Y: 6, Width: width}, chips, ComboboxChipsOptions{
			InputID: ui.Item(1), Label: "Frameworks", Placeholder: "Add", Focused: true,
		}, scratch[:0])
		return layout, &canvas
	}
	wide, _ := paint(600, chips)
	narrow, canvas := paint(150, chips)
	if len(narrow.Chips) != len(chips) {
		t.Fatalf("laid out %d chips, want %d", len(narrow.Chips), len(chips))
	}
	if narrow.Bounds.Height <= wide.Bounds.Height {
		t.Fatalf("narrow container is %v high, wide one %v; wrapping must make it taller", narrow.Bounds.Height, wide.Bounds.Height)
	}
	for index, chip := range narrow.Chips {
		if chip.X < narrow.Bounds.X || chip.X+chip.Width > narrow.Bounds.X+narrow.Bounds.Width || chip.Y+chip.Height > narrow.Bounds.Y+narrow.Bounds.Height {
			t.Errorf("chip %d %+v leaves the container %+v", index, chip, narrow.Bounds)
		}
		for other := index + 1; other < len(narrow.Chips); other++ {
			o := narrow.Chips[other]
			if chip.X < o.X+o.Width && o.X < chip.X+chip.Width && chip.Y < o.Y+o.Height && o.Y < chip.Y+chip.Height {
				t.Errorf("chips %d and %d overlap: %+v %+v", index, other, chip, o)
			}
		}
	}
	if narrow.Input.Y < narrow.Chips[len(chips)-1].Y {
		t.Fatalf("the input %+v sits above the last chip %+v", narrow.Input, narrow.Chips[len(chips)-1])
	}
	removes := 0
	for _, hit := range canvas.Hits() {
		if hit.Role == ui.RoleButton {
			removes++
		}
	}
	if removes != len(chips) {
		t.Fatalf("%d remove buttons, want %d", removes, len(chips))
	}
	empty, _ := paint(300, nil)
	if empty.Bounds.Height != theme.Metrics.ControlHeight {
		t.Fatalf("an empty chips container is %v high, want one control height", empty.Bounds.Height)
	}
	var again ui.Canvas
	if allocations := testing.AllocsPerRun(100, func() {
		again.Reset()
		PaintComboboxChips(&again, theme, ui.Rect{Width: 300}, chips, ComboboxChipsOptions{InputID: ui.Item(1), Label: "x"}, scratch[:0])
	}); allocations != 0 {
		t.Fatalf("chips paint allocated %v times", allocations)
	}
}

// A combobox whose options do not all fit scrolls through them, and the
// keyboard can navigate and typeahead the last paint's rows.
func TestComboboxScrollsAndNavigatesItsOptions(t *testing.T) {
	var canvas ui.Canvas
	var scroll MenuScroll
	combobox := BeginCombobox(&canvas, ui.LightTheme(), ui.Rect{Width: 200}, ComboboxOptions{
		InputID: ui.Item(1), Label: "Shell", Open: true,
		Content: ui.Rect{X: 0, Y: 40, Width: 200, Height: 80}, Layer: 4, Scroll: &scroll,
	})
	for index := 0; index < 8; index++ {
		combobox.AddItem(ComboboxItem{ID: ui.Item(10 + index), Label: "Shell " + string(rune('a'+index))})
	}
	combobox.End()
	if scroll.Max() <= 0 {
		t.Fatal("eight rows did not overflow an 80px popup")
	}
	id, ok := combobox.Navigate(ui.KeyEnd, ui.Item(10), false)
	if !ok || id != ui.Item(17) {
		t.Fatalf("End = (%v, %v), want the last option", id, ok)
	}
	if scroll.Offset <= 0 {
		t.Fatal("End did not scroll the last option into view")
	}
	if id, ok := combobox.Match("shell h", ui.ID{}); !ok || id != ui.Item(17) {
		t.Fatalf("Match = (%v, %v)", id, ok)
	}
}
