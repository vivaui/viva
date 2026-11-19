package componentdemo

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/skeleton"
)

var skeletonComponent = Component{
	ID: "skeleton", Title: "Skeleton", Category: "Feedback", Summary: "无后台计时器的静态加载占位。",
	paint: (*State).paintSkeleton,
	Examples: []Example{
		{ID: "default", Title: "内容占位", Description: "文本行与圆角块表达标题和正文仍在加载；尺寸与不透明度都由页面显式给出。", Snippet: `title := ui.Slot(c).FillWidth().MaxWidth(236).Height(22)
line1 := ui.Slot(c).Height(14)
line2 := ui.Slot(c).WidthPercent(82).Height(14)
line3 := ui.Slot(c).WidthPercent(64).Height(14)
components.BeginSkeleton(canvas, theme, title.Rect()).Paint()
components.BeginSkeleton(canvas, theme, line1.Rect()).Radius(4).Paint()
components.BeginSkeleton(canvas, theme, line2.Rect()).Radius(4).Paint()
components.BeginSkeleton(canvas, theme, line3.Rect()).Radius(4).Paint()`, PageSnippet: `m.title.Reset().Block().Size(236, 22).Opacity(1)
skeleton.View(c, "Title line", m.title)
ui.Column(c).FillWidth().MaxWidth(236).Gap(12).Children(func() {
    m.line1.Reset().Text().Size(236, 14).Radius(4).Opacity(1)
    skeleton.View(c, "Body line 1", m.line1)
    m.line2.Reset().Text().Size(193, 14).Radius(4).Opacity(1)
    skeleton.View(c, "Body line 2", m.line2)
    m.line3.Reset().Text().Size(151, 14).Radius(4).Opacity(1)
    skeleton.View(c, "Body line 3", m.line3)
})`, PageSnippetZh: `m.title.Reset().Block().Size(236, 22).Opacity(1)
skeleton.View(c, "标题行", m.title)
ui.Column(c).FillWidth().MaxWidth(236).Gap(12).Children(func() {
    m.line1.Reset().Text().Size(236, 14).Radius(4).Opacity(1)
    skeleton.View(c, "正文行 1", m.line1)
    m.line2.Reset().Text().Size(193, 14).Radius(4).Opacity(1)
    skeleton.View(c, "正文行 2", m.line2)
    m.line3.Reset().Text().Size(151, 14).Radius(4).Opacity(1)
    skeleton.View(c, "正文行 3", m.line3)
})`},
		{ID: "profile", Title: "资料卡", Description: "圆形头像与多行资料占位组合；每种占位都由同一个 New 加上链式选项声明。", Snippet: `avatar := ui.Slot(c).Size(56, 56)
name := ui.Slot(c).Size(172, 16)
handle := ui.Slot(c).Size(118, 12)
body := ui.Slot(c).Height(92)
components.BeginSkeleton(canvas, theme, avatar.Rect()).Circle().Paint()
components.BeginSkeleton(canvas, theme, name.Rect()).Paint()
components.BeginSkeleton(canvas, theme, handle.Rect()).Radius(4).Paint()
components.BeginSkeleton(canvas, theme, body.Rect()).Paint()`, PageSnippet: `m.avatar.Reset().Circle().Size(56, 56).Opacity(1)
m.name.Reset().Block().Size(172, 16).Opacity(1)
m.handle.Reset().Text().Size(118, 12).Opacity(1)
ui.Row(c).Gap(16).Children(func() {
    skeleton.View(c, "Avatar placeholder", m.avatar)
    ui.Column(c).Gap(10).Children(func() {
        skeleton.View(c, "Name placeholder", m.name)
        skeleton.View(c, "Handle placeholder", m.handle)
    })
})
m.body.Reset().Block().Size(236, 92).Opacity(1)
skeleton.View(c, "Body placeholder", m.body)`, PageSnippetZh: `m.avatar.Reset().Circle().Size(56, 56).Opacity(1)
m.name.Reset().Block().Size(172, 16).Opacity(1)
m.handle.Reset().Text().Size(118, 12).Opacity(1)
ui.Row(c).Gap(16).Children(func() {
    skeleton.View(c, "头像占位", m.avatar)
    ui.Column(c).Gap(10).Children(func() {
        skeleton.View(c, "姓名占位", m.name)
        skeleton.View(c, "账号占位", m.handle)
    })
})
m.body.Reset().Block().Size(236, 92).Opacity(1)
skeleton.View(c, "正文占位", m.body)`},
		{ID: "opacity", Title: "显式不透明度", Description: "四行占位使用不同的静态不透明度；组件自身不启动计时器，也不伪造动画相位。", Snippet: `rows := [4]ui.Box{}
for index := range rows {
    rows[index] = ui.Slot(c).Size(236, 20)
}
for index, opacity := range [...]float32{1, 0.75, 0.5, 0.75} {
    color := theme.Muted
    color.Alpha *= opacity
    canvas.Rounded(rows[index].Rect(), 5, color)
}`, PageSnippet: `m.line1.Reset().Text().Size(236, 20).Radius(5).Opacity(1)
skeleton.View(c, "Line at 100%", m.line1)
m.line2.Reset().Text().Size(236, 20).Radius(5).Opacity(0.75)
skeleton.View(c, "Line at 75%", m.line2)
m.line3.Reset().Text().Size(236, 20).Radius(5).Opacity(0.5)
skeleton.View(c, "Line at 50%", m.line3)
m.line4.Reset().Text().Size(236, 20).Radius(5).Opacity(0.75)
skeleton.View(c, "Line at 75%", m.line4)`, PageSnippetZh: `m.line1.Reset().Text().Size(236, 20).Radius(5).Opacity(1)
skeleton.View(c, "100% 不透明", m.line1)
m.line2.Reset().Text().Size(236, 20).Radius(5).Opacity(0.75)
skeleton.View(c, "75% 不透明", m.line2)
m.line3.Reset().Text().Size(236, 20).Radius(5).Opacity(0.5)
skeleton.View(c, "50% 不透明", m.line3)
m.line4.Reset().Text().Size(236, 20).Radius(5).Opacity(0.75)
skeleton.View(c, "75% 不透明", m.line4)`},
	},
}

