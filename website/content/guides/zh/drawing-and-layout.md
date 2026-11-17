---
title: 绘制与布局
summary: 在页面的元素里自绘，以及整窗画布：坐标、行、列和网格，矩形、文字、图标和路径。
order: 6
group: guides
---

屏幕上的一切都是画进 `gfx.Canvas` 的。大多数应用不会直接为自己的控件调用它：它们用 `ui` 包声明一个页面（见[声明式布局](declarative-layout)），由页面排布元素并绘制控件，只有在 `ui` 里没有合适的东西时才自己画。自己画有两种方式。

## 在页面里绘制

给页面中的元素挂一个 `Draw`（在它的背景和边框之后、子元素之前执行）或 `DrawOver`（在子元素之后执行）。元素放在哪里、有多大由页面决定，回调在 `d.Rect` 里用画布、主题以及元素的悬停、按下和焦点状态来画。用 `Hit` 和一个名字声明这个元素，指针和辅助技术就能像对待控件一样触达它。

<!-- example: paintinpage/main.go -->
```go
package main

import (
	"log"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

type model struct {
	samples []float32 // 0 to 1
}

// page declares a chart's frame and the button that feeds it. The page lays the
// chart's element out; what is drawn inside it is the chart's own.
func (m *model) page(c ui.C) {
	theme := c.Theme()
	ui.Column(c).Fill().Pad(24).Gap(16).Children(func() {
		ui.Text(c, "Samples").FontSize(20).Bold()
		ui.Row(c).FillWidth().Height(120).Radius(8).Background(theme.Card).Border(1, theme.Border).
			Hit(ui.ID(c), "Samples chart").
			Draw(m.bars).
			DrawOver(func(d *ui.DrawContext) {
				if d.State.Hovered {
					d.Canvas.Fill(d.Rect, d.Fade(theme.Foreground.WithAlpha(0.06)))
				}
			})
		if ui.Button(c, "Add a sample").Clicked() {
			m.samples = append(m.samples, float32(len(m.samples)%7+1)/8)
		}
	})
}

// bars paints one bar for each sample in d.Rect, the room the page gave the element.
// It only paints: it may run more than once a frame and declares no elements.
func (m *model) bars(d *ui.DrawContext) {
	const gap = 6
	if len(m.samples) == 0 {
		return
	}
	width := (d.Rect.Width - gap*float32(len(m.samples)+1)) / float32(len(m.samples))
	for i, value := range m.samples {
		height := d.Rect.Height * 0.8 * value
		d.Canvas.Fill(ui.Rect{
			X:     d.Rect.X + gap + float32(i)*(width+gap),
			Y:     d.Rect.Y + d.Rect.Height - height - 8,
			Width: width, Height: height,
		}, d.Fade(d.Theme.Primary))
	}
}

func main() {
	m := &model{samples: []float32{0.3, 0.6, 0.45}}
	if err := ui.Run(app.Config{Title: "Samples", Width: 480, Height: 300}, m.page); err != nil {
		log.Fatal(err)
	}
}
```

回调只负责绘制：一帧里可能运行多次，并且不能声明元素。区域还要接收键盘、指针或文本时，用带 `Handlers` 的 `ui.Control` 代替 `Hit`；它的 `Parts` 给你在绘制时声明的内部区域命名。

## 绘制整个窗口

没有页面的应用在 `Paint` 回调里画出一切：没有需要构建的控件树，也没有样式表，你计算矩形，然后让画布填充它们、在里面绘制文字，或者把它们声明为可交互。矩形要自己切分，每个命中区要自己命名，每个控件的状态（展开的菜单、滚动位置）也要自己保管，而这些 `ui` 对它已有的控件都会替你做，所以只在 `ui` 里没有合适的东西时才选这条路。下面的各节讲的就是这条路；画布能画什么，在页面的 `Draw` 里同样适用。

## 坐标

位置使用逻辑像素，从窗口左上角开始量，所以同样的数字在普通显示器和高密度显示器上都是对的。`gfx.Rect` 保存 `X`、`Y`、`Width` 和 `Height`。`Rect.Inset` 按 `gfx.Insets`（各边的内边距）缩小它，`gfx.All(n)` 生成四边相等的内边距。

`Paint` 收到当前窗口大小，并在每次改变大小时运行，所以依赖 `size` 的布局会跟随窗口。

## 切分空间

三个小型值类型负责分配矩形，都不分配内存。

| 类型 | 用途 |
| --- | --- |
| `gfx.BeginRow(bounds, gap)` | 从左到右的槽位：`Next(width)` 取固定宽度，`NextFlex()` 取剩余部分，`Even(n)` 平均切分。 |
| `gfx.BeginColumn(bounds, gap)` | 同上，方向从上到下。 |
| `gfx.BeginGrid(bounds, spec)` | 由等大单元格组成、会换行的网格。`GridSpec.MinCellWidth` 是单元格最窄的宽度；网格尽可能多地放入列，再把单元格拉伸以填满宽度。 |

当界面就是一个页眉、一个页脚和一个主体时，从边缘切条比一列槽位更好读。`Rect.CutTop`、`CutBottom`、`CutLeft` 和 `CutRight` 返回切下的条和剩余部分，并做了夹取，所以两者都不会出现负尺寸；`Centered`、`Offset`、`Right` 和 `Bottom` 覆盖其余的边缘计算。

