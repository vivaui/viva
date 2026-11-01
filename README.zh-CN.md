<h1 align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="website/src/assets/viva-lockup-dark.svg">
    <img src="website/src/assets/viva-lockup.svg" alt="Viva" width="240">
  </picture>
</h1>

<p align="center"><a href="README.md">English</a> | 简体中文</p>

Viva 是一个 Go 优先、GPU 原生的桌面框架。应用通过操作系统自己的窗口和 GPU API 绘制——Cocoa 与 Metal、Win32 与 Direct3D 11、XCB 与 Vulkan——不使用 WebView，不捆绑浏览器引擎，也不捆绑运行时。它追求小的二进制产物、低的空闲资源占用，以及对原生平台能力的直接访问。需要展示网页内容的应用可以在 Viva 的绘制旁边承载系统自带的网页视图（WebKitGTK、WKWebView、WebView2）：它是可选的，第一个页面打开时才加载，不捆绑任何东西。

下面的状态列表是今天实际能跑的东西。

## 快速开始

Viva 是一个 Go 模块，`github.com/vivaui/viva`。`viva` 命令负责构建、打包和热重载应用；它本身不会被链接进你发布的程序。

```sh
brew install vivaui/tap/viva        # macOS, Linux
scoop bucket add vivaui https://github.com/vivaui/homebrew-tap && scoop install viva   # Windows
viva doctor                         # is this machine ready?
viva init hello
cd hello
viva dev .        # hot reload: save a file, the window updates and keeps its state
```

从 `v0.1.3` 起的每个发布都提供预编译压缩包（带 `SHA256SUMS`）；已经装了 Go 的话，`go install github.com/vivaui/viva/cmd/viva@latest` 同样可用。你不必先装 Go：机器上没有时，`viva build`、`viva dev` 和 `viva init` 会把官方工具链下载一次到用户缓存（按 go.dev 的 SHA-256 校验；`VIVA_GO_DOWNLOAD=0` 可关闭）。`viva init` 生成的项目在 `go.mod` 里用一条 `tool` 记录固定命令版本，所以 `go tool viva` 跑的正是该项目构建时所用的版本。见《构建与打包》指南。

`viva init` 会写一个可直接运行的项目：一个窗口、一个不用打开窗口就能驱动它的测试、`viva.toml`，以及一份描述编程模型的 `AGENTS.md`。`viva build` 把发布程序写到 `build/`，`viva package` 写出平台自己的包。

环境要求：Go 1.26 或更新版本（缺失时 viva 会自己下载）。macOS 上需要 Xcode 命令行工具，因为 Viva 通过 cgo 访问 Cocoa 和 Metal；Linux 上需要带系统 XCB 和 Vulkan 加载库的 X11；Windows 上需要 Win32、Direct3D 11 和 DirectWrite。Viva 在运行时加载系统库，自身不捆绑任何一个。

