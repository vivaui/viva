---
title: 处理输入
summary: 指针、键盘、快捷键、滚动和焦点，附带一个可以拖动、打字和滚动的完整示例。
order: 4
group: guides
---

输入以有类型的回调形式出现在 `app.Config` 上。每个回调接收一个事件，返回一个 `ui.EventResult`，说明应用是否使用了这个事件，以及窗口是否需要重绘。下面的示例用到了所有回调：一个带可拖动分隔条的面板、一个可以点击并用方向键移动的列表、一次次按钮按下、一个键盘快捷键和滚轮滚动。布局用的是 `flex`，所以示例只处理页面自己无法声明的部分。

<!-- example: input/main.go -->
```go
package main

import (
	"fmt"
	"log"
	"runtime"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/ui"
)

// Identities are names in a tree, not numbers: the application declares the
// tree once and every control it paints is a name inside a feature of it.
var (
	tree = ui.Tree{}
	page = tree.Feature("mail")
	rows = tree.Sub(page, tree.Name("rows"))

	divider = tree.ID(page, "divider")
	list    = tree.ID(page, "list")
)

var mailboxes = []string{"Inbox", "Drafts", "Sent", "Archive", "Trash"}

type model struct {
	split    flex.SplitterState
	selected int
	palette  bool
	context  string
	view     flex.View
}

// primary is the modifier that means "the command key" on this platform.
func primary() ui.Modifiers {
	if runtime.GOOS == "darwin" {
		return ui.ModifierMeta
	}
	return ui.ModifierControl
}

func newConfig(m *model) app.Config {
	theme := ui.LightTheme()
	config := app.Config{Tree: &tree, Title: "Input", Width: 640, Height: 360}

	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		bounds := ui.Rect{Width: size.Width, Height: size.Height}
		canvas.Fill(bounds, theme.Background)

		m.view.Draw(canvas, theme, bounds, func(f *flex.Frame) {
			f.Row().Fill().Children(func() {
				f.ScrollArea(list).Width(m.split.Pos).Background(theme.Card).Children(func() {
					for i, name := range mailboxes {
						row := f.Row().Pad(8).PadX(12).Hover(flex.Look{}.Background(theme.Hover)).Hit(rows.At(i), name)
						if i == m.selected {
							row.Background(theme.Hover)
						}
						if row.Children(func() { f.Text(name) }).Clicked() {
							m.selected = i
						}
					}
				})
				// The splitter is the draggable divider: it captures the pointer, shows
				// the resize cursor and keeps both panes within their minimum sizes.
				f.Splitter(divider, &m.split).Mins(120, 200)
				f.Column().Grow(1).Pad(16).Gap(8).Children(func() {
					detail := fmt.Sprintf("%s selected", mailboxes[m.selected])
					if m.palette {
						detail = "Command palette"
					}
					f.Text(detail)
					f.Text(m.context).Muted()
				})
			})
		})
	}

	// OnPointer sees presses, releases and moves; Target is the identity under
	// the pointer. A drag may be coalesced to its latest position, so a handler
	// must follow the coordinates, not count moves. Here a secondary press on a
	// row is the start of a context menu.
	config.OnPointer = func(event ui.PointerEvent) ui.EventResult {
		if event.Kind == ui.PointerDown && event.Button == ui.PointerButtonSecondary && event.Target.Scope() == rows {
			row, _ := event.Target.Name().Index()
			m.context = "Context menu for " + mailboxes[row]
			return ui.EventResult{Handled: true, Repaint: true}
		}
		return ui.EventResult{}
	}

	// OnKey is called for the control that has keyboard focus: a row of the list,
	// or the divider, which moves with the arrow keys.
	config.OnKey = func(event ui.KeyEvent) ui.EventResult {
		if result := m.split.Key(event); result.Handled {
			return result
		}
		if event.Target.Scope() != rows {
			return ui.EventResult{}
		}
		switch event.Key {
		case ui.KeyDown:
			m.selected = min(m.selected+1, len(mailboxes)-1)
		case ui.KeyUp:
			m.selected = max(m.selected-1, 0)
		default:
			return ui.EventResult{}
		}
		return ui.EventResult{Handled: true, Repaint: true, Focus: rows.At(m.selected), FocusVisible: true}
	}

	config.OnShortcut = func(event ui.KeyEvent) ui.EventResult {
		if event.Modifiers == primary() && event.Text == "k" {
			m.palette = !m.palette
			return ui.EventResult{Handled: true, Repaint: true}
		}
		return ui.EventResult{}
	}

	// The wheel goes to the scroll area under the pointer.
	config.OnScroll = m.view.Wheel
	return config
}

func main() {
	m := &model{split: flex.SplitterState{Pos: 220}}
	var canvas ui.Canvas
	if err := app.RunCanvas(newConfig(m), &canvas); err != nil {
		log.Fatal(err)
	}
}
```

