package markdown

import "unsafe"

// TableCell describes one cell when constructing a Table block programmatically
// with Document.AppendTable.
type TableCell struct {
	Align Align
	Spans []Span
}

// Reset clears the blocks and spans of d while keeping its backing storage for
// reuse by subsequent Append calls or Parse.
func (d *Document) Reset() {
	d.Blocks = d.Blocks[:0]
	d.spans = d.spans[:0]
	d.cells = d.cells[:0]
	d.stack = d.stack[:0]
	d.code = d.code[:0]
}

// AppendParagraph adds a Paragraph block with caller-supplied spans. Unlike
// Parse, spans are preserved as given, including opaque caller link targets.
func (d *Document) AppendParagraph(quote, level int, spans []Span) {
	lo := int32(len(d.spans))
	d.spans = append(d.spans, spans...)
	d.Blocks = append(d.Blocks, Block{
		Kind:  Paragraph,
		Quote: clampInt(quote, 0, maxQuote),
		Level: clampInt(level, 0, maxLevel),
		lo:    lo,
		hi:    int32(len(d.spans)),
	})
}

// AppendHeading adds a Heading block at heading level 1..6.
func (d *Document) AppendHeading(quote, level int, spans []Span) {
	lo := int32(len(d.spans))
	d.spans = append(d.spans, spans...)
	d.Blocks = append(d.Blocks, Block{
		Kind:  Heading,
		Quote: clampInt(quote, 0, maxQuote),
		Level: clampInt(level, 1, 6),
		lo:    lo,
		hi:    int32(len(d.spans)),
	})
}

// AppendItem adds a list item block. number == 0 renders a bullet; number > 0
// renders an ordered marker.
func (d *Document) AppendItem(quote, level, number int, spans []Span) {
	lo := int32(len(d.spans))
	d.spans = append(d.spans, spans...)
	if number < 0 {
		number = 0
	}
	d.Blocks = append(d.Blocks, Block{
		Kind:   Item,
		Quote:  clampInt(quote, 0, maxQuote),
		Level:  clampInt(level, 0, maxLevel),
		Number: number,
		lo:     lo,
		hi:     int32(len(d.spans)),
	})
}

// AppendCode adds a preformatted code block.
func (d *Document) AppendCode(quote int, info, text string) {
	d.Blocks = append(d.Blocks, Block{
		Kind:  Code,
		Quote: clampInt(quote, 0, maxQuote),
		Info:  info,
		Text:  text,
	})
}

// AppendRule adds a horizontal divider rule.
func (d *Document) AppendRule(quote int) {
	d.Blocks = append(d.Blocks, Block{
		Kind:  Rule,
		Quote: clampInt(quote, 0, maxQuote),
	})
}

// AppendLabeled adds a borderless two-column label–content block.
func (d *Document) AppendLabeled(quote int, label, value []Span) {
	labelLo := int32(len(d.spans))
	d.spans = append(d.spans, label...)
	labelHi := int32(len(d.spans))
	lo := labelHi
	d.spans = append(d.spans, value...)
	d.Blocks = append(d.Blocks, Block{
		Kind:    Labeled,
		Quote:   clampInt(quote, 0, maxQuote),
		labelLo: labelLo,
		labelHi: labelHi,
		lo:      lo,
		hi:      int32(len(d.spans)),
	})
}

// AppendTable adds a Table block with the given column count and row-major
// cells. Rows shorter than columns are padded with empty cells; extra columns
// past maxColumns are ignored.
func (d *Document) AppendTable(quote, columns int, rows [][]TableCell) {
	columns = clampInt(columns, 0, maxColumns)
	if columns == 0 || len(rows) == 0 {
		return
	}
	cellLo := int32(len(d.cells))
	for _, row := range rows {
		for c := 0; c < columns; c++ {
			var align Align
			spanLo := int32(len(d.spans))
			if c < len(row) {
				align = row[c].Align
				d.spans = append(d.spans, row[c].Spans...)
			}
			d.cells = append(d.cells, Cell{Align: align, lo: spanLo, hi: int32(len(d.spans))})
		}
	}
	d.Blocks = append(d.Blocks, Block{
		Kind:    Table,
		Quote:   clampInt(quote, 0, maxQuote),
		Columns: columns,
		lo:      cellLo,
		hi:      int32(len(d.cells)),
	})
}

// AllocatedBytes reports a conservative estimate of retained heap memory held
// by d, including backing slice capacities and referenced string payloads.
func (d *Document) AllocatedBytes() int {
	total := cap(d.Blocks)*int(unsafe.Sizeof(Block{})) +
		cap(d.spans)*int(unsafe.Sizeof(Span{})) +
		cap(d.cells)*int(unsafe.Sizeof(Cell{})) +
		cap(d.stack)*int(unsafe.Sizeof(int(0))) +
		cap(d.code)
	for i := range d.Blocks {
		total += len(d.Blocks[i].Info) + len(d.Blocks[i].Text)
	}
	for i := range d.spans {
		total += len(d.spans[i].Text) + len(d.spans[i].Link) + len(d.spans[i].Image)
	}
	return total
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
