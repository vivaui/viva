---
title: 对话框、菜单与弹出层
summary: 图层、焦点范围与关闭方式：点击外部就关闭的下拉菜单，以及模态对话框。
order: 9
group: guides
---

浮层最后绘制，位于更高的图层，并且会改变输入能去往哪里。在 `ui` 里，浮层声明在页面中它打开的那个位置，并且不占那里的空间。Viva 给每个浮层提供三样东西。

- **图层。** 弹出框（popover）和对话框画在页面之上，声明在它们上面的命中区域优先于下面的。
- **焦点范围。** 对话框打开期间把 Tab 困在里面，它后面的任何东西都拿不到键盘焦点；弹出框让 Tab 先在自己的控件间移动。两者关闭时都会把焦点还给打开前的位置。
- **关闭标识。** 用户按下 Escape 或点击浮层外部时，报告点击的那个标识。页面（`ui.Popover`、`ui.Modal`）替你生成它并关闭浮层。

和 Viva 里一贯的做法一样，浮层是否打开是你的状态。你声明它，它才存在。

<!-- example: overlay/main.go -->
```go
package main

import (
	"log"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

type model struct {
	menuOpen   bool
	dialogOpen bool
	status     string
}

func (m *model) page(c ui.C) {
	ui.Column(c).Fill().Pad(24).Gap(12).Items(ui.Start).Children(func() {
		actions := ui.Button(c, "Actions").Outline()
		if actions.Clicked() {
			m.menuOpen = !m.menuOpen
		}
		ui.Text(c, m.status).Muted()

		// A popover hangs off its anchor while menuOpen is true. A press outside it
		// or Escape closes it, and the press goes on to what is under the pointer;
		// the focus goes back to the anchor.
		ui.PopoverWith(c, actions, ui.PopoverSpec{Width: 180, Align: ui.Start}, &m.menuOpen, func() {
			if ui.Button(c, "New file").Ghost().Clicked() {
				m.menuOpen, m.status = false, "Created"
			}
			if ui.Button(c, "Delete…").Ghost().Destructive().Clicked() {
				m.menuOpen, m.dialogOpen = false, true
			}
		})

		// A modal dims the page and keeps the keyboard while dialogOpen is true:
		// Escape, a press on the dim and the close button set it to false.
		ui.ModalWith(c, &m.dialogOpen, "Delete file?", ui.ModalSpec{Description: "This cannot be undone.", Width: 400}, nil, func() {
			if ui.Button(c, "Cancel").Outline().Clicked() {
				m.dialogOpen = false
			}
			if ui.Button(c, "Delete").Destructive().Clicked() {
				m.dialogOpen, m.status = false, "Deleted"
			}
		})
	})
}

func main() {
	m := &model{}
	if err := ui.Run(app.Config{Title: "Overlays", Width: 640, Height: 360}, m.page); err != nil {
		log.Fatal(err)
	}
}
```

## 弹出框

`ui.Popover(c, anchor, &open, body)` 在 `open` 为真时显示 `body` 声明的面板，挂在 `anchor` 上，外观与 shadcn 的 Popover 一致。`ui.PopoverWith` 接受 `ui.PopoverSpec`，可以指定方向、对齐和宽度。它默认在锚点下方打开，下方放不下时翻到上方。要锚定到触发控件，就保留它的 `Box`（`actions := ui.Button(c, ...)`）。

在面板外按下或按 Escape 会把 `open` 设为 false。这次按下随后会传给指针下方的控件，与网页里的 popover 一致：一次点击另一个按钮，既关闭菜单，也按下那个按钮。在锚点上按下只会关闭面板，所以切换 `open` 的触发按钮不会在同一次点击里又把它打开。焦点回到面板打开前的位置。`Popover` 报告关闭它的那一遍绘制；必须保持打开的面板在那里把 `open` 改回 true。

