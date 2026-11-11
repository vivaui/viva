---
title: 声明式布局
summary: 用行、列、内边距和伸缩来描述页面，而不是算坐标；由 ui 包完成布局。
order: 7
group: guides
---

手工切矩形（见“绘制与布局”）是 Viva 最底层的做法，适合自定义控件。对于由普通部件组成的页面（页眉、表单、列表行、页脚），它要做很多算术，标签变长时算术还会出错。`ui` 包按 CSS flexbox 的方式，根据描述来布局页面。

```go
func (m *model) page(c ui.C) {
	ui.Column(c).Pad(24).Gap(12).Children(func() {
		ui.Text(c, "标题").FontSize(18).Bold()
		ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
			ui.Button(c, "取消").Outline()
			if ui.Button(c, "保存").Clicked() {
				model.save()
			}
		})
	})
}
```

## 模型

- `Row` 从左到右排列子元素，`Column` 从上到下，`Stack` 相互叠放。
- 子元素默认和内容一样大，或者按 `Width`、`Height`、`Size` 指定。`FillWidth`、`FillHeight`、`Fill` 占满容器的交叉方向，或者平分沿着容器方向剩下的空间。
- `Grow(n)` 把容器方向的剩余空间按比例分给会伸缩的子元素。`Spacer()` 是会伸缩的空元素。
- `Pad` 和 `Margin` 接收一个数字；`PadX`、`PadY` 设置两边；`PadTop`、`PadRight`、`PadBottom`、`PadLeft` 及其 `Margin` 形式只设置一边；`PadSides`、`MarginSides` 按上、右、下、左的顺序设置四边。后调用的方法覆盖它设置的那些边。`Gap` 是子元素之间的间距。
- 交叉方向上子元素默认拉伸；`Items(ui.Center)` 居中，`Self` 只对一个子元素生效。沿容器方向用 `Justify(ui.Center)`、`ui.End`、`ui.Between` 摆放剩余空间。`Center()` 两个方向都居中。
- `Basis(px)` 是在分配剩余空间之前元素的起始尺寸，`WidthPercent`、`HeightPercent` 取容器的一个比例。`MinWidth`、`MaxWidth`、`MinHeight`、`MaxHeight` 限定尺寸；被上限卡住的元素吃不下的空间会分给其他元素（和 CSS 一样），所以 `Grow(1).MaxWidth(470)` 旁边放一个 `Spacer()`，后面的元素仍然贴着远端。
- `Items(ui.Baseline)` 让一行里的子元素按第一行文字的基线对齐，大标题、小说明和按钮可以落在同一条基线上。`Text` 会先于固定尺寸的兄弟元素收缩并裁剪。
- `Background`、`Radius`、`Border`、`Shadow`、`Clip` 为容器设置样式。在 `Input`、`Select`、`Button` 等控件上，`Radius` 和 `FontSize` 为该控件设置它自己的圆角和字号；`Button` 上的 `Height` 就是按钮画出来的高度。
- `Paragraph(text, maxLines)` 是会按所得宽度换行的文字，所以标签变长会让这一行变高，并把后面的内容往下推。`LineHeight` 设置行高。
- `Grid(minCell)` 按宽度放下尽可能多的等宽列，并换到新的行。
- `Flow()` 是会换行的 Row：子元素保持各自的宽度从左到右排列，放不下下一个时就换到新的一行，和 CSS 的 `flex-wrap` 一样。每一行像 Row 一样分配自己的空间，所以 `Grow` 和 `Justify` 对每一行单独生效，`Items` 在各自的行内对齐子元素。`Gap` 是子元素之间的间距，也是行与行之间的间距，除非用 `LineGap` 另行设置。会换行的标签、徽章和工具栏都用 Flow。
- `Scroll(offset)` 是比自己更高的、带裁剪的列。偏移量由你持有（滚轮事件到 `Config.OnScroll`）；`End` 之后 `Content()` 是子元素的总高度，可据此限制偏移量。

