package componentdemo

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// toastDemo is the page-owned toaster of one example. The toasts themselves are
// kept in it, so a warm paint neither rebuilds the stack nor relocates its
// identities.
type toastDemo struct {
	toaster   ui.Toaster
	seeded    bool
	shownType int32
}

var toastComponent = Component{
	ID: "toast", Title: "Toast", Category: "Feedback", Summary: "无隐藏计时器的单条通知快照。",
	paint: (*State).paintToast, initialize: (*State).initializeToast,
	Examples: []Example{
		{ID: "default", Title: "保存成功", Description: "成功通知带稳定操作与关闭按钮。", Snippet: `snapshot := components.ToastSnapshot{
    Title: "Project saved", Description: "Changes are safely written locally.", Type: components.ToastSuccess,
}
if open {
    components.PaintToast(canvas, theme, viewport, snapshot, components.ToastOptions{
        Layer: 52, Scope: tree.ID(page, "toast"),
        DismissID: tree.ID(page, "dismiss"), ActionID: tree.ID(page, "action"),
        ActionLabel: "View",
    })
    if canvas.Clicked(tree.ID(page, "dismiss")) { open = false }
}`, PageSnippet: `if !m.toastShown {
    m.toaster.ShowAction("Project saved", "Changes are safely written locally.", ui.ToastSuccess, "View")
    m.toastShown = true
}
if ui.Button(c, "Save project").Clicked() {
    m.toaster.ShowAction("Project saved", "Changes are safely written locally.", ui.ToastSuccess, "View")
}
ui.Toasts(c, &m.toaster)`, PageSnippetZh: `if !m.toastShown {
    m.toaster.ShowAction("项目已保存", "更改已安全写入本地。", ui.ToastSuccess, "查看")
    m.toastShown = true
}
if ui.Button(c, "保存项目").Clicked() {
    m.toaster.ShowAction("项目已保存", "更改已安全写入本地。", ui.ToastSuccess, "查看")
}
ui.Toasts(c, &m.toaster)`},
		{ID: "types", Title: "语义类型", Description: "通过点击操作在信息、警告和错误样式间切换。", Snippet: `types := [...]components.ToastType{
    components.ToastInfo, components.ToastWarning, components.ToastError,
}
titles := [...]string{"Update available", "Low disk space", "Build failed"}
snapshot := components.ToastSnapshot{
    Title: titles[selected], Description: "Activate the action button to see the next semantic type.",
    Type: types[selected],
}
components.PaintToast(canvas, theme, viewport, snapshot, components.ToastOptions{
    Layer: 52, Scope: tree.ID(page, "toast"), DismissID: tree.ID(page, "dismiss"),
    ActionID: tree.ID(page, "action"), ActionLabel: "Switch",
})
if canvas.Clicked(tree.ID(page, "action")) { selected = (selected + 1) % len(types) }`, PageSnippet: `titles := []string{"Update available", "Low disk space", "Build failed"}
kinds := []ui.ToastKind{ui.ToastInfo, ui.ToastWarning, ui.ToastError}
if m.toastShown != m.toastType {
    m.toaster.ShowAction(titles[m.toastType], "Activate the action button to see the next semantic type.", kinds[m.toastType], "Switch")
    m.toastShown = m.toastType
}
ui.Toasts(c, &m.toaster)
if _, acted := m.toaster.Acted(); acted {
    m.toastType = (m.toastType + 1) % len(titles)
}`, PageSnippetZh: `titles := []string{"有可用更新", "磁盘空间不足", "构建失败"}
kinds := []ui.ToastKind{ui.ToastInfo, ui.ToastWarning, ui.ToastError}
if m.toastShown != m.toastType {
    m.toaster.ShowAction(titles[m.toastType], "激活操作按钮查看下一个语义类型。", kinds[m.toastType], "切换")
    m.toastShown = m.toastType
}
ui.Toasts(c, &m.toaster)
if _, acted := m.toaster.Acted(); acted {
    m.toastType = (m.toastType + 1) % len(titles)
}`},
		{ID: "swipe", Title: "手势快照", Description: "仅展示调用方提供的横向位移；组件不解释手势。", Snippet: `snapshot := components.ToastSnapshot{
    Title: "Download almost complete", Description: "The horizontal offset comes from the caller's snapshot.",
    Type: components.ToastInfo, SwipeX: -8,
}
components.PaintToast(canvas, theme, viewport, snapshot, components.ToastOptions{
    Layer: 52, Scope: tree.ID(page, "toast"), DismissID: tree.ID(page, "dismiss"),
    ActionID: tree.ID(page, "action"), ActionLabel: "Continue",
})`, PageSnippet: `// The page stack has no caller-controlled horizontal offset yet, so this
// example stays on the canvas painter to show the swipe snapshot exactly.
m.toaster.Show("Download almost complete", "The horizontal offset comes from the caller's snapshot.", ui.ToastInfo)
ui.Toasts(c, &m.toaster)`, PageSnippetZh: `// 页面通知堆栈尚不支持调用方提供的横向位移，因此该示例保留 canvas 绘制以如实展示手势快照。
m.toaster.Show("下载即将完成", "横向位移来自调用方提供的快照。", ui.ToastInfo)
ui.Toasts(c, &m.toaster)`},
	},
}

