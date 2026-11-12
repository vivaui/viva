package components

import (
	"slices"
	"testing"

	"github.com/vivaui/viva/gfx"
)

func TestComboboxCapturesEditableOverlayContract(t *testing.T) {
	var canvas gfx.Canvas
	combobox := BeginCombobox(&canvas, gfx.LightTheme(), gfx.Rect{X: 10, Y: 12, Width: 180}, ComboboxOptions{
		InputID: gfx.Item(1), ClearID: gfx.Item(2), Label: "Framework", Value: "React", Query: "Re", Open: true,
		ShowTrigger: true, ShowClear: true, Content: gfx.Rect{X: 10, Y: 50, Width: 180, Height: 116},
		Layer: 9, Scope: gfx.Item(55), DismissID: gfx.Item(77),
	})
	if combobox.InputBounds().Height != 32 || combobox.ContentBounds().Width != 208 {
		t.Fatalf("geometry input=%+v content=%+v", combobox.InputBounds(), combobox.ContentBounds())
	}
	if canvas.TrappedFocusScope() != gfx.Item(55) || canvas.EscapeTarget() != gfx.Item(77) {
		t.Fatalf("overlay contract scope=%d escape=%d", canvas.TrappedFocusScope(), canvas.EscapeTarget())
	}
	combobox.AddLabel("Libraries")
	selected := combobox.AddItem(ComboboxItem{ID: gfx.Item(3), Label: "React", Value: "react", Selected: true})
	combobox.AddItem(ComboboxItem{ID: gfx.Item(4), Label: "Unavailable", Disabled: true})
	if selected != (gfx.Rect{X: 14, Y: 82, Width: 200, Height: 32}) {
		t.Fatalf("selected bounds = %+v", selected)
	}
	hits := nonBlockHits(canvas.Hits())
	if len(hits) != 4 {
		t.Fatalf("hits = %+v", hits)
	}
	if input := hits[0]; input.Role != gfx.RoleCombobox || input.Value != "React" || !input.Expanded {
		t.Fatalf("input hit = %+v", input)
	}
	if clear := hits[1]; clear.Role != gfx.RoleButton || clear.ID != gfx.Item(2) {
		t.Fatalf("clear hit = %+v", clear)
	}
	if item := hits[2]; item.Role != gfx.RoleOption || item.Scope != gfx.Item(55) || !item.Selected {
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
	var canvas gfx.Canvas
	paint := func() {
		canvas.Reset()
		combobox := BeginCombobox(&canvas, gfx.DarkTheme(), gfx.Rect{X: 2, Y: 3, Width: 160}, ComboboxOptions{
			InputID: gfx.Item(1), Label: "Language", Placeholder: "Select language", Open: true, ShowTrigger: true,
			Content: gfx.Rect{X: 2, Y: 41, Width: 160, Height: 108}, Layer: 4, Scope: gfx.Item(6), DismissID: gfx.Item(8),
		})
		combobox.AddLabel("Popular")
		combobox.AddItem(ComboboxItem{ID: gfx.Item(2), Label: "Go", Value: "go", Detail: "Backend"})
		combobox.AddSeparator()
		combobox.PaintChip(gfx.Rect{X: 2, Y: 154, Width: 70, Height: 24}, ComboboxChip{RemoveID: gfx.Item(3), Label: "Go", Value: "go"})
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

func TestComboboxQueryPaintingFollowsNativeInputOwnership(t *testing.T) {
	theme := gfx.DarkTheme()
	inputID := gfx.Item(1)
	for _, query := range []string{"host-name-with-a-long-address.example", "שלום עולם בדיקת ממשק בחירת שרת", "开发主机"} {
		for _, state := range []struct {
			name     string
			focused  bool
			disabled bool
			noInput  bool
		}{
			{name: "native editor", focused: true},
			{name: "unfocused"},
			{name: "disabled", focused: true, disabled: true},
			{name: "display only", noInput: true},
		} {
			t.Run(state.name+"/"+query, func(t *testing.T) {
				var canvas gfx.Canvas
				options := ComboboxOptions{InputID: inputID, Label: "Host", Query: query, Disabled: state.disabled}
				if state.noInput {
					options.InputID = gfx.ID{}
				}
				if state.focused {
					canvas.SetInteraction(gfx.ID{}, gfx.ID{}, inputID, true)
				}
				BeginCombobox(&canvas, theme, gfx.Rect{Width: 180}, options).End()
				native := state.focused && !state.disabled && !state.noInput
				if native && len(canvas.Texts()) != 0 {
					t.Fatalf("component painted query owned by the native editor: %+v", canvas.Texts())
				}
				if !native && len(canvas.Texts()) == 0 {
					t.Fatal("query without a native editor is not displayed")
				}
				if !state.noInput {
					hits := nonBlockHits(canvas.Hits())
					if len(hits) != 1 {
						t.Fatalf("editable input geometry missing: %+v", hits)
					}
					geometry, ok := canvas.HitText(&hits[0])
					if !ok || geometry.Bounds.Width <= 0 || geometry.Bounds.Height <= 0 {
						t.Fatalf("editable input geometry missing: %+v", geometry)
					}
				}
			})
		}
	}
}

func TestComboboxEmptyQueryKeepsSelectedDetailAndPlaceholder(t *testing.T) {
	inputID := gfx.Item(1)
	for _, focused := range []bool{false, true} {
		for _, selected := range []bool{false, true} {
			var canvas gfx.Canvas
			if focused {
				canvas.SetInteraction(gfx.ID{}, gfx.ID{}, inputID, true)
			}
			options := ComboboxOptions{InputID: inputID, Label: "Host", Placeholder: "Choose a host"}
			want := []string{options.Placeholder}
			if selected {
				options.Value, options.Detail = "server", "192.0.2.1"
				want = []string{options.Value, options.Detail}
			}
			BeginCombobox(&canvas, gfx.DarkTheme(), gfx.Rect{Width: 320}, options).End()
			for _, value := range want {
				if !slices.ContainsFunc(canvas.Texts(), func(text gfx.Text) bool { return text.Value == value }) {
					t.Fatalf("focused=%v selected=%v: missing static text %q", focused, selected, value)
				}
			}
		}
	}
}

func BenchmarkComboboxEditableDetail(b *testing.B) {
	var canvas gfx.Canvas
	theme := gfx.DarkTheme()
	inputID := gfx.Item(1)
	options := ComboboxOptions{InputID: inputID, Label: "Host", Value: "server", Detail: "192.0.2.1", Query: "server.example"}
	paint := func() {
		canvas.Reset()
		canvas.SetInteraction(gfx.ID{}, gfx.ID{}, inputID, true)
		BeginCombobox(&canvas, theme, gfx.Rect{Width: 240}, options).End()
	}
	paint()
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		paint()
	}
}

// A selected row is told apart by its check; only the row the caller marked
// highlighted is tinted, so a long multi-select list does not read as a wall of
// highlights and the pointer cannot tint a second row beside the one Enter names.
func TestComboboxTintsOnlyTheHighlightedRowNotTheSelectedOne(t *testing.T) {
	theme := gfx.LightTheme()
	paint := func(hovered, highlighted gfx.ID) []gfx.Rectangle {
		var canvas gfx.Canvas
		canvas.SetInteraction(hovered, gfx.ID{}, gfx.ID{}, false)
		combobox := BeginCombobox(&canvas, theme, gfx.Rect{Width: 180}, ComboboxOptions{
			InputID: gfx.Item(1), Label: "Framework", Open: true, Content: gfx.Rect{Y: 40, Width: 200, Height: 100}, Layer: 3,
		})
		combobox.AddItem(ComboboxItem{ID: gfx.Item(3), Label: "React", Selected: true, Highlighted: highlighted == gfx.Item(3)})
		combobox.AddItem(ComboboxItem{ID: gfx.Item(4), Label: "Vue", Highlighted: highlighted == gfx.Item(4)})
		return canvas.Rectangles()
	}
	selectedOnly := paint(gfx.ID{}, gfx.ID{})
	if highlighted := paint(gfx.ID{}, gfx.Item(4)); len(highlighted) != len(selectedOnly)+1 {
		t.Fatalf("the highlighted row added %d rectangles, want its tint alone", len(highlighted)-len(selectedOnly))
	}
	if hovered := paint(gfx.Item(4), gfx.ID{}); len(hovered) != len(selectedOnly) {
		t.Fatalf("hovering a row the caller did not highlight added %d rectangles", len(hovered)-len(selectedOnly))
	}
}

func TestComboboxEmptyRowIsOneLineHigh(t *testing.T) {
	var canvas gfx.Canvas
	combobox := BeginCombobox(&canvas, gfx.LightTheme(), gfx.Rect{Width: 180}, ComboboxOptions{
		InputID: gfx.Item(1), Label: "Framework", Open: true, Content: gfx.Rect{Y: 40, Width: 200, Height: 100},
	})
	if empty := combobox.AddEmpty("No results"); empty.Height >= 56 || empty.Height < 20 {
		t.Fatalf("empty row = %+v, want a short row of one line with padding", empty)
	}
}

func TestComboboxViewportDismissesOnOutsidePress(t *testing.T) {
	var canvas gfx.Canvas
	BeginCombobox(&canvas, gfx.LightTheme(), gfx.Rect{Width: 180}, ComboboxOptions{
		InputID: gfx.Item(1), Label: "Framework", Open: true, Content: gfx.Rect{Y: 40, Width: 200, Height: 100},
		DismissID: gfx.Item(9), Viewport: gfx.Rect{Width: 800, Height: 600},
	})
	dismiss := false
	for _, hit := range canvas.Hits() {
		if hit.ID == gfx.Item(9) && hit.Bounds == (gfx.Rect{Width: 800, Height: 600}) {
			dismiss = true
		}
	}
	if !dismiss {
		t.Fatalf("no dismiss hit over the viewport: %+v", canvas.Hits())
	}
}

func TestComboboxChipsWrapAndGrowTheContainer(t *testing.T) {
	theme := gfx.LightTheme()
	chips := []ComboboxChip{
		{RemoveID: gfx.Item(11), Label: "React"}, {RemoveID: gfx.Item(12), Label: "Vue"},
		{RemoveID: gfx.Item(13), Label: "Svelte"}, {RemoveID: gfx.Item(14), Label: "Angular"},
	}
	var scratch [8]gfx.Rect
	paint := func(width float32, chips []ComboboxChip) (ComboboxChipsLayout, *gfx.Canvas) {
		var canvas gfx.Canvas
		layout := PaintComboboxChips(&canvas, theme, gfx.Rect{X: 5, Y: 6, Width: width}, chips, ComboboxChipsOptions{
			InputID: gfx.Item(1), Label: "Frameworks", Placeholder: "Add", Focused: true,
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
		if hit.Role == gfx.RoleButton {
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
	var again gfx.Canvas
	if allocations := testing.AllocsPerRun(100, func() {
		again.Reset()
		PaintComboboxChips(&again, theme, gfx.Rect{Width: 300}, chips, ComboboxChipsOptions{InputID: gfx.Item(1), Label: "x"}, scratch[:0])
	}); allocations != 0 {
		t.Fatalf("chips paint allocated %v times", allocations)
	}
}

// A combobox whose options do not all fit scrolls through them, and the
// keyboard can navigate and typeahead the last paint's rows.
func TestComboboxScrollsAndNavigatesItsOptions(t *testing.T) {
	var canvas gfx.Canvas
	var scroll MenuScroll
	combobox := BeginCombobox(&canvas, gfx.LightTheme(), gfx.Rect{Width: 200}, ComboboxOptions{
		InputID: gfx.Item(1), Label: "Shell", Open: true,
		Content: gfx.Rect{X: 0, Y: 40, Width: 200, Height: 80}, Layer: 4, Scroll: &scroll,
	})
	for index := 0; index < 8; index++ {
		combobox.AddItem(ComboboxItem{ID: gfx.Item(10 + index), Label: "Shell " + string(rune('a'+index))})
	}
	combobox.End()
	if scroll.Max() <= 0 {
		t.Fatal("eight rows did not overflow an 80px popup")
	}
	id, ok := combobox.Navigate(gfx.KeyEnd, gfx.Item(10), false)
	if !ok || id != gfx.Item(17) {
		t.Fatalf("End = (%v, %v), want the last option", id, ok)
	}
	if scroll.Offset <= 0 {
		t.Fatal("End did not scroll the last option into view")
	}
	if id, ok := combobox.Match("shell h", gfx.ID{}); !ok || id != gfx.Item(17) {
		t.Fatalf("Match = (%v, %v)", id, ok)
	}
}

func TestComboboxItemDetailLayoutAndA11y(t *testing.T) {
	var canvas gfx.Canvas
	theme := gfx.LightTheme()
	combobox := BeginCombobox(&canvas, theme, gfx.Rect{X: 10, Y: 10, Width: 220}, ComboboxOptions{
		InputID: gfx.Item(1), Label: "Server", Open: true,
		Content: gfx.Rect{X: 10, Y: 46, Width: 220, Height: 120}, Layer: 3, Scope: gfx.Item(20),
	})
	combobox.AddItem(ComboboxItem{
		ID: gfx.Item(2), Label: "bastion-gw-1", Detail: "10.0.0.1", Value: "1", Selected: true,
	})
	combobox.AddItem(ComboboxItem{
		ID: gfx.Item(3), Label: "very-long-server-name-that-truncates", Detail: "192.168.1.100:2222", Value: "2",
	})
	combobox.End()

	hits := nonBlockHits(canvas.Hits())
	var item2Hit, item3Hit gfx.Hit
	for _, hit := range hits {
		if hit.ID == gfx.Item(2) {
			item2Hit = hit
		}
		if hit.ID == gfx.Item(3) {
			item3Hit = hit
		}
	}

	// 验证 a11y Description 包含了 Detail 文本，用于读屏播报（aria-description）
	if item2Hit.Description != "10.0.0.1" {
		t.Fatalf("item2 accessible description = %q, want '10.0.0.1'", item2Hit.Description)
	}
	if item3Hit.Description != "192.168.1.100:2222" {
		t.Fatalf("item3 accessible description = %q, want '192.168.1.100:2222'", item3Hit.Description)
	}

	// 检查绘制的文本记录中是否包含 Detail
	foundDetail := false
	for _, txt := range canvas.Texts() {
		if txt.Value == "10.0.0.1" {
			foundDetail = true
			if txt.Style.Size != theme.Metrics.FontSizeS {
				t.Fatalf("detail font size = %v, want %v", txt.Style.Size, theme.Metrics.FontSizeS)
			}
			if txt.Style.Weight != gfx.WeightRegular {
				t.Fatalf("detail font weight = %v, want WeightRegular", txt.Style.Weight)
			}
		}
	}
	if !foundDetail {
		t.Fatal("expected detail text '10.0.0.1' to be painted")
	}
}
