# Platform conformance

Viva keeps one public API over three native backends. `cmd/viva-conformance` is how
we find out that they still mean the same thing: it runs the same lifecycle
scenarios through the public `app` package on each platform, and prints PASS,
FAIL or SKIP for each.

```sh
tools/conformance/run.sh mac        # this Mac
tools/conformance/run.sh linux      # the Linux test container
tools/conformance/run.sh windows    # the Windows test VM
tools/conformance/run.sh all
VIVA_CONFORMANCE=1 go test ./cmd/viva-conformance   # on the machine you are on
```

The container and the VM are machines you provide; set `VIVA_LINUX_ENV` and
`VIVA_WINDOWS_ENV` (see `tools/testenv/env.sh`). The program opens real windows,
so it needs a desktop session.

## What it checks

| Scenario | Why it matters |
| --- | --- |
| A window paints, closes from another goroutine, and closing twice is harmless | `Window.Close` is documented thread-safe and idempotent |
| An idle window does no work | the runtime's idle contract: no paint, no wake without a request |
| Watching the system appearance costs an idle window nothing | on Linux a watcher opens one session-bus connection that the event loop polls; it must add no goroutine, no wake and no report of a change nobody made |
| `Invalidate` from another goroutine wakes the window once | every background update goes through it |
| A hidden window stays open and still receives wakes | an application that hides to the status area updates its icon from a window nobody can see |
| A panel opens, paints and closes from another goroutine | `WindowPanel` is a window kind of its own on each backend |
| A status item can be added, changed and removed (SKIP where the desktop has no status area) | `AddStatusItem` and its setters on three different APIs |
| `Quit` ends the loop though windows refuse to close, even as the last-used window closes | the loop must end whatever the windows say |

It was written after the status-icon work, and each check is something a real
platform got wrong first. The last scenario fails on the Linux backend as it was
before `Quit` posted to every window, and passes now.

## What it cannot check

These need input or eyes, so they are checked by hand on each platform, with a
screenshot, when the code behind them changes:

- a click on the status icon reaches `OnClick` exactly once, per button (Windows
  reports one click as two messages, and only a real click shows it), or opens
  the system's menu when the item has a `Menu`, which closes on a click elsewhere
  and shows what the application listed when it opened, not what it listed the
  time before (KDE keeps what it was told about an id);
- a panel closes when it loses focus, and the click that did it does not reopen it;
- the window manager's close button reaches `OnCloseRequest`;
- a hidden window is repainted when shown again (a compositor drops its contents);
- a custom-titlebar window with a drawn frame on Linux (`VIVA_LINUX_FRAME=on`): shadow, round corners, a resize from the margin, maximize and restore;
- a custom-titlebar window (`cmd/titlebar-demo`): minimize, maximize and restore,
  close (reaches `OnCloseRequest`), a drag of the strip, a double click on the
  strip, a drag of each edge and corner, and a click on a window button that does
  not reach `OnClick`; on Windows the system caption must not be drawn and a
  maximized window must stop at the taskbar;
- where a panel and a new window are placed on a multi-monitor or scaled screen (Linux: not
  verified with more than one monitor or a real HiDPI display);
- text, which each platform draws with its own engine and is not expected to match.

## Known differences

Not failures; the API reports what the platform can say.

| | macOS | Windows | Linux/X11 |
| --- | --- | --- | --- |
| Status icon position | the item's rectangle | the item's rectangle (an overflowed icon reports its place in the flyout) | the click point only |
| Panel dismissal | resign key | deactivation | `FocusOut`, from a dialog-type window (KWin gives no focus to utility or popup windows) |
| New icon with a full bar | hidden by the system, notably next to the notch | shown in the overflow flyout until promoted | up to the host |
| Corner radius of a panel | the layer's | the system's on Windows 11 (`DWMWA_WINDOW_CORNER_PREFERENCE`, set in the code, not yet checked on screen), square on Windows 10 | square |
| Shadow and round corners of a custom-titlebar window | the system's | the system's | drawn by Viva where the desktop composites and the graphics device is a GPU ("Window frame on Linux"); otherwise flat and square |

