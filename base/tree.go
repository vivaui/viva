package base

import (
	"time"
	"unsafe"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/style"
)

// TreeNode is what a Tree shows of one of its visible rows: how deep it is, whether
// it has children and shows them, whether it is the selected one, and its height.
type TreeNode struct {
	Depth              int
	Branch, Expanded   bool
	Selected, Disabled bool
	// Height is this row's height; zero uses TreeState.RowHeight, or the
	// theme's control height when that is zero too. It is read with the
	// description of the row, so a tree of compact and full rows needs no
	// per-row state outside Node.
	Height float32
}

// TreeState is a Tree's: the visible rows of the application's tree, flattened in
// order, described by Node; ListState's Key names them by an item's ID so that a
// row keeps its focus as rows open and close above it. RowHeight is the default
// height of a row and Indent the step of a level (a control tall and 16 when
// zero); TreeNode.Height overrides the default for its row. Keep one in the model.
//
// The tree's structure is the application's: a press on a branch's chevron, or
// Left and Right on it, is Toggled, and the application opens or closes it and
// gives the Tree the rows that then show; a click or Enter on a row is Chosen.
//
// With CanDrop set, a row can be dragged onto another, before it, into it or after
// it: CanDrop says which of those the application takes, and Dropped reports the
// one let go on. Typing the start of a row's name moves to it.
type TreeState struct {
	ListState
	Node      func(i int) TreeNode
	RowHeight float32
	Indent    float32
	// DisplayDepth maps a row's Depth to the level the tree indents it by, when
	// the application compresses deep rows to keep their names in view. Depth
	// stays the tree's structure, which keyboard movement, typeahead and drops
	// read, so a compressed indent never makes a child look like a sibling.
	// Nil indents by Depth itself.
	DisplayDepth func(depth int) int
	// CanDrop says whether row from may be dropped at where of row to: a group not
	// into itself or what is inside it, a row not before a sibling of another kind.
	// It is asked as the pointer moves; nil makes the rows not draggable.
	CanDrop func(from, to int, where TreeDrop) bool

	toggled, chosen int
	dropped         TreeMove
	drag            treeDrag
	typeahead       MenuTypeahead
	names           []MenuTypeaheadItem // the rows as typeahead matches them
	nameOf          func(int) string    // label, until the next paint: the keys between read it
	f               *Frame              // the frame the tree was last declared in
	scope           gfx.Scope           // the ids of the rows (scope.At(0)) and chevrons (At(1))
	rows, chevrons  gfx.Scope
	Scroll          *ScrollState // the List's, between paints: its area's state lives while it is declared
	n               int
	declared        []TreeRow // the rows of the last paint
	declare         func(i int, Box Box)
	paint           func(*DrawContext)
	c               C
	rowLabel        func(int) string
	Content         func(int)
	// customKeys leaves the arrow, Home, End and Page keys to the application
	// (TreeKeys(false)); it is reset by each declaration.
	customKeys bool
}

type TreeRow struct {
	index int
	key   int
	Node  int32  // the row element
	gen   uint32 // the paint it was declared in
	item  style.TreeItem
}

// TreeDrop is where a dragged row goes, relative to the row it is let go on.
type TreeDrop uint8

const (
	TreeDropBefore TreeDrop = iota // above the row, as its sibling
	TreeDropInside                 // into the row, as its child
	TreeDropAfter                  // below the row, as its sibling
)

// TreeMove is a drop: row From let go at Where of row To, indices of the visible
// rows as Node has them.
type TreeMove struct {
	From, To int
	Where    TreeDrop
}

// treeDrag is a press on a row, and once the pointer has moved far enough with it,
// a drag: where it is and where it would drop.
type treeDrag struct {
	pressed, active bool
	from            int // the row's key, which survives rows opening above it
	start           gfx.Point
	at              gfx.Point
	target, Next    TreeMove // the drop the last paint found, and the one this paint finds
	valid, nextOK   bool
	Rect            Rect          // the row the drop is at, as painted
	View            Rect          // the tree's visible area, as of the last paint
	scrolled        time.Duration // the last step of the scroll at an edge
}

// dragStartDistance is how far a pressed row moves before it is dragged, and not
// clicked; dragEdgeSpeed is how fast a drag resting at the top or bottom edge of a
// tree or list scrolls it.
const (
	dragStartDistance = 4
	dragEdgeSpeed     = 600 // pixels a second
)

