package ui

import (
	"runtime"
	"strings"
	"unsafe"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/keymap"
)

// ListState is what a List knows of its rows between paints: the height of each row
// it has laid out, from which it places the rows it has not, and the row the user
// chose. Keep one per list in the model; the zero value is an empty list.
type ListState struct {
	// Key names row i, so a row keeps its focus and state as rows are inserted
	// above it: an item's ID, unique among the rows. Nil names a row by its index.
	Key func(i int) int
	// Label is the text of row i: typing the first letters of a row's text while
	// the list has the keyboard focus chooses it, and assistive technology reads it
	// as the row's name. Nil leaves the rows unnamed.
	Label func(i int) string
	// Selection, when set, lets the user choose several rows, which it holds by
	// their keys (Key), or their indices without one: a click chooses one, Primary
	// (Command on macOS, Control elsewhere) with a click adds a row or takes it out,
	// Shift with a click or the keys chooses those from the row last chosen, and
	// Primary+A chooses all. On Linux and Windows Control with the keys moves
	// without choosing, and Control+Space adds the row there or takes it out.
	// Selected, set as well, is the row last chosen, where the keys move from.
	Selection *Selection

	heights []float32 // the measured height of each row, zero when not measured
	tree    listTree
	total   float64 // the sum of the measured heights
	count   int     // how many rows are measured
	list    listInternals
}

// listInternals is the part of ListState the list keeps for itself.
type listInternals struct {
	n            int
	scope        gfx.Scope // the list's place in the page
	rows         gfx.Scope // the identities of its rows, rows.At(key)
	selected     int32     // 1 + the row the keys are on, or 0
	anchor       int32     // 1 + the row a Shift range starts at, or 0
	span         int32     // where the last Shift range ended, from the anchor
	changed      bool
	changedAt    uint32 // the paint Changed is reported in
	submitted    bool
	submittedRow int32
	submittedAt  uint32
	first, last  int // the rows the last paint showed, last below first when none
	laidOut      bool
	scroll       *ScrollState
	pending      listScroll // where the application asked to scroll
	pendingRow   int32      // 1 + the row it asked to show
	declared     []listDeclRow
	f            *Frame
	typeahead    components.MenuTypeahead
	paint        func(*DrawContext) // the area's Draw, made once
}

// listDeclRow is a row a List or a Tree declared in the last paint.
type listDeclRow struct {
	index int
	key   int
	id    gfx.ID
}

// ListAlign is where ScrollTo puts a row in the list.
type ListAlign uint8

const (
	ListStart  ListAlign = iota // at the top (or the left)
	ListCenter                  // in the middle
	ListEnd                     // at the bottom (or the right)
)

// listScroll is a scroll the application asked for, applied at the next declaration.
type listScroll uint8

const (
	listScrollNone listScroll = iota
	listScrollStart
	listScrollCenter
	listScrollEnd
	listScrollNear
	listScrollToEnd
)

// Selected is the row the user chose last, or the keys are on, or -1.
func (s *ListState) Selected() int { return int(s.list.selected) - 1 }

// Select chooses row as if the user had clicked it without a modifier; a negative
// row clears the choice. It does not report Changed, and it scrolls nothing: see
// ScrollIntoView.
func (s *ListState) Select(row int) {
	row = min(max(row, -1), max(s.list.n-1, -1))
	s.list.selected = int32(row + 1)
	s.list.anchor, s.list.span = s.list.selected, 0
}

// Chosen reports whether row is in the choice: in the Selection when the list has
// one, else the selected row.
func (s *ListState) Chosen(row int) bool {
	if row < 0 || row >= s.list.n {
		return false
	}
	if s.Selection != nil {
		return s.Selection.Has(s.key(row))
	}
	return s.Selected() == row
}

// Changed reports whether the paint that declared the list delivered a choice the
// user made: a click, a key, typing a row's first letters. It is true in the paint
// that delivers it, and in the one after a key between paints, and false again in
// the next.
func (s *ListState) Changed() bool { return s.list.changed }

