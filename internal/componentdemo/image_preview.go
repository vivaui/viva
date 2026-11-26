package componentdemo

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/image"
	"github.com/vivaui/viva/ui/imagepreview"
)

var imagePreviewComponent = Component{
	ID: "image-preview", Title: "Image Preview", Category: "Data Display", Summary: "图片预览：点缩略图打开对话框，缩放、适应、实际大小，拖动平移。",
	initialize: (*State).initializeImageView, Source: "ui/imagepreview/imagepreview.go", paint: (*State).paintImagePreview,
	Examples: []Example{
		{ID: "default", Title: "缩略图与预览", Description: "缩略图是页面自己的控件，按下时调用 Open；滚轮、+ 和 - 缩放，0 回到适应，拖动平移，Esc 关闭。", Snippet: `m.preview = imagepreview.New()
// when the thumbnail is pressed:
if m.thumbnail.Clicked() {
    m.preview.Open()
}
m.preview.View(c, imagepreview.Spec{Bitmap: m.bitmap, Title: "Screenshot"})`, PageSnippet: `m.preview = imagepreview.New()
// when the thumbnail is pressed:
if m.thumbnail.Clicked() {
    m.preview.Open()
}
m.preview.View(c, imagepreview.Spec{Bitmap: m.bitmap, Title: "Screenshot"})`, PageSnippetZh: `m.preview = imagepreview.New()
// 按下缩略图时：
if m.thumbnail.Clicked() {
    m.preview.Open()
}
m.preview.View(c, imagepreview.Spec{Bitmap: m.bitmap, Title: "Screenshot"})`},
	},
}

type imagePreviewDemo struct {
	preview   *imagepreview.Preview
	thumbnail image.State
}

var imagePreviewDemos = map[*State]*imagePreviewDemo{}

func imagePreviewDemoFor(state *State) *imagePreviewDemo {
	if demo := imagePreviewDemos[state]; demo != nil {
		return demo
	}
	demo := &imagePreviewDemo{preview: imagepreview.New()}
	imagePreviewDemos[state] = demo
	return demo
}

func (state *State) paintImagePreview(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	demo := imagePreviewDemoFor(state)
	state.drawPage(canvas, theme, size, 520, state.Text("Image Preview · 图片预览", "Image Preview"), func(c ui.C) {
		demo.thumbnail.Reset().Bitmap(*state.imageBitmap).Fit()
		image.View(c, state.Text("缩略图", "Thumbnail"), &demo.thumbnail).Size(160, 100)
		if demo.thumbnail.Clicked() {
			demo.preview.Open()
		}
		demo.preview.View(c, imagepreview.Spec{
			Bitmap: *state.imageBitmap, Title: state.Text("图片预览", "Image preview"), Width: 440, Height: 260,
			Labels: [4]string{state.Text("缩小", "Zoom out"), state.Text("放大", "Zoom in"), state.Text("适应", "Fit"), state.Text("实际大小", "Actual size")},
		})
	})
}
