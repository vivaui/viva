package componentdemo

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var attachmentComponent = Component{
	ID: "attachment", Title: "Attachment", Category: "Data Display", Summary: "文件和图片附件的尺寸、方向与传输状态。",
	paint: (*State).paintAttachment,
	Examples: []Example{
		{ID: "default", Title: "文件状态", Description: "点击附件依次查看完成、上传、处理和错误状态。", Snippet: `file := tree.ID(page, "file")
card := components.BeginAttachment(canvas, theme, file, bounds,
    components.AttachmentData{
        Name: "product-demo.mov", Description: descriptions[index],
        Kind: components.AttachmentFile, State: states[index], Progress: 0.64,
    })
card.Paint()
if canvas.Clicked(file) {
    index = (index + 1) % len(states)
}`},
		{ID: "variants", Title: "尺寸与方向", Description: "展示小型文件与竖向图片附件。", Snippet: `components.BeginAttachment(canvas, theme, tree.ID(page, "notes"), bounds,
    components.AttachmentData{
        Name: "release-notes.md", Description: "18 KB · Markdown",
        Kind: components.AttachmentFile, State: components.AttachmentDone,
        Size: components.AttachmentSizeSmall,
    }).Paint()
components.BeginAttachment(canvas, theme, tree.ID(page, "cover"), cover,
    components.AttachmentData{
        Name: "cover.png", Description: "1200 × 1200", Kind: components.AttachmentImage,
        Orientation: components.AttachmentVertical,
    }).Paint()`},
		{ID: "actions", Title: "操作与分组", Description: "附件上的删除、重试按钮，以及一行并排的附件组。", Snippet: `group := components.BeginAttachmentGroup(theme, bounds)
for index, data := range attachments {
    card := components.BeginAttachment(canvas, theme, items.At(index), gfx.Rect{}, data)
    card = card.Action(components.AttachmentAction{
        ID: remove.At(index), Label: "Remove " + data.Name, Icon: crossIcon,
    })
    if group.Fits(card.Size()) {
        card.PaintIn(&group)
    }
}
failedCard := components.BeginAttachment(canvas, theme, failedID, cover, failed)
failedCard.Action(components.AttachmentAction{ID: retryID, Label: "Retry"}).Paint()`},
	},
}

var (
	attachmentUI = tree.Feature("attachment")
	// attachmentItem's item i is the i-th attachment of the example.
	attachmentItem = tree.Sub(attachmentUI, tree.Name("item"))
	// attachmentRemove's item i removes the i-th attachment of the group.
	attachmentRemove = tree.Sub(attachmentUI, tree.Name("remove"))
	attachmentRetry  = tree.ID(attachmentUI, "retry")
)

// Lucide's x and rotate-ccw (the arc drawn as cubic curves), compiled to the gfx.Icon form.
var (
	attachmentCrossIcon = gfx.Icon{ViewBox: 24, Stroke: 2, Cap: gfx.CapRound, Join: gfx.JoinRound, Commands: []gfx.PathCommand{
		gfx.MoveTo(18, 6), gfx.LineTo(6, 18), gfx.MoveTo(6, 6), gfx.LineTo(18, 18),
	}}
	attachmentRetryIcon = gfx.Icon{ViewBox: 24, Stroke: 2, Cap: gfx.CapRound, Join: gfx.JoinRound, Commands: []gfx.PathCommand{
		gfx.MoveTo(3, 12),
		gfx.CubicTo(3, 7.03, 7.03, 3, 12, 3),
		gfx.CubicTo(14.6, 3, 16.95, 4.1, 18.6, 5.9),
		gfx.MoveTo(21, 12),
		gfx.CubicTo(21, 16.97, 16.97, 21, 12, 21),
		gfx.CubicTo(9.4, 21, 7.05, 19.9, 5.4, 18.1),
		gfx.MoveTo(21, 3), gfx.LineTo(21, 8), gfx.LineTo(16, 8),
		gfx.MoveTo(3, 21), gfx.LineTo(3, 16), gfx.LineTo(8, 16),
	}}
)