// Submitted reports the row Enter or a double click submitted, and whether this
// paint delivered one.
func (s *ListState) Submitted() (int, bool) {
	return int(s.list.submittedRow) - 1, s.list.submitted
}

// ScrollTo scrolls the list to row at align, at its next declaration.
func (s *ListState) ScrollTo(row int, align ListAlign) {
	kind := listScrollStart
	switch align {
	case ListCenter:
		kind = listScrollCenter
	case ListEnd:
		kind = listScrollEnd
	}
	s.list.pending, s.list.pendingRow = kind, int32(row+1)
}

// ScrollIntoView scrolls the list as little as shows row whole, at its next
// declaration.
func (s *ListState) ScrollIntoView(row int) {
	s.list.pending, s.list.pendingRow = listScrollNear, int32(row+1)
}

// ScrollToEnd scrolls the list to its end.
func (s *ListState) ScrollToEnd() {
	s.list.pending, s.list.pendingRow = listScrollToEnd, 0
}

// Visible returns the first and the last row the list showed in the last paint,
// last below first when it showed none: a list nearing the end of what was loaded
// loads more.
func (s *ListState) Visible() (first, last int) {
	if !s.list.laidOut {
		return 0, -1
	}
	return s.list.first, s.list.last
}

// AtEnd reports whether the list showed its end in the last paint.
func (s *ListState) AtEnd() bool {
	scroll := s.list.scroll
	return scroll != nil && scroll.AtEnd()
}

// key is row i's key: its index without a Key.
func (s *ListState) key(i int) int {
	if s.Key != nil {
		return s.Key(i)
	}
	return i
}

// markChanged records a choice the user made in epoch, the paint its reader sees
// it in.
func (s *ListState) markChanged(epoch uint32) {
	s.list.changed, s.list.changedAt = true, epoch
}

// submit records that Enter or a double click submitted row in epoch.
func (s *ListState) submit(row int, epoch uint32) {
	s.list.submitted, s.list.submittedRow, s.list.submittedAt = true, int32(row+1), epoch
}

// chooseRow makes row the choice, alone, and shows it.
func (s *ListState) chooseRow(row int, epoch uint32) {
	if s.Selection != nil {
		k := s.key(row)
		if s.Selection.Len() != 1 || !s.Selection.Has(k) {
			s.Selection.Clear()
			s.Selection.Add(k)
			s.markChanged(epoch)
		}
	}
	if s.Selected() != row {
		s.list.selected = int32(row + 1)
		s.markChanged(epoch)
	}
	s.list.anchor, s.list.span = int32(row+1), 0
	s.revealRow(row)
}

// toggleRow adds row to the choice or takes it out, and shows it.
func (s *ListState) toggleRow(row int, epoch uint32) {
	s.Selection.set(s.key(row), !s.Selection.Has(s.key(row)))
	s.markChanged(epoch)
	s.list.selected = int32(row + 1)
	s.list.anchor, s.list.span = int32(row+1), 0
	s.revealRow(row)
}

// extendRow chooses the rows from the anchor to row, as Shift does: on macOS in
// place of those it chose last, elsewhere in place of all others unless keep.
func (s *ListState) extendRow(row int, keep bool, epoch uint32) {
	sel := s.Selection
	pivot := int(s.list.anchor) - 1
	if pivot < 0 || pivot >= s.list.n {
		s.chooseRow(row, epoch)
		return
	}
	lo, hi := min(pivot, row), max(pivot, row)
	changed := false
	if runtime.GOOS == "darwin" {
		last := min(max(pivot+int(s.list.span), 0), s.list.n-1)
		for j := min(pivot, last); j <= max(pivot, last); j++ {
			if (j < lo || j > hi) && sel.set(s.key(j), false) {
				changed = true
			}
		}
	} else if !keep {
		in := 0
		for j := lo; j <= hi; j++ {
			if sel.Has(s.key(j)) {
				in++
			}
		}
		if sel.Len() > in {
			sel.Clear()
			changed = true
		}
	}
	for j := lo; j <= hi; j++ {
		if sel.set(s.key(j), true) {
			changed = true
		}
	}
	if changed {
		s.markChanged(epoch)
	}
	s.list.span = int32(row - pivot)
	s.leadRow(row, epoch)
}

