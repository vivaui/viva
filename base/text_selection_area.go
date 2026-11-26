package base

import (
	"math"
	"strings"
	"time"
	"unicode/utf8"
	"unsafe"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
)

// SetPrimarySelection makes text the primary selection of an X11 desktop, which
// selecting text fills and a middle click pastes. The styled layer installs the
// system's; a host that installs nothing has none.
var SetPrimarySelection func(string) error

// TextSelectionState is the selection of one TextSelectionArea: what the user has
// selected in it and where the last paint drew its texts. Keep it in the model
// next to what the area shows; its zero value selects nothing.
type TextSelectionState struct {
	// Selection is the selected run, from the press (Anchor) to where the pointer
	// or the keyboard took it (Focus). Surface numbers the area's texts in reading
	// order, as the last paint drew them; Offset is a byte offset in that text.
	Selection TextSelection

	id       gfx.ID
	handlers Handlers
	bound    *TextSelectionState // the state handlers were made for, so a copy of a state makes its own
	canvas   *gfx.Canvas

	// texts and lines are what the last paint drew, which the events between two
	// paints are read against.
	texts []selectableText
	lines []selectableLine
	// parent is the element the last text collected sits in, and inRow whether
	// that element lays its children out in a row, which joins two texts with a tab.
	parent int32

	// pressed says a primary press on a text is held; press is where it went
	// down, moved whether it has left that spot, and pointer where it is now.
	pressed, moved bool
	press, pointer gfx.Point
	unit           selectUnit
	// span is the word or line the press picked, which a drag by words or lines
	// keeps whole. anchor is where the last press began, which Shift with a press
	// extends from even when that press selected nothing.
	span      [2]TextPoint
	anchor    TextPoint
	hasAnchor bool

	focused  bool // the area had the keyboard focus in the last paint, or lost it to nothing
	page     *Page
	menuBody func(*MenuItems)
	menuOpen bool
	menuWas  bool // the menu was open in the last paint: the keyboard is on its way back
	menuAt   gfx.Point
	primary  string // what the area last made the primary selection

	scroll     *ScrollState // the area the area scrolls in, which a drag past its edge scrolls
	scrollNode int32        // its element, whose rectangle the last paint gave it
	scrollRect Rect
	stepAt     time.Duration
}

// selectUnit is what a press selects and a drag extends by.
type selectUnit uint8

const (
	unitChar selectUnit = iota
	unitWord
	unitLine
)

// selectableText is one text of an area as a paint drew it.
type selectableText struct {
	text  string
	style gfx.TextStyle
	align gfx.Alignment
	// first and count are its lines in TextSelectionState.lines.
	first, count int32
	bounds       Rect // where its element put it
	// tab joins it to the text before it with a tab instead of a line break: the two
	// sit side by side in one row, as a label and its value.
	tab bool
}

// selectableLine is one line of a text: the source bytes from to to, drawn as
// shown in bounds. A line cut short by an ellipsis shows fewer bytes than it
// covers: the cut-off rest belongs to it.
type selectableLine struct {
	from, to int
	shown    string
	visible  int // how many bytes of shown are the text's own, before an ellipsis
	bounds   Rect
}

