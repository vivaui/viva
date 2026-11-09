package gfx

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// Text without spaces wraps where its script allows, not only where it overflows.
func TestWrapLinesThaiBreaksBetweenSyllablesWithoutSpaces(t *testing.T) {
	canvas := &Canvas{}
	text := "ประเทศไทยมีภาษาไทยเป็นภาษาราชการ"
	lines := canvas.WrapLines(nil, text, 16, 0, 120, 10)
	if len(lines) < 3 {
		t.Fatalf("Thai text wrapped into %d line(s): %q", len(lines), lines)
	}
	if strings.Join(lines, "") != text {
		t.Fatalf("lost characters: %q", lines)
	}
	for _, line := range lines {
		if w := canvas.TextWidthWeight(line, 16, 0); w > 120 {
			t.Errorf("line %q is %.0f wide", line, w)
		}
		if r, _ := utf8.DecodeRuneInString(line); r >= 0xE30 && r <= 0xE4E && !(r >= 0xE40 && r <= 0xE44) && r != 0xE2F && r < 0xE2F+1 {
			t.Errorf("line %q starts with a following vowel or mark", line)
		}
	}
	// Leading vowels go to the line of their consonant, never alone at the end of one.
	for _, line := range lines[:len(lines)-1] {
		if r, _ := utf8.DecodeLastRuneInString(line); r >= 0xE40 && r <= 0xE44 {
			t.Errorf("line %q ends with a leading vowel", line)
		}
	}
}

func TestWrapLinesKeepsKoreanWordsWholeUntilTheyDoNotFit(t *testing.T) {
	canvas := &Canvas{}
	size := float32(16) // a Hangul syllable is one em
	lines := canvas.WrapLines(nil, "안녕하세요 여러분 반갑습니다 오늘도 좋은 하루", size, 0, 6*size, 10)
	for _, line := range lines {
		for _, word := range strings.Fields(line) {
			if !strings.Contains("안녕하세요 여러분 반갑습니다 오늘도 좋은 하루", word) || utf8.RuneCountInString(word) < 2 && word != "" && word != "좋은" {
				t.Errorf("line %q has a word cut: %q", line, word)
			}
		}
	}
	if got := strings.Join(lines, " "); got != "안녕하세요 여러분 반갑습니다 오늘도 좋은 하루" {
		t.Errorf("words regrouped or broken: %q", lines)
	}
	// A word wider than the line is broken between syllables, not overflowed.
	lines = canvas.WrapLines(nil, "대한민국만세대한민국만세", size, 0, 5*size, 10)
	if len(lines) != 3 || strings.Join(lines, "") != "대한민국만세대한민국만세" {
		t.Errorf("long Korean word wrapped as %q", lines)
	}
}

func TestWrapLinesNeverSplitsAnIndicConjunctOrArabicWord(t *testing.T) {
	canvas := &Canvas{}
	for _, text := range []string{"क्षत्रिय क्षत्रिय क्षत्रिय", "مرحبا بالعالم مرحبا بالعالم مرحبا بالعالم", "שלום עולם שלום עולם"} {
		for width := float32(30); width < 120; width += 15 {
			lines := canvas.WrapLines(nil, text, 16, 0, width, 20)
			if strings.Join(lines, " ") != text && strings.Join(lines, "") != strings.ReplaceAll(text, " ", "") {
				t.Fatalf("%q at %v: %q", text, width, lines)
			}
			for _, line := range lines {
				if r, _ := utf8.DecodeRuneInString(line); r == 0x94D || r == 0x93F || r >= 0x64B && r <= 0x652 {
					t.Errorf("%q at %v: line %q starts with a mark or virama", text, width, line)
				}
			}
		}
	}
}
