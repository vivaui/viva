package flex

import (
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/ui"
)

// clock is the time type-ahead measures its pause by; tests replace it.
var clock = time.Now

// typeAheadPause is how long a pause ends a word being typed.
const typeAheadPause = time.Second

// SelectState is whether a Select's list is open and, when the list is longer than
// the window, which option is its first. The application keeps one for each select;
// the zero value is a closed one.
type SelectState struct {
	Open  bool
	First int

	// What the last frame declared, for the key and wheel handlers.
	labels   []string
	scope    ui.Scope
	selected int
	visible  int
	wheel    float32 // the part of a row a trackpad has scrolled so far
	typed    [16]byte
	typedLen int
	typedAt  time.Time
}

// Wheel scrolls the open list by what the wheel or trackpad reports: a mouse notch
// moves a row, a trackpad a row for each 32 pixels. While the list is open it takes the
// scroll (the page behind it does not move); closed, it does nothing and reports the
// event unhandled. Call it first from Config.OnScroll:
//
//	config.OnScroll = func(e ui.ScrollEvent) ui.EventResult {
//		if m.sizeSel.Open { return m.sizeSel.Wheel(e) }
//		return m.list.Wheel(e)
//	}
func (s *SelectState) Wheel(event ui.ScrollEvent) ui.EventResult {
	if !s.Open {
		return ui.EventResult{}
	}
	last := len(s.labels) - s.visible
	step := event.DeltaY / selectItem
	if !event.Precise {
		switch {
		case event.DeltaY > 0:
			step = max(step, 1)
		case event.DeltaY < 0:
			step = min(step, -1)
		}
	}
	s.wheel += step
	rows := int(s.wheel)
	s.wheel -= float32(rows)
	before := s.First
	s.First = min(max(s.First+rows, 0), max(last, 0))
	return ui.EventResult{Handled: true, Repaint: s.First != before}
}

// Key does type-ahead: characters typed while the list is open, or while the closed
// select has the keyboard, jump to the first option that starts with them (compared
// without regard to case; a pause of a second starts a new word, and the same letter
// repeated cycles through the options that start with it). In an open list the match is
// scrolled into view and takes the focus, so Enter picks it; on a closed select the list
// opens on it. Give it to Config.OnKey, ahead of your own handling. It reacts to the
// characters in KeyEvent.Text of an unmodified key, which a window delivers there only
// for the platforms and focus that report typed characters as keys; everywhere else it
// is never called with one and does nothing.
func (s *SelectState) Key(event ui.KeyEvent) ui.EventResult {
	count := len(s.labels)
	if count == 0 || event.Text == "" || event.Modifiers&(ui.ModifierControl|ui.ModifierMeta|ui.ModifierAlt) != 0 {
		return ui.EventResult{}
	}
	char, _ := utf8.DecodeRuneInString(event.Text)
	if char == utf8.RuneError || unicode.IsControl(char) || char == ' ' && s.typedLen == 0 {
		return ui.EventResult{}
	}
	at, onItem := event.Target.In(s.scope)
	onTrigger := event.Target == s.scope.At(count)
	if !s.Open && !onTrigger || s.Open && !onItem && !onTrigger {
		return ui.EventResult{}
	}
	now := clock()
	if s.typedLen > 0 && now.Sub(s.typedAt) > typeAheadPause || s.typedLen+len(event.Text) > len(s.typed) {
		s.typedLen = 0
	}
	s.typedLen += copy(s.typed[s.typedLen:], event.Text)
	s.typedAt = now

	current := s.selected
	if onItem && at < count {
		current = at
	}
	match := s.match(current)
	if match < 0 {
		return ui.EventResult{Handled: true}
	}
	if !s.Open {
		s.Open = true
		s.First = min(max(match-s.visible/2, 0), max(count-s.visible, 0))
	} else if match < s.First {
		s.First = match
	} else if match >= s.First+s.visible {
		s.First = min(match-s.visible+1, max(count-s.visible, 0))
	}
	return ui.EventResult{Handled: true, Repaint: true, Focus: s.scope.At(match), FocusVisible: true}
}