控件就是 `components` 里的那些，外观完全一致：`Button`（以及 `Outline`、`Ghost`、`Secondary`、`Destructive`、`Small` 等变体）、`Checkbox`、`Switch`、`Icon`、`Divider`。`Clicked()` 在声明控件的地方读取：它只在递送这次点击的那一遍里为真。任何元素都可以用 `Hit(id, label)` 成为指针目标；它在指针悬停、按下和获得焦点时的样子用 `Hover`、`Pressed` 和 `Focus` 声明（见下文）。

## 页面：ui.C

页面 API 是同一套布局，再替你做掉页面的簿记。每个元素都是 `ui` 的一个函数，第一个参数是页面的 `c`；容器在一个函数里声明子元素，控件接收它要修改的值。不需要你起任何名字，页面也只需导入 `ui`：颜色、几何和主题都从这里重新导出（`ui.RGB`、`ui.Rect`、`ui.Theme`），底层的 `gfx` 包放画布、输入事件和按键：

```go
func (m *model) page(c ui.C) {
	ui.Column(c).Fill().Pad(24).Gap(12).Children(func() {
		ui.TextField(c, &m.name, "Name", "Your name")
		ui.Switch(c, &m.sync, "Sync over SSH")
		ui.Choice(c, &m.theme, themes, nil)
		if ui.Button(c, "Save").Clicked() {
			m.save()
		}
	})
}

ui.Run(app.Config{Title: "Settings", Width: 480, Height: 320}, m.page)
```

`ui.Choice` 和 `ui.Tabs` 各占一个 Tab 停靠点：左右方向键（垂直的标签页用上下键）移动到下一个可用选项并选中它，到头会回绕；Home 和 End 去第一个和最后一个。

点击会在递送它的那一遍绘制里翻转 `m.sync`：在控件第一次被询问（`Clicked`、`Changed`）时写入，没有被询问就在页面结束时写入；在那之前读 `m.sync` 看到的还是点击前的值，所以写在控件之后的 `Disabled` 依然有效。`Choice`、`Tabs` 和 `Select` 接受任意可比较类型的值，并在声明时就写入：要禁用它们，用 `if` 包起来，而不是在后面写 `Disabled`。`TextField` 不需要 `Config.TextInput`：页面的 `Config` 把它声明过的每个输入框的编辑器交给平台，其他获得焦点的输入框交给配置里原有的提供者。`Popover` 和 `Modal` 在一个 `*bool` 为真时打开，被关闭时把它设为 false（见[对话框、菜单和弹出层](/zh/docs/overlays/)）。`Box.NativeView` 把一个元素的区域交给系统网页引擎的页面。

元素按位置命名：它的种类，以及它在所在容器里同类元素中的序号。另有三条细化：

- 绑定了值的控件以该值的地址命名，值移动时名字跟着走：在密码框上方出现的输入框不会拿走密码框的焦点；过滤列表时，每个复选框的焦点留在它那一行。绑定副本（`v := m.items[k]; ui.Checkbox(c, &v, ...)`）同样可用：它沿用所在位置的名字，调用之后把副本写回即可。
- 在 `c.Key(item.ID)` 或 `c.KeyInt(id)` 里，元素只在这一行内部编号，所以行出现、消失或移动时，行里的按钮跟着这一行走。行消失两遍绘制之后，它的 Key 才会被复用。行里有 `Button` 的循环要给行加 Key；没加时 `viva vet` 会报告。
- 用户按住一个按钮（按下它，或它有焦点）时，如果某一遍绘制里另一个按钮占了它的位置，这次按下不会递送，焦点也会被放弃。

