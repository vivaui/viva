package componentdemo

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

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
}`},
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
if canvas.Clicked(tree.ID(page, "action")) { selected = (selected + 1) % len(types) }`},
		{ID: "swipe", Title: "手势快照", Description: "仅展示调用方提供的横向位移；组件不解释手势。", Snippet: `snapshot := components.ToastSnapshot{
    Title: "Download almost complete", Description: "The horizontal offset comes from the caller's snapshot.",
    Type: components.ToastInfo, SwipeX: -8,
}
components.PaintToast(canvas, theme, viewport, snapshot, components.ToastOptions{
    Layer: 52, Scope: tree.ID(page, "toast"), DismissID: tree.ID(page, "dismiss"),
    ActionID: tree.ID(page, "action"), ActionLabel: "Continue",
})`},
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
		Title: state.Text("项目已保存", "Project saved"), Description: state.Text("更改已安全写入本地。", "Changes are safely written locally."),
		Type: components.ToastSuccess,
	}
	actionLabel := state.Text("查看", "View")
	if state.exampleID == "types" {
		actionLabel = state.Text("切换", "Switch")
		types := [...]components.ToastType{components.ToastInfo, components.ToastWarning, components.ToastError}
		titles := [...]string{
			state.Text("有可用更新", "Update available"),
			state.Text("磁盘空间不足", "Low disk space"),
			state.Text("构建失败", "Build failed"),
		}
		index := int(state.values[0])
		snapshot.Type = types[index]
		snapshot.Title = titles[index]
		snapshot.Description = state.Text("激活操作按钮查看下一个语义类型。", "Activate the action button to see the next semantic type.")
	} else if state.exampleID == "swipe" {
		snapshot.Type = components.ToastInfo
		snapshot.Title = state.Text("下载即将完成", "Download almost complete")
		snapshot.Description = state.Text("横向位移来自调用方快照。", "The horizontal offset comes from the caller's snapshot.")
		snapshot.SwipeX = -8
		actionLabel = state.Text("继续", "Continue")
	}
	components.PaintToast(canvas, theme, gfx.Rect{Width: size.Width, Height: size.Height}, snapshot, components.ToastOptions{
		Layer: 52, Scope: toastScope, DismissID: toastDismiss, ActionID: toastAction, ActionLabel: actionLabel,
	})

	if canvas.Clicked(toastDismiss) {
		state.flags &^= 1
	}
	if canvas.Clicked(toastAction) {
		state.values[0] = (state.values[0] + 1) % 3
	}
}