// leadRow makes row the one the keys move from, without choosing it.
func (s *ListState) leadRow(row int, epoch uint32) {
	if s.Selected() != row {
		s.list.selected = int32(row + 1)
		s.markChanged(epoch)
	}
	s.revealRow(row)
}

// selectAll chooses every row, and reports whether that changed the choice.
func (s *ListState) selectAll(epoch uint32) bool {
	changed := false
	for i := 0; i < s.list.n; i++ {
		if s.Selection.set(s.key(i), true) {
			changed = true
		}
	}
	if changed {
		s.markChanged(epoch)
	}
	return changed
}

// revealRow scrolls the list so that row shows; the next paint declares it.
func (s *ListState) revealRow(row int) {
	scroll := s.list.scroll
	if scroll == nil || scroll.Viewport <= 0 || row < 0 || row >= s.list.n {
		return
	}
	top, bottom := s.top(row), s.top(row+1)
	switch {
	case top < scroll.Offset:
		scroll.ScrollTo(top)
	case bottom > scroll.Offset+scroll.Viewport:
		scroll.ScrollTo(bottom - scroll.Viewport)
	}
}

// indexOf is the row a row identity names in the last paint, or -1.
func (s *ListState) indexOf(id gfx.ID) int {
	key, ok := id.In(s.list.rows)
	if !ok {
		return -1
	}
	for i := range s.list.declared {
		if s.list.declared[i].key == key {
			return s.list.declared[i].index
		}
	}
	for i := 0; i < s.list.n; i++ { // scrolled out of sight
		if s.key(i) == key {
			return i
		}
	}
	return -1
}

