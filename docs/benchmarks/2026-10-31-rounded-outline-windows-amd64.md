# Rounded outline specialization

Host: Windows build 26300, amd64, AMD Ryzen 9 9950X, Go 1.27.3.
Baseline: `d5fd167`. Both versions use the same `BenchmarkInputPaintSurface`,
with a warmed, reused Canvas, a 200-by-32 focused input and a light theme.
The translucent case sets the page background alpha to 0.1.

```powershell
go test ./components -run '^$' -bench '^BenchmarkInputPaintSurface$' -benchmem -count=5
```

Runs were sequential, without concurrent test/build jobs. Values below are
the medians of five runs, not a statistical confidence claim:

| Case | Before | After | After allocations |
| --- | ---: | ---: | ---: |
| Opaque input | 101.7 ns/op | 91.11 ns/op | 0 B/op, 0 allocs/op |
| Translucent input | 4,368 ns/op | 647.9 ns/op | 0 B/op, 0 allocs/op |

Raw translucent samples: before 4389, 4392, 4354, 4331, 4368 ns/op;
after 648.9, 645.6, 646.8, 648.9, 647.9 ns/op. The translucent CPU scene-building
cost falls by about 85% (6.7x). These are not GPU frame times. The opaque path
is unchanged; the small difference there should be treated as measurement noise.
Earlier exploratory timings used a different run and should not be substituted
for this paired comparison.

The implementation directly connects matching vertices of the outer and inner
rounded rectangles. It avoids generic Bezier flattening, offset construction
and polygon-with-hole triangulation. Only boundary edges receive antialiasing;
shared edges stay fully covered. Arc resolution follows device scale, and the
bounded quarter-circle scratch array is stack-local. No cache or persistent
structure fields were added. Canvas triangle storage is reused after warmup.

The Hello example, built with `go build -trimpath -ldflags='-s -w'`, changes
from 3,789,312 to 3,792,384 bytes (+3,072 bytes). This CPU improvement does not
resolve the separately identified opt-in linking overhead of window materials.

Geometry tests cover sharp, rounded and capsule corners, widths below and
above the radius, empty interiors, nonoverlap and boundary-edge masks. Native
offscreen pixel tests check transparent interiors, seam coverage and alpha at
1x, 1.25x, 1.5x, 2x and 3x. This is a correctness/performance optimization;
it does not revert to the old, incorrect solid backing beneath translucent fills.

The temporary build-tag isolation was superseded by runtime configuration.
No extra build tags are required. Control surfaces select the corrected outline
path from background alpha; the benchmark workload remains the same.

Validation: host `go vet ./...`, Linux static checks, Web production builds and
the prescribed Web backend/app vet checks passed. Pixel tests passed on the
default Windows renderer and WARP. Full host tests passed excluding the two
previously confirmed baseline failures in font fallback and toolchain unpacking.
An additional Web `go vet ./components` check fails because the existing
`color_swatch_test.go` references `app.NewDriver`, unavailable on that target;
the same failure was reproduced in the unoptimized `d5fd167` snapshot.
