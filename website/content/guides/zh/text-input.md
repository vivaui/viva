---
title: 文本输入与表单
summary: 应用持有的 text.Editor 如何接到平台文本系统，附带一个可运行的表单字段。
order: 5
group: guides
---

文本是 Viva 与操作系统必须共享同一份文档的地方：平台的文本服务处理选区、光标、死键和输入法组合，而你的应用需要知道文本是什么。Viva 这样分工，让文本始终归应用所有。

- **你持有文本。** `ui.TextField` 编辑模型里的一个 `string`：用户输入的内容在下一遍绘制时已写进它，你赋给它的值就是输入框显示的内容。输入框的 `text.Editor`（选区、光标、输入法组合）由页面保存。如果你的代码还要读取或移动光标和选区（命令行、查找栏），就在模型里放一个 `ui.Field`：它就是那个编辑器，用 `Text()` 读取、用 `SetText` 修改，`ui.TextField` 同样接受它。
- **平台驱动它。** 文本框获得焦点时，Viva 向窗口的 `Config.TextInput` 提供者询问这个字段对应哪个编辑器，把平台文本服务连到它，并把每个原生事件应用到它上面。页面（`ui.Run`、`ui.NewPage`）就是它所声明的 TextField 的提供者。
- **绘制显示结果。** `ui.TextField` 根据编辑器绘制字段，焦点和光标已替你填好。

<!-- example: textfield/main.go -->
```go
package main

import (
	"log"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

type model struct {
	name  string
	saved string
}

func (m *model) page(c ui.C) {
	ui.Column(c).Fill().Pad(24).Gap(12).Items(ui.Start).Children(func() {
		// m.name is the text: the field writes what is typed into it, and shows
		// what the program puts there. Typing, the caret, selection, the clipboard
		// and input methods need nothing more; the page keeps the field's editor.
		ui.TextField(c, &m.name, "Name", "Your name").Width(320)
		if ui.Button(c, "Save").Clicked() {
			m.saved = m.name
		}
		if m.saved != "" {
			ui.Text(c, "Saved "+m.saved).Muted()
		}
	})
}

func main() {
	m := &model{}
	if err := ui.Run(app.Config{Title: "Form", Width: 480, Height: 240}, m.page); err != nil {
		log.Fatal(err)
	}
}
```

## 提供者

页面的 `Config` 按它给每个输入框起的标识，为上一次绘制声明过的全部 TextField 回答 `TextInput`；其他获得焦点的字段（比如你自己的终端）交给 `app.Config` 原有的提供者。调用 `TextInput` 时会传入一个 `app.TextInputContext`，描述获得焦点的命中区域：它的 ID、边界，以及 `Multiline`、`ReadOnly`、`Secure` 等标志。返回一个 `Editor` 是你自己的 `app.TextInput`，或者返回 `nil` 表示这个字段不做原生编辑。设置了 `Editor` 时，运行时会把它镜像到平台，并把每个原生事件应用到它上面，所以你不需要编写事件处理。如果需要自己过滤或转换编辑，就让 `Editor` 为空，改为设置 `OnEvent`。

字段声明的文本几何信息（边界、字号和颜色）会补全 `TextInput` 没有设置的部分。这正是文本在字段获得焦点时不会跳动的原因。

## 绘制字段

应用自己命名、并且自己持有 editor 的字段写成 `ui.TextFieldID(c, id, &m.searchField, label, placeholder)`，其中 `m.searchField.Bind(&m.search)` 在创建模型时调用一次（`ui.Field` 和其他字段一样放在模型里）：id 是字段在页面 Tree 里的身份，窗口自己的 `TextInput` 路由、初始焦点和测试都用它定位，而 editor 就是应用自己读取和移动的那一个。`.NavigationKeys()` 把 Up、Down、PageUp、PageDown 交给应用而不是移动光标，用于方向键移动列表选择项的搜索框。

`ui.TextField(c, &m.name, label, placeholder)` 直接从字段取得快照，不需要自己构造；`Changed` 报告用户编辑之后的第一次绘制，你自己赋值或调用 `SetText` 不算；`.Secure()` 让它成为密码字段。自己绘制字段的组件则接受 `InputSnapshot`，它是字段某一帧的视图：`Value`、`Placeholder`、`Invalid`、`Disabled`、`ReadOnly` 和 `Secure`，以及 `Focused` 和 `PlatformEditing`。页面的字段会替你设置最后一个。平台编辑器显示在屏幕上时，文本、光标和选区都由它自己绘制，静态标签不能画在它下面。所以 `PlatformEditing` 只在字段获得焦点且有文本时为真；焦点离开后，下一帧会把当前文本重新画成普通标签。示例的测试检查的正是这一点：点击 Save 之后，原生编辑器就消失了。

快照里的选区和组合输入偏移量，是已绘制文本的 UTF-8 字节偏移；平台的文本事件使用的是 rune 索引。两者是不同的东西，不要在需要其中一个的地方传另一个。

## 编辑器提供什么