## Native Wayland on Linux

The Linux backend has a native Wayland implementation in addition to X11: it
uses xdg-shell, Wayland input and frame callbacks, fractional/integer scaling,
Vulkan WSI, IBus and AT-SPI. The `linux-platform` CI job runs its unit and
protocol-fake tests on Ubuntu without a display or GPU.

This is implementation and test coverage, not real-compositor acceptance. The
interactive Linux environment below is still KDE Plasma on X11 with software
Vulkan. A native Wayland compositor/GPU matrix has not been validated, so the
manual scenarios above do not yet establish Wayland conformance.

## Not done, and why

What the Windows and Linux backends do not have yet, recorded so that nobody
mistakes an absence for an oversight. Items marked *measured* were tried or sized.

| Gap | Platform | Reason |
| --- | --- | --- |
| Smooth scrolling (`Precise`) | Linux | It needs XInput2 valuators, and a window that selects XI2 events stops receiving the core pointer events, so the whole pointer path would move. The test machine's Xvnc has no scroll valuator, so the code could not be verified; none was added. |
| Text Services Framework | Windows | IMM32 is used (candidate placement, reconversion, clause styling). TSF means implementing `ITextStoreACP`; not done. |
| Toast notifications | Windows | They need a Start-menu shortcut carrying an AppUserModelID, and NSIS cannot write that property without a plugin. A balloon is shown instead. The process AppUserModelID is set only when the application sets `Config.Class`, so a pinned shortcut and its window group together. |
| Notification click callbacks | all | `Notification` has none on any platform; adding it to one would make the three differ. |
| Full-screen mode | Windows, Linux | `WindowActions` has no full-screen member. The Linux side already treats `_NET_WM_STATE_FULLSCREEN` like maximize (no margin); the API wants `ToggleFullscreen` and `Fullscreen`. |
| A native application menu | all | The contract has a status-item menu only; macOS installs a Quit item. |
| Colour management, HDR, wide gamut | all | Every backend writes sRGB-ish values; nothing converts. Not measured. |
| Arm64 UI Automation | Windows | The floating-point thunk exists for amd64 only. |
| AppImage, rpm | Linux | AppImage needs an external runtime binary; rpm could not be checked here. `viva package` makes tar.gz and .deb. |
| Native text events in `viva dev` | all | The worker protocol does not carry them; native Canvas examples do not run under `viva dev`. |
| A flat accessibility tree | Windows, macOS | Every node hangs from the window; Linux derives parents from geometry. |
| XIM input | Linux | Composition goes through IBus (IBus and Fcitx5's IBus frontend); XIM-only applications cannot compose. |

## Test environments

Windows is a Windows 11 virtual machine with a display-only GPU, so Direct3D runs
on WARP; Linux is KDE Plasma on X11 with software Vulkan. macOS is arm64; Intel
Macs, their GPUs, IME and text rasterization are not part of that matrix. None of
these machines is evidence for GPU performance, only for behavior.

## Machines without a GPU

A window needs a graphics driver, not a GPU. Windows falls back to WARP, Direct3D's
software device, by itself. Linux needs a Vulkan driver in software form, Mesa's
lavapipe (`mesa-vulkan-drivers`, plus the loader `libvulkan1`); without one
`OpenWindow` fails with an error that wraps `app.ErrNoGraphics` and names what to
install. macOS always has Metal. A server with no display at all (most cloud
machines) cannot open a window on any platform: give it a virtual display (Xvfb or
Xvnc on Linux) or use the headless build (`-tags vivahead`), which renders offscreen.
The Linux test machine is a virtual display plus lavapipe. The case of no Vulkan
loader at all is covered by a unit test, not by a machine without the library.

## Visual parity

The conformance run above checks that the platforms *behave* alike; `tools/visualparity`
checks that they *look* alike. The rule: sizes, spacing, colors,
shapes and the layout of everything that does not depend on a font are the same on macOS,
Windows and Linux. **Text is not**: Viva lays a line out, draws it, and places its caret,
selection and clicks from the one line the platform's own engine shaped (CoreText,
DirectWrite, Pango, the browser; `ui.TextEngine`), as a native application does. A string
is therefore as wide as the glyphs that are drawn, nothing is squeezed, and the widths of
auto-sized buttons, tabs and tooltips differ between platforms by the difference between
their fonts. A layout that must not move with the font declares its sizes (`flex`).