它按主题的动效淡入，同时从锚点方向滑入几个像素，关闭时以同样方式淡出（`Theme.Transition` 为零时没有动画）；正在离场的面板只是一张画面，不接收点击。弹出框显示在它所在的窗口里；需要超出窗口边缘的，用[独立的窗口](/zh/docs/windows/)。

示例的测试把这些全检查了：窗口远角的按下关闭菜单，Escape 关闭菜单，选择 Delete… 会关闭菜单并打开对话框。

## 弹出层（组件作者）

`f.Popup()` 是组件自己绘制弹出框、选择框和菜单时的基础：它不占所在容器的空间，大小由内容决定，始终保持在窗口内，并画在所有东西之上，除此之外什么都不做。针对页面上的某个元素，用 `BelowBox`、`AboveBox` 或 `BesideBox` 定位；针对某个点或矩形，用 `At(x, y)`、`Below(rect, ...)` 和 `Above(rect, ...)`。它优先用你指定的一侧，放不下时翻到另一侧。`Box.Rect()` 在这一帧结束前为零，所以才有 `Box` 版本的定位方法。右键菜单是用 `At` 放在指针处的弹出层，在 `OnPointer` 里收到 `PointerButtonSecondary` 按下时打开；关闭由你负责，在它下面放一个与窗口一样大的 `Hit`，并调用 `canvas.SetEscapeTarget`。应用页面改用 `ui.Popover`、`ui.Menu`、`ui.ContextMenu`、`ui.Select` 和 `ui.Combobox`。

## 对话框

`ui.Modal(c, &open, title, body, footer)` 在 `open` 为真时把窗口变暗，画出居中的表面，带标题和关闭按钮，并占住键盘。标题是必填的：它是对话框给辅助技术的名字。`body` 声明正文，`footer` 声明按钮，按钮按给定顺序靠右排列；两者都可以为 nil，表面高度正好等于正文所需。Escape、点击变暗的背景和关闭按钮都会把 `open` 设为 false。对话框打开时，正文里的第一个控件获得焦点（`ModalSpec.InitialFocus` 可以指定另一个）；关闭时，焦点回到打开它的那个控件。`ui.ModalWith` 接受 `ui.ModalSpec`，用来加描述、指定大小或去掉关闭按钮。

`ui.Dialog(c, ui.DialogSpec{...})` 是同一个对话框由应用命名目标的形式：`Scope` 是焦点范围，`DismissID` 是背景和 Escape 报告的标识，`CloseID` 是右上角按钮的，`InitialFocusID` 是获得焦点的控件。`Dismissed()` 在递送其中任一点击的那一遍绘制里为真。

## 选择框及其他

`ui.Select(c, &m.size, sizes, nil)` 是下拉列表：模型里放一个 `ui.SelectState` 和它自己的 scope，列表在触发器下方展开（放不下时在上方），画在所有东西之上；在对话框里它在对话框上方展开。工具提示和其他非模态面板是不声明关闭标识的弹出层，所以它们不困住焦点，也不吞掉外部点击。Combobox、命令面板、菜单栏和导航菜单遵循同样的图层、范围和关闭约定；[组件目录](/zh/ui/dialog/)为每一个都提供了可运行的示例，[API 参考](/zh/api/components/)列出了全部选项。

## 进场与退场

对话框、Sheet、抽屉，以及组件配方里的菜单、弹出层、提示和 Toast，会在你开始绘制它们时淡入并移动到位，在你停止绘制时淡出，和 shadcn 的 `animate-in`、`animate-out` 一样。你不需要为此保存任何状态：不再声明对话框，画布会在接下来的 100 毫秒里把它最后一帧画完淡出，这期间它没有命中区域、焦点陷阱和 Escape 目标，不会从下面的界面抢走点击或键盘。退场中再次绘制，会从当前位置掉头。它需要稳定的标识（对话框的 `DismissID`，菜单的 `DismissID` 或 `Scope`），菜单、Select 和 Combobox 需要调用 `End()`；主题的 `Transition` 没有时长、减弱动画或运行时没有帧时钟时，直接显示最终状态。
