# 同源组件示例

`Catalog()` 返回固定目录及其 `Examples`；所有返回的 slice 均只读，调用者不得修改。
每个组件的完整示例（目录、交互和绘制）位于同名文件，直接使用 `github.com/vivaui/viva/components`。
`state.go` 仅负责选择、主题、背景和分派，不包含组件配方。

## 运行器合同

- 从 URL 或 flags 取得 component/example/theme，调用 `New(componentID, exampleID)`；
  空 example 选择首个，未知 component/example 返回错误，不隐式回退。
- 以 `SetDark(bool)` 设置主题、`SetDensity(gfx.Density)` 设置控件密度（默认 Regular）；切换示例或重置使用新的 `State`。
- 展示页（`internal/gallery`）为每个示例准备独立 `gfx.Canvas`，用 `Canvas.BeginEmbedded` / `Embed` 嵌入页面；
  示例按自己的坐标与自己的身份绘制，页面把它挂载在自己的 scope 上（`gfx.Tree.Mount`），
  同一示例在多个槽位里得到互不相同的身份，见 `gfx.Tree` 的身份说明。
- 所有示例共用一棵身份树：`componentdemo.Tree()`。独立运行器把它设为 `app.Config.Tree`；
  展示页在同一棵树里命名自己的控件。`State.Paint` 也会把它设给画布，因此裸画布同样可用。
- 运行器清空 Canvas，并以单调时间调用 `BeginFrame` / `EndFrame` 包围 `Paint`。
  `Paint` 不清空宿主 Canvas，不创建 timer/goroutine，也不自行调度帧。
- 点击在绘制时读取（`Clicked()`）：运行器在点击完成后重绘一次作为事件遍历，嵌入的画布
  把点击还原成示例自己的身份。`pointer` / `key` / `scroll` 仍是带身份目标的类型化事件回调。
  运行器沿用 Canvas focus scope、Escape target 与焦点恢复；
  dialog 不响应遮罩点击，避免把内容空白区域误当作取消。
- Web 与原生运行器都调用 `EnableTextEditing()`，并把各自的平台文本服务连接到同一个应用持有的 Go Editor。
  provider 用 `Editor(context.ID)` 解析编辑器，应用文本、选区和 IME 事件后通知宿主重绘。
  按生产 Input 的约定，编辑区为 hit 水平内缩 12px，字号为主题 `FontSizeM`。
- 只有当前 case 的可编辑 input 返回 Editor；disabled/read-only、其他组件、未知 ID
  及未启用平台编辑均返回 `(nil, false)`。不要求焦点已经建立，方便 provider 解析。
- Web 与原生运行器都显式启用编辑；焦点下非空文本交给当前平台绘制，空值保留
  Canvas placeholder，失焦时恢复同一 Go Editor 的静态文本快照。

状态及 Editor 由运行器在 UI 所有者线程同步使用，不支持并发修改。状态只保留计数、选择、
开关、一个按需 Editor 和弹窗结果，不保存 Canvas 或历史帧；New 是唯一重置边界。
`State` 另持有一个 `*ui.Page`（`drawPage` 的布局、身份与绘制内存，首次绘制时由 `ui.NewPage` 建立，页面热起来后绘制不再分配）。
文本缓冲区的生命周期和内存由现有 `text.Editor` 管理，本包不新增池或后台任务。

## 如何写一个示例

控件身份是树里的名字，点击在声明控件的地方读取；页面部分用 `ui.Page` 的 `C` 声明（`drawPage`），组件插槽仍由示例自己的身份绘制。

1. **身份**：在示例文件里为示例声明一个 feature scope，为每组控件声明一个 list scope 或一个具名 ID
   （包级变量，启动时创建一次，绝不在 `Paint` 里创建）。`tree` 是包内共享的身份树（`ids.go`）。
2. **点击**：在 `paintXxx` 里，控件声明处直接 `if control.Clicked() { ... }`
   （页面的 `Box.Clicked()`、组件绘制器的 `Paint().Clicked()`，或 `canvas.Clicked(id)` /
   `canvas.ClickedIn(list)`）。事件遍历是唯一允许改状态的绘制；运行时从不向禁用控件投递点击。
3. **布局**：用 `state.drawPage(canvas, theme, size, maxWidth, title, func(c ui.C) {...})`
   声明标题与内容（居中、最大宽度、16px 间距），控件用页面的 `ui.Button`/`Checkbox`/`Switch`/`TextField`/
   `Choice`/`Row`/`Column`/`Text`。没有 `ui` 元素的组件（例如 Invalid 复选框、Badge）用 `ui.Slot(c).Size(w, h)`
   预留位置，`drawPage` 返回后按 `Box.Rect()` 调用组件的绘制函数。只有真正的绘制几何才手算矩形。