The reference metrics (`internal/textmetrics`: the advance widths of a reference font plus
rules for CJK, emoji and the rest) remain as the measurer of unit tests and of
`tools/visualparity`'s layout golden (`headless.Options.ReferenceText`), so that golden and
`internal/goldens` do not depend on the machine that wrote them. A headless screenshot
measures with the platform's engine, as a window does.

```sh
tools/visualparity/run_all.sh                    # all three, results in /tmp/parity-{mac,linux,windows}
tools/visualparity/run_all.sh windows            # one of them
go run -tags vivahead ./tools/visualparity run -out /tmp/parity-mac      # macOS by hand (vivahead for the renderer)
go run ./tools/visualparity compare -out /tmp/parity-diff /tmp/parity-mac /tmp/parity-windows
go test -tags vivahead ./tools/visualparity       # the guards below, on this machine
tools/testenv/go-test.sh linux ./tools/visualparity     # and on the Linux and Windows machines
```

`run` renders a fixed catalogue with the platform's own offscreen renderer (the one
`headless.Session.Image` uses; no window is opened): a form (buttons in every variant
and size, switches, checkboxes, fields, a segmented control, a closed select, a slider
and a progress bar), an open select, a sorted table with a selected row, two tab strips,
a dialog over a scrim, a scrolled and clipped list, cards with borders of 0.5 to 3
pixels, corner radii of 0 to 16 and circles, text specimens (sizes 11 to 24, the four weights,
Latin, CJK, emoji, mixed, mono in both faces, wrapped paragraphs at three widths, and for the
complex scripts Arabic, Persian, Hebrew, Thai, Hindi, Bengali, Tamil, Telugu, Korean, Khmer and Myanmar, mixed direction
lines and wrapped paragraphs of the complex scripts) and a
baseline probe; each in the light and the dark theme, at 1x, 1.5x and 2x. For each it writes a PNG and a JSON of the canvas's layout
facts: every text with its bounds, style and the width layout gave it (the reference
metrics, for the golden), every rectangle, triangle and hit region. The facts are read from the canvas, so they do not depend on rasterization.

`compare` reports, for two platforms:

| Report | What it says |
| --- | --- |
| Text measurement | the platform engines' natural width ratio of the same string over sizes 11 to 24 and four weights, by script; the fonts' line boxes; the fixed-pitch cells; the fonts fontconfig resolves on Linux (diagnostic: layout does not use any of it) |
| Layout facts | every text, rectangle, triangle and hit that moved by more than 0.5 px, grouped by kind and axis; items that exist on one side only are structure changes (a different line break, for one). Expected: none |
| Pixels outside text | text bounds masked: differing-pixel ratio, mean and maximum difference, and each differing pixel classified as noise (up to 2 levels off an edge), flat (a fill of another color), aa (an edge pixel that differs by up to 96 levels: anti-aliasing) or shape (an edge pixel that differs by more: the edge is somewhere else); diff PNGs |
| Glyph placement | from the baseline probe, in pixels from the top of the line box: baseline row, cap height, x-height and relative ink mass of the same text on both |

### Tolerances and guards

`go test ./tools/visualparity` (with `-tags vivahead` on macOS) runs on each platform:

- the layout of every scene (light theme), texts included, measured with the reference
  metrics, equals the committed golden `tools/visualparity/testdata/layout.json` within
  **0.5 px**: it fails when Viva's own layout, the reference metrics or a platform's
  rounding changes. A deliberate change regenerates it on
  any platform (the golden does not depend on the machine):
  `go test ./tools/visualparity -run LayoutIsTheSame -update`;