// TextSelectionArea declares a column whose texts the user can select with the
// pointer and copy, as the labels of a dialog, an error or a detail page are in a
// desktop application. Text, Paragraph and Wrap inside it take part, across their
// boundaries: a drag may begin in one and end in another. Text that belongs to a
// control does not (a button's, a link's, a field's, a label given LabelFor:
// anything inside an element that takes presses; a group region given HitWith to
// name its content keeps it in), nor what Unselectable takes out; a field and a
// view with a selection of its own keep theirs.
//
// A press on a text begins a selection, a double press selects a word and a triple
// press a line; Shift with a press extends it. A press in the area's blank part
// begins at the nearest text, so a click there drops the selection. Primary+C
// copies, Primary+A selects all the area holds, Shift with the arrow keys extends
// the selection and Escape drops it; a secondary press opens the page's Copy and
// Select All menu (see PageConfig.EditMenu). The selection is dropped when another
// control takes the keyboard. A drag past the edge of
// the scrolling area the area is in scrolls it.
//
// The area takes no Tab stop. Its texts are copied as their source: a text cut
// short by an ellipsis copies whole. Texts side by side in a row are joined with a
// tab, others with a line break. A text inside a Memo is drawn but not selected.
//
//	ui.TextSelectionArea(c, &m.details).Gap(6).Children(func() {
//		ui.Text(c, m.fingerprint)
//	})
func TextSelectionArea(c C, state *TextSelectionState) Box {
	p := c.page
	p.ForbidInMemo("TextSelectionArea")
	if state == nil {
		panic("ui: TextSelectionArea needs its state")
	}
	id := p.boundID(pkTextSelection, unsafe.Pointer(state))
	state.id = id
	if state.bound != state {
		state.bound = state
		state.handlers = Handlers{Role: gfx.RoleGroup, Pointer: state.onPointer, Key: state.onKey}
		state.menuBody = state.menu
	}
	state.handlers.parts = p.tree.Sub(id.Scope(), id.Name())
	p.controls = append(p.controls, controlEntry{ID: id, handlers: &state.handlers})
	if p.pointerRoute == nil {
		p.keyRoute, p.pointerRoute = (*Page).RouteKey, (*Page).routePointer
	}
	canvas := c.f.Surface
	state.canvas = canvas
	if canvas != nil {
		state.followFocus(canvas.Focused())
	}
	state.menuWas = state.menuOpen

	state.scroll, state.scrollNode = nil, -1
	for i := c.f.Parent; i >= 0; i = c.f.Nodes[i].Parent {
		if n := &c.f.Nodes[i]; n.Ext != 0 && c.f.Exts[n.Ext].scrollState != nil {
			state.scroll, state.scrollNode = c.f.Exts[n.Ext].scrollState, i
			break
		}
	}
	if canvas != nil {
		state.dragScroll(canvas)
	}

	b := p.container(c.f.Column())
	b.HitWith(id, "", gfx.HitOptions{Role: gfx.RoleGroup, SkipTab: true})
	c.f.textAreas = append(c.f.textAreas, state)
	c.f.Extra(b.i).textArea = int32(len(c.f.textAreas))

	if p.shortcut(c, Primary, "c", id, 0, 0) && state.Has() {
		state.copy()
	}
	if p.shortcut(c, Primary, "a", id, 0, 0) {
		state.SelectAll()
	}
	state.page = p
	ContextMenuAt(c, b, &state.menuOpen, state.menuAt, state.menuBody)
	return b
}

// actsOnPress reports whether an element takes presses, which keeps the text inside
// it out of the TextSelectionArea it is in: a button, a link, a label of a control.
// A group region, which declares a hit to name what it holds for assistive
// technology and does nothing when pressed, leaves its text in.
func actsOnPress(n *Node, extra *Extra) bool {
	return n.Flags&flagHasHit != 0 && (n.Flags&flagLabelFor != 0 || extra.hit.Role != gfx.RoleGroup)
}

// Unselectable keeps the element and what it holds out of the TextSelectionArea it
// is in: words the user has no use for copying, such as a decorative caption.
func (b Box) Unselectable() Box {
	b.N()
	b.f.Extra(b.i).unselectable = true
	return b
}

// Has reports whether text is selected.
func (s *TextSelectionState) Has() bool {
	if s.Selection.Empty() {
		return false
	}
	for i := range s.texts {
		if s.selected(i) != "" {
			return true
		}
	}
	return false
}

// Text is the selected text, what Primary+C puts on the clipboard: each text's
// selected part in reading order, the parts of two texts in a row joined with a
// tab and others with a line break.
func (s *TextSelectionState) Text() string {
	if s.Selection.Empty() {
		return ""
	}
	var out strings.Builder
	for i := range s.texts {
		piece := s.selected(i)
		if piece == "" {
			continue
		}
		if out.Len() > 0 {
			if s.texts[i].tab {
				out.WriteByte('\t')
			} else {
				out.WriteByte('\n')
			}
		}
		out.WriteString(piece)
	}
	return out.String()
}

// Clear drops the selection.
func (s *TextSelectionState) Clear() {
	s.Selection.Clear()
	s.pressed, s.hasAnchor = false, false
}

// SelectAll selects every text the area held in its last paint.
func (s *TextSelectionState) SelectAll() {
	s.Selection.SelectAll(len(s.texts), s.length)
	s.pressed = false
	s.publish()
}

func (s *TextSelectionState) length(surface int) int { return len(s.texts[surface].text) }

// selected is the selected part of text i.
func (s *TextSelectionState) selected(i int) string {
	value := s.texts[i].text
	start, end := s.Selection.In(i).rangeIn(value)
	return value[start:end]
}