当前版本是 `v0.2.1`；要固定版本就写明（`@v0.2.1`）。指南、API 参考和带实时预览的组件目录在 [`website/`](website/) 里，本地用 `npm run dev` 运行；也已经发布在 [vivaui.dev](https://vivaui.dev/zh/)。

下面的一切描述的都是这个仓库：它的测量、它自己的程序，以及如何开发框架本身，而不是如何用框架。

## 范围

Viva 面向 macOS、Windows 和 Linux 上的原生桌面应用。WebAssembly/WebGPU 后端是可选的次要目标，不得给原生构建增加成本。iOS、iPadOS、Android、移动端打包，以及以触摸为主的移动运行时，明确不在范围内。不会为了保留假设中的未来移动兼容性而削弱桌面性能与原生集成。

三个后端都直接使用操作系统自己的窗口和 GPU API，没有 WebView，也不捆绑渲染运行时。

## 当前状态

- macOS 原生窗口与事件循环。
- 直接 Metal 清屏与呈现。
- 扁平的分代保留式场景，节点标识稳定。
- 合并后的 O(1) 局部矩形更新，热路径零分配。
- 预编译 Metal 着色器与实例化矩形渲染。
- 事件驱动的 C 到 Go 指针分发，以及 32 字节的部分 Metal 上传。
- 按绘制顺序的悬停与按下交互状态和点击激活；在一个区域内移动指针不产生渲染器工作。
- 键盘焦点、Tab 遍历、Enter 与空格激活和焦点环；一次按键事件最多重绘一次，且只在焦点真正变化时发布无障碍更新。
- 事件驱动的滚动，按图元裁剪：1000 行的列表虚拟化为可见窗口，一次滚动手势恰好重绘一次并做一次部分 GPU 上传，被裁剪的内容永远不会到达渲染器。
- 三种原生窗口模式：标准带标题栏、无边框（没有红绿灯按钮）和自定义标题栏（透明标题栏、内容占满高度、由页面声明的区域拖动窗口）。窗口关闭可以被应用拦截。
- 通过 `app.RunApplication` 显式支持多窗口应用：每个窗口各自拥有独立的保留式绘制、焦点、输入、文本、失效和关闭状态，最后一个窗口关闭后原生循环退出。
- 状态区图标与弹出面板：`Application.AddStatusItem` 把图标放到 macOS 菜单栏、Windows 通知区域或 Linux 的 StatusNotifier 宿主；`StatusItemConfig.Menu` 让图标打开系统自己的菜单（NSMenu、Win32 弹出菜单、`com.canonical.dbusmenu`）；`WindowPanel` 是在它旁边打开、失焦即关闭的无边框弹出面板；`Window.Hide`/`Show` 让应用在没有可见窗口时继续运行。
- `Application.SetDockBadge` 在 macOS 为应用 Dock 图标画出的红色角标里放一段文字，空文字表示去掉角标。
- `app.StoreSecret`/`LoadSecret`/`DeleteSecret` 把少量机密保存在系统自己的存储里（Keychain、Credential Manager、Secret Service）。
- `ui.MonoFamily` 用任何已安装的字体族绘制等宽文本，`app.MonospaceFamilies` 列出它们。
- `Config.OnFileDrop` 接收从文件管理器拖入的文件，并带上指针下的命中区域。
- 一棵独立的保留式语义无障碍树，镜像成原生 NSAccessibility 元素、Windows UI Automation 提供程序（Invoke、Toggle、Value、RangeValue、SelectionItem、ExpandCollapse 和 Text 模式，并带事件）和 Linux AT-SPI 对象，浏览器构建里映射为 ARIA。屏幕阅读器的按下、聚焦和设置值请求通过 `platform.AccessibilityHandler` 到达应用。见 [docs/accessibility.md](docs/accessibility.md)。
- 事件驱动的窗口缩放重绘：每次缩放恰好一次 GPU 提交，尺寸不变时零工作。
- `Config.OnFrameChange`、`Config.OnMinimize` 和 `Window.Minimized` 报告窗口的位置，以及窗口管理器是否把它最小化。
- 保留式 Row/Column/Stack/Flex 布局，布局不变时 O(1) 检查。
- 按绘制顺序的指针路由、稳定的键盘焦点和 Tab 遍历。
- macOS 原生 `NSTextInputClient` 集成，支持真实的中文输入法候选组合、按字素编辑和 UTF-16 映射。
- macOS、Windows 和 Linux 上由焦点驱动的动态原生文本编辑器，带显式的启用/替换/禁用帧更新。macOS 和 Windows 的光标定时器只在可编辑字段获得焦点时存在。Linux 通过 IBus 协议组字（IBus 以及 Fcitx5 的 IBus 前端）；XIM 仍未支持。
- 一棵独立的保留式语义无障碍树。
- 直接 CoreText 整形、系统字体回退、GPU 合成的文本纹理，以及有上限的测量缓存。
- 实验性的 Windows shell：Win32 事件、D3D11 保留式渲染、内嵌 DXBC 着色器、DirectWrite 测量，以及有上限的 GDI 标签纹理；原生构建不加载着色器编译器。
- 实验性的 Linux/X11 shell：动态加载系统 XCB、Vulkan、Pango 和 Cairo；保留式圆角矩形与三角形、分层 GPU 合成的系统文本、指针与键盘输入、自定义标题栏拖动，以及没有空闲重绘的阻塞式事件循环。
- 事件驱动渲染，没有空闲帧循环。
- 主题令牌，以及把固定版本的 shadcn 组件目录静态类型化、源码自有的移植，包括表单、浮层、菜单、日历、滚动配方、反馈界面，以及从有类型切片绘制的面积图、折线图和柱状图，不需要浏览器运行时、反射或运行时样式解析器。
- 共享的原生/WASM 组件浏览器：按分类分组的全部组件目录，以及每个组件一页、并排运行它的所有示例（点击、悬停、键盘、拖动、滚轮和原生文本编辑都可用），页面滚动条可拖动，热重载保留状态。示例就是 `internal/componentdemo` 里的那些，通过 `ui.Canvas.Embed` 嵌入页面，所以组件浏览器不保留任何组件的副本。
- 窗口关闭或应用终止时干净地退出事件循环。
- 可复现的二进制、启动、CPU、内存和 GPU 提交测量。

Windows 后端支持 amd64 和 arm64 交叉构建，并在 Wine/DXVK 下通过平台测试以及 Gallery 的启动、指针和键盘冒烟测试。平台测试、conformance 场景，以及输入、拖放和设备丢失路径，在一台 Windows 11 虚拟机（WARP）和一台 Debian KDE/X11 机器（lavapipe）上运行；`docs/conformance.md` 记录了需要真实硬件的场景。UI Automation 在一台真实的 Windows 11 机器上由外部客户端检查（见 docs/accessibility.md），AT-SPI 在 KDE/X11 机器上针对 libatspi 检查。Linux 后端通过 Ubuntu arm64 Xvfb/llvmpipe 的 M0 和 Debian amd64 X11/Lavapipe 的保留式场景冒烟测试，包括中英混排文本和局部场景更新。macOS 运行时测试和基准在 arm64 上运行。

原生标签渲染器绘制单行标签。浏览器预览运行在 Canvas2D 后端。

## 运行这个仓库

以下命令从这份检出构建并驱动框架自己的程序。

```sh
go run ./cmd/showcase        # 原生组件展示程序
go run ./cmd/component-preview -component button -example variants   # 单独原生运行一个示例
tools/build_web.sh           # viva build --target web，然后服务 http://127.0.0.1:8090
go run ./tools/docs -check   # 网站生成的 API 与组件数据
```

浏览器里的组件浏览器通过 Canvas2D WebAssembly 后端绘制相同的 Go 组件；它的视觉方向与测量成本记录在 `docs/benchmarks/m3-gallery-style-darwin-arm64.md`。

## 文档网站

[vivaui.dev](https://vivaui.dev/zh/) 是公开站点，提供英文和中文：指南的代码块就是 `examples/guides/` 下的文件，由 `go test` 构建并无窗口驱动；API 参考从 Go 源码生成；每个组件一页，带实时的 Go/WASM 预览。Astro + Svelte 源码在 [`website/`](website/) 里，它不属于原生 UI，也不进入发布运行时。

```sh
cd website
npm ci
npm run dev          # http://127.0.0.1:4321/zh/ 和 /en/
npm test && npm run build
```

预览流水线、生成的数据和 `test:site` 见 [website/README.md](website/README.md)。

## 构建与打包应用

`viva build` 为 `--target darwin`、`windows`、`linux` 或 `web` 生成发布程序（`-trimpath`、剥离符号、固定构建 ID，同样的源码得到同样的字节），并把它的体积与周围的东西分开报告。`viva package` 再加上平台自己的安装器。两者都读取 `go.mod` 旁边的可选 `viva.toml`：

```toml
[app]
name = "Viva Gallery"       # 显示在菜单栏；也是 .app 的名字
id = "dev.viva.gallery"     # bundle identifier
package = "./cmd/showcase"  # 要构建的主包

[mac]
min_system = "12.0"         # 程序按哪个 macOS 版本编译；下限 12.0
```

```sh
go run ./cmd/viva build --target windows --arch amd64
go run ./cmd/viva package --zip
go run ./cmd/viva help package     # 每个命令和参数
```

`viva.toml` 严格解码：未知的表或键、重复项、类型错误的值都是错误，并会指出行号。macOS 程序按 `min_system` 编译，而不是按构建机器的 macOS 版本；`package` 会拒绝声称支持的系统比其程序所需更旧的包。签名与公证通过 `--sign-identity` 或 `APPLE_SIGNING_IDENTITY` 加 notarytool 凭据显式开启；`--no-sign` 把它们全部关闭。`--arch amd64` 从任一宿主构建另一种 Darwin 架构：构建保留 cgo 和 Objective-C/Metal 桥，让 clang 产出另一份 Mach-O 切片。每个目标、安装器格式和参数见[构建与打包指南](https://vivaui.dev/zh/docs/build-and-package/)。

发布链接是测出来的，不是假设的。`go-toml/v2` 只被 `viva` 命令使用，`purego` 只被 Linux 后端使用，所以两者都不在 macOS 或 Windows 程序里。开发期热重载协议、单实例套接字、PNG 图标解码和 Go 测试运行时都留在发布程序之外（热重载、单实例和 PNG 是可选包；测试运行时则完全不存在），泄漏计数器保留，因为它们每个窗口或驱动只花两个原子量。

静态画布可以通过公开 API 呈现：

```go
theme := ui.DarkTheme()
var canvas ui.Canvas
components.Card(&canvas, theme, ui.Rect{X: 24, Y: 24, Width: 320, Height: 180})
if err := app.RunCanvas(app.Config{Title: "My App"}, &canvas); err != nil {
    log.Fatal(err)
}
```

## 工程规则

性能预算、依赖政策、架构边界和完成定义由签入的基准和测试来约束。测量是证据；“原生速度”“空闲零工作”和“轻量”这类说法不会作为未经核实的营销话术使用。

## 参与贡献

Bug 报告、平台发现和文档修正永远欢迎——改动必须通过的门禁，以及框架行为与应用行为之间的界线，见 [CONTRIBUTING.md](CONTRIBUTING.md)。安全问题走 [SECURITY.md](SECURITY.md)，绝不要开公开 Issue。

## 贡献者

<a href="https://github.com/vivaui/viva/graphs/contributors">
  <img src="https://contrib.rocks/image?repo=vivaui/viva&max=300&columns=15" alt="Contributors" />
</a>

## 社区

<a href="https://linux.do"><img src="https://img.shields.io/badge/LINUX%20DO-Community-blue" alt="LINUX DO" /></a>

## 许可证

Viva 以 Apache License 2.0 授权。见 [LICENSE](LICENSE) 和 [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md)。
