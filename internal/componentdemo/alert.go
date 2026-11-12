package componentdemo

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var alertComponent = Component{
	ID: "alert", Title: "Alert", Category: "Feedback", Summary: "用于普通说明和破坏性提示的静态消息面板。",
	paint: (*State).paintAlert,
	Examples: []Example{
		{ID: "default", Title: "说明", Description: "展示普通通知及其补充说明。", Snippet: `data := components.AlertData{
    Icon:        icon, // caller-owned gfx.Icon, optional
    Title:       "Sync complete",
    Description: "All local changes are safely on disk.",
}
components.Alert(canvas, theme, bounds, data)`},
		{ID: "variants", Title: "语义变体", Description: "并列展示默认与破坏性提示。", Snippet: `plainData := components.AlertData{
    Icon: icon, Title: "Sync complete",
    Description: "All local changes are safely on disk.",
}
dangerData := components.AlertData{
    Variant: components.AlertDestructive, Icon: icon,
    Title: "Unable to delete project", Description: "Remove any release configurations that still use this project first.",
}
components.Alert(canvas, theme, plainBounds, plainData)
components.Alert(canvas, theme, dangerBounds, dangerData)`},
	},
}

func (state *State) paintAlert(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	plainData := components.AlertData{
		Icon:        infoIcon,
		Title:       state.Text("同步已完成", "Sync complete"),
		Description: state.Text("所有本地更改都已安全写入磁盘。远端主机确认了最后一次写入，可以放心关闭会话。", "All local changes are safely on disk. The remote host confirmed the final write, so you can close the session safely."),
	}
	dangerData := components.AlertData{
		Variant: components.AlertDestructive, Icon: alertIcon, Title: state.Text("无法删除项目", "Unable to delete project"),
		Description: state.Text("请先移除仍在使用该项目的发布配置。", "Remove any release configurations that still use this project first."),
	}
	var plain, destructive ui.Box
	variants := state.exampleID == "variants"
	// The page column is at most 560 wide inside 24 of padding.
	contentWidth := min(float32(560), size.Width-48)
	state.drawPage(canvas, theme, size, 560, state.Text("Alert · 明确反馈", "Alert · Clear feedback"), func(c ui.C) {
		plain = ui.Slot(c).FillWidth().Height(components.AlertHeight(canvas, theme, contentWidth, plainData))
		if variants {
			destructive = ui.Slot(c).FillWidth().Height(components.AlertHeight(canvas, theme, contentWidth, dangerData))
		}
	})
	components.Alert(canvas, theme, plain.Rect(), plainData)
	if variants {
		components.Alert(canvas, theme, destructive.Rect(), dangerData)
	}
}
