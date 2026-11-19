import type { Locale } from './i18n';

interface Card { title: string; text: string }
interface Measure { label: string; value: string; detail: string }
interface Platform { name: string; stack: string; detail: string }

// The home page's words. Numbers are never typed here as a second source: they
// are chosen by id from figures.ts and checked against the reports they quote.
const english = {
  title: 'Viva — native desktop applications in Go',
  summary: 'Viva is a Go framework for native desktop applications: direct operating-system windows and GPU rendering, no WebView, nothing running while idle.',
  eyebrow: 'macOS · Windows · Linux · Web (WASM)',
  heading: 'Native desktop applications, written in Go.',
  headingMark: 'Go',
  headingKey: 'Native',
  // The first sentence is the definition a reader or an assistant quotes, so it
  // names the product, the language and the category before the copy argues.
  lead: 'Viva is a Go framework for native desktop applications. It draws windows, text and components straight through each operating system\'s own APIs and the GPU: there is no WebView, no browser engine and no bundled runtime, and an idle application does no work at all.',
  start: 'Get started', browse: 'Browse components',
  heroStats: [
    { value: '1.6 MiB', label: 'stripped executable' },
    { value: '0', label: 'GPU submissions while idle' },
    { value: '0.0%', label: 'idle CPU' },
  ],
  componentCountLabel: 'components',
  principlesHeading: 'What Viva is built around',
  principles: [
    { title: 'Native, not a browser', text: 'macOS calls Cocoa and Metal, Windows calls Win32 and Direct3D 11, Linux calls XCB and Vulkan from the system\'s own libraries. An application links system frameworks and the Go runtime, nothing else.' },
    { title: 'Idle means idle', text: 'An unchanged frame does no layout, rebuilds no scene and submits nothing to the GPU. There are no background timers or polling loops by default.' },
    { title: 'Components you own', text: 'viva add copies a component\'s Go source into your project. There is no component registry and no stylesheet parser at runtime: a component is ordinary Go that you are free to edit.' },
    { title: 'Testable without a window', text: 'A headless driver runs the real painter, input router and focus rules from explicit calls and, on macOS, can capture the pixels a window would show, so tests need no desktop session.' },
    { title: 'One API, three native backends', text: 'Cocoa and Metal on macOS, Win32 and Direct3D 11 on Windows, and X11 and Vulkan on Linux. An application links the system frameworks and the Go runtime, nothing else.' },
    { title: 'Desktop only', text: 'Viva targets macOS, Windows and Linux. Mobile is out of scope, so desktop APIs are never weakened to keep a mobile door open.' },
  ] satisfies Card[],
  measuredHeading: 'Measured, reproducibly',
  measuredIntro: 'Every number is quoted from a benchmark report in the repository, on the machine and build named there — you can rerun them yourself.',
  measuredLink: 'Full benchmarks',
  measures: {
    binary: { label: 'Stripped executable', value: '1.6 MiB', detail: '1,697,506 bytes, a minimal macOS window with a Metal pipeline (M0, Apple silicon).' },
    idle: { label: 'GPU submissions while idle', value: '0', detail: 'After the first present an unchanged window submits nothing, and settled CPU reads 0.0% (M0).' },
    scene: { label: 'Retained scene, 10,000 nodes', value: '2.0 ns · 12.4 ns', detail: 'Draining an unchanged scene, then updating one node. Both take zero allocations (M1).' },
    showcase: { label: 'The component showcase', value: '1.9 MiB', detail: '1,972,834 bytes stripped: the native showcase application with its previews and icons (Apple M2 Pro).' },
    footprint: { label: 'Memory after the first present', value: '26.3–26.8 MiB', detail: 'The budget is 32 MiB and the goal is 15 MiB. The gap comes from the fixed AppKit and Metal cost, which has not been separated from Viva\'s own yet.' },
    firstFrame: { label: 'Time to first frame', value: '123–374 ms', detail: 'The budget is 400 ms warm and the goal is 50 ms. The measurement includes loading the Metal library and building the pipeline.' },
  } satisfies Record<string, Measure>,
  platformsHeading: 'Where it runs today',
  platforms: [
    { name: 'macOS', stack: 'Cocoa · Metal · CoreText', detail: 'Native text input with Chinese IME composition, accessibility, status items, and packaging to .app, .zip and .dmg.' },
    { name: 'Windows', stack: 'Win32 · Direct3D 11 · DirectWrite', detail: 'Native windows, Direct3D rendering, system text and packaged amd64 and arm64 builds.' },
    { name: 'Linux', stack: 'X11 · Vulkan · Pango/Cairo', detail: 'X11 through system XCB and Vulkan libraries, with Pango/Cairo text and packaged builds.' },
  ] satisfies Platform[],
  platformsLink: 'Platform details',
  platformsWebNote: 'There is also an optional web target: the same Go components compiled to WebAssembly and drawn with WebGL2, falling back to Canvas2D when WebGL2 is unavailable or a context is lost. It is a secondary deployment and preview target, not part of the desktop runtime, and it does not replace native verification.',
};

