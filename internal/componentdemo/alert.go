package componentdemo

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/alert"
)

var alertComponent = Component{
	ID: "alert", Title: "Alert", Category: "Feedback", Summary: "页面内的提示条：状态图标、标题、说明，可带操作按钮和关闭按钮。",
	PageSource: "ui/alert/alert.go", paint: (*State).paintAlert,
	Examples: []Example{
		{ID: "default", Title: "说明", Description: "展示普通通知及其补充说明。", Snippet: `data := components.AlertData{
    Icon:        icon, // caller-owned gfx.Icon, optional
    Title:       "Sync complete",
    Description: "All local changes are safely on disk.",
}
components.Alert(canvas, theme, bounds, data)`, PageSnippet: `m.notice = alert.New()
// in the page:
m.notice.View(c, alert.Spec{
    Title:       "Sync complete",
    Description: "All local changes are safely on disk.",
}).FillWidth()`, PageSnippetZh: `m.notice = alert.New()
// 在页面中：
m.notice.View(c, alert.Spec{
    Title:       "Sync complete",
    Description: "All local changes are safely on disk.",
}).FillWidth()`},
		{ID: "variants", Title: "状态", Description: "默认、成功、警告、危险共用 ui.Status，颜色取自主题。", Snippet: `dangerData := components.AlertData{
    Variant: components.AlertDestructive, Icon: icon,
    Title: "Unable to delete project", Description: "Remove any release configurations that still use this project first.",
}
components.Alert(canvas, theme, bounds, dangerData)`, PageSnippet: `m.notice.View(c, alert.Spec{
    Status:      ui.StatusDanger,
    Title:       "Unable to delete project",
    Description: "Remove any release configurations that still use this project first.",
}).FillWidth()`, PageSnippetZh: `m.notice.View(c, alert.Spec{
    Status:      ui.StatusDanger,
    Title:       "Unable to delete project",
    Description: "Remove any release configurations that still use this project first.",
}).FillWidth()`},
		{ID: "actions", Title: "操作、关闭与横幅", Description: "Action 与 Closable 给提示条加按钮，关闭后保持关闭；Banner 去掉圆角与侧边框，用于页面顶部。", Snippet: `// A page control only: the canvas recipe has no buttons.
// See PageSnippet.`, PageSnippet: `m.notice.View(c, alert.Spec{
    Status:   ui.StatusWarning,
    Title:    "Host key changed",
    Action:   "Review",
    Closable: true,
}).FillWidth()
if m.notice.ActionClicked() {
    // the user pressed Review
}
m.offline.View(c, alert.Spec{Status: ui.StatusDanger, Title: "Offline", Banner: true}).FillWidth()`, PageSnippetZh: `m.notice.View(c, alert.Spec{
    Status:   ui.StatusWarning,
    Title:    "Host key changed",
    Action:   "Review",
    Closable: true,
}).FillWidth()
if m.notice.ActionClicked() {
    // 用户点了 Review
}
m.offline.View(c, alert.Spec{Status: ui.StatusDanger, Title: "Offline", Banner: true}).FillWidth()`},
	},
}

type alertDemo struct {
	notices [4]*alert.Alert
	banner  *alert.Alert
}

var alertDemos = map[*State]*alertDemo{}

func alertDemoFor(state *State) *alertDemo {
	if demo := alertDemos[state]; demo != nil {
		return demo
	}
	demo := &alertDemo{banner: alert.New()}
	for i := range demo.notices {
		demo.notices[i] = alert.New()
	}
	alertDemos[state] = demo
	return demo
}

func (state *State) paintAlert(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	demo := alertDemoFor(state)
	close := state.Text("关闭", "Close")
	state.drawPage(canvas, theme, size, 560, state.Text("Alert · 明确反馈", "Alert · Clear feedback"), func(c ui.C) {
		switch state.exampleID {
		case "variants":
			demo.notices[0].View(c, alert.Spec{Title: state.Text("同步已完成", "Sync complete"), Description: state.Text("所有本地更改都已安全写入磁盘。", "All local changes are safely on disk.")}).FillWidth()
			demo.notices[1].View(c, alert.Spec{Status: ui.StatusSuccess, Title: state.Text("已保存", "Saved"), Description: state.Text("所有更改已写入主机。", "All changes were written to the host.")}).FillWidth()
			demo.notices[2].View(c, alert.Spec{Status: ui.StatusWarning, Title: state.Text("主机密钥已变更", "Host key changed"), Description: state.Text("连接前请先核对。", "Review it before you connect.")}).FillWidth()
			demo.notices[3].View(c, alert.Spec{Status: ui.StatusDanger, Title: state.Text("无法删除项目", "Unable to delete project"), Description: state.Text("请先移除仍在使用该项目的发布配置。", "Remove any release configurations that still use this project first.")}).FillWidth()
		case "actions":
			demo.notices[0].View(c, alert.Spec{Status: ui.StatusWarning, Title: state.Text("主机密钥已变更", "Host key changed"), Description: state.Text("build-01 的密钥与你信任的不一致。", "The key of build-01 differs from the one you trusted."), Action: state.Text("核对", "Review"), Closable: true, CloseLabel: close}).FillWidth()
			demo.banner.View(c, alert.Spec{Status: ui.StatusDanger, Title: state.Text("已离线：更改将稍后同步", "Offline: changes will sync later"), Banner: true}).FillWidth()
		default:
			demo.notices[0].View(c, alert.Spec{Title: state.Text("同步已完成", "Sync complete"), Description: state.Text("所有本地更改都已安全写入磁盘。远端主机确认了最后一次写入，可以放心关闭会话。", "All local changes are safely on disk. The remote host confirmed the final write, so you can close the session safely.")}).FillWidth()
		}
	})
}
