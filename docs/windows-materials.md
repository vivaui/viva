# Windows window materials

Set `app.Config.WindowMaterials` to opt into native materials at runtime.
Build, run, test and package commands need no additional tags. Leaving
`WindowBackdrop` nil keeps the ordinary renderer, without DirectComposition
resources, material policy queries or material power notifications. Configuring
the callback opts the window into the composition path at creation; returning
`BackdropNone` disables the material while retaining that renderer so the callback
can select a material later. Controls select translucent outlines from their
background alpha at paint time.

The shared implementation and configuration/state fields add a small baseline
cost even when materials are unused. Native resources are allocated only for opted-in windows on
supported systems.

```go
config := app.Config{
    ClearColor: app.Color{Alpha: 1},
    WindowMaterials: app.WindowMaterials{
        WindowBackdrop: func() app.BackdropStyle {
            return app.BackdropStyle{Kind: app.BackdropMicaAlt, Dark: true}
        },
    },
}
```

For a complete example, run:

```powershell
go run ./examples/windows-materials -material mica-alt
```

`WindowBackdrop` returns an `app.BackdropStyle`. Its `Kind`
selects the material and `Dark` selects its appearance. The callback runs on
the window thread. Use an opaque `ClearColor` (`Alpha: 1`) for a solid fallback,
and paint translucent content only while `OnBackdropChange` reports `true`.
Start application paint state with the material inactive: other platforms do
not invoke that callback. The callback reports API activation, not a measurement
of the visible blur; Windows may substitute a solid material for other reasons.

## Version and API restrictions

| Windows version | Acrylic | Mica | Mica Alt |
| --- | --- | --- | --- |
| Before Windows 10 1809, build 17763 | ClearColor | ClearColor | ClearColor |
| Windows 10 1809+ through Windows 11 21H2 | Legacy accent policy | ClearColor | ClearColor |
| Windows 11 22H2+, build 22621+ | DWM transient-window material | DWM main-window material | DWM tabbed-window material |

Version detection uses `RtlGetVersion` rather than manifest-dependent version
helpers. Entry points are checked before use. An unknown material value falls
back to `BackdropNone`. Windows amd64 and arm64 are the supported build targets.

The build-22621 floor for modern materials follows Microsoft's
[DWM_SYSTEMBACKDROP_TYPE documentation](https://learn.microsoft.com/en-us/windows/win32/api/dwmapi/ne-dwmapi-dwm_systembackdrop_type).
The legacy path uses dynamically loaded `SetWindowCompositionAttribute` and the
undocumented `WCA_ACCENT_POLICY`. It is restricted to builds 17763–22620 and is
a compatibility implementation, not the Windows App SDK acrylic controller.
Microsoft [does not recommend this API](https://learn.microsoft.com/en-us/windows/win32/dwm/setwindowcompositionattribute).
Its availability or a successful return does not establish visual fidelity on
every Windows 10 release or graphics driver.

## Runtime restrictions and recovery

- High contrast or disabled transparency suppress all three materials.
- Battery saver suppresses Acrylic; Mica and Mica Alt remain eligible.
- Policy changes restore the requested material when it becomes eligible again.
- These policies are read at initialization and on setting, theme, composition
  and power notifications, rather than querying the registry on each paint.
- A window opting into materials subscribes to battery-saver notifications;
  the subscription is released when the window is destroyed.
- Theme/DWM notifications and graphics-device recovery invalidate the native
  material cache. Reapplication does not emit a duplicate activation callback.
- A failed DWM/accent call restores the configured clear color. A later system
  notification can retry it; paints do not continually retry a failed call.
- DirectComposition creation or pipeline initialization failure releases its
  resources and attempts the ordinary window swap chain, restoring the window's
  redirection bitmap. Failure of that renderer is still an application error.
- Transparent render targets use premultiplied clears and grayscale text
  coverage. Custom titlebars keep their window actions, system menu and resize
  styles; the material does not replace them with native caption buttons.

The fallback color is application-owned. High contrast disables the material
but does not automatically replace every application color with system colors.
Remote Desktop, driver limitations and other DWM policy decisions may affect
the visible result; they are not inferred from the activation callback.

See Microsoft's [acrylic policy guidance](https://learn.microsoft.com/en-us/windows/apps/design/style/acrylic)
and [battery-saver status API](https://learn.microsoft.com/en-us/windows/win32/api/winbase/ns-winbase-system_power_status).
