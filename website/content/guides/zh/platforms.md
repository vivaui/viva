---
title: 平台
summary: Viva 在 macOS、Windows、Linux 和浏览器上的构建方式。
order: 14
group: more
---

Viva 在三个原生后端之上保持同一套公开 API。每个后端都使用操作系统自己的窗口、GPU 和文字服务；原生构建只链接系统框架和 Go 运行时。

## macOS

使用 Cocoa、`CAMetalLayer` 和 Metal 绘制，CoreText 负责文字。

- 原生文本输入，支持带候选窗的中文输入法组合。
- 独立的语义树，映射为 `NSAccessibility` 元素。
- 状态栏图标和弹出面板，状态项可以使用系统自己的 `NSMenu`。
- 可打包为 `.app`、`.zip` 与 `.dmg`。程序默认面向 macOS 12 编译。

## Windows

Win32 事件、带预编译着色器的 Direct3D 11、用于测量的 DirectWrite，以及用于标签的有界 GDI 纹理。原生构建不加载着色器编译器。

- 支持 amd64 与 arm64 Windows。
- 原生窗口、输入和系统文字。
- 可打包为 NSIS 安装程序或 `.zip`。

## Linux

通过动态加载的系统 XCB、Pango 和 Cairo 使用 X11，有 GPU 驱动时再用 Vulkan；事件循环是阻塞式的，空闲时不重绘。

- X11，使用系统自带的 XCB、Pango 和 Cairo 库；Vulkan 加载库和驱动提供 GPU 加速，缺失时用 CPU 渲染。
- Pango 与 Cairo 负责文字。
- 可打包为带安装脚本的 `tar.gz` 或 `.deb`。
- 文件对话框、打开 URL 和通知使用已安装的桌面工具，见[系统集成](/zh/docs/system-integration/)。

## 浏览器（可选）

次要目标：把同样的 Go 组件编译为 WebAssembly，优先用 WebGL2 绘制；WebGL2 不可用或上下文丢失时回退 Canvas2D。组件目录里的预览就用它。

- 浏览器构建复用与原生后端相同的应用、布局和组件代码。
- 剪贴板和文件选择器使用浏览器的异步 API。
- 文本编辑已在 Input 示例中接通，使用与原生预览相同的 Go 编辑器。
- WebGL2 是主要的浏览器渲染器，Canvas2D 是兼容回退。WebGPU 不是必需的；只有测量证明有需要时，才可能作为可选后端加入。

浏览器后端单独构建，绝不会让原生产物变大、变慢或变复杂。

## 不支持

iOS、iPadOS、Android 等移动系统不在范围内，触控优先的运行时也不是目标。桌面 API 不会为了给移动端留门而被削弱。