// Dropped reports the drop this paint delivers: a row dragged onto another and let
// go there, where CanDrop took it. The application moves it.
func (s *TreeState) Dropped() (TreeMove, bool) { return s.dropped, s.dropped.From >= 0 }

// Dragging reports whether a row is being dragged.
func (s *TreeState) Dragging() bool { return s.drag.active }

// DragSource reports the visible row a drag is carrying and where the pointer
// is, while a drag runs. A row no longer shown has no source.
func (s *TreeState) DragSource() (row int, at gfx.Point, ok bool) {
	if !s.drag.active {
		return 0, gfx.Point{}, false
	}
	row = s.indexOf(s.drag.from)
	if row < 0 {
		return 0, gfx.Point{}, false
	}
	return row, s.drag.at, true
}

// Toggled reports which row's chevron this paint delivers a press on, or Left or
// Right on a branch: the application opens or closes it.
func (s *TreeState) Toggled() (int, bool) { return s.toggled, s.toggled >= 0 }

// Chosen reports which row this paint delivers a click or Enter on.
func (s *TreeState) Chosen() (int, bool) { return s.chosen, s.chosen >= 0 }

// Tree is a scrolling tree of n visible rows, of which it declares only those on
// screen, as List does: a chevron opens and closes a branch, the arrows move
// between rows (Left to the parent, Right to the first child, opening or closing a
// branch on the way), and Enter or a click chooses one. label names row i, for
// assistive technology and as its content when content is nil; content declares
// what is in the row after its chevron (an icon, the name, a count). Keep label and
// content in the model, as Node is: a method value made in each paint allocates.
//
//	ui.Tree(c, &m.files, len(m.visible), m.name, nil).Grow(1)
//	if i, ok := m.files.Toggled(); ok { m.toggle(i) }
//	if i, ok := m.files.Chosen(); ok { m.open(i) }
//
// Not inside a Memo.
func Tree(c C, state *TreeState, n int, Label func(i int) string, Content func(i int)) Box {
	p := c.page
	p.ForbidInMemo("Tree")
	if state.Node == nil {
		panic("ui: Tree needs TreeState.Node, which describes each visible row")
	}
	if state.declare == nil {
		state.declare, state.paint = state.row, state.draw // made once: method values made in each paint allocate
	}
	scope := p.BoundScope(pkTree, unsafe.Pointer(state))
	if scope != state.scope {
		state.scope, state.rows, state.chevrons = scope, p.tree.Sub(scope, gfx.Index(0)), p.tree.Sub(scope, gfx.Index(1))
	}
	state.toggled, state.chosen = -1, -1
	state.dropped = TreeMove{From: -1}
	state.customKeys = false // TreeKeys(false) sets it again after this declaration
	state.c, state.rowLabel, state.Content, state.n = c, Label, Content, n
	state.nameOf, state.f = Label, c.f
	if state.CanDrop != nil {
		state.dragPointer(c.f.Surface)
	} else {
		state.drag = treeDrag{}
	}
	state.declared = state.declared[:0]
	// The ListState a Tree builds on answers for the keyboard focus the way a
	// List does: its rows sit in the tree's own row scope.
	state.ListState.list.f = c.f
	state.ListState.list.rows = state.rows
	area := p.ListArea(c, &state.ListState, n, false, state.declare)
	state.Scroll = c.f.Exts[c.f.Nodes[area.i].Ext].scrollState
	state.c, state.rowLabel, state.Content = nil, nil, nil // the model's functions: not kept past the paint
	p.trees = append(p.trees, state)
	if p.treeRoute == nil {
		p.keyRoute, p.treeRoute, p.typeRoute = (*Page).RouteKey, (*Page).TreeKey, (*Page).TypeKey
	}
	return area
}

// TreeKeys controls whether the Tree handles the arrow, Home, End and Page keys
// itself, moving the keyboard between its rows and opening or closing a branch on
// Left and Right. false leaves those keys to the application, for a tree whose
// navigation is its own, such as NavigateTree over the application's rows; the
// application then moves the focus itself, typically to TreeState.RowID. The
// default is true. Chain it on the Box Tree returns.
//
//	ui.Tree(c, &m.tree, rows, m.label, nil).TreeKeys(false)
func (b Box) TreeKeys(on bool) Box {
	b.N()
	found := false
	if p := b.f.page; p != nil {
		for _, s := range p.trees {
			if s.f != b.f || s.list.area != b.i {
				continue
			}
			found = true
			s.customKeys = !on
			if !on {
				// The List a Tree is built on would otherwise take the arrows
				// after the Tree leaves them: its keys are the application's too.
				p.dropListRoute(&s.ListState)
			}
			break
		}
	}
	if !found {
		panic("ui: TreeKeys is for the Tree it is chained on")
	}
	return b
}

