---
title: 窗口与状态栏
summary: 窗口样式、自定义标题栏、多窗口、关闭前询问，以及菜单栏或托盘里的图标。
order: 3
group: guides
---

## 一个窗口还是多个

`app.RunCanvas` 打开单个窗口，窗口关闭时返回。需要多个窗口，或者窗口隐藏后应用仍要继续运行时，使用 `app.RunApplication`。它的启动回调在原生事件循环线程上运行一次，用 `Application.OpenWindow` 打开第一个窗口；之后的窗口从这个回调里，或者从别的窗口的回调里打开，那些回调运行在同一个线程上。

每个窗口都有自己的画布、绘制回调、焦点、输入状态和关闭处理。最后一个窗口关闭时，或有人调用 `Application.Quit` 时，事件循环结束；`Quit` 会关闭所有窗口，无论它们的关闭处理怎么说。

## 窗口样式

`Config.WindowMode` 决定窗口外框：

| 模式 | 窗口 |
| --- | --- |
| `WindowTitled` | 标准的带标题栏窗口，默认值。 |
| `WindowFrameless` | 没有任何外框，包括红绿灯按钮。 |
| `WindowCustomTitlebar` | 内容延伸到透明标题栏之后，占满整个高度，窗口从你用 `MovesWindow`（或 `Canvas.MoveRegion`）声明的区域移动，别处不能。 |
| `WindowPanel` | 无外框的临时弹出窗口，位于其他窗口之上，用来在状态栏图标旁边打开。 |

使用自定义标题栏时，窗口按钮在 macOS 上由系统绘制，在 Windows 和 Linux 上由 Viva 绘制。无论哪种，它们都会盖住条带的一部分，所以用 `app.TitlebarInsets` 询问盖住多少，让自己的内容避开。`Config.TitlebarHeight` 设置条带高度（Viva 放置自绘按钮和 macOS 红绿灯的位置）；窗口从你标记了 `MovesWindow` 的那一行移动，行内的控件仍然接收自己的点击，行上方打开的菜单或对话框会被那里的点击关闭，指针拖动后窗口才移动。`Config.WindowControls` 给 Viva 绘制的按钮定样式，使它们跟随你的主题。

## 全屏

调用 `window.SetFullscreen(true)` 进入原生全屏，调用 `window.SetFullscreen(false)` 退出。这个调用可以从任意 goroutine 发起；返回 nil 表示请求已被接受，并不表示原生动画已经结束。不支持全屏的宿主会返回一个包装了 `app.ErrUnsupported` 的错误。

`Config.OnFullscreen func(bool)` 在窗口线程上运行，既用于应用发起的请求，也用于系统发起的切换（包括 macOS 的绿色按钮和全屏快捷键）。回调运行时 `window.Fullscreen()` 已经更新。回调先于这次切换的 resize/frame 通知执行，之后 Viva 会自动重绘。用这个回调切换应用自己的布局；显示哪些内容仍由应用决定。对同一状态重复请求不会再次触发切换回调。

Windows 使用整个显示器而不是任务栏工作区，并保存之前的位置和最大化状态。macOS 使用 AppKit 的全屏空间和原生动画。Linux 在 X11 上使用 `_NET_WM_STATE_FULLSCREEN`，在 GTK/Wayland 上使用 GTK 的原生全屏 API。Linux 的位置恢复由窗口管理器负责。macOS 的状态栏面板窗口不是全屏窗口。浏览器宿主不实现这个能力。

全屏时 Viva 会隐藏自己的窗口按钮，并禁用标题栏的移动和缩放行为。macOS 的原生全屏外框仍由 AppKit 管理。全屏期间普通窗口位置不会通过 `OnFrameChange` 上报，所以持久化普通窗口边界时不会误存一个显示器矩形。`Frame()` 仍然描述当前的内容区域。

测试时，`Driver.SetFullscreen(bool) error` 使用同一套回调并立即重绘。`Driver.SetFullscreenMonitor(ui.Rect) error` 以逻辑像素定义整个显示器的边界（默认 `(0,0,1920,1080)`）；负的原点表示位于主显示器左侧的显示器。全屏期间更换显示器或调整大小不会替换退出时恢复的位置。`Driver.Fullscreen()` 报告模拟出来的状态。与 `Window` 的方法不同，Driver 的操作在调用者的线程上运行，不是并发安全的。

