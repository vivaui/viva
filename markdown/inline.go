package markdown

import "strings"

// inline parses the inline formatting of s and appends its spans. style and
// link are what the surrounding text already has.
func (d *Document) inline(s string, style Style, link string, depth int) {
	if depth == 0 {
		// Every search below spends from this budget, so a line made of nothing
		// but unmatched delimiters costs time proportional to its length.
		d.work = 32*len(s) + 4096
	}
	text := 0 // where the plain text not yet emitted begins
	for i := 0; i < len(s); {
		switch c := s[i]; c {
		case '\\':
			if i+1 < len(s) && isPunct(s[i+1]) {
				d.emit(s[text:i], style, link)
				text = i + 1
				i += 2
				continue
			}
		case '`':
			n := run(s, i, '`')
			if end := d.findCodeEnd(s, i+n, n); end >= 0 {
				d.emit(s[text:i], style, link)
				code := s[i+n : end]
				if len(code) >= 2 && code[0] == ' ' && code[len(code)-1] == ' ' && strings.Trim(code, " ") != "" {
					code = code[1 : len(code)-1]
				}
				if code != "" {
					d.spans = append(d.spans, Span{Text: code, Style: style | Mono, Link: link})
				}
				i = end + n
				text = i
				continue
			}
			i += n
			continue
		case '*', '_':
			n := run(s, i, c)
			if n <= 3 && depth < maxNesting {
				if end := d.findEmphasisEnd(s, i, n, c); end >= 0 {
					d.emit(s[text:i], style, link)
					d.inline(s[i+n:end], style|emphasisStyle(n), link, depth+1)
					i = end + n
					text = i
					continue
				}
			}
			i += n
			continue
		case '~':
			n := run(s, i, '~')
			if n == 2 && depth < maxNesting {
				if end := d.findEmphasisEnd(s, i, n, c); end >= 0 {
					d.emit(s[text:i], style, link)
					d.inline(s[i+2:end], style|Strike, link, depth+1)
					i = end + 2
					text = i
					continue
				}
			}
			i += n
			continue
		case '!':
			if i+1 < len(s) && s[i+1] == '[' {
				if labelEnd, dest, end := d.findLink(s, i+1); end >= 0 {
					d.emit(s[text:i], style, link)
					d.spans = append(d.spans, Span{Text: s[i+2 : labelEnd], Style: style, Link: link, Image: dest})
					i = end
					text = i
					continue
				}
			}
		case '[':
			if link == "" && depth < maxNesting {
				if labelEnd, dest, end := d.findLink(s, i); end >= 0 {
					d.emit(s[text:i], style, link)
					if safeLink(dest) {
						d.inline(s[i+1:labelEnd], style, dest, depth+1)
					} else {
						d.inline(s[i+1:labelEnd], style, "", depth+1)
					}
					i = end
					text = i
					continue
				}
			}
		case 'h':
			if link == "" && (i == 0 || !isWord(s[i-1])) {
				if end := autolinkEnd(s, i); end > 0 {
					d.emit(s[text:i], style, link)
					d.spans = append(d.spans, Span{Text: s[i:end], Style: style, Link: s[i:end]})
					i = end
					text = i
					continue
				}
			}
		}
		i++
	}
	d.emit(s[text:], style, link)
}

func (d *Document) emit(text string, style Style, link string) {
	if text != "" {
		d.spans = append(d.spans, Span{Text: text, Style: style, Link: link})
	}
}

func emphasisStyle(n int) Style {
	switch n {
	case 1:
		return Italic
	case 2:
		return Bold
	}
	return Bold | Italic
}

// run counts the repeats of c at the start of s[i:].
func run(s string, i int, c byte) int {
	n := 0
	for i+n < len(s) && s[i+n] == c {
		n++
	}
	return n
}

// spend takes n from the scanning budget and reports whether any is left.
func (d *Document) spend(n int) bool {
	d.work -= n
	return d.work > 0
}

// findCodeEnd finds the run of exactly n backticks that closes a code span whose
// content starts at from, and returns where it starts, or -1.
func (d *Document) findCodeEnd(s string, from, n int) int {
	for j := from; j < len(s); {
		if !d.spend(1) {
			return -1
		}
		if s[j] != '`' {
			j++
			continue
		}
		m := run(s, j, '`')
		if m == n {
			return j
		}
		j += m
	}
	return -1
}