`ui.TextArea(c, &m.notes, label, placeholder)` 是多行输入框。自己写的控件（终端、编辑器、可以画画的画布）用 `ui.Control(c, label, &m.handlers)`：它是一个可获得焦点的空元素，用 `Draw` 绘制，`Handlers` 接收它的按键、指针和滚轮；角色为 `RoleTextField` 或 `RoleTerminal` 时，还通过 `Text` 接收平台的打字和输入法。不用声明任何标识，窗口也不用分派任何事件。有多个可交互部分的控件（评分的几颗星、图的节点）在绘制时声明它们，用 `Handlers.Parts().At(i)` 作标识：落在某个部分上的按键、点击和滚轮，会连同该部分作为事件的 `Target` 交给同一个 `Handlers`；控件有焦点时打出的可打印字符，以 `Text` 交给 `Key`。在行或列上写 `.FocusGroup(ui.FocusHorizontal)`，里面的控件就成为一个 Tab 停靠点，方向键在它们之间移动，像工具栏一样；`.FocusOrder` 可以给自定义网格按行列移动。没有外观的控件有 `ui.ButtonBase`、`CheckboxBase`、`SwitchBase`、`ToggleBase`、`RadioBase`、`TabBase`、`SliderBase`、`SelectBase` 和 `TextFieldBase`：绑定和命名方式与带外观的版本相同，用来做你自己的设计。`ui.TextField(...).AutoFocus()` 在输入框出现时把焦点给它（只给一次，用户移开后不抢回），`.Submitted()` 在递送回车的那一遍绘制里为真；`.Secure()` 让它成为密码字段。`.Tooltip("Delete")` 给控件一个指针停留时显示的提示，它同时也是控件的描述。`ui.Menu(c, anchor, &m.menu, func(menu *ui.MenuItems) { ... })` 是 shadcn 的下拉菜单：像读按钮一样用 `menu.Item("Rename").Clicked()` 读取一项，方向键在各项之间移动，输入字母会跳到以它开头的项，选中一项后菜单关闭。`menu.Sub("Move to", func(to *ui.MenuItems) { ... })` 是一个旁边带子菜单的项：指针在它上面停留、点击它，或按 Right、Enter、Space 时打开（键盘打开时焦点落在子菜单第一项）；Left 返回，Escape 关闭所有菜单，在子菜单里选中一项也会全部关闭。 `menu.Item("Model").Description("What answers")` 给项加一行说明；`menu.WidthRange(140, 320)` 限制菜单宽度，`menu.Above()` 让菜单在锚点上方展开（锚点在面板底部时），`menu.Beside()` 让菜单在锚点右侧展开（列表或弹出层里的行，菜单不遮住下面的行）。`ui.Combobox(c, &m.zone, zones, Zone.String, "Time zone")` 接受 Select 的参数再加一个标签，用户输入时（支持平台输入法）筛选选项。`.Clearable()` 加一个清除选择的按钮，`.Empty("No zone matches")` 是没有匹配项时列表显示的文字，`.Creatable("")` 把输入的内容作为新选项提供（"Create dev"），由 `.Created()` 报告，应用自己把它加进选项。Button 用 `.Icon(plus)` 在标签前加图标，加 `.IconOnly()` 则只显示图标。`ui.ContextMenu(c, row, func(menu *ui.MenuItems) { ... })` 在 `row` 上按下副键时于指针处打开同样的菜单项，按 Shift+F10 时在它下方打开；菜单项可以加 `.Icon`、用 `.Keys(ui.Primary, "c")` 显示快捷键键帽（只显示，快捷键本身是动作所在处的一个 `Shortcut`），用 `.Checked` 或 `.Chosen` 成为复选或单选项。搜索框是带 `.Icon(search)`（文字前的图标）和 `.Action(cross, "Clear")`（末尾的小按钮，用 `.ActionClicked()` 读取）的 TextField。`ui.Drawer(c, &m.open, "Host", ui.DrawerSpec{Side: ui.DrawerLeft, Size: 320}, body, footer)` 从窗口一边滑入一个面板，即 shadcn 的 Sheet：标题为空时整个面板都给 body，`NonModal` 让键盘留在背后的页面；`Swipe` 在面板内侧边缘加一个把手，拖动它面板随之移向边缘，拖过一半或快速甩出时关闭，否则滑回原位。`ui.Tree(c, &m.tree, n, label, content)` 是应用自己的树里可见行组成的 List，由 `TreeState.Node` 描述每一行（深度、是否为分支、是否展开）；方向键移动、展开和折叠，树报告 `Toggled()` 和 `Chosen()` 由应用处理。输入行名的开头会跳到那一行。设置 `TreeState.CanDrop` 后，行可以拖到另一行的前面、里面或后面（由 `CanDrop` 决定接受哪些），`Dropped()` 报告放下的那一次；移动行要放在 `c.After` 里做，因为报告它的那次绘制还在声明这些行。`state.RowID(i)` 是第 i 个可见行的身份（`DisclosureID(i)` 是它箭头的身份，`ListState` 也有 `RowID`），用来给某一行焦点或找到它的 hit。其他元素可以用模型里的 `ui.DragState` 拖放：`ui.Row(c).Hit(ui.ID(c), "Group").DragSource(&m.drag)` 把元素声明为源，`.DropTarget(&m.drag, m.accepts, m.drops)` 把它声明为目标。页面负责按下、把按下变成拖拽的短距离，以及指针下的目标；`m.drag.Dragging()`、`Source()`、`Target()` 和 `At()` 用来绘制拖拽，`accept(source, target)` 决定接不接受，`drop(source, target)` 在松开时执行，Escape 放弃。没有移动的按下仍然是点击。在 `ui.Control` 的 `Draw` 里，自定义网格的卡片用 `d.Canvas.Hit(part, r)` 声明为 part，再用 `d.DragSource(&m.drag, part, r)` 和 `d.DropTarget(&m.drag, part, r, accept, drop)` 声明为同一个状态的源和目标，一个控件就能承载多个身份稳定、可拖拽的 part；目标 part 不需要 Hit。回调保存在模型里，元素每遍绘制都重新声明，不要放在 `Memo` 里。Toast 是模型里的一个 `ui.Toaster`：`m.toaster.Show("Saved", "", ui.ToastSuccess)` 显示一条，`ui.Toasts(c, &m.toaster)` 在页面里声明一次、放在最后，负责绘制并在五秒后收起；它只在下一条到期时请求一帧，没有 toast 时不做任何调度。`ShowAction(title, description, kind, "Undo")` 给 toast 加一个按钮，按下后 toast 收起，在 `Toasts` 之后用 `m.toaster.Acted()` 读取；`ui.ToastLoading` 的 toast 显示转圈且不会自动收起，直到 `m.toaster.Done(id, "Uploaded", "", ui.ToastSuccess)` 替换它；`Toaster.Position` 把整组 toast 放到别的角落或居中（`ui.ToastTopCenter`）。键盘快捷键在它起作用的地方声明，和读取点击一样：`if ui.Shortcut(c, ui.Primary, "s") { m.save() }` 是整个页面的快捷键（Primary 在 macOS 上是 Command，其他平台是 Control）；在容器或控件上用 `list.Shortcut(0, "delete")`，只在焦点位于它之内时生效，嵌套时最内层的生效。不带 Primary、Ctrl 或 Alt 的按键留给正在输入的输入框或终端；Popover、Modal、Menu 或 Select、Combobox 列表打开时，页面级快捷键不生效。`ui.List(c, &state, n, row)` 只声明 `n` 行列表里显示在屏幕上的行，每行高度由内容决定，并负责滚动；`row` 得到一个 `ui.ListRow`，其中 `Element` 是这一行的容器（用来设置样式），`C` 用来声明行里的内容，`ListState.Key` 按条目 ID 为行命名。点击会选中一行，方向键、Home、End、Page Up 和 Page Down 在行之间移动，选中的行在 `state.Selected()` 里，`state.Changed()` 报告用户做了选择；回车或双击是 `state.Submitted()`；设置 `ListState.Label` 后，输入行文字的开头会跳到那一行。把 `ListState.Selection` 设为一个 `ui.Selection` 就能让用户选多行（Primary 加点击增减一行，Shift 扩展，Primary+A 全选）。`ScrollTo`、`ScrollIntoView`、`ScrollToEnd`、`Visible` 和 `AtEnd` 移动并读取滚动位置，`box.ClickModifiers()` 和 `box.DoubleClicked()` 说明任何元素上的点击是怎么发生的。行的 `Element` 是一个 Row，所以图标、名称和大小并排放。`ListState.Header` 标出作为分组标题的行：处在顶部的那一组的标题会留在顶部，它的行从下面滚过，直到下一个标题把它顶走（给它设个背景；标题不会被选中、聚焦，也不参与输入跳转，按键会跳过它）。`ListState.Reorder` 让用户把行拖到别的位置，拖动的行被选中时拖走所有选中的行，并用这些行和它们要放到哪一行之前来调用它；列表会显示它们要去的位置，靠近边缘时自动滚动，按 Escape 放弃拖动。`ListState.RowWidth` 让行至少这么宽：行比列表宽时列表也能横向滚动（水平滚轮，或按住 Shift 用滚轮），不比列表宽时行会撑满列表，像表格的列那样；列表上方的表头用 `box.ScrollWith(&state)` 加在它的内容上，随行一起移动：给它和行一样的宽度，放在定高、会裁剪的容器里。`ui.State[T](c, "open")` 是元素自己的小状态（比如菜单是否打开），在它的容器里按名字查找，容器被声明期间一直保留。`ui.Memo(c, *row, fn)` 在那个值比较相等时从上一遍绘制复制 `fn` 声明的部分，在它被悬停、获得焦点、处于动画中或改变大小时重新声明；见下面的开销。

