package gfx

import (
	"strings"
	"testing"
)

// cellWrapper is wrapper with a fixed-pitch cell one unit wide: a proportional width is a
// count of characters and a Mono one a count of cells, so where the two disagree is where
// a wide character sits.
func cellWrapper() *Canvas {
	canvas := wrapper()
	canvas.SetCellMeasurer(func(float32, MonoFace) (float32, float32) { return 1, 2 })
	return canvas
}

// A Mono line is as wide as its cells, and a wide character takes two of them: the commit
// panel's "分离的 HEAD" is eight characters but eleven cells, so at eight cells it no longer
// fits on one line, though a proportional text of the same characters does.
func TestWrapLinesStyleBreaksAMonoTextWhereItsCellsFillTheLine(t *testing.T) {
	canvas := cellWrapper()
	mono := TextStyle{Size: 1, Mono: true}
	if got := canvas.WrapLinesStyle(nil, "分离的 HEAD", mono, 8, 10, nil); strings.Join(got, "|") != "分离的|HEAD" {
		t.Fatalf("Mono lines = %q, want 分离的|HEAD", got)
	}
	if got := canvas.WrapLinesStyle(nil, "分离的 HEAD", TextStyle{Size: 1}, 8, 10, nil); strings.Join(got, "|") != "分离的 HEAD" {
		t.Fatalf("proportional lines = %q, want one line", got)
	}
	if got := canvas.WrapLinesCached(nil, "分离的 HEAD", 1, 0, 8, 10, nil); strings.Join(got, "|") != "分离的 HEAD" {
		t.Fatalf("WrapLinesCached lines = %q, want one line as before", got)
	}
}

// The last line of a Mono text is cut to the cells left beside the ellipsis. Cut by its
// characters instead, it would end two cells past the width.
func TestWrapLinesStyleCutsTheLastMonoLineOnItsCells(t *testing.T) {
	canvas := cellWrapper()
	mono := TextStyle{Size: 1, Mono: true}
	const text = "分离的 HEAD · 新仓库"
	got := canvas.WrapLinesStyle(nil, text, mono, 12, 1, nil)
	if len(got) != 1 || got[0] != "分离的 HEAD…" {
		t.Fatalf("Mono cut = %q, want 分离的 HEAD…", got)
	}
	proportional := canvas.WrapLinesStyle(nil, text, TextStyle{Size: 1}, 12, 1, nil)
	if len(proportional) != 1 || proportional[0] == got[0] {
		t.Fatalf("proportional cut = %q, want one line cut by characters", proportional)
	}
	if width := canvas.TextWidthStyle(proportional[0], mono); width <= 12 {
		t.Fatalf("the cut by characters %q is %v cells, so it would not show the difference", proportional[0], width)
	}
}

// Whatever the Mono text, width and line limit: no more lines than asked, every line within
// the width in cells (a single character wider than the line aside), and with room enough,
// every non-blank character of the text in order.
func TestWrapLinesStyleMonoProperties(t *testing.T) {
	canvas := cellWrapper()
	style := TextStyle{Size: 1, Mono: true}
	alphabet := []rune("ab c  \n主机，。x-/:é́界")
	random := newTestRandom(11)
	for round := 0; round < 3000; round++ {
		runes := make([]rune, random(40))
		for index := range runes {
			runes[index] = alphabet[random(len(alphabet))]
		}
		value := string(runes)
		width := float32(1 + random(12))
		limit := 1 + random(6)
		lines := canvas.WrapLinesStyle(nil, value, style, width, limit, nil)
		if len(lines) > limit {
			t.Fatalf("%q at %v: %d lines for a limit of %d", value, width, len(lines), limit)
		}
		for _, line := range lines {
			if cells := canvas.TextWidthStyle(line, style); cells > width && nextCluster(line, 0) < len(line) {
				t.Fatalf("%q at %v: line %q is %v cells", value, width, line, cells)
			}
		}
		squeeze := func(parts ...string) string {
			return strings.NewReplacer(" ", "", "\n", "").Replace(strings.Join(parts, ""))
		}
		if roomy := canvas.WrapLinesStyle(nil, value, style, width, 100, nil); squeeze(roomy...) != squeeze(value) {
			t.Fatalf("%q at %v: wrapping changed the text: %q", value, width, roomy)
		}
	}
}

