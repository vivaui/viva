---
title: 处理输入
summary: 指针、键盘、快捷键、滚动和焦点，附带一个可以拖动、打字和滚动的完整示例。
order: 4
group: guides
---

页面的输入在它起作用的地方声明。点击在声明控件的地方读取：`ui.Button(c, "Save").Clicked()` 在投递该点击的那一遍绘制中为真。`ui.List` 自己选择被点击的行，并让方向键移动选择；`ui.Splitter` 自己捕获拖动，并让两个窗格保持在限制内；次按钮按下是行上的 `ui.ContextMenu`。页面把窗口的指针、按键、滚轮和文本输入路由给它声明的控件，而 `app.Config` 仍为页面未建模的输入保留有类型的回调，例如自绘控件，或负责转发另一个页面事件的主机。本指南覆盖这两层。下面的示例是一个带可拖动分隔条的面板、一个可以点击并用方向键移动的列表、一个上下文菜单、一个键盘快捷键和滚轮滚动。

<!-- example: input/main.go -->
```go
package main

import (
	"log"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var mailboxes = []string{"Inbox", "Drafts", "Sent", "Archive", "Trash"}

type model struct {
	split   ui.SplitterState
	list    ui.ListState
	palette bool
	context string
}

func newModel() *model {
	m := &model{split: ui.SplitterState{Pos: 220}}
	// ListState.Label is the row's name for assistive technology, for the text
	// type-ahead matches, and for a test looking a row up by name.
	m.list.Label = func(i int) string { return mailboxes[i] }
	return m
}

// selected is the mailbox the detail pane speaks of: the list's choice, or the
// first one until the user makes a choice.
func (m *model) selected() int {
	if i := m.list.Selected(); i >= 0 {
		return i
	}
	return 0
}

// page declares the window. Each control writes what it changed into the model,
// and the page reads that state as it declares the next paint.
func (m *model) page(c ui.C) {
	// A shortcut is declared where it acts, as a click is read where it acts.
	if ui.Shortcut(c, ui.Primary, "k") {
		m.palette = !m.palette
	}

	theme := c.Theme()
	detail := mailboxes[m.selected()] + " selected"
	if m.palette {
		detail = "Command palette"
	}
	ui.Row(c).Fill().Children(func() {
		// The first pane is as wide as the splitter's state says; the list itself
		// keeps its selection and scrolling.
		ui.Column(c).Width(m.split.Pos).Background(theme.Card).Children(func() {
			ui.List(c, &m.list, len(mailboxes), m.row).Label("Mailboxes").Grow(1)
		})
		// The splitter is the draggable divider: it captures the pointer, shows
		// the resize cursor and keeps both panes within their minimum sizes.
		ui.Splitter(c, &m.split).Mins(120, 200).Label("Resize panes")
		ui.Column(c).Grow(1).Pad(16).Gap(8).Children(func() {
			ui.Text(c, detail)
			ui.Text(c, m.context).Muted()
		})
	})
}

// row is one mailbox: a row a click chooses and the keys move through, with a
// context menu on a secondary press.
func (m *model) row(r ui.ListRow) {
	theme := r.C.Theme()
	r.Element.Pad(8).PadX(12).Hover(ui.Look{}.Background(theme.Hover))
	if r.Index == m.selected() {
		r.Element.Background(theme.Hover)
	}
	ui.Text(r.C, mailboxes[r.Index])
	ui.ContextMenu(r.C, r.Element, func(menu *ui.MenuItems) {
		if menu.Item("Open").Clicked() {
			m.context = "Opened " + mailboxes[r.Index]
		}
	})
}

func newConfig(m *model) app.Config {
	theme := gfx.LightTheme()
	page := ui.NewPage(ui.PageConfig{View: m.page, Theme: &theme})
	return page.Config(app.Config{
		Title: "Input", Width: 640, Height: 360,
		// The page routes pointer, wheel, shortcut and text input to the controls
		// it declares. A splitter keeps its arrow keys in its own state, so give
		// them to the window's key callback.
		OnKey: m.split.Key,
	})
}

func main() {
	var canvas gfx.Canvas
	if err := app.RunCanvas(newConfig(newModel()), &canvas); err != nil {
		log.Fatal(err)
	}
}
```

## 标识

页面不会手工给元素命名。它按元素在父容器中同类元素里的位置命名，行则用 `c.Key`、`c.KeyInt`，或在 `ui.List` 里用 `ListState.Key` 命名，所以即使在上面插入行，焦点、悬停和按下仍会跟着那一行走。这些名字同时也是无障碍名称：`ListState.Label` 为辅助技术命名一行，也是输入前几个字母时跳转到的行，还是测试查找控件时使用的名字；`Box.Label` 为容器（例如列表）命名。测试用用户听到的同一个名字找到控件，不需要自己写坐标或树标识。

