package componentdemo

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var toggleGroupComponent = Component{
	ID: "toggle-group", Title: "Toggle Group", Category: "Actions", Summary: "支持单选或多选的稳定按钮组。",
	paint: (*State).paintToggleGroup, key: (*State).keyToggleGroup, initialize: (*State).initializeToggleGroup,
	Examples: []Example{
		{ID: "single", Title: "文本对齐", Description: "一个调用方持有的单选按钮组。", Snippet: `items := tree.Sub(page, tree.Name("item"))
options := []components.ToggleGroupOption{
    {ID: items.At(0), Label: "Left", Active: selected == 0},
    {ID: items.At(1), Label: "Center", Active: selected == 1},
    {ID: items.At(2), Label: "Right", Active: selected == 2},
}
group := components.BeginToggleGroup(canvas, theme, bounds, options).Outline().Spacing(0)
if id, ok := group.Paint().Clicked(); ok {
    selected, _ = id.In(items)
}`},
		{ID: "multiple", Title: "文本样式", Description: "每个选项独立切换的多选按钮组。", Snippet: `items := tree.Sub(page, tree.Name("item"))
options := []components.ToggleGroupOption{
    {ID: items.At(0), Label: "Bold", Active: flags&1 != 0},
    {ID: items.At(1), Label: "Italic", Active: flags&2 != 0},
    {ID: items.At(2), Label: "Underline", Active: flags&4 != 0},
}
group := components.BeginToggleGroup(canvas, theme, bounds, options).Outline()
group = group.Spacing(0).SizeToFit()
if id, ok := group.Paint().Clicked(); ok {
    index, _ := id.In(items)
    flags ^= 1 << index
}`},
		{ID: "vertical", Title: "纵向与禁用", Description: "纵向描边组，包含一个禁用选项。", Snippet: `items := tree.Sub(page, tree.Name("item"))
options := []components.ToggleGroupOption{
    {ID: items.At(0), Label: "Left", Active: selected == 0},
    {ID: items.At(1), Label: "Center", Active: selected == 1},
    {ID: items.At(2), Label: "Right", Disabled: true},
}
group := components.BeginToggleGroup(canvas, theme, bounds, options).Vertical()
group = group.Outline().Spacing(1)
if id, ok := group.Paint().Clicked(); ok {
    selected, _ = id.In(items)
}`},
	},
}

var (
	toggleGroupUI = tree.Feature("toggle-group")
	// toggleGroupItem's item i is the i-th button of the group (bit i of flags).
	toggleGroupItem = tree.Sub(toggleGroupUI, tree.Name("item"))
)

func (state *State) initializeToggleGroup() { state.flags = 1 }

func (state *State) keyToggleGroup(event gfx.KeyEvent) gfx.EventResult {
	if listIndex(event.Target, toggleGroupItem) < 0 {
		return gfx.EventResult{}
	}
	options := state.toggleGroupOptions()
	orientation := components.ToggleGroupHorizontal
	if state.exampleID == "vertical" {
		orientation = components.ToggleGroupVertical
	}
	next, handled := components.ToggleGroupKeyDown(event.Target, options[:], orientation, components.DirectionLTR, event.Key)
	if !handled {
		return gfx.EventResult{}
	}
	changed := false
	if state.exampleID != "multiple" {
		bit := uint64(1) << listIndex(next, toggleGroupItem)
		changed = state.flags != bit
		state.flags = bit
	}
	return gfx.EventResult{Focus: next, FocusVisible: true, Handled: true, Repaint: changed}
}

func (state *State) toggleGroupOptions() [3]components.ToggleGroupOption {
	labels := [...]string{
		state.Text("左对齐", "Left"),
		state.Text("居中", "Center"),
		state.Text("右对齐", "Right"),
	}
	if state.exampleID == "multiple" {
		labels = [...]string{
			state.Text("粗体", "Bold"),
			state.Text("斜体", "Italic"),
			state.Text("下划线", "Underline"),
		}
	}
	options := [3]components.ToggleGroupOption{
		{ID: toggleGroupItem.At(0), Label: labels[0], Active: state.flags&1 != 0},
		{ID: toggleGroupItem.At(1), Label: labels[1], Active: state.flags&2 != 0},
		{ID: toggleGroupItem.At(2), Label: labels[2], Active: state.flags&4 != 0},
	}
	if state.exampleID == "vertical" {
		options[2].Disabled = true
	}
	return options
}

func (state *State) paintToggleGroup(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	var group ui.Box
	vertical := state.exampleID == "vertical"
	state.page(canvas, theme, size, 500, state.Text("Toggle Group · 显式选择", "Toggle Group · Explicit selection"), func(f *ui.Frame) {
		if vertical {
			group = f.Slot().FillWidth().MaxWidth(168).Height(theme.Metrics.ControlHeight * 3).Self(ui.Start)
			caption(f, state.Text("第三项禁用；上下方向键会自动跳过。", "The third item is disabled; arrow keys skip it."))
			return
		}
		group = f.Slot().Height(theme.Metrics.ControlHeight)
		text := state.Text("单选模式由调用方在点击时替换选择位。", "Single selection: the caller replaces the selected bit on click.")
		if state.exampleID == "multiple" {
			text = state.Text("多选模式由调用方独立翻转每个选择位。", "Multiple selection: the caller flips each bit independently.")
		}
		caption(f, text)
	})

	options := state.toggleGroupOptions()
	// The horizontal examples are joined into one control (Spacing(0)); the
	// multiple-selection one is also sized to its labels, not to the slot.
	toggleGroup := components.BeginToggleGroup(canvas, theme, group.Rect(), options[:]).Outline().Spacing(0)
	if vertical {
		toggleGroup = toggleGroup.Vertical().Spacing(1)
	} else if state.exampleID == "multiple" {
		toggleGroup = toggleGroup.SizeToFit()
	}
	if id, ok := toggleGroup.Paint().Clicked(); ok {
		bit := uint64(1) << listIndex(id, toggleGroupItem)
		if state.exampleID == "multiple" {
			state.flags ^= bit
		} else {
			state.flags = bit
		}
	}
}