// dropListRoute takes state out of the page's list keys, so a Tree whose
// application owns its navigation does not also have the List it is built on
// move its focus.
func (p *Page) dropListRoute(state *ListState) {
	for i, candidate := range p.listStates {
		if candidate == state {
			p.listStates = append(p.listStates[:i], p.listStates[i+1:]...)
			return
		}
	}
}

// appKeys reports whether the application owns the tree's navigation keys.
func (s *TreeState) appKeys() bool { return s.customKeys }

// RowID is the identity of visible row i as the last paint named it, by Key when
// one is set: the target to give the focus (Canvas.RequestFocus), to look up in a
// pointer event or to find among the hits. It is the zero ID before the tree has
// been declared, and for a row the tree has not declared.
func (s *TreeState) RowID(i int) gfx.ID {
	if i < 0 || i >= s.list.n {
		return gfx.ID{}
	}
	return s.rows.At(s.key(i))
}

// DisclosureID is the identity of the chevron of branch row i, which opens and
// closes it: the zero ID before the tree has been declared, and for a row the
// tree has not declared. A row that is not a branch has no chevron, and no hit is
// declared at this ID.
func (s *TreeState) DisclosureID(i int) gfx.ID {
	if i < 0 || i >= s.list.n {
		return gfx.ID{}
	}
	return s.chevrons.At(s.key(i))
}

func (s *TreeState) rowHeight(Palette *Theme) float32 {
	if s.RowHeight > 0 {
		return s.RowHeight
	}
	return Palette.Metrics.ControlHeight
}

func (s *TreeState) HeightOf(Node TreeNode, Palette *Theme) float32 {
	if Node.Height > 0 {
		return Node.Height
	}
	return s.rowHeight(Palette)
}

func (s *TreeState) IndentValue(Palette *Theme) float32 {
	if s.Indent > 0 {
		return s.Indent
	}
	return 16 * Palette.Metrics.ControlHeight / 32
}

// dragPointer follows a press on a row, which turns into a drag once the pointer
// moves far enough with it, and drops it when it is let go. It runs before the rows
// are declared, from what the last paint found under the pointer.
func (s *TreeState) dragPointer(Surface *gfx.Canvas) {
	drag := &s.drag
	if drag.active {
		drag.target, drag.valid = drag.Next, drag.nextOK // what the rows under the pointer found
		drag.nextOK = false
		s.edgeScroll(Surface)
	}
	if Surface == nil {
		return
	}
	at, event, ok := Surface.PointerIn(s.rows)
	if !ok {
		return
	}
	point := gfx.Point{X: event.X, Y: event.Y}
	switch event.Kind {
	case gfx.PointerDown:
		*drag = treeDrag{}
		if event.Button <= gfx.PointerButtonPrimary {
			// Not consumed: a press that does not move is still the row's click.
			drag.pressed, drag.from, drag.start, drag.at = true, at, point, point
		}
	case gfx.PointerMove:
		if !drag.pressed {
			return
		}
		dx, dy := point.X-drag.start.X, point.Y-drag.start.Y
		if !drag.active && dx*dx+dy*dy < dragStartDistance*dragStartDistance {
			return
		}
		drag.active, drag.at = true, point
		Surface.Consume()
	case gfx.PointerUp:
		active, move, valid := drag.active, drag.target, drag.valid
		*drag = treeDrag{}
		if !active {
			return // a click
		}
		// A drag is not also a click on the row it ends on: its moves were consumed,
		// which already says so, and so does its release.
		Surface.Consume()
		if valid {
			s.dropped = move
		}
	}
}

