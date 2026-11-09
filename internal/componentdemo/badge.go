package componentdemo

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var badgeComponent = Component{
	ID: "badge", Title: "Badge", Category: "Display", Summary: "随文本自然伸展的状态标签。",
	paint: (*State).paintBadge,
	Examples: []Example{
		{ID: "default", Title: "自然宽度", Description: "短标签按内容测量宽度，而不是平分容器。", Snippet: `labels := []string{"New", "99+", "Published"}
variants := []components.BadgeVariant{
    components.BadgeDefault, components.BadgeSecondary, components.BadgeOutline,
}
x := bounds.X
for index, label := range labels {
    width, height := components.BadgeExtent(canvas, theme, label)
    badge := gfx.Rect{X: x, Width: width, Height: height}
    components.PaintBadge(canvas, theme, badge, label, variants[index])
    x += width + 12
}`},
		{ID: "variants", Title: "六种变体", Description: "默认、次要、危险、描边、幽灵与链接外观；均为展示标签。", Snippet: `labels := []string{"Default", "Secondary", "Destructive", "Outline", "Ghost", "Link"}
variants := []components.BadgeVariant{
    components.BadgeDefault, components.BadgeSecondary, components.BadgeDestructive,
    components.BadgeOutline, components.BadgeGhost, components.BadgeLink,
}
x := bounds.X
for index, label := range labels {
    width, height := components.BadgeExtent(canvas, theme, label)
    badge := gfx.Rect{X: x, Width: width, Height: height}
    components.PaintBadge(canvas, theme, badge, label, variants[index])
    x += width + 12
}`},
		{ID: "long-text", Title: "长中文", Description: "完整展示长中文及中英混排标签，空间不足时另起一行。", Snippet: `// BadgeExtent keeps each badge as wide as its label, so a grid can wrap them.
labels := []string{"In review", "This is a long status label", "Viva native component · Published"}
variants := []components.BadgeVariant{
    components.BadgeSecondary, components.BadgeOutline, components.BadgeDefault,
}
x := bounds.X
for index, label := range labels {
    width, height := components.BadgeExtent(canvas, theme, label)
    badge := gfx.Rect{X: x, Width: width, Height: height}
    components.PaintBadge(canvas, theme, badge, label, variants[index])
    x += width + 12
}`},
	},
}

func (state *State) paintBadge(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	labels := []string{"New", "99+", state.Text("已发布", "Published")}
	variants := []components.BadgeVariant{components.BadgeDefault, components.BadgeSecondary, components.BadgeOutline}
	switch state.exampleID {
	case "variants":
		labels = []string{"Default", "Secondary", "Destructive", "Outline", "Ghost", "Link"}
		variants = []components.BadgeVariant{
			components.BadgeDefault, components.BadgeSecondary, components.BadgeDestructive,
			components.BadgeOutline, components.BadgeGhost, components.BadgeLink,
		}
	case "long-text":
		labels = []string{
			state.Text("审核中", "In review"),
			state.Text("这是一条长中文状态标签", "This is a long status label"),
			state.Text("Viva 原生组件 · 已发布", "Viva native component · Published"),
		}
		variants = []components.BadgeVariant{components.BadgeSecondary, components.BadgeOutline, components.BadgeDefault}
	}
	// A badge is as wide as its label. The grid's cell is as wide as the widest
	// one, so the badges wrap onto new rows when the stage is too narrow for them
	// side by side; each is a Slot of its natural size that the component paints.
	cell := float32(0)
	for _, label := range labels {
		width, _ := components.BadgeExtent(canvas, theme, label)
		cell = max(cell, width)
	}
	var slots [6]ui.Box
	state.page(canvas, theme, size, 560, state.Text("Badge · 内容决定宽度", "Badge · Width follows content"), func(f *ui.Frame) {
		f.Grid(cell).Gap(12).Children(func() {
			for index, label := range labels {
				width, height := components.BadgeExtent(canvas, theme, label)
				slots[index] = f.Slot().Size(width, height).Self(ui.Start)
			}
		})
		f.Text(state.Text("仅展示状态，不声明点击区域。", "Display only; no click target is declared.")).FontSize(theme.Metrics.FontSizeS).Muted()
	})
	for index, label := range labels {
		bounds := slots[index].Rect()
		components.PaintBadge(canvas, theme, gfx.Rect{X: bounds.X, Y: bounds.Y}, label, variants[index])
	}
}
