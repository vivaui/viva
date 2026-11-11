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
}`},
		{ID: "states", Title: "局部禁用", Description: "展示一个不可用选项与完整控件禁用状态。", Snippet: `view := tree.Sub(page, tree.Name("view"))
control := components.BeginSegmentedControl(canvas, theme, bounds, []components.SegmentOption{
    {ID: view.At(0), Label: "List"}, {ID: view.At(1), Label: "Board"},
    {ID: view.At(2), Label: "Timeline", Disabled: true},
}, selected)
if id, ok := control.Paint().Clicked(); ok {
    selected = id
}
locked := tree.Sub(page, tree.Name("locked"))
components.BeginSegmentedControl(canvas, theme, lockedBounds, []components.SegmentOption{
    {ID: locked.At(0), Label: "List"}, {ID: locked.At(1), Label: "Board"},
    {ID: locked.At(2), Label: "Timeline"},
}, locked.At(1)).Disabled().Paint()`},
	},
}

var (
	segmentedControlUI = tree.Feature("segmented-control")
	// segmentedControlView's item i is the i-th view option.
	segmentedControlView = tree.Sub(segmentedControlUI, tree.Name("view"))
	// segmentedControlLocked's item i is the i-th option of the disabled specimen,
	// which declares its own (inert) hits beside the live control's.
	segmentedControlLocked = tree.Sub(segmentedControlUI, tree.Name("locked"))
)

var segmentedControlOptions = [...]components.SegmentOption{
	{ID: segmentedControlView.At(0), Label: "List"},
	{ID: segmentedControlView.At(1), Label: "Board"},
	{ID: segmentedControlView.At(2), Label: "Timeline"},
}

var segmentedControlLockedOptions = [...]components.SegmentOption{
	{ID: segmentedControlLocked.At(0), Label: "List"},
	{ID: segmentedControlLocked.At(1), Label: "Board"},
	{ID: segmentedControlLocked.At(2), Label: "Timeline"},
}

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

func (state *State) paintSegmentedControl(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	options := segmentedControlOptions[:]
	if state.exampleID == "states" {
		options = segmentedControlStateOptions[:]
	}
	var live, disabled ui.Box
	state.drawPage(canvas, theme, size, 520, "View", func(c ui.C) {
		live = ui.Slot(c).FillWidth().MaxWidth(420).Height(theme.Metrics.ControlHeight)
		if state.exampleID == "states" {
			// The component paints a whole-control disabled specimen in a slot:
			// it declares its own (inert) hits beside the live control's.
			disabled = ui.Slot(c).FillWidth().MaxWidth(420).Height(theme.Metrics.ControlHeight)
		}
		caption(c, state.Text("方向键通过 typed helper 更新焦点与选择。", "Arrow keys update focus and selection through the typed helper."))
	})
	if id, ok := components.BeginSegmentedControl(canvas, theme, live.Rect(), options, segmentedControlView.At(int(state.values[0]))).Paint().Clicked(); ok {
		state.values[0] = int32(listIndex(id, segmentedControlView))
	}
	if state.exampleID == "states" {
		components.BeginSegmentedControl(canvas, theme, disabled.Rect(), segmentedControlLockedOptions[:], segmentedControlLocked.At(1)).Disabled().Paint()
	}
}