// dragOver finds, as row is painted, whether the pointer is over it and where on
// it the dragged row would drop, and paints that: a line between rows, or a ring
// around the row it would go into.
func (s *TreeState) DragOver(row *TreeRow, d *DrawContext) {
	drag := &s.drag
	r := d.Rect
	// The tree's visible area, from a row painted in it: the rows start where the
	// list does, less how far it is scrolled.
	drag.View = Rect{X: r.X, Y: r.Y - (s.top(row.index) - s.scrollOffset()), Width: r.Width}
	if s.Scroll != nil {
		drag.View.Height = s.Scroll.Viewport
	}
	from := s.indexOf(drag.from)
	if from >= 0 && drag.at.Y >= r.Y && drag.at.Y < r.Y+r.Height && from != row.index {
		if where, ok := s.dropAt(from, row.index, (drag.at.Y-r.Y)/r.Height); ok {
			drag.Next, drag.nextOK, drag.Rect = TreeMove{From: from, To: row.index, Where: where}, true, r
		}
	}
	if !drag.valid || drag.target.To != row.index {
		return
	}
	theme := d.Theme
	switch drag.target.Where {
	case TreeDropInside:
		// A faint wash and a half-strength edge mark the destination, as DBX
		// marks the folder it is dropped into; the dragged row's focus ring is
		// gone, so this is the only strong outline.
		tint := theme.Primary
		tint.Alpha *= 0.05
		edge := theme.Primary
		edge.Alpha *= 0.5
		d.Canvas.Rounded(r, theme.Metrics.RadiusItem(), tint)
		d.Canvas.RoundedBorder(r, theme.Metrics.RadiusItem(), 2, edge, gfx.Color{})
	case TreeDropBefore, TreeDropAfter:
		y := r.Y - 1
		if drag.target.Where == TreeDropAfter {
			y = r.Y + r.Height - 1
		}
		d.Canvas.Fill(Rect{X: r.X + 8, Y: y, Width: max(r.Width-16, 0), Height: 2}, theme.Primary)
	}
}

// dropAt is where on row to the pointer, at fraction y down it, drops row from: its
// top quarter before it, its bottom quarter after it, its middle into it, each
// falling back to the next one CanDrop takes.
func (s *TreeState) dropAt(from, to int, y float32) (TreeDrop, bool) {
	order := [3]TreeDrop{TreeDropInside, TreeDropBefore, TreeDropAfter}
	switch {
	case y < 0.25:
		order = [3]TreeDrop{TreeDropBefore, TreeDropInside, TreeDropAfter}
	case y >= 0.75:
		order = [3]TreeDrop{TreeDropAfter, TreeDropInside, TreeDropBefore}
	case y >= 0.5:
		order = [3]TreeDrop{TreeDropInside, TreeDropAfter, TreeDropBefore}
	}
	for _, where := range order {
		if s.CanDrop(from, to, where) {
			return where, true
		}
	}
	return 0, false
}

// edgeScroll scrolls a tree whose drag rests near its top or bottom edge.
func (s *TreeState) edgeScroll(Surface *gfx.Canvas) {
	drag := &s.drag
	if drag.View.Height <= 0 {
		return
	}
	band := min(s.rowHeight(&s.c.f.Palette), drag.View.Height/4)
	dragEdgeScroll(Surface, s.Scroll, drag.View, drag.at, band, &drag.scrolled)
}

// dragEdgeScroll scrolls scroll while a drag at the point at rests within band of the
// top or bottom edge of view, in steps timed by the frame clock: since is when the
// last step was taken, and a frame is asked for only while the drag stays there.
func dragEdgeScroll(Surface *gfx.Canvas, Scroll *ScrollState, View Rect, at gfx.Point, Band float32, since *time.Duration) {
	if Surface == nil || Scroll == nil || View.Height <= 0 {
		return
	}
	direction := float32(0)
	switch {
	case at.Y < View.Y+Band && Scroll.Offset > 0:
		direction = -1
	case at.Y > View.Y+View.Height-Band && Scroll.Offset+Scroll.Viewport < Scroll.Content:
		direction = 1
	}
	now := Surface.Now()
	if direction == 0 {
		*since = 0
		return
	}
	if *since > 0 {
		step := min(now-*since, 100*time.Millisecond) // a late frame does not jump
		Scroll.ScrollTo(Scroll.Offset + direction*dragEdgeSpeed*float32(step.Seconds()))
	}
	*since = now
	Surface.RequestFrameIn(16 * time.Millisecond)
}

// scrollOffset is how far the tree is scrolled.
func (s *TreeState) scrollOffset() float32 {
	if s.Scroll == nil {
		return 0
	}
	return s.Scroll.Offset
}

// indexOf is the visible row named key in the last paint, or -1.
func (s *TreeState) indexOf(key int) int {
	if s.Key == nil {
		if key < s.n {
			return key
		}
		return -1
	}
	for i := range s.declared {
		if s.declared[i].key == key {
			return s.declared[i].index
		}
	}
	for i := 0; i < s.n; i++ { // scrolled out of sight
		if s.Key(i) == key {
			return i
		}
	}
	return -1
}