// findEmphasisEnd finds the closing delimiter of an emphasis that opens at i
// with n characters c, and returns where it starts, or -1. The opener must be
// followed by text and the closer preceded by text; "_" does not open or close
// inside a word, so snake_case stays as it is.
func (d *Document) findEmphasisEnd(s string, i, n int, c byte) int {
	if i+n >= len(s) || isSpace(s[i+n]) {
		return -1
	}
	if c == '_' && i > 0 && isWord(s[i-1]) {
		return -1
	}
	for j := i + n; j < len(s); {
		if !d.spend(1) {
			return -1
		}
		switch s[j] {
		case '\\':
			j += 2
		case '`':
			m := run(s, j, '`')
			if end := d.findCodeEnd(s, j+m, m); end >= 0 {
				j = end + m
			} else {
				j += m
			}
		case c:
			m := run(s, j, c)
			if m == n && j > i+n && !isSpace(s[j-1]) && (c != '_' || j+m >= len(s) || !isWord(s[j+m])) {
				return j
			}
			j += m
		default:
			j++
		}
	}
	return -1
}

// findLink recognizes [label](destination) at s[i]. It returns where the label
// ends (the index of "]"), the destination, and where the whole link ends; end
// is -1 when s[i:] is not a link.
func (d *Document) findLink(s string, i int) (labelEnd int, dest string, end int) {
	depth := 0
	j := i
	for ; j < len(s) && j-i <= 1000; j++ {
		if !d.spend(1) {
			return 0, "", -1
		}
		c := s[j]
		if c == '\\' {
			j++
			continue
		}
		if c == '[' {
			depth++
		} else if c == ']' {
			depth--
			if depth == 0 {
				break
			}
		}
	}
	if j >= len(s) || s[j] != ']' || depth != 0 || j+1 >= len(s) || s[j+1] != '(' {
		return 0, "", -1
	}
	labelEnd = j
	start := j + 2
	parens := 0
	k := start
	for ; k < len(s) && k-start <= maxLink; k++ {
		if !d.spend(1) {
			return 0, "", -1
		}
		c := s[k]
		if c == '(' {
			parens++
		} else if c == ')' {
			if parens == 0 {
				break
			}
			parens--
		} else if c == ' ' || c == '\t' {
			break // a title may follow
		}
	}
	if k >= len(s) {
		return 0, "", -1
	}
	dest = strings.TrimSuffix(strings.TrimPrefix(s[start:k], "<"), ">")
	if s[k] != ')' { // skip the title up to the closing parenthesis
		window := s[k:min(len(s), k+400)]
		if !d.spend(len(window)) {
			return 0, "", -1
		}
		close := strings.IndexByte(window, ')')
		if close < 0 {
			return 0, "", -1
		}
		k += close
	}
	return labelEnd, dest, k + 1
}

// safeLink reports whether dest is a destination worth making clickable.
func safeLink(dest string) bool {
	if dest == "" || len(dest) > maxLink {
		return false
	}
	for i := 0; i < len(dest); i++ {
		if dest[i] <= ' ' || dest[i] == 0x7f {
			return false
		}
	}
	lower := dest
	if len(lower) > 8 {
		lower = lower[:8]
	}
	lower = strings.ToLower(lower)
	return strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "mailto:")
}

// autolinkEnd returns where the bare http(s) address starting at s[i] ends, or 0
// if there is none.
func autolinkEnd(s string, i int) int {
	rest := s[i:]
	switch {
	case strings.HasPrefix(rest, "http://"):
		i += 7
	case strings.HasPrefix(rest, "https://"):
		i += 8
	default:
		return 0
	}
	start := i
	for i < len(s) && urlByte(s[i]) && i-start < maxLink-8 {
		i++
	}
	// Trailing punctuation belongs to the sentence, and so does a closing
	// bracket nobody opened.
	for i > start {
		c := s[i-1]
		if strings.IndexByte(".,;:!?'*_~", c) >= 0 {
			i--
			continue
		}
		if c == ')' && strings.Count(s[start:i], "(") < strings.Count(s[start:i], ")") {
			i--
			continue
		}
		if c == ']' && strings.Count(s[start:i], "[") < strings.Count(s[start:i], "]") {
			i--
			continue
		}
		break
	}
	if i <= start {
		return 0
	}
	return i
}

