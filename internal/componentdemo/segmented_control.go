package componentdemo

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var segmentedControlComponent = Component{
	ID: "segmented-control", Title: "Segmented Control", Category: "Forms", Summary: "稳定选项顺序与调用方单选状态的紧凑切换器。",
	paint: (*State).paintSegmentedControl, key: (*State).keySegmentedControl,
	Examples: []Example{
		{ID: "default", Title: "视图模式", Description: "切换列表、看板与时间线视图。", Snippet: `view := tree.Sub(page, tree.Name("view"))
options := []components.SegmentOption{
    {ID: view.At(0), Label: "List"},
    {ID: view.At(1), Label: "Board"},
    {ID: view.At(2), Label: "Timeline"},
}
control := components.BeginSegmentedControl(canvas, theme, bounds, options, selected)
if id, ok := control.Paint().Clicked(); ok {
    selected = id
}`, PageSnippet: `// m.view starts as "List".
views := []string{"List", "Board", "Timeline"}
ui.Choice(c, &m.view, views, nil)`, PageSnippetZh: `// m.view 初始为 "List"。
views := []string{"List", "Board", "Timeline"}
ui.Choice(c, &m.view, views, nil)`},
		{ID: "states", Title: "局部禁用", Description: "展示一个不可用选项与完整控件禁用状态。", Snippet: `// m.view starts as "List"; Timeline is unavailable here.
views := []string{"List", "Board", "Timeline"}
ui.Choice(c, &view, views, nil).DisabledOptions([]bool{false, false, true})
ui.Choice(c, &lockedView, views, nil).Disabled(true)`, PageSnippet: `// m.view starts as "List"; Timeline is unavailable here.
views := []string{"List", "Board", "Timeline"}
ui.Choice(c, &m.view, views, nil).DisabledOptions([]bool{false, false, true})
ui.Choice(c, &m.lockedView, views, nil).Disabled(true)`, PageSnippetZh: `// m.view 初始为 "List"；这里的 Timeline 不可用。
views := []string{"List", "Board", "Timeline"}
ui.Choice(c, &m.view, views, nil).DisabledOptions([]bool{false, false, true})
ui.Choice(c, &m.lockedView, views, nil).Disabled(true)`},
	},
}

var (
	segmentedControlUI = tree.Feature("segmented-control")
	// segmentedControlView's item i is the i-th view option.
	segmentedControlView = tree.Sub(segmentedControlUI, tree.Name("view"))
)

var segmentedControlOptions = [...]components.SegmentOption{
	{ID: segmentedControlView.At(0), Label: "List"},
	{ID: segmentedControlView.At(1), Label: "Board"},
	{ID: segmentedControlView.At(2), Label: "Timeline"},
}

// segmentedControlDisabled marks Timeline unavailable in the states example; the
// page declaration and the typed key helper read the same entries.
var segmentedControlDisabled = [...]bool{false, false, true}

var segmentedControlStateOptions = [...]components.SegmentOption{
	{ID: segmentedControlView.At(0), Label: "List"},
	{ID: segmentedControlView.At(1), Label: "Board"},
	{ID: segmentedControlView.At(2), Label: "Timeline", Disabled: true},
}

func (state *State) keySegmentedControl(event gfx.KeyEvent) gfx.EventResult {
	if listIndex(event.Target, segmentedControlView) < 0 {
		return gfx.EventResult{}
	}
	options := segmentedControlOptions[:]
	if state.exampleID == "states" {
		options = segmentedControlStateOptions[:]
	}
	next, handled := components.SegmentedControlKeyDown(event.Target, options, components.DirectionLTR, event.Key)
	if !handled {
		return gfx.EventResult{}
	}
	nextValue := int32(listIndex(next, segmentedControlView))
	changed := state.values[0] != nextValue
	state.values[0] = nextValue
	return gfx.EventResult{Focus: next, FocusVisible: true, Handled: true, Repaint: changed}
}

// segmentedPageState is the page Choice's model: the chosen view of the live
// control, the view of the locked one, and whether the demo has seeded them from
// State.values.
type segmentedPageState struct {
	view       string
	lockedView string
	init       bool
}

func (state *State) paintSegmentedControl(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	if state.exampleID == "states" {
		state.paintSegmentedControlStates(canvas, theme, size)
		return
	}
	views := [...]string{"List", "Board", "Timeline"}
	state.drawPage(canvas, theme, size, 520, "View", func(c ui.C) {
		page := ui.State[segmentedPageState](c, "view")
		if !page.init {
			page.view, page.init = views[state.values[0]], true
		}
		ui.Choice(c, &page.view, views[:], nil)
		if index := radioPlanIndex(views[:], page.view); index >= 0 {
			state.values[0] = int32(index)
		}
		caption(c, state.Text("方向键通过 typed helper 更新焦点与选择。", "Arrow keys update focus and selection through the typed helper."))
	})
}

// paintSegmentedControlStates declares both controls from the page API: the live
// one disables Timeline per option, and the second is disabled as a whole.
func (state *State) paintSegmentedControlStates(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	views := [...]string{"List", "Board", "Timeline"}
	state.drawPage(canvas, theme, size, 520, "View", func(c ui.C) {
		page := ui.State[segmentedPageState](c, "view")
		if !page.init {
			page.view, page.lockedView, page.init = views[state.values[0]], views[1], true
		}
		ui.Choice(c, &page.view, views[:], nil).DisabledOptions(segmentedControlDisabled[:])
		if index := radioPlanIndex(views[:], page.view); index >= 0 {
			state.values[0] = int32(index)
		}
		ui.Choice(c, &page.lockedView, views[:], nil).Disabled(true)
		caption(c, state.Text("方向键会跳过不可用选项，禁用控件完全惰性。", "Arrow keys skip unavailable options; a disabled control is inert."))
	})
}
