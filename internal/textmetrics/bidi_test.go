package textmetrics

import (
	"math"
	"sort"
	"strings"
	"testing"
	"unicode/utf8"
)

type drawn struct {
	run      string
	x, width float32
}

// collect is the pieces of a line in the order bidiLine lays them out, which is also the
// layout Caret and Selection place carets and highlights by.
func collect(value string, size float32) []drawn {
	b := bidiPool.Get().(*bidiLine)
	defer b.release()
	b.resolve(value, weightIndex(0))
	b.layout(value)
	scale := size / unitsPerEm
	var out []drawn
	for _, p := range b.pieces {
		out = append(out, drawn{value[p.start:p.end], float32(p.x) * scale, float32(p.width) * scale})
	}
	return out
}

func find(t *testing.T, pieces []drawn, run string) drawn {
	t.Helper()
	for _, p := range pieces {
		if p.run == run {
			return p
		}
	}
	t.Fatalf("no piece %q in %v", run, pieces)
	return drawn{}
}

// A right-to-left line is laid out in the same width, but its first word is at the right.
func TestRunsPlaceHebrewWordsRightToLeft(t *testing.T) {
	const size = 20
	value := "שלום עולם טוב"
	pieces := collect(value, size)
	if len(pieces) != 3 {
		t.Fatalf("%d pieces: %v", len(pieces), pieces)
	}
	first, second, third := find(t, pieces, "שלום"), find(t, pieces, "עולם"), find(t, pieces, "טוב")
	if !(first.x > second.x && second.x > third.x) {
		t.Errorf("words are not displayed right to left: %v %v %v", first.x, second.x, third.x)
	}
	if third.x != 0 {
		t.Errorf("the last word starts at %v, want the left end", third.x)
	}
	total := Width(value, size, 0)
	if right := first.x + first.width; abs(right-total) > 0.01 {
		t.Errorf("the first word ends at %v, the line is %v wide", right, total)
	}
}

func TestRunsKeepEnglishInOrderInsideAHebrewLine(t *testing.T) {
	const size = 16
	pieces := collect("שלום Hello World עולם", size)
	hello, world := find(t, pieces, "Hello"), find(t, pieces, "World")
	if hello.x >= world.x {
		t.Errorf("an English phrase in a Hebrew line is reversed: Hello at %v, World at %v", hello.x, world.x)
	}
	if find(t, pieces, "שלום").x < world.x || find(t, pieces, "עולם").x > hello.x {
		t.Errorf("the Hebrew words are on the wrong sides of the English phrase: %v", pieces)
	}
}

func TestRunsKeepALeftToRightLineWithOneHebrewWordInPlace(t *testing.T) {
	const size = 16
	pieces := collect("say שלום now", size)
	say, word, now := find(t, pieces, "say"), find(t, pieces, "שלום"), find(t, pieces, "now")
	if !(say.x < word.x && word.x < now.x) {
		t.Errorf("a single Hebrew word moved: %v %v %v", say.x, word.x, now.x)
	}
	if abs(word.x-Width("say ", size, 0)) > 0.01 {
		t.Errorf("the Hebrew word is at %v, want the width of what precedes it", word.x)
	}
}

// The number inside Arabic text reads left to right, and Arabic is joined as one piece.
func TestRunsSplitANumberOutOfArabic(t *testing.T) {
	pieces := collect("الصفحة 123 من 456", 16)
	n123, n456 := find(t, pieces, "123"), find(t, pieces, "456")
	if n123.x <= n456.x {
		t.Errorf("123 is displayed left of 456 in a right-to-left line: %v %v", n123.x, n456.x)
	}
}

func TestBidiPiecesTileTheLineAndStayInsideIt(t *testing.T) {
	for _, value := range []string{"שלום עולם", "abc שלום עולם def", "مرحبا 123 abc", "(שלום) world!", "a‏b", "אבג‫def‬"} {
		const size = 14
		total := Width(value, size, 0)
		pieces := collect(value, size)
		sort.Slice(pieces, func(i, j int) bool { return pieces[i].x < pieces[j].x })
		end := float32(0)
		for _, p := range pieces {
			if p.x < end-0.01 {
				t.Errorf("%q: %q overlaps the piece before it (%v < %v)", value, p.run, p.x, end)
			}
			end = p.x + p.width
		}
		if end > total+0.01 {
			t.Errorf("%q: pieces end at %v, the line is %v", value, end, total)
		}
	}
}