func urlByte(c byte) bool {
	return c > ' ' && c < 0x7f && c != '<' && c != '>' && c != '"' && c != '`' && c != '\\' && c != '|'
}

func isPunct(c byte) bool {
	return c > ' ' && c < 0x7f && !(c >= '0' && c <= '9') && !(c >= 'a' && c <= 'z') && !(c >= 'A' && c <= 'Z')
}

func isWord(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

func isSpace(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' }

// tableSeparator reports how many columns the line at the start of rest declares
// if it is the "---|:---:" line under a table header, and 0 if it is not.
func tableSeparator(rest string) int {
	end := strings.IndexByte(rest, '\n')
	if end < 0 {
		end = len(rest)
	}
	_, line := stripQuote(strings.TrimSuffix(rest[:end], "\r"))
	trimmed, _ := trimIndent(line)
	if strings.IndexByte(trimmed, '|') < 0 {
		return 0
	}
	columns := 0
	ok := true
	eachCell(trimmed, func(cell string) {
		columns++
		cell = strings.TrimSpace(cell)
		cell = strings.TrimSuffix(strings.TrimPrefix(cell, ":"), ":")
		if cell == "" || strings.Trim(cell, "-") != "" {
			ok = false
		}
	})
	if !ok || columns == 0 || columns > maxColumns {
		return 0
	}
	return columns
}

// eachCell calls fn with the text of every cell of a table row.
func eachCell(row string, fn func(cell string)) {
	row = strings.TrimSpace(row)
	row = strings.TrimPrefix(row, "|")
	start := 0
	for i := 0; i < len(row); i++ {
		switch row[i] {
		case '\\':
			i++
		case '|':
			fn(row[start:i])
			start = i + 1
		}
	}
	if strings.TrimSpace(row[start:]) != "" {
		fn(row[start:])
	}
}

// table reads the table whose header is header and whose separator line starts
// at pos, and returns where the text after it begins.
func (d *Document) table(header, src string, pos, quote, columns int) int {
	var aligns [maxColumns]Align
	count := 0
	sepEnd := strings.IndexByte(src[pos:], '\n')
	if sepEnd < 0 {
		sepEnd = len(src) - pos
	}
	_, sep := stripQuote(strings.TrimSuffix(src[pos:pos+sepEnd], "\r"))
	eachCell(sep, func(cell string) {
		cell = strings.TrimSpace(cell)
		left, right := strings.HasPrefix(cell, ":"), strings.HasSuffix(cell, ":")
		if count >= maxColumns {
			return
		}
		switch {
		case left && right:
			aligns[count] = AlignCenter
		case right:
			aligns[count] = AlignRight
		}
		count++
	})
	pos += sepEnd
	if pos < len(src) {
		pos++
	}
	first := int32(len(d.cells))
	row := func(text string) {
		column := 0
		eachCell(text, func(cell string) {
			if column >= columns {
				return
			}
			lo := int32(len(d.spans))
			d.inline(strings.TrimSpace(cell), 0, "", 0)
			d.cells = append(d.cells, Cell{Align: aligns[column], lo: lo, hi: int32(len(d.spans))})
			column++
		})
		for ; column < columns; column++ {
			at := int32(len(d.spans))
			d.cells = append(d.cells, Cell{Align: aligns[column], lo: at, hi: at})
		}
	}
	row(header)
	for pos < len(src) {
		end := strings.IndexByte(src[pos:], '\n')
		next := len(src)
		if end < 0 {
			end = len(src)
		} else {
			end += pos
			next = end + 1
		}
		_, line := stripQuote(strings.TrimSuffix(src[pos:end], "\r"))
		trimmed, _ := trimIndent(line)
		if trimmed == "" || strings.IndexByte(trimmed, '|') < 0 {
			break
		}
		row(trimmed)
		pos = next
	}
	d.Blocks = append(d.Blocks, Block{Kind: Table, Quote: quote, Columns: columns, lo: first, hi: int32(len(d.cells))})
	return pos
}
