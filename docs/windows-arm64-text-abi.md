# Direct2D text origin on Windows ARM64

`ID2D1RenderTarget::DrawTextLayout` takes `D2D1_POINT_2F` by value. On x64 its
eight bytes use an integer argument slot. On ARM64 it is a homogeneous
floating-point aggregate: its two floats use s0/s1, and the layout pointer
follows `this` in x1. Packing the point into a `uintptr` puts its bits where
native ARM64 code expects a layout pointer and faults on the first text draw.

The ARM64 call is typed and bound once per Direct2D target; x64 keeps its packed
integer call. The cached function adds 8 bytes to an ARM64 target, and the empty
x64 call state adds no target storage.

Run the native regression with:

```
go test ./internal/platform/windows -run TestDirectTextLayoutOriginByValue -count=1
```

It draws the same glyphs at two origins and checks the translated native ink.
Both native Windows amd64 and ARM64 passed. The ARM64 real offscreen application
suite also rendered both themes at 100%, 150% and 200% after the fix; before it,
the first label crashed through `comCall`/`syscall.SyscallN` with 0xc0000005.

On a native Windows ARM64 CI host, `BenchmarkRasterizeOnly` (3 × 100ms) reports
295703–349122 ns/op, 358–380 B/op, 10 allocs/op. There is no valid timing for the
previous crashing ARM64 path. Local x64 samples remain at 5 allocs/op; short
loaded samples were 159327–159972 ns/op before and 163917–166752 ns/op after.
Stripped x64 hello increased from 4,280,320 to 4,280,832 bytes (+512 bytes).

The ARM64 call goes through purego's typed struct binding, which costs about
5 more allocations per text draw than the x64 integer-slot call. That cost is
accepted for correctness; an assembly or register-passing `SyscallN` path is a
possible follow-up for this hot path.

References: [Windows ARM64 ABI](https://learn.microsoft.com/en-us/cpp/build/arm64-windows-abi-conventions)
and [DrawTextLayout](https://learn.microsoft.com/en-us/windows/win32/api/d2d1/nf-d2d1-id2d1rendertarget-drawtextlayout).