自己的控件用 `ui.ID(c)` 命名。用 `ui.Frame` 编写的组件自己在一棵 `gfx.Tree` 里声明命中区域；绘制到画布里的页面沿用画布的那棵树；应用页面两者都不写。

## 指针

点击在声明控件的地方处理：`ui.Button(c, "Save").Clicked()`，或 `ListRow.Selected()`，列表已经把点击造成的选择显示出来。两个窗格之间的拖动也不需要处理函数：`ui.Splitter(c, &m.split)` 会捕获指针、显示调整大小的光标，并让两个窗格都不小于各自的最小尺寸，示例只需要把第一个窗格的宽度设为 `m.split.Pos`。上下文菜单同样声明在它的目标上：`ui.ContextMenu(c, row.Element, ...)` 在次按钮按下时于指针处打开，Shift+F10 也可以从键盘打开。

`OnPointer` 负责页面没有建模的部分：自绘控件，或转发另一个页面指针事件的主机。它收到按下、移动、抬起和离开事件，坐标是逻辑窗口像素，`Target` 是指针下方的标识。一次按下会在之后的移动和抬起事件中保持原来的目标，此时 `Captured` 为真，这正是你自己实现的拖动在指针离开控件后仍然有效的原因。拖动过程中的移动事件可能被合并为队列里最新的位置（平台原生也会这样处理），因此处理逻辑应当依赖坐标而不是调用次数：按按下点到当前点的位移来拖动的实现，最终仍会停在指针所在的位置。

`PointerEvent.Button` 区分主按钮和次按钮。`ClickCount` 为相隔很近的同一按钮连续按下编号，单击是 1，双击的第二次是 2。`HitOptions.Cursor` 设置命中区域被悬停或拖动时的指针形状（`ui.Splitter` 已经在用 `PointerCursorResizeHorizontal`）。需要多个交互区域的自绘控件通过 `ui.Handlers` 声明它们。

## 键盘与焦点

页面把按键路由给持有焦点的控件。`ui.List` 自己使用它们：上和下移动选择，Home 和 End 到第一行和最后一行，Page Up 和 Page Down 按一屏移动，Enter 或双击由 `ListState.Submitted()` 报告，输入 `ListState.Label` 给出的名字的开头会跳到那一行。菜单打开时自己拿走方向键和 Escape，文本框自己拿走编辑键，都不需要处理函数。`ui.List` 返回的 box 上的 `Shortcut` 覆盖列表的行，所以某一行持有焦点时 `list.Shortcut(0, "delete")` 会生效。TextField 的 `.OnKey` 处理函数认领它被输入时该拿走的按键；没有认领的键回到页面，而输入的文字总是到达字段。

`ui.Splitter` 把键盘行为保存在自己的状态里：把 `SplitterState.Key` 交给窗口的按键回调，获得焦点的分隔条就能用方向键移动：

```go
page.Config(app.Config{OnKey: m.split.Key /* ... */})
```

`OnKey` 收到页面没有拿走的按键事件，目标是当前持有焦点的控件。`Target` 说明是哪一个，所以处理函数在动手前要先检查它。`Key` 是导航键、编辑键和功能键的名字（`KeyDown`、`KeyEscape`、`KeyEnter`、`KeyTab`……）。对于 Ctrl+C 这样的组合，`Key` 是 `KeyUnknown`，`Text` 是不带修饰键的字符。自己的控件用 `ui.Handlers` 接收按键；`OnKey` 是页面之下的兜底。

焦点由页面替你跟踪。在声明元素的地方，`Box.State().Focused`、`Hovered` 和 `Pressed` 报告指针与焦点状态；自绘的绘制代码读取 `Canvas.Focused` 和 `Canvas.FocusVisible`。读取这些值而不是自己保存副本，悬停、按下和焦点就永远不会和运行时的认知不一致。Tab 在页面可聚焦的命中区域之间移动，Enter 和空格激活获得焦点的按钮，标记了 `SkipFocus` 或 `Disabled` 的会被跳过。鼠标点击会让控件获得焦点但不显示焦点环；按 Tab 才会显示。

页面想把焦点放在模型里时，在控件上用 `ui.FocusBind`。设置字段会把焦点移到那里；用户移动焦点会写回字段；`ui.FocusedValue` 读取已经提交的焦点，而不是等待隐藏控件的请求。请求是边沿触发的，控件未声明时会等待：

```go
ui.FocusBind(ui.TextField(c, &m.query, "Search", ""), &m.focus, focusSearch)
// m.focus = focusSearch 把焦点移过去；FocusedValue 报告当前焦点。
```

控件需要有标识（控件本身，或用 `Hit` 的容器），并且焦点绑定不能声明在 `Memo` 里。

## 快捷键

