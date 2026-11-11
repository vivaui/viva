// Package markdown parses the Markdown that chat assistants and release notes
// are written in: headings, paragraphs, bulleted and numbered lists, block
// quotes, fenced code, rules and pipe tables, with bold, italic, strike,
// inline code and links inside them.
//
// It is a reader for display, not a CommonMark implementation. The result is a
// flat list of blocks (lists and quotes are a depth on a block, not a tree)
// whose text is sliced from the source, so parsing builds no strings except for
// the rare fenced block that sits inside a list or a quote. A Document keeps its
// storage between calls to Parse: re-parsing a message that grows by a few
// words allocates nothing once the Document is warm.
//
// A newline inside a paragraph is a line break, as it is in chat. Text that is
// not closed (a "**" with no partner, a fence nobody ended yet) stays as typed,
// so half a message that is still arriving reads sensibly. Links are kept only
// when they point to http, https or mailto; any other destination is shown as
// plain text. HTML, footnotes and indented code are not interpreted; an image is a
// Span whose Image is its destination.
package markdown

import "strings"

// Kind says what a Block is.
type Kind uint8

const (
	// Paragraph is running text.
	Paragraph Kind = iota
	// Heading is a line that begins with one to six "#".
	Heading
	// Item is one list item. A line that continues it belongs to the same Item.
	Item
	// Code is a fenced block of code.
	Code
	// Rule is a horizontal rule.
	Rule
	// Table is a pipe table; its first row is the header.
	Table
)

// Style is the inline formatting of a Span.
type Style uint8

const (
	Bold Style = 1 << iota
	Italic
	Strike
	// Mono marks inline code.
	Mono
)

// Span is a run of inline text with one style and at most one link.
type Span struct {
	Text  string
	Style Style
	// Link is the destination when the text is a link, and empty otherwise.
	// Only http, https and mailto destinations are ever set.
	Link string
	// Image is the destination of an image (![alt](destination)) as written,
	// whatever its scheme, and empty for text; Text is then the alt text. The
	// package never reads it: a view asks the application for the bitmap.
	Image string
}

// Newline is the Text of the Span that separates two lines of a block.
const Newline = "\n"

// Align is the horizontal alignment of a table column.
type Align uint8

const (
	AlignLeft Align = iota
	AlignCenter
	AlignRight
)

// Cell is one cell of a Table. Read its text with Document.CellSpans.
type Cell struct {
	Align  Align
	lo, hi int32
}

// Block is one block of a Document, in reading order.
type Block struct {
	Kind Kind
	// Level is the heading level (1 to 6) of a Heading and the nesting depth of
	// an Item, 0 for the outermost list. For a Paragraph it is how many list
	// levels the paragraph sits inside: 0 for ordinary text, 1 for a paragraph
	// indented under an outermost item.
	Level int
	// Quote is how many block quotes the block is inside.
	Quote int
	// Number is the value of an ordered Item's marker, and 0 for a bullet.
	Number int
	// Info is the text after the opening fence of a Code block: usually a
	// language name.
	Info string
	// Text is the content of a Code block, without the final line break.
	Text string
	// Columns is the number of columns of a Table.
	Columns int
	// Start and End are the byte range of the block in the text given to
	// Document.Parse: from the first byte of its first line to the end of its
	// last line, not counting that line's break. Both lie on line boundaries,
	// which are character boundaries whatever the text, and the ranges of
	// consecutive blocks do not overlap. For a Code block they include the
	// fences; for a quoted or indented block, the quote markers and the indent.
	Start, End int

	lo, hi int32 // inline spans, or table cells
}

// Document is a parsed text. The zero value is empty and ready for Parse.
type Document struct {
	Blocks []Block

	spans []Span
	cells []Cell
	stack []int // marker indents of the open list levels
	code  []byte
	work  int // inline scanning left, see inline.go
}

// Spans returns the inline content of a Paragraph, Heading or Item. It is valid
// until the next call to Parse.
func (d *Document) Spans(b *Block) []Span {
	if b.Kind == Table || b.Kind == Code || b.Kind == Rule {
		return nil
	}
	return d.spans[b.lo:b.hi]
}

// Cells returns the cells of a Table, row by row: Columns cells make a row and
// the first row is the header.
func (d *Document) Cells(b *Block) []Cell {
	if b.Kind != Table {
		return nil
	}
	return d.cells[b.lo:b.hi]
}

// CellSpans returns the inline content of a table cell.
func (d *Document) CellSpans(c Cell) []Span { return d.spans[c.lo:c.hi] }

// Limits that keep a hostile text from costing more than a linear amount of work.
const (
	maxQuote   = 8
	maxLevel   = 16
	maxColumns = 24
	maxLink    = 2048
	maxNesting = 6
)

