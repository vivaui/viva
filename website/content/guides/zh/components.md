---
title: 组件
summary: 直接使用组件目录，或者把组件的 Go 源码复制进自己的项目并拥有它。
order: 10
group: guides
---

## 目录里有什么

组件目录是对固定版本 shadcn/ui 组件集的静态类型 Go 翻译：按钮与表单、浮层与菜单、日历、表格、图表和反馈界面。它们都是通过 `gfx.Canvas` 绘制的普通 Go 函数，没有反射，没有运行时样式解析器，底下也没有浏览器。[组件目录](/zh/ui/button/)展示了每一个组件的运行效果、示例、源码和键盘行为。

## 使用组件

页面用 `ui` 包编写：`ui.Button`、`ui.Checkbox`、`ui.Select` 等控件内部就是用这些组件绘制的，[第一个窗口](/zh/docs/getting-started/)就是这样做的。只有在手工向 `gfx.Canvas` 绘制，或要安装组件源码来修改时，才需要自己引入 `github.com/vivaui/viva/components`。

```go
components.BeginButton(canvas, theme, hitSave, bounds).Outline("Save")
```

主题是一个值，`gfx.LightTheme()` 或 `gfx.DarkTheme()`。切换外观就是改用另一个主题绘制。

`Begin*` 是样式层的构造器，不是第二套页面 API。页面控件用 `ui.*`、`ui.*Base` 和 `ui.Control` 声明；`components` 包里的 `Begin*` 用于 `Draw` 内部、纯测量或 `viva add` 复制出的源码。“零 Begin”不是目标：边界是页面行为不再手拼，而不是绘制构造器消失。`ui/multiselect`、`ui/hierarchypicker` 这类可复用的重型复合控件走 opt-in 包。

## 可选页面包

重功能或有状态的页面控件放在 `ui/<能力名>` 的独立包里。用到才 import；包本身就是按需付费的边界。

- 编辑器与可视化：`ui/codeeditor`、`ui/textview`、`ui/chart`、`ui/markdown`、`ui/image`、`ui/calendar`、`ui/carousel`、`ui/columnresize`、`ui/reordergrid`
- 导航与浮层：`ui/command`、`ui/menubar`、`ui/navigationmenu`、`ui/sidebar`、`ui/hovercard`
- 数据与输入：`ui/avatar`、`ui/avatareditor`、`ui/attachment`、`ui/metriccard`、`ui/marker`、`ui/skeleton`、`ui/aspectratio`、`ui/questionnaire`、`ui/messagescroller`、`ui/message`、`ui/inputgroup`、`ui/inputotp`、`ui/multiselect`、`ui/hierarchypicker`、`ui/buttongroup`、`ui/togglegroup`、`ui/steps`、`ui/descriptions`、`ui/timeline`、`ui/rate`、`ui/inputnumber`、`ui/datepicker`、`ui/timepicker`、`ui/transfer`、`ui/alert`、`ui/tag`、`ui/breadcrumb`、`ui/card`、`ui/empty`、`ui/result`、`ui/pagination`、`ui/editabletabs`、`ui/popconfirm`、`ui/divider`、`ui/typography`, `ui/validate`, `ui/upload`

`ui/textview` 用来显示可选择、可复制的只读文本，例如日志和进程输出。在 model 中保留 `textview.View`，调用 `m.output.Declare(c, textview.Spec{Source: lines, Scroll: true})`；`textview.Strings` 可以把字符串切片适配成数据源。调用方拥有文本、样式和动作，视图负责选择与绘制，不带编辑或撤销模型。

包名是能力名，没有 `view` 后缀；状态类型按角色命名，不重复包名：`calendar.State`、`message.Row`、`attachment.Card`。局部变量避让包名（`cal`、`msg`、`pic`）；model 字段可以保留领域名词（`m.calendar`），因为选择器不会遮蔽包。

有些包暴露 `pkg.View(c, label, &state)`（`calendar.View`、`markdown.View`），有些则是状态上的方法（`avatar.New().View(c, label)`、`questionnaire.New().View(c, label)`）。用 `go doc github.com/vivaui/viva/ui/<package>` 查看具体形式。

```go
cal := &m.calendar
calendar.View(c, "Calendar", cal).FillWidth()
msg := message.New().Name("Ari").Body("Ready")
message.View(c, "Ari", msg)
```