// followFocus drops the selection once another control has taken the keyboard, as
// a selectable label's selection goes when it stops being the first responder. The
// area's own menu, which holds the keyboard while it is open, does not count, and
// neither does a press on nothing, which leaves the focus nowhere.
func (s *TextSelectionState) followFocus(focused gfx.ID) {
	switch {
	case focused == s.id:
		s.focused = true
	case s.menuOpen, s.menuWas:
	case focused.IsZero():
	default:
		if s.focused && !s.Selection.Empty() {
			s.Clear()
		}
		s.focused = false
	}
}

func (s *TextSelectionState) copy() {
	if value := s.Text(); value != "" && SetClipboardText != nil {
		_ = SetClipboardText(value)
	}
}

// publish makes the selection the primary selection, where the desktop has one.
func (s *TextSelectionState) publish() {
	if SetPrimarySelection == nil {
		return
	}
	value := s.Text()
	if value == "" || value == s.primary {
		return
	}
	if SetPrimarySelection(value) == nil {
		s.primary = value
	}
}

// menu is the body of the area's context menu: the page's words for Copy and
// Select All, which act on the area.
func (s *TextSelectionState) menu(menu *MenuItems) {
	var labels EditMenuLabels
	if s.page != nil && s.page.config.EditMenu != nil {
		labels = *s.page.config.EditMenu
	}
	labels = labels.WithDefaults()
	// The item chosen acts on the area and gives it the keyboard back, as a field's
	// edit menu does.
	if menu.Item(labels.Copy).Keys(Primary, "c").Disabled(!s.Has()).Clicked() {
		s.copy()
		s.page.editFocus = s.id
	}
	menu.Separator()
	if menu.Item(labels.SelectAll).Keys(Primary, "a").Disabled(len(s.texts) == 0).Clicked() {
		s.SelectAll()
		s.page.editFocus = s.id
	}
}

// dragScroll scrolls the area the selection is dragged out of, a step of lines
// every 50 ms that grows with the distance past its edge, as a text view does. It
// asks for the frames of the steps only while a drag holds the pointer outside.
func (s *TextSelectionState) dragScroll(canvas *gfx.Canvas) {
	if !s.pressed || !s.moved || s.scroll == nil {
		return
	}
	top, bottom := s.scrollRect.Y, s.scrollRect.Y+s.scrollRect.Height
	var distance float32
	switch {
	case s.pointer.Y < top:
		distance = s.pointer.Y - top
	case s.pointer.Y > bottom:
		distance = s.pointer.Y - bottom
	default:
		return
	}
	line := s.lineHeight()
	if now := canvas.Now(); now-s.stepAt >= textDragStep {
		s.stepAt = now
		lines := min(1+float32(math.Abs(float64(distance)))/line, 8)
		if distance < 0 {
			lines = -lines
		}
		s.scroll.ScrollTo(min(max(s.scroll.Offset+float32(int(lines))*line, 0), s.scroll.Max()))
	}
	canvas.RequestFrameIn(textDragStep)
}

const textDragStep = 50 * time.Millisecond

// lineHeight is the height of a line of the area's first text, which a drag past
// the edge scrolls by.
func (s *TextSelectionState) lineHeight() float32 {
	for _, line := range s.lines {
		if line.bounds.Height > 0 {
			return line.bounds.Height
		}
	}
	return 16
}

// partOf is the text a press landed on, given the target of the event.
func (s *TextSelectionState) partOf(target gfx.ID) (int, bool) {
	i, ok := target.In(s.handlers.parts)
	return i, ok && i < len(s.texts)
}