设置类结构体的表单可以自动生成。给字段加 tag，然后运行 `viva form`（写成 `//go:generate go tool viva form`）：它生成 `viva_form.go`，里面是 `func (v *Settings) Form(c ui.C) ui.Box`，就是普通的页面代码，不用反射：

```go
type Settings struct {
	Name   string         `form:"Name" placeholder:"Your name"`
	Sync   bool             `form:"Sync over SSH"`            // Switch；写 "Sync,checkbox" 则为 Checkbox
	Theme  Theme            `form:"Theme" choices:"themes,Theme.String"`
	Size   string           `form:"Size,select" choices:"sizes"`
	Volume ui.SliderState `form:"Volume"`
}
```

`choices` 指定一个包级切片；选项不是字符串时，再指定一个为选项命名的函数。没有 `form` tag 的字段不生成任何东西；给它不认识的类型加 tag 会报错，并列出它支持的类型。

`examples/guides/page` 的 fleet 页面是一个完整的程序：一个过滤控件、一个可滚动的主机列表（每台主机一个绑定到它的复选框）、一个进度条和一个按钮，没有声明任何一个标识。

<!-- example: page/main.go -->
```go
// Fleet check written as a page: a filter, a list of hosts with checkboxes, a
// progress bar and a button, written without naming a control.
//
// Each checkbox writes into the host it shows, and is named by that host's field,
// so filtering the list moves no focus or click from one host to another.
package main

import (
	"log"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

type host struct {
	name    string
	addr    string
	item    string
	handled bool
}

type model struct {
	hosts  []host
	filter string
	scans  int
}

var filters = []string{"All", "Open", "Handled"}

func newModel() *model {
	return &model{filter: "All", hosts: []host{
		{name: "edge-01", addr: "10.0.4.11", item: "Disable password auth"},
		{name: "edge-02", addr: "10.0.4.12", item: "Rotate host key"},
		{name: "build-01", addr: "10.0.8.3", item: "Pin to ed25519"},
		{name: "build-02", addr: "10.0.8.4", item: "Restrict agent forwarding", handled: true},
		{name: "db-primary", addr: "10.0.9.2", item: "Close port 22 to the world"},
		{name: "db-replica", addr: "10.0.9.3", item: "Enable key-only login", handled: true},
		{name: "jump-01", addr: "10.0.1.5", item: "Set a login grace time"},
		{name: "nas-01", addr: "10.0.12.7", item: "Upgrade OpenSSH"},
	}}
}

func (m *model) matches(h *host) bool {
	switch m.filter {
	case "Open":
		return !h.handled
	case "Handled":
		return h.handled
	}
	return true
}

func (m *model) handled() int {
	n := 0
	for i := range m.hosts {
		if m.hosts[i].handled {
			n++
		}
	}
	return n
}

func (m *model) page(c ui.C) {
	ui.Column(c).Fill().Pad(22).Gap(14).Children(func() {
		ui.Text(c, "Fleet check").FontSize(26).Bold()
		ui.Text(c, "SSH posture across the hosts this workstation reaches.").FontSize(13).Muted()
		ui.Choice(c, &m.filter, filters, nil)

		// The list takes the room left over, so the footer stays on screen.
		ui.Scroll(c).Grow(1).Gap(8).Children(func() {
			for i := range m.hosts {
				if h := &m.hosts[i]; m.matches(h) {
					m.row(c, h)
				}
			}
		})

		handled, total := m.handled(), len(m.hosts)
		ui.Progress(c, float32(handled)/float32(max(total, 1))).FillWidth()
		ui.Row(c).FillWidth().Gap(10).Items(ui.Center).Children(func() {
			ui.Textf(c, "%d of %d handled", handled, total).FontSize(12).Muted().Mono()
			ui.Spacer(c)
			ui.Textf(c, "scanned %dx", m.scans).FontSize(12).Muted().Mono()
			if ui.Button(c, "Scan again").Clicked() {
				m.scans++
			}
		})
	})
}

// row is one host: a component of the page's own, written as Viva's elements are.
func (m *model) row(c ui.C, h *host) {
	theme := c.Theme()
	ui.Row(c).FillWidth().Pad(12).Gap(12).Radius(10).Items(ui.Center).Background(theme.Card).Children(func() {
		if h.handled {
			ui.Text(c, "ok").FontSize(11).Color(theme.Success)
		} else {
			ui.Text(c, "open").FontSize(11).Color(theme.Danger)
		}
		ui.Column(c).Grow(1).Gap(2).Children(func() {
			ui.Row(c).Gap(8).Items(ui.Center).Children(func() {
				ui.Text(c, h.name).FontSize(14).Bold()
				ui.Text(c, h.addr).FontSize(12).Muted().Mono()
			})
			item := ui.Text(c, h.item).FontSize(12)
			if h.handled {
				item.Muted()
			}
		})
		// The checkbox writes into the host; the bar above derives itself from the
		// hosts on the next paint.
		ui.Checkbox(c, &h.handled, "").Label(h.name)
	})
}

func main() {
	if err := ui.Run(app.Config{Title: "Viva fleet", Width: 460, Height: 560}, newModel().page); err != nil {
		log.Fatal(err)
	}
}
```