const chinese: typeof english = {
  title: 'Viva — 用 Go 写原生桌面应用',
  summary: 'Viva 是面向原生桌面应用的 Go 框架：直接使用操作系统窗口与 GPU 渲染，没有 WebView，空闲时不做任何事。',
  eyebrow: 'macOS · Windows · Linux · Web (WASM)',
  heading: '用 Go 写原生桌面应用。',
  headingMark: 'Go',
  headingKey: '原生',
  lead: 'Viva 是面向原生桌面应用的 Go 框架。它直接通过各操作系统自己的接口和 GPU 绘制窗口、文字与组件：没有 WebView，没有浏览器引擎，也没有捆绑的运行时；应用空闲时完全不做事。',
  start: '开始使用', browse: '浏览组件',
  heroStats: [
    { value: '1.6 MiB', label: '剥离后的可执行文件' },
    { value: '0', label: '空闲时的 GPU 提交' },
    { value: '0.0%', label: '空闲 CPU' },
  ],
  componentCountLabel: '个组件',
  principlesHeading: 'Viva 围绕什么构建',
  principles: [
    { title: '原生，不是浏览器', text: 'macOS 调用 Cocoa 与 Metal，Windows 调用 Win32 与 Direct3D 11，Linux 调用系统自带的 XCB 与 Vulkan 库。应用只链接系统框架和 Go 运行时，别无其他。' },
    { title: '空闲就是空闲', text: '未变化的一帧不做布局、不重建场景、不向 GPU 提交任何东西。默认没有后台定时器和轮询。' },
    { title: '组件归你所有', text: 'viva add 把组件的 Go 源码复制进你的项目。没有组件注册表，运行时也没有样式表解析器：组件就是可以随意修改的普通 Go 代码。' },
    { title: '无需窗口即可测试', text: '无头驱动用显式调用运行真实的绘制器、输入路由和焦点规则，在 macOS 上还能截取窗口会显示的像素，测试不需要桌面会话。' },
    { title: '一套 API，三个原生后端', text: 'macOS 使用 Cocoa 与 Metal，Windows 使用 Win32 与 Direct3D 11，Linux 使用 X11 与 Vulkan。应用只链接系统框架和 Go 运行时，别无其他。' },
    { title: '只做桌面', text: 'Viva 面向 macOS、Windows 和 Linux。移动端不在范围内，所以桌面 API 不会为了给移动端留门而被削弱。' },
  ],
  measuredHeading: '实测，可复现',
  measuredIntro: '每个数字都引自仓库里的基准报告，写明了机器和构建方式——你可以自己重新跑一遍。',
  measuredLink: '完整基准',
  measures: {
    binary: { label: '剥离后的可执行文件', value: '1.6 MiB', detail: '1,697,506 字节，带 Metal 管线的最小 macOS 窗口（M0，Apple 芯片）。' },
    idle: { label: '空闲时的 GPU 提交', value: '0', detail: '首次呈现之后，未变化的窗口不提交任何东西，稳定后 CPU 读数为 0.0%（M0）。' },
    scene: { label: '保留式场景，10,000 个节点', value: '2.0 ns · 12.4 ns', detail: '先是清空一个未变化的场景，再是更新其中一个节点，两者都是零分配（M1）。' },
    showcase: { label: '组件展示应用', value: '1.9 MiB', detail: '剥离后 1,972,834 字节：带预览与图标的原生展示应用（Apple M2 Pro）。' },
    footprint: { label: '首次呈现后的内存', value: '26.3–26.8 MiB', detail: '预算 32 MiB，目标 15 MiB；差距来自 AppKit 与 Metal 的固定开销，它还没有和 Viva 自身的部分分开。' },
    firstFrame: { label: '首帧用时', value: '123–374 ms', detail: '预算 400 ms（预热），目标 50 ms；其中包含加载 Metal 库与创建管线。' },
  },
  platformsHeading: '目前能在哪里运行',
  platforms: [
    { name: 'macOS', stack: 'Cocoa · Metal · CoreText', detail: '原生文本输入，支持中文输入法组合；无障碍、状态栏图标，可打包为 .app、.zip 与 .dmg。' },
    { name: 'Windows', stack: 'Win32 · Direct3D 11 · DirectWrite', detail: '原生窗口、Direct3D 渲染、系统文字，以及 amd64 与 arm64 打包构建。' },
    { name: 'Linux', stack: 'X11 · Vulkan · Pango/Cairo', detail: '通过系统 XCB 与 Vulkan 库使用 X11，Pango/Cairo 文字，并提供打包构建。' },
  ],
  platformsLink: '平台详情',
  platformsWebNote: '另有可选的 Web 目标：同一份 Go 组件编译为 WASM，优先用 WebGL2 绘制，WebGL2 不可用或上下文丢失时回退 Canvas2D。它是次要的部署与预览目标，不进入桌面运行时，也不替代原生验证。',
};

export const home = { zh: chinese, en: english } as const;
export function homeText(locale: Locale): typeof english {
  return home[locale];
}
