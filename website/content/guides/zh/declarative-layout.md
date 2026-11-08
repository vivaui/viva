---
title: 声明式布局
summary: 用行、列、内边距和伸缩来描述页面，而不是算坐标；由 flex 包完成布局。
order: 7
group: guides
---

手工切矩形（见“绘制与布局”）是 Viva 最底层的做法，适合自定义控件。对于由普通部件组成的页面（页眉、表单、列表行、页脚），它要做很多算术，标签变长时算术还会出错。`flex` 包按 CSS flexbox 的方式，根据描述来布局页面。

```go
view.Draw(canvas, theme, bounds, func(f *flex.Frame) {
	f.Column().Pad(24).Gap(12).Children(func() {
		f.Text("标题").FontSize(18).Bold()
		f.Row().Gap(8).Justify(flex.End).Children(func() {
			f.Button(cancel, "取消").Outline()
			if f.Button(save, "保存").Clicked() {
				model.save()
			}
		})
	})
})
```

## 模型

- `Row` 从左到右排列子元素，`Column` 从上到下，`Stack` 相互叠放。
- 子元素默认和内容一样大，或者按 `Width`、`Height`、`Size` 指定。`FillWidth`、`FillHeight`、`Fill` 占满容器的交叉方向，或者平分沿着容器方向剩下的空间。
- `Grow(n)` 把容器方向的剩余空间按比例分给会伸缩的子元素。`Spacer()` 是会伸缩的空元素。
- `Pad` 和 `Margin` 接收一个数字，更多情况用 `PadX`、`PadY`、`PadSides`。`Gap` 是子元素之间的间距。
- 交叉方向上子元素默认拉伸；`Items(flex.Center)` 居中，`Self` 只对一个子元素生效。沿容器方向用 `Justify(flex.Center)`、`flex.End`、`flex.Between` 摆放剩余空间。`Center()` 两个方向都居中。
- `Basis(px)` 是在分配剩余空间之前元素的起始尺寸，`WidthPercent`、`HeightPercent` 取容器的一个比例。`MinWidth`、`MaxWidth`、`MinHeight`、`MaxHeight` 限定尺寸；被上限卡住的元素吃不下的空间会分给其他元素（和 CSS 一样），所以 `Grow(1).MaxWidth(470)` 旁边放一个 `Spacer()`，后面的元素仍然贴着远端。
- `Items(flex.Baseline)` 让一行里的子元素按第一行文字的基线对齐，大标题、小说明和按钮可以落在同一条基线上。`Text` 会先于固定尺寸的兄弟元素收缩并裁剪。
- `Background`、`Radius`、`Border`、`Shadow`、`Clip` 为容器设置样式。在 `Input`、`Select`、`Button` 等控件上，`Radius` 和 `FontSize` 为该控件设置它自己的圆角和字号；`Button` 上的 `Height` 就是按钮画出来的高度。
- `Paragraph(text, maxLines)` 是会按所得宽度换行的文字，所以标签变长会让这一行变高，并把后面的内容往下推。`LineHeight` 设置行高。
- `Grid(minCell)` 按宽度放下尽可能多的等宽列，并换到新的行。
- `Scroll(offset)` 是比自己更高的、带裁剪的列。偏移量由你持有（滚轮事件到 `Config.OnScroll`）；`End` 之后 `Content()` 是子元素的总高度，可据此限制偏移量。

控件就是 `components` 里的那些，外观完全一致：`Button`（以及 `Outline`、`Ghost`、`Secondary`、`Destructive`、`Small` 等变体）、`Checkbox`、`Switch`、`Icon`、`Divider`。`Clicked()` 在声明控件的地方读取：它只在递送这次点击的那一遍里为真。任何元素都可以用 `Hit(id, label)` 成为指针目标；它在指针悬停、按下和获得焦点时的样子用 `Hover`、`Pressed` 和 `Focus` 声明（见下文）。

## 不起名字的页面：flex.Screen

`flex.Screen` 是同一套布局，再替你做掉页面的簿记。Frame 的控件要用你在 `ui.Tree` 里声明的标识，点击结果要你自己写回；Screen 的控件接受它要修改的值，由 Screen 为它们命名：

