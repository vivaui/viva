package components

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/vivaui/viva/markdown"
)

const anchorSource = "# Title\n\nfirst paragraph line\nsecond line\n\n```go\none\ntwo\nthree\nfour\n```\n\n中文段落，含有汉字\n"

func TestMarkdownAnchorsCoverBlocksInOrder(t *testing.T) {
	view, doc := layoutMarkdown(t, anchorSource, 300, MarkdownStyle{Size: 12})
	anchors := view.Anchors()
	if len(anchors) != len(doc.Blocks) {
		t.Fatalf("%d anchors for %d blocks", len(anchors), len(doc.Blocks))
	}
	for i, a := range anchors {
		if a.Start != doc.Blocks[i].Start || a.End != doc.Blocks[i].End {
			t.Errorf("anchor %d range %d..%d, block %d..%d", i, a.Start, a.End, doc.Blocks[i].Start, doc.Blocks[i].End)
		}
		if a.Bottom <= a.Top {
			t.Errorf("anchor %d has no height: %v..%v", i, a.Top, a.Bottom)
		}
		if i > 0 && (a.Top < anchors[i-1].Bottom || a.Start < anchors[i-1].End) {
			t.Errorf("anchor %d overlaps the one before", i)
		}
	}
}

// YAtSource must not go backwards as the offset grows, at every character
// boundary, and must stay inside the document.
func TestMarkdownYAtSourceIsMonotonic(t *testing.T) {
	view, _ := layoutMarkdown(t, anchorSource, 300, MarkdownStyle{Size: 12})
	previous := float32(-1)
	for offset := 0; offset <= len(anchorSource); offset++ {
		if offset < len(anchorSource) && !utf8.RuneStart(anchorSource[offset]) {
			continue
		}
		y, ok := view.YAtSource(anchorSource, offset)
		if !ok || y < previous || y < 0 || y > view.Height() {
			t.Fatalf("offset %d: y=%v ok=%v after %v (height %v)", offset, y, ok, previous, view.Height())
		}
		previous = y
	}
	if y, _ := view.YAtSource(anchorSource, -5); y != 0 {
		t.Errorf("before the text: %v", y)
	}
	if y, _ := view.YAtSource(anchorSource, len(anchorSource)+9); y != view.Anchors()[len(view.Anchors())-1].Bottom {
		t.Errorf("after the text: %v", y)
	}
}

// Code lines are real rows, so a line inside a code block lands on its own row.
func TestMarkdownYAtSourceFollowsCodeLines(t *testing.T) {
	view, _ := layoutMarkdown(t, anchorSource, 300, MarkdownStyle{Size: 12})
	at := func(needle string) float32 {
		y, ok := view.YAtSource(anchorSource, strings.Index(anchorSource, needle))
		if !ok {
			t.Fatal("no position")
		}
		return y
	}
	one, two, three := at("one"), at("two"), at("three")
	if !(one < two && two < three) {
		t.Fatalf("lines of one block are not in order: %v %v %v", one, two, three)
	}
	if d1, d2 := two-one, three-two; !near(d1, d2) {
		t.Errorf("code rows are unevenly spaced: %v then %v", d1, d2)
	}
}

// SourceAtY and YAtSource agree to within the line they land on, in both
// directions, and the offset it returns is always a character boundary.
func TestMarkdownSourceAtYRoundTrips(t *testing.T) {
	view, _ := layoutMarkdown(t, anchorSource, 300, MarkdownStyle{Size: 12})
	for y := float32(0); y <= view.Height(); y += 2 {
		offset, ok := view.SourceAtY(anchorSource, y)
		if !ok || offset < 0 || offset > len(anchorSource) {
			t.Fatalf("y=%v: offset %d ok=%v", y, offset, ok)
		}
		if offset < len(anchorSource) && !utf8.RuneStart(anchorSource[offset]) {
			t.Fatalf("y=%v: offset %d is inside a character", y, offset)
		}
		back, _ := view.YAtSource(anchorSource, offset)
		if back > y+0.01 && y > 0 {
			// Rounding down to a character may only move the position up.
			t.Errorf("y=%v maps to %d which maps back to %v", y, offset, back)
		}
	}
	if offset, _ := view.SourceAtY(anchorSource, -10); offset != 0 {
		t.Errorf("above the document: %d", offset)
	}
	if offset, _ := view.SourceAtY(anchorSource, view.Height()+100); offset != len(anchorSource) {
		t.Errorf("below the document: %d", offset)
	}
}

func TestMarkdownAnchorsOfAnEmptyDocument(t *testing.T) {
	view, _ := layoutMarkdown(t, "", 300, MarkdownStyle{Size: 12})
	if _, ok := view.YAtSource("", 0); ok {
		t.Error("an empty document has a position")
	}
	if _, ok := view.SourceAtY("", 0); ok {
		t.Error("an empty document has an offset")
	}
}

func BenchmarkMarkdownYAtSource(b *testing.B) {
	src := strings.Repeat("paragraph of text that goes on\nand on\n\n```\ncode\nmore\n```\n\n", 500)
	var view MarkdownView
	canvas := markdownCanvas()
	doc := parseForBench(src)
	view.Layout(canvas, MarkdownStyle{Size: 12}, doc, 300)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		y, _ := view.YAtSource(src, len(src)/2)
		view.SourceAtY(src, y)
	}
}

func parseForBench(src string) *markdown.Document {
	var doc markdown.Document
	doc.Parse(src)
	return &doc
}
