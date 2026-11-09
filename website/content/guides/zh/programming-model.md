---
title: 编程模型
summary: 应用持有的状态、声明的命中区域、有类型的事件，以及后台任务如何回到窗口。
order: 2
group: concepts
---

## 状态归你

Viva 应用把自己的状态放在普通的 Go 值里。`Paint` 回调读取状态并描述界面；Viva 把每次描述与一个保留式场景对齐。什么都没变时，重绘不做布局、不重建场景、不向 GPU 提交任何东西；只有一个节点变化时，只上传一小段实例数据。

组件遵循同样的规则。`f.Button`、`f.Dialog` 等都不拥有数值、勾选状态或打开状态。你把当前值传进去，再处理那次应该改变它的点击。

## 命中区域与标识

任何可交互的东西都是绘制时声明的命中区域，标识由你在 `gfx.Tree` 中命名。标识在帧与帧之间要保持稳定：焦点、悬停和按下状态都按标识跟踪，`Canvas.Focused`、`Canvas.FocusVisible` 和 `Canvas.Hovered` 会把它们报告回来，所以绘制代码读取它们，而不是自己再存一份。

```go
var (
	tree = gfx.Tree{}
	page = tree.Feature("dashboard")

	hitRefresh = page.ID(tree.Name("refresh"))
	hitPanel   = page.ID(tree.Name("panel"))
)
```

## 事件

按钮在声明处处理：`f.Button(save, "Save").Clicked()` 只在递送这次点击的那一遍（事件遍）里为真，也只有在这一遍里绘制才可以修改状态。只有绘制调用了 `Clicked`，运行时才会加这一遍，不用的应用不付出任何代价。如果应用只有一个集中的点击处理函数，在绘制开头读一次 `canvas.ClickTarget()`，并通过 `canvas.AfterPaint(func() { ... })` 把标识交给它：处理函数在这次绘制返回之后（绘制之外，所以可以阻塞或打开原生对话框）运行，随后重绘；这同样会让该次绘制被视为在监听点击。优先在每个控件处用 `Clicked`。需要更多的控件使用有类型的回调，每个都返回 `gfx.EventResult`：

| 回调 | 收到什么 |
| --- | --- |
| `OnPointer` | 按下、移动、抬起、离开事件，附带目标和逻辑坐标。一次按下会在对应的移动和抬起事件中保持原来的目标。 |
| `OnKey` | 焦点目标的键盘事件。 |
| `OnShortcut` | 带 Control 或 Command 的可打印按键组合，早于原生文本系统收到。Windows 上获得焦点的普通文本框会先拿走标准编辑组合。 |
| `OnScroll` | 像素增量，附带窗口位置、修饰键和目标。 |

`gfx.EventResult` 说明应用对事件做了什么。`Handled` 取消该事件的内置行为，`Repaint` 请求一次同步绘制，`Focus`、`Blur` 和 `FocusVisible` 用来移动焦点。事件没有改变任何东西时，返回零值就不会重绘。

```go
config.OnKey = func(event gfx.KeyEvent) gfx.EventResult {
	if event.Key == gfx.KeyEscape && state.open {
		state.open = false
		return gfx.EventResult{Handled: true, Repaint: true}
	}
	return gfx.EventResult{}
}
```

所有回调都在窗口线程上运行，不能阻塞。

## 后台任务

其他 goroutine 从不直接操作窗口。goroutine 在你自己的同步保护下发布结果，然后调用 `Invalidator.Invalidate`。窗口线程被唤醒一次，运行 `OnWake`，再重绘。