4. **可编辑字段**：`editableField.ID` 使用树里的 ID（`inputEditable` 那样的包级变量）；
   `Editor(id)` 与 `fieldID` 比较的是示例自己的身份，展示页已经用 `Tree.Local` 还原过。
5. **测试**：用 `click(&canvas, state, id)`（`state_test.go`）驱动：它把 `id` 作为事件遍历的点击，
   再用 `paintFrame` 看结果。`findHit` 与 `canvas.Hits()` 按树 ID 查找控件。
   展示页的测试按 Label 找控件，不引用示例私有的 ID。

示例（`switch.go`，节选）：

```go
var (
	switchUI     = tree.Feature("switch")
	switchToggle = tree.Sub(switchUI, tree.Name("toggle")) // 第 i 个开关：switchToggle.At(i)
)

func (state *State) paintSwitch(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	state.drawPage(canvas, theme, size, 480, "Switch · 调用方持有状态", func(c ui.C) {
		for index, label := range labels {
			checked := state.switches.checked&(1<<index) != 0
			disabled := state.exampleID == "states" && index > 0
			if disabled {
				checked = index == 2
			}
			state.switches.toggles[index] = checked
			ui.Row(c).Gap(12).Items(ui.Center).PadX(12).MarginY(4).Children(func() {
				control := ui.Switch(c, &state.switches.toggles[index], "").Hit(switchToggle.At(index), label).Disabled(disabled)
				if state.exampleID == "sizes" && index == 1 {
					control.Small()
				}
				if control.Clicked() {
					state.switches.checked ^= 1 << index
				}
				ui.Text(c, label).FontSize(theme.Metrics.FontSizeM)
			})
		}
	})
}
```

### 各示例保留组件绘制器的原因

- accordion（组 A）：`accordionItem.At(i)`，`canvas.Clicked`，Slot 内由 `components.Accordion` 绘制
- alert（组 A）：Slot 内由 `components.Alert` 绘制
- alert-dialog（组 A）：树身份，`canvas.Clicked`，打开按钮用 flex Button，模态本体仍由 `components.AlertDialog` 铺满视口
- aspect-ratio（组 A）：Slot 尺寸由 `components.AspectRatio` 给出
- attachment（组 A）：`attachmentItem.At(i)`，`canvas.Clicked`
- avatar（组 A）：Slot 内由 `components.Avatar` / `AvatarGroup` 绘制
- breadcrumb（组 A）：`breadcrumbLink.At(i)`，`canvas.ClickedIn`
- bubble（组 A）：`bubbleReaction`，`canvas.Clicked`
- button-group（组 A）：`buttonGroupItem.At(i)`，`canvas.ClickedIn`
- calendar（组 A）：`calendarPrevious`/`calendarNext`/`calendarDay.At(i)`，整页是一个 Slot
- card（组 A）：纯 flex 列（背景、描边、圆角与 `PaintCard` 一致）
- carousel（组 A）：`carouselPrevious`/`carouselNext`，Slot 留出按钮的外边距
- chart（组 A）：Slot 内是真正的绘图几何
- collapsible（组 A）：`collapsibleTrigger`，`canvas.Clicked`
- combobox（组 A）：树身份，`canvas.Clicked`/`ClickedIn`，弹层相对输入 Slot 定位