`ui.Scroll(c)` 的偏移量由区域自己的名字保存；在它（或 `ui.List`、`ui.Tree` 返回的区域）上调用 `.ScrollState()`，可读取上一次布局时的 `Offset`、`Content`、`Viewport`，也可以用 `ScrollTo`、`ScrollToEnd` 用代码滚动，比如把选中行滚进视野。每次绘制都重新取，不要保存指针：某次绘制没有声明的区域会忘掉它的偏移量。

`.Sideways()` 让区域也能左右滚动：子元素保持各自想要的宽度，最宽的那个决定内容有多宽，所有子元素都按这个宽度排布，区域更宽时按区域的宽度，所以一排标签保持各自的宽度，高亮的行横跨整个宽度。`Paragraph` 在这里不换行。和高度一样，区域的宽度是给它的宽度（在 `Row` 里需要 `Grow`），而不是内容的宽度。水平滚轮，或者按住 Shift 的滚轮，会横向移动它；状态里有 `OffsetX`、`ContentWidth`、`ViewportWidth`、`MaxX` 和 `ScrollToX`。`.Scrollbars()` 在内容溢出的边缘上画细滚动条，指针可以拖动；在滑块旁边按下，滑块会移到那里。

```go
ui.Scroll(c).Grow(1).Sideways().Scrollbars().Children(func() {
	for _, line := range m.lines {
		ui.Text(c, line).Mono()
	}
})
```

