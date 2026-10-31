# Windows window materials

Set `app.Config.WindowBackdrop` to return an `app.BackdropStyle`. Its `Kind`
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

## Verification on 2026-10-31

Host: Windows build 26300, amd64, Go 1.27.3. Native tests passed using the default
graphics path and with `VIVA_SOFTWARE_RENDERING=1` (WARP):

- Mica / Acrylic / Mica Alt / None switches, dark/light changes and resizing.
- Hardware/software device recovery and premultiplied transparent clears.
- Custom titlebar controls, system menu and sizing styles.
- Simulated high-contrast, transparency and battery-saver transitions, including
  clear-color restoration and callback behavior. User settings were not changed.
- Reapplying Mica Alt after clearing the window's DWM attribute.
- Failed DWM calls restoring ClearColor and retrying after a settings event.
- Missing DirectComposition entry-point injection followed by successful
  ordinary-swap-chain presentation.
- Power notification registration, repeated registration and cleanup.

`go vet` passed for the host and Windows arm64, Linux amd64 and Web targets.
The full repository test run passed with the two previously reproduced baseline
failures excluded: `TestFallbackFamiliesDrawTheCharactersThePrimaryLacks` and
`TestInstallDownloadsVerifiesAndUnpacksTheNewestStableToolchain`.

The Windows 10 native integration test was skipped because this host uses the
modern DWM path. Windows 10, Windows 11 21H2, physical arm64 hardware, Remote
Desktop, real power/high-contrast transitions and a visual screenshot review
remain unverified. Version-policy unit tests are not substitutes for those runs.

Run the focused suite with:

```powershell
go test ./internal/platform/windows -run 'Test(Backdrop|WindowBackdrop|CompositionClear|CompositionFallback|CompositionCreation|CustomBackdrop|AcrylicAccent|LegacyAcrylic)' -v -count=1
$env:VIVA_SOFTWARE_RENDERING = '1'
go test ./internal/platform/windows -run 'Test(Backdrop|WindowBackdrop|CompositionClear|CompositionFallback|CompositionCreation|CustomBackdrop|AcrylicAccent|LegacyAcrylic)' -v -count=1
Remove-Item Env:VIVA_SOFTWARE_RENDERING
```