## 标识

命中区域由 `ui.ID` 标识：一个位置（`ui.Scope`）加上位置内的名字。零值 ID 表示“没有目标”。任何东西都不再手工编号。程序声明一棵 `ui.Tree`，为每个页面或浮层取一个 feature，为每个控件取一个名字；列表是它自己的 scope，行就是 `ui.Index(i)`。

```go
var (
	tree    ui.Tree
	mail    = tree.Feature("mail")
	rows    = tree.Sub(mail, tree.Name("rows"))
	hitSend = mail.ID(tree.Name("send"))   // 一个控件
)

canvas.Hit(rows.ID(ui.Index(row)), bounds) // 绘制时，每行一个 ID

if id.Scope() == rows {                    // 点击到来时
	row, _ := id.Name().Index()
	open(row)
}
```

不同的位置或名字永远不会混淆，因为它们是被驻留（interned）的，而不是哈希或编号，所以不同人写的功能不会冲突，也没有会用尽的区间。`tree.Within(id.Scope(), mail)` 能判断一次点击是否发生在某个页面上，不论多深。用 `Config.Tree` 把这棵树交给窗口，日志和无头驱动里的标识就会以 `mail/rows/#3` 这样的路径显示。命名只发生一次：初始化包级变量时，或列表第一次出现时；绘制一个已存在的 ID 不会分配内存。

## 指针

点击在声明控件的地方处理：`f.Button(...).Clicked()`，或者 `Hit` 盒子上的 `Clicked()`，示例里列表的行就是这样。两个窗格之间的拖动也不需要处理函数：`f.Splitter` 会捕获指针、显示调整大小的光标，并让两个窗格都不小于各自的最小尺寸，示例只需要把第一个窗格的宽度设为 `m.split.Pos`。

`OnPointer` 负责剩下的部分。它收到按下、移动、抬起和离开事件，坐标是逻辑窗口像素，`Target` 是指针下方控件的标识。一次按下会在之后的移动和抬起事件中保持原来的目标，此时 `Captured` 为真，这正是你自己实现的拖动在指针离开控件后仍然有效的原因。示例用它处理行上的次按钮按下，也就是右键菜单的起点。

拖动过程中的移动事件可能被合并为队列里最新的位置（平台原生也会这样处理），因此处理逻辑应当依赖坐标而不是调用次数：按按下点到当前点的位移来拖动的实现，最终仍会停在指针所在的位置。

`Button` 区分主按钮和次按钮。`ClickCount` 为相隔很近的同一按钮连续按下编号，单击是 1，双击的第二次是 2。`HitOptions.Cursor` 设置命中区域被悬停或拖动时的指针形状（`f.Splitter` 已经在用 `PointerCursorResizeHorizontal`）。

## 键盘与焦点

`OnKey` 收到焦点所在命中区域的键盘事件。`Target` 说明是哪一个，所以处理函数在动手前要先检查它：示例检查它是不是某一行，并把分隔条的方向键交给 `m.split.Key`。`Key` 是导航键、编辑键和功能键的名字（`KeyDown`、`KeyEscape`、`KeyEnter`、`KeyTab`……）。对于 Ctrl+C 这样的组合，`Key` 是 `KeyUnknown`，`Text` 是不带修饰键的字符。