func (s *TextSelectionState) onPointer(event gfx.PointerEvent) gfx.EventResult {
	at := gfx.Point{X: event.X, Y: event.Y}
	switch event.Kind {
	case gfx.PointerDown:
		if event.Button == gfx.PointerButtonSecondary {
			s.menuOpen, s.menuAt = true, at
			return gfx.EventResult{Handled: true, Repaint: true, Focus: s.id}
		}
		if event.Button != gfx.PointerButtonPrimary {
			return gfx.EventResult{}
		}
		// A press beside the words, in the area's blank part, begins at the nearest of
		// them, as a drag in a document does; a click there selects nothing.
		var point TextPoint
		if surface, onText := s.partOf(event.Target); onText {
			point = s.pointIn(surface, at)
		} else if nearest, ok := s.pointAt(at); ok {
			point = nearest
		} else {
			had := !s.Selection.Empty()
			s.Clear()
			return gfx.EventResult{Handled: true, Repaint: had, Focus: s.id}
		}
		clicks := max(event.ClickCount, 1)
		if event.Modifiers&gfx.ModifierShift != 0 && clicks == 1 && (s.hasAnchor || !s.Selection.Empty()) {
			s.unit = unitChar
			if s.Selection.Empty() {
				s.Selection.Anchor = s.anchor
			}
			s.Selection.Focus = point
		} else {
			s.unit = unitChar
			switch {
			case clicks >= 3:
				s.unit = unitLine
			case clicks == 2:
				s.unit = unitWord
			}
			s.span[0], s.span[1] = s.unitSpan(point)
			s.Selection.Select(s.span[0], s.span[1])
			s.anchor, s.hasAnchor = point, true
		}
		s.pressed, s.moved, s.press, s.pointer = true, false, at, at
		return gfx.EventResult{Handled: true, Repaint: true, Focus: s.id}
	case gfx.PointerMove:
		if !s.pressed {
			return gfx.EventResult{}
		}
		s.pointer = at
		if !s.moved && math.Abs(float64(at.X-s.press.X)) < 3 && math.Abs(float64(at.Y-s.press.Y)) < 3 {
			return gfx.EventResult{Handled: true}
		}
		s.moved = true
		return gfx.EventResult{Handled: true, Repaint: s.extendTo(at)}
	case gfx.PointerUp:
		if !s.pressed {
			return gfx.EventResult{}
		}
		s.pressed = false
		if s.moved {
			s.extendTo(at)
		}
		if s.Selection.Empty() {
			s.Selection.Clear()
		}
		s.publish()
		return gfx.EventResult{Handled: true, Repaint: true}
	case gfx.PointerExit:
		// The release of a held press was not seen: the system took the press, or
		// the button came up outside the window.
		if s.pressed {
			s.pressed = false
			s.publish()
			return gfx.EventResult{Handled: true, Repaint: true}
		}
	}
	return gfx.EventResult{}
}

// extendTo moves the selection's focus to the text under a point of the window, by
// the unit of the press, and reports whether the selection changed.
func (s *TextSelectionState) extendTo(at gfx.Point) bool {
	point, ok := s.pointAt(at)
	if !ok {
		return false
	}
	was := s.Selection
	if s.unit == unitChar {
		s.Selection.Focus = point
	} else {
		from, to := s.unitSpan(point)
		if from.before(s.span[0]) {
			s.Selection.Anchor, s.Selection.Focus = s.span[1], from
		} else {
			s.Selection.Anchor, s.Selection.Focus = s.span[0], to
		}
	}
	return s.Selection != was
}

// unitSpan is the unit of the press around a point: the point itself for
// characters, the word or the line of its text around it.
func (s *TextSelectionState) unitSpan(point TextPoint) (TextPoint, TextPoint) {
	if s.unit == unitChar || point.Surface >= len(s.texts) {
		return point, point
	}
	value := s.texts[point.Surface].text
	index := utf8.RuneCountInString(value[:min(point.Offset, len(value))])
	var picked text.Range
	if s.unit == unitWord {
		picked = text.WordRange(value, index)
	} else {
		picked = text.LineRange(value, index)
	}
	from, to := byteOfRune(value, picked.Start), byteOfRune(value, picked.End)
	return TextPoint{point.Surface, from}, TextPoint{point.Surface, to}
}

// byteOfRune is the byte offset of rune index i in value, its length past the end.
func byteOfRune(value string, i int) int {
	n := 0
	for offset := range value {
		if n == i {
			return offset
		}
		n++
	}
	return len(value)
}

func (s *TextSelectionState) onKey(event gfx.KeyEvent) gfx.EventResult {
	if event.Release {
		return gfx.EventResult{}
	}
	switch event.Key {
	case gfx.KeyEscape:
		if s.Selection.Empty() {
			return gfx.EventResult{}
		}
		s.Clear()
		return gfx.EventResult{Handled: true, Repaint: true}
	case gfx.KeyLeft, gfx.KeyRight, gfx.KeyUp, gfx.KeyDown:
		if event.Modifiers&^gfx.ModifierShift != 0 || event.Modifiers&gfx.ModifierShift == 0 || s.Selection.Empty() {
			return gfx.EventResult{}
		}
		focus, ok := s.step(s.Selection.Focus, event.Key)
		if !ok || focus == s.Selection.Focus {
			return gfx.EventResult{Handled: true}
		}
		s.Selection.Focus = focus
		s.publish()
		return gfx.EventResult{Handled: true, Repaint: true}
	}
	return gfx.EventResult{}
}

