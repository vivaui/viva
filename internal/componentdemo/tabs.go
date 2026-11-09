package componentdemo

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var tabsComponent = Component{
	ID: "tabs", Title: "Tabs", Category: "Navigation", Summary: "调用方持有选中项的标签页导航。",
	paint: (*State).paintTabs, key: (*State).keyTabs,
	Examples: []Example{
		{ID: "icons", Title: "图标与等宽标签", Description: "图标和文字作为整体居中；宽度随容器分配，支持键盘方向键。", Snippet: `options := []components.TabOption{
    {ID: tabs.At(0), Label: "Overview", Icon: overviewIcon},
    {ID: tabs.At(1), Label: "Activity", Icon: activityIcon},
    {ID: tabs.At(2), Label: "Settings", Icon: settingsIcon},
}
components.BeginTabs(canvas, theme, bounds, options, options[selected].ID).Paint()`},
		{ID: "default", Title: "分段标签", Description: "三个稳定 tab ID 驱动关联内容。", Snippet: `tabs := tree.Sub(page, tree.Name("tab"))
labels := []string{"Overview", "Activity", "Settings"}
strip := f.Tabs(tabs, labels, selected)
if index, ok := strip.TabClicked(); ok {
    selected = index
}
f.Text(labels[selected]).FontSize(theme.Metrics.FontSizeM)`},
		{ID: "line", Title: "线形标签", Description: "透明列表与两像素活动指示条。", Snippet: `tabs := tree.Sub(page, tree.Name("tab"))
labels := []string{"Overview", "Activity", "Settings"}
strip := f.Tabs(tabs, labels, selected).Line()
if index, ok := strip.TabClicked(); ok {
    selected = index
}`},
		{ID: "vertical", Title: "纵向标签", Description: "标签纵向排列，内容位于右侧。", Snippet: `tabs := tree.Sub(page, tree.Name("tab"))
labels := []string{"Overview", "Activity", "Settings"}
f.Row().Gap(20).Children(func() {
    strip := f.Tabs(tabs, labels, selected).Line().Vertical().Self(ui.Start)
    if index, ok := strip.TabClicked(); ok {
        selected = index
    }
    f.Text(labels[selected]).Grow(1).Basis(0)
})`},
	},
}

var (
	tabsUI = tree.Feature("tabs")
	// tabsItem's item i is the i-th tab.
	tabsItem = tree.Sub(tabsUI, tree.Name("tab"))
)

// tabsLabelSets holds each language's tab labels. ui.Tabs keeps the label
// slice until the frame is drawn, so translating into a fresh array every paint
// would allocate it; picking a preallocated set keeps the paint allocation-free.
var tabsLabelSets = [2][3]string{
	{"概览", "活动", "设置"},
	{"Overview", "Activity", "Settings"},
}

// tabsLabels returns the strip labels for the preview's language, before ui
// lays the strip out.
func (state *State) tabsLabels() []string {
	language := 0
	if state.english {
		language = 1
	}
	return tabsLabelSets[language][:]
}

func (state *State) keyTabs(event gfx.KeyEvent) gfx.EventResult {
	if listIndex(event.Target, tabsItem) < 0 {
		return gfx.EventResult{}
	}
	orientation := components.TabsHorizontal
	if state.exampleID == "vertical" {
		orientation = components.TabsVertical
	}
	labels := state.tabsLabels()
	options := [...]components.TabOption{
		{ID: tabsItem.At(0), Label: labels[0]},
		{ID: tabsItem.At(1), Label: labels[1]},
		{ID: tabsItem.At(2), Label: labels[2]},
	}
	next, handled := components.TabsKeyDown(event.Target, options[:], orientation, components.DirectionLTR, event.Key)
	if !handled {
		return gfx.EventResult{}
	}
	// Base UI tabs move focus on arrows; Enter or Space activates the focused
	// tab through the runtime's ordinary click activation.
	return gfx.EventResult{Focus: next, FocusVisible: true, Handled: true, Repaint: true}
}

func (state *State) paintTabs(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	selected := int(state.values[0])
	labels := state.tabsLabels()
	if state.exampleID == "icons" {
		var slot ui.Box
		state.page(canvas, theme, size, 540, state.Text("Tabs · 图标标签", "Tabs · Icon labels"), func(f *ui.Frame) {
			slot = f.Slot().FillWidth().Height(36)
			state.tabsPanel(f, theme, selected)
		})
		options := [...]components.TabOption{{ID: tabsItem.At(0), Label: labels[0], Icon: infoIcon}, {ID: tabsItem.At(1), Label: labels[1], Icon: alertIcon}, {ID: tabsItem.At(2), Label: labels[2], Icon: folderIcon}}
		components.BeginTabs(canvas, theme, slot.Rect(), options[:], options[selected].ID).Paint()
		if index, ok := canvas.ClickedIn(tabsItem); ok {
			state.values[0] = int32(index)
		}
		return
	}
	state.page(canvas, theme, size, 540, state.Text("Tabs · 内容切换", "Tabs · Content switching"), func(f *ui.Frame) {
		if state.exampleID == "vertical" {
			f.Row().Gap(20).Children(func() {
				strip := f.Tabs(tabsItem, labels, selected).Line().Vertical().Self(ui.Start)
				if index, ok := strip.TabClicked(); ok {
					state.values[0] = int32(index)
				}
				state.tabsPanel(f, theme, selected).Grow(1).Basis(0)
			})
			return
		}
		strip := f.Tabs(tabsItem, labels, selected)
		if state.exampleID == "line" {
			strip.Line()
		}
		if index, ok := strip.TabClicked(); ok {
			state.values[0] = int32(index)
		}
		state.tabsPanel(f, theme, selected)
	})
}

// tabsPanel declares the content of the selected tab.
func (state *State) tabsPanel(f *ui.Frame, theme gfx.Theme, selected int) ui.Box {
	title, description := state.Text("项目概览", "Project overview"), state.Text("查看近期状态与关键摘要。", "See recent status and key highlights.")
	switch selected {
	case 1:
		title, description = state.Text("最近活动", "Recent activity"), state.Text("跟踪保存、构建和发布记录。", "Track saves, builds, and releases.")
	case 2:
		title, description = state.Text("工作区设置", "Workspace settings"), state.Text("配置显示与本地运行选项。", "Configure display and local runtime options.")
	}
	panel := f.Column().Gap(8).Pad(16).Background(theme.Background).Border(1, theme.Border).Radius(theme.Metrics.RadiusSurface)
	return panel.Children(func() {
		f.Text(title).FontSize(theme.Metrics.FontSizeM).Weight(gfx.WeightSemibold)
		f.Wrap(description).FontSize(theme.Metrics.FontSizeS).Muted()
	})
}