保留自己布局的自定义表面可以在 `ui.Slot` 里、按组件原来的矩形放一个页面控件：

```go
slot := ui.Slot(c).Size(22, 24).Shrink(0)
slot.Children(func() {
    ui.Checkbox(c, &copy, "").Absolute().Left(0).Top(0).Size(16, 16).Hit(id, "Select")
})
```

应用自己的 dispatcher 管理真实值时，绑定一个副本。表面其余部分继续用 `Draw`/canvas；不要为了统一把整个表面改成页面声明。`ui/slot_parity_test.go` 保证 Checkbox、Switch、FormField 这条路径与 `components` 包里的 `Begin*` 的像素和 hit 一致。

## 树形导航

`BeginTree` 根据应用自己扁平化的层级绘制可见行。处理键盘事件时，将可见行的实际深度、展开状态和禁用状态传给 `ui.NavigateTree`；深度不能使用为了适应宽度而截断的视觉缩进。它返回下一个可见行的索引，或者展开、折叠动作。子节点加载、展开状态、选择和焦点仍由应用负责，不产生隐藏模型或后台工作；导航切片可作为调用方拥有的临时存储复用。

## 图表

`PaintAreaChart` 和 `PaintBarChart` 用类型化切片绘制完整的图表：图例、坐标轴、网格、序列、悬停标记和提示框。没有字段映射，也没有反射：`Values[k]` 是 `Series[k]` 的 Y 值，每个点一个，`NaN` 表示断点。

```go
chart := components.AreaChart{
    Series: series,                       // 标签和颜色，图例与提示框共用
    Values: [][]float32{desktop, mobile}, // 每个序列一个切片
    Curve:  components.ChartCurveMonotone,
    XAxis:  components.ChartAxis{Format: formatMonth},
    Grid:   components.ChartGridHorizontal,
}
plot := components.PaintAreaChart(canvas, theme, bounds, chartID, chart, &state)
```

图表自己不处理输入。把 `components.ChartState` 和数据放在一起，由你自己的事件驱动它：指针移动时 `state.Hover(plot.IndexAt(event.X))`，移出时 `state.Clear()`，方向键用 `components.ChartStepForKey`。状态预热之后，稳定的图表每次绘制不分配内存；没人操作的图表也不会调度帧。

有两条限制是有意为之。图表会画出给它的每一个点，所以很长的序列应当在数据变化时用 `components.ChartDownsample` 降采样一次，而不是每次绘制都做。堆叠的序列不能为负：负值按零堆叠。

## 拥有一份副本

想修改组件时，就复制它。`viva list` 列出安装器能复制的配方，`viva add` 把它们作为普通 Go 文件写进你的项目。复制出来的是绘制配方：它在你给定的矩形里往 `gfx.Canvas` 上画（`BeginButton(canvas, theme, id, bounds)`），所以要在页面元素的 `Draw` 回调里或整窗的 `Paint` 里使用，外观就是你要改的那段代码。想改变所有 `ui.Button` 的外观又不想复制任何东西，给页面设置 `PageConfig.Styles`；想基于 `ui` 自带的行为做自己外观的控件，用 `ui.ButtonBase` 等 Base。

```sh
# 先安装 viva（见「构建与打包」）；viva init 生成的项目里用 go tool viva
viva list
viva add --out ./components button card badge
```

| 参数 | 含义 |
| --- | --- |
| `--out` | 写入的目录，默认 `components`。 |
| `--package` | 生成文件的包名，默认 `components`。 |
| `--force` | 覆盖已存在的文件。 |

复制出来的文件由你修改。它们仍然引入 Viva 的 `gfx` 包来使用画布和主题类型，所以依赖 Viva 模块，但没有任何东西在中央注册或更新它们。每个组件都是安装器能复制的配方；一个配方若调用了另一个（例如 `badge` 用到 `button`），会连同它一起复制，保证复制出的这一组能独立编译。这些副本由 `github.com/vivaui/viva/components` 包的同一批源文件生成，所以与它保持一致。导入 `github.com/vivaui/viva/components` 但不调用任何配方，不会链接其中任何内容，因此只有在选项不够用时才需要复制组件。目录中每个组件页都有源码文件的**下载**链接和安装命令。