// skeletonDemo keeps one placeholder per role. The view's draw function is made
// once, so reconfiguring and declaring the placeholders each paint allocates
// nothing.
type skeletonDemo struct {
	title, line1, line2, line3 skeleton.State
	avatar, name, handle, body skeleton.State
	opacity                    [4]skeleton.State
}

// skeletonDemos is keyed by State because the constrained demo model has no
// field for this example; a State is painted on one thread at a time.
var skeletonDemos = map[*State]*skeletonDemo{}

func skeletonDemoFor(state *State) *skeletonDemo {
	if demo := skeletonDemos[state]; demo != nil {
		return demo
	}
	demo := new(skeletonDemo)
	skeletonDemos[state] = demo
	return demo
}

func (state *State) paintSkeleton(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	demo := skeletonDemoFor(state)
	state.drawPage(canvas, theme, size, 520, state.Text("Skeleton · 加载占位", "Skeleton · Loading placeholder"), func(c ui.C) {
		switch state.exampleID {
		case "profile":
			demo.avatar.Reset().Circle().Size(56, 56).Opacity(1)
			demo.name.Reset().Block().Size(172, 16).Opacity(1)
			demo.handle.Reset().Text().Size(118, 12).Opacity(1)
			demo.body.Reset().Block().Size(236, 92).Opacity(1)
			ui.Row(c).Gap(16).Children(func() {
				skeleton.View(c, state.Text("头像占位", "Avatar placeholder"), &demo.avatar)
				ui.Column(c).Gap(10).Children(func() {
					skeleton.View(c, state.Text("姓名占位", "Name placeholder"), &demo.name)
					skeleton.View(c, state.Text("账号占位", "Handle placeholder"), &demo.handle)
				})
			})
			skeleton.View(c, state.Text("正文占位", "Body placeholder"), &demo.body)
		case "opacity":
			for index, opacity := range [...]float32{1, 0.75, 0.5, 0.75} {
				demo.opacity[index].Reset().Text().Size(236, 20).Radius(5).Opacity(opacity)
			}
			labels := [...]string{
				state.Text("100% 不透明", "Line at 100%"),
				state.Text("75% 不透明", "Line at 75%"),
				state.Text("50% 不透明", "Line at 50%"),
				state.Text("75% 不透明", "Line at 75%"),
			}
			ui.Column(c).Gap(22).Children(func() {
				for index := range demo.opacity {
					skeleton.View(c, labels[index], &demo.opacity[index])
				}
			})
		default:
			demo.title.Reset().Block().Size(236, 22).Opacity(1)
			demo.line1.Reset().Text().Size(236, 14).Radius(4).Opacity(1)
			demo.line2.Reset().Text().Size(193, 14).Radius(4).Opacity(1)
			demo.line3.Reset().Text().Size(151, 14).Radius(4).Opacity(1)
			skeleton.View(c, state.Text("标题行", "Title line"), &demo.title)
			ui.Column(c).FillWidth().MaxWidth(236).Gap(12).Children(func() {
				skeleton.View(c, state.Text("正文行 1", "Body line 1"), &demo.line1)
				skeleton.View(c, state.Text("正文行 2", "Body line 2"), &demo.line2)
				skeleton.View(c, state.Text("正文行 3", "Body line 3"), &demo.line3)
			})
		}
	})
}