自定义布局用 `ui.CustomLayout(c, &m.masonry)`：容器里的流动子元素由 model 中的 `ui.Layout` 测量和摆放。`MeasureWidth`、`MeasureHeight` 说明内容想要多宽多高，`PlaceWidth`、`PlaceHeight` 给每个子元素尺寸和位置。页面负责 identity、焦点、命中、裁剪和绝对定位子元素；布局拥有子元素的尺寸和位置。`PlaceHeight` 还可以在知道高度后横向移动子元素（masonry 就在这里分列）；页面会一起移动子元素的子树，并按新锚点重新放置它的 popup。滚动和虚拟化请包在 `Scroll`/`List`/`Tree` 里，不要声明在 `Memo` 里。

`c.Bounds()` 和 `c.Canvas()` 给出页面的矩形和画布，用来量文本或组件、读指针和焦点状态；声明和绘制仍走页面。`c.Part("word")` 给下一个元素起一个你自己的名字，测试和无头驱动可以用它。一个应用窗口就是一个 page：`ui.Run` 或 `Page.Config` 接好输入，窗格是同一个 page 里的 keyed 容器，不是第二个 `Page`。当 `Modal` 自动生成的 ID 不合适时，用 `ui.Dialog(c, ui.DialogSpec{...})` 声明由应用自己命名目标的对话框。应用代码不触碰 Page 下面的引擎；组件作者有自己的文档。