无头应用的 `Window.SetFullscreen` 把工作排进 driver 的下一次 pump，不需要 worker，也不轮询。它的命令协议还提供 `fullscreen on|off` 和 `fullscreenmonitor x y width height`。

## 完整示例

下面的程序打开一个自定义标题栏窗口，其中的“Settings”按钮会打开第二个窗口，并且在有未保存的笔记时拒绝关闭。顶部条带是一个普通的 `Row`，用 `app.TitlebarInsets` 留出内边距，所以标题不会压到窗口按钮下面；确认框是 `f.Dialog`，只在有关闭请求挂起时才声明：

<!-- example: windows/main.go -->
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
	tree    = ui.Tree{}
	page    = tree.Feature("notes")
	confirm = tree.Sub(page, tree.Name("confirm"))

	settingsButton = tree.ID(page, "settings")
	confirmScope   = tree.ID(confirm, "scope")
	keep           = tree.ID(confirm, "keep")
	discard        = tree.ID(confirm, "discard")
)

type notes struct {
	unsaved        bool
	confirmClosing bool
	allowClose     func()
	view           flex.View
}

func main() {
	theme := ui.LightTheme()
	state := &notes{unsaved: true}

	err := app.RunApplication(func(application *app.Application) error {
		var main, settings ui.Canvas

		config := app.Config{
			Tree: &tree, Title: "Notes", Width: 720, Height: 480,
			// Content runs full height under a transparent title bar; the window
			// moves from the row marked MovesWindow. Keep your own content clear of
			// the window buttons.
			WindowMode: app.WindowCustomTitlebar,
		}
		leading, trailing := app.TitlebarInsets()
		config.Paint = func(canvas *ui.Canvas, size ui.Size) {
			bounds := ui.Rect{Width: size.Width, Height: size.Height}
			canvas.Fill(bounds, theme.Background)

			state.view.Draw(canvas, theme, bounds, func(f *flex.Frame) {
				f.Column().Fill().Children(func() {
					// The bar is as tall as the title bar and leaves room for the
					// window buttons on whichever side the platform puts them.
					f.Row().Height(28).PadSides(0, trailing, 0, leading).Items(flex.Center).MovesWindow().Children(func() {
						f.Text("Notes").Muted()
					})
					f.Column().Grow(1).Pad(24).Items(flex.Start).Children(func() {
						if f.Button(settingsButton, "Settings").Outline().Clicked() {
							// Windows are opened from the start callback or from another
							// window's callbacks, which run on the native event-loop thread.
							_, _ = application.OpenWindow(app.Config{Title: "Settings", Width: 420, Height: 320}, &settings)
						}
					})
					if state.confirmClosing {
						d := f.Dialog(flex.DialogSpec{
							Title: "Discard changes?", Description: "The note has not been saved.", Width: 400,
							Scope: confirmScope, DismissID: keep, NoClose: true,
						})
						d.Footer(func() {
							f.Button(keep, "Keep editing").Outline()
							if f.Button(discard, "Discard").Destructive().Clicked() {
								state.confirmClosing = false
								state.unsaved = false
								state.allowClose()
							}
						})
						if d.Dismissed() {
							state.confirmClosing = false
						}
					}
				})
			})
		}
		// The close button asks. Call allow now, later, or never to keep the window.
		config.OnCloseRequest = func(allow func()) {
			if !state.unsaved {
				allow()
				return
			}
			state.confirmClosing, state.allowClose = true, allow
		}

		_, err := application.OpenWindow(config, &main)
		return err
	})
	if err != nil {
		log.Fatal(err)
	}
}
```

`OnCloseRequest` 收到的是一个 `allow` 函数，而不是返回一个答案，所以决定可以等用户来做。现在、稍后或永远不调用 `allow` 都可以；永远不调用就让窗口保持打开。这里窗口弹出一个确认对话框，由“Discard”按钮调用 `allow`。

## 隐藏窗口

`Window.Hide` 和 `Window.Show` 让窗口及其状态保留，只是把它从屏幕上拿走，`Window.Hidden` 和 `Window.Closed` 报告它的状态。隐藏的窗口仍然能收到 `Invalidator` 的唤醒，所以常驻状态栏的应用可以借助一个没人看得见的窗口持续更新自己的图标。`Window.Close` 是幂等的，可以从任何 goroutine 调用。

## 窗口在屏幕上的位置

`Config.Origin` 请求窗口内容区打开的位置，`Window.Frame` 报告它实际所在的位置，两者都用屏幕坐标：从主显示器左上角起算的逻辑像素，y 向下。`ui.PointerEvent` 同时带有 `ScreenX` 和 `ScreenY`，按住不放的按键在指针离开窗口后仍会持续报告它们，所以结束在应用另一个窗口上的拖动，可以拿指针和各窗口的 `Frame` 比较来找到目标窗口。Windows 和 macOS 把内容区放在该原点；X11 窗口管理器把窗口框架放在那里，因此内容区在标题栏下面。在缩放比例不同的显示器之间，不同显示器上窗口的坐标只能近似比较。

`Config.OnFrameChange` 在这个矩形移动或在屏幕上改变大小时运行，运行在窗口线程上，参数就是那一刻 `Frame` 会返回的矩形；尺寸变化时 `OnResize` 仍会先运行，用于重建布局。应用在这里保存矩形（拖动过程中的连续事件需要自己防抖），下次开窗时再作为 `Config.Origin`、`Config.Width` 和 `Config.Height` 传回来。

`Config.OnMinimize` 在窗口管理器最小化或还原窗口时运行，不管是谁要求的；`Window.Minimized` 报告它最后一次报告的状态。`Window.Hide` 是应用自己的请求，不会在这里报告。最小化的窗口仍然存在、仍然收到唤醒，所以正在推流画面的应用应该停止向远端索取画面，而不是关闭任何东西。被其他窗口遮挡不会报告：三个平台并不能都可靠地说出来。

## 状态栏里的图标

`Application.AddStatusItem` 在 macOS 的菜单栏、Windows 的通知区域和 Linux 的 StatusNotifier 托盘里放一个图标。给它一个正方形的 RGBA 图标、一段提示文字，以及 `OnClick` 处理函数或 `Menu`：

<!-- example: windows/tray.go -->
```go
package main