第三组（group C）逐个转换，带 `canvas.Clicked` / `ClickedIn` 与 `page`；括号里是它保留组件绘制器的原因：
- navigation-menu：菜单条在槽位里，浮层面板挂在菜单条下方；触发器/链接/dismiss 各有具名身份。
- pagination：槽位里画 `BeginPagination`；窄舞台只画当前页（`paginationCurrent`）。
- popover：触发器是 flex `Button`，其矩形即锚点；浮层与正文按钮用组件绘制。
- progress：flex `Progress`。
- questionnaire：问卷卡占据页面剩余高度（槽位），选项为列表 `questionnaireChoice`。
- radio-group：槽位 + `RadioGroup`；键盘走 `radioGroupItem` 列表身份。
- resizable：槽位 + `Resizable`（要组件的 `FractionAt` 与抓手）；指针回调读取上次绘制记下的槽位矩形（`widgetBag.placed`）。
- scroll-area：槽位 + `ScrollArea`（示例的重点是组件的偏移与滚动条几何，不是 flex `ScrollArea`）。
- segmented-control：flex `Segmented`（`Chosen()`）；整体禁用的那条用组件绘制。
- select：触发器槽位 + `Select`（要 Invalid/Disabled/向上翻转与 dismiss 身份，flex `Select` 没有）。
- separator：flex `Divider`。
- sheet：触发器是 flex `Button`；模态面板用 `BeginSheet`。
- sidebar：整页是 flex `Row`（侧栏槽位 + 内容列），侧栏本体用组件绘制。
- skeleton：槽位 + `BeginSkeleton`（flex 没有骨架屏）。
- slider：槽位 + `Slider`/`RangeSlider`；指针读取 `widgetBag.placed`，范围滑块的两个拇指也放在 `widgetBag`。
- sonner：页面文字用 flex，通知堆栈由 `PaintSonner` 在视口上绘制。
- spinner：槽位 + `PaintSpinner`（示例的重点是调用方选相位；flex `Spinner` 自己转）。
- table：槽位 + `BeginTable`（带页脚、说明与调用方持有的多选位；flex `Table` 是单选/可排序/可滚动的那种）。
- tabs：flex `Tabs`（`TabClicked()`），键盘走 `tabsItem` 列表身份。
- textarea：flex `Textarea`，可编辑字段身份是 `textareaField`。
- toast：页面文字用 flex，`PaintToast` 在视口绘制。
- toggle：槽位 + `BeginToggle`（flex 没有 Toggle）。
- toggle-group：槽位 + `BeginToggleGroup`；键盘走 `toggleGroupItem`。
- tooltip：槽位当锚点，提示用 `BeginTooltip`；悬停仍走 `pointer` 回调；"clamped" 例子刻意把锚点放在窗口边缘。
- tree：槽位 + `BeginTree`；分组行、展开箭头与叶子各有身份。
- typography：槽位 + `components.Typography`（示例展示的就是组件的文字角色）。

## 验证

布局目标：宽 320–760px，高约 340px，默认密度；较小 viewport 只裁剪，不提供滚动。
测试使用固定时钟和 reduced motion 比较确定性绘制流，另测真实 Switch 过渡最终停止。

```sh
go test ./internal/componentdemo
go test -race ./internal/componentdemo
go test ./internal/componentdemo -run '^$' -bench . -benchmem
```

Editor 的 `String()` 将 rune 缓冲区转为不可变字符串。为及时反映 provider 直接修改，
平台接管的非空输入每次绘制需要一次字符串快照分配；这不是闲置后台工作。
本包不修改核心 Editor API 来掩盖这项成本。运行器仍应仅在输入或其他失效事件后绘制。
系统字体的最终栅格、桌面 first responder/IME 和浏览器文本事件由各平台运行器集成验证。

### 首次测量（2026-10-14）

参考环境：Apple M2 Pro，macOS 15.5 (24F74)，darwin/arm64，Go 1.26.5。
使用普通 `go test` 优化构建（非 race、非 stripped release），默认 `GOMAXPROCS=10`：

```sh
go test ./internal/componentdemo -run '^$' -bench . -benchmem -benchtime=100ms -count=1
```

基准复用 Canvas，使用默认估算测宽器；稳定帧固定时钟并启用 reduced motion，
动画基准显式推进时间。数字仅衡量 Go 绘制命令生成，不包括字体栅格、GPU 或屏幕呈现。

| 测量 | 结果 |
| --- | --- |
| 19 个静态 case，宽 320 / 760 | 152–677 ns/op，全部 0 B/op、0 allocs/op |
| 平台接管的非空输入快照 | 186–222 ns/op，示例文本 16–24 B/op、1 alloc/op |
| 平台接管的空值、disabled/read-only | 0 B/op、0 allocs/op |
| Checkbox / Switch / Dialog 激活并绘制 | 162–596 ns/op，0 B/op、0 allocs/op |
| Button 激活并绘制 | 239 ns/op，23 B/op、1 alloc/op（更新计数字符串） |
| Switch 实际过渡帧 | 306 ns/op，0 B/op、0 allocs/op |
| `New`，未启用平台编辑 | State 112 B，112 B/op、1 alloc/op |
| `New` + 平台默认输入 | 208 B/op、3 allocs/op（含 Editor 与初始文本） |

这是新建的可选示例包，没有旧实现可作同功能基线。没有修改现有应用入口或核心持久结构，
不据此宣称 native release 大小、RSS、startup 或 idle GPU 指标已经完成实机验证。
测试覆盖率为 100.0% statements；普通测试、race、vet 和 darwin/arm64、windows/amd64、
linux/amd64、js/wasm 编译通过。非 macOS 目标仅做编译验证，未运行原生 UI。
