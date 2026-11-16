#!/bin/sh
# Runs the pure-Go benchmarks a pull request must not regress and writes one
# `go test -bench` stream that tools/perfhistory compare can read. The set
# covers scene diffing, renderer updates, input routing, text, wrapping and
# painter pages; GPU, window and platform benchmarks stay out so the job can run
# on a shared Linux runner.
#
#   benchmarks/ci.sh /tmp/base.txt          # in a base checkout or worktree
#   VIVA_ROOT=../base benchmarks/ci.sh /tmp/base.txt   # from the candidate tree
#   benchmarks/ci.sh /tmp/head.txt          # in the candidate tree
#   go run ./tools/perfhistory compare -threshold 0.15 /tmp/base.txt < /tmp/head.txt
set -eu

out=${1:?usage: ci.sh OUTFILE}
root=${VIVA_ROOT:-$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)}
: >"$out"

run() {
    go test -C "$root" "$1" -run '^$' -bench "$2" -benchmem -count=5 -benchtime=200ms >>"$out"
}

run ./internal/scene 'Benchmark(SceneNoChanges|SceneLocalizedUpdate)$'
run ./internal/render 'BenchmarkStore'
run ./internal/input 'Benchmark(HitTest10KMiss|FocusNext10K)$'
run ./text 'Benchmark(CursorMoveMixedText|ReplaceComposition|TypingWithUndoHistory)$'
run ./ui 'Benchmark(WrapRows|WrapLines|DrawIcon)$'
run ./components 'Benchmark(CheckboxPaint|MarkdownLayout)$'
run ./app 'Benchmark(Painter(LabelledPage|SteadyStateRepaint|IconPageHoverToggle|ModalToggle)|PointerCaretOwnership)$'