- the dark theme moves nothing;
- the ink of a line drawn at 11 to 20 px in four weights (Latin, and digits before CJK) starts
  within 0.2 em of where the platform's text engine puts it, and ends inside the advance box
  of its last glyph: how far short of the engine's width the ink stops is that glyph's side
  bearing, which the font decides (Noto Sans's `1` carries a quarter em), but the ink may not
  stop before the glyph begins. Layout and drawing use the same shaped line, so a font that
  does not scale with the size, a semibold emboldened from regular, or a draw path that shapes
  differently from the measuring one leaves the band;
- each platform draws its specimens through the label rasterizer and as a platform
  application does, in one native call, and the label's edge energy per ink and its share of
  solid pixels stay at the native drawing's (`internal/textsharp`, the sharpness and
  fidelity tests of each platform package; macOS needs `-tags vivahead`).

Pixel results are reported, not asserted, except the Windows edge test
(`TestRoundedEdgesAreFeatheredOverOneDevicePixel`): on a GPU-less machine the images come
from software rasterizers, which are not evidence for a GPU's rounding.

### Known differences, by design

- Glyph shapes, stroke weight, hinting and the width of every string: each platform draws and
  measures with its own font (SF, Segoe UI, and Noto Sans or whatever the desktop names for
  sans), so a label that is as wide as its text is as wide as that platform's text.
  Layouts built from declared sizes do not move.
- The columns of a terminal do not depend on the font: a cell is 0.6 em by 1.18 em
  everywhere; the terminal grid is unchanged.
- The font of a script nobody asked for: CJK comes from PingFang (macOS), the font linked
  to Segoe UI (Microsoft YaHei UI on the Windows test machine) or Noto Sans CJK (Linux).
- Emoji: Apple Color Emoji, Noto Color Emoji and (through DirectWrite) Segoe UI Emoji are
  colored.
- Fixed-pitch faces: SF Mono, Consolas and Noto Sans Mono advance 0.62, 0.55 and 0.60 of the
  size; a terminal is laid out on a 0.6 by 1.18 grid on all of them, the platform's characters
  stand on it (Consolas looser than its own spacing). Non-ASCII runs of a grid keep their own
  advances.
