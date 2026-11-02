---
title: Get started
summary: Install the requirements, add Viva to a Go module, and open your first window.
order: 1
group: start
---

## Requirements

- **Go 1.26 or newer.** If you use the `viva` command and have none, it downloads one for you; see [Build and package](/en/docs/build-and-package/).
- **macOS:** the Xcode command-line tools. Viva reaches Cocoa and Metal through cgo.
- **Linux:** X11 with the system XCB and Vulkan loaders. Viva loads them at run time and bundles none of them.
- **Windows:** Win32 with the system Direct3D 11 and DirectWrite libraries.

## Add Viva to a project

Viva's module path is `github.com/vivaui/viva`:

```sh
mkdir hello && cd hello
go mod init example.com/hello
go get github.com/vivaui/viva
```

`go get` without a version resolves to the latest tagged release, which is `v0.2.3`. Pin it by naming the version: `go get github.com/vivaui/viva@v0.2.3`. Viva is not yet at 1.0, so the API in `go.mod` can still change between minor releases; read the release notes before upgrading.

If you are working on Viva itself rather than using it, `replace` it with your checkout instead:

```sh
go mod edit -require=github.com/vivaui/viva@v0.0.0 -replace=github.com/vivaui/viva=../viva
```

## Your first window

<!-- example: hello/main.go -->
```go
package main

import (
	"fmt"
	"log"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/ui"
)

// Identities are names in a tree, not numbers: the application declares the
// tree once and every control it paints is a name inside a feature of it.
var (
	tree = ui.Tree{}
	page = tree.Feature("hello")

	click = tree.ID(page, "click")
)

type model struct {
	clicks int
	view   flex.View
}

// newConfig describes the window. Keeping it in a function lets a test drive
// the same application without opening anything.
func newConfig(m *model) app.Config {
	theme := ui.LightTheme()
	config := app.Config{Tree: &tree, Title: "Hello", Width: 480, Height: 240}

	config.Paint = func(canvas *ui.Canvas, size ui.Size) {
		bounds := ui.Rect{Width: size.Width, Height: size.Height}
		canvas.Fill(bounds, theme.Background)

		m.view.Draw(canvas, theme, bounds, func(f *flex.Frame) {
			f.Column().Fill().Pad(24).Gap(12).Children(func() {
				// A button is handled where it is declared: Clicked is true for the
				// one paint that delivers its click, and only then may state change.
				if f.Button(click, fmt.Sprintf("Clicked %d times", m.clicks)).Clicked() {
					m.clicks++
				}
			})
		})
	}
	return config
}

func main() {
	var canvas ui.Canvas
	if err := app.RunCanvas(newConfig(&model{}), &canvas); err != nil {
		log.Fatal(err)
	}
}
```

Run it with `go run .`. The program is the whole programming model in miniature:

- `newConfig` describes the window. Keeping it in a function lets a test drive the same application without opening anything.
- `Paint` describes the interface from your own state, here the `clicks` counter. Viva keeps no copy of that state.
- `flex.View` lays the page out from a description: a padded column holding a button. No coordinates are written, so the page keeps working when the label or the window changes size. The button has the stable identity `click`, a name in the `hello` feature of the tree.
- `Clicked()` is true for exactly one paint: the extra paint the runtime makes when a click completes, and only for the button that was clicked. Your code changes its own state there, next to the button it handles. The window then repaints once. Every other paint is read-only.

Nothing else runs. If you stop clicking, the application does no layout, submits nothing to the GPU and wakes no timers.

## Try the demos

In a Viva checkout you can run these demos directly (they are not part of what a consumer installs):

```sh
go run ./cmd/showcase            # the native component showcase
go run ./cmd/viva list            # components viva add can copy into a project
go run ./cmd/component-preview -component button -example variants
```

The same components run in the browser. The [component catalog](/en/ui/button/) has live WebAssembly previews, the Go source of every example, and the native command that runs it.

## Where next

- [Programming model](/en/docs/programming-model/): state, events, focus and background work.
- [Components](/en/docs/components/): using the catalog and owning a copy of a component.
- [Build and package](/en/docs/build-and-package/): `viva.toml`, release builds and installers.
