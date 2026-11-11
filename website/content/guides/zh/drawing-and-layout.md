---
title: 绘制与布局
summary: 坐标、用行、列和网格切分空间，以及绘制矩形、文字、图标和路径。
order: 6
group: guides
---

屏幕上的一切都是在你的 `Paint` 回调里画进 `gfx.Canvas` 的。没有需要构建的控件树，也没有样式表：你计算矩形，然后让画布填充它们、在里面绘制文字，或者把它们声明为可交互。

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