// A Mono and a proportional cut of the same line at the same width differ, and neither the
// canvas's memo nor an EllipsisCache shared by the two ever serves one for the other.
func TestWrapLinesStyleNeverMixesUpMonoAndProportionalCuts(t *testing.T) {
	canvas := cellWrapper()
	mono := TextStyle{Size: 1, Mono: true}
	const text = "分离的 HEAD · 新仓库"
	monoCut := canvas.WrapLinesStyle(nil, text, mono, 12, 1, nil)[0]
	plainCut := canvas.FitLine(text, 1, 0, 12)
	if monoCut == plainCut {
		t.Fatalf("both cuts are %q", monoCut)
	}
	var cache EllipsisCache
	for round := 0; round < 3; round++ {
		if got := canvas.WrapLinesStyle(nil, text, mono, 12, 1, nil)[0]; got != monoCut {
			t.Fatalf("round %d: remembered Mono cut = %q, want %q", round, got, monoCut)
		}
		if got := canvas.FitLine(text, 1, 0, 12); got != plainCut {
			t.Fatalf("round %d: remembered cut = %q, want %q", round, got, plainCut)
		}
		if got := canvas.WrapLinesStyle(nil, text, mono, 12, 1, &cache)[0]; got != monoCut {
			t.Fatalf("round %d: cached Mono cut = %q, want %q", round, got, monoCut)
		}
		if got := canvas.WrapLinesCached(nil, text, 1, 0, 12, 1, &cache)[0]; got != plainCut {
			t.Fatalf("round %d: cached cut = %q, want %q", round, got, plainCut)
		}
	}
	// Faces of different cells cut differently too.
	canvas.SetCellMeasurer(func(_ float32, face MonoFace) (float32, float32) {
		if face == MonoClassic {
			return 2, 2
		}
		return 1, 2
	})
	classic := mono
	classic.MonoFace = MonoClassic
	if got := canvas.WrapLinesStyle(nil, text, classic, 12, 1, &cache)[0]; got == monoCut || canvas.TextWidthStyle(got, classic) > 12 {
		t.Fatalf("cut in the wider face = %q", got)
	}
}

// A Mono paragraph painted every frame wraps again at the same width: once warm, with an
// EllipsisCache or without, it builds nothing.
func TestWrapLinesStyleAllocatesNothingOnceWarm(t *testing.T) {
	canvas := cellWrapper()
	style := TextStyle{Size: 1, Mono: true}
	scratch := make([]string, 0, 4)
	const text = "fix(desktop): 编辑器滚到最后一行贴底即停止，并让缩略图与滚动条对齐"
	var cache EllipsisCache
	scratch = canvas.WrapLinesStyle(scratch[:0], text, style, 24, 2, &cache)
	if len(scratch) != 2 || !strings.HasSuffix(scratch[1], "…") {
		t.Fatalf("lines = %q, want two, the last cut", scratch)
	}
	if allocations := testing.AllocsPerRun(100, func() { scratch = canvas.WrapLinesStyle(scratch[:0], text, style, 24, 2, &cache) }); allocations != 0 {
		t.Fatalf("re-wrapping with a cache allocated %v times", allocations)
	}
	if allocations := testing.AllocsPerRun(100, func() { scratch = canvas.WrapLinesStyle(scratch[:0], text, style, 24, 2, nil) }); allocations != 0 {
		t.Fatalf("re-wrapping without a cache allocated %v times", allocations)
	}
}

func BenchmarkWrapLinesStyleMono(b *testing.B) {
	canvas := cellWrapper()
	style := TextStyle{Size: 1, Mono: true}
	scratch := make([]string, 0, 4)
	text := "Connection failed: 连接 10.255.255.1:22 超时: dial tcp 10.255.255.1:22: i/o timeout"
	b.ReportAllocs()
	for range b.N {
		scratch = canvas.WrapLinesStyle(scratch[:0], text, style, 30, 3, nil)
	}
}