func TestCaretOfRightToLeftText(t *testing.T) {
	const size = 20
	value := "אבג"
	total := Width(value, size, 0)
	if got := Caret(value, size, 0, 0); abs(got-total) > 0.01 {
		t.Errorf("caret at the start of Hebrew is at %v, want the right end %v", got, total)
	}
	if got := Caret(value, size, 0, len(value)); abs(got) > 0.01 {
		t.Errorf("caret at the end of Hebrew is at %v, want the left end", got)
	}
	// Moving right through logical text moves the caret left.
	prev := total + 1
	for offset := 0; offset <= len(value); offset += 2 {
		x := Caret(value, size, 0, offset)
		if x >= prev {
			t.Errorf("caret at %d is at %v, not left of %v", offset, x, prev)
		}
		prev = x
	}
	// Left to right text is a prefix width.
	if got, want := Caret("hello", size, 0, 3), Width("hel", size, 0); got != want {
		t.Errorf("latin caret %v, want %v", got, want)
	}
	// Mixed: after the English word the caret is at its end; the next Hebrew letter starts at the right.
	mixed := "ab אב"
	if got, want := Caret(mixed, size, 0, 2), Width("ab", size, 0); abs(got-want) > 0.01 {
		t.Errorf("caret after ab is %v, want %v", got, want)
	}
}

func TestSelectionIsSplitWhereDirectionChanges(t *testing.T) {
	const size = 20
	if spans := Selection(nil, "hello world", size, 0, 2, 7); len(spans) != 1 || abs(spans[0].X-Width("he", size, 0)) > 0.01 || abs(spans[0].Width-Width("llo w", size, 0)) > 0.01 {
		t.Errorf("latin selection %v", spans)
	}
	value := "abc אבגד def"
	spans := Selection(nil, value, size, 0, 2, strings.Index(value, "ד")) // "c " and the first Hebrew letters
	if len(spans) < 1 {
		t.Fatal("no spans")
	}
	for _, s := range spans {
		if s.Width <= 0 || s.X < 0 || s.X+s.Width > Width(value, size, 0)+0.01 {
			t.Errorf("span %v outside the line", s)
		}
	}
	// A selection of one Hebrew word is one span at the word's place.
	hebrew := "אבג דהו"
	wordSpan := Selection(nil, hebrew, size, 0, 0, len("אבג"))
	if len(wordSpan) != 1 || abs(wordSpan[0].X+wordSpan[0].Width-Width(hebrew, size, 0)) > 0.01 {
		t.Errorf("first Hebrew word selected at %v, want the right end of the line", wordSpan)
	}
}

func TestOffsetAtInvertsCaret(t *testing.T) {
	for _, value := range []string{"hello", "אבגדה", "abc אבג def", "مرحبا بك"} {
		for offset := 0; offset <= len(value); offset = nextRuneOffset(value, offset) {
			x := Caret(value, 16, 0, offset)
			if got := OffsetAt(value, 16, 0, x); Caret(value, 16, 0, got) != x {
				t.Errorf("%q: caret %d at %v, OffsetAt gives %d at %v", value, offset, x, got, Caret(value, 16, 0, got))
			}
			if offset == len(value) {
				break
			}
		}
	}
}

func TestHitAtUsesTheSideOfADirectionBoundary(t *testing.T) {
	const left, first, second, right = "ab ", "\u05d0\u05b7", "\u05d1", " cd"
	const value = left + first + second + right
	const size = 20
	lo, hi := len(left), len(left+first+second)
	xLeft := Width(left, size, 0)
	xRight := Width(left+first+second, size, 0)
	for _, check := range []struct {
		name string
		x    float32
		want int
		edge float32
	}{
		{"Latin at the left edge", xLeft - 0.5, lo, xLeft},
		{"Hebrew at the left edge", xLeft + 0.5, hi, xLeft},
		{"Hebrew at the right edge", xRight - 0.5, lo, xRight},
		{"Latin at the right edge", xRight + 0.5, hi, xRight},
	} {
		t.Run(check.name, func(t *testing.T) {
			if got := HitAt(value, size, 0, check.x); got.Offset != check.want || abs(got.X-check.edge) > 0.01 {
				t.Errorf("at x=%g: hit %+v, want byte %d at %g", check.x, got, check.want, check.edge)
			}
		})
	}
}

