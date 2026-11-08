---
title: 组件
summary: 直接使用组件目录，或者把组件的 Go 源码复制进自己的项目并拥有它。
order: 10
group: guides
---

## 目录里有什么

组件目录是对固定版本 shadcn/ui 组件集的静态类型 Go 翻译：按钮与表单、浮层与菜单、日历、表格、图表和反馈界面。它们都是通过 `ui.Canvas` 绘制的普通 Go 函数，没有反射，没有运行时样式解析器，底下也没有浏览器。[组件目录](/zh/ui/button/)展示了每一个组件的运行效果、示例、源码和键盘行为。

## 使用组件

引入 `github.com/vivaui/viva/components`，在绘制时声明组件。这是常规用法，[第一个窗口](/zh/docs/getting-started/)就是这样做的。

```go
components.BeginButton(canvas, theme, hitSave, bounds).Outline("Save")
```

主题是一个值，`ui.LightTheme()` 或 `ui.DarkTheme()`。切换外观就是改用另一个主题绘制。

## 树形导航

`BeginTree` 根据应用自己扁平化的层级绘制可见行。处理键盘事件时，将可见行的实际深度、展开状态和禁用状态传给 `components.NavigateTree`；深度不能使用为了适应宽度而截断的视觉缩进。它返回下一个可见行的索引，或者展开、折叠动作。子节点加载、展开状态、选择和焦点仍由应用负责，不产生隐藏模型或后台工作；导航切片可作为调用方拥有的临时存储复用。

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

想修改组件时，就复制它。`viva list` 列出安装器能复制的配方，`viva add` 把它们作为普通 Go 文件写进你的项目：

```sh
# 先安装 viva（见「构建与打包」）；viva init 生成的项目里用 go tool viva
viva list
viva add --out ./components/ui button card badge
```

| 参数 | 含义 |
| --- | --- |
| `--out` | 写入的目录，默认 `components/ui`。 |
| `--package` | 生成文件的包名，默认 `ui`。 |
| `--force` | 覆盖已存在的文件。 |

复制出来的文件由你修改。它们仍然引入 Viva 的 `ui` 包来使用画布和主题类型，所以依赖 Viva 模块，但没有任何东西在中央注册或更新它们。每个组件都是安装器能复制的配方；一个配方若调用了另一个（例如 `badge` 用到 `button`），会连同它一起复制，保证复制出的这一组能独立编译。这些副本由 `github.com/vivaui/viva/components` 包的同一批源文件生成，所以与它保持一致。导入 `github.com/vivaui/viva/components` 但不调用任何配方，不会链接其中任何内容，因此只有在选项不够用时才需要复制组件。目录中每个组件页都有源码文件的**下载**链接和安装命令。