`examples/guides/page` 的 fleet 页面（一个选择控件、一个包含八行的滚动区域、每行一个复选框、一个进度条和一个按钮）在 Apple M2 Pro 上声明、布局并绘制用时 9.8 µs，预热后不分配；一千个带 Key 的行是 0.75 ms。两百行的列表每行套一个 `Memo` 后从 271 µs 降到 87 µs，整个列表套一个是 37 µs；复制保留的是上一遍绘制的命令，这两百行约占 0.5 MB（`go test ./ui -bench 'Fleet$|Rows1000|Rows200'`）。

## 状态与效果

有标识的元素在指针悬停、被按住、键盘聚焦时会呈现不同的样子。每个状态是一个 `Look`：只列出发生变化的部分的值，用与元素本身相同的词来构造（`Background`、`Border`、`Color`、`Ring`、`Shadow`、`Opacity`）。

```go
ui.Row(c).Pad(8).Radius(8).Hit(ui.ID(c), "Open").Transition().
	Hover(ui.Look{}.Background(theme.Hover)).
	Pressed(ui.Look{}.Background(theme.Muted)).
	Focus(ui.Look{}.Ring(3, theme.Ring))
```

`Focus` 在焦点应当可见时生效（由按键移动而来，点击不算）；`Hit` 默认可获得焦点，除非用 `HitWith` 指定 `SkipFocus`。几个 Look 按 Focus、Hover、Pressed 的顺序逐字段合并，所以既聚焦又悬停的行会同时显示前者的光环和后者的底色。`Transition()` 按主题的动效在它们之间渐变（`TransitionWith` 可指定其他动效）；没有变化时不会安排任何帧。`StateFrom(row)` 让一个元素跟随另一个元素的状态，行内的内容就能随行变化：`ui.Icon(c, edit).Opacity(0).StateFrom(row).Hover(ui.Look{}.Opacity(1))` 是一个在所在行悬停时才出现的图标。

另有四种不依赖状态的效果。`Ring(width, color)` 在元素外侧画一圈线且不占空间，焦点环就是这样画的。`Opacity(v)` 让元素及其内部的一切（包括控件）淡化；取 0 时仍参与布局、仍可点击；`Image` 不会被淡化。`Gradient(angle, from, to)` 和 `GradientStops(angle, stops...)` 沿圆角用线性渐变填充元素，画在 `Background` 之上、`Border` 之下；角度与 CSS 相同，以向上为 0 顺时针计，`ui.ToRight`、`ui.ToBottomRight` 等给常用方向命名。

## 表单字段与覆盖层