```go
header, body := page.CutTop(48)
sidebar, content := body.CutLeft(240)
footer, content := content.CutBottom(32)
```

<!-- example: drawing/main.go -->
```go
package main

import (
	"fmt"
	"log"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
)

// Identities are names in a tree, not numbers: the application declares the
// tree once and every control it paints is a name inside a feature of it.
var (
	tree = gfx.Tree{}
	page = tree.Feature("drawing")

	hitNew = page.ID(tree.Name("new"))
)

func paint(canvas *gfx.Canvas, size gfx.Size) {
	theme := gfx.LightTheme()

	// Lay out by slicing rectangles; none of this allocates.
	page := gfx.Rect{Width: size.Width, Height: size.Height}.Inset(gfx.All(24))
	column := gfx.BeginColumn(page, 16)

	canvas.Label("Dashboard", column.Next(32), gfx.TextStyle{Size: 20, Weight: gfx.WeightSemibold, Color: theme.Foreground})

	toolbar := gfx.BeginRow(column.Next(36), 8)
	components.BeginButton(canvas, theme, hitNew, toolbar.Next(100)).Default("New")

	cards := column.NextFlex() // whatever height is left
	grid := gfx.BeginGrid(cards, gfx.GridSpec{MinCellWidth: 160, CellHeight: 96, Gap: 12})
	for index := 1; index <= 6; index++ {
		cell := grid.Next()
		canvas.RoundedBorder(cell, 8, 1, theme.Border, theme.Card)
		canvas.Label(fmt.Sprintf("Card %d", index), cell.Inset(gfx.All(12)), gfx.TextStyle{Size: 14, Color: theme.CardForeground})
	}
}

func main() {
	var canvas gfx.Canvas
	config := app.Config{Tree: &tree, Title: "Layout", Width: 720, Height: 480, Paint: paint}
	if err := app.RunCanvas(config, &canvas); err != nil {
		log.Fatal(err)
	}
}
```

把窗口缩窄，网格就会减少列数，一直减到一列，不需要任何额外代码，因为列数来自传给 `Paint` 的宽度。示例的测试检查的正是这一点。

## 绘制

画布按你调用的顺序绘制，后面的调用覆盖前面的。

| 方法 | 绘制 |
| --- | --- |
| `Fill`、`Rounded` | 矩形，直角或圆角。 |
| `Border`、`RoundedBorder` | 带边框的填充矩形。 |
| `Elevate`、`Shadow` | 圆角矩形下的柔和投影：`Elevate` 取 Tailwind 阴影档位（`gfx.ShadowXS`、`ShadowSM`、`ShadowMD`、`ShadowLG`），`Shadow` 是单层 box-shadow。要在表面之前调用；投影里会挖掉矩形本身。 |
| `Label`、`LabelCenter`、`LabelAligned` | 一行系统字体文字。 |
| `DrawIcon` | 已编译的 `gfx.Icon`，带着色。用 `viva icons` 把 SVG 编译成 Go。 |
| `DrawPath`、`DrawBitmap` | 矢量路径或图片。 |
| `DrawBackdrop` | 画在整帧所有内容之下的图片，不受层和裁剪影响；半透明的界面会透出它。 |
| `Clip`、`Unclip` | 把绘制和命中区域限制在一个矩形内，可以嵌套。 |

文字使用 `gfx.TextStyle` 绘制：字号、字重（`gfx.WeightRegular`、`WeightMedium`、`WeightSemibold`……）、颜色，以及用于终端和代码的 `Mono`。文字是单行的。要把一段文字排进某个宽度，调用 `Canvas.WrapLines`，它在空格处和中日韩字符之间断行，然后每行画一个 `Label`。用 `Mono` 或斜体样式绘制的行，改用 `Canvas.WrapLinesStyle`，它按该样式实际绘制的宽度量每一行，所以等宽段落在格子排满宽度处断行，一个中日韩字符占两格。

排版文字时常常需要知道它的尺寸。`Canvas.TextWidth` 和 `TextWidthWeight` 用系统字体按你将要绘制的字重来测量，因为更粗的文字更宽。

## 声明交互

画一个矩形并不会让它可点击。要声明命中区域并用 `gfx.Tree` 中的名字作为标识：`Hit`、`HitWithLabel`（标签是辅助技术对它的称呼），或者带角色、光标、状态等信息的 `HitWithOptions`。组件会替你做这件事；自己画控件时，就由你来做。接下来会发生什么见[处理输入](/zh/docs/handling-input/)。

## 动画

Viva 只在有东西请求时才绘制，所以动画必须去请求。`Canvas.Animate(id, slot, target, motion)` 返回现在应当绘制的值，从当前位置缓动到 `target`，并在到达之前持续重绘，所以过渡中途反向也是连续的。用户要求减弱动效时，它原样返回 `target`。对于变化缓慢的东西，比如每 125 毫秒走一步的旋转指示器，`Canvas.RequestFrameIn(delay)` 请求在延迟之后再绘制一次；不再请求的那一帧会让应用回到空闲。