var (
	toastUI      = tree.Feature("toast")
	toastDismiss = tree.ID(toastUI, "dismiss")
	toastAction  = tree.ID(toastUI, "action")
	toastScope   = tree.ID(toastUI, "toast")
)

func (state *State) initializeToast() { state.flags = 1 }

func (state *State) paintToast(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	if state.exampleID == "swipe" {
		state.paintToastSwipe(canvas, theme, size)
		return
	}
	state.drawPage(canvas, theme, size, 520, state.Text("Toast · 调用方生命周期", "Toast · Caller lifecycle"), func(c ui.C) {
		demo := ui.State[toastDemo](c, "toast-demo")
		if !demo.seeded {
			demo.seeded, demo.shownType = true, -1
		}
		ui.Text(c, state.Text("关闭后重新进入示例可恢复初始快照。", "Re-entering the example after dismissal restores the initial snapshot.")).FontSize(theme.Metrics.FontSizeS).Muted()
		if demo.toaster.Len() == 0 {
			ui.Text(c, state.Text("通知已关闭。", "Notification dismissed.")).FontSize(theme.Metrics.FontSizeM).Weight(gfx.WeightMedium).Muted()
		}
		savedTitle, savedDescription := state.Text("项目已保存", "Project saved"), state.Text("更改已安全写入本地。", "Changes are safely written locally.")
		switch state.exampleID {
		case "types":
			titles := [3]string{
				state.Text("有可用更新", "Update available"),
				state.Text("磁盘空间不足", "Low disk space"),
				state.Text("构建失败", "Build failed"),
			}
			kinds := [3]ui.ToastKind{ui.ToastInfo, ui.ToastWarning, ui.ToastError}
			index := int(state.values[0] % 3)
			if demo.shownType != int32(index) {
				demo.toaster.ShowAction(titles[index], state.Text("激活操作按钮查看下一个语义类型。", "Activate the action button to see the next semantic type."), kinds[index], state.Text("切换", "Switch"))
				demo.shownType = int32(index)
			}
		default:
			if demo.shownType < 0 {
				demo.toaster.ShowAction(savedTitle, savedDescription, ui.ToastSuccess, state.Text("查看", "View"))
				demo.shownType = 0
			}
			if ui.Button(c, state.Text("保存项目", "Save project")).Self(ui.Start).Hit(toastAction, state.Text("保存项目", "Save project")).Clicked() {
				demo.toaster.ShowAction(savedTitle, savedDescription, ui.ToastSuccess, state.Text("查看", "View"))
			}
		}
		ui.Toasts(c, &demo.toaster)
		if _, acted := demo.toaster.Acted(); acted && state.exampleID == "types" {
			state.values[0] = (state.values[0] + 1) % 3
		}
		if demo.toaster.Len() > 0 {
			state.flags |= 1
		} else {
			state.flags &^= 1
		}
	})
}

// paintToastSwipe draws the caller's horizontal offset, which the page stack
// cannot carry yet: the toast component takes the snapshot as-is.
func (state *State) paintToastSwipe(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	open := state.flags&1 != 0
	state.drawPage(canvas, theme, size, 520, state.Text("Toast · 调用方生命周期", "Toast · Caller lifecycle"), func(c ui.C) {
		ui.Text(c, state.Text("关闭后重新进入示例可恢复初始快照。", "Re-entering the example after dismissal restores the initial snapshot.")).FontSize(theme.Metrics.FontSizeS).Muted()
		if !open {
			ui.Text(c, state.Text("通知已关闭。", "Notification dismissed.")).FontSize(theme.Metrics.FontSizeM).Weight(gfx.WeightMedium).Muted()
		}
	})
	if !open {
		return
	}
	snapshot := components.ToastSnapshot{
		Title:       state.Text("下载即将完成", "Download almost complete"),
		Description: state.Text("横向位移来自调用方快照。", "The horizontal offset comes from the caller's snapshot."),
		Type:        components.ToastInfo,
		SwipeX:      -8,
	}
	components.PaintToast(canvas, theme, gfx.Rect{Width: size.Width, Height: size.Height}, snapshot, components.ToastOptions{
		Layer: 52, Scope: toastScope, DismissID: toastDismiss, ActionID: toastAction, ActionLabel: state.Text("继续", "Continue"),
	})
	if canvas.Clicked(toastDismiss) {
		state.flags &^= 1
	}
}
