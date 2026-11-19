package componentdemo

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// sonnerDemo is the page-owned toaster of one example, made once so a warm
// paint reuses the stack and its identities.
type sonnerDemo struct {
	toaster ui.Toaster
	seeded  bool
}

var sonnerComponent = Component{
	ID: "sonner", Title: "Sonner", Category: "Feedback", Summary: "调用方管理生命周期的通知堆栈。",
	paint: (*State).paintSonner, initialize: (*State).initializeSonner,
	Examples: []Example{
		{ID: "default", Title: "单条通知", Description: "真实 Toast 配方，带稳定操作与关闭 ID。", Snippet: `toast := tree.ID(page, "toast")
toasts := [...]components.SonnerToast{{
    Snapshot: components.ToastSnapshot{
        Title: "Project saved", Description: "All local changes are written.", Type: components.ToastSuccess,
    },
    Scope: toast, DismissID: tree.ID(page, "dismiss"),
    ActionID: tree.ID(page, "action"), ActionLabel: "View",
}}
components.PaintSonner(canvas, theme, viewport, toasts[:], components.SonnerOptions{Layer: 40})`, PageSnippet: `if !m.notified {
    m.toaster.ShowAction("Project saved", "All local changes are written.", ui.ToastSuccess, "View")
    m.notified = true
}
if ui.Button(c, "Save project").Clicked() {
    m.toaster.ShowAction("Project saved", "All local changes are written.", ui.ToastSuccess, "View")
}
ui.Toasts(c, &m.toaster)`, PageSnippetZh: `if !m.notified {
    m.toaster.ShowAction("项目已保存", "所有本地更改均已写入。", ui.ToastSuccess, "查看")
    m.notified = true
}
if ui.Button(c, "保存项目").Clicked() {
    m.toaster.ShowAction("项目已保存", "所有本地更改均已写入。", ui.ToastSuccess, "查看")
}
ui.Toasts(c, &m.toaster)`},
		{ID: "stack", Title: "折叠堆栈", Description: "最新通知在前，使用小间距叠放。", Snippet: `toast := tree.Sub(page, tree.Name("toast"))
toasts := [...]components.SonnerToast{
    {Snapshot: components.ToastSnapshot{Title: "Project saved", Type: components.ToastSuccess},
        Scope: toast.At(0)},
    {Snapshot: components.ToastSnapshot{Title: "Update available", Type: components.ToastInfo},
        Scope: toast.At(1)},
    {Snapshot: components.ToastSnapshot{Title: "Connection lost", Type: components.ToastError},
        Scope: toast.At(2)},
}
components.PaintSonner(canvas, theme, viewport, toasts[:], components.SonnerOptions{Layer: 40})`, PageSnippet: `if !m.notified {
    m.toaster.Show("Project saved", "All local changes are written.", ui.ToastSuccess)
    m.toaster.Show("Update available", "Takes effect after the app restarts.", ui.ToastInfo)
    m.toaster.Show("Connection lost", "Waiting for the network to recover.", ui.ToastError)
    m.notified = true
}
ui.Toasts(c, &m.toaster)`, PageSnippetZh: `if !m.notified {
    m.toaster.Show("项目已保存", "所有本地更改均已写入。", ui.ToastSuccess)
    m.toaster.Show("有可用更新", "重启应用后生效。", ui.ToastInfo)
    m.toaster.Show("连接已中断", "正在等待网络恢复。", ui.ToastError)
    m.notified = true
}
ui.Toasts(c, &m.toaster)`},
		{ID: "expanded", Title: "展开堆栈", Description: "显式展开多条不同语义通知。", Snippet: `toast := tree.Sub(page, tree.Name("toast"))
action := tree.Sub(page, tree.Name("action"))
toasts := [...]components.SonnerToast{
    {Snapshot: components.ToastSnapshot{Title: "Project saved", Type: components.ToastSuccess},
        Scope: toast.At(0), ActionID: action.At(0), ActionLabel: "View"},
    {Snapshot: components.ToastSnapshot{Title: "Update available", Type: components.ToastInfo},
        Scope: toast.At(1), ActionID: action.At(1), ActionLabel: "Later"},
    {Snapshot: components.ToastSnapshot{Title: "Connection lost", Type: components.ToastError},
        Scope: toast.At(2)},
}
components.PaintSonner(canvas, theme, viewport, toasts[:], components.SonnerOptions{
    Layer: 40, Expanded: true,
})`, PageSnippet: `// The stack expands while the pointer rests on it or focus is inside it.
// A static page cannot hold that hover, so the preview stays on the canvas
// painter to show the expanded geometry exactly.
if !m.notified {
    m.toaster.ShowAction("Project saved", "All local changes are written.", ui.ToastSuccess, "View")
    m.toaster.ShowAction("Update available", "Takes effect after the app restarts.", ui.ToastInfo, "Later")
    m.toaster.Show("Connection lost", "Waiting for the network to recover.", ui.ToastError)
    m.notified = true
}
ui.Toasts(c, &m.toaster)`, PageSnippetZh: `// 指针停留或焦点在通知堆栈内时，堆栈会展开。
// 静态页面无法保持悬停，因此该示例保留 canvas 绘制以如实展示展开几何。
if !m.notified {
    m.toaster.ShowAction("项目已保存", "所有本地更改均已写入。", ui.ToastSuccess, "查看")
    m.toaster.ShowAction("有可用更新", "重启应用后生效。", ui.ToastInfo, "稍后")
    m.toaster.Show("连接已中断", "正在等待网络恢复。", ui.ToastError)
    m.notified = true
}
ui.Toasts(c, &m.toaster)`},
	},
}