```go
func (m *model) page(s flex.Screen) {
	s.Column().Fill().Pad(24).Gap(12).Children(func() {
		s.Field(&m.name, "Name", "Your name")
		s.Switch(&m.sync, "Sync over SSH")
		s.Choice(&m.theme, themes)
		if s.Button("Save").Clicked() {
			m.save()
		}
	})
}

flex.Run(app.Config{Title: "Settings", Width: 480, Height: 320}, m.page)
```

点击会在递送它的那一遍绘制里翻转 `m.sync`、设置 `m.theme`：在控件第一次被询问（`Clicked`、`Changed`）时写入，没有被询问就在页面结束时写入，所以写在控件之后的 `Disabled` 依然有效。`Field` 不需要 `Config.TextInput`：Screen 的 `Config` 把它声明的每个 Field 的编辑器交给平台，其他获得焦点的字段交给配置里原有的提供者。`Popover` 和 `Modal` 在一个 `*bool` 为真时打开，被关闭时把它设为 false（见[对话框、菜单与弹出层](/zh/docs/overlays/)）。`Box.NativeView` 把一个元素的位置交给系统网页引擎的页面。

Screen 为控件命名的方式保证一个名字不会在用户不知情时转到另一个控件上：

- 绑定了值的控件以该值的地址命名。在密码框上方出现的输入框不会拿走密码框的焦点和输入；过滤列表时，每个复选框的焦点留在它所在的行。要绑定模型里的字段，而不是副本：局部副本在每次绘制时地址都不同，它的点击会丢失（Screen 会通过 `diagnostics` 报告）。
- 没有绑定值的控件（`Button`）按它在所在范围内的按钮里的顺序编号。用户按住它（按下或它有焦点）期间，如果某次绘制里它的编号被另一个按钮占了，这次按下不会递送，焦点也会释放。
- 在 `s.Key(item.ID)` 或 `s.KeyIndex(id)` 里，控件只在这一行内部编号，所以带按钮的列表行要加 Key：行出现、消失或移动时，行里的按钮跟着它走。行消失两遍绘制之后它的 key 编号会被复用，所以列表的开销只取决于同时显示的行数。同一范围里两个没有 Key 的同名按钮会被报告，因为没有办法区分它们。

`examples/guides/screen` 的 fleet 页面是一个完整的程序：一个过滤控件、一个可滚动的主机列表（每行一个绑定到该主机的复选框）、一个进度条和一个按钮，没有声明任何标识。

