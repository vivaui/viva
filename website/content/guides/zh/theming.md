---
title: 主题与字体
summary: 浅色与深色主题、覆盖令牌、控件密度、减弱动效和等宽字体。
order: 8
group: guides
---

## 主题是值

`ui.Theme` 是一个由语义化颜色令牌和尺寸度量组成的结构体。没有样式表，运行时也不解析任何东西。每个组件都以参数形式接收你给它的主题，所以改变外观就是用另一个值来绘制：

<!-- example: theming/main.go -->
```go
package main

import (
	"log"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/ui"
)

// Identities are names in a tree, not numbers: the application declares the
// tree once and every control it paints is a name inside a feature of it.
var (
	tree = ui.Tree{}
	page = tree.Feature("theming")

	toggle = tree.ID(page, "toggle")
)

type model struct {
	dark bool
	view flex.View
}

// theme is the active theme: a value, so changing appearance is changing which
// value Paint reads.
func (m *model) theme() ui.Theme {
	theme := ui.LightTheme()
	if m.dark {
		theme = ui.DarkTheme()
	}
	// Tokens are plain fields. Override one to brand the application.
	theme.Primary = ui.Color{Red: 0.9, Green: 0.45, Blue: 0.1, Alpha: 1}
	theme.PrimaryForeground = ui.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}
	return theme
}

func newConfig(m *model) app.Config {
	config := app.Config{Tree: &tree, Title: "Theme", Width: 480, Height: 240}
	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		theme := m.theme()
		bounds := ui.Rect{Width: size.Width, Height: size.Height}
		canvas.Fill(bounds, theme.Background)

		m.view.Draw(canvas, theme, bounds, func(f *flex.Frame) {
			f.Column().Fill().Pad(24).Gap(12).Items(flex.Start).Children(func() {
				if f.Button(toggle, "Toggle theme").Clicked() {
					m.dark = !m.dark
				}
				f.Text("Hello")
			})
		})
	}
	return config
}

func main() {
	var canvas ui.Canvas
	if err := app.RunCanvas(newConfig(&model{}), &canvas); err != nil {
		log.Fatal(err)
	}
}
```

`ui.LightTheme()` 和 `ui.DarkTheme()` 返回完整的主题。令牌是普通字段：页面用 `Background` 和 `Foreground`，表面用 `Card` 和 `CardForeground`，菜单和对话框用 `Popover`，还有带对应前景色的 `Primary`、`Secondary`、`Accent` 和 `Muted`，文本选区用 `Selection` 和 `SelectionForeground`，以及 `Border`、`Input`、用于焦点环的 `Ring`、`Success` 和 `Danger`，外加五种 `Chart` 颜色。覆盖某个字段就能为应用定制品牌色，示例就是这样改 `Primary` 的。

用一个函数返回当前主题，像上面的 `theme` 方法那样，并在 `Paint` 开头读取它。其他地方都不需要知道当前用的是哪个主题。

## 跟随系统

Viva 目前没有暴露操作系统的浅色或深色设置，也从不替你选择主题。应用把用户的选择保存在自己的状态里，示例里的切换按钮就是这样做的。运行时会报告的外观偏好只有减弱动效。

## 密度

`Theme.Metrics` 携带组件使用的尺寸令牌：控件高度、圆角、字号和内边距。`Metrics.WithDensity(ui.DensityCompact)`（或 `DensityLarge`）可以派生出另一种密度的度量，而不改变颜色：

```go
theme := ui.DarkTheme()
theme.Metrics = theme.Metrics.WithDensity(ui.DensityCompact)
```

组件从度量中读取几何尺寸而不是写死，所以窗口里的每个控件都会跟着变化。

## 动效

`Theme.Transition` 是控件默认的状态变化动效；零值会关闭动画，手写的主题就是这样。`app.ReduceMotion()` 在 macOS 和 Windows 上报告操作系统的减弱动效偏好，其他平台返回 `false`。`Canvas.Animate` 已经遵守它，会原样返回目标值，所以通过画布做动画的组件不需要额外处理。

## 文字

`ui.TextStyle` 接受 `Size`、`Weight` 和 `Color`。文字用系统字体绘制，所以每个平台显示自己的字体，中日韩文字不需要任何设置就会回退到系统的中日韩字体。Viva 不捆绑字体。

用于代码和终端时，设置 `Mono: true`，默认使用系统的等宽字体。`MonoFace` 可以选择别的：`ui.MonoClassic` 请求传统的终端字体，`ui.MonoFamily(name)` 选择任何已安装的字体族。`app.MonospaceFamilies()` 列出它们，供设置页使用；它每次都会读取系统，可能要几十毫秒，所以在页面打开时调用，不要在绘制过程中调用。等宽行里每个 ASCII 字符的前进宽度都是 `Canvas.CellSize(size)`，这正是字符网格需要的。
