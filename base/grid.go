package base

// A Grid has as many equal columns as fit and rows as tall as their tallest cell.
// An element may span several columns (ColSpan) and rows (RowSpan), as in CSS grid
// with auto-placement:
//
//   - Elements are placed in declaration order, left to right then down, each at the
//     first free place after the one before it (the cursor never goes back, as CSS's
//     default sparse packing), skipping cells that a row-spanning element above has
//     taken. A span wider than the grid is as wide as the grid.
//   - A column span is as wide as the cells it covers and the gaps between them.
//   - Rows are sized by the elements that cover one row; an element that covers
//     several rows then grows the rows it crosses, equally, by what they lack, and
//     only when they lack something. Shorter spans are settled before longer ones.

// ColSpan and RowSpan make a Grid child cover that many columns or rows (at least
// one). They have no effect outside a Grid.
func (b Box) ColSpan(n int) Box { b.N().ColSpan = spanOf(n); return b }
func (b Box) RowSpan(n int) Box { b.N().RowSpan = spanOf(n); return b }

func spanOf(n int) uint8 { return uint8(min(max(n, 1), 255)) }

// span is how many columns and rows a child covers; the zero value is one.
func (n *Node) Span() (cols, rows int) {
	return max(int(n.ColSpan), 1), max(int(n.RowSpan), 1)
}

// gridColumns is how many cells of at least minCell fit in width.
func gridColumns(width, minCell, Gap float32) int {
	return max(int((width+Gap)/(minCell+Gap)), 1)
}

// placeGrid gives every child of the grid its column and row, and reports how many rows
// there are.
func (f *Frame) placeGrid(n *Node, columns int) int {
	s := &f.scratch
	total, tall := 0, false
	for c := n.First; c >= 0; c = f.Nodes[c].Next {
		_, rs := f.Nodes[c].Span()
		total += rs
		tall = tall || rs > 1
	}
	if tall {
		size := columns * total
		if cap(s.cells) < size {
			s.cells = make([]bool, size)
		}
		s.cells = s.cells[:size]
		clear(s.cells)
	}
	col, row, rows := 0, 0, 0
	for c := n.First; c >= 0; c = f.Nodes[c].Next {
		child := &f.Nodes[c]
		cs, rs := child.Span()
		cs = min(cs, columns)
		for {
			if col+cs > columns {
				col, row = 0, row+1
			}
			if !tall || s.free(columns, col, row, cs, rs) {
				break
			}
			col++
		}
		child.GridCol, child.GridRow = uint16(col), uint16(row)
		if tall {
			for r := row; r < row+rs; r++ {
				for k := col; k < col+cs; k++ {
					s.cells[r*columns+k] = true
				}
			}
		}
		col += cs
		rows = max(rows, row+rs)
	}
	return rows
}

// free reports whether the cells cs wide and rs tall at (col, row) are all untaken.
func (s *scratch) free(columns, col, row, cs, rs int) bool {
	for r := row; r < row+rs; r++ {
		for k := col; k < col+cs; k++ {
			if s.cells[r*columns+k] {
				return false
			}
		}
	}
	return true
}

// gridRows sizes the rows of a placed grid into scratch.rows and their offsets from the
// top of the content into scratch.rowAt, and returns how many there are.
func (f *Frame) gridRows(n *Node) int {
	s := &f.scratch
	rows, widest := 0, 1
	for c := n.First; c >= 0; c = f.Nodes[c].Next {
		child := &f.Nodes[c]
		_, rs := child.Span()
		rows = max(rows, int(child.GridRow)+rs)
		widest = max(widest, rs)
	}
	s.rows = s.rows[:0]
	for range rows {
		s.rows = append(s.rows, 0)
	}
	for span := 1; span <= widest; span++ {
		for c := n.First; c >= 0; c = f.Nodes[c].Next {
			child := &f.Nodes[c]
			if _, rs := child.Span(); rs != span {
				continue
			}
			ms, me := child.marginOn(AxisY)
			need := child.Pref[AxisY] + ms + me
			first := int(child.GridRow)
			have := n.Gap * float32(span-1)
			for r := first; r < first+span; r++ {
				have += s.rows[r]
			}
			if need > have {
				add := (need - have) / float32(span)
				for r := first; r < first+span; r++ {
					s.rows[r] += add
				}
			}
		}
	}
	s.rowAt = s.rowAt[:0]
	var y float32
	for _, h := range s.rows {
		s.rowAt = append(s.rowAt, y)
		y += h + n.Gap
	}
	return rows
}

func (f *Frame) measureGrid(n *Node, axis int) {
	is, ie := n.Inset(axis)
	if axis == AxisX {
		// As narrow as one cell: the grid takes the width it is given and fits
		// as many columns as it can. An element spanning columns needs its share.
		widest := n.GridMin
		for c := n.First; c >= 0; c = f.Nodes[c].Next {
			child := &f.Nodes[c]
			cs, _ := child.Span()
			widest = max(widest, (child.Pref[AxisX]-n.Gap*float32(cs-1))/float32(cs))
		}
		n.Pref[AxisX] = widest + is + ie
		return
	}
	rows := f.gridRows(n)
	var total float32
	if rows > 0 {
		total = f.scratch.rowAt[rows-1] + f.scratch.rows[rows-1]
	}
	n.Pref[AxisY] = total + is + ie
}

// solveGrid gives every child the cells it covers.
func (f *Frame) solveGrid(n *Node, axis int, start, avail float32) {
	columns := gridColumns(n.Rect.Width-n.PadX(), n.GridMin, n.Gap)
	if axis == AxisX {
		cell := (avail - n.Gap*float32(columns-1)) / float32(columns)
		f.placeGrid(n, columns)
		for c := n.First; c >= 0; c = f.Nodes[c].Next {
			child := &f.Nodes[c]
			cs, _ := child.Span()
			cs = min(cs, columns)
			ms, me := child.marginOn(AxisX)
			covered := float32(cs)*cell + float32(cs-1)*n.Gap
			child.Rect.Width = clamp(covered-ms-me, child.MinS[AxisX], child.MaxS[AxisX])
			child.Rect.X = start + float32(child.GridCol)*(cell+n.Gap) + ms
		}
		return
	}
	f.gridRows(n)
	s := &f.scratch
	for c := n.First; c >= 0; c = f.Nodes[c].Next {
		child := &f.Nodes[c]
		_, rs := child.Span()
		first := int(child.GridRow)
		last := first + rs - 1
		ms, me := child.marginOn(AxisY)
		covered := s.rowAt[last] + s.rows[last] - s.rowAt[first]
		child.Rect.Y = start + s.rowAt[first] + ms
		child.Rect.Height = clamp(covered-ms-me, child.MinS[AxisY], child.MaxS[AxisY])
	}
}
