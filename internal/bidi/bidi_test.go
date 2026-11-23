package bidi

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"
)

type conformCase struct {
	text    string
	runes   []rune
	dir     Direction
	para    uint8
	levels  []int // -1 for x
	visual  []int
	lineNum int
}

func parseCase(t *testing.T, line string, num int) conformCase {
	f := strings.Split(line, ";")
	if len(f) < 5 {
		t.Fatalf("line %d: bad format", num)
	}
	var c conformCase
	c.lineNum = num
	for _, h := range strings.Fields(f[0]) {
		v, err := strconv.ParseUint(h, 16, 32)
		if err != nil {
			t.Fatalf("line %d: %v", num, err)
		}
		c.runes = append(c.runes, rune(v))
	}
	c.text = string(c.runes)
	d, _ := strconv.Atoi(f[1])
	c.dir = [...]Direction{LTR, RTL, Auto}[d]
	p, _ := strconv.Atoi(f[2])
	c.para = uint8(p)
	for _, s := range strings.Fields(f[3]) {
		if s == "x" {
			c.levels = append(c.levels, -1)
		} else {
			v, _ := strconv.Atoi(s)
			c.levels = append(c.levels, v)
		}
	}
	for _, s := range strings.Fields(f[4]) {
		v, _ := strconv.Atoi(s)
		c.visual = append(c.visual, v)
	}
	return c
}

// check returns a description of the first disagreement, or "".
func (c conformCase) check(r *Resolver) string {
	p := r.Resolve(c.text, c.dir)
	if p.Level != c.para {
		return fmt.Sprintf("paragraph level %d, want %d", p.Level, c.para)
	}
	lv := r.LineLevels(p, 0, len(c.text), nil)
	if len(lv) != len(c.levels) {
		return fmt.Sprintf("%d levels, want %d", len(lv), len(c.levels))
	}
	for i, want := range c.levels {
		if want >= 0 && int(lv[i]) != want {
			return fmt.Sprintf("levels %v, want %v", lv, c.levels)
		}
	}
	byteIdx := make(map[int]int, len(c.runes)) // byte offset -> rune index
	off := 0
	for i, ru := range c.runes {
		byteIdx[off] = i
		off += utf8.RuneLen(ru)
	}
	var got []int
	for _, run := range r.LineRuns(p, 0, len(c.text), nil) {
		var idx []int
		for o := run.Start; o < run.End; {
			i := byteIdx[o]
			if c.levels[i] >= 0 {
				idx = append(idx, i)
			}
			_, w := utf8.DecodeRuneInString(c.text[o:])
			o += w
		}
		if run.Level&1 != 0 {
			for x, y := 0, len(idx)-1; x < y; x, y = x+1, y-1 {
				idx[x], idx[y] = idx[y], idx[x]
			}
		}
		got = append(got, idx...)
	}
	if fmt.Sprint(got) != fmt.Sprint(c.visual) {
		return fmt.Sprintf("visual %v, want %v", got, c.visual)
	}
	return ""
}

func runConformance(t *testing.T, path string) {
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(nil, 1<<20)
	var r Resolver
	n, bad := 0, 0
	for num := 1; sc.Scan(); num++ {
		line := strings.TrimSpace(sc.Text())
		if line == "" || line[0] == '#' {
			continue
		}
		c := parseCase(t, line, num)
		n++
		if msg := c.check(&r); msg != "" {
			bad++
			if bad <= 10 {
				t.Errorf("line %d (%s): %s", num, line, msg)
			}
		}
	}
	if n == 0 {
		t.Fatal("no cases")
	}
	t.Logf("%d cases, %d failures", n, bad)
	if bad > 0 {
		t.Fail()
	}
}

func TestConformanceSubset(t *testing.T) { runConformance(t, "testdata/BidiCharacterTest.subset.txt") }

// TestConformanceFull runs the whole BidiCharacterTest.txt named by BIDI_CHARACTER_TEST.
func TestConformanceFull(t *testing.T) {
	path := os.Getenv("BIDI_CHARACTER_TEST")
	if path == "" {
		t.Skip("BIDI_CHARACTER_TEST not set")
	}
	runConformance(t, path)
}

func visualString(r *Resolver, text string, base Direction) (string, uint8) {
	p := r.Resolve(text, base)
	var sb strings.Builder
	for _, run := range r.LineRuns(p, 0, len(text), nil) {
		s := []rune(text[run.Start:run.End])
		if run.Level&1 != 0 {
			for x, y := 0, len(s)-1; x < y; x, y = x+1, y-1 {
				s[x], s[y] = s[y], s[x]
			}
			for i, c := range s {
				s[i] = Mirrored(c)
			}
		}
		sb.WriteString(string(s))
	}
	return sb.String(), p.Level
}

func TestMixed(t *testing.T) {
	var r Resolver
	const shalom = "שלום"
	rev := func(s string) string {
		x := []rune(s)
		for i, j := 0, len(x)-1; i < j; i, j = i+1, j-1 {
			x[i], x[j] = x[j], x[i]
		}
		return string(x)
	}
	tests := []struct {
		name, text string
		base       Direction
		want       string
		para       uint8
	}{
		{"hebrew then digits then latin", shalom + " 123 abc", Auto, "abc 123 " + rev(shalom), 1},
		{"brackets", "(" + shalom + ")", Auto, "(" + rev(shalom) + ")", 1},
		{"brackets in ltr paragraph", "a (" + shalom + ") b", LTR, "a (" + rev(shalom) + ") b", 0},
		{"number after arabic", "اب 12", Auto, "12 " + rev("اب"), 1},
		{"latin forced rtl", "abc def", RTL, "abc def", 1},
	}
	for _, tc := range tests {
		got, para := visualString(&r, tc.text, tc.base)
		want := tc.want
		if got != want || para != tc.para {
			t.Errorf("%s: got %q level %d, want %q level %d", tc.name, got, para, want, tc.para)
		}
	}
	p := r.Resolve("ab "+shalom, LTR)
	if p.LevelAt(0) != 0 || p.LevelAt(3) != 1 || p.LevelAt(100) != 0 {
		t.Errorf("LevelAt: %d %d", p.LevelAt(0), p.LevelAt(3))
	}
	if Mirrored('(') != ')' || Mirrored('a') != 'a' {
		t.Error("Mirrored")
	}
}

