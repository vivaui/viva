# Optional LF Edit Context

`text.Editor.TrackLineEdits` opts into the insertion context needed to move
line-owned decorations through actual edits. It shares the existing byte-edit
journal and cursor; `TrackEdits` and `Edit` retain their existing behavior.

```go
editor := text.NewEditor("first\nsecond")
cursor := editor.TrackLineEdits()
editor.MoveTo(0, false)
editor.Insert("new\n")
edits, next, complete := editor.LineEditsSince(cursor, nil)
```

Each `LineEdit` embeds the original UTF-8 byte `Edit`, plus source LF facts:

- `Line`: zero-based logical line of Start in that primitive's source.
- `RemovedLines` / `InsertedLines`: exact LF counts in the removed/inserted text.

- `AtLineStart`: the insertion point was byte zero or followed LF in that
  primitive's source, before its deletion and insertion.
- `InsertedLF`: the accepted inserted text contained LF. It does not count CR
  or Unicode line separators; clients using other line-ending models need
  separate handling.

Removed and inserted user text is not copied into this metadata. Restrictions,
overwrite, IME updates and ordinary history report what was actually applied.
This is not a promise that Editor's native history grouping/caret behavior is
identical to another editor engine. Byte anchors and LF booleans alone are not
sufficient to project arbitrary line-marker history. For example, remove the
LF in `aa\nbb`, then insert LF at byte one. The old second-line marker merges
into line zero and stays there; shifting its collapsed byte anchor would
incorrectly put it on line one. Consumers should apply the original per-line
merge and insertion rules using the captured logical line and LF deltas.

## Batches And Transactions

ReplaceRanges keeps its existing reverse-target order. ReplaceRangesWith can
request forward target order; its contexts describe each resulting intermediate
prefix, not just the initial text. Undo reverses the primitive order and swaps
the removed/inserted LF counts and flags. Its source line stays the same because
the prefix before Start is not changed by that primitive. A transaction captures its draft's actual
steps, including nested batches/transactions and text absent from the final
buffer. The final one-buffer commit is not reported as a synthetic whole-buffer
mutation.

For example, starting with `ab`, insert `x` at zero, insert LF at one, then
delete the first byte. Replacing that temporary `x` with LF gives identical
initial/final text and byte counts, but the second insertion's line context
differs. Final buffers cannot recover that difference.

## Availability And Lifetime

Enable before attaching line decorations. Repeated enable calls do not reset
either journal. LineEditsSince reuses its destination and returns the same
cursor as EditsSince. A false result always has length zero, not a retained
suffix that a client could mistake for a complete edit sequence.

The false cases are disabled tracking, an expired/future cursor, old byte
entries recorded before line tracking, and old batch history without captured
context. Ordinary Undo can compute context from its actual replacement; a
flattened old batch/transaction cannot. Enable does not fabricate that data.

The ordinary mutation ring retains at most `EditJournalLimit` (1024) primitives.
The most-recent atomic batch/transaction/Undo/Redo may additionally be delivered
in full when the query cursor is exactly that action's start and its end is
still the current revision. The next real mutation or ClearHistory releases
that extra slot. Selection-only, refused and no-op work does not. An older
missing prefix is never reconstructed from the slot. Repeated queries copy
independent values. Byte EditsSince retains its original ring-only behavior.

This is one-action retention, not an unlimited mutation backlog, and retains
metadata only. Clients consume each operation before another mutation/history
reset and must still explicitly handle `complete == false`. History-side batch
context is released when history is cleared, branched or pruned; it is charged
to the existing history budget in four-byte rune units. Clearing history does
not enter or reset the ordinary mutation ring.

## Cost Boundary

Editor and history-unit layouts are unchanged. Byte-only/save-point clients
allocate no LF fact arrays or batch-context map. Opted-in clients keep source
line/count contexts alongside retained byte entries and batch primitives;
public LineEdit values are copied on query. The optional batch map shares the
existing primitive array identity and is bounded by retained history, not the
lifetime count of edits. It retains no extra text payload.

Ordinary line lookup uses a reusable rune-position/line memo, not a fresh full
prefix scan per keystroke. Batch contexts use one source scan and prior LF
deltas for forward order; batch replay invalidates that memo. The richer
private context is 32 bytes on amd64, not the earlier one-byte flag prototype.
Its default/opted costs need fresh measurements. Full atomic delivery beyond
the ring now has dedicated forward/Undo/Redo/lifetime tests and an original
CellBuffer integration scenario; consumers must not silently discard bookmarks
on any remaining incomplete sequence. Publication and application-wide
snapshot retirement remain separate gates.

Measure on the target machine:

```sh
GOWORK=off go test ./text -run '^$' \
  -bench 'BenchmarkLineJournal|BenchmarkSavePointMetadata|BenchmarkTypingWithUndoHistory' \
  -benchtime=300ms -count=3 -benchmem
```

Local measurements and the still-open default linked-size/timing gate are in
[the validation report](benchmarks/2026-11-10-line-fact-delivery.md). Semantic
tests passing does not make this unpublished extension ready for delivery.