func TestHitAtKeepsTheLineEdges(t *testing.T) {
	const value = "ab אַב cd"
	const size = 20
	width := Width(value, size, 0)
	for _, check := range []struct {
		name         string
		edge, inside float32
	}{
		{"left", 0, 0.0001},
		{"right", width, width - 0.0001},
	} {
		t.Run(check.name, func(t *testing.T) {
			if got, want := HitAt(value, size, 0, check.edge), HitAt(value, size, 0, check.inside); got != want {
				t.Fatalf("click on the %s edge at %g gave %+v, just inside gave %+v", check.name, check.edge, got, want)
			}
		})
	}
}

func nextRuneOffset(value string, offset int) int {
	if offset >= len(value) {
		return offset + 1
	}
	_, w := utf8.DecodeRuneInString(value[offset:])
	return offset + w
}

func TestRightToLeftDirection(t *testing.T) {
	for value, want := range map[string]bool{"hello": false, "שלום": true, "123 שלום": true, "(مرحبا)": true, "": false, "a שלום": false, "‏abc": true} {
		if got := RightToLeft(value); got != want {
			t.Errorf("RightToLeft(%q) = %v", value, got)
		}
	}
}

func TestBidiCaretAndSelectionAreAllocationFreeWhenWarm(t *testing.T) {
	if raceEnabled {
		t.Skip("race instrumentation intentionally drops sync.Pool entries; allocation budgets run without -race")
	}
	value := "abc שלום עולם def مرحبا"
	var spans [8]Span
	Selection(spans[:0], value, 14, 0, 2, 12)
	if n := testing.AllocsPerRun(50, func() { Caret(value, 14, 0, 6) }); n != 0 {
		t.Errorf("Caret allocates %v times", n)
	}
	if n := testing.AllocsPerRun(50, func() { Selection(spans[:0], value, 14, 0, 2, 12) }); n != 0 {
		t.Errorf("Selection allocates %v times", n)
	}
}

func FuzzBidiLayout(f *testing.F) {
	for _, seed := range []string{"abc שלום עולם def", "مرحبا 123", "(שלום)", "‮abc‬", "a⁧b⁩c", "‏", " שלום "} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, value string) {
		if len(value) > 300 {
			t.Skip()
		}
		const size = 12
		total := Width(value, size, 0)
		end := float32(0)
		var pieces []drawn
		for _, p := range collect(value, size) {
			pieces = append(pieces, p)
			if math.IsNaN(float64(p.x)) || p.x < -0.01 || p.x+p.width > total+0.5 {
				t.Fatalf("piece %q at %v width %v outside the line of %v", p.run, p.x, p.width, total)
			}
		}
		sort.Slice(pieces, func(i, j int) bool { return pieces[i].x < pieces[j].x })
		for _, p := range pieces {
			if p.x < end-0.5 && p.width > 0 {
				t.Fatalf("pieces overlap: %q at %v < %v", p.run, p.x, end)
			}
			end = p.x + p.width
		}
		for offset := 0; offset <= len(value); offset += 1 + len(value)/7 {
			if c := Caret(value, size, 0, offset); c < -0.01 || c > total+0.01 {
				t.Fatalf("caret %v outside 0..%v", c, total)
			}
		}
		for _, s := range Selection(nil, value, size, 0, 0, len(value)) {
			if s.X < -0.01 || s.X+s.Width > total+0.5 {
				t.Fatalf("selection %v outside 0..%v", s, total)
			}
		}
	})
}

// A bracketed number in a Hebrew line is one piece, so that the engine mirrors its brackets
// as it does for any word; an English word run into Hebrew stays two pieces.
func TestWordsThatReadTheSameAloneAreDrawnWhole(t *testing.T) {
	pieces := collect("חיבור (192.168.1.1) נכשל", 14)
	if len(pieces) != 3 {
		t.Errorf("%d pieces: %v", len(pieces), pieces)
	}
	find(t, pieces, "(192.168.1.1)")
	// In a left to right line the same brackets belong to the text before them.
	pieces = collect("Connect to (192.168.1.1) failed: חיבור", 14)
	find(t, pieces, "(192.168.1.1)")
	pieces = collect("שלום abcשלום עולם", 14)
	find(t, pieces, "abc")
	for _, p := range pieces {
		if strings.Contains(p.run, "abc") && p.run != "abc" {
			t.Errorf("English run into Hebrew drawn as one piece: %q", p.run)
		}
	}
	// Brackets round Hebrew in a left to right line are left to right and the Hebrew inside
	// reads right to left; alone, the engine shows the same, so it is one piece.
	pieces = collect("abc (שלום) def", 14)
	find(t, pieces, "(שלום)")
}
