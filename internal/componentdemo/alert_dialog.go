package componentdemo

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var alertDialogComponent = Component{
	ID: "alert-dialog", Title: "Alert Dialog", Category: "Overlay", Summary: "需要明确确认或取消的模态警告。",
	paint: (*State).paintAlertDialog, initialize: initializeAlertDialog,
	Examples: []Example{
		{ID: "default", Title: "删除确认", Description: "确认和取消动作均由调用方处理。", Snippet: `options := components.AlertDialogOptions{
    Layer: 8, Scope: m.scope, ActionID: m.confirmID, CancelID: m.cancelID,
    ActionVariant: components.ButtonDestructive,
}
components.BeginAlertDialog(canvas, theme, viewport, options).
    Paint("Delete this project?", "This cannot be undone.", "Delete", "Cancel")
if canvas.Clicked(m.confirmID) || canvas.Clicked(m.cancelID) {
    open = false
}`, PageSnippet: `ui.AlertDialog(c, ui.AlertDialogSpec{
    Title: "Permanently delete this project?",
    Description: "This cannot be undone. Local data will be removed.",
    Scope: m.scope, ActionID: m.confirmID, CancelID: m.cancelID,
    ActionLabel: "Delete", CancelLabel: "Cancel",
})`, PageSnippetZh: `ui.AlertDialog(c, ui.AlertDialogSpec{
    Title: "永久删除这个项目？",
    Description: "此操作无法撤销，本地数据将被移除。",
    Scope: m.scope, ActionID: m.confirmID, CancelID: m.cancelID,
    ActionLabel: "删除", CancelLabel: "取消",
})`},
		{ID: "media", Title: "带媒体的小尺寸", Description: "预留可由调用方绘制的媒体区域。", Snippet: `dialog := components.BeginAlertDialog(canvas, theme, viewport, components.AlertDialogOptions{
    Layer: 8, Scope: m.scope, ActionID: m.confirmID, CancelID: m.cancelID,
    Size: components.AlertDialogSmall,
}).WithMedia()
layout := dialog.Begin("Replace avatar?", "The new image replaces it.", "Replace", "Cancel")
canvas.Rounded(layout.Media, min(layout.Media.Width, layout.Media.Height)/2, theme.Muted)
canvas.LabelCenter("SK", layout.Media, gfx.TextStyle{Size: theme.Metrics.FontSizeM})
dialog.End()`, PageSnippet: `ui.Dialog(c, ui.DialogSpec{
    Title: "Replace avatar?",
    Description: "The current avatar is replaced immediately with the new local image.",
    Width: 320, Scope: m.scope, DismissID: m.scope, NoClose: true,
}).Body(func() {
    ui.Column(c).Size(64, 64).Center().Radius(32).Background(c.Theme().Muted).Children(func() {
        ui.Text(c, "SK").Weight(gfx.WeightMedium)
    })
}).Footer(func() {
    if ui.Button(c, "Cancel").Outline().Hit(m.cancelID, "Cancel").Clicked() {
        m.alertOpen = false
    }
    if ui.Button(c, "Replace").Hit(m.confirmID, "Replace").Clicked() {
        m.alertOpen = false
    }
})`, PageSnippetZh: `ui.Dialog(c, ui.DialogSpec{
    Title: "替换头像？",
    Description: "当前头像将立即被新的本地图片替换。",
    Width: 320, Scope: m.scope, DismissID: m.scope, NoClose: true,
}).Body(func() {
    ui.Column(c).Size(64, 64).Center().Radius(32).Background(c.Theme().Muted).Children(func() {
        ui.Text(c, "SK").Weight(gfx.WeightMedium)
    })
}).Footer(func() {
    if ui.Button(c, "取消").Outline().Hit(m.cancelID, "取消").Clicked() {
        m.alertOpen = false
    }
    if ui.Button(c, "替换").Hit(m.confirmID, "替换").Clicked() {
        m.alertOpen = false
    }
})`},
	},
}

var (
	alertDialogUI     = tree.Feature("alert-dialog")
	alertDialogOpen   = tree.ID(alertDialogUI, "open")
	alertDialogAction = tree.ID(alertDialogUI, "action")
	alertDialogCancel = tree.ID(alertDialogUI, "cancel")
	alertDialogScope  = tree.ID(alertDialogUI, "scope")
)

const alertDialogOpenFlag uint64 = 1

func initializeAlertDialog(state *State) { state.flags |= alertDialogOpenFlag }

func (state *State) paintAlertDialog(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	if state.flags&alertDialogOpenFlag == 0 {
		state.drawPage(canvas, theme, size, 320, state.Text("操作已取消", "Action canceled"), func(c ui.C) {
			label := state.Text("再次打开警告", "Reopen alert")
			if ui.Button(c, label).Hit(alertDialogOpen, label).Clicked() {
				state.flags |= alertDialogOpenFlag
			}
		})
		return
	}
	state.drawPage(canvas, theme, size, 320, state.Text("Alert Dialog · 确认", "Alert Dialog · Confirmation"), func(c ui.C) {
		if state.exampleID == "media" {
			dialog := ui.Dialog(c, ui.DialogSpec{
				Title:       state.Text("替换头像？", "Replace avatar?"),
				Description: state.Text("当前头像将立即被新的本地图片替换。", "The current avatar is replaced immediately with the new local image."),
				Width:       320, Scope: alertDialogScope, DismissID: alertDialogScope, NoClose: true,
			})
			dialog.Body(func() {
				ui.Column(c).Size(64, 64).Center().Radius(32).Background(c.Theme().Muted).Children(func() {
					ui.Text(c, "SK").Weight(gfx.WeightMedium)
				})
			})
			dialog.Footer(func() {
				cancel, replace := state.Text("取消", "Cancel"), state.Text("替换", "Replace")
				if ui.Button(c, cancel).Outline().Hit(alertDialogCancel, cancel).Clicked() {
					state.flags &^= alertDialogOpenFlag
				}
				if ui.Button(c, replace).Hit(alertDialogAction, replace).Clicked() {
					state.flags &^= alertDialogOpenFlag
				}
			})
			if dialog.Dismissed() {
				state.flags &^= alertDialogOpenFlag
			}
			return
		}
		dialog := ui.AlertDialog(c, ui.AlertDialogSpec{
			Title:       state.Text("永久删除这个项目？", "Permanently delete this project?"),
			Description: state.Text("此操作无法撤销，本地数据将被移除。", "This cannot be undone. Local data will be removed."),
			Scope:       alertDialogScope, ActionID: alertDialogAction, CancelID: alertDialogCancel,
			ActionLabel: state.Text("删除", "Delete"), CancelLabel: state.Text("取消", "Cancel"),
			ActionVariant: components.ButtonDestructive,
		})
		if id, ok := dialog.Chosen(); ok && (id == alertDialogAction || id == alertDialogCancel) {
			state.flags &^= alertDialogOpenFlag
		}
	})
}