<!-- example: background/main.go -->
```go
package main

import (
	"log"
	"sync"
	"time"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// Identities are names in a tree, not numbers: the application declares the
// tree once and every control it paints is a name inside a feature of it.
var (
	tree = gfx.Tree{}
	page = tree.Feature("background")

	refresh = tree.ID(page, "refresh")
)

// slowWork stands in for anything that takes a while: a request, a disk scan.
var slowWork = func() string {
	time.Sleep(time.Second)
	return "done"
}

type model struct {
	mu      sync.Mutex
	pending string // written by the worker goroutine, under mu
	status  string // read by Paint, written only on the window thread
	view    ui.View
}

func newConfig(m *model, invalidator *app.Invalidator) app.Config {
	theme := gfx.LightTheme()
	config := app.Config{Tree: &tree, Title: "Background", Width: 480, Height: 240, Invalidator: invalidator}

	config.Paint = func(canvas *gfx.Canvas, size gfx.Size) {
		bounds := gfx.Rect{Width: size.Width, Height: size.Height}
		canvas.Fill(bounds, theme.Background)

		m.view.Draw(canvas, theme, bounds, func(f *ui.Frame) {
			f.Row().Fill().Pad(24).Gap(12).Items(ui.Start).Children(func() {
				if f.Button(refresh, "Refresh").Clicked() {
					m.status = "working"
					go func() {
						result := slowWork()
						m.mu.Lock()
						m.pending = result
						m.mu.Unlock()
						invalidator.Invalidate() // wakes the window thread once
					}()
				}
				f.Text(m.status)
			})
		})
	}

	// OnWake runs on the window thread right before the repaint Invalidate asked
	// for: move what the goroutine published into state that Paint reads.
	config.OnWake = func() {
		m.mu.Lock()
		m.status, m.pending = m.pending, ""
		m.mu.Unlock()
	}
	return config
}

func main() {
	var invalidator app.Invalidator
	var canvas gfx.Canvas
	if err := app.RunCanvas(newConfig(&model{status: "idle"}, &invalidator), &canvas); err != nil {
		log.Fatal(err)
	}
}
```

`Invalidator` 不拥有 goroutine、定时器或 channel。从不调用它的应用不为它付出任何代价，而 `Paint` 读到的始终是 `OnWake` 已经搬到窗口线程上的状态。

### 页面作用域的 effect

属于页面的工作——当前目录的列表、选中主机的请求——用 `ui.Effect`，不要自己起 goroutine。effect 状态和 model 一起创建一次，在页面里声明：

```go
var load = ui.NewEffect[dirListing, string](func(ctx context.Context) (dirListing, error) {
	return readDir(ctx) // 不碰 C、Box、Frame：它在绘制之外运行
})

func (m *model) page(c ui.C) {
	ui.Effect(c, m.load, m.dir) // key：目录变了就取消旧 run
	if listing, err, ok := m.load.Take(); ok {
		m.apply(listing, err)
	}
	// ...
}
```

key 命名一次 run。同一个 key 不会再次启动；key 改变会取消旧 run 并启动新的；某次绘制不再声明它时取消；窗口关闭时全部取消。key 改变、effect 消失或窗口关闭之后才到的结果会被丢弃。`Take` 在窗口线程上、effect 声明之后读取一次；结果落地时页面会自己醒来。一个状态属于一个页面，effect 不能声明在 `Memo` 里。

run 函数收到的 context 是取消信号，它必须在该 context 结束时返回；框架不杀 goroutine、不轮询。只把页面绘制到自己 canvas 里的应用，把该窗口的 invalidator 交给 `PageConfig.Invalidator`，并在页面消失时调用 `Page.Close`；没有给 invalidator 的页面保留结果到下一次绘制，但无法主动唤醒窗口。

没有页面 key 可取消的长时间 watcher 仍然用自己的锁发布结果并调用 `Invalidator.Invalidate`。

## 文本输入

文本同样归应用所有。给 `Config.TextInput` 一个返回当前焦点字段 `text.Editor` 的提供者，Viva 会把它接到平台文本系统，包括选区和输入法组合输入。只有获得焦点的字段才交给原生编辑器；焦点离开时，当前文本在同一帧里重新绘制为静态标签。

## 窗口与生命周期

`app.RunCanvas` 运行一个窗口。`app.RunApplication` 运行多个：每个窗口各自拥有绘制、焦点、输入和关闭状态，最后一个窗口关闭时事件循环结束。`OnCloseRequest` 可以让窗口保持打开，例如在有未保存内容时确认。状态栏图标、弹出面板和隐藏窗口见[窗口与状态栏](/zh/docs/windows/)。