// Parse replaces the content of d with the blocks of src.
func (d *Document) Parse(src string) {
	d.Blocks = d.Blocks[:0]
	d.spans = d.spans[:0]
	d.cells = d.cells[:0]
	d.stack = d.stack[:0]

	var (
		code       bool // inside a fenced block
		raw        bool // its text is a slice of src; otherwise it is collected in d.code
		fence      byte
		fenceLen   int
		fenceQuote int
		fenceInd   int
		codeStart  int
		codeEnd    int
		codeLines  int
		// open is the block that the next line continues if it is plain text: the
		// paragraph or item just before it, with no blank line between. -1 if none.
		open = -1
	)
	finishCode := func() {
		block := &d.Blocks[len(d.Blocks)-1]
		if raw {
			block.Text = src[codeStart:max(codeEnd, codeStart)]
		} else {
			block.Text = string(d.code)
		}
		code = false
	}
	for pos := 0; pos < len(src); {
		lineStart := pos
		end := strings.IndexByte(src[pos:], '\n')
		if end < 0 {
			end = len(src)
			pos = len(src)
		} else {
			end += pos
			pos = end + 1
		}
		line := strings.TrimSuffix(src[lineStart:end], "\r")
		lineEnd := lineStart + len(line)

		if code {
			body := line
			if fenceQuote > 0 {
				_, body = stripQuote(line)
			}
			d.Blocks[len(d.Blocks)-1].End = lineEnd
			if trimmed, indent := trimIndent(body); indent <= 3 && closesFence(trimmed, fence, fenceLen) {
				finishCode()
				continue
			}
			if raw {
				codeEnd = lineStart + len(line)
			} else {
				if codeLines > 0 {
					d.code = append(d.code, '\n')
				}
				d.code = append(d.code, cutIndent(body, fenceInd)...)
			}
			codeLines++
			continue
		}

		quote, line := stripQuote(line)
		trimmed, indent := trimIndent(line)
		if trimmed == "" {
			open = -1
			continue
		}
		if indent <= 3 {
			if ch, n, info, ok := opensFence(trimmed); ok {
				code, fence, fenceLen, fenceQuote, fenceInd, codeLines = true, ch, n, quote, indent, 0
				raw = quote == 0 && indent == 0
				codeStart, codeEnd = pos, pos
				d.code = d.code[:0]
				d.Blocks = append(d.Blocks, Block{Kind: Code, Quote: quote, Info: info, Start: lineStart, End: lineEnd})
				d.stack = d.stack[:0]
				open = -1
				continue
			}
			if isRule(trimmed) {
				d.Blocks = append(d.Blocks, Block{Kind: Rule, Quote: quote, Start: lineStart, End: lineEnd})
				d.stack = d.stack[:0]
				open = -1
				continue
			}
			if level, text, ok := heading(trimmed); ok {
				lo := int32(len(d.spans))
				d.inline(text, 0, "", 0)
				d.Blocks = append(d.Blocks, Block{Kind: Heading, Level: level, Quote: quote, Start: lineStart, End: lineEnd, lo: lo, hi: int32(len(d.spans))})
				d.stack = d.stack[:0]
				open = -1
				continue
			}
		}
		if number, text, ok := listItem(trimmed); ok {
			// The levels of a list are told apart by how far its markers are indented.
			for len(d.stack) > 0 && d.stack[len(d.stack)-1] > indent {
				d.stack = d.stack[:len(d.stack)-1]
			}
			if len(d.stack) == 0 || d.stack[len(d.stack)-1] < indent {
				d.stack = append(d.stack, indent)
			}
			lo := int32(len(d.spans))
			d.inline(text, 0, "", 0)
			d.Blocks = append(d.Blocks, Block{
				Kind: Item, Level: min(len(d.stack)-1, maxLevel), Quote: quote, Number: number,
				Start: lineStart, End: lineEnd, lo: lo, hi: int32(len(d.spans)),
			})
			open = len(d.Blocks) - 1
			continue
		}
		if pos < len(src) && strings.IndexByte(trimmed, '|') >= 0 {
			if columns := tableSeparator(src[pos:]); columns > 0 {
				pos = d.table(trimmed, src, pos, quote, columns)
				// The table ends where its last row does, before that row's break.
				table := &d.Blocks[len(d.Blocks)-1]
				table.Start, table.End = lineStart, pos
				if table.End > lineStart && src[table.End-1] == '\n' {
					table.End--
				}
				if table.End > lineStart && src[table.End-1] == '\r' {
					table.End--
				}
				d.stack = d.stack[:0]
				open = -1
				continue
			}
		}
		if open >= 0 && d.Blocks[open].Quote == quote {
			// Another line of the same paragraph or item.
			d.spans = append(d.spans, Span{Text: Newline})
			d.inline(trimmed, 0, "", 0)
			d.Blocks[open].hi = int32(len(d.spans))
			d.Blocks[open].End = lineEnd
			continue
		}
		// Running text. Indented under a list item it belongs to that item.
		level := 0
		for _, marker := range d.stack {
			if marker < indent {
				level++
			}
		}
		if level == 0 {
			d.stack = d.stack[:0]
		}
		lo := int32(len(d.spans))
		d.inline(trimmed, 0, "", 0)
		d.Blocks = append(d.Blocks, Block{Kind: Paragraph, Level: min(level, maxLevel), Quote: quote, Start: lineStart, End: lineEnd, lo: lo, hi: int32(len(d.spans))})
		open = len(d.Blocks) - 1
	}
	if code {
		finishCode()
	}
}

