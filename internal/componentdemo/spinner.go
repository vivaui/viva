package componentdemo

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var spinnerComponent = Component{
	ID: "spinner", Title: "Spinner", Category: "Feedback", Summary: "由调用方推进相位的轻量加载指示器。",
	paint: (*State).paintSpinner,
	Examples: []Example{
		{ID: "default", Title: "加载状态", Description: "一个官方尺寸的静态相位及状态文字。", Snippet: `// The caller chooses the phase; the spinner schedules no frames.
icon := f.Slot().Size(24, 24)
components.PaintSpinner(canvas, theme, icon.Rect(), 2)
f.Text("Syncing workspace…").FontSize(theme.Metrics.FontSizeM).Weight(gfx.WeightMedium)`},
		{ID: "phases", Title: "八个相位", Description: "同屏展示全部确定性相位，不启动动画循环。", Snippet: `for index := range 8 {
    cell := f.Slot().Size(32, 32)
    components.PaintSpinner(canvas, theme, cell.Rect(), components.SpinnerPhase(index))
}`},
		{ID: "sizes", Title: "约束尺寸", Description: "指示器在不同边界中居中并保持比例。", Snippet: `for index, side := range [...]float32{16, 32, 48} {
    box := f.Slot().Size(side, side)
    canvas.RoundedBorder(box.Rect(), 5, 1, theme.Border, theme.Background)
    components.PaintSpinner(canvas, theme, box.Rect(), components.SpinnerPhase(index*2))
}`},
	},
}

// The demo paints the component, not ui.Spinner, because its point is the phase the
// caller chooses; ui.Spinner turns by itself.
func (state *State) paintSpinner(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	var slots [8]ui.Box
	state.page(canvas, theme, size, 500, state.Text("Spinner · 显式帧", "Spinner · Explicit frames"), func(f *ui.Frame) {
		switch state.exampleID {
		case "phases":
			for row := range 2 {
				f.Row().Gap(24).Children(func() {
					for column := range 4 {
						slots[row*4+column] = f.Slot().Size(32, 32)
					}
				})
			}
			caption(f, state.Text("每个相位均由调用方选择。", "Every phase is chosen by the caller."))
		case "sizes":
			f.Row().Gap(40).Items(ui.Center).Children(func() {
				slots[0] = f.Slot().Size(16, 16)
				slots[1] = f.Slot().Size(32, 32)
				slots[2] = f.Slot().Size(48, 48)
			})
			caption(f, state.Text("图标最大为 16px，在更大边界内居中。", "The icon is at most 16px and stays centered in larger bounds."))
		default:
			f.Row().Gap(14).Items(ui.Center).Children(func() {
				slots[0] = f.Slot().Size(24, 24)
				f.Text(state.Text("正在同步工作区…", "Syncing workspace…")).FontSize(theme.Metrics.FontSizeM).Weight(gfx.WeightMedium)
			})
			caption(f, state.Text("组件不启动计时器，也不自行请求重绘。", "The component starts no timer and requests no repaint."))
		}
	})

	switch state.exampleID {
	case "phases":
		for index := range slots {
			components.PaintSpinner(canvas, theme, slots[index].Rect(), components.SpinnerPhase(index))
		}
	case "sizes":
		for index, slot := range slots[:3] {
			canvas.RoundedBorder(slot.Rect(), 5, 1, theme.Border, theme.Background)
			components.PaintSpinner(canvas, theme, slot.Rect(), components.SpinnerPhase(index*2))
		}
	default:
		components.PaintSpinner(canvas, theme, slots[0].Rect(), 2)
	}
}