页面在快捷键起作用的地方声明它：示例里的 `if ui.Shortcut(c, ui.Primary, "k") { m.palette = !m.palette }`。`ui.Primary` 在 macOS 上是 Command，在 Windows 和 Linux 上是 Control，所以不需要辅助函数。容器或控件上的 `Box.Shortcut` 只在焦点位于其中时生效，嵌套时最内层的胜出。不带 Primary、Ctrl 或 Alt 的组合会留给正在输入文本的文本框或终端；Popover、Modal、Menu，或 Select、Combobox 的列表打开时，页面自己的快捷键会等待。

自己写的处理器匹配到组合键、想让它像点击某个控件那样生效时，在结果里返回带那个控件标识的 `EventResult.Activate`：这一遍绘制会作为该点击的事件遍运行，与在获得焦点的按钮上按回车相同。

`OnShortcut` 在原生文本系统看到之前，先收到可打印按键组合，主要是带 Control 或 Command 的。只对应用自己拥有的组合返回 `Handled`：未处理的组合会继续走正常的文本编辑，所以在文本框里 Command-C 仍然是复制。在 Windows 上，获得焦点的普通文本框会先拿走标准编辑与剪贴板组合（复制、剪切、粘贴、全选、撤销、重做、Home/End、按词和按文档移动），终端和 `Placed` 编辑器不会。它是主机或自绘控件的兜底；页面自己的组合用 `ui.Shortcut`。

## 在列表中打字

设置了 `ListState.Label` 的 `ui.List` 会在用户输入一行开头的字母时移动到那一行。需要输入即搜索的自绘控件要声明自己接收打字：在持有焦点的命中区域上设置 `gfx.HitOptions{TypeAhead: true}`，并在 `config.TextInput` 里为它返回一个 `app.TextInput`（该命中区域的 `context.TypeAhead` 为 true）。它获得焦点时键盘是原始的，和终端一样：可打印字符和输入法输入（包括拼音）以 `app.TextInsert` 事件到达 `OnEvent`，其他按键（方向键、Enter、Escape、Backspace、Tab）作为 `gfx.KeyEvent` 到达 `OnKey`。查询串由应用持有：追加插入的文本，并在 `TextState`（`Composing`、`Composition`）里回传输入法的组合状态，输入法才会在组合期间保留它需要的按键；移动选中项、自己画搜索浮层。把 `TextInput.Bounds` 设在希望输入法绘制组合文本的位置。没有声明的命中区域不受影响，没有设置 `TextInput` 的窗口也不付出任何代价。headless 测试里 `Session.Keys("f")` 会打进这样的命中区域。

`OnShortcut` 用于应用自己拥有的组合键，不用于打字：它早于输入法运行，被它拿走的按键不会进入输入法组合。

`OnModifierTap` 收到单独按下又松开的修饰键：期间没有任何其他按键、输入文本、指针按下或滚动，窗口也一直持有键盘。除了要求整个键盘的终端，`OnKey` 看不到单独的修饰键，所以应用要靠它实现“连按两次 Shift”这类手势。Viva 只上报每一次点按，第二次由应用自己计时，框架内部没有定时器。设置它只会让窗口把修饰键的按下和松开报告给运行时，其他都不变：输入法仍然能看到单独的 Shift，`OnKey` 仍然收不到修饰键事件。要求整个键盘的终端仍会收到修饰键本身。headless 测试里对修饰键调用 `Session.KeyDown` 和 `Session.KeyUp` 会产生点按。

## 滚动

`ui.List` 和 `ui.Scroll` 自己保存偏移量，所以滚轮交给指针下方的滚动区域，页面不需要偏移字段，也不需要 `OnScroll` 处理函数；示例里的滚轮就是列表的。`ListState.ScrollTo`、`ScrollIntoView`、`ScrollToEnd`、`Visible` 和 `AtEnd` 移动和读取列表，`ui.Scroll` 则在同一遍绘制里通过 `ScrollState()` 报告同样的信息。页面滚动的内容会被裁剪；自绘的绘制代码用 `Canvas.Clip` 裁剪自己的内容，避免画到邻居上，被裁掉的命中区域也不再响应。

`OnScroll` 收到页面没有拿走的事件：像素增量，附带窗口位置、修饰键、`Viewport`（当前窗口大小）和指针下方的命中区域，所以你自己的处理函数不依赖之前的某次指针移动。正的增量露出下方的内容。

## 重绘

只有状态确实变了，才从 Config 回调返回 `Repaint: true`。什么都没改变的事件返回零值 `EventResult`，这不会触发重绘。页面的点击会在重新声明页面的那一遍绘制中投递，所以在改变状态的控件之后读取模型状态的绘制会看到新值；把依赖某个变化的内容声明在做出变化的控件之后，就像示例里的详情窗格跟在列表和快捷键后面。