<!-- example: screen/main.go -->
```go
// Fleet check with flex.Screen: the page examples/guides/fleet declares with a
// Frame and a tree of identities, written without naming a control.
//
// Each checkbox writes into the host it shows, and is named by that host's field,
// so filtering the list moves no focus or click from one host to another.
package main

import (
	"log"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/flex"
)

type host struct {
	name    string
	addr    string
	item    string
	handled bool
}

type model struct {
	hosts  []host
	filter int
	scans  int
}

var filters = []string{"All", "Open", "Handled"}

func newModel() *model {
	return &model{hosts: []host{
		{name: "edge-01", addr: "10.0.4.11", item: "Disable password auth"},
		{name: "edge-02", addr: "10.0.4.12", item: "Rotate host key"},
		{name: "build-01", addr: "10.0.8.3", item: "Pin to ed25519"},
		{name: "build-02", addr: "10.0.8.4", item: "Restrict agent forwarding", handled: true},
		{name: "db-primary", addr: "10.0.9.2", item: "Close port 22 to the world"},
		{name: "db-replica", addr: "10.0.9.3", item: "Enable key-only login", handled: true},
		{name: "jump-01", addr: "10.0.1.5", item: "Set a login grace time"},
		{name: "nas-01", addr: "10.0.12.7", item: "Upgrade OpenSSH"},
	}}
}

func (m *model) matches(h *host) bool {
	switch filters[m.filter] {
	case "Open":
		return !h.handled
	case "Handled":
		return h.handled
	}
	return true
}

func (m *model) handled() int {
	n := 0
	for i := range m.hosts {
		if m.hosts[i].handled {
			n++
		}
	}
	return n
}

func (m *model) page(s flex.Screen) {
	s.Column().Fill().Pad(22).Gap(14).Children(func() {
		s.Text("Fleet check").FontSize(26).Bold()
		s.Text("SSH posture across the hosts this workstation reaches.").FontSize(13).Muted()
		s.Choice(&m.filter, filters)

		// The list takes the room left over, so the footer stays on screen.
		s.ScrollArea("hosts").Grow(1).Gap(8).Children(func() {
			for i := range m.hosts {
				if h := &m.hosts[i]; m.matches(h) {
					m.row(s, h)
				}
			}
		})

		handled, total := m.handled(), len(m.hosts)
		s.Progress(float32(handled) / float32(max(total, 1))).FillWidth()
		s.Row().FillWidth().Gap(10).Items(flex.Center).Children(func() {
			s.Textf("%d of %d handled", handled, total).FontSize(12).Muted().Mono()
			s.Spacer()
			s.Textf("scanned %dx", m.scans).FontSize(12).Muted().Mono()
			if s.Button("Scan again").Clicked() {
				m.scans++
			}
		})
	})
}

func (m *model) row(s flex.Screen, h *host) {
	theme := s.Theme()
	s.Row().FillWidth().Pad(12).Gap(12).Radius(10).Items(flex.Center).Background(theme.Card).Children(func() {
		if h.handled {
			s.Text("ok").FontSize(11).Color(theme.Success)
		} else {
			s.Text("open").FontSize(11).Color(theme.Danger)
		}
		s.Column().Grow(1).Gap(2).Children(func() {
			s.Row().Gap(8).Items(flex.Center).Children(func() {
				s.Text(h.name).FontSize(14).Bold()
				s.Text(h.addr).FontSize(12).Muted().Mono()
			})
			item := s.Text(h.item).FontSize(12)
			if h.handled {
				item.Muted()
			}
		})
		// The checkbox writes into the host; the bar above derives itself from the
		// hosts on the next paint.
		s.Checkbox(&h.handled, "").Label(h.name)
	})
}

func main() {
	if err := flex.Run(app.Config{Title: "Viva fleet", Width: 460, Height: 560}, newModel().page); err != nil {
		log.Fatal(err)
	}
}
```

`s.Raw()` 是底下的 Frame，用于 Screen 没有转发的东西（`Textarea`、`InputGroup`、各个 `Base` 控件）；`s.ID("word")` 给在那里声明的控件起一个你自己的名字，测试和无头驱动可以按它定位。画在窗口一部分（一个窗格）里的页面，用 `flex.NewScreen` 并传入窗口的 Tree，再调用 `screen.Draw(canvas, bounds, body)`。按标识集中分派事件的窗口级处理器，以及自定义控件，继续使用 Frame。

`examples/guides/screen` 的 fleet 页面（一个选择控件、一个包含八行的滚动区域、每行一个复选框、一个进度条和一个按钮）在 Apple M2 Pro 上声明、布局并绘制一次需要 9.8 µs，同一页面用 Frame 写是 9.0 µs，预热后都不分配内存；一千个带 key 的行需要 0.68 ms，Frame 是 0.57 ms（`go test ./flex -bench 'Fleet$|Keyed1000$'`）。

## 状态与效果

有标识的元素在指针悬停、被按住、键盘聚焦时会呈现不同的样子。每个状态是一个 `Look`：只列出发生变化的部分的值，用与元素本身相同的词来构造（`Background`、`Border`、`Color`、`Ring`、`Shadow`、`Opacity`）。

```go
f.Row().Pad(8).Radius(8).Hit(id, "Open").Transition().
	Hover(flex.Look{}.Background(theme.Hover)).
	Pressed(flex.Look{}.Background(theme.Muted)).
	Focus(flex.Look{}.Ring(3, theme.Ring))
```

