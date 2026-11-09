package ui

// ListState is what a List knows of its rows between paints: the height of each row
// it has laid out, from which it places the rows it has not. Keep one per list in
// the model; the zero value is an empty list.
type ListState struct {
	// Key names row i, so a row keeps its focus and state as rows are inserted
	// above it: an item's ID. Nil names a row by its index.
	Key func(i int) int

	heights []float32 // the measured height of each row, zero when not measured
	tree    listTree
	total   float64 // the sum of the measured heights
	count   int     // how many rows are measured
}

// estimate is the height a row not yet measured is taken to have: the average of
// the measured ones, or a line of text before any is.
func (s *ListState) estimate() float32 {
	if s.count == 0 {
		return 32
	}
	return float32(s.total / float64(s.count))
}

// resize keeps the heights of the rows that remain when the list has n rows.
func (s *ListState) resize(n int) {
	if n == len(s.heights) {
		return
	}
	heights := s.heights
	if n < len(heights) {
		heights = heights[:n]
	} else {
		heights = append(heights, make([]float32, n-len(heights))...)
	}
	s.heights = heights
	s.tree.build(heights)
	s.total, s.count = 0, 0
	for _, h := range heights {
		if h > 0 {
			s.total += float64(h)
			s.count++
		}
	}
}

// measured records row i's height after a layout.
func (s *ListState) measured(i int, h float32) {
	if i >= len(s.heights) || h <= 0 || s.heights[i] == h {
		return
	}
	old := s.heights[i]
	s.heights[i] = h
	known := int32(0)
	if old == 0 {
		known, s.count = 1, s.count+1
	}
	s.total += float64(h - old)
	s.tree.add(i, float64(h-old), known)
}

// top is where row i starts.
func (s *ListState) top(i int) float32 {
	sum, known := s.tree.prefix(i)
	return float32(sum + float64(int32(i)-known)*float64(s.estimate()))
}

// rowAt is the row at offset y.
func (s *ListState) rowAt(y float32) int {
	return s.tree.find(float64(y), float64(s.estimate()), len(s.heights))
}

// listTree is a Fenwick tree of the measured heights and of how many rows are
// measured, so the start of a row and the row at an offset are found in log n
// steps in a list of millions.
type listTree struct {
	sum   []float64
	known []int32
}

func (t *listTree) build(heights []float32) {
	n := len(heights)
	t.sum = append(t.sum[:0], make([]float64, n+1)...)
	t.known = append(t.known[:0], make([]int32, n+1)...)
	for i, h := range heights {
		if h > 0 {
			t.add(i, float64(h), 1)
		}
	}
}

func (t *listTree) add(i int, dh float64, dk int32) {
	for j := i + 1; j < len(t.sum); j += j & -j {
		t.sum[j] += dh
		t.known[j] += dk
	}
}

// prefix sums rows [0, i).
func (t *listTree) prefix(i int) (float64, int32) {
	var sum float64
	var known int32
	for j := i; j > 0; j -= j & -j {
		sum += t.sum[j]
		known += t.known[j]
	}
	return sum, known
}

// find returns the last row that starts at or above y.
func (t *listTree) find(y, estimate float64, n int) int {
	if n == 0 {
		return 0
	}
	at := 0
	var sum float64
	var known int32
	step := 1
	for step*2 <= n {
		step *= 2
	}
	for ; step > 0; step /= 2 {
		next := at + step
		if next > n {
			continue
		}
		s, k := sum+t.sum[next], known+t.known[next]
		if s+float64(int32(next)-k)*estimate <= y {
			at, sum, known = next, s, k
		}
	}
	return min(at, n-1)
}

type listDecl struct {
	state       *ListState
	first       int
	rows, until int32 // the List's rows in Page.listRows
}

// listOverscan is how many rows a List declares beyond what is visible each way, so
// a small scroll shows rows already laid out.
const listOverscan = 2

// List is a scrolling list of n rows of which it declares only the ones on screen,
// with row declaring row i: a list of a million rows costs what its visible rows
// cost. Rows may be of any content and height; a row's height is measured as it
// shows and the others are estimated from the measured ones. Give it a size, or
// Grow it in its parent.
//
//	ui.List(c, &m.files, len(m.files.items), func(i int) {
//		ui.Text(c, m.files.items[i].Name)
//	}).Grow(1)
func List(c C, state *ListState, n int, row func(i int)) Box {
	p := c.page
	p.forbidInMemo("List")
	at, _ := p.name(pkContainer)
	area := c.f.ScrollArea(p.id(at))
	p.setNode(area.i, at)
	state.resize(n)
	scroll := c.f.exts[c.f.nodes[area.i].ext].scrollState
	viewport := scroll.Viewport
	if viewport <= 0 {
		viewport = c.f.bounds.Height
	}
	first := max(state.rowAt(scroll.Offset)-listOverscan, 0)
	if n == 0 {
		first = 0
	}
	start := len(p.listRows)
	area.Children(func() {
		if top := state.top(first); top > 0 {
			c.f.Slot().Height(top)
		}
		end := first
		bottom := scroll.Offset + viewport
		for i := first; i < n; i++ {
			key := i
			if state.Key != nil {
				key = state.Key(i)
			}
			box := Column(c.KeyInt(key)).Shrink(0).Children(func() { row(i) })
			p.listRows = append(p.listRows, box.i)
			end = i + 1
			if state.top(end) >= bottom && end-first > listOverscan && end > state.rowAt(bottom)+listOverscan {
				break
			}
		}
		if rest := state.top(n) - state.top(end); rest > 0 {
			c.f.Slot().Height(rest)
		}
	})
	p.lists = append(p.lists, listDecl{state: state, first: first, rows: int32(start), until: int32(len(p.listRows))})
	return area
}

// measureLists records the heights the layout gave the rows the Lists declared.
func (p *Page) measureLists() {
	for _, list := range p.lists {
		for k, node := range p.listRows[list.rows:list.until] {
			list.state.measured(list.first+k, p.ctx.f.nodes[node].rect.Height)
		}
	}
	clear(p.lists)
	p.lists = p.lists[:0]
	p.listRows = p.listRows[:0]
}
