# Native window watermarks

A native watermark is an input-transparent tiled text overlay above both canvas
content and embedded native views. It belongs to a window, including an
`OpenContentWindow` document or tool window. Text, identity and coverage policy
belong to the application.

```go
config := app.Config{
    Title: "Document",
    Width: 800, Height: 500,
    Watermark: app.Watermark{Text: "DRAFT"},
}
```

Text alone is visible: the zero color selects neutral linear gray at 12% opacity,
zero font size selects 12 logical pixels, and the tile defaults to 240 by 180
logical pixels. Text is centered horizontally and vertically using font metrics.
There is no SVG baseline parameter. Empty text, including `app.Watermark{}`,
removes the overlay. A nonzero color is used exactly, including its alpha.

For explicit styling, use existing framework types:

```go
config.Watermark = app.Watermark{
    Text: "CONFIDENTIAL",
    FontWeight: gfx.WeightLight,
    FontFamily: "Arial", // optional; empty uses the native system font
    Angle: -25,
    Insets: gfx.All(24),
}
```

The watermark's typography is explicit and independent of interface-font state.
Applications that want the same family pass their selected family when updating
the watermark. This avoids making window composition depend on the canvas text
engine or on process-global font changes. Native fallback handles missing faces
and glyphs. Weight uses the same scale and constants as `gfx.TextStyle.Weight`.

`app.Supports(app.CapabilityWindowWatermark)` reports native support on macOS and
Windows. Capabilities are static for a build target, rather than a runtime-host
promise. Other native platforms ignore initial configuration and return
`app.ErrUnsupported` for updates. The headless application host records normalized
state and accepts updates for lifecycle tests; it does not draw a native overlay.

`window.SetWatermark(value)` and `contentWindow.SetWatermark(value)` are safe from
any goroutine. Validation completes before a request reaches the host. Windows
coalesces pending requests and applies the latest one on its window thread;
macOS applies on the main thread. Success means the request was accepted, not that
a frame is already presented. Later native failures use the diagnostic sink.
Custom `app.Host` controls implement the optional public method
`SetWatermark(app.Watermark) error`; no internal package import is required.

Text must be valid UTF-8 without NUL or line breaks, at most 16 KiB. Color channels
must be finite and in [0,1], dimensions and insets finite and nonnegative, and
weight between `gfx.WeightLight` and `gfx.WeightBold`. Font size is at most 4096,
tile sides at most 8192 logical pixels; a native tile is additionally limited to
8192 pixels per side and 16 Mpixels at the actual backing scale. Invalid public
requests leave existing state intact. Windows restores prior configuration if
native application of an accepted request fails.

## Lifecycle and cost

No watermark view or popup is created for an empty watermark. Enabled macOS
windows retain a small backing-scale text tile and clipping/composition layers.
The inverse-rotated clip rectangle bounds the pattern plane and keeps its origin
stable during resize. An unattached view waits for the real window's backing
scale before generating a tile.

Enabled Windows windows retain a 4-byte-per-pixel client DIB, a small tile and
Direct2D resources. The client DIB is released while hidden or minimized.
Translation reuses retained pixels; rotation and insets do not rasterize text
again. Resize follows the window's existing resize scheduler. The owned overlay
tracks its owner's topmost promotion and demotion.

## Reproduce and verify

Run `go run ./examples/guides/watermark` on macOS or Windows. Verify button input,
resize, mixed-DPI moves, hide/show, and removal through `SetWatermark` with empty
text. To exercise native-view stacking, configure a web view in the same window
and verify that the watermark covers it while text selection and scrolling still
reach the page.

Automated contracts cover normalization, visible zero-value styling, bounds,
public-host forwarding, content-window forwarding and headless retention/close.
The native macOS fixture creates real CoreText lines and CGPatterns, tests
centering, backing-scale orientation, inverse-rotation coverage, view stacking,
clipping and input transparency, without drawing text or taking screenshots.
Windows native tests additionally exercise topmost transitions and retained
translation/resize behavior; cross-compilation alone does not verify those tests
on a Windows desktop. Real-device visual acceptance is separate from these
contracts.