// step is the place one character to either side of point, across the end of a
// text into the next, or one line above or below it, at the same distance across.
func (s *TextSelectionState) step(point TextPoint, key gfx.Key) (TextPoint, bool) {
	if point.Surface < 0 || point.Surface >= len(s.texts) {
		return point, false
	}
	value := s.texts[point.Surface].text
	offset := min(max(point.Offset, 0), len(value))
	switch key {
	case gfx.KeyLeft:
		if offset > 0 {
			return TextPoint{point.Surface, text.PreviousGraphemeBoundary(value, offset)}, true
		}
		if point.Surface > 0 {
			return TextPoint{point.Surface - 1, len(s.texts[point.Surface-1].text)}, true
		}
		return point, true
	case gfx.KeyRight:
		if offset < len(value) {
			return TextPoint{point.Surface, nextGraphemeOf(value, offset)}, true
		}
		if point.Surface+1 < len(s.texts) {
			return TextPoint{point.Surface + 1, 0}, true
		}
		return point, true
	}
	caret, line, ok := s.caretOf(point)
	if !ok {
		return point, false
	}
	y := line.Y - 1
	if key == gfx.KeyDown {
		y = line.Y + line.Height + 1
	}
	if next, ok := s.pointAt(gfx.Point{X: caret, Y: y}); ok {
		return next, true
	}
	return point, true
}

func nextGraphemeOf(value string, offset int) int {
	for next := offset + 1; next < len(value); next++ {
		if gfx.GraphemeStart(value, next) == next {
			return next
		}
	}
	return len(value)
}

// caretOf is the x of the place point is in the line the last paint drew it on,
// and that line's box.
func (s *TextSelectionState) caretOf(point TextPoint) (float32, Rect, bool) {
	t := &s.texts[point.Surface]
	if t.count == 0 || s.canvas == nil {
		return 0, Rect{}, false
	}
	lines := s.lines[t.first : t.first+t.count]
	index := len(lines) - 1
	for k, line := range lines {
		if point.Offset < line.to || k == len(lines)-1 {
			index = k
			break
		}
	}
	line := lines[index]
	within := min(max(point.Offset-line.from, 0), line.visible)
	origin := s.canvas.LabelOrigin(line.shown, line.bounds, t.style, t.align)
	return origin + s.canvas.TextCaretStyle(line.shown, t.style, within), line.bounds, true
}

// pointAt is the place in the area's texts under a point of the window: in the
// text under it, or else the nearest one, so a drag that leaves a text or crosses
// the gap between two goes on into the next. Above a text it is the text's start
// and below it its end. ok is false for an area with no text.
func (s *TextSelectionState) pointAt(at gfx.Point) (TextPoint, bool) {
	best, found := -1, false
	var bestDistance float32
	for i := range s.texts {
		b := s.texts[i].bounds
		dy := max(b.Y-at.Y, at.Y-(b.Y+b.Height), 0)
		dx := max(b.X-at.X, at.X-(b.X+b.Width), 0)
		distance := dy*1_000_000 + dx
		if !found || distance < bestDistance {
			best, bestDistance, found = i, distance, true
		}
	}
	if !found {
		return TextPoint{}, false
	}
	return s.pointIn(best, at), true
}

// pointIn is the place in text i nearest a point of the window.
func (s *TextSelectionState) pointIn(i int, at gfx.Point) TextPoint {
	t := &s.texts[i]
	if t.count == 0 {
		return TextPoint{Surface: i}
	}
	lines := s.lines[t.first : t.first+t.count]
	switch {
	case at.Y < lines[0].bounds.Y:
		return TextPoint{Surface: i}
	case at.Y >= lines[len(lines)-1].bounds.Y+lines[len(lines)-1].bounds.Height:
		return TextPoint{Surface: i, Offset: len(t.text)}
	}
	line := lines[len(lines)-1]
	for _, candidate := range lines {
		if at.Y < candidate.bounds.Y+candidate.bounds.Height {
			line = candidate
			break
		}
	}
	if s.canvas == nil {
		return TextPoint{Surface: i, Offset: line.from}
	}
	origin := s.canvas.LabelOrigin(line.shown, line.bounds, t.style, t.align)
	offset := s.canvas.TextOffsetAtStyle(line.shown, t.style, at.X-origin)
	if offset > line.visible {
		// The ellipsis stands for the rest of the text.
		return TextPoint{Surface: i, Offset: line.to}
	}
	return TextPoint{Surface: i, Offset: line.from + offset}
}