// stripQuote removes the block quote markers at the start of line and counts them.
func stripQuote(line string) (depth int, rest string) {
	for depth < maxQuote {
		trimmed, indent := trimIndent(line)
		if indent > 3 || trimmed == "" || trimmed[0] != '>' {
			break
		}
		line = strings.TrimPrefix(trimmed[1:], " ")
		depth++
	}
	return depth, line
}

// trimIndent cuts the blanks off both ends of line and counts those at the start
// (a tab counts as four).
func trimIndent(line string) (trimmed string, indent int) {
	i := 0
scan:
	for ; i < len(line); i++ {
		switch line[i] {
		case ' ':
			indent++
		case '\t':
			indent += 4
		default:
			break scan
		}
	}
	return strings.TrimRight(line[i:], " \t"), indent
}

// cutIndent removes up to n leading blanks from line.
func cutIndent(line string, n int) string {
	i := 0
	for i < len(line) && i < n && line[i] == ' ' {
		i++
	}
	return line[i:]
}

// opensFence recognizes ``` or ~~~ (three or more) with an optional info string.
func opensFence(trimmed string) (ch byte, n int, info string, ok bool) {
	if trimmed == "" || (trimmed[0] != '`' && trimmed[0] != '~') {
		return 0, 0, "", false
	}
	ch = trimmed[0]
	for n < len(trimmed) && trimmed[n] == ch {
		n++
	}
	if n < 3 {
		return 0, 0, "", false
	}
	info = strings.TrimSpace(trimmed[n:])
	if ch == '`' && strings.IndexByte(info, '`') >= 0 {
		return 0, 0, "", false // ```code``` on one line is inline code
	}
	if i := strings.IndexAny(info, " \t"); i >= 0 {
		info = info[:i]
	}
	return ch, n, info, true
}

// closesFence reports whether trimmed ends a block opened with n fence characters.
func closesFence(trimmed string, ch byte, n int) bool {
	count := 0
	for count < len(trimmed) && trimmed[count] == ch {
		count++
	}
	return count >= n && count == len(trimmed)
}

// isRule recognizes three or more of "-", "*" or "_", with blanks allowed between.
func isRule(trimmed string) bool {
	ch := trimmed[0]
	if ch != '-' && ch != '*' && ch != '_' {
		return false
	}
	count := 0
	for i := 0; i < len(trimmed); i++ {
		switch trimmed[i] {
		case ch:
			count++
		case ' ', '\t':
		default:
			return false
		}
	}
	return count >= 3
}

// heading recognizes one to six "#" followed by a blank or the end of the line.
func heading(trimmed string) (level int, text string, ok bool) {
	for level < len(trimmed) && trimmed[level] == '#' {
		level++
	}
	if level == 0 || level > 6 || (level < len(trimmed) && trimmed[level] != ' ' && trimmed[level] != '\t') {
		return 0, "", false
	}
	text = strings.TrimSpace(trimmed[level:])
	// A closing run of "#" is decoration.
	if cut := strings.TrimRight(text, "#"); cut != text && (cut == "" || cut[len(cut)-1] == ' ') {
		text = strings.TrimSpace(cut)
	}
	return level, text, true
}

// listItem recognizes "- ", "* ", "+ " and "1. " or "1) " markers.
func listItem(trimmed string) (number int, text string, ok bool) {
	switch trimmed[0] {
	case '-', '*', '+':
		if len(trimmed) == 1 {
			return 0, "", true
		}
		if trimmed[1] == ' ' || trimmed[1] == '\t' {
			return 0, strings.TrimSpace(trimmed[2:]), true
		}
		return 0, "", false
	}
	digits := 0
	for digits < len(trimmed) && digits < 9 && trimmed[digits] >= '0' && trimmed[digits] <= '9' {
		number = number*10 + int(trimmed[digits]-'0')
		digits++
	}
	if digits == 0 || digits >= len(trimmed) || (trimmed[digits] != '.' && trimmed[digits] != ')') {
		return 0, "", false
	}
	if digits+1 == len(trimmed) {
		return number, "", true
	}
	if trimmed[digits+1] != ' ' && trimmed[digits+1] != '\t' {
		return 0, "", false
	}
	return number, strings.TrimSpace(trimmed[digits+2:]), true
}