// applyScrollRequest applies the scroll the application asked for, before the list
// places its rows.
func (s *ListState) applyScrollRequest(f *Frame) {
	in := &s.list
	if in.pending == listScrollNone || in.scroll == nil {
		return
	}
	kind, row := in.pending, int(in.pendingRow)-1
	in.pending, in.pendingRow = listScrollNone, 0
	scroll := in.scroll
	viewport := scroll.Viewport
	if viewport <= 0 {
		viewport = f.bounds.Height
	}
	switch kind {
	case listScrollToEnd:
		scroll.ScrollToEnd()
	case listScrollStart:
		if row >= 0 && row < in.n {
			scroll.ScrollTo(s.top(row))
		}
	case listScrollCenter:
		if row >= 0 && row < in.n {
			scroll.ScrollTo((s.top(row)+s.top(row+1))/2 - viewport/2)
		}
	case listScrollEnd:
		if row >= 0 && row < in.n {
			scroll.ScrollTo(s.top(row+1) - viewport)
		}
	case listScrollNear:
		if row >= 0 && row < in.n {
			top, bottom := s.top(row), s.top(row+1)
			switch {
			case top < scroll.Offset:
				scroll.ScrollTo(top)
			case bottom > scroll.Offset+viewport:
				scroll.ScrollTo(bottom - viewport)
			}
		}
	}
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

// ListRow is what a List tells of one of its visible rows, to the row builder: its
// index, key and identity, the page's context, and the row's element to style.
type ListRow struct {
	Index int
	Key   int
	ID    gfx.ID
	C     C
	// Element is the row's container. Style it (Background, PadY, ...), give it a
	// ContextMenu, and declare what is in it with C.
	Element Box
	state   *ListState
}

// Selected reports whether the row is in the choice: see ListState.Chosen.
func (r ListRow) Selected() bool { return r.state.Chosen(r.Index) }

// Current reports whether the row is the one the keys are on.
func (r ListRow) Current() bool { return r.state.Selected() == r.Index }

// Focused reports whether the keyboard focus is visibly on the row.
func (r ListRow) Focused() bool {
	return r.Element.f.canvas != nil && r.Element.f.canvas.FocusVisible() == r.ID
}

// List is a scrolling list of n rows of which it declares only the ones on screen:
// a list of a million rows costs what its visible rows cost. Rows may be of any
// content and height; a row's height is measured as it shows and the others are
// estimated from the measured ones. A click chooses a row; the arrows move between
// rows, Home and End to the first and last, Page Up and Down by a screen; Enter or a
// double click submits one (Submitted); typing a row's first letters moves to it
// (Label). With a Selection the user chooses several (see ListState).
//
// The rows are one stop of Tab, on the row chosen last or the first shown. Name the
// list for assistive technology with Box.Label after it:
//
//	files := ui.List(c, &m.files, len(m.files), func(row ui.ListRow) {
//		row.Element.PadX(8).Gap(8).Items(ui.Center).Children(func() {
//			ui.Icon(row.C, iconOf(m.files[row.Index]))
//			ui.Text(row.C, m.files[row.Index].Name)
//		})
//	}).Label("Files").Grow(1)
//	if row, ok := m.files.Submitted(); ok { m.open(row) }
func List(c C, state *ListState, n int, row func(ListRow)) Box {
	p := c.page
	p.forbidInMemo("List")
	in := &state.list
	in.n, in.f = n, c.f
	in.declared = in.declared[:0]
	if int(in.selected) > n {
		in.selected = 0
	}
	if int(in.anchor) > n {
		in.anchor = 0
	}
	if in.changedAt != p.epoch {
		in.changed = false
	}
	if in.submittedAt != p.epoch {
		in.submitted = false
	}
	if in.paint == nil {
		in.paint = in.draw // made once: a method value made in each paint allocates
	}
	scope := p.boundScope(pkList, unsafe.Pointer(state))
	if scope != in.scope {
		in.scope, in.rows = scope, p.tree.Sub(scope, gfx.Index(0))
	}
	area := p.listArea(c, state, n, func(i int, box Box) {
		in.declareRow(c, state, i, box, row)
	})
	area.Draw(in.paint)
	p.listStates = append(p.listStates, state)
	if p.listRoute == nil {
		p.keyRoute, p.listRoute = (*Page).routeKey, (*Page).listKey
	}
	if p.typeRoute == nil {
		p.typeRoute = (*Page).typeKey
	}
	return area
}

// listArea declares the scroll area a List (or a Tree) shows: rows [first, last) laid
// out at their offsets, with spacers for what is above and below them. row fills row
// i's element.
func (p *Page) listArea(c C, state *ListState, n int, row func(i int, box Box)) Box {
	at, _ := p.name(pkContainer)
	area := c.f.ScrollArea(p.id(at))
	p.setNode(area.i, at)
	in := &state.list
	in.n = n
	state.resize(n)
	scroll := c.f.exts[c.f.nodes[area.i].ext].scrollState
	in.scroll = scroll
	state.applyScrollRequest(c.f)
	viewport := scroll.Viewport
	if viewport <= 0 {
		viewport = c.f.bounds.Height
	}
	first := max(state.rowAt(scroll.Offset)-listOverscan, 0)
	if in.n == 0 {
		first = 0
	}
	bottom := scroll.Offset + viewport
	last := first
	for i := first; i < in.n; i++ {
		last = i + 1
		if state.top(last) >= bottom && last-first > listOverscan && last > state.rowAt(bottom)+listOverscan {
			break
		}
	}
	in.first, in.last, in.laidOut = first, last-1, true
	start := len(p.listRows)
	area.Children(func() {
		if top := state.top(first); top > 0 {
			c.f.Slot().Height(top)
		}
		for i := first; i < last; i++ {
			box := Column(c.KeyInt(state.key(i))).Shrink(0)
			row(i, box)
			p.listRows = append(p.listRows, box.i)
		}
		if rest := state.top(in.n) - state.top(last); rest > 0 {
			c.f.Slot().Height(rest)
		}
	})
	p.lists = append(p.lists, listDecl{state: state, first: first, rows: int32(start), until: int32(len(p.listRows))})
	return area
}

// tabStop is the row Tab reaches: the one the keys are on when it shows, else the
// first one shown.
func (in *listInternals) tabStop() int {
	if cur := int(in.selected) - 1; cur >= in.first && cur <= in.last {
		return cur
	}
	return in.first
}

// draw marks the list as the listbox its rows are options of, named by the area's
// label.
func (in *listInternals) draw(d *DrawContext) {
	d.Canvas.Container(d.Rect, gfx.RoleListBox, d.Label)
}

// declareRow declares row i of a List: its whole rectangle is a pointer target and a
// stop of the keyboard, and a click delivered to it chooses, extends or toggles the
// choice before the row shows it.
func (in *listInternals) declareRow(c C, s *ListState, i int, box Box, row func(ListRow)) {
	key := s.key(i)
	id := in.rows.At(key)
	label := ""
	if s.Label != nil {
		label = s.Label(i)
	}
	if canvas := c.f.canvas; canvas != nil && !id.IsZero() && canvas.Clicked(id) {
		info, _ := canvas.ClickInfo()
		s.clickRow(c.page, i, info)
	}
	options := gfx.HitOptions{Role: gfx.RoleOption, Selected: s.Chosen(i)}
	if in.n > 0 {
		options.PosInSet, options.SetSize = int32(i+1), int32(in.n)
	}
	options.SkipTab = i != in.tabStop()
	box.HitWith(id, label, options)
	box.Children(func() {
		row(ListRow{Index: i, Key: key, ID: id, C: c, Element: box, state: s})
	})
	in.declared = append(in.declared, listDeclRow{index: i, key: key, id: id})
}

// clickRow chooses row as a click with info does.
func (s *ListState) clickRow(p *Page, row int, info gfx.ClickInfo) {
	epoch := p.epoch
	if info.Clicks >= 2 {
		s.submit(row, epoch)
	}
	switch {
	case s.Selection == nil:
		s.chooseRow(row, epoch)
	case info.Modifiers&gfx.ModifierShift != 0:
		s.extendRow(row, info.Modifiers&keymap.Primary != 0, epoch)
	case info.Modifiers&keymap.Primary != 0:
		s.toggleRow(row, epoch)
	default:
		s.chooseRow(row, epoch)
	}
}

// listMove is how a key moves the choice: choosing the row it reaches, extending
// from the anchor with Shift, or moving the keys' row without choosing with Primary
// in a list that chooses several.
type listMove uint8

const (
	listChoose listMove = iota + 1
	listExtend
	listCursor
)

// moveOf says how event moves the choice, if it is a key that does.
func moveOf(event gfx.KeyEvent, multi bool) (listMove, bool) {
	switch event.Modifiers {
	case 0:
		return listChoose, true
	case gfx.ModifierShift:
		return listExtend, true
	case keymap.Primary:
		if multi {
			return listCursor, true
		}
	}
	return 0, false
}

// listKey moves through a List from the row that has the focus: the arrows and
// Home, End, Page Up and Down choose another row (Shift extends, Primary moves
// without choosing), Enter submits the row, and in a list that chooses several
// Space is kept from activating the row, which would replace the choice; Control
// with Space on Linux and Windows toggles it.
func (p *Page) listKey(event gfx.KeyEvent) (gfx.EventResult, bool) {
	if event.Release {
		return gfx.EventResult{}, false
	}
	for _, s := range p.listStates {
		if _, inside := event.Target.In(s.list.rows); !inside {
			continue
		}
		row := s.indexOf(event.Target)
		if row < 0 {
			continue
		}
		n := s.list.n
		if event.Key == gfx.KeyEnter && event.Modifiers == 0 {
			s.submit(row, p.epoch+1)
			return gfx.EventResult{Handled: true, Repaint: true}, true
		}
		if event.Key == gfx.KeySpace {
			if s.Selection == nil {
				return gfx.EventResult{}, false // the runtime activates the row
			}
			if runtime.GOOS != "darwin" && event.Modifiers == gfx.ModifierControl {
				s.toggleRow(row, p.epoch+1)
				return listFocus(s, row), true
			}
			return gfx.EventResult{Handled: true}, true // not the row's activation
		}
		if n == 0 {
			return gfx.EventResult{}, false
		}
		how, ok := moveOf(event, s.Selection != nil)
		if !ok {
			return gfx.EventResult{}, false
		}
		target := row
		switch event.Key {
		case gfx.KeyUp:
			target--
		case gfx.KeyDown:
			target++
		case gfx.KeyHome:
			target = 0
		case gfx.KeyEnd:
			target = n - 1
		case gfx.KeyPageUp:
			target -= max(s.list.last-s.list.first+1, 1)
		case gfx.KeyPageDown:
			target += max(s.list.last-s.list.first+1, 1)
		default:
			return gfx.EventResult{}, false
		}
		target = min(max(target, 0), n-1)
		switch how {
		case listChoose:
			s.chooseRow(target, p.epoch+1)
		case listExtend:
			s.extendRow(target, false, p.epoch+1)
		case listCursor:
			s.leadRow(target, p.epoch+1)
		}
		s.revealRow(target)
		return listFocus(s, target), true
	}
	return gfx.EventResult{}, false
}

// listFocus is the result that moves the keyboard focus to row, visibly.
func listFocus(s *ListState, row int) gfx.EventResult {
	return gfx.EventResult{Handled: true, Repaint: true, Focus: s.list.rows.At(s.key(row)), FocusVisible: true}
}

// typeKey is the OnShortcut handler for printable keys no field is typing in: an
// open Menu's typeahead, then a Tree's, then a List's and its Primary+A.
func (p *Page) typeKey(event gfx.KeyEvent) (gfx.EventResult, bool) {
	if result, ok := p.treeType(event); ok {
		return result, true
	}
	return p.listType(event)
}

// listType moves to the row whose name begins with the letters typed, from the row
// the keys are on, choosing it; and chooses every row on Primary+A in a list that
// chooses several.
func (p *Page) listType(event gfx.KeyEvent) (gfx.EventResult, bool) {
	text := event.Text
	if text == "" && event.Key == gfx.KeySpace {
		text = " "
	}
	if text == "" {
		return gfx.EventResult{}, false
	}
	for _, s := range p.listStates {
		if _, inside := event.Target.In(s.list.rows); !inside || s.list.f == nil || s.list.f.canvas == nil {
			continue
		}
		epoch := p.epoch + 1
		if s.Selection != nil && event.Modifiers == keymap.Primary && strings.EqualFold(text, "a") {
			s.selectAll(epoch)
			return gfx.EventResult{Handled: true, Repaint: true}, true
		}
		if s.Label == nil || event.Modifiers&^gfx.ModifierShift != 0 {
			return gfx.EventResult{}, false
		}
		rows := components.MenuTypeaheadRows{
			N: s.list.n, ID: func(i int) gfx.ID { return s.list.rows.At(s.key(i)) }, Label: s.Label,
		}
		now := clockEpoch.Add(s.list.f.canvas.Now())
		typing := text == " " && s.list.typeahead.Typing(now)
		if id, ok := s.list.typeahead.TypeRows(text, now, rows, event.Target); ok {
			if row := s.indexOf(id); row >= 0 {
				s.chooseRow(row, epoch)
				s.revealRow(row)
			}
			return gfx.EventResult{Handled: true, Repaint: true, Focus: id, FocusVisible: true}, true
		}
		if typing {
			return gfx.EventResult{Handled: true}, true
		}
		return gfx.EventResult{}, false
	}
	return gfx.EventResult{}, false
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
