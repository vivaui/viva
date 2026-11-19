package componentdemo

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/attachmentview"
)

var attachmentComponent = Component{
	ID: "attachment", Title: "Attachment", Category: "Data Display", Summary: "文件和图片附件的尺寸、方向与传输状态。",
	paint: (*State).paintAttachment, initialize: (*State).initializeAttachment,
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
}`, PageSnippet: `// m.attachment* were made with attachmentview.New() in the model.
card := m.attachment
states := [...]string{"2.4 MB · Complete", "Uploading · 64%", "Generating preview", "Upload failed · Click to retry"}
card.Reset().ID(m.fileID).Title("product-demo.mov").Meta(states[m.fileState]).File()
switch m.fileState {
case 1:
    card.Uploading(0.64)
case 2:
    card.Processing()
case 3:
    card.Failed()
}
card.OnClick(m.nextState)
card.View(c, "product-demo.mov").FillWidth()`, PageSnippetZh: `// m.attachment* 在 model 中用 attachmentview.New() 创建。
card := m.attachment
states := [...]string{"2.4 MB · 已完成", "正在上传 · 64%", "正在生成预览", "上传失败 · 点击重试"}
card.Reset().ID(m.fileID).Title("product-demo.mov").Meta(states[m.fileState]).File()
switch m.fileState {
case 1:
    card.Uploading(0.64)
case 2:
    card.Processing()
case 3:
    card.Failed()
}
card.OnClick(m.nextState)
card.View(c, "product-demo.mov").FillWidth()`},
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
    }).Paint()`, PageSnippet: `// m.attachment* were made with attachmentview.New() in the model.
file := m.smallAttachment
file.Reset().ID(m.notesID).Title("release-notes.md").Meta("18 KB · Markdown").File().Small()
file.View(c, "release-notes.md")
cover := m.coverAttachment
cover.Reset().ID(m.coverID).Title("cover.png").Meta("1200 × 1200").Image().Idle().Vertical()
cover.View(c, "cover.png")`, PageSnippetZh: `// m.attachment* 在 model 中用 attachmentview.New() 创建。
file := m.smallAttachment
file.Reset().ID(m.notesID).Title("release-notes.md").Meta("18 KB · Markdown").File().Small()
file.View(c, "release-notes.md")
cover := m.coverAttachment
cover.Reset().ID(m.coverID).Title("cover.png").Meta("1200 × 1200").Image().Idle().Vertical()
cover.View(c, "cover.png")`},
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
failedCard.Action(components.AttachmentAction{ID: retryID, Label: "Retry"}).Paint()`, PageSnippet: `group := m.attachmentGroup
group.Reset().Gap(12)
for index, name := range [...]string{"release-notes.md", "cover.png", "report.pdf"} {
    if m.removed(index) {
        continue
    }
    group.Add().ID(m.itemIDs[index]).ExtraSmall().Title(name).Meta("18 KB").
        Action(m.removeIDs[index], "Remove "+name, m.crossIcon, func() { m.remove(index) })
}
group.View(c, "Attachments")
failed := m.failedAttachment
failed.Reset().ID(m.failedID).Title("cover.png").Meta("Upload failed").
    Image().Failed().Vertical().
    Action(m.retryID, "Retry", m.retryIcon, m.retry)
failed.View(c, "cover.png")`, PageSnippetZh: `group := m.attachmentGroup
group.Reset().Gap(12)
for index, name := range [...]string{"release-notes.md", "cover.png", "report.pdf"} {
    if m.removed(index) {
        continue
    }
    group.Add().ID(m.itemIDs[index]).ExtraSmall().Title(name).Meta("18 KB").
        Action(m.removeIDs[index], "删除 "+name, m.crossIcon, func() { m.remove(index) })
}
group.View(c, "附件")
failed := m.failedAttachment
failed.Reset().ID(m.failedID).Title("cover.png").Meta("上传失败").
    Image().Failed().Vertical().
    Action(m.retryID, "重试", m.retryIcon, m.retry)
