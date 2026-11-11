package componentdemo

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var skeletonComponent = Component{
	ID: "skeleton", Title: "Skeleton", Category: "Feedback", Summary: "无后台计时器的静态加载占位。",
	paint: (*State).paintSkeleton,
	Examples: []Example{
		{ID: "default", Title: "内容占位", Description: "使用圆角块表达标题与正文仍在加载。", Snippet: `title := ui.Slot(c).FillWidth().MaxWidth(236).Height(22)
line1 := ui.Slot(c).Height(14)
line2 := ui.Slot(c).WidthPercent(82).Height(14)
line3 := ui.Slot(c).WidthPercent(64).Height(14)
components.BeginSkeleton(canvas, theme, title.Rect()).Paint()
components.BeginSkeleton(canvas, theme, line1.Rect()).Radius(4).Paint()
components.BeginSkeleton(canvas, theme, line2.Rect()).Radius(4).Paint()
components.BeginSkeleton(canvas, theme, line3.Rect()).Radius(4).Paint()`},
		{ID: "profile", Title: "资料卡", Description: "圆形头像与多行资料占位组合。", Snippet: `avatar := ui.Slot(c).Size(56, 56)
name := ui.Slot(c).Size(172, 16)
handle := ui.Slot(c).Size(118, 12)
body := ui.Slot(c).Height(92)
components.BeginSkeleton(canvas, theme, avatar.Rect()).Circle().Paint()
components.BeginSkeleton(canvas, theme, name.Rect()).Paint()
components.BeginSkeleton(canvas, theme, handle.Rect()).Radius(4).Paint()
components.BeginSkeleton(canvas, theme, body.Rect()).Paint()`},
		{ID: "phases", Title: "显式相位", Description: "由调用方提供固定动画相位；组件自身不启动计时器。", Snippet: `rows := [4]ui.Box{}
for index := range rows {
    rows[index] = ui.Slot(c).Height(20)
}
for index, phase := range [...]float32{0, 0.25, 0.50, 0.75} {
    components.BeginSkeleton(canvas, theme, rows[index].Rect()).Radius(5).Phase(phase).Paint()
}`},
	},
}

// skeletonBlock is one placeholder: the slot ui gave it and how the component
// paints it.
type skeletonBlock struct {
	slot   ui.Box
	circle bool
	radius float32
	phased bool
	phase  float32
}

func (state *State) paintSkeleton(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	var blocks [6]skeletonBlock
	count := 0
	add := func(c ui.C, block skeletonBlock) ui.Box {
		block.slot = ui.Slot(c)
		blocks[count] = block
		count++
		return block.slot
	}
	state.drawPage(canvas, theme, size, 520, state.Text("Skeleton · 加载占位", "Skeleton · Loading placeholder"), func(c ui.C) {
		switch state.exampleID {
		case "profile":
			ui.Row(c).Gap(16).Children(func() {
				add(c, skeletonBlock{circle: true}).Size(56, 56)
				ui.Column(c).Gap(10).Children(func() {
					add(c, skeletonBlock{}).Size(172, 16)
					add(c, skeletonBlock{radius: 4}).Size(118, 12)
				})
			})
			add(c, skeletonBlock{}).Height(92)
			caption(c, state.Text("头像、身份与内容区域分别占位。", "Avatar, identity, and content areas each have their own placeholder."))
		case "phases":
			ui.Column(c).Gap(22).Children(func() {
				for _, phase := range [...]float32{0, 0.25, 0.50, 0.75} {
					add(c, skeletonBlock{radius: 5, phased: true, phase: phase}).Height(20)
				}
			})
			caption(c, state.Text("四个确定性相位；动画调度仍由应用显式负责。", "Four deterministic phases; the application still schedules the animation."))
		default:
			add(c, skeletonBlock{}).FillWidth().MaxWidth(236).Height(22)
			ui.Column(c).Gap(12).Children(func() {
				add(c, skeletonBlock{radius: 4}).Height(14)
				add(c, skeletonBlock{radius: 4}).WidthPercent(82).Height(14)
				add(c, skeletonBlock{radius: 4}).WidthPercent(64).Height(14)
			})
			caption(c, state.Text("静态占位不会自行请求下一帧。", "A static placeholder never requests the next frame."))
		}
	})
	for _, block := range blocks[:count] {
		skeleton := components.BeginSkeleton(canvas, theme, block.slot.Rect())
		if block.circle {
			skeleton = skeleton.Circle()
		}
		if block.radius != 0 {
			skeleton = skeleton.Radius(block.radius)
		}
		if block.phased {
			skeleton = skeleton.Phase(block.phase)
		}
		skeleton.Paint()
	}
}