func TestNeedsBidiAgrees(t *testing.T) {
	var r Resolver
	for _, s := range []string{
		"", "hello world", "café 中文 123", "1 + 2 = 3", "a⁧b", "א", "ا", "x١",
		"tab\there\n", "\xff\xfe bad", "‏", "‪", "﷽", "\U00010800", "ééé", "a⁩",
	} {
		need := NeedsBidi(s)
		p := r.Resolve(s, Auto)
		if !need {
			if p.Level != 0 {
				t.Errorf("%q: level %d", s, p.Level)
			}
			for i, l := range p.Levels() {
				if l != 0 {
					t.Errorf("%q: NeedsBidi false but level[%d]=%d", s, i, l)
				}
			}
		}
	}
	if NeedsBidi("hello") || !NeedsBidi("א") || !NeedsBidi("a⁦") {
		t.Error("NeedsBidi basics")
	}
}

func TestLineL1(t *testing.T) {
	var r Resolver
	text := "אב abc   "
	p := r.Resolve(text, RTL)
	lv := r.LineLevels(p, 0, len(text), nil)
	if lv[len(lv)-1] != 1 || lv[len(lv)-2] != 1 {
		t.Errorf("trailing white space not reset: %v", lv)
	}
	runs := r.LineRuns(p, 0, len(text), nil)
	end := 0
	for _, run := range runs {
		end = max(end, run.End)
	}
	if end != len(text) {
		t.Error("runs do not cover the line")
	}
}

const (
	englishParagraph = "The quick brown fox jumps over the lazy dog while the sun sets slowly behind the distant hills, " +
		"and a gentle breeze carries the scent of pine across the quiet valley where the river bends. " +
		"Nobody noticed the lantern flicker twice."
	mixedLine = "שלום world עולם (hello) 12:30 בדיקה test " +
		"שלום world עולם (hello) 12:30 בדיקה test ok"
)

func BenchmarkNeedsBidiEnglish(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if NeedsBidi(englishParagraph) {
			b.Fatal("unexpected")
		}
	}
}

func BenchmarkResolveLineMixed(b *testing.B) {
	var r Resolver
	runs := make([]Run, 0, 64)
	p := r.Resolve(mixedLine, Auto)
	runs = r.LineRuns(p, 0, len(mixedLine), runs)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p := r.Resolve(mixedLine, Auto)
		runs = r.LineRuns(p, 0, len(mixedLine), runs)
	}
}

func FuzzResolve(f *testing.F) {
	for _, s := range []string{"", "abc", "אב (abc) 123", "‫⁧x⁩‬", "\xff\xc3(", "a\nb א", "ا١⁦א"} {
		f.Add(s, uint8(0))
	}
	var r Resolver
	f.Fuzz(func(t *testing.T, s string, d uint8) {
		dir := Direction(d % 3)
		p := r.Resolve(s, dir)
		if p.Level > 1 {
			t.Fatalf("paragraph level %d", p.Level)
		}
		for _, l := range p.Levels() {
			if l > MaxDepth+1 {
				t.Fatalf("level %d", l)
			}
		}
		if !NeedsBidi(s) && dir != RTL {
			for _, l := range p.Levels() {
				if l != 0 {
					t.Fatalf("NeedsBidi false, level %d", l)
				}
			}
		}
		// a line from the middle of the text, snapped to rune boundaries
		n := p.Len()
		for _, cut := range [][2]int{{0, len(s)}, {len(s) / 3, len(s) - len(s)/4}} {
			runs := r.LineRuns(p, cut[0], cut[1], nil)
			a, b := p.runeSpan(cut[0], cut[1])
			wantStart, wantEnd := int(p.offs[a]), int(p.offs[b])
			if a == b {
				if len(runs) != 0 {
					t.Fatal("runs for empty line")
				}
				continue
			}
			// runs must tile [wantStart, wantEnd) exactly, each rune once, in some order
			covered := make(map[int]bool, len(runs))
			total := 0
			for _, run := range runs {
				if run.Start >= run.End || run.Level > MaxDepth+1 {
					t.Fatalf("bad run %+v", run)
				}
				if covered[run.Start] {
					t.Fatal("overlapping runs")
				}
				covered[run.Start] = true
				total += run.End - run.Start
			}
			if total != wantEnd-wantStart {
				t.Fatalf("runs cover %d bytes, want %d", total, wantEnd-wantStart)
			}
			for pos := wantStart; pos < wantEnd; {
				var hit *Run
				for i := range runs {
					if runs[i].Start == pos {
						hit = &runs[i]
					}
				}
				if hit == nil {
					t.Fatalf("gap at %d in %+v", pos, runs)
				}
				pos = hit.End
			}
			// the rune-level visual order is a permutation by construction; verify it
			seen := make([]bool, n)
			count := 0
			for _, run := range runs {
				for i := p.RuneIndex(run.Start); i < n && int(p.offs[i]) < run.End; i++ {
					if seen[i] {
						t.Fatal("rune visited twice")
					}
					seen[i] = true
					count++
				}
			}
			if count != b-a {
				t.Fatalf("visited %d runes, want %d", count, b-a)
			}
		}
	})
}
