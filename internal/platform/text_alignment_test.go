package platform

import (
	"testing"
	"unicode/utf8"

	"github.com/vivaui/viva/ui"
)

func TestTextAlignmentInsetKeepsFittedTextAndPreservesOtherInputs(t *testing.T) {
	for _, testcase := range []struct {
		name      string
		alignment ui.Alignment
		line      float32
		terminal  bool
		placed    bool
		want      float32
	}{
		{"default left", ui.AlignLeft, 30, false, false, 0},
		{"center", ui.AlignCenter, 30, false, false, 45},
		{"right", ui.AlignRight, 30, false, false, 90},
		{"overlong center", ui.AlignCenter, 140, false, false, 0},
		{"overlong right", ui.AlignRight, 140, false, false, 0},
		{"terminal", ui.AlignRight, 30, true, false, 0},
		{"placed", ui.AlignRight, 30, false, true, 0},
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
		alignment ui.Alignment
		x, want   float32
	}{
		{ui.AlignLeft, 130, 130},
		{ui.AlignCenter, 85, 85},
		{ui.AlignRight, 130, 128},
		{ui.AlignRight, 125, 125},
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
		alignment ui.Alignment
		value     string
		want      ui.Alignment
	}{
		{"start reads left in left-to-right text", ui.AlignStart, leftToRight, ui.AlignLeft},
		{"start reads right in right-to-left text", ui.AlignStart, rightToLeft, ui.AlignRight},
		{"end reads right in left-to-right text", ui.AlignEnd, leftToRight, ui.AlignRight},
		{"end reads left in right-to-left text", ui.AlignEnd, rightToLeft, ui.AlignLeft},
		{"center stays center", ui.AlignCenter, leftToRight, ui.AlignCenter},
		{"right stays right", ui.AlignRight, leftToRight, ui.AlignRight},
		{"left stays left", ui.AlignLeft, rightToLeft, ui.AlignLeft},
		{"an empty line reads left", ui.AlignStart, "", ui.AlignLeft},
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
	resolved := ResolveTextAlignment(ui.AlignEnd, value, utf8.RuneCountInString(value))
	if resolved != ui.AlignLeft {
		t.Fatalf("AlignEnd over right-to-left text resolved to %v, want left", resolved)
	}
	if inset := TextAlignmentInset(resolved, 120, 30, false, false); inset != 0 {
		t.Fatalf("a left-aligned line has inset %v, want 0", inset)
	}
	start := ResolveTextAlignment(ui.AlignStart, value, utf8.RuneCountInString(value))
	if inset := TextAlignmentInset(start, 120, 30, false, false); inset != 90 {
		t.Fatalf("a start-aligned right-to-left line has inset %v, want 90", inset)
	}
}
