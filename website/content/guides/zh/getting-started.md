---
title: 开始使用
summary: 准备环境，把 Viva 加入 Go 模块，打开第一个窗口。
order: 1
group: start
---

## 环境要求

- **Go 1.26 或更新版本。** 使用 `viva` 命令时如果机器上没有，它会替你下载；见[构建与打包](/zh/docs/build-and-package/)。
- **macOS：** Xcode 命令行工具。Viva 通过 cgo 调用 Cocoa 和 Metal。
- **Linux：** X11，以及系统自带的 XCB 和 Vulkan 加载库。Viva 在运行时加载它们，自身不捆绑任何一个。
- **Windows：** Win32，以及系统自带的 Direct3D 11 和 DirectWrite 库。

## 把 Viva 加入项目

Viva 的模块路径是 `github.com/vivaui/viva`：

```sh
mkdir hello && cd hello
go mod init example.com/hello
go get github.com/vivaui/viva
```

`go get` 不带版本号时取最新的正式版本，目前就是 `v0.4.0`。要固定版本就写明：`go get github.com/vivaui/viva@v0.4.0`。Viva 还没有到 1.0，所以小版本之间 `go.mod` 的 API 仍可能变化；升级前看一眼发版说明。

如果你是在 Viva 仓库里改框架本身、而不是使用它，改用 `replace` 指向检出目录：

```sh
go mod edit -require=github.com/vivaui/viva@v0.0.0 -replace=github.com/vivaui/viva=../viva
```

## 第一个窗口

<!-- example: hello/main.go -->
```go
package main

import (
	"fmt"
	"log"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/ui"
)

// Identities are names in a tree, not numbers: the application declares the
// tree once and every control it paints is a name inside a feature of it.
var (
	tree = ui.Tree{}
	page = tree.Feature("hello")

	click = tree.ID(page, "click")
)

type model struct {
	clicks int
	view   flex.View
}

// newConfig describes the window. Keeping it in a function lets a test drive
// the same application without opening anything.
func newConfig(m *model) app.Config {
	theme := ui.LightTheme()
	config := app.Config{Tree: &tree, Title: "Hello", Width: 480, Height: 240}

	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		bounds := ui.Rect{Width: size.Width, Height: size.Height}
		canvas.Fill(bounds, theme.Background)

		m.view.Draw(canvas, theme, bounds, func(f *flex.Frame) {
			f.Column().Fill().Pad(24).Gap(12).Children(func() {
				// A button is handled where it is declared: Clicked is true for the
				// one paint that delivers its click, and only then may state change.
				if f.Button(click, fmt.Sprintf("Clicked %d times", m.clicks)).Clicked() {
					m.clicks++
				}
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

用 `go run .` 运行。这个程序就是整个编程模型的缩影：

- `newConfig` 描述窗口。把它放进一个函数，测试就可以在不打开任何窗口的情况下驱动同一个应用。
- `Paint` 根据你自己的状态（这里是 `clicks` 计数）描述界面。Viva 不保存这份状态的副本。
- `flex.View` 根据描述来布局页面：一个带内边距的列，里面放一个按钮。不写任何坐标，所以标签变长或窗口改变大小时页面依然正常。按钮有稳定的标识 `click`，是树中 `hello` feature 下的一个名字。
- 一次点击完成时，运行时会多画一遍，`Clicked()` 只在这一遍里、且只对被点击的按钮为真；你的代码就在按钮旁边修改自己的状态，窗口随后重绘一次。其他所有绘制都是只读的。

除此之外什么都不会运行。你不点击时，应用不做布局、不向 GPU 提交任何东西，也没有被唤醒的定时器。

## 试试演示程序

如果你检出了 Viva 仓库，可以直接在里面看这些演示（它们不在别人安装的模块里运行）：

```sh
go run ./cmd/showcase            # 原生组件展示
go run ./cmd/viva list            # viva add 可以复制进项目的组件
go run ./cmd/component-preview -component button -example variants
```

同样的组件也能在浏览器里运行。[组件目录](/zh/ui/button/)提供可交互的 WebAssembly 预览、每个示例的 Go 源码，以及运行它的原生命令。

## 接下来

- [编程模型](/zh/docs/programming-model/)：状态、事件、焦点与后台任务。
- [组件](/zh/docs/components/)：使用组件目录，以及拥有组件的一份副本。
- [构建与打包](/zh/docs/build-and-package/)：`viva.toml`、发布构建与安装包。