// beginTextArea starts collecting the texts of an area as its element is painted.
func (f *Frame) beginTextArea(s *TextSelectionState) {
	s.texts, s.lines, s.parent = s.texts[:0], s.lines[:0], -1
	if s.scrollNode >= 0 && int(s.scrollNode) < len(f.Nodes) {
		s.scrollRect = f.Nodes[s.scrollNode].Rect
	}
}

// endTextArea keeps the selection inside the texts the paint drew, and follows a
// drag the paint scrolled the texts under.
func (f *Frame) endTextArea(s *TextSelectionState) {
	// A press that has not moved yet holds an empty selection, which is the start of
	// a drag and not one to drop.
	s.Selection.dragging = s.pressed
	s.Selection.Clamp(len(s.texts), s.length)
	s.Selection.dragging = false
	if s.pressed && s.moved {
		s.extendTo(s.pointer)
	}
}

// beginSelectableText collects a Text of the area being painted, whose element
// put it in r; addSelectableLine then gives each of its lines as painted, and
// paintSelection draws its selected part and the region over it that takes the
// pointer for the area, before its words are drawn over that.
func (f *Frame) beginSelectableText(n *Node, r Rect, style gfx.TextStyle, alignment gfx.Alignment) {
	s := f.area
	t := selectableText{text: n.Text, style: style, align: alignment, first: int32(len(s.lines)), bounds: r}
	t.tab = n.Parent >= 0 && n.Parent == s.parent && f.Nodes[n.Parent].Dir == row
	s.parent = n.Parent
	s.texts = append(s.texts, t)
	f.lineAt = 0
}

func (f *Frame) addSelectableLine(n *Node, shown string, bounds Rect, last bool) {
	s := f.area
	line := selectableLine{shown: shown, bounds: bounds}
	line.from, line.to, line.visible, f.lineAt = sourceOfLine(n.Text, shown, f.lineAt, last)
	s.lines = append(s.lines, line)
	s.texts[len(s.texts)-1].count++
}

func (f *Frame) paintSelection() {
	s := f.area
	canvas := f.Surface
	if canvas == nil {
		return
	}
	surface := len(s.texts) - 1
	t := &s.texts[surface]
	canvas.HitWithOptions(s.handlers.parts.At(surface), t.bounds, "", gfx.HitOptions{Role: gfx.RoleGroup, Cursor: gfx.PointerCursorText, Presentational: true})
	start, end := s.Selection.In(surface).rangeIn(t.text)
	if start >= end {
		return
	}
	color := fade(f.Palette.Selection, f.Opacity)
	for _, line := range s.lines[t.first : t.first+t.count] {
		lo, hi := max(start, line.from), min(end, line.to)
		if lo >= hi {
			continue
		}
		from, to := min(lo-line.from, line.visible), hi-line.from
		if to > line.visible {
			to = len(line.shown) // into the ellipsis, which stands for the rest
		}
		f.selectionSpans = canvas.TextSelectionStyle(f.selectionSpans[:0], line.shown, t.style, from, to)
		origin := canvas.LabelOrigin(line.shown, line.bounds, t.style, t.align)
		for _, span := range f.selectionSpans {
			canvas.Fill(Rect{X: origin + span.X, Y: line.bounds.Y, Width: span.Width, Height: line.bounds.Height}, color)
		}
	}
}

// sourceOfLine finds a line a Text was broken into in its source, from pos on: the
// bytes it covers, how many bytes of shown are the source's own, and where the next
// line is looked for. A line is the source as it is, without the spaces a break
// took; the last may end in an ellipsis, standing for the rest of the text.
func sourceOfLine(source, shown string, pos int, last bool) (from, to, visible, next int) {
	pos = min(pos, len(source))
	if shown == "" {
		if pos < len(source) && source[pos] == '\n' {
			pos++
		}
		return pos, pos, 0, pos
	}
	if at := strings.Index(source[pos:], shown); at >= 0 {
		from = pos + at
		return from, from + len(shown), len(shown), from + len(shown)
	}
	prefix := strings.TrimSuffix(shown, ellipsisMark)
	if at := strings.Index(source[pos:], prefix); at >= 0 {
		return pos + at, len(source), len(prefix), len(source)
	}
	if last {
		return pos, len(source), 0, len(source)
	}
	return pos, pos, 0, pos
}

const ellipsisMark = "…"
