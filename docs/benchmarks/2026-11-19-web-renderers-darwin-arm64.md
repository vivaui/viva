# Browser renderers: WebGL2 and Canvas2D

Measured 2026-11-19 on Apple M2 Pro, 16 GiB RAM, macOS 15.5 (24F74),
darwin/arm64, Go 1.26.0. These are local observations, not budget-compliance
claims. The browser runs were headless Playwright builds on the same machine.

## Scope

The browser target now renders `platform.Frame` with WebGL2 and falls back to
Canvas2D at runtime. This report records the artifact sizes and the browser
behavior that were actually exercised. It does not claim a steady-state frame
time for a large application yet; that benchmark still needs a dedicated scene
with a large list, editor and charts.

## Artifact size

The fixture is the browser verification program
(`tools/webcheck/fixture`) built with:

```sh
go run ./cmd/viva build --target web ./tools/webcheck/fixture -o <dir>
```

| Build | Stripped wasm | gzip |
| --- | ---: | ---: |
| Frame renderer with Canvas2D only (before WebGL2) | 5,504,604 B (5.25 MiB) | 1,557,350 B (1.49 MiB) |
| WebGL2 renderer plus Canvas2D fallback | 5,801,972 B (5.53 MiB) | 1,637,943 B (1.56 MiB) |
| Difference | +297,368 B (+0.28 MiB) | +80,593 B (+0.08 MiB) |

The increase is the GLSL shaders, the WebGL2 renderer, per-label texture
management and the Canvas2D bitmap path. No new Go module dependency was
added. Native builds do not link any of it: the work is under `js && wasm` and
the shared frame builder is the same code moved out of `app/painter`.

## Browser behavior

`tools/webcheck/check.mjs` ran against Chromium 158.0.8127.18, Firefox 159.0
and WebKit 27.2 in both forced renderers:

```sh
go run ./cmd/viva build --target web ./tools/webcheck/fixture -o build/webcheck
node tools/webcheck/check.mjs --dir build/webcheck --browser all --renderer webgl2
node tools/webcheck/check.mjs --dir build/webcheck --browser all --renderer canvas2d
```

All three engines passed the pointer, keyboard, focus, slider, text-editor,
IME composition, backdrop, pass-through and accessibility checks in both
renderers. The fixture also draws a solid bitmap, and the gate samples that
pixel from its screenshot in both renderers. The forced-WebGL2 run also lost
the context with `WEBGL_lose_context`; the page reported `canvas2d`, kept
pointer input working, and the subsequent click reached the application.

The docs previews were built through the same `viva build --target web`
pipeline and rendered in WebGL2 and Canvas2D. Button, Image View, Code Editor,
Chart and Slider previews produced matching screenshots in both renderers,
including the Image View bitmap that the old Canvas2D renderer did not draw at
all.

## Additional hardening

- The fixture draws a warped label (`gfx.Canvas.LabelQuad`); both renderers
  painted it. WebGL2 uses the four-corner quad and per-corner tint; Canvas2D
  uses horizontal strip subdivision and a corner-to-corner tint gradient.
- WebGL2 labels wider than `MAX_TEXTURE_SIZE` are split into overlapping
  horizontal tiles. A run with `window.vivaMaxTexture = 64` forced the fixture's
  button and editor labels across several tiles and rendered without errors or
  visible seams at 640×480.
- The browser fixture now also draws a solid bitmap; the gate samples its pixel
  from the screenshot under both renderers.

## Not measured

- Steady-state frame time and GPU submission for a large application scene.
- Texture memory under a long scrolling session with thousands of labels.
- Native binary size and startup; the web renderer is excluded by build tags,
  but a before/after native link measurement was not part of this run.
- Real Safari via `safaridriver`: `tools/webcheck/safari.mjs` is provided, but
  remote automation is not enabled on this Mac. Playwright's WebKit is not the
  same browser shell.
- Physical IME candidate windows; composition is synthesized at the DOM level.
