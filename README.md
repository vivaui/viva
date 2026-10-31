<h1 align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="website/src/assets/viva-lockup-dark.svg">
    <img src="website/src/assets/viva-lockup.svg" alt="Viva" width="240">
  </picture>
</h1>

<p align="center">English | <a href="README.zh-CN.md">简体中文</a></p>

Viva is a Go-first, GPU-native desktop framework. Applications draw through
the platform's own window and GPU APIs — Cocoa and Metal, Win32 and Direct3D 11,
XCB and Vulkan — with no WebView, no browser engine, and no bundled runtime. It
is aimed at small binaries, low idle resource usage, and direct access to native
platform capabilities.

The status list below is what runs today.

## Quick start

Viva is a Go module, `github.com/vivaui/viva`. The `viva` command builds,
packages and live-reloads applications; nothing in it is linked into the program
you ship.

```sh
brew install vivaui/tap/viva        # macOS, Linux
scoop bucket add vivaui https://github.com/vivaui/homebrew-tap && scoop install viva   # Windows
viva doctor                         # is this machine ready?
viva init hello
cd hello
viva dev .        # hot reload: save a file, the window updates and keeps its state
```

Prebuilt archives (with `SHA256SUMS`) are on each release from `v0.1.3` on; with Go
already installed, `go install github.com/vivaui/viva/cmd/viva@latest` works too.
You do not need Go first: `viva build`, `viva dev` and `viva init` download the
official toolchain once into your user cache when there is none (checked against
go.dev's SHA-256; `VIVA_GO_DOWNLOAD=0` turns it off). A project made by `viva init`
pins the command with a `tool` line in `go.mod`, so `go tool viva` runs the version
that project builds against. See the Build and package guide.

`viva init` writes a runnable project: a window, a test that drives it without
opening one, `viva.toml`, and an `AGENTS.md` describing the programming model.
`viva build` writes a release program to `build/`, and `viva package` writes the
platform's own bundle.

Requirements: Go 1.26 or newer (viva fetches it when missing). On macOS the Xcode command-line tools, because
Viva reaches Cocoa and Metal through cgo; on Linux, X11 with the system XCB and
Vulkan loaders; on Windows, Win32 with Direct3D 11 and DirectWrite. Viva loads
the system libraries at runtime and bundles none of them.

The current release is `v0.2.1`; name it explicitly (`@v0.2.1`) to pin it. Guides,
the API reference, and a component catalog with live previews live in
[`website/`](website/); run it with `npm run dev` there, or read them on
[vivaui.dev](https://vivaui.dev/en/), which is also available in
[Chinese](https://vivaui.dev/zh/).

Everything below describes this repository: its measurements, its own programs,
and how to work on the framework rather than with it.

## Scope

Viva targets native desktop applications on macOS, Windows, and Linux. A
WebAssembly/WebGPU backend is optional and must not add cost to native builds.
iOS, iPadOS, Android, mobile packaging, and a touch-first mobile runtime are
explicitly out of scope. Desktop performance and native integration will not be
weakened to preserve hypothetical future mobile compatibility.

All three backends use direct operating-system window and GPU APIs with no
WebView and no bundled rendering runtime.

## Current Status

- Native macOS window and event loop.
- Direct Metal clear and present.
- Flat generational retained scene with stable node identities.
- Coalesced O(1) localized rectangle updates with zero hot-path allocations.
- Precompiled Metal shader and instanced rectangle rendering.
- Event-driven C-to-Go pointer dispatch and 32-byte partial Metal uploads.
- Paint-order hover and press interaction state with click activation;
  moving the pointer inside one region performs no renderer work.
- Keyboard focus with Tab traversal, Enter and Space activation, and focus
  rings; one key event repaints at most once and publishes accessibility
  updates only when focus actually changed.
- Event-driven scrolling with per-primitive clipping: a 1000-row list
  virtualizes to its visible window, one scroll gesture repaints exactly once
  with one partial GPU upload, and clipped content never reaches the
  renderer.
- Three native window modes: standard titled, frameless (no traffic lights),
  and custom titlebar (transparent title bar, full-height content, the window
  moves from the regions the page declares). Window close can be intercepted
  by the application.
- Explicit multi-window applications through `app.RunApplication`: every
  window owns independent retained paint, focus, input, text, invalidation, and
  close state, and the native loop exits after the final window closes.
- Status-area icons and popup panels: `Application.AddStatusItem` puts an icon
  in the macOS menu bar, the Windows notification area, or a Linux
  StatusNotifier host; `StatusItemConfig.Menu` makes it open the system's own
  menu (NSMenu, a Win32 popup menu, `com.canonical.dbusmenu`); `WindowPanel` is
  the borderless popup that opens beside it and closes when it loses focus;
  `Window.Hide`/`Show` keep an application running without a visible window.
- `Application.SetDockBadge` puts a label in the red badge macOS draws at the
  corner of the application's Dock tile, and an empty label removes it.
- `app.StoreSecret`/`LoadSecret`/`DeleteSecret` keep a small secret in the
  system's own store (Keychain, Credential Manager, Secret Service).
- `ui.MonoFamily` draws fixed-pitch text in any installed family and
  `app.MonospaceFamilies` lists them.
- `Config.OnFileDrop` receives files dragged from the file manager, with the hit
  under the pointer.
- A separate retained semantic accessibility tree mirrored into native
  NSAccessibility elements, Windows UI Automation providers (Invoke, Toggle,
  Value, RangeValue, SelectionItem, ExpandCollapse and Text patterns, with
  events) and Linux AT-SPI objects, and into ARIA in the browser build. A
  screen reader's press, focus and set-value requests reach the application
  through `platform.AccessibilityHandler`. See
  [docs/accessibility.md](docs/accessibility.md).
- Event-driven window-resize repainting with exactly one GPU submission per
  resize and zero work while the size is unchanged.
- `Config.OnFrameChange`, `Config.OnMinimize` and `Window.Minimized` report
  where a window is and whether the window manager minimized it.
- Retained Row/Column/Stack/Flex layout with O(1) unchanged-layout checks.
- Paint-order pointer routing, stable keyboard focus, and Tab traversal.
- Native macOS `NSTextInputClient` integration with real Chinese IME candidate
  composition, grapheme-aware editing, and UTF-16 mapping.
- Focus-driven dynamic native text editors on macOS, Windows, and Linux, with
  explicit enable/replace/disable frame updates. macOS and Windows caret timers
  exist only while an editable field is focused. Linux composes through the IBus
  protocol (IBus and Fcitx5's IBus frontend); XIM is still open.
- A separate retained semantic accessibility tree.
- Direct CoreText shaping, system-font fallback, GPU-composited text textures,
  and a bounded measurement cache.
- Experimental Windows shell with Win32 events, D3D11 retained rendering,
  embedded DXBC shaders, DirectWrite measurement, and bounded GDI label
  textures; native builds do not load a shader compiler.
- Experimental Linux/X11 shell with dynamically loaded system XCB, Vulkan,
  Pango, and Cairo; retained rounded rectangles and triangles, layered
  GPU-composited system text, pointer and keyboard input, custom titlebar drag,
  and a blocking event loop with no idle redraw.
- Event-driven rendering with no idle frame loop.
- Theme tokens and a statically typed, source-owned translation of the pinned
  shadcn component catalog, including forms, overlays, menus, calendars,
  scrolling recipes, feedback surfaces, and area, line and bar charts drawn
  from typed slices without a browser runtime, reflection, or a runtime style
  parser.
- A shared native/WASM component browser: a catalog of every component grouped
  by category, and a page per component that runs all of its examples side by
  side, each live (clicks, hover, keyboard, drag, wheel and native text
  editing), with a draggable page scrollbar and hot-reload state preservation.
  The examples are the ones in `internal/componentdemo`, embedded into the page
  with `ui.Canvas.Embed`, so the gallery holds no copy of any component.
- Clean event-loop return on window close or application termination.
- Reproducible binary, startup, CPU, memory, and GPU-submission measurement.

The Windows backend cross-builds for amd64 and arm64 and passes its platform
tests plus Gallery startup, pointer, and keyboard smoke tests under Wine/DXVK.
Platform tests, the conformance scenarios and the input, drop and device-loss
paths run on a Windows 11 virtual machine (WARP) and a Debian KDE/X11 machine
(lavapipe); `docs/conformance.md` records the scenarios that need real hardware.
UI Automation is checked against a real Windows 11 machine from an external
client (see docs/accessibility.md), and AT-SPI against libatspi on KDE/X11. The
Linux backend passes Ubuntu arm64 Xvfb/llvmpipe M0 and Debian amd64
X11/Lavapipe retained-scene smokes, including mixed Chinese/Latin text and
localized scene updates. The macOS runtime tests and benchmarks run on arm64.

Windows Mica, Mica Alt and Acrylic are enabled through `app.Config.WindowMaterials`
without changing build commands. Unconfigured windows keep the ordinary renderer. See [Windows materials](docs/windows-materials.md) for version
limits, fallback behavior and the platforms actually verified.

Native label renderers draw single-line labels. The browser preview runs on the
Canvas2D backend.

## Run the repository

These commands build and drive the framework's own programs from this checkout.

```sh
go run ./cmd/showcase        # the native component showcase
go run ./cmd/component-preview -component button -example variants   # one example, natively
tools/build_web.sh           # viva build --target web, then serves http://127.0.0.1:8090
go run ./tools/docs -check   # the API and component data the website generates from
```

The browser gallery paints the same Go components through the Canvas2D
WebAssembly backend; its visual direction and measured costs are recorded in
`docs/benchmarks/m3-gallery-style-darwin-arm64.md`.

## Documentation website

[vivaui.dev](https://vivaui.dev/en/) is the public site, in English and Chinese:
guides whose code blocks are files under `examples/guides/` that `go test` builds
and drives headlessly, an API reference generated from the Go source, and a page
per component with a live Go/WASM preview. The Astro + Svelte source is in
[`website/`](website/); it is not part of the native UI or the release runtime.

```sh
cd website
npm ci
npm run dev          # http://127.0.0.1:4321/zh/ and /en/
npm test && npm run build
```

See [website/README.md](website/README.md) for the preview pipeline, the
generated data and `test:site`.

## Build and ship an application

`viva build` makes a release program for `--target darwin`, `windows`, `linux`
or `web` (`-trimpath`, stripped, a fixed build ID, so the same sources give the
same bytes) and reports its size apart from anything around it. `viva package`
adds the platform's own installer. Both read an optional `viva.toml` next to
`go.mod`:

```toml
[app]
name = "Viva Gallery"       # shown in the menu bar; names the .app
id = "dev.viva.gallery"     # bundle identifier
package = "./cmd/showcase"  # the main package to build

[mac]
min_system = "12.0"         # the macOS the program is compiled for; floor 12.0
```

```sh
go run ./cmd/viva build --target windows --arch amd64
go run ./cmd/viva package --zip
go run ./cmd/viva help package     # every command and flag
```

`viva.toml` is decoded strictly: an unknown table or key, a duplicate or a value
of the wrong type is an error that names the line. A macOS program is compiled
for `min_system`, not for the macOS of the machine building it, and `package`
refuses a bundle that claims an older system than its program needs. Signing and
notarization are opt-in through `--sign-identity` or `APPLE_SIGNING_IDENTITY`
plus notarytool credentials; `--no-sign` turns both off. `--arch amd64` builds
the other Darwin architecture from either host: the build keeps cgo and the
Objective-C/Metal bridge and lets clang emit the other Mach-O slice. The
[Build and package guide](https://vivaui.dev/en/docs/build-and-package/) covers
every target, installer format and flag.

The release link is measured, not assumed. `go-toml/v2` is used only by the
`viva` command and `purego` only by the Linux backend, so neither is in a macOS
or Windows program. The development reload protocol, single-instance sockets,
PNG icon decoding and the Go test runtime stay out of release programs (reload,
single instance and PNG are opt-in packages; the test runtime is absent), and
leak counters remain because they cost two atomics per window or driver.

A static canvas can be presented through the public API:

```go
theme := ui.DarkTheme()
var canvas ui.Canvas
components.Card(&canvas, theme, ui.Rect{X: 24, Y: 24, Width: 320, Height: 180})
if err := app.RunCanvas(app.Config{Title: "My App"}, &canvas); err != nil {
    log.Fatal(err)
}
```

## Engineering Rules

The performance budgets, dependency policy, architecture boundaries, and
definition of done are enforced by the checked-in benchmarks and tests.
Measurements are evidence; terms such as
“native speed,” “zero idle work,” and “lightweight” are not used as unverified
marketing claims.

## Contributing

Bug reports, platform findings, and documentation fixes are always welcome — see
[CONTRIBUTING.md](CONTRIBUTING.md) for the gate a change has to pass and for
where the line between framework and application behaviour sits. Security
problems go through [SECURITY.md](SECURITY.md), never a public issue.

## Contributors

<a href="https://github.com/vivaui/viva/graphs/contributors">
  <img src="https://contrib.rocks/image?repo=vivaui/viva&max=300&columns=15" alt="Contributors" />
</a>

## Community

<a href="https://linux.do"><img src="https://img.shields.io/badge/LINUX%20DO-Community-blue" alt="LINUX DO" /></a>

## License

Viva is licensed under the Apache License 2.0. See [LICENSE](LICENSE) and
[THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).
