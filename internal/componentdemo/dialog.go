package componentdemo

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var dialogComponent = Component{
	ID: "dialog", Title: "Dialog", Category: "Overlays", Summary: "真实开关、确认与取消，以及模态焦点约束。",
	paint: (*State).paintDialog, initialize: (*State).initDialog,
	Examples: []Example{
		{ID: "default", Title: "打开弹窗", Description: "从按钮打开；确认、取消、关闭与 Escape 均可结束弹窗。", Snippet: `if open {
    d := f.Dialog(ui.DialogSpec{
        Title: "Save changes?", Description: "This updates the local example state.",
        Width: 440, Scope: scope, CloseID: closeID,
    })
    d.Footer(func() {
        if f.Button(cancelID, "Cancel").Outline().Clicked() { open = false }
        if f.Button(confirmID, "Confirm").Clicked() { open, saved = false, true }
    })
    if d.Dismissed() { open = false }
}`},
		{ID: "confirm", Title: "确认操作", Description: "初始打开的确认弹窗；Tab 限于弹窗，关闭后可重新打开。", Snippet: `if open {
    d := f.Dialog(ui.DialogSpec{
        Title: "Discard changes?", Description: "Unsaved edits will be lost.",
        Scope: scope, CloseID: closeID,
    })
    d.Footer(func() {
        if f.Button(cancelID, "Cancel").Outline().Clicked() { open = false }
        if f.Button(confirmID, "Discard").Clicked() { open = false }
    })
    if d.Dismissed() { open = false }
}`},
		{ID: "destructive", Title: "危险操作", Description: "危险按钮仅改变本地示例结果，不删除任何数据。", Snippet: `if open {
    d := f.Dialog(ui.DialogSpec{
        Title: "Remove draft?", Description: "Demo only; no data is deleted.",
        Scope: scope, CloseID: closeID,
    })
    d.Footer(func() {
        if f.Button(cancelID, "Cancel").Outline().Clicked() { open = false }
        if f.Button(confirmID, "Remove").Destructive().Clicked() { open, removed = false, true }
    })
    if d.Dismissed() { open = false }
}`},
	},
}

var (
	dialogUI      = tree.Feature("dialog")
	dialogOpen    = tree.ID(dialogUI, "open")
	dialogClose   = tree.ID(dialogUI, "close")
	dialogCancel  = tree.ID(dialogUI, "cancel")
	dialogConfirm = tree.ID(dialogUI, "confirm")
	dialogFocus   = tree.ID(dialogUI, "focus")
)

type dialogState struct {
	open   bool
	result string
}

func (state *State) initDialog() {
	state.dialog.open = state.exampleID != "default"
}

func (state *State) closeDialog(result string) {
	state.dialog.open = false
	state.dialog.result = result
}

func (state *State) paintDialog(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	result := state.dialog.result
	if result == "" {
		result = state.Text("等待操作 · Confirm / Cancel", "Awaiting action · Confirm / Cancel")
	}
	state.page(canvas, theme, size, 480, state.Text("Dialog · 模态操作", "Dialog · Modal actions"), func(f *ui.Frame) {
		if f.Button(dialogOpen, "Open dialog").Outline().Self(ui.Start).Clicked() {
			state.dialog.open = true
		}
		f.Text(result).FontSize(theme.Metrics.FontSizeS).Muted()
		if !state.dialog.open {
			return
		}
		title, description := state.Text("保存更改？", "Save changes?"), state.Text("确认后更新本地示例状态。", "Confirm to update the local example state.")
		if state.exampleID == "destructive" {
			title, description = state.Text("移除草稿？", "Remove draft?"), state.Text("仅作演示，不会删除数据。", "Demo only; no data is deleted.")
		}
		// No DismissID: a press on the scrim does nothing, so the empty margin of
		// the content is never mistaken for a cancel. Escape and the close button
		// cancel.
		dialog := f.Dialog(ui.DialogSpec{
			Title: title, Description: description, Width: 440, Scope: dialogFocus, CloseID: dialogClose,
		})
		dialog.Body(func() {
			f.Text(state.Text("Tab 在弹窗内移动焦点。", "Tab moves focus within the dialog.")).FontSize(theme.Metrics.FontSizeS)
			f.Text(state.Text("Escape 取消并关闭弹窗。", "Escape cancels and closes the dialog.")).FontSize(theme.Metrics.FontSizeS).Muted()
		})
		dialog.Footer(func() {
			if f.Button(dialogCancel, "Cancel").Outline().Clicked() {
				state.closeDialog(state.Text("已取消 · 未应用更改", "Canceled · No changes applied"))
			}
			confirm := f.Button(dialogConfirm, "Confirm")
			if state.exampleID == "destructive" {
				confirm.Destructive()
			}
			if confirm.Clicked() {
				state.closeDialog(state.Text("已确认 · 仅更新示例状态", "Confirmed · Example state updated"))
			}
		})
		if dialog.Dismissed() {
			state.closeDialog(state.Text("已取消 · 未应用更改", "Canceled · No changes applied"))
		}
	})
}
