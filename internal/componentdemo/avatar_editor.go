package componentdemo

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
)

var avatarEditorComponent = Component{
	ID: "avatar-editor", Title: "Avatar Editor", Category: "Media",
	Summary: "用可拖动、可缩放的画布裁切头像，并支持任意宽高比和圆形预览。",
	paint:   (*State).paintAvatarEditor, pointer: (*State).pointerAvatarEditor,
	key: (*State).keyAvatarEditor, scroll: (*State).scrollAvatarEditor,
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
			open:   true,
		}
		components.InitializeAvatarEditorState(&state.avatarEditor.state)
	},
	Examples: []Example{
		{
			ID: "avatar", Title: "方形圆形预览", Height: 560,
			Description: "头像场景使用 1:1 比例；拖动画布定位，滚轮、滑杆、按钮或键盘缩放。",
			Snippet: `layout := components.BeginAvatarEditor(canvas, theme, viewport, components.AvatarEditorOptions{
    IDs: ids, Bitmap: bitmap, State: draft,
    AspectRatio: 1, Circular: true, Viewport: viewport,
    Labels: labels,
}).Paint()
// Route pointer, wheel, and key events to layout.Pointer, layout.Scroll,
// and layout.Key. Commit layout.Crop when the application's Confirm ID clicks.`,
		},
		{
			ID: "cover", Title: "自定义宽高比", Height: 520,
			Description: "同一个组件可裁剪 16:9 封面；Crop 始终是源图片中的目标比例区域。",
			Snippet: `options.AspectRatio = 16.0 / 9.0
options.Circular = false
layout := components.BeginAvatarEditor(canvas, theme, viewport, options).Paint()
sourceRegion := layout.Crop`,
		},
	},
}

var avatarEditorDemoFeature = tree.Feature("avatar-editor")
var avatarEditorDemoTrigger = tree.ID(avatarEditorDemoFeature, "open")
var avatarEditorDemoIDs = components.AvatarEditorIDs{
	Scope: tree.ID(avatarEditorDemoFeature, "scope"), Dismiss: tree.ID(avatarEditorDemoFeature, "dismiss"),
	Close: tree.ID(avatarEditorDemoFeature, "close"), Canvas: tree.ID(avatarEditorDemoFeature, "canvas"),
	Zoom: tree.ID(avatarEditorDemoFeature, "zoom"), ZoomOut: tree.ID(avatarEditorDemoFeature, "zoom-out"),
	ZoomIn: tree.ID(avatarEditorDemoFeature, "zoom-in"), Reset: tree.ID(avatarEditorDemoFeature, "reset"),
	Cancel: tree.ID(avatarEditorDemoFeature, "cancel"), Confirm: tree.ID(avatarEditorDemoFeature, "confirm"),
}

type avatarEditorDemoState struct {
	bitmap      gfx.Bitmap
	state       components.AvatarEditorState
	layout      components.AvatarEditorLayout
	open, saved bool
}

func (state *State) paintAvatarEditor(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	demo := state.avatarEditor
	if !demo.open {
		canvas.Label(state.Text("头像裁切", "Avatar crop"), gfx.Rect{X: 24, Y: 24, Width: 320, Height: 28},
			gfx.TextStyle{Size: theme.Metrics.FontSizeL, Weight: gfx.WeightSemibold, Color: theme.Foreground})
		canvas.Label(state.Text("裁切已确认。可以重新打开编辑器继续体验。", "The crop was confirmed. Open the editor to try it again."),
			gfx.Rect{X: 24, Y: 62, Width: min(520, size.Width-48), Height: 40},
			gfx.TextStyle{Size: theme.Metrics.FontSizeM, Color: theme.MutedForeground})
		button := components.BeginButton(canvas, theme, avatarEditorDemoTrigger, gfx.Rect{X: 24, Y: 116, Width: 142, Height: 32}).Default(state.Text("重新打开编辑器", "Open editor"))
		if button.Clicked() {
			demo.open = true
			components.InitializeAvatarEditorState(&demo.state)
		}
		return
	}
	aspect := float32(1)
	circular := true
	if state.exampleID == "cover" {
		aspect, circular = 16.0/9.0, false
	}
	options := components.AvatarEditorOptions{
		IDs: avatarEditorDemoIDs, Bitmap: demo.bitmap, State: demo.state,
		AspectRatio: aspect, Circular: circular, Viewport: gfx.Rect{Width: size.Width, Height: size.Height},
		Layer: 4,
		Labels: components.AvatarEditorLabels{
			Title: state.Text("调整头像", "Adjust image"),
			Description: state.Text(
				"拖动图片调整位置，滚轮、滑杆或按钮缩放。",
				"Drag to reposition. Scroll, drag the slider, or use the buttons to zoom.",
			),
			Canvas: state.Text("头像裁切区域", "Image crop"), Zoom: state.Text("缩放", "Zoom"),
			ZoomOut: state.Text("缩小", "Zoom out"), ZoomIn: state.Text("放大", "Zoom in"),
			Reset: state.Text("重置", "Reset"), Cancel: state.Text("取消", "Cancel"),
			Confirm:     state.Text("确定", "Apply"),
			Loading:     state.Text("正在读取图片…", "Loading image…"),
			Unavailable: state.Text("无法读取这张图片", "This image could not be displayed."),
		},
	}
	demo.layout = components.BeginAvatarEditor(canvas, theme, options.Viewport, options).Paint()
	demo.state = demo.layout.State
	switch {
	case canvas.Clicked(avatarEditorDemoIDs.Dismiss), canvas.Clicked(avatarEditorDemoIDs.Close),
		canvas.Clicked(avatarEditorDemoIDs.Cancel):
		demo.open = false
		canvas.RequestFocus(avatarEditorDemoTrigger)
	case canvas.Clicked(avatarEditorDemoIDs.Confirm):
		demo.saved = true
		demo.open = false
		canvas.RequestFocus(avatarEditorDemoTrigger)
	}
}

func (state *State) pointerAvatarEditor(event gfx.PointerEvent) gfx.EventResult {
	if state.avatarEditor == nil || !state.avatarEditor.open {
		return gfx.EventResult{}
	}
	var result gfx.EventResult
	state.avatarEditor.state, result = state.avatarEditor.layout.Pointer(event, state.avatarEditor.state)
	return result
}

func (state *State) scrollAvatarEditor(event gfx.ScrollEvent) gfx.EventResult {
	if state.avatarEditor == nil || !state.avatarEditor.open {
		return gfx.EventResult{}
	}
	var result gfx.EventResult
	state.avatarEditor.state, result = state.avatarEditor.layout.Scroll(event, state.avatarEditor.state)
	return result
}

func (state *State) keyAvatarEditor(event gfx.KeyEvent) gfx.EventResult {
	if state.avatarEditor == nil || !state.avatarEditor.open {
		return gfx.EventResult{}
	}
	if event.Key == gfx.KeyEscape {
		state.avatarEditor.open = false
		return gfx.EventResult{Handled: true, Focus: avatarEditorDemoTrigger}
	}
	var result gfx.EventResult
	state.avatarEditor.state, result = state.avatarEditor.layout.Key(event, state.avatarEditor.state)
	if result.Activate == avatarEditorDemoIDs.Confirm {
		state.avatarEditor.saved = true
		state.avatarEditor.open = false
		result.Focus = avatarEditorDemoTrigger
	}
	return result
}
