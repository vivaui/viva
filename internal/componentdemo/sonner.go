package componentdemo

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

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
components.PaintSonner(canvas, theme, viewport, toasts[:], components.SonnerOptions{Layer: 40})`},
		{ID: "stack", Title: "折叠堆栈", Description: "最新通知在前，使用小间距叠放。", Snippet: `toast := tree.Sub(page, tree.Name("toast"))
toasts := [...]components.SonnerToast{
    {Snapshot: components.ToastSnapshot{Title: "Project saved", Type: components.ToastSuccess},
        Scope: toast.At(0)},
    {Snapshot: components.ToastSnapshot{Title: "Update available", Type: components.ToastInfo},
        Scope: toast.At(1)},
    {Snapshot: components.ToastSnapshot{Title: "Connection lost", Type: components.ToastError},
        Scope: toast.At(2)},
}
components.PaintSonner(canvas, theme, viewport, toasts[:], components.SonnerOptions{Layer: 40})`},
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
})`},
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
	limit := len(toasts)
	if state.exampleID == "default" {
		limit = 1
	}
	for index := 0; index < limit; index++ {
		if state.flags&(1<<index) != 0 {
			visible = append(visible, toasts[index])
		}
	}
	state.page(canvas, theme, size, 520, state.Text("Sonner · 通知堆栈", "Sonner · Notification stack"), func(f *ui.Frame) {
		f.Text(state.Text("生命周期、过期与动画由调用方持有。", "Lifecycle, expiry, and animation belong to the caller.")).FontSize(theme.Metrics.FontSizeS).Muted()
		if len(visible) == 0 {
			f.Text(state.Text("通知已全部关闭。", "All notifications are dismissed.")).FontSize(theme.Metrics.FontSizeM).Weight(gfx.WeightMedium).Muted()
		}
	})
	if len(visible) == 0 {
		return
	}
	components.PaintSonner(canvas, theme, gfx.Rect{Width: size.Width, Height: size.Height}, visible, components.SonnerOptions{
		Layer: 40, Expanded: state.exampleID == "expanded",
	})

	if index, ok := canvas.ClickedIn(sonnerDismiss); ok {
		state.flags &^= 1 << index
	}
	if _, ok := canvas.ClickedIn(sonnerAction); ok {
		state.flags ^= 1 << 8
	}
}
