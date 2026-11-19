package componentdemo

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/image"
)

var imageViewComponent = Component{
	ID: "image-view", Title: "Image View", Category: "Media", Summary: "按视口适配或按实际像素显示位图，与显示缩放无关。",
	initialize: (*State).initializeImageView,
	paint:      (*State).paintImageView,
	Examples: []Example{
		{ID: "fit", Title: "适配视口", Description: "应用提供已解码且有界的像素；页面控件把整张图居中放进调用方给出的视口。", Height: 300, Snippet: `components.PaintImageView(canvas, theme, bounds, components.ImageViewOptions{Bitmap: bitmap})`, PageSnippet: `// m.bitmap is the caller-decoded 160 × 100 image.
m.picture.Reset().Bitmap(m.bitmap).Fit()
image.View(c, "Fit preview", m.picture).FillWidth().Height(240)`, PageSnippetZh: `// m.bitmap 是调用方已解码的 160 × 100 图像。
m.picture.Reset().Bitmap(m.bitmap).Fit()
image.View(c, "适配预览", m.picture).FillWidth().Height(240)`},
		{ID: "actual", Title: "一个图像像素对应一个设备像素", Description: "Actual 在分数 DPI 缩放下保持实际像素尺寸；视口由页面用 Size 给出。", Height: 300, Snippet: `components.PaintImageView(canvas, theme, bounds, components.ImageViewOptions{Bitmap: bitmap, Zoom: 1})`, PageSnippet: `// m.bitmap is 160 × 100 bitmap pixels; Actual is one per device pixel.
m.picture.Reset().Bitmap(m.bitmap).Actual()
image.View(c, "Actual pixels", m.picture).Size(160, 100)`, PageSnippetZh: `// m.bitmap 是 160 × 100 位图像素；Actual 让一个位图像素对应一个设备像素。
m.picture.Reset().Bitmap(m.bitmap).Actual()
image.View(c, "实际像素", m.picture).Size(160, 100)`},
		{ID: "zoom-pan", Title: "缩放与平移", Description: "显式 Zoom 与 Pan 让大图超出视口并由组件裁剪；Pointer 与 Scroll 交给应用处理拖动和滚轮缩放。", Height: 300, Snippet: `components.PaintImageView(canvas, theme, bounds, components.ImageViewOptions{
    Bitmap: bitmap, Zoom: 2, Pan: gfx.Point{X: 18, Y: -12},
})`, PageSnippet: `// m.zoom and m.pan are the application's zoom and pan state.
m.picture.Reset().Bitmap(m.bitmap).Zoom(m.zoom).Pan(m.pan)
m.picture.Pointer = m.dragImage
m.picture.Scroll = m.zoomImage
image.View(c, "Zoom and pan", m.picture).FillWidth().Height(240)`, PageSnippetZh: `// m.zoom 和 m.pan 是应用的缩放与平移状态。
m.picture.Reset().Bitmap(m.bitmap).Zoom(m.zoom).Pan(m.pan)
m.picture.Pointer = m.dragImage
m.picture.Scroll = m.zoomImage
image.View(c, "缩放与平移", m.picture).FillWidth().Height(240)`},
	},
}

func (state *State) initializeImageView() {
	const w, h = 160, 100
	pix := make([]byte, w*h*4)
	for y := range h {
		for x := range w {
			i := (y*w + x) * 4
			pix[i] = byte(x * 255 / w)
			pix[i+1] = byte(y * 255 / h)
			pix[i+2] = 100
			pix[i+3] = 255
		}
	}
	state.imageBitmap = &gfx.Bitmap{Pix: pix, Width: w, Height: h, Stride: w * 4, ID: 1, Version: 1}
}

// imageViewDemo is one image example's page control and its drag/zoom state. It
// is kept beside the State rather than rebuilt in each paint, so the control's
// handlers and draw function are stable and a warm paint allocates nothing.
type imageViewDemo struct {
	state    *State
	view     image.State
	zoom     float32
	pan      gfx.Point
	last     gfx.Point
	dragging bool
}

// imageViewDemos is keyed by State because the constrained demo model has no
// field for this example; a State is painted on one thread at a time.
var imageViewDemos = map[*State]*imageViewDemo{}

func imageViewDemoFor(state *State) *imageViewDemo {
	if demo := imageViewDemos[state]; demo != nil {
		return demo
	}
	demo := &imageViewDemo{state: state, zoom: 2, pan: gfx.Point{X: 18, Y: -12}}
	demo.view.Pointer = demo.pointer
	demo.view.Scroll = demo.scroll
	imageViewDemos[state] = demo
	return demo
}

func (state *State) paintImageView(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	demo := imageViewDemoFor(state)
	state.drawPage(canvas, theme, size, 520, state.Text("图像视图", "Image View"), func(c ui.C) {
		switch state.exampleID {
		case "actual":
			demo.view.Reset().Bitmap(*state.imageBitmap).Actual()
			image.View(c, state.Text("实际像素", "Actual pixels"), &demo.view).Size(160, 100)
		case "zoom-pan":
			demo.view.Reset().Bitmap(*state.imageBitmap).Zoom(demo.zoom).Pan(demo.pan)
			image.View(c, state.Text("缩放与平移", "Zoom and pan"), &demo.view).FillWidth().Height(240)
		default:
			demo.view.Reset().Bitmap(*state.imageBitmap).Fit()
			image.View(c, state.Text("适配预览", "Fit preview"), &demo.view).FillWidth().Height(240)
		}
	})
}

// pointer drags the zoomed image. Only the zoom example shows it; the fitted
// and actual views leave pointer input to the page.
func (demo *imageViewDemo) pointer(event gfx.PointerEvent) gfx.EventResult {
	if demo.state.exampleID != "zoom-pan" {
		return gfx.EventResult{}
	}
	switch event.Kind {
	case gfx.PointerDown:
		demo.dragging = true
		demo.last = gfx.Point{X: event.X, Y: event.Y}
		return gfx.EventResult{Handled: true}
	case gfx.PointerMove:
		if !demo.dragging && !event.Captured {
			return gfx.EventResult{}
		}
		demo.pan.X += event.X - demo.last.X
		demo.pan.Y += event.Y - demo.last.Y
		demo.last = gfx.Point{X: event.X, Y: event.Y}
		return gfx.EventResult{Handled: true, Repaint: true}
	case gfx.PointerUp:
		demo.dragging = false
		return gfx.EventResult{Handled: true}
	}
	return gfx.EventResult{}
}

// scroll zooms the zoom example between 50% and 400%.
func (demo *imageViewDemo) scroll(event gfx.ScrollEvent) gfx.EventResult {
	if demo.state.exampleID != "zoom-pan" {
		return gfx.EventResult{}
	}
	demo.zoom = min(max(demo.zoom-event.DeltaY*0.01, 0.5), 4)
	return gfx.EventResult{Handled: true, Repaint: true}
}