var (
	sonnerUI = tree.Feature("sonner")
	// The lists' item i belong to the i-th toast of the stack.
	sonnerToast   = tree.Sub(sonnerUI, tree.Name("toast"))
	sonnerDismiss = tree.Sub(sonnerUI, tree.Name("dismiss"))
	sonnerAction  = tree.Sub(sonnerUI, tree.Name("action"))
)

func (state *State) initializeSonner() { state.flags = 0b111 }

func (state *State) paintSonner(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	if state.exampleID == "expanded" {
		state.paintSonnerExpanded(canvas, theme, size)
		return
	}
	state.drawPage(canvas, theme, size, 520, state.Text("Sonner · 通知堆栈", "Sonner · Notification stack"), func(c ui.C) {
		demo := ui.State[sonnerDemo](c, "sonner-demo")
		ui.Text(c, state.Text("生命周期、过期与动画由调用方持有。", "Lifecycle, expiry, and animation belong to the caller.")).FontSize(theme.Metrics.FontSizeS).Muted()
		if !demo.seeded {
			demo.seeded = true
			if state.exampleID == "default" {
				demo.toaster.ShowAction(state.Text("项目已保存", "Project saved"), state.Text("所有本地更改均已写入。", "All local changes are written."), ui.ToastSuccess, state.Text("查看", "View"))
			} else {
				demo.toaster.Show(state.Text("项目已保存", "Project saved"), state.Text("所有本地更改均已写入。", "All local changes are written."), ui.ToastSuccess)
				demo.toaster.Show(state.Text("有可用更新", "Update available"), state.Text("重启应用后生效。", "Takes effect after the app restarts."), ui.ToastInfo)
				demo.toaster.Show(state.Text("连接已中断", "Connection lost"), state.Text("正在等待网络恢复。", "Waiting for the network to recover."), ui.ToastError)
			}
		}
		if state.exampleID == "default" {
			label := state.Text("保存项目", "Save project")
			if ui.Button(c, label).Self(ui.Start).Hit(sonnerAction.At(0), label).Clicked() {
				demo.toaster.ShowAction(state.Text("项目已保存", "Project saved"), state.Text("所有本地更改均已写入。", "All local changes are written."), ui.ToastSuccess, state.Text("查看", "View"))
			}
		}
		if demo.toaster.Len() == 0 {
			ui.Text(c, state.Text("通知已全部关闭。", "All notifications are dismissed.")).FontSize(theme.Metrics.FontSizeM).Weight(gfx.WeightMedium).Muted()
		}
		ui.Toasts(c, &demo.toaster)
		if demo.toaster.Len() == 0 {
			state.flags = 0
		} else {
			state.flags = 1<<demo.toaster.Len() - 1
		}
	})
}

// paintSonnerExpanded paints the forced expanded geometry, which the page
// stack only enters while the pointer rests on it or focus is inside it: a
// static preview cannot hold that hover.
func (state *State) paintSonnerExpanded(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	toasts := [...]components.SonnerToast{
		{
			Snapshot: components.ToastSnapshot{
				Title:       state.Text("项目已保存", "Project saved"),
				Description: state.Text("所有本地更改均已写入。", "All local changes are written."),
				Type:        components.ToastSuccess,
			},
			Scope: sonnerToast.At(0), DismissID: sonnerDismiss.At(0), ActionID: sonnerAction.At(0),
			ActionLabel: state.Text("查看", "View"),
		},
		{
			Snapshot: components.ToastSnapshot{
				Title:       state.Text("有可用更新", "Update available"),
				Description: state.Text("重启应用后生效。", "Takes effect after the app restarts."),
				Type:        components.ToastInfo,
			},
			Scope: sonnerToast.At(1), DismissID: sonnerDismiss.At(1), ActionID: sonnerAction.At(1),
			ActionLabel: state.Text("稍后", "Later"),
		},
		{
			Snapshot: components.ToastSnapshot{
				Title:       state.Text("连接已中断", "Connection lost"),
				Description: state.Text("正在等待网络恢复。", "Waiting for the network to recover."),
				Type:        components.ToastError,
			},
			Scope: sonnerToast.At(2), DismissID: sonnerDismiss.At(2),
		},
	}
	visible := toasts[:0]
	for index := range toasts {
		if state.flags&(1<<index) != 0 {
			visible = append(visible, toasts[index])
		}
	}
	state.drawPage(canvas, theme, size, 520, state.Text("Sonner · 通知堆栈", "Sonner · Notification stack"), func(c ui.C) {
		ui.Text(c, state.Text("生命周期、过期与动画由调用方持有。", "Lifecycle, expiry, and animation belong to the caller.")).FontSize(theme.Metrics.FontSizeS).Muted()
		if len(visible) == 0 {
			ui.Text(c, state.Text("通知已全部关闭。", "All notifications are dismissed.")).FontSize(theme.Metrics.FontSizeM).Weight(gfx.WeightMedium).Muted()
		}
	})
	if len(visible) == 0 {
		return
	}
	components.PaintSonner(canvas, theme, gfx.Rect{Width: size.Width, Height: size.Height}, visible, components.SonnerOptions{
		Layer: 40, Expanded: true,
	})
	if index, ok := canvas.ClickedIn(sonnerDismiss); ok {
		state.flags &^= 1 << index
	}
	if _, ok := canvas.ClickedIn(sonnerAction); ok {
		state.flags ^= 1 << 8
	}
}
