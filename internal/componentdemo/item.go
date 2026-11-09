package componentdemo

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var itemComponent = Component{
	ID: "item", Title: "Item", Category: "Display", Summary: "可交互列表行、密度与表面变体。",
	paint: (*State).paintItem,
	Examples: []Example{
		{ID: "default", Title: "可选择行", Description: "点击生产 Item 行切换调用方持有的选中提示。", Snippet: `id := row.At(0)
trailing := "Open"
if selected {
    trailing = "Selected"
}
layout := components.BeginItem(canvas, theme, id, bounds).
    Variant(components.ItemOutline).LeadingSpace(28).
    Paint("Release notes", "Latest runtime changes.", trailing)
canvas.Rounded(layout.Media, 6, theme.Primary)
if canvas.Clicked(id) {
    selected = !selected
}`},
		{ID: "variants", Title: "表面与密度", Description: "描边、弱化与紧凑尺寸组合。", Snippet: `components.BeginItem(canvas, theme, row.At(0), rowBounds[0]).
    Variant(components.ItemOutline).Paint("Outline", "Default density", "⌘1")
components.BeginItem(canvas, theme, row.At(1), rowBounds[1]).
    Variant(components.ItemMuted).Size(components.ItemSizeSmall).
    Paint("Muted", "Small density", "⌘2")
components.BeginItem(canvas, theme, row.At(2), rowBounds[2]).
    Size(components.ItemSizeExtraSmall).Disabled().
    Paint("Disabled", "", "Unavailable")`},
	},
}

var (
	itemUI = tree.Feature("item")
	// itemRow's item i is the i-th row of the example.
	itemRow = tree.Sub(itemUI, tree.Name("row"))
)

func (state *State) paintItem(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	var rows [3]ui.Box
	state.page(canvas, theme, size, 560, state.Text("Item · 生产列表行", "Item · Production list rows"), func(f *ui.Frame) {
		if state.exampleID == "default" {
			rows[0] = f.Slot().FillWidth().Height(72)
			return
		}
		rows[0] = f.Slot().FillWidth().Height(68)
		rows[1] = f.Slot().FillWidth().Height(54)
		rows[2] = f.Slot().FillWidth().Height(42)
	})
	if state.exampleID == "default" {
		trailing := "Open"
		if state.flags&1 != 0 {
			trailing = "Selected"
		}
		bounds := rows[0].Rect()
		components.BeginItem(canvas, theme, itemRow.At(0), bounds).Variant(components.ItemOutline).LeadingSpace(28).Paint("Release notes", "Latest runtime changes.", trailing)
		canvas.Rounded(gfx.Rect{X: bounds.X + 12, Y: bounds.Y + (bounds.Height-24)/2, Width: 24, Height: 24}, 6, theme.Primary)
		if canvas.Clicked(itemRow.At(0)) {
			state.flags ^= 1
		}
		return
	}
	components.BeginItem(canvas, theme, itemRow.At(0), rows[0].Rect()).Variant(components.ItemOutline).Paint("Outline", "Default density", "⌘1")
	components.BeginItem(canvas, theme, itemRow.At(1), rows[1].Rect()).Variant(components.ItemMuted).Size(components.ItemSizeSmall).Paint("Muted", "Small density", "⌘2")
	components.BeginItem(canvas, theme, itemRow.At(2), rows[2].Rect()).Size(components.ItemSizeExtraSmall).Disabled().Paint("Disabled", "", "Unavailable")
}
