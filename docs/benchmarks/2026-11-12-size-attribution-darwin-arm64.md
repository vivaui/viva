# Size attribution: what hello carries and where the growth went

Measured 2026-11-12 on Apple M2 Pro, darwin/arm64, Go 1.26.5 with CGO, at
`1460fb0`. Method: unstripped `go build -trimpath` builds plus `go tool nm
-size` aggregated by package, section sizes from `size -m -l -x`, and
reachability edges from `go tool link -dumpdep`. Canonical figures come from
`benchmarks/size.sh`, which builds through `viva build`; its hello limit is
3,840,000 bytes and the build measured **3,969,394**, 129,394 over.

## What the binary is made of

Stripped manual build (`-trimpath -ldflags='-s -w -buildid='`): 3,886,306 B.
Its largest sections:

| Section | Bytes |
| --- | ---: |
| `__text` | 1,575,908 |
| `__gopclntab` | 1,351,698 |
| `__rodata` | 65,953 |
| ObjC sections (methlist/methname/methtype/classname/stubs) | ~56,000 |

The Go PC-line table is 86% of the code section's size; both grow with the
number of reachable functions, so the levers below are all reachability.

Unstripped, the viva packages contribute:

| Package | Bytes (text+rodata) |
| --- | ---: |
| gfx | 546,008 |
| internal/platform/darwin | 326,014 |
| ui | 306,254 |
| app | 267,086 |
| internal/uniprops | 266,012 |
| components | 262,840 |
| internal/textmetrics | 153,818 |
| text | 66,114 |

## Components rides in every app+ui page — and did before the rename

hello imports `app` and `ui` only, yet 262,840 B of `components` text and
rodata is linked. The edges are live calls, not the import graph:

- `ui.(*frame).measureLeafIn -> components.Button.SizeToFit`, `.geometry`
- `ui.(*frame).trim -> type:components.InputSnapshot <UsedInIface>` (and the
  group, textarea, segment snapshots)

The 2026-10-22 linkage report proved a blank import of components links none
of its symbols; that is still true. A page that declares widgets genuinely
calls their measure and geometry code, so this is reachability by design of
the ui widget layer, not a linker accident. It also predates the flex→ui
rename: at `9919383d^` (Nov 8) hello already carried 250,814 B of components
through the same plumbing. The rename and the Page model did not cause it;
the ~250 KB ride is the price of the layout layer knowing every widget kind.

Checked and rejected: dead-stripping the C side. Rebuilding with
`CGO_CFLAGS=-ffunction-sections -fdata-sections CGO_LDFLAGS=-Wl,-dead_strip`
produced 3,887,810 B, 1.5 KB larger — the native object code hello keeps is
already only what the ObjC classes and the exported entry points reach.

## Where the growth went

Canonical hello: 3,006,242 B (2026-10-22) → 3,969,394 B (2026-11-12), +963 KB
in three weeks. Per-commit samples of the unstripped build: 5,580,882 B at
`9919383d^` (Nov 8), 5,823,394 B at `9d1eba20` (Page model), 6,076,258 B at
`1460fb0`. Components grew only 12 KB across that window; the growth is
feature accretion in the always-reachable core — gfx (cell geometry, blur,
gradients, icon paths), ui (tables, flow, sideways scrolling, effects), app
(presentation pacing, activation, document opens) — each feature landing on
surfaces a plain page reaches.

## Options

1. Restore a layout/widget boundary so an app+ui page links only the widget
   kinds it declares. That is the direction the Oct-22 report's isolation
   work assumed, and the opposite of what the Page model chose; it needs an
   ADR, not a patch.
2. Accept the catalog as core and raise the limit consciously, recording the
   per-feature cost as each lands.
3. Gate capabilities behind opt-in packages the way `text/collation` is gated
   (unimported capability links nothing) for the ones a plain page cannot
   reach anyway.

Until one of those is decided, the size job runs in CI without blocking,
so the number is visible on every change instead of only in this directory;
the hard limit returns when the boundary does.
