# windows: add runtime-configured Acrylic, Mica and Mica Alt

Local Draft PR text; not published to GitHub.

## Summary

Add native Windows window materials through `app.Config.WindowMaterials`,
without additional build flags. Leaving `WindowBackdrop` nil keeps the ordinary
renderer and avoids material resources, policy queries and power subscriptions.
Providing the callback selects composition at window creation; it can then
switch between materials and `BackdropNone` at runtime. Disabling a material
retains that opted-in window's composition renderer for subsequent switches.

Windows 11 22H2+ uses DWM system backdrops for Acrylic, Mica and Mica Alt.
Windows 10 1809 through Windows 11 21H2 uses the undocumented Accent Policy
for Acrylic only. OS version and API availability checks gate both paths.
Unsupported materials or native failures fall back to the application's opaque
clear color. High contrast and disabled transparency suppress all materials;
battery saver suppresses Acrylic. System notifications and device recovery
reapply the requested material without duplicate activation callbacks.

Translucent controls paint hollow rounded outlines instead of opaque border
backings. Specialized tessellation removes the generic polygon-with-hole
triangulation cost. Source-owned component templates are regenerated.

Two test assumptions are also corrected: downloaded Windows toolchain fixtures
must contain `go.exe`, and an explicit YaHei fallback may match DirectWrite's
system fallback. Production toolchain/font behavior is unchanged.

## Usage

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

Applications should paint translucent content only while `OnBackdropChange`
reports active. The callback reports API activation, not guaranteed visible
blur. Other platforms retain their existing rendering and do not invoke it.

Complete reproduction, including fallback-aware painting:

```sh
go run ./examples/windows-materials -material mica-alt
```

Also accepts `-material mica` and `-material acrylic`.

## Costs

Windows amd64, Go 1.27.3. Original baseline: `8e45fdc`.

| Measurement | Original baseline | Current runtime configuration | Increase |
| --- | ---: | ---: | ---: |
| Stripped Hello executable | 3,764,224 B | 3,795,968 B | 31,744 B (0.84%) |
| `windowState` | 1,736 B | 1,808 B | 72 B |
| `renderer` | 368 B | 400 B | 32 B |
| `platform.Config` | 400 B | 416 B | 16 B |

Binary command: `go build -trimpath -ldflags='-s -w' ./examples/guides/hello`.
Current structure sizes were rechecked using `unsafe.Sizeof`. These are shallow
structure sizes, not total process/GPU memory. Material resources are additional
for opted-in windows. Unused material support still adds shared code and fields.
The contributor accepts that tradeoff for unchanged build commands; upstream
acceptance remains a review decision under the contribution guide's size policy.

The recorded focused translucent-input benchmark improved from a median
4,368 ns/op at `d5fd167` to 647.9 ns/op after outline specialization, with zero
steady-state allocations. A later run measured 796.1 ns/op; it was not a paired
comparison. These historical measurements describe the outline optimization,
not a new benchmark of this PR draft. See
`docs/benchmarks/2026-10-31-rounded-outline-windows-amd64.md`.
The separate build-tag experiment report is historical and does not represent
this implementation's footprint.

## Validation

On Windows build 26300, amd64:

- `go test ./... -count=1` and `go vet ./...` passed without failure exclusions.
- Material example builds with ordinary `go build`, without additional tags.
- Focused WARP material, fallback, rounded-outline and translucent-control tests passed.
- Cross-vet passed for Windows arm64, Linux amd64 and the Web backend/app.
- Native coverage includes material switching, resize/device recovery, failed
  API/compositor fallback, custom titlebars, simulated policy changes and power
  subscription cleanup. Offscreen pixel tests cover multiple DPI scales.
- Formatting and `git diff --check` passed.

## Why this is a draft

- The contribution guide asks for advance issue discussion of larger API and
  rendering changes. No upstream agreement or linked issue is established here.
- The ordinary-build footprint increase needs explicit upstream review.
- The Win10 Acrylic path uses an undocumented API and needs compatibility review.
- Windows 10 native integration skips on this Windows 11 host. Windows 10,
  Windows 11 21H2, ARM64 hardware, RDP, actual power/high-contrast transitions
  and visual screenshot review remain unverified.
- The macOS/headless gate and hosted CI have not run for this change. API success
  and unit-test policy simulations do not establish visual fidelity on all systems.
