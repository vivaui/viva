package componentdemo

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
)

var imageViewComponent = Component{
	ID: "image-view", Title: "Image View", Category: "Media", Summary: "按视口适配或按实际像素显示位图，与显示缩放无关。",
	initialize: func(state *State) {
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
	},
	paint: func(state *State, canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
		var zoom float32
		if state.exampleID == "actual" {
			zoom = 1
		}
		components.PaintImageView(canvas, theme, gfx.Rect{X: 24, Y: 24, Width: max(0, size.Width-48), Height: max(0, size.Height-48)}, components.ImageViewOptions{Bitmap: *state.imageBitmap, Zoom: zoom})
	},
	Examples: []Example{
		{ID: "fit", Title: "适配视口", Description: "应用提供已解码且有界的像素，组件只负责绘制。", Height: 260, Snippet: `components.PaintImageView(canvas, theme, bounds, components.ImageViewOptions{Bitmap: bitmap})`},
		{ID: "actual", Title: "一个图像像素对应一个设备像素", Description: "Zoom 1 在分数 DPI 缩放下保持实际像素尺寸。", Height: 260, Snippet: `components.PaintImageView(canvas, theme, bounds, components.ImageViewOptions{Bitmap: bitmap, Zoom: 1})`},
	},
}
