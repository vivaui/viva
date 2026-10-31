> Historical build-tag experiment, superseded by runtime configuration.
> The sizes below do not describe the current default build.

# Optional window material footprint

Windows build 26300, amd64, Go 1.27.3. Compared with original `8e45fdc` and
material implementation `d5fd167`; no benchmark/test files were linked into the
measured application. Command:

```powershell
go build -trimpath -ldflags='-s -w' -o hello.exe ./examples/guides/hello
```

| Build | Hello bytes | windowState | renderer | platform.Config |
| --- | ---: | ---: | ---: | ---: |
| Original `8e45fdc` | 3,764,224 | 1,736 B | 368 B | 400 B |
| Material implementation `d5fd167` | 3,789,312 | 1,800 B | 400 B | 416 B |
| After outline optimization, before isolation | 3,792,384 | 1,800 B | 400 B | 416 B |
| Isolated, default build | 3,765,760 | 1,736 B | 368 B | 400 B |

The default window, renderer and platform configuration sizes are restored.
State sizes are measured using `unsafe.Sizeof`, not total process RSS; the
window embeds its renderer/config, so these columns must not be summed.

An unstripped default Hello was also inspected with `go tool nm`: no native
material application, DirectComposition creation, policy-reading or
`RoundedOutline` symbols were linked. Material DLL entry points and power
notifications are compiled only with `vivamaterials`. Source-owned material-aware
controls use the same tag, so ordinary buttons do not retain outline geometry.
`RoundedOutline` itself remains available to applications that explicitly call it.

This is not a claim of byte-for-byte zero cost. The default executable is still
1,536 bytes (0.041%) above the original. PE `.text` virtual size changes by 128
bytes but fits the same raw section size; `.rdata` increases by 984 virtual
bytes / 1,024 raw bytes. There are also section/alignment changes. Empty embedded
configuration/state types and integration code are not free of metadata cost.
This small residual should be disclosed to upstream rather than concealed.

Enabled windows pay for the implementation: `windowState` is 1,808 B,
`renderer` 400 B and `platform.Config` 416 B. These are build-time choices, not
per-paint allocations. The API now groups the callbacks under
`app.Config.WindowMaterials`, allowing the ordinary configuration field to be
empty. Use `-tags vivamaterials` to compile either the example or an application
that initializes `app.WindowMaterials.WindowBackdrop`.

The two baseline test failures have separate test-only corrections: a synthetic
Windows Go archive needs `go.exe`; explicit YaHei may equal the system fallback,
while two distinct explicit fallback families must still differ. These changes
do not alter production rendering or suppress failing assertions about the
fallback list itself.

After isolation, `go test ./... -count=1` and
`go test -tags vivamaterials ./... -count=1` both passed without exclusions on
the Windows host. Default and tagged vet passed; tagged Windows amd64/arm64,
Linux amd64 and Web checks also passed. Native Win10 and macOS execution remain
external verification requirements, not successes inferred from cross-compiling.

The final tagged input-paint benchmark was repeated five times with the same
workload: translucent samples 799.5, 796.1, 793.3, 778.2, 799.4 ns/op, median
796.1 ns/op, 0 B/op and 0 allocs/op. This later run is slower than the earlier
647.9 ns/op median but remains below 1 microsecond; do not substitute it into
the earlier paired comparison as if the host conditions were identical.