// treeKey moves through a Tree from the focused row with the arrows: Up and Down to
// the row above and below, Home and End, Page Up and Down by a screen, Right to open
// a branch or go to its first child, Left to close it or go to the parent.
func (p *Page) TreeKey(event gfx.KeyEvent) (gfx.EventResult, bool) {
	if event.Release {
		return gfx.EventResult{}, false
	}
	if event.Key == gfx.KeyEscape {
		for _, s := range p.trees {
			if s.drag.active { // Escape lets go of a drag without dropping it
				s.drag = treeDrag{}
				return gfx.EventResult{Handled: true, Repaint: true}, true
			}
		}
	}
	if event.Modifiers&^gfx.ModifierShift == 0 {
		if result, ok := p.TreeType(event); ok {
			return result, true
		}
	}
	if event.Modifiers != 0 {
		return gfx.EventResult{}, false
	}
	for _, s := range p.trees {
		var current *TreeRow
		for i := range s.declared {
			if s.declared[i].item.ID == event.Target {
				current = &s.declared[i]
			}
		}
		if current == nil {
			continue
		}
		if s.appKeys() {
			// The application owns the arrows, Home, End and Page: its own key
			// handler receives them, as TreeKeys(false) asked.
			return gfx.EventResult{}, false
		}
		i, n := current.index, s.n
		node := s.Node(i)
		page := max(len(s.declared)-2, 1)
		target := i
		switch event.Key {
		case gfx.KeyUp:
			target = max(i-1, 0)
		case gfx.KeyDown:
			target = min(i+1, n-1)
		case gfx.KeyHome:
			target = 0
		case gfx.KeyEnd:
			target = n - 1
		case gfx.KeyPageUp:
			target = max(i-page, 0)
		case gfx.KeyPageDown:
			target = min(i+page, n-1)
		case gfx.KeyRight:
			switch {
			case node.Branch && !node.Expanded:
				return gfx.EventResult{Handled: true, Activate: current.item.DisclosureID}, true
			case node.Branch && i+1 < n && s.Node(i+1).Depth > node.Depth:
				target = i + 1
			}
		case gfx.KeyLeft:
			if node.Branch && node.Expanded {
				return gfx.EventResult{Handled: true, Activate: current.item.DisclosureID}, true
			}
			for j := i - 1; j >= 0; j-- {
				if s.Node(j).Depth < node.Depth {
					target = j
					break
				}
			}
		default:
			return gfx.EventResult{}, false
		}
		s.reveal(target)
		return gfx.EventResult{Handled: true, Repaint: true, Focus: s.rows.At(s.key(target)), FocusVisible: true}, true
	}
	return gfx.EventResult{}, false
}

// reveal scrolls the tree so that row i shows; the repaint declares it, and the
// focus goes to it there.
func (s *TreeState) reveal(i int) {
	scroll := s.Scroll
	if scroll == nil || scroll.Viewport <= 0 {
		return
	}
	top, bottom := s.top(i), s.top(i+1)
	switch {
	case top < scroll.Offset:
		scroll.ScrollTo(top)
	case bottom > scroll.Offset+scroll.Viewport:
		scroll.ScrollTo(bottom - scroll.Viewport)
	}
}

// treeType moves to the next row whose name begins with the letters typed, from
// the focused row of a Tree, as a file list does.
func (p *Page) TreeType(event gfx.KeyEvent) (gfx.EventResult, bool) {
	if event.Modifiers&^gfx.ModifierShift != 0 {
		return gfx.EventResult{}, false
	}
	text := event.Text
	if text == "" && event.Key == gfx.KeySpace {
		text = " "
	}
	if text == "" {
		return gfx.EventResult{}, false
	}
	for _, s := range p.trees {
		if _, inside := event.Target.In(s.rows); !inside || s.nameOf == nil || s.f == nil || s.f.Surface == nil {
			continue
		}
		s.names = s.names[:0]
		for i := 0; i < s.n; i++ {
			s.names = append(s.names, MenuTypeaheadItem{ID: s.rows.At(s.key(i)), Label: s.nameOf(i), Disabled: s.Node(i).Disabled})
		}
		now := ClockEpoch.Add(s.f.Surface.Now())
		typing := text == " " && s.typeahead.Typing(now)
		if id, ok := s.typeahead.TypeItems(text, now, s.names, event.Target); ok {
			for i := range s.names {
				if s.names[i].ID == id {
					s.reveal(i)
				}
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