`ui.Field` 在第一次使用时创建它的编辑器；也可以用 `text.NewEditor(initial)` 手工创建一个。`Insert`、`DeleteBackward`、`DeleteForward`、`MoveLeft` 和 `MoveRight` 用来编辑，`Selection` 和 `SetSelection` 按 rune 索引读取和设置选区，`Composition` 报告输入法组合期间的标记范围。`Restrict(accept, maxRunes)` 限制编辑器接受什么，一次性验证码字段就是这样只接受数字、最多六位。`SetOverwrite` 切换到改写模式。

编辑器还持有有界的撤销历史。macOS 的 Command+Z、Command+Shift+Z，以及 Windows/Linux 的 Control+Z、Control+Y 或 Control+Shift+Z，会在应用没有接管该快捷键时调用 `Undo` 和 `Redo`。连续输入和连续删除会合并；粘贴、换行、替换选区和一次完整的输入法组合各自是独立单元。应用可以用 `BreakUndoGroup` 划分额外边界，用 `ClearHistory` 丢弃保留的历史。直接连接 `Editor` 的密码字段会在每次原生编辑后清空历史，避免为了撤销而继续保留旧密码。终端不使用这套历史，Control+Z 仍会发送给终端中的程序。

`Editor` 不是并发安全的：只在窗口线程上访问它。

## 密码字段

在 `ui.TextField` 上调用 `.Secure()`（或在 `ui.Field` 上设置 `Secure`，或设置 `InputSnapshot.Secure`）。组件为每个字符画一个圆点，永远不会把值暴露给无障碍树，运行时也会遮住平台文本服务绘制的内容。

## 禁用与只读字段

`.Disabled(true)`（或 `.Disabled(条件)`）让字段像其他控件一样不可交互并变暗：它不再是 Tab 停靠点，按下不会给它焦点，也不会为它打开编辑器。`.ReadOnly(true)` 让字段保持可获得焦点并显示值，但没有光标、不能输入、不打开输入法；平台会被告知该命中区域是只读的。两者都可以用 `false` 恢复；模型里的 `ui.Field` 也可以直接设置 `Disabled` 或 `ReadOnly`，`Field`、`FieldBase` 和 `FieldArea` 都会读取它：

```go
ui.TextField(c, &m.id, "Identifier", "").ReadOnly(true)
ui.TextField(c, &m.name, "Name", "Your name").Disabled(m.pending)
```

## 多行与附加元素

`components.PaintInputGroup` 接受同类型的快照，支持 `Multiline`，并提供四个附加槽位 `InlineStart`、`InlineEnd`、`BlockStart` 和 `BlockEnd`，用于图标、文字前缀和按钮。`components.PaintTextarea` 和其他字段建立在同样的部件之上；[组件目录](/zh/ui/input/)里每个都有一页，带可运行的示例。

## 多行文本区域

页面里写多行输入框，和单行一样传它编辑的字段：`ui.TextArea(c, &m.notes, "Notes", "").Height(120)`。回车换行，长行自动折行，行会滚动以跟随光标；页面负责按行移动光标（上、下、Home、End），并把编辑器交给平台。下面说明这些写法背后做了什么，适用于直接用组件搭建的输入框。

`components.PaintTextarea` 默认按框宽软换行（`NoWrap` 关闭）。平台编辑器只认识一行，所以多行字段设置 `HideEditorText`，由组件自己画文字、选区、组合输入下划线和光标。需要换行后的光标行为时，把一个 `components.TextareaLayout` 放在应用里、与编辑器放在一起，并通过 `TextareaSnapshot.Layout` 传给每一帧：

- 它保存换行后的视觉行和滚动位置，稳定状态下绘制零分配；插入点移动时自动滚动到插入点，`ScrollBy` 响应滚轮。
- 方向键、Home、End、翻页键由应用的按键处理转给 `components.TextareaMovementForKey` 和 `layout.Move`，得到的字节偏移再交给编辑器（`Editor.MoveTo(index, extend)`，Shift 扩展选区）。上下移动在一串连续按键里保持目标列；Home/End 到视觉行首和行尾。
- 点击和拖动由运行时按视觉行定位光标（`Layout` 通过命中区域告诉运行时行的位置），输入法候选窗跟随光标所在的行。
- `layout.ContentHeight` 给出随内容增高的框高度（shadcn 的 `field-sizing-content`），应用自己决定上限。

偏移量仍是应用持有文本的 UTF-8 字节偏移。组合输入期间，方向键归输入法所有。

## 各平台验证到什么程度

原生编辑在 macOS 上已实现，包括真实的中文输入法组合、按字形簇的编辑，以及点击、拖动和全选。Windows 有原生编辑器，支持微软拼音的组合输入与候选窗。Linux 上，运行在 IBus（或带 IBus 前端的 Fcitx5）下的输入法可以在文本框中组合，候选由输入法自己的面板绘制。在浏览器里，目录中的 Input 示例接到了同一个编辑器，其余都只是视觉展示。见[平台](/zh/docs/platforms/)。

上面描述的软换行 Textarea 在 macOS 上通过无头驱动验证；Windows 与 Linux 的对应路径（平台不再自行定位光标、输入法窗口跟随已绘制的光标）已经写好，但还没有在真实窗口里运行；浏览器里仍由它自己的文本框处理。
