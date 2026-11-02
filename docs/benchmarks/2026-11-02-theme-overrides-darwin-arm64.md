# Per-control radius and font size: Darwin arm64

Reference machine: Apple M2 Pro, macOS 15.5, Go 1.27.3, native build. Binary sizes
are `-trimpath -ldflags="-s -w"`. Before is the commit this work started from
(`83da57f`, built in a separate worktree); after adds `ui.Theme.WithRadius`,
`WithFontSize` and `WithControlHeight`, `Box.Radius` and `Box.FontSize` on flex
controls, and the text-entry geometry that follows the font size and density.
Linux and Windows were not measured.

**The machine was busy** (load average 46 at the second batch of runs), so the
timings below are the best of several runs and carry a few percent of noise; they
are evidence that nothing got meaningfully slower, not a quiet-machine baseline.

## Allocations

Every path stays at zero allocations once warm, checked by tests:

- `TestInputPaintWithOverridesIsAllocationFree`: `PaintInput` with
  `theme.WithRadius(ui.RadiusFull).WithFontSize(16)` at a 40px height, with
  selection, composition and caret.
- `TestStyledControlsAllocateNothingOnceWarm`: a flex page with an `Input` and a
  `Button`, each with its own `Radius` and `FontSize`.

## Paint time

`PaintInput`, one focused field with selection, composition and caret, 200px wide,
default theme (best of five, 300,000 iterations):

```text
before   ~335 ns/op   0 allocs/op
after    ~351 ns/op   0 allocs/op   (+~15 ns: the text band is now derived from the font size)
```

`PaintTextarea` default: 251 ns before, 252 ns after.

An override costs one theme copy. `Theme` is 520 bytes; `th.WithRadius(4).WithFontSize(16)`
measured 80 ns/op against 7 ns for a plain copy, with no allocation. It is paid once
per overridden control per paint, not per node.

## Executable size

`examples/guides/input` and `examples/guides/textfield`, which use the text-entry
components:

```text
                      __text      __rodata   file bytes
input      before   1,336,032      23,872     3,297,842
input      after    1,337,424      23,872     3,297,858   +1,392 text, +16 file
textfield  before   1,330,704      23,840     3,281,298
textfield  after    1,332,080      23,840     3,281,314   +1,376 text, +16 file
```

About 1.4 KB of code (0.1%) in a program that uses a text field. The methods on
`Theme` are linked only by a program that calls them. File size is quantized by the
16 KiB Mach-O pages and the per-page signature hashes, so the file delta is smaller
than the code delta.

## Idle

Nothing here runs outside a paint: no timers, goroutines or polling were added, and
the overrides are plain values read while a control is painted.

## Not measured

Real windows on any platform, and the platform text editors' own caret and selection
(they are told the font size through `HitOptions.TextSize`, as before). The rendering
of the custom radius and font size was checked in a headless offscreen render on macOS
(`-tags vivahead`), not in a window.