`ui.FormField(c, &m.proxyField, "Proxy").FillWidth().Children(func() { ... })` 是一个表单行：标题，以及 children 声明的唯一控件；控件落在 `components.BeginField` 的 control 槽位，节奏也相同。`ui.FormFieldState` 和值一起放在 model 里；字段不绑定值，也不给控件做 label。放在固定高度的行里时它填满该行；单独放置时和 `BeginField` 预留的高度一致（标题、控件，以及 description 和 error 两行）。Select 或 Combobox 的列表可以越出字段边界。

`ui.Overlay(c, ui.OverlaySpec{Band: ui.LayerPaneChrome, Bounds: r})` 是应用自己的覆盖层：一个浮动的页面区域，在页面之后绘制，层带低于 Viva 的菜单和对话框，没有 dismissal、backdrop 或 focus trap。`ui.LayerPaneChrome` 用于工作区之上的界面框架；`ui.LayerDrag` 用于拖拽幽灵或放置指示器。它的 children 是普通页面元素，命中与 Tab 都按层带处理。空白表面默认让按下事件穿透；要让整块区域挡住下层，在它上面声明一个 `Canvas.Block`（Block 只挡按下事件，滚轮需要应用自己处理）。同一层带上重叠的两个覆盖层可能让形状和文字交错，请用不同层带或避免重叠。

```go
ui.Overlay(c, ui.OverlaySpec{Band: ui.LayerPaneChrome, Bounds: sidebar}).Children(func() {
	ui.Slot(c).Fill().Draw(func(d *ui.DrawContext) { d.Canvas.Block(d.Rect) })
	ui.Column(c).Fill().Children(func() { /* rows */ })
})
```

## 和手工绘制混用

`ui.Slot(c)` 为你自己绘制的内容预留一块矩形，`Box.Rect()` 告诉你它最后落在哪里；用 `Draw` 回调画它，需要输入时用 `ui.Control`。需要页面声明之下的绘制时，用页面的画布（`c.Canvas()`）和它给出的矩形。

## 开销

声明、测量、排布并绘制一个约六十个元素的页面大约需要 8 微秒，页面见过最大页面之后不再分配内存。为每个窗口在你的状态里保存一个 `ui.Page`；它的配置零值即可使用。传给 `Children` 的函数立即调用且不会被保存，所以也没有开销。

单行 `Text` 不换行：放不下的行会被裁剪，并且先于固定尺寸的兄弟元素收缩。需要换行的文字用 `Paragraph`。

## 创建项目

`viva init my-app` 会写出一个能直接运行的项目：用 `ui` 布局的窗口、无需窗口即可驱动它的测试、`viva.toml`，以及 `AGENTS.md`，一页纸的布局、标识、点击和测试指南，供编码助手在修改界面前阅读。之后 `viva dev .` 以热重载方式运行。

<!-- example: layout/main.go -->
```go
package main

import (
	"log"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

type model struct {
	saved bool
	notes bool
}

func (m *model) page(c ui.C) {
	ui.Column(c).Fill().Pad(24).Gap(12).Children(func() {
		ui.Text(c, "Deploy settings").FontSize(18).Bold()
		ui.Text(c, "Where the build goes when it passes.").Muted()
		ui.Divider(c)
		ui.Row(c).Items(ui.Center).Gap(8).Children(func() {
			ui.Text(c, "Notify the team")
			ui.Spacer(c)
			ui.Switch(c, &m.notes, "Notify the team")
		})
		ui.Spacer(c)
		ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
			ui.Button(c, "Cancel").Outline()
			if ui.Button(c, "Save").Clicked() {
				m.saved = true
			}
		})
	})
}

func newConfig(m *model) app.Config {
	theme := gfx.LightTheme()
	page := ui.NewPage(ui.PageConfig{View: m.page, Theme: &theme})
	return page.Config(app.Config{Title: "Layout", Width: 420, Height: 220})
}

func main() {
	var canvas gfx.Canvas
	if err := app.RunCanvas(newConfig(&model{}), &canvas); err != nil {
		log.Fatal(err)
	}
}
```
