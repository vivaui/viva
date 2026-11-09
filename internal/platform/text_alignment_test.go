package platform

import (
	"testing"
	"unicode/utf8"

	"github.com/vivaui/viva/gfx"
)

func TestTextAlignmentInsetKeepsFittedTextAndPreservesOtherInputs(t *testing.T) {
	for _, testcase := range []struct {
		name      string
		alignment gfx.Alignment
		line      float32
		terminal  bool
		placed    bool
		want      float32
	}{
		{"default left", gfx.AlignLeft, 30, false, false, 0},
		{"center", gfx.AlignCenter, 30, false, false, 45},
		{"right", gfx.AlignRight, 30, false, false, 90},
		{"overlong center", gfx.AlignCenter, 140, false, false, 0},
		{"overlong right", gfx.AlignRight, 140, false, false, 0},
		{"terminal", gfx.AlignRight, 30, true, false, 0},
		{"placed", gfx.AlignRight, 30, false, true, 0},
	} {
		t.Run(testcase.name, func(t *testing.T) {
			if got := TextAlignmentInset(testcase.alignment, 120, testcase.line, testcase.terminal, testcase.placed); got != testcase.want {
				t.Fatalf("alignment inset = %v, want %v", got, testcase.want)
			}
		})
	}
}

func TestTextCaretInsideFieldKeepsRightAlignedStrokeVisible(t *testing.T) {
	for _, testcase := range []struct {
		alignment gfx.Alignment
		x, want   float32
	}{
		{gfx.AlignLeft, 130, 130},
		{gfx.AlignCenter, 85, 85},
		{gfx.AlignRight, 130, 128},
		{gfx.AlignRight, 125, 125},
	} {
		if got := TextCaretInsideField(testcase.alignment, 10, 120, testcase.x, 2); got != testcase.want {
			t.Errorf("caret at %v with alignment %v = %v, want %v", testcase.x, testcase.alignment, got, testcase.want)
		}
	}
}

// A native editor only places glyphs at a physical edge, so a start- or
// end-relative request has to be resolved against the direction of the line the
// caret is on before it reaches the platform.
func TestResolveTextAlignmentMatchesTheCanvasLabelRule(t *testing.T) {
	const leftToRight, rightToLeft = "hello", "שלום"
	for _, testcase := range []struct {
		name      string
		alignment gfx.Alignment
		value     string
		want      gfx.Alignment
	}{
		{"start reads left in left-to-right text", gfx.AlignStart, leftToRight, gfx.AlignLeft},
		{"start reads right in right-to-left text", gfx.AlignStart, rightToLeft, gfx.AlignRight},
		{"end reads right in left-to-right text", gfx.AlignEnd, leftToRight, gfx.AlignRight},
		{"end reads left in right-to-left text", gfx.AlignEnd, rightToLeft, gfx.AlignLeft},
		{"center stays center", gfx.AlignCenter, leftToRight, gfx.AlignCenter},
		{"right stays right", gfx.AlignRight, leftToRight, gfx.AlignRight},
		{"left stays left", gfx.AlignLeft, rightToLeft, gfx.AlignLeft},
		{"an empty line reads left", gfx.AlignStart, "", gfx.AlignLeft},
	} {
		t.Run(testcase.name, func(t *testing.T) {
			caret := utf8.RuneCountInString(testcase.value)
			if got := ResolveTextAlignment(testcase.alignment, testcase.value, caret); got != testcase.want {
				t.Fatalf("resolved alignment = %v, want %v", got, testcase.want)
			}
		})
	}
}

// The two helpers are meant to be used together, so a resolved alignment must
// actually move the line: this is the pair the native backends call.
func TestResolvedAlignmentAndInsetPlaceARightToLeftLineAtTheRightEdge(t *testing.T) {
	const value = "שלום"
	resolved := ResolveTextAlignment(gfx.AlignEnd, value, utf8.RuneCountInString(value))
	if resolved != gfx.AlignLeft {
		t.Fatalf("AlignEnd over right-to-left text resolved to %v, want left", resolved)
	}
	if inset := TextAlignmentInset(resolved, 120, 30, false, false); inset != 0 {
		t.Fatalf("a left-aligned line has inset %v, want 0", inset)
	}
	start := ResolveTextAlignment(gfx.AlignStart, value, utf8.RuneCountInString(value))
	if inset := TextAlignmentInset(start, 120, 30, false, false); inset != 90 {
		t.Fatalf("a start-aligned right-to-left line has inset %v, want 90", inset)
	}
}
