# Optional Source-Line Facts: Cost And Scope

This draft extends the bounded primitive journal with opt-in source logical
line, LF deltas and most-recent atomic-action delivery. It retains no extra
user text and does not change Editor/history-unit layouts. The byte journal's
1024-entry retention is unchanged. See [the API contract](../editor-line-edits.md).

## Local Controls

Go 1.26.3, darwin/amd64, VirtualApple 2.50 GHz. Baseline text and its internal
dependencies were exported from published app fork `2dc475b17563`. Baseline
and head ran sequentially, five 300 ms samples, CPU 1, with no task-owned
concurrent builds/tests. These are local observations, not Linux CI or a
zero-default-cost guarantee.

| Workload | Baseline Median ns/op | Head Median ns/op | B/op Before/After | Allocations Before/After |
| --- | ---: | ---: | ---: | ---: |
| Mixed cursor movement | 823.0 | 824.5 | 0 / 0 | 0 / 0 |
| Typing with Undo | 869.2 | 878.2 | 432 / 432 | 17 / 17 |
| Default construction | 85.45 | 84.32 | 400 / 400 | 2 / 2 |
| Save-point construction | 133.0 | 131.3 | 472 / 472 | 4 / 4 |

The typing median is about +1.0%, with head samples 867.7-954.5 ns/op. Editor
is 368 B. Head line-tracked construction costs 528 B/4 allocations; private
source contexts are 32 B on amd64 and retained contexts count against the
existing history budget. Large-action query copies into a reused destination
are zero-allocation in the regression test.

The identical default probe (`text/testdata/line_context_probe/probe.go`), built
with `GOWORK=off go build -trimpath -ldflags '-s -w'`, is 1718896 baseline versus
1731504 head bytes: +12608 B, about 0.734%. It never enables the new API, and
both builds restore/print the original two lines after insertion/Undo. This
nonzero linked cost is explicit. A warmed 256 Undo + 256 Redo traversal has
byte/line medians 11.051/21.068 us, both zero allocations.

## Semantic Evidence

Consumer tests compile original Notepad++ v8.9.8.1 CellBuffer/LineMarkers and
UndoHistory at `dd40fe45598791e9aa38f3aeb200e10fb3d6338a`. They found a real
counterexample to byte-anchor projection: delete the LF in `aa\nbb`, then
insert LF at byte one. A marker from the second line must stay on line zero.
Actual source line/LF counts, not booleans or final-buffer inference, are needed.

The actual producer plus per-line mapper matches all 5460 source states,
including live mark changes, 64 fixed-seed transient scripts and a 2049-step
atomic operation with Undo/Redo. The fixture SHA-256 is
`a43840f87ee68b3a522c0efc0fc9daf65301e9b4239a965b39db6b45b9e8db16`.
The source adapter is normalized LF, not original CRLF/code-page notifications,
native caret grouping or Windows/Linux physical/pixel acceptance.

## Delivery Boundary

The latest atomic action can be read beyond the ordinary ring only from its
exact start cursor while its end remains current. The next mutation or explicit
history clear releases that metadata-only slot. An older missing prefix remains
incomplete. No callbacks run during mutation and no unlimited action backlog
is retained. Refusal, no-op, source-history pruning and late tracking have
separate regression coverage.

The measured tradeoff is retained for review: default object/allocation costs
stay unchanged while linking additional code and paying optional metadata cost.
Do not describe this as zero overhead. Fresh publication-branch CI/performance
and maintainer review are still required before upstream readiness.