任何用 `Hit`、`HitWithLabel` 或 `HitWithOptions` 声明的命中区域都可以获得焦点。Tab 在它们之间移动，Enter 和空格激活获得焦点的按钮，标记了 `SkipFocus` 或 `Disabled` 的会被跳过。焦点由窗口替你跟踪：

- `Canvas.Focused` 返回持有焦点的命中区域，`Canvas.FocusVisible` 只在应该绘制焦点环时才返回它。鼠标点击会让控件获得焦点但不显示焦点环；按 Tab 才会显示。
- `Canvas.Hovered` 和 `Canvas.Pressed` 以同样的方式报告指针状态。

绘制代码读取这些值，而不是自己保存副本，所以悬停、按下和焦点永远不会和运行时的认知不一致。

要自己移动焦点，在 `EventResult` 里返回 `Focus`（以及 `FocusVisible`），或用 `Blur` 清除焦点。

## 快捷键

`OnShortcut` 在原生文本系统看到之前，先收到可打印按键组合，主要是带 Control 或 Command 的。只对应用自己拥有的组合返回 `Handled`：未处理的组合会继续走正常的文本编辑，所以在文本框里 Command-C 仍然是复制。在 Windows 上，获得焦点的普通文本框会先拿走标准编辑与剪贴板组合（复制、剪切、粘贴、全选、撤销、重做、Home/End、按词和按文档移动），终端和 `Placed` 编辑器不会。示例用 `primary()` 辅助函数来比较 `Modifiers`，因为常规的按键在 macOS 上是 Command，在其他平台上是 Control。

## 在列表中打字

输入即搜索的列表、树或表格，以及带单字母命令的控件，需要声明自己接收打字：在持有焦点的命中区域上设置 `ui.HitOptions{TypeAhead: true}`，并在 `config.TextInput` 里为它返回一个 `app.TextInput`（该命中区域的 `context.TypeAhead` 为 true）。它获得焦点时键盘是原始的，和终端一样：可打印字符和输入法输入（包括拼音）以 `app.TextInsert` 事件到达 `OnEvent`，其他按键（方向键、Enter、Escape、Backspace、Tab）作为 `ui.KeyEvent` 到达 `OnKey`。查询串由应用持有：追加插入的文本，并在 `TextState`（`Composing`、`Composition`）里回传输入法的组合状态，输入法才会在组合期间保留它需要的按键；移动选中项、自己画搜索浮层。把 `TextInput.Bounds` 设在希望输入法绘制组合文本的位置。没有声明的命中区域不受影响，没有设置 `TextInput` 的窗口也不付出任何代价。headless 测试里 `Session.Keys("f")` 会打进这样的命中区域。

`OnShortcut` 用于应用自己拥有的组合键，不用于打字：它早于输入法运行，被它拿走的按键不会进入输入法组合。

`OnModifierTap` 收到单独按下又松开的修饰键：期间没有任何其他按键、输入文本、指针按下或滚动，窗口也一直持有键盘。除了要求整个键盘的终端，`OnKey` 看不到单独的修饰键，所以应用要靠它实现“连按两次 Shift”这类手势。Viva 只上报每一次点按，第二次由应用自己计时，框架内部没有定时器。设置它只会让窗口把修饰键的按下和松开报告给运行时，其他都不变：输入法仍然能看到单独的 Shift，`OnKey` 仍然收不到修饰键事件。要求整个键盘的终端仍会收到修饰键本身。headless 测试里对修饰键调用 `Session.KeyDown` 和 `Session.KeyUp` 会产生点按。

## 滚动

`f.ScrollArea(id)` 把偏移量保存在 `flex.View` 里，所以滚动不需要你保存任何状态：`config.OnScroll = m.view.Wheel` 把滚轮交给指针下方的区域。`OnScroll` 收到像素增量，附带窗口位置、修饰键、`Viewport`（当前窗口大小）和指针下方的命中区域，所以你自己的处理函数不依赖之前的某次指针移动。正的增量露出下方的内容。自己滚动的内容要用 `Canvas.Clip` 裁剪，避免它画到邻居上；被裁掉的命中区域也不再响应。

## 重绘

只有状态确实变了才返回 `Repaint: true`。什么都没改变的事件返回零值 `EventResult`，这不会触发重绘。