// match finds the option the typed text selects, searching on from current: the one
// after it when one letter is repeated, current itself when more has been typed.
func (s *SelectState) match(current int) int {
	typed := s.typed[:s.typedLen]
	first, size := utf8.DecodeRune(typed)
	repeated := len(typed) > size
	for rest := typed[size:]; len(rest) > 0 && repeated; {
		r, n := utf8.DecodeRune(rest)
		repeated, rest = r == first, rest[n:]
	}
	prefix, start := typed, max(current, 0)
	if repeated {
		prefix, start = typed[:size], current+1
	}
	count := len(s.labels)
	for k := 0; k < count; k++ {
		index := (start + k) % count
		if hasPrefixFold(s.labels[index], prefix) {
			return index
		}
	}
	return -1
}

// hasPrefixFold reports whether label starts with prefix, ignoring case.
func hasPrefixFold(label string, prefix []byte) bool {
	for len(prefix) > 0 {
		if label == "" {
			return false
		}
		a, an := utf8.DecodeRuneInString(label)
		b, bn := utf8.DecodeRune(prefix)
		if unicode.ToLower(a) != unicode.ToLower(b) {
			return false
		}
		label, prefix = label[an:], prefix[bn:]
	}
	return true
}

type selectDecl struct {
	scope    ui.Scope
	state    *SelectState
	labels   []string
	selected int
	popup    int32 // the Popup that places the list, or -1 when it is closed
	visible  int   // how many options the list shows at once
	layer    uint16
}

// The parts of a Select's list, from components.Select: an option is 32 tall, the
// list has 4 of padding above and below, and a scroll arrow is 24 tall.
const (
	selectItem   = 32
	selectPad    = 8
	selectArrow  = 24
	selectMargin = 16 // what is kept free of the frame's edge
	selectGap    = 4
)

// Select adds a drop-down: a trigger showing the selected label and, while the state
// is open, a list of labels under it (above it when there is no room below), painted
// by components.Select over everything else. The options are the items of scope, one
// per label; scope.At(len(labels)) to At(len(labels)+4) are the trigger, the dismiss
// target (a press anywhere outside the list, or Escape), the list's focus scope and
// its two scroll arrows, so give the select a scope of its own. A list too long for the
// frame shows a window of options with scroll arrows. selected is the index of the
// current label, negative for none. Placeholder is the text shown then; Small, Disabled
// and Label work as for a button.
//
//	if i, ok := f.Select(sizes, &m.sizeOpen, labels, m.size).Placeholder("Size").Picked(); ok { m.size = i }
//
// Picked does the bookkeeping: a click on the trigger opens or closes the list, a click
// outside it or on an option closes it, and it reports the option chosen. The labels
// slice is kept until End.
func (f *Frame) Select(scope ui.Scope, state *SelectState, labels []string, selected int) Box {
	b := f.box(kindSelect)
	n := &f.nodes[b.i]
	n.aux = int32(len(f.widgets.selects))
	d := selectDecl{scope: scope, state: state, labels: labels, selected: selected, popup: -1, layer: f.widgets.layer + 1}
	d.visible = selectRows(len(labels), f.bounds.Height)
	state.labels, state.scope, state.selected, state.visible = labels, scope, selected, d.visible
	if state.Open && len(labels) > 0 {
		state.First = min(max(state.First, 0), len(labels)-d.visible)
		height := float32(selectPad + selectItem*d.visible)
		if d.visible < len(labels) {
			height += 2 * selectArrow
		}
		popup := f.Popup().BelowBox(b, selectGap, Start).Size(selectListWidth(f.canvas, labels, f.theme.Metrics.FontSizeM), height)
		d.popup = popup.i
	}
	f.widgets.selects = append(f.widgets.selects, d)
	return b
}

// selectListWidth is the width of an open list of labels set in size.
func selectListWidth(canvas *ui.Canvas, labels []string, size float32) float32 {
	var widest float32
	for _, label := range labels {
		widest = max(widest, canvas.TextWidthWeight(label, size, ui.WeightRegular))
	}
	return max(128, widest+48)
}