`Focus` 在焦点应当可见时生效（由按键移动而来，点击不算）；`Hit` 默认可获得焦点，除非用 `HitWith` 指定 `SkipFocus`。几个 Look 按 Focus、Hover、Pressed 的顺序逐字段合并，所以既聚焦又悬停的行会同时显示前者的光环和后者的底色。`Transition()` 按主题的动效在它们之间渐变（`TransitionWith` 可指定其他动效）；没有变化时不会安排任何帧。`StateFrom(row)` 让一个元素跟随另一个元素的状态，行内的内容就能随行变化：`f.Icon(edit).Opacity(0).StateFrom(row).Hover(flex.Look{}.Opacity(1))` 是一个在所在行悬停时才出现的图标。

另有四种不依赖状态的效果。`Ring(width, color)` 在元素外侧画一圈线且不占空间，焦点环就是这样画的。`Opacity(v)` 让元素及其内部的一切（包括控件）淡化；取 0 时仍参与布局、仍可点击；`Image` 不会被淡化。`Gradient(angle, from, to)` 和 `GradientStops(angle, stops...)` 沿圆角用线性渐变填充元素，画在 `Background` 之上、`Border` 之下；角度与 CSS 相同，以向上为 0 顺时针计，`flex.ToRight`、`flex.ToBottomRight` 等给常用方向命名。

## 和手工绘制混用

`Slot()` 为你自己绘制的内容预留一块矩形，`Box.Rect()` 告诉你它最后落在哪里。需要在绘制前拿到矩形时，用 `view.Begin(...)` 和 `f.End()` 代替 `Draw`：先声明，调用 `End`，再往槽位里绘制。自定义控件就是这样放进已布局的页面里的。

## 开销

声明、测量、排布并绘制一个约六十个元素的页面大约需要 8 微秒，`View` 见过最大页面之后不再分配内存。为每个每帧都绘制的页面在你的状态里保存一个 `View`；零值即可使用。传给 `Children` 的函数立即调用且不会被保存，所以也没有开销。

单行 `Text` 不换行：放不下的行会被裁剪，并且先于固定尺寸的兄弟元素收缩。需要换行的文字用 `Paragraph`。

## 创建项目

`viva init my-app` 会写出一个能直接运行的项目：用 `flex` 布局的窗口、无需窗口即可驱动它的测试、`viva.toml`，以及 `AGENTS.md`，一页纸的布局、标识、点击和测试指南，供编码助手在修改界面前阅读。之后 `viva dev .` 以热重载方式运行。

<!-- example: layout/main.go -->
```go
package main

import (
	"log"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/ui"
)

var (
	tree = ui.Tree{}
	page = tree.Feature("layout")

	save   = tree.ID(page, "save")
	cancel = tree.ID(page, "cancel")
)

type model struct {
	saved bool
	notes bool
	view  flex.View
}

func newConfig(m *model) app.Config {
	theme := ui.LightTheme()
	config := app.Config{Tree: &tree, Title: "Layout", Width: 420, Height: 220}

	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		canvas.Fill(ui.Rect{Width: size.Width, Height: size.Height}, theme.Background)
		bounds := ui.Rect{Width: size.Width, Height: size.Height}

		m.view.Draw(canvas, theme, bounds, func(f *flex.Frame) {
			f.Column().Fill().Pad(24).Gap(12).Children(func() {
				f.Text("Deploy settings").FontSize(18).Bold()
				f.Text("Where the build goes when it passes.").Muted()
				f.Divider()
				f.Row().Items(flex.Center).Gap(8).Children(func() {
					f.Text("Notify the team")
					f.Spacer()
					f.Switch(tree.ID(page, "notes"), m.notes).Label("Notify the team")
				})
				f.Spacer()
				f.Row().Gap(8).Justify(flex.End).Children(func() {
					f.Button(cancel, "Cancel").Outline()
					if f.Button(save, "Save").Clicked() {
						m.saved = true
					}
				})
			})
		})
	}
	return config
}

func main() {
	m := &model{}
	var canvas ui.Canvas
	if err := app.RunCanvas(newConfig(m), &canvas); err != nil {
		log.Fatal(err)
	}
}
```