- Windows draws text with DirectWrite at fractional advances, so a word can be
  off its layout width by the difference of the fonts; CJK widths vary with the size
  (a 13 px Chinese string is 1.24 to 1.32 times macOS's natural width).
- Linux has one display scale per window, found from the user's settings (see "Display scale on
  Linux"), not per monitor: X11 has no per-monitor scale, so a window moved between monitors
  of different density changes only when the scale came from RandR.
- Software rasterizers are the Linux and Windows machines' (lavapipe, WARP). Their
  per-pixel rounding (the scrim over white is 127 there and 128 on Metal) is not that of a GPU.

## Display scale on Windows

The window's scale is the monitor's dpi over 96 (`GetDpiForWindow`, and `WM_DPICHANGED` when it
moves). `VIVA_SCALE` overrides it for Viva alone, snapped to quarters and kept between 1 and 8, as on
Linux: it draws a window at 125% or 150% on a display at 100%, which is how fractional scaling
was measured. The renderer puts every label texture and image on a whole device pixel.

## Window frame on Linux

A `WindowCustomTitlebar` window has no frame from the window manager, so on X11 it has
no shadow and square corners. On a desktop that composites (a `_NET_WM_CM_S0` owner)
and with a GPU, Viva makes the window 24 logical pixels larger on every side, draws the
content in the middle with 10-pixel round corners and a soft shadow in the margin, and
tells the window manager with `_GTK_FRAME_EXTENTS` that the margin is not the window:
`Frame`, `OnFrameChange`, `OnResize` and pointer positions are the content's. The
margin takes input only 8 pixels out (a press there resizes), and it is dropped while
the window is maximized or fullscreen. The window has a 32-bit visual and its swapchain
composites with premultiplied alpha.

`VIVA_LINUX_FRAME` chooses: unset is automatic, `on` also draws it on a software device,
`force` skips the check for a compositor (tests), `off` never draws it. Software
devices are left out of the automatic choice because Mesa 22.3's software presentation
puts a 24-bit image into a 32-bit window, which the X server refuses, and the window is
then invisible; Mesa 25.0.7 presents it correctly.

Verified on the Linux test machine (KWin on X11, compositing with OpenGL, lavapipe from
Mesa 25.0.7 selected with `VIVA_LINUX_FRAME=on` and `VK_ICD_FILENAMES`): the window, its
shadow and corners on screen, a click on a control, a drag of the margin that resizes,
maximize and restore. Not verified: a real GPU, GNOME/Mutter, a HiDPI display.

During an interactive resize the window answers `_NET_WM_SYNC_REQUEST` after drawing the
new size (KWin sends one for each step; seen answered), and `_NET_WM_PING`. A lost Vulkan
device or surface is made again and the frame drawn again (tested with failures injected
at the Vulkan call; a real driver reset has not been caused).

## Display scale on Linux

Layout, window sizes, pointer and screen positions are in logical pixels on every
platform, and the window and the renderer work in device pixels: a window of
`Config.Width` x `Config.Height` is `round(size * scale)` device pixels, the swapchain
has that extent, canvas and label clips are rounded to device pixels (a clip edge to the
nearest one, as on macOS and Windows), and text is rasterized at its logical size times the
scale. `platform.Size` given to `OnResize`, pointer, scroll and drop coordinates, `ScreenX/Y`,
`Window.Frame` and `Config.Origin` are logical (device pixels over the scale); the input
method's caret rectangle and a panel's placement, which the host reports in pixels, are device.

The scale is the first of these that says anything (`internal/platform/linux/display_scale.go`):

1. `VIVA_SCALE`, then `QT_SCALE_FACTOR`, then `QT_SCREEN_SCALE_FACTORS` (the entry for the
   window's monitor, else the first without a name, else the first), then `GDK_SCALE`;
2. the XSETTINGS manager: `Xft/DPI` over 96 (144 is 1.5), else `Gdk/WindowScalingFactor`
   (GNOME keeps `Xft/DPI` at 96 and scales by the factor; a manager that says 96 ends the search);
3. `Xft.dpi` in the X resource database (`xrdb`);
4. RandR 1.5 (else the core screen): the monitor under the window's center, only when its
   physical size is plausible (at least 100 x 60 mm, 40 to 800 dpi, the same across and down);
5. 1.

It is snapped to quarters (1.25, 1.5, 1.75, 2 ...), never below 1 and at most 8. It is read when
the window opens, again when the XSETTINGS manager rewrites its property (the window listens to
that one window; nothing polls), when the window gains focus (picks up `xrdb` changes and a manager
that started later) and, only if the scale came from RandR and there is more than one monitor, when the
window moves. A change keeps the window's logical size (the window manager has the last word:
a maximized window keeps its device size). Cursors come from the user's theme through
`libxcb-cursor` when it is installed, which sizes them from `Xcursor.size` or `Xft.dpi`; they are
not scaled by `VIVA_SCALE` or the other environment variables alone.

Measured (`tools/visualparity`, Linux against macOS and Windows, `fixed` mode, text masked,
same machines as above; 15 scenes, light and dark):

| Scale | Linux vs macOS: edge pixels differing (aa) / shape / max level | Linux vs Windows: aa / shape |
| --- | --- | --- |
| 1x | 8,134 / 0 / 2 | 261 / 0 |
| 1.5x | 11,793 / 0 / 2 | 7,463 / 0 |
| 2x | 10,912 / 0 / 2 | 422 / 0 |

(macOS against Windows: 8,122, 15,180 and 10,801 at 1x, 1.5x and 2x.) The `flat` class in the
totals is the scrim over white (127 against 128) in the dialog scene. Layout facts agree to 0.00 px
at every scale. On the Linux machine a real window with `VIVA_SCALE=2`, `GDK_SCALE=2` and
`QT_SCALE_FACTOR=1.5` showed a 160 x 40 logical button as 320 x 80 and 240 x 60 screen pixels with
the same layout; a pointer press at 2x landed on the control it was over; changing `Xft.dpi` in
`xrdb` while the window was open resized it to 2x when it next gained focus.
