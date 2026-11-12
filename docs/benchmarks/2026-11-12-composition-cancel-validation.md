# Composition cancellation: CI attribution

The explicit composition-cancel candidate consists of `996eeba`, `f361d7c` and
`75b4be9`. Its first PR run used base
`a6c0fdf17985663daf42ff07a1abdd70b23053e0` and merge candidate
`0b0ab31cfb7bc166bd6ad27468f6866777b4aae3`.

## Store benchmark

The first run reported `BenchmarkStoreAppendTail` at 1075 ns/op versus a base
median of 868 ns/op, crossing the existing 20% threshold. Neither the benchmark,
`internal/render`, `internal/scene`, `internal/input`, `go.mod` nor `go.sum`
differs between that base and merge candidate.

To check the executable rather than infer from file names, both complete trees
were exported without VCS metadata and built with Go 1.26.3 on darwin/amd64:

```sh
go test -c -trimpath -buildvcs=false -ldflags '-buildid=' \
  -o store.test ./internal/render
```

The two independently built test binaries have the same SHA-256:

```text
73c81c3f5145fd3ac44d0fc6d814121ccff2cd289265738f83603bc6b9cbb4d2
```

The same comparison cross-compiled for linux/amd64 with CGO disabled also
produces identical binaries, SHA-256
`55f60dfbffd1d998c59d6b623ab39c74005a4f44f42f02f844fa3889d80ee69a`.
Those Linux binaries were not executed on the macOS host.

Each copy then ran two rounds of five 200 ms samples of the unchanged
`BenchmarkStoreAppendTail`. Timings varied from 883.2 to 5618 ns/op across the
identical executables; both retained 2 allocs/op. This is evidence of measurement
variation, not a reason to weaken the gate or optimize an unrelated renderer
path to compensate for an IME patch. The hosted Linux run remains a separate
environment and must pass its own matched comparison.

No benchmark, workload, comparison threshold, renderer implementation or required
CI check has been changed. A fresh hosted run must establish the final result;
the initial failure remains recorded rather than relabeled as a pass.

## Release-size result

The first PR run reported canonical `hello darwin/arm64` at 4,003,154 bytes
against the existing 3,840,000-byte alarm limit. This is not the smaller
headless public-text-event probe used to describe the patch's incremental cost.

Upstream's `2026-11-12-size-attribution-darwin-arm64.md` already records hello
above that limit before this candidate and leaves the layout/widget boundary
decision open. Upstream CI explicitly makes `size` non-blocking with
`continue-on-error: true` and excludes it from the required `ci passed` job.
The composition change does not alter that policy, suppress the result, raise
the size limit or claim the existing boundary problem is solved.

## Remaining acceptance

The text, public/platform event and Objective-C harness tests, full framework
tests, cross-platform vet and focused race checks pass on the candidate.
Application probes compare exact original/committed text and saved bytes after
commit, Undo, Redo and a second Undo in five native editor contexts.

The default `Editor` layout grows by 64 bytes (1128 to 1192 on this target).
That API/memory tradeoff needs maintainer review. A physical final-candidate
WeType/native-window check is still pending; automated harness and Driver
results do not replace it. The PR remains a draft until those acceptance
conditions are addressed.
