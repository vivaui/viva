package platform

import (
	"testing"

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