failed.View(c, "cover.png")`},
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

// attachmentDemo is one attachment example's page controls and the callbacks
// they run. They are kept beside the State rather than rebuilt in each paint,
// so their handlers and draw functions are stable and a warm paint allocates
// nothing.
type attachmentDemo struct {
	card, small, cover, failed *attachmentview.Attachment
	group                      *attachmentview.Group
	next                       func()
	remove                     [3]func()
	retry                      func()
	removeLabels               [3]string
	retryLabel                 string
}

// attachmentDemos is keyed by State because the constrained demo model has no
// field for these controls; a State is painted on one thread at a time.
var attachmentDemos = map[*State]*attachmentDemo{}

var attachmentNames = [...]string{"release-notes.md", "cover.png", "report.pdf"}

func (state *State) initializeAttachment() { attachmentDemoFor(state) }

func attachmentDemoFor(state *State) *attachmentDemo {
	if demo := attachmentDemos[state]; demo != nil {
		return demo
	}
	demo := &attachmentDemo{
		card:   attachmentview.New(),
		small:  attachmentview.New(),
		cover:  attachmentview.New(),
		failed: attachmentview.New(),
		group:  attachmentview.NewGroup(),
	}
	demo.next = func() { state.values[0] = (state.values[0] + 1) % 4 }
	for index := range demo.remove {
		value := index
		demo.remove[index] = func() { state.flags |= 1 << value }
		prefix := state.Text("删除 ", "Remove ")
		demo.removeLabels[index] = prefix + attachmentNames[index]
	}
	demo.retry = func() { state.flags = 0 }
	demo.retryLabel = state.Text("重试", "Retry")
	attachmentDemos[state] = demo
	return demo
}

func (state *State) paintAttachment(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	demo := attachmentDemoFor(state)
	state.drawPage(canvas, theme, size, 560, state.Text("Attachment · 生产资源摘要", "Attachment · Production resource summary"), func(c ui.C) {
		switch state.exampleID {
		case "variants":
			demo.small.Reset().ID(attachmentItem.At(0)).Title("release-notes.md").Meta("18 KB · Markdown").File().Small()
			demo.cover.Reset().ID(attachmentItem.At(1)).Title("cover.png").Meta("1200 × 1200").Image().Idle().Vertical()
			ui.Row(c).Gap(12).Items(ui.Start).Children(func() {
				demo.small.View(c, "release-notes.md")
				demo.cover.View(c, "cover.png")
			})
		case "actions":
			group := demo.group
			group.Reset().Gap(12)
			for index, name := range attachmentNames {
				if state.flags&(1<<index) != 0 {
					continue // removed
				}
				group.Add().ID(attachmentItem.At(index)).ExtraSmall().Title(name).Meta("18 KB").File().
					Action(attachmentRemove.At(index), demo.removeLabels[index], attachmentCrossIcon, demo.remove[index])
			}
			demo.failed.Reset().ID(attachmentItem.At(3)).Title("cover.png").Meta(state.Text("上传失败", "Upload failed")).
				Image().Failed().Vertical().
				Action(attachmentRetry, demo.retryLabel, attachmentRetryIcon, demo.retry)
			ui.Column(c).Gap(12).Items(ui.Start).Children(func() {
				group.View(c, state.Text("附件", "Attachments"))
				demo.failed.View(c, "cover.png")
			})
		default:
			index := int(state.values[0])
			descriptions := [...]string{
				state.Text("2.4 MB · 已完成", "2.4 MB · Complete"),
				state.Text("正在上传 · 64%", "Uploading · 64%"),
				state.Text("正在生成预览", "Generating preview"),
				state.Text("上传失败 · 点击重试", "Upload failed · Click to retry"),
			}
			card := demo.card
			card.Reset().ID(attachmentItem.At(0)).Title("product-demo.mov").Meta(descriptions[index]).File()
			switch index {
			case 1:
				card.Uploading(0.64)
			case 2:
				card.Processing()
			case 3:
				card.Failed()
			}
			card.OnClick(demo.next)
			card.View(c, "product-demo.mov").FillWidth()
		}
	})
}