import "github.com/vivaui/viva/app"

// addTray keeps an icon in the menu bar, notification area or tray. The icon is
// square RGBA; a template icon has only alpha and is tinted to suit the bar.
func addTray(application *app.Application, window *app.Window, icon app.StatusIcon) (*app.StatusItem, error) {
	return application.AddStatusItem(app.StatusItemConfig{
		Tooltip: "Notes",
		Icon:    icon,
		Menu: func() []app.MenuItem {
			// Built each time the menu is about to open, so it shows current state.
			return []app.MenuItem{
				{Title: "Show Notes", OnSelect: window.Show},
				{Title: "Hide Notes", OnSelect: window.Hide, Disabled: window.Hidden()},
				{Separator: true},
				{Title: "Quit", OnSelect: application.Quit},
			}
		},
	})
}
```

设置了 `Menu` 时，点击会打开系统自己的原生菜单，每次菜单将要打开时都会调用你的函数来构建，所以总是显示当前状态。没有 `Menu` 时，`OnClick` 收到一个 `StatusClick`，里面有按钮，以及一个 `Anchor`，把它赋给 `Config.Anchor` 就可以在图标旁边打开 `WindowPanel`。面板在失去焦点时会自己关闭；`StatusClick.PanelDismissed` 告诉你的处理函数：刚刚到达的这次点击正是关闭它的那一次，所以用来切换面板的点击不会把它又打开。

之后可以用 `StatusItem.SetIcon`、`SetTooltip` 和 `Remove` 修改或移除图标。

## 各平台的差异

同一个调用只能说出平台能说的东西。状态栏图标在 macOS 和 Windows 上报告它的矩形，在 Linux 上只报告点击点；面板分别因失去 key 状态、窗口停用或 `FocusOut` 而关闭。在已满的 macOS 菜单栏里，新图标可能被系统隐藏，尤其是刘海旁边；Windows 11 会把新图标放进溢出浮层，直到用户把它提升出来。仓库中的 `docs/conformance.md`列出了这些，以及每个平台必须手动检查的项目。
