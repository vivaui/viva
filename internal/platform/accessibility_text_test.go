package platform

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

func rangeOf(now, min, max float64) gfx.ValueRange {
	return gfx.ValueRange{Now: now, Min: min, Max: max, Step: 1, Set: true}
}

func runes(text string) []rune { return []rune(text) }

func TestTextUnitsForReading(t *testing.T) {
	text := runes("Hello, wide  world\nsecond line")
	for _, test := range []struct {
		name       string
		offset     int
		unit       AccessibilityTextUnit
		start, end int
	}{
		{"word at its start", 0, TextUnitWord, 0, 5},
		{"punctuation is a word", 5, TextUnitWord, 5, 7},
		{"word keeps trailing spaces", 9, TextUnitWord, 7, 13},
		{"inside the trailing spaces", 11, TextUnitWord, 7, 13},
		{"the break stands alone", 18, TextUnitWord, 18, 19},
		{"first line includes its break", 3, TextUnitLine, 0, 19},
		{"second line", 25, TextUnitLine, 19, 30},
		{"character", 4, TextUnitCharacter, 4, 5},
		{"document", 12, TextUnitDocument, 0, 30},
	} {
		if start := TextUnitStart(text, test.offset, test.unit); start != test.start {
			t.Errorf("%s: start = %d, want %d", test.name, start, test.start)
		}
		if end := TextUnitEnd(text, test.offset, test.unit); end != test.end {
			t.Errorf("%s: end = %d, want %d", test.name, end, test.end)
		}
	}
}

func TestTextUnitsDoNotSplitACharacter(t *testing.T) {
	// e + combining acute, a family emoji joined by ZWJ, a flag, and Han text.
	text := runes("e\u0301\U0001F468\u200d\U0001F469\U0001F1E8\U0001F1F3\u4e2d\u6587")
	cases := []struct{ offset, start, end int }{
		{0, 0, 2}, {1, 0, 2}, {2, 2, 5}, {3, 2, 5}, {4, 2, 5}, {5, 5, 7}, {6, 5, 7}, {7, 7, 8}, {8, 8, 9},
	}
	for _, test := range cases {
		if start := TextUnitStart(text, test.offset, TextUnitCharacter); start != test.start {
			t.Errorf("start at %d = %d, want %d", test.offset, start, test.start)
		}
		if end := TextUnitEnd(text, test.offset, TextUnitCharacter); end != test.end {
			t.Errorf("end at %d = %d, want %d", test.offset, end, test.end)
		}
	}
	if start, end := TextUnitStart(text, 7, TextUnitWord), TextUnitEnd(text, 7, TextUnitWord); start != 7 || end != 8 {
		t.Errorf("an ideograph is its own word, got %d..%d", start, end)
	}
}

func TestTextMoveSpanStopsAtTheEnds(t *testing.T) {
	text := runes("one two three")
	start, end, moved := TextMoveSpan(text, 0, 0, TextUnitWord, 1)
	if start != 4 || end != 8 || moved != 1 {
		t.Fatalf("forward one word: %d..%d moved %d", start, end, moved)
	}
	start, end, moved = TextMoveSpan(text, start, end, TextUnitWord, 5)
	if start != 8 || end != 13 || moved != 1 {
		t.Fatalf("forward past the end: %d..%d moved %d", start, end, moved)
	}
	start, end, moved = TextMoveSpan(text, start, end, TextUnitWord, -9)
	if start != 0 || end != 4 || moved != -2 {
		t.Fatalf("backward past the start: %d..%d moved %d", start, end, moved)
	}
	if _, _, moved = TextMoveSpan(nil, 0, 0, TextUnitCharacter, 3); moved != 0 {
		t.Fatalf("empty text moved %d", moved)
	}
}

func TestTextMoveEndpointNeverInvertsASpan(t *testing.T) {
	text := runes("one two three")
	start, end, moved := TextMoveEndpoint(text, 4, 7, true, TextUnitWord, -3)
	if start != 0 || end != 0 || moved != -2 {
		t.Fatalf("end moved before start: %d..%d moved %d", start, end, moved)
	}
	start, end, moved = TextMoveEndpoint(text, 0, 3, false, TextUnitCharacter, 5)
	if start != 5 || end != 5 || moved != 5 {
		t.Fatalf("start moved past end: %d..%d moved %d", start, end, moved)
	}
}

func TestTextExpandKeepsAWholeSpan(t *testing.T) {
	text := runes("one two three")
	if start, end := TextExpand(text, 5, 5, TextUnitWord); start != 4 || end != 8 {
		t.Errorf("caret expands to its word, got %d..%d", start, end)
	}
	if start, end := TextExpand(text, 4, 8, TextUnitWord); start != 4 || end != 8 {
		t.Errorf("a whole word stays, got %d..%d", start, end)
	}
	if start, end := TextExpand(text, 5, 10, TextUnitWord); start != 4 || end != 13 {
		t.Errorf("a span across words grows to both, got %d..%d", start, end)
	}
}

func TestTextFind(t *testing.T) {
	text := runes("Alpha beta alpha")
	if start, end, ok := TextFind(text, "alpha", 0, false, false); !ok || start != 11 || end != 16 {
		t.Errorf("case-sensitive forward: %d..%d %v", start, end, ok)
	}
	if start, _, ok := TextFind(text, "alpha", 0, false, true); !ok || start != 0 {
		t.Errorf("ignoring case finds the first: %d %v", start, ok)
	}
	if start, _, ok := TextFind(text, "alpha", 16, true, true); !ok || start != 11 {
		t.Errorf("backward from the end: %d %v", start, ok)
	}
	if _, _, ok := TextFind(text, "gamma", 0, false, false); ok {
		t.Error("found text that is not there")
	}
}

func TestOffsetsConvertBetweenCodePointsAndUTF16(t *testing.T) {
	text := "a\U0001F468b中"
	for _, test := range []struct{ runes, units int }{{0, 0}, {1, 1}, {2, 3}, {3, 4}, {4, 5}} {
		if got := UTF16Offset(text, test.runes); got != test.units {
			t.Errorf("UTF16Offset(%d) = %d, want %d", test.runes, got, test.units)
		}
		if got := RuneOffset(text, test.units); got != test.runes {
			t.Errorf("RuneOffset(%d) = %d, want %d", test.units, got, test.runes)
		}
	}
	if got := UTF16Offset(text, -1); got != 5 {
		t.Errorf("length = %d, want 5", got)
	}
	if got := RuneOffset(text, 2); got != 1 {
		t.Errorf("inside a surrogate pair = %d, want 1", got)
	}
}
