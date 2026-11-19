package componentdemo

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var radioGroupComponent = Component{
	ID: "radio-group", Title: "Radio Group", Category: "Forms", Summary: "调用方持有单选值的纵向与横向选项组。",
	paint: (*State).paintRadioGroup, key: (*State).keyRadioGroup, initialize: (*State).initializeRadioGroup,
	Examples: []Example{
		{ID: "vertical", Title: "纵向选择", Description: "点击方案切换单一选中值。", Snippet: `plan := tree.Sub(page, tree.Name("plan"))
options := []components.RadioOption{
    {ID: plan.At(0), Label: "Personal"},
    {ID: plan.At(1), Label: "Team"},
    {ID: plan.At(2), Label: "Enterprise"},
}
group := components.BeginRadioGroup(canvas, theme, bounds, options, selected)
if id, ok := group.Paint().Clicked(); ok {
    selected = id
}`, PageSnippet: `plan := "Team"
ui.RadioGroup(c).Children(func() {
    ui.Radio(c, &plan, "Personal", "Personal")
    ui.Radio(c, &plan, "Team", "Team")
    ui.Radio(c, &plan, "Enterprise", "Enterprise")
})`, PageSnippetZh: `plan := "Team"
ui.RadioGroup(c).Children(func() {
    ui.Radio(c, &plan, "Personal", "Personal")
    ui.Radio(c, &plan, "Team", "Team")
    ui.Radio(c, &plan, "Enterprise", "Enterprise")
})`},
		{ID: "horizontal", Title: "横向选择", Description: "等宽横向选项适合紧凑筛选。", Snippet: `plan := tree.Sub(page, tree.Name("plan"))
options := []components.RadioOption{
    {ID: plan.At(0), Label: "Personal"},
    {ID: plan.At(1), Label: "Team"},
    {ID: plan.At(2), Label: "Enterprise"},
}
group := components.BeginRadioGroup(canvas, theme, bounds, options, selected)
if id, ok := group.Horizontal().Paint().Clicked(); ok {
    selected = id
}`, PageSnippet: `plan := "Team"
ui.Row(c).Gap(12).FocusGroup(ui.FocusHorizontal).Children(func() {
    ui.Radio(c, &plan, "Personal", "Personal")
    ui.Radio(c, &plan, "Team", "Team")
    ui.Radio(c, &plan, "Enterprise", "Enterprise")
})`, PageSnippetZh: `plan := "Team"
ui.Row(c).Gap(12).FocusGroup(ui.FocusHorizontal).Children(func() {
    ui.Radio(c, &plan, "Personal", "Personal")
    ui.Radio(c, &plan, "Team", "Team")
    ui.Radio(c, &plan, "Enterprise", "Enterprise")
})`},
		{ID: "states", Title: "禁用与无效", Description: "展示选中、无效和局部禁用语义。", Snippet: `plan := tree.Sub(page, tree.Name("plan"))
options := []components.RadioOption{
    {ID: plan.At(0), Label: "Personal"},
    {ID: plan.At(1), Label: "Team", Invalid: true},
    {ID: plan.At(2), Label: "Enterprise", Disabled: true},
}
group := components.BeginRadioGroup(canvas, theme, bounds, options, selected)
if id, ok := group.Paint().Clicked(); ok {
    selected = id
}`, PageSnippet: `plan := "Team"
ui.RadioGroup(c).Children(func() {
    ui.Radio(c, &plan, "Personal", "Personal")
    ui.Radio(c, &plan, "Team", "Team")
    ui.Radio(c, &plan, "Enterprise", "Enterprise").Disabled(true)
})`, PageSnippetZh: `plan := "Team"
ui.RadioGroup(c).Children(func() {
    ui.Radio(c, &plan, "Personal", "Personal")
    ui.Radio(c, &plan, "Team", "Team")
    ui.Radio(c, &plan, "Enterprise", "Enterprise").Disabled(true)
})`},
	},
}

