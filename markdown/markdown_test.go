package markdown

import (
	"fmt"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

// dump writes a document in a compact form: one line per block, its spans
// marked up as {text}, with the style and link after a colon.
func dump(d *Document) string {
	var out strings.Builder
	spans := func(list []Span) {
		for _, span := range list {
			if span.Text == Newline {
				out.WriteString("\\n")
				continue
			}
			out.WriteString("{" + span.Text)
			if span.Style&Bold != 0 {
				out.WriteString(":b")
			}
			if span.Style&Italic != 0 {
				out.WriteString(":i")
			}
			if span.Style&Strike != 0 {
				out.WriteString(":s")
			}
			if span.Style&Mono != 0 {
				out.WriteString(":m")
			}
			if span.Link != "" {
				out.WriteString(" -> " + span.Link)
			}
			out.WriteString("}")
		}
	}
	for index := range d.Blocks {
		b := &d.Blocks[index]
		if b.Quote > 0 {
			fmt.Fprintf(&out, "%s", strings.Repeat(">", b.Quote))
		}
		switch b.Kind {
		case Paragraph:
			fmt.Fprintf(&out, "P%d ", b.Level)
			spans(d.Spans(b))
		case Heading:
			fmt.Fprintf(&out, "H%d ", b.Level)
			spans(d.Spans(b))
		case Item:
			if b.Number > 0 {
				fmt.Fprintf(&out, "N%d.%d ", b.Level, b.Number)
			} else {
				fmt.Fprintf(&out, "I%d ", b.Level)
			}
			spans(d.Spans(b))
		case Code:
			fmt.Fprintf(&out, "CODE[%s] %q", b.Info, b.Text)
		case Rule:
			out.WriteString("RULE")
		case Table:
			cells := d.Cells(b)
			fmt.Fprintf(&out, "TABLE %dx%d", b.Columns, len(cells)/b.Columns)
			for i, cell := range cells {
				if i%b.Columns == 0 {
					out.WriteString(" |")
				}
				out.WriteString(" ")
				spans(d.CellSpans(cell))
				out.WriteString([]string{"", "^", ">"}[[]int{0, 1, 2}[cell.Align]])
			}
		}
		out.WriteString("\n")
	}
	return out.String()
}

func parse(src string) string {
	var d Document
	d.Parse(src)
	return dump(&d)
}

func TestAChatAnswerKeepsItsStructure(t *testing.T) {
	src := "## Plan\n" +
		"Run **`ls -la`** first, then check *the logs* (see [docs](https://example.com/a_(b))).\n" +
		"\n" +
		"1. Install it\n" +
		"   - with `brew`\n" +
		"   - or from https://example.com/dl.\n" +
		"2. Restart\n" +
		"\n" +
		"> Note: ~~old~~ is gone\n" +
		"\n" +
		"```bash\nsudo systemctl restart sshd\n```\n" +
		"---\n"
	want := "H2 {Plan}\n" +
		"P0 {Run }{ls -la:b:m}{ first, then check }{the logs:i}{ (see }{docs -> https://example.com/a_(b)}{).}\n" +
		"N0.1 {Install it}\n" +
		"I1 {with }{brew:m}\n" +
		"I1 {or from }{https://example.com/dl -> https://example.com/dl}{.}\n" +
		"N0.2 {Restart}\n" +
		">P0 {Note: }{old:s}{ is gone}\n" +
		"CODE[bash] \"sudo systemctl restart sshd\"\n" +
		"RULE\n"
	if got := parse(src); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestTextThatIsNotClosedStaysAsTyped(t *testing.T) {
	// A message is parsed while it is still arriving: whatever has not been
	// closed yet must read as the text it is, and a fence stays open.
	for src, want := range map[string]string{
		"a **half bold":      "P0 {a **half bold}\n",
		"1 * 2 * 3":          "P0 {1 * 2 * 3}\n",
		"call snake_case_x":  "P0 {call snake_case_x}\n",
		"`open":              "P0 {`open}\n",
		"```go\nfmt.Println": "CODE[go] \"fmt.Println\"\n",
		"```go\n":            "CODE[go] \"\"\n",
	} {
		if got := parse(src); got != want {
			t.Errorf("%q:\ngot  %swant %s", src, got, want)
		}
	}
}

func TestOnlyWebAndMailLinksAreKept(t *testing.T) {
	for _, dest := range []string{"javascript:alert(1)", "file:///etc/passwd", "ssh://host", "./local.md", "data:text/html,x"} {
		got := parse("[click me](" + dest + ")")
		if got != "P0 {click me}\n" {
			t.Errorf("destination %q produced %q, want the label as plain text", dest, got)
		}
	}
	for _, dest := range []string{"http://a.b/c", "HTTPS://A.B", "mailto:a@b.c"} {
		if got := parse("[x](" + dest + ")"); !strings.Contains(got, "-> "+dest) {
			t.Errorf("destination %q was dropped: %q", dest, got)
		}
	}
}

func TestAVeryLongAddressIsNotMadeALink(t *testing.T) {
	var d Document
	d.Parse("http://" + strings.Repeat("a", 5000))
	for _, span := range d.Spans(&d.Blocks[0]) {
		if len(span.Link) > maxLink || !safeLink(span.Link) && span.Link != "" {
			t.Fatalf("a link of %d bytes was kept", len(span.Link))
		}
	}
}

func TestListsNestByIndentAndKeepTheirParagraphs(t *testing.T) {
	src := "- a\n  - b\n    - c\n  - d\n- e\n\n1. one\n\n   more of one\n\n2. two\n   continued\n"
	want := "I0 {a}\nI1 {b}\nI2 {c}\nI1 {d}\nI0 {e}\n" +
		"N0.1 {one}\nP1 {more of one}\nN0.2 {two}\\n{continued}\n"
	if got := parse(src); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestACodeBlockInsideAListLosesTheListsIndent(t *testing.T) {
	src := "1. Run:\n   ```sh\n   ls\n     -la\n   ```\n2. Done\n"
	want := "N0.1 {Run:}\nCODE[sh] \"ls\\n  -la\"\nN0.2 {Done}\n"
	if got := parse(src); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestTablesHaveAlignedCellsAndPadShortRows(t *testing.T) {
	src := "| Name | Port | Note |\n|:--|--:|:-:|\n| ssh | 22 |\n| **web** | 80 | ok | extra |\n\nafter"
	want := "TABLE 3x3 | {Name} {Port}> {Note}^ | {ssh} {22}> ^ | {web:b} {80}> {ok}^\nP0 {after}\n"
	if got := parse(src); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
	// A pipe in running text is not a table.
	if got := parse("a | b\nc | d"); strings.Contains(got, "TABLE") {
		t.Errorf("text with pipes became a table: %q", got)
	}
}

func TestEveryPrefixOfADocumentParses(t *testing.T) {
	src := "# T\n- **a** `b` [c](https://d.e)\n> q\n```x\ny\n```\n| a | b |\n|---|---|\n| 1 | 2 |\n\u4e2d\u6587 **\u7c97**\n"
	var d Document
	for end := 0; end <= len(src); end++ {
		if !utf8.ValidString(src[:end]) {
			continue
		}
		d.Parse(src[:end])
		for index := range d.Blocks {
			b := &d.Blocks[index]
			for _, span := range d.Spans(b) {
				if span.Text == "" {
					t.Fatalf("prefix %d has an empty span in block %d", end, index)
				}
			}
		}
	}
}

func TestParsingAgainDoesNotAllocate(t *testing.T) {
	src := strings.Repeat("## Title\nSome **bold** and `code` with [a link](https://example.com).\n- item *one*\n- item two\n> quote\n| a | b |\n|---|---|\n| 1 | 2 |\n\n```go\ncode\n```\n", 20)
	var d Document
	d.Parse(src)
	if allocs := testing.AllocsPerRun(20, func() { d.Parse(src) }); allocs != 0 {
		t.Errorf("a warm Parse allocates %v times", allocs)
	}
}

func TestHostileTextCostsLinearTime(t *testing.T) {
	// Without a budget each of these is quadratic: every opener scans to the
	// end of the line for a closer that is not there.
	for _, unit := range []string{"*a ", "[", "`", "_x", "~~", "[a](b ", "**"} {
		src := strings.Repeat(unit, 200000)
		var d Document
		start := time.Now()
		d.Parse(src)
		if elapsed := time.Since(start); elapsed > 3*time.Second {
			t.Errorf("%q repeated took %v", unit, elapsed)
		}
	}
}

func FuzzParse(f *testing.F) {
	for _, seed := range []string{
		"", "# h\n- a\n  - b\n", "**a** _b_ `c` [d](https://e)", "```\ncode", "| a |\n|---|\n| b |",
		"> > q\n> r", "1. a\n2) b", "[a]((b)", "\\*x\\*", "~~a~~ ***b***", "http://a.b/(c).",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, src string) {
		var d Document
		d.Parse(src)
		last := 0
		for index := range d.Blocks {
			b := &d.Blocks[index]
			if b.Start < last || b.End < b.Start || b.End > len(src) {
				t.Fatalf("block %d range %d..%d after %d in %d bytes", index, b.Start, b.End, last, len(src))
			}
			last = b.End
			for _, span := range d.Spans(b) {
				if span.Link != "" && !safeLink(span.Link) {
					t.Fatalf("unsafe link %q kept", span.Link)
				}
			}
			for _, cell := range d.Cells(b) {
				d.CellSpans(cell)
			}
			if b.Kind == Table && b.Columns < 1 {
				t.Fatalf("table without columns")
			}
		}
	})
}

func BenchmarkParse(b *testing.B) {
	src := strings.Repeat("## Title\nSome **bold** and `code` with [a link](https://example.com).\n- item *one*\n- item two\n> quote\n\n```go\ncode\n```\n", 40)
	var d Document
	b.ReportAllocs()
	b.SetBytes(int64(len(src)))
	for b.Loop() {
		d.Parse(src)
	}
}

// The range of each block is the text of its own lines: re-parsing just that
// range gives back the same kind, so the ranges name the right bytes whatever
// the block is made of.
func TestBlockRangesNameTheLinesOfTheirBlocks(t *testing.T) {
	cases := []struct {
		src  string
		want []string
	}{
		{"# Title\n\nfirst\nsecond\n\n- a\n- b\n\n```go\nx := 1\n\n```\n\n| h |\n|---|\n| c |\n\n---\n", []string{
			"# Title", "first\nsecond", "- a", "- b", "```go\nx := 1\n\n```", "| h |\n|---|\n| c |", "---"}},
		{"中文段落\r\n续行\r\n\r\n> 引用\r\n", []string{"中文段落\r\n续行", "> 引用"}},
		{"```\nunclosed\ncode", []string{"```\nunclosed\ncode"}},
		{"no trailing break", []string{"no trailing break"}},
		{"", nil},
	}
	for _, c := range cases {
		var d Document
		d.Parse(c.src)
		if len(d.Blocks) != len(c.want) {
			t.Fatalf("%q: %d blocks, want %d", c.src, len(d.Blocks), len(c.want))
		}
		last := 0
		for i, b := range d.Blocks {
			if b.Start < last || b.End < b.Start || b.End > len(c.src) {
				t.Fatalf("%q block %d: range %d..%d after %d", c.src, i, b.Start, b.End, last)
			}
			if got := c.src[b.Start:b.End]; got != c.want[i] {
				t.Errorf("%q block %d = %q, want %q", c.src, i, got, c.want[i])
			}
			last = b.End
		}
	}
}

func TestAnImageIsASpanWithItsDestination(t *testing.T) {
	var d Document
	d.Parse("see ![the diagram](docs/arch.png) and [![build](https://x/badge.svg)](https://x/ci)")
	var images []Span
	for _, span := range d.Spans(&d.Blocks[0]) {
		if span.Image != "" {
			images = append(images, span)
		}
	}
	if len(images) != 2 {
		t.Fatalf("images = %+v", images)
	}
	if images[0].Image != "docs/arch.png" || images[0].Text != "the diagram" || images[0].Link != "" {
		t.Errorf("first image %+v", images[0])
	}
	if images[1].Image != "https://x/badge.svg" || images[1].Link != "https://x/ci" {
		t.Errorf("a linked badge %+v", images[1])
	}
}