func (state *State) paintAttachment(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	var first, second ui.Box
	variants := state.exampleID == "variants"
	state.drawPage(canvas, theme, size, 560, state.Text("Attachment · 生产资源摘要", "Attachment · Production resource summary"), func(c ui.C) {
		switch state.exampleID {
		case "variants":
			first = ui.Slot(c).FillWidth().Height(62)
			second = ui.Slot(c).Size(120, 170)
		case "actions":
			first = ui.Slot(c).FillWidth().Height(66)
			second = ui.Slot(c).FillWidth().Height(170)
		default:
			first = ui.Slot(c).FillWidth().Height(92)
			ui.Text(c, state.Text("点击附件切换状态。", "Click the attachment to cycle its state.")).FontSize(theme.Metrics.FontSizeS).Muted()
		}
	})
	if variants {
		components.BeginAttachment(canvas, theme, attachmentItem.At(0), first.Rect(), components.AttachmentData{
			Name: "release-notes.md", Description: "18 KB · Markdown", Kind: components.AttachmentFile,
			State: components.AttachmentDone, Size: components.AttachmentSizeSmall,
		}).Paint()
		components.BeginAttachment(canvas, theme, attachmentItem.At(1), second.Rect(), components.AttachmentData{
			Name: "cover.png", Description: "1200 × 1200", Kind: components.AttachmentImage,
			State: components.AttachmentIdle, Size: components.AttachmentSizeDefault, Orientation: components.AttachmentVertical,
		}).Paint()
		return
	}
	if state.exampleID == "actions" {
		// A row of natural-sized attachments, each with its own remove button, and
		// an upload that failed with a retry button.
		group := components.BeginAttachmentGroup(theme, first.Rect())
		names := [...]string{"release-notes.md", "cover.png", "report.pdf"}
		removeLabels := [...]string{"Remove release-notes.md", "Remove cover.png", "Remove report.pdf"}
		for index, name := range names {
			if state.flags&(1<<index) != 0 {
				continue // removed
			}
			card := components.BeginAttachment(canvas, theme, attachmentItem.At(index), gfx.Rect{}, components.AttachmentData{
				Name: name, Description: "18 KB", Kind: components.AttachmentFile,
			}).Action(components.AttachmentAction{ID: attachmentRemove.At(index), Label: removeLabels[index], Icon: attachmentCrossIcon})
			if !group.Fits(card.Size()) {
				break // a real row would scroll; the example shows what fits
			}
			card.PaintIn(&group)
		}
		vertical := components.BeginAttachment(canvas, theme, attachmentItem.At(3), gfx.Rect{X: second.Rect().X, Y: second.Rect().Y + 8}, components.AttachmentData{
			Name: "cover.png", Description: state.Text("上传失败", "Upload failed"), Kind: components.AttachmentImage, State: components.AttachmentError, Orientation: components.AttachmentVertical,
		}).Action(components.AttachmentAction{ID: attachmentRetry, Label: "Retry", Icon: attachmentRetryIcon})
		vertical.Paint()
		for index := range names {
			if canvas.Clicked(attachmentRemove.At(index)) {
				state.flags |= 1 << index
			}
		}
		if canvas.Clicked(attachmentRetry) {
			state.flags = 0
		}
		return
	}
	states := [...]components.AttachmentState{
		components.AttachmentDone, components.AttachmentUploading, components.AttachmentProcessing, components.AttachmentError,
	}
	descriptions := [...]string{
		state.Text("2.4 MB · 已完成", "2.4 MB · Complete"),
		state.Text("正在上传 · 64%", "Uploading · 64%"),
		state.Text("正在生成预览", "Generating preview"),
		state.Text("上传失败 · 点击重试", "Upload failed · Click to retry"),
	}
	index := int(state.values[0])
	components.BeginAttachment(canvas, theme, attachmentItem.At(0), first.Rect(), components.AttachmentData{
		Name: "product-demo.mov", Description: descriptions[index], Kind: components.AttachmentFile, State: states[index], Progress: 0.64,
	}).Paint()
	if canvas.Clicked(attachmentItem.At(0)) {
		state.values[0] = (state.values[0] + 1) % 4
	}
}
