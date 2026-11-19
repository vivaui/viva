package componentdemo

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/togglegroup"
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
}`, PageSnippet: `group := m.toggleGroup
group.Items("Left", "Center", "Right").
    Single(&m.alignment).Outline().SizeToFit()
group.View(c, "Text alignment")`, PageSnippetZh: `group := m.toggleGroup
group.Items("左对齐", "居中", "右对齐").
    Single(&m.alignment).Outline().SizeToFit()
group.View(c, "文本对齐")`},
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
}`, PageSnippet: `group := m.toggleGroup
group.Items("Bold", "Italic", "Underline").
    Multiple(m.styles[:]).Outline().SizeToFit()
group.View(c, "Text styles")`, PageSnippetZh: `group := m.toggleGroup
group.Items("粗体", "斜体", "下划线").
    Multiple(m.styles[:]).Outline().SizeToFit()
group.View(c, "文本样式")`},
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
}`, PageSnippet: `group := m.toggleGroup
group.Items("Left", "Center", "Right").
    Single(&m.alignment).Disable(2).Vertical().Outline()
group.View(c, "Text alignment").Width(168)`, PageSnippetZh: `group := m.toggleGroup
group.Items("左对齐", "居中", "右对齐").
    Single(&m.alignment).Disable(2).Vertical().Outline()
group.View(c, "文本对齐").Width(168)`},
	},
}

var (
	toggleGroupUI = tree.Feature("toggle-group")
	// toggleGroupItem's item i is the i-th button of the group.
	toggleGroupItem = tree.Sub(toggleGroupUI, tree.Name("item"))
)

// toggleGroupDemos keeps each State's configured page control. The group lives
// beside the State rather than in it because State must stay comparable, and
// one group per State lets the single, multiple and vertical examples share
// the same preview code.
var toggleGroupDemos = map[*State]*toggleGroupDemo{}

type toggleGroupDemo struct {
	state    *State
	group    togglegroup.Group
	selected int
	active   [3]bool
	multiple bool
}

func toggleGroupDemoFor(state *State) *toggleGroupDemo {
	if demo := toggleGroupDemos[state]; demo != nil {
		return demo
	}
	demo := &toggleGroupDemo{state: state, multiple: state.exampleID == "multiple"}
	labels := toggleGroupLabels(state)
	demo.group.Items(labels[:]...).Outline()
	switch {
	case demo.multiple:
		demo.active[0] = true
		demo.group.Multiple(demo.active[:]).SizeToFit()
	case state.exampleID == "vertical":
		demo.selected = 0
		demo.group.Single(&demo.selected).Vertical().Disable(2)
	default:
		demo.selected = 0
		demo.group.Single(&demo.selected).SizeToFit()
	}
	demo.group.OnChange(demo.changed)
	for index := range labels {
		demo.group.ItemID(index, toggleGroupItem.At(index))
	}
	toggleGroupDemos[state] = demo
	return demo
}

func (demo *toggleGroupDemo) changed(index int, active bool) {
	bit := uint64(1) << index
	if !demo.multiple {
		demo.state.flags = bit
		return
	}
	if active {
		demo.state.flags |= bit
	} else {
		demo.state.flags &^= bit
	}
}

func toggleGroupLabels(state *State) [3]string {
	if state.exampleID == "multiple" {
		return [3]string{
			state.Text("粗体", "Bold"),
			state.Text("斜体", "Italic"),
			state.Text("下划线", "Underline"),
		}
	}
	return [3]string{
		state.Text("左对齐", "Left"),
		state.Text("居中", "Center"),
		state.Text("右对齐", "Right"),
	}
}

func (state *State) initializeToggleGroup() {
	state.flags = 1
	toggleGroupDemoFor(state)
}

func (state *State) keyToggleGroup(event gfx.KeyEvent) gfx.EventResult {
	demo := toggleGroupDemos[state]
	if demo == nil {
		return gfx.EventResult{}
	}
	return demo.group.Key(event)
}

func (state *State) paintToggleGroup(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	demo := toggleGroupDemoFor(state)
	captionText := state.Text("单选模式由调用方在点击时替换选择位。", "Single selection: the caller replaces the selected bit on click.")
	controlLabel := state.Text("文本对齐", "Text alignment")
	if demo.multiple {
		captionText = state.Text("多选模式由调用方独立翻转每个选择位。", "Multiple selection: the caller flips each bit independently.")
		controlLabel = state.Text("文本样式", "Text styles")
	} else if state.exampleID == "vertical" {
		captionText = state.Text("第三项禁用；上下方向键会自动跳过。", "The third item is disabled; arrow keys skip it.")
	}
	state.drawPage(canvas, theme, size, 500, state.Text("Toggle Group · 显式选择", "Toggle Group · Explicit selection"), func(c ui.C) {
		box := demo.group.View(c, controlLabel)
		if state.exampleID == "vertical" {
			box.Width(168)
		}
		caption(c, captionText)
	})
}
