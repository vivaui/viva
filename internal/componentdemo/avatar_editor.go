package componentdemo

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/avatareditor"
)

var avatarEditorComponent = Component{
	ID: "avatar-editor", Title: "Avatar Editor", Category: "Media",
	Summary: "嵌入任意容器的头像裁切表面，支持拖动、缩放、圆形预览和任意宽高比。",
	paint:   (*State).paintAvatarEditor,
	initialize: func(state *State) {
		const width, height = 360, 240
		pixels := make([]byte, width*height*4)
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				offset := (y*width + x) * 4
				pixels[offset] = byte(42 + x*170/width)
				pixels[offset+1] = byte(35 + y*190/height)
				pixels[offset+2] = byte(70 + (x+y)*80/(width+height))
				pixels[offset+3] = 255
			}
		}
		state.avatarEditor = &avatarEditorDemoState{
			bitmap: gfx.Bitmap{Pix: pixels, Width: width, Height: height, Stride: width * 4, ID: 0xa7e1, Version: 1},
		}
	},
	Examples: []Example{
		{
			ID: "avatar", Title: "嵌入圆形裁切表面", Height: 560,
			Description: "组件只绘制预览与缩放控件；标题、弹窗和确认操作由调用方安排。",
			Snippet: `layout := components.BeginAvatarEditor(canvas, theme, bounds, components.AvatarEditorOptions{
    IDs: ids, Bitmap: bitmap, State: draft,
    AspectRatio: 1, Circular: true, MaxStageSize: 320,
    Labels: labels,
}).Paint()
draft = layout.State
// The caller owns the surrounding dialog or panel and uses layout.Crop when layout.Ready.`,
			PageSnippet: `// m.avatarEditor is an avatareditor.Editor.
editor := m.avatarEditor
editor.Bitmap = m.avatarBitmap
editor.IDs = m.avatarIDs
editor.State = m.avatarCrop
editor.AspectRatio, editor.Circular = 1, true
editor.MaxStageSize = 320
editor.Labels.Canvas = "Image crop"
editor.Labels.Hint = "Drag to reposition. Scroll, use the slider, or use the buttons to zoom."
editor.View(c, "Avatar crop").FillWidth().MaxWidth(360)
m.avatarCrop = editor.State`,
			PageSnippetZh: `// m.avatarEditor 是 avatareditor.Editor。
editor := m.avatarEditor
editor.Bitmap = m.avatarBitmap
editor.IDs = m.avatarIDs
editor.State = m.avatarCrop
editor.AspectRatio, editor.Circular = 1, true
editor.MaxStageSize = 320
editor.Labels.Canvas = "头像裁切区域"
editor.Labels.Hint = "拖动图片调整位置，滚轮、滑杆或按钮缩放。"
editor.View(c, "头像裁切区域").FillWidth().MaxWidth(360)
m.avatarCrop = editor.State`,
		},
		{
			ID: "cover", Title: "自定义宽高比", Height: 520,
			Description: "同一个组件可裁剪 16:9 封面；Crop 始终是源图片中的目标比例区域。",
			Snippet: `options.AspectRatio = 16.0 / 9.0
options.Circular = false
layout := components.BeginAvatarEditor(canvas, theme, bounds, options).Paint()
sourceRegion := layout.Crop`,
			PageSnippet: `// m.coverEditor is an avatareditor.Editor.
editor := m.coverEditor
editor.Bitmap = m.coverBitmap
editor.IDs = m.coverIDs
editor.State = m.coverCrop
editor.AspectRatio, editor.Circular = 16.0/9.0, false
editor.MaxStageSize = 320
editor.View(c, "Cover crop").FillWidth().MaxWidth(440)
sourceRegion := editor.Layout().Crop
m.coverCrop = editor.State
// sourceRegion is the selected rectangle in the source image.`,
			PageSnippetZh: `// m.coverEditor 是 avatareditor.Editor。
editor := m.coverEditor
editor.Bitmap = m.coverBitmap
editor.IDs = m.coverIDs
editor.State = m.coverCrop
editor.AspectRatio, editor.Circular = 16.0/9.0, false
editor.MaxStageSize = 320
editor.View(c, "封面裁切区域").FillWidth().MaxWidth(440)
sourceRegion := editor.Layout().Crop
m.coverCrop = editor.State
// sourceRegion 是源图片中被选中的区域。`,
		},
	},
}

var avatarEditorDemoIDs = avatareditor.IDs{
	Canvas:  tree.ID(tree.Feature("avatar-editor"), "canvas"),
	Zoom:    tree.ID(tree.Feature("avatar-editor"), "zoom"),
	ZoomOut: tree.ID(tree.Feature("avatar-editor"), "zoom-out"),
	ZoomIn:  tree.ID(tree.Feature("avatar-editor"), "zoom-in"),
	Reset:   tree.ID(tree.Feature("avatar-editor"), "reset"),
}

type avatarEditorDemoState struct {
	bitmap gfx.Bitmap
	state  avatareditor.State
	view   avatareditor.Editor
}

func (state *State) paintAvatarEditor(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	demo := state.avatarEditor
	aspect := float32(1)
	circular := true
	if state.exampleID == "cover" {
		aspect, circular = 16.0/9.0, false
	}
	stageSize := min(float32(320), max(float32(1), size.Width-48))
	control := max(float32(32), theme.Metrics.ControlHeight)
	height := min(max(float32(1), size.Height-48), stageSize+control+12)
	demo.view.Bitmap = demo.bitmap
	demo.view.IDs = avatarEditorDemoIDs
	demo.view.State = demo.state
	demo.view.AspectRatio = aspect
	demo.view.Circular = circular
	demo.view.MaxStageSize = stageSize
	demo.view.Labels = avatareditor.Labels{
		Hint:   state.Text("拖动图片调整位置，滚轮、滑杆或按钮缩放。", "Drag to reposition. Scroll, use the slider, or use the buttons to zoom."),
		Canvas: state.Text("头像裁切区域", "Image crop"), Zoom: state.Text("缩放", "Zoom"),
		ZoomOut: state.Text("缩小", "Zoom out"), ZoomIn: state.Text("放大", "Zoom in"),
		Reset: state.Text("重置", "Reset"), Loading: state.Text("正在读取图片…", "Loading image…"),
		Unavailable: state.Text("无法读取这张图片", "This image could not be displayed."),
	}
	state.drawPage(canvas, theme, size, 440, state.Text("头像裁切 · 圆形预览", "Avatar editor · Circular crop"), func(c ui.C) {
		avatareditor.View(c, state.Text("头像裁切区域", "Image crop"), &demo.view).
			FillWidth().MaxWidth(440).Height(height)
	})
	demo.state = demo.view.State
}
