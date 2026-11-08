---
title: 无需窗口的测试
summary: 在测试里驱动应用、用行协议编写脚本，并检查各平台是否仍然一致。
order: 13
group: distribute
---

验证桌面应用通常意味着打开一个窗口：它会打断你，取决于窗口落在哪块显示器上，也不能无人值守地运行。Viva 提供三层避免这些问题的能力，每一层都可以不依赖下一层单独使用。

## app.Driver

`app.NewDriver` 用显式调用运行应用真实的绘制器、输入路由和焦点规则：`Move`、`Click`、`Key`、`Type`、`Shortcut`、`Scroll`、`Resize` 和 `Advance`。它不含任何平台代码，动画时钟只在你要求时才前进。`Snapshot` 返回绘制出来的内容：图元、标签、命中区域和焦点。下面的测试驱动的就是[开始使用](/zh/docs/getting-started/)里的那个程序：

<!-- example: hello/main_test.go -->
```go
package main

import (
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/flex"
)

// open runs the page in a headless window: the real painter, router and focus
// rules, driven by explicit calls.
func open(t *testing.T, m *model) *app.Driver {
	t.Helper()
	page := flex.NewPage(flex.PageConfig{View: m.page})
	driver, err := app.NewDriver(page.Config(app.Config{Width: 480, Height: 240}), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)
	return driver
}

// click presses the middle of the control a user would find by its name.
func click(t *testing.T, driver *app.Driver, name string) {
	t.Helper()
	for _, hit := range driver.Snapshot().Hits {
		if strings.HasPrefix(hit.Label, name) {
			b := hit.Bounds
			if err := driver.Click(b.X+b.Width/2, b.Y+b.Height/2); err != nil {
				t.Fatal(err)
			}
			return
		}
	}
	t.Fatalf("no control named %q on screen", name)
}

func shows(driver *app.Driver, want string) bool {
	for _, label := range driver.Snapshot().Labels {
		if strings.Contains(label.Value, want) {
			return true
		}
	}
	return false
}

func TestClickCounts(t *testing.T) {
	driver := open(t, &model{})
	if !shows(driver, "Clicked 0 times") {
		t.Fatal("the button should start at zero")
	}
	click(t, driver, "Clicked")
	if !shows(driver, "Clicked 1 times") {
		t.Fatal("the click did not reach the button")
	}
}
```

测试按用户和读屏器认识控件的名字（`Hit.Label`）找控件，而不是按坐标。页面自己为元素命名，在它的树里是 `page/~k1/#0` 这样的路径；`c.Part("word")` 给元素起一个你自己的名字，测试和无头驱动可以按路径定位。

## 能编译却做错事的写法

页面可能能编译却做错事：循环里每一行都有同名 `Button`、行又没加 Key 时，上方新增一行后，点击可能落到别的行上。`viva vet` 不运行程序就能找出它：

```sh
viva vet            # 或 go tool viva vet；包默认为 ./...
```

每个问题报告为 `文件:行:列: 哪里错了以及应该怎么写 (检查名)`，发现问题时以非零状态退出，并使用项目自己的 Go 运行。在该行或上一行写 `// viva:ignore <检查名>` 可以忽略一条。`flex.Memo` 的版本号漏掉了函数读取的某个状态时会显示旧内容：在测试里打开 `flex.MemoCheck(true)`，它会声明每个部件，并在复制结果与声明结果不同时 panic。

## 行协议

在带构建标签的文件里引入 `github.com/vivaui/viva/headless/auto`，这样发布构建永远不会包含它：

```go
//go:build vivahead

package main

import _ "github.com/vivaui/viva/headless/auto"
```

用 `-tags vivahead` 构建，并以 `VIVA_HEADLESS=1` 启动程序。此时 `RunCanvas` 会从标准输入读取命令，而不是打开窗口。控件通过标签而不是坐标寻址：

```text
click "New Host"
type example.com
key tab
waitfor "Connected" 5000
texts
shot /tmp/frame.png
record /tmp/frames 700
quit
```

每个请求得到零行或多行数据，以及一行最终的 `ok` 或 `error`，所以脚本、其他程序或AI 编程助手都可以在不碰屏幕的情况下驱动并检查应用。完整命令列表在 `headless/control.go` 的文档注释里。

`layout` 会检查当前帧里几乎必然是错误的几何，每个问题输出一行：被裁剪截断的文字、超出所在按钮或输入框的文字、同一层上互相覆盖的两行文字、跑出窗口的文字，以及互相交叉而不是一个包含另一个的可聚焦控件。它从不失败，所以每次改完界面都可以运行它；几行文字比一张图更快读完。它按 Viva 布局所用的参考度量判断文字宽度，所以平台把一行画得宽几个百分点不会被报告。

**截图和录制需要离屏渲染器。** macOS 上它只链接进以 `-tags vivahead` 构建的产物；Windows 和 Linux 上总是可用。它用窗口自己的管线（Metal、Direct3D 11、Vulkan）绘制，所以抓到的就是窗口的像素。没有渲染器时协议仍可操作并回答查询，但 `Image`、`Screenshot` 和 `Record` 返回 `ErrNoRenderer`。

`record DIR MS [FPS]` 再播放 MS 的动画时钟，把像素发生变化的帧写成从 `frame-0000.png` 开始的文件，外加一份记录每帧停留时长的 `manifest.json`，所以一段过渡可以当成视频而不是一张静止图来审查。重复的帧只写一次并延长它的时长，所以静止的应用只会录到一帧。`shot` 仍是单帧版本，会先把动画播放到静止。

## 平台一致性

`cmd/viva-conformance` 通过公开的 `app` 包，在每个平台上运行同一组生命周期场景，并对每个场景输出 PASS、FAIL 或 SKIP：从其他 goroutine 关闭、空闲窗口不做事、`Invalidate` 只唤醒一次、隐藏窗口、面板、状态栏图标，以及无论窗口怎么说 `Quit` 都能结束循环。每一项检查的存在，都是因为某个真实平台曾经在那里出过错。它会打开真实窗口，所以需要桌面会话。它检查不了什么、这些如何手动检查，见仓库中的 `docs/conformance.md`。
