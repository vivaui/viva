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
		{ID: "icons", Title: "图标与等宽标签", Description: "图标和文字作为整体居中；宽度随容器分配，支持键盘方向键。", Snippet: `tabs := tree.Sub(page, tree.Name("tab"))
options := []components.TabOption{
    {ID: tabs.At(0), Label: "Overview", Icon: overviewIcon},
    {ID: tabs.At(1), Label: "Activity", Icon: activityIcon},
    {ID: tabs.At(2), Label: "Settings", Icon: settingsIcon},
}
components.BeginTabs(canvas, theme, bounds, options, options[selected].ID).Paint()`},
		{ID: "default", Title: "分段标签", Description: "三个稳定 tab ID 驱动关联内容。", Snippet: `labels := []string{"Overview", "Activity", "Settings"}
tab := labels[0]
ui.Tabs(c, &tab, labels, nil)
ui.Text(c, tab).FontSize(c.Theme().Metrics.FontSizeM)`},
		{ID: "line", Title: "线形标签", Description: "透明列表与两像素活动指示条。", Snippet: `labels := []string{"Overview", "Activity", "Settings"}
tab := labels[0]
ui.Tabs(c, &tab, labels, nil).Line()`},
		{ID: "vertical", Title: "纵向标签", Description: "标签纵向排列，内容位于右侧。", Snippet: `labels := []string{"Overview", "Activity", "Settings"}
tab := labels[0]
ui.Row(c).Gap(20).Children(func() {
    ui.Tabs(c, &tab, labels, nil).Line().Vertical().Self(ui.Start)
    ui.Text(c, tab).Grow(1).Basis(0)
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

// keyTabs moves the focus between the tabs of the icons example, which the
// components package paints from explicit IDs. The other examples are ui.Tabs:
// the Page's roving focus takes their arrow keys.
func (state *State) keyTabs(event gfx.KeyEvent) gfx.EventResult {
	if state.exampleID != "icons" || listIndex(event.Target, tabsItem) < 0 {
		return gfx.EventResult{}
	}
	labels := state.tabsLabels()
	options := [...]components.TabOption{
		{ID: tabsItem.At(0), Label: labels[0]},
		{ID: tabsItem.At(1), Label: labels[1]},
		{ID: tabsItem.At(2), Label: labels[2]},
	}
	next, handled := components.TabsKeyDown(event.Target, options[:], components.TabsHorizontal, components.DirectionLTR, event.Key)
	if !handled {
		return gfx.EventResult{}
	}
	// Base UI tabs move focus on arrows; Enter or Space activates the focused
	// tab through the runtime's ordinary click activation.
	return gfx.EventResult{Focus: next, FocusVisible: true, Handled: true, Repaint: true}
}

func (state *State) paintTabs(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	labels := state.tabsLabels()
	selected := int(state.values[0])
	if selected < 0 || selected >= len(labels) {
		selected = 0
	}
	if state.exampleID == "icons" {
		var slot ui.Box
		state.drawPage(canvas, theme, size, 540, state.Text("Tabs · 图标标签", "Tabs · Icon labels"), func(c ui.C) {
			slot = ui.Slot(c).FillWidth().Height(36)
			state.tabsPanel(c, theme, selected)
		})
		options := [...]components.TabOption{{ID: tabsItem.At(0), Label: labels[0], Icon: infoIcon}, {ID: tabsItem.At(1), Label: labels[1], Icon: alertIcon}, {ID: tabsItem.At(2), Label: labels[2], Icon: folderIcon}}
		components.BeginTabs(canvas, theme, slot.Rect(), options[:], options[selected].ID).Paint()
		if index, ok := canvas.ClickedIn(tabsItem); ok {
			state.values[0] = int32(index)
		}
		return
	}
	state.drawPage(canvas, theme, size, 540, state.Text("Tabs · 内容切换", "Tabs · Content switching"), func(c ui.C) {
		if state.exampleID == "vertical" {
			ui.Row(c).Gap(20).Children(func() {
				ui.Tabs(c, &state.values[0], tabsChoices[:], state.tabLabel).Line().Vertical().Self(ui.Start)
				state.tabsPanel(c, theme, selected).Grow(1).Basis(0)
			})
			return
		}
		strip := ui.Tabs(c, &state.values[0], tabsChoices[:], state.tabLabel)
		if state.exampleID == "line" {
			strip.Line()
		}
		state.tabsPanel(c, theme, selected)
	})
}

// tabsChoices are the tab indices ui.Tabs chooses among.
var tabsChoices = [...]int32{0, 1, 2}

// tabLabel is the label of tab index in the preview's language. ui.Tabs calls it
// where the tab is declared, so it needs the State's language only.
func (state *State) tabLabel(index int32) string { return state.tabsLabels()[index] }

// tabsPanel declares the content of the selected tab.
func (state *State) tabsPanel(c ui.C, theme gfx.Theme, selected int) ui.Box {
	title, description := state.Text("项目概览", "Project overview"), state.Text("查看近期状态与关键摘要。", "See recent status and key highlights.")
	switch selected {
	case 1:
		title, description = state.Text("最近活动", "Recent activity"), state.Text("跟踪保存、构建和发布记录。", "Track saves, builds, and releases.")
	case 2:
		title, description = state.Text("工作区设置", "Workspace settings"), state.Text("配置显示与本地运行选项。", "Configure display and local runtime options.")
	}
	panel := ui.Column(c).Gap(8).Pad(16).Background(theme.Background).Border(1, theme.Border).Radius(theme.Metrics.RadiusSurface)
	return panel.Children(func() {
		ui.Text(c, title).FontSize(theme.Metrics.FontSizeM).Weight(gfx.WeightSemibold)
		ui.Wrap(c, description).FontSize(theme.Metrics.FontSizeS).Muted()
	})
}