var (
	radioGroupUI = tree.Feature("radio-group")
	// radioGroupItem's item i is the i-th plan.
	radioGroupItem = tree.Sub(radioGroupUI, tree.Name("plan"))
)

var radioGroupOptions = [...]components.RadioOption{
	{ID: radioGroupItem.At(0), Label: "Personal"},
	{ID: radioGroupItem.At(1), Label: "Team"},
	{ID: radioGroupItem.At(2), Label: "Enterprise"},
}

func (state *State) initializeRadioGroup() {
	state.values[0] = 1
}

// radioGroupOptionsFor returns the options with the example's invalid and disabled
// marks.
func (state *State) radioGroupOptionsFor() [len(radioGroupOptions)]components.RadioOption {
	options := radioGroupOptions
	if state.exampleID == "states" {
		options[1].Invalid = true
		options[2].Disabled = true
	}
	return options
}

func (state *State) keyRadioGroup(event gfx.KeyEvent) gfx.EventResult {
	if listIndex(event.Target, radioGroupItem) < 0 {
		return gfx.EventResult{}
	}
	options := state.radioGroupOptionsFor()
	next, handled := components.RadioGroupKeyDown(event.Target, options[:], components.DirectionLTR, event.Key)
	if !handled {
		return gfx.EventResult{}
	}
	nextValue := int32(listIndex(next, radioGroupItem))
	changed := state.values[0] != nextValue
	state.values[0] = nextValue
	return gfx.EventResult{Focus: next, FocusVisible: true, Handled: true, Repaint: changed}
}

// radioPageState is the page RadioGroup's model: the chosen plan, and whether
// the demo has seeded it from State.values.
type radioPageState struct {
	plan string
	init bool
}

func (state *State) paintRadioGroup(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	if state.exampleID == "states" {
		state.paintRadioGroupStates(canvas, theme, size)
		return
	}
	horizontal := state.exampleID == "horizontal" && min(size.Width-48, float32(520)) >= 360
	plans := [...]string{"Personal", "Team", "Enterprise"}
	state.drawPage(canvas, theme, size, 520, "Workspace plan", func(c ui.C) {
		page := ui.State[radioPageState](c, "plan")
		if !page.init {
			page.plan, page.init = plans[state.values[0]], true
		}
		radios := func() {
			for index, plan := range plans {
				ui.RadioID(c, radioGroupItem.At(index), &page.plan, plan, plan)
			}
		}
		if horizontal {
			ui.Row(c).Gap(12).FocusGroup(ui.FocusHorizontal).Children(radios)
		} else {
			ui.RadioGroup(c).Children(radios)
		}
		if index := radioPlanIndex(plans[:], page.plan); index >= 0 {
			state.values[0] = int32(index)
		}
		caption(c, state.Text("方向键移动焦点并更新调用方选中值。", "Arrow keys move focus and update the caller's selection."))
	})
}

func radioPlanIndex(plans []string, plan string) int {
	for index, option := range plans {
		if option == plan {
			return index
		}
	}
	return -1
}

// paintRadioGroupStates keeps the invalid example on the canvas API: the page
// Radio has no per-option invalid state.
func (state *State) paintRadioGroupStates(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	var slot ui.Box
	state.drawPage(canvas, theme, size, 520, "Workspace plan", func(c ui.C) {
		slot = ui.Slot(c).Height(theme.Metrics.ControlHeight*3 + 24)
		caption(c, state.Text("方向键移动焦点并更新调用方选中值。", "Arrow keys move focus and update the caller's selection."))
	})
	options := state.radioGroupOptionsFor()
	group := components.BeginRadioGroup(canvas, theme, slot.Rect(), options[:], radioGroupItem.At(int(state.values[0])))
	group.Paint()
	if index, ok := canvas.ClickedIn(radioGroupItem); ok {
		state.values[0] = int32(index)
	}
}