// sizeSelectList widens or narrows the open list of n to the font size the Select
// was given after it was declared, which is when the size is known.
func (f *Frame) sizeSelectList(n *node) {
	d := &f.widgets.selects[n.aux]
	if d.popup < 0 {
		return
	}
	f.nodes[d.popup].size[axisX] = dimension{dimPx, selectListWidth(f.canvas, d.labels, n.fontSize)}
}

// selectRows is how many of count options a list shows in a frame height tall.
func selectRows(count int, frame float32) int {
	room := frame - selectMargin
	if float32(selectPad+selectItem*count) <= room {
		return count
	}
	return max(int((room-selectPad-2*selectArrow)/selectItem), 1)
}

// Placeholder is the text a Select shows while nothing is selected.
func (b Box) Placeholder(value string) Box {
	if n := b.n(); n.kind == kindSelect {
		n.text = value
	}
	return b
}

// Picked reports which option of a Select this paint is delivering a click on, and
// keeps the select's state: the trigger opens and closes the list, an option or a press
// outside closes it, the arrows move its window.
func (b Box) Picked() (int, bool) {
	n := b.n()
	if n.kind != kindSelect || b.f.canvas == nil {
		return 0, false
	}
	d := &b.f.widgets.selects[n.aux]
	count := len(d.labels)
	index, ok := b.f.canvas.ClickedIn(d.scope)
	if !ok {
		return 0, false
	}
	switch {
	case index < count:
		d.state.Open = false
		return index, true
	case index == count: // the trigger
		d.state.Open = !d.state.Open
		if d.state.Open {
			d.state.First = min(max(d.selected-d.visible/2, 0), max(count-d.visible, 0))
		}
	case index == count+1: // a press outside, or Escape
		d.state.Open = false
	case index == count+3:
		d.state.First = max(d.state.First-max(d.visible-1, 1), 0)
	case index == count+4:
		d.state.First = min(d.state.First+max(d.visible-1, 1), max(count-d.visible, 0))
	}
	return 0, false
}

func (f *Frame) measureSelect(n *node) {
	d := &f.widgets.selects[n.aux]
	var widest float32
	for _, label := range d.labels {
		widest = max(widest, f.canvas.TextWidthWeight(label, f.theme.Metrics.FontSizeM, ui.WeightRegular))
	}
	height := f.theme.Metrics.ControlHeight
	if n.flags&flagSmall != 0 {
		height -= 4
	}
	n.pref = [2]float32{widest + f.theme.Metrics.PadControlX + 36, height}
}

func (f *Frame) paintSelect(n *node) {
	d := &f.widgets.selects[n.aux]
	count := len(d.labels)
	trigger := ui.Rect{X: n.rect.X, Y: n.rect.Y, Width: n.rect.Width, Height: n.pref[axisY]}
	options := components.SelectOptions{
		TriggerID: d.scope.At(count), Label: n.label, Placeholder: n.text,
		Disabled: n.flags&flagDisabled != 0,
		Layer:    d.layer, Scope: d.scope.At(count + 2), DismissID: d.scope.At(count + 1), Viewport: f.bounds,
	}
	if n.flags&flagSmall != 0 {
		options.Size = components.SelectSizeSmall
	}
	if d.selected >= 0 && d.selected < count {
		options.Value = d.labels[d.selected]
	}
	if options.Label == "" {
		options.Label = n.text
	}
	open := d.popup >= 0
	if open {
		list := f.nodes[d.popup].rect
		list.Width = max(list.Width, trigger.Width)
		options.Open, options.Content = true, list
	}
	control := components.BeginSelect(f.canvas, f.theme, trigger, options)
	if !open {
		return
	}
	first, visible := d.state.First, d.visible
	if visible < count {
		control.AddScrollUp(d.scope.At(count+3), first == 0)
	}
	for index := first; index < first+visible && index < count; index++ {
		control.AddItem(components.SelectItem{ID: d.scope.At(index), Label: d.labels[index], Selected: index == d.selected})
	}
	if visible < count {
		control.AddScrollDown(d.scope.At(count+4), first+visible >= count)
	}
}
