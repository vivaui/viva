package base

import (
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/vivaui/viva/gfx"
)

// clock is the time type-ahead measures its pause by; tests replace it.
var Clock = time.Now

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
	disabled []bool // DisabledOptions of the last declaration, for the keys
	scope    gfx.Scope
	selected int
	Visible  int
	trigger  gfx.ID // the trigger's hit: the application's id when it named one
	// chosen is 1 + the option a key chose, reported by the next Picked and
	// cleared then; 0 when no key chose one.
	chosen   int
	wheel    float32 // the part of a row a trackpad has scrolled so far
	typed    [16]byte
	typedLen int
	typedAt  time.Time

	// The widest option label at optionsSize. Measuring every option costs a
	// text measurement per label, so the state keeps the answer while the
	// labels and the size are the ones it was measured for; a repaint then
	// costs one slice comparison instead.
	optionsMeasured bool
	optionsWidest   float32
	optionsSize     float32

	// The limits ListMaxWidth and ListMaxHeight put on the open list; zero is
	// no limit. They are read when the list is declared, so they apply to the
	// frame the list opens in.
	listMaxWidth  float32
	listMaxHeight float32
}

// ListMaxWidth caps the width of an open Select's list: zero keeps the list's
// measured width (the longest option and its padding), never narrower than the
// trigger. It has no effect on a box that is not a Select.
func (b Box) ListMaxWidth(width float32) Box {
	if n := b.N(); n.Kind == kindSelect {
		WidgetsOf(b.f).selects[n.Aux].state.listMaxWidth = max(width, 0)
	}
	return b
}

// ListMaxHeight caps the total height of an open Select's list, its padding and
// the scroll arrows included; zero keeps the room the window leaves. The list
// shows as many options as fit and scrolls for the rest. It has no effect on a
// box that is not a Select.
func (b Box) ListMaxHeight(height float32) Box {
	if n := b.N(); n.Kind == kindSelect {
		WidgetsOf(b.f).selects[n.Aux].state.listMaxHeight = max(height, 0)
	}
	return b
}

// listRowsForHeight is how many of count options a list shows within height
// pixels: its padding and, when it scrolls, its two arrows count against the
// height.
func listRowsForHeight(count int, height float32) int {
	rows := int((height - selectPad) / selectItem)
	if rows < count {
		rows = int((height - selectPad - 2*selectArrow) / selectItem)
	}
	return max(rows, 1)
}

// Wheel scrolls the open list by what the wheel or trackpad reports: a mouse notch
// moves a row, a trackpad a row for each 32 pixels. While the list is open it takes the
// scroll (the page behind it does not move); closed, it does nothing and reports the
// event unhandled. Call it first from Config.OnScroll:
//
//	config.OnScroll = func(e gfx.ScrollEvent) gfx.EventResult {
//		if m.sizeSel.Open { return m.sizeSel.Wheel(e) }
//		return m.list.Wheel(e)
//	}
func (s *SelectState) Wheel(event gfx.ScrollEvent) gfx.EventResult {
	if !s.Open {
		return gfx.EventResult{}
	}
	last := len(s.labels) - s.Visible
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
	return gfx.EventResult{Handled: true, Repaint: s.First != before}
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
func (s *SelectState) Key(event gfx.KeyEvent) gfx.EventResult {
	count := len(s.labels)
	if count == 0 || event.Release {
		return gfx.EventResult{}
	}
	at, onItem := event.Target.In(s.scope)
	onTrigger := event.Target == s.TriggerID(count)
	if result, handled := s.navigate(event, count, at, onTrigger, onItem && at < count); handled {
		return result
	}
	if event.Text == "" || event.Modifiers&(gfx.ModifierControl|gfx.ModifierMeta|gfx.ModifierAlt) != 0 {
		return gfx.EventResult{}
	}
	char, _ := utf8.DecodeRuneInString(event.Text)
	if char == utf8.RuneError || unicode.IsControl(char) || char == ' ' && s.typedLen == 0 {
		return gfx.EventResult{}
	}
	if !s.Open && !onTrigger || s.Open && !onItem && !onTrigger {
		return gfx.EventResult{}
	}
	now := Clock()
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
		return gfx.EventResult{Handled: true}
	}
	if !s.Open {
		s.Open = true
		s.First = min(max(match-s.Visible/2, 0), max(count-s.Visible, 0))
	} else if match < s.First {
		s.First = match
	} else if match >= s.First+s.Visible {
		s.First = min(match-s.Visible+1, max(count-s.Visible, 0))
	}
	return gfx.EventResult{Handled: true, Repaint: true, Focus: s.scope.At(match), FocusVisible: true}
}

// navigate moves an open list and opens or closes it from the keyboard, as a
// select does: the arrows and Home/End move the focus among its options, Enter or
// Space chooses the focused one, and Escape closes it back onto the trigger. The
// value a key chooses is reported by the next Picked, like a click.
func (s *SelectState) navigate(event gfx.KeyEvent, count, at int, onTrigger, onOption bool) (gfx.EventResult, bool) {
	if event.Modifiers&(gfx.ModifierControl|gfx.ModifierMeta|gfx.ModifierAlt) != 0 {
		return gfx.EventResult{}, false
	}
	handled := gfx.EventResult{Handled: true, Repaint: true}
	if !s.Open {
		if !onTrigger {
			return gfx.EventResult{}, false
		}
		target := -1
		switch event.Key {
		case gfx.KeyEnter, gfx.KeySpace:
			if event.Repeat {
				return gfx.EventResult{}, false
			}
			target = s.selected
		case gfx.KeyDown:
			target = s.selected
		case gfx.KeyUp:
			target = s.lastEnabled()
		default:
			return gfx.EventResult{}, false
		}
		if target < 0 || OptionDisabled(s.disabled, target) {
			target = s.nextEnabled(-1, 1)
		}
		if target < 0 {
			return handled, true
		}
		s.Open = true
		s.reveal(target)
		return gfx.EventResult{Handled: true, Repaint: true, Focus: s.scope.At(target), FocusVisible: true}, true
	}
	if onTrigger {
		switch event.Key {
		case gfx.KeyDown:
			if target := s.nextFocus(s.selected, 1); target >= 0 {
				return s.focus(target), true
			}
		case gfx.KeyUp:
			if target := s.nextFocus(s.selected, -1); target >= 0 {
				return s.focus(target), true
			}
		case gfx.KeyHome:
			if target := s.firstEnabled(); target >= 0 {
				return s.focus(target), true
			}
		case gfx.KeyEnd:
			if target := s.lastEnabled(); target >= 0 {
				return s.focus(target), true
			}
		case gfx.KeyEnter, gfx.KeySpace:
			if !event.Repeat {
				s.Open = false
			}
			return handled, true
		case gfx.KeyEscape:
			s.Open = false
			return handled, true
		}
		return gfx.EventResult{}, false
	}
	if !onOption {
		return gfx.EventResult{}, false
	}
	switch event.Key {
	case gfx.KeyDown:
		if target := s.nextFocus(at, 1); target >= 0 {
			return s.focus(target), true
		}
	case gfx.KeyUp:
		if target := s.nextFocus(at, -1); target >= 0 {
			return s.focus(target), true
		}
	case gfx.KeyHome:
		if target := s.firstEnabled(); target >= 0 {
			return s.focus(target), true
		}
	case gfx.KeyEnd:
		if target := s.lastEnabled(); target >= 0 {
			return s.focus(target), true
		}
	case gfx.KeyEnter, gfx.KeySpace:
		if event.Repeat || OptionDisabled(s.disabled, at) {
			return handled, true
		}
		s.chosen, s.Open = at+1, false
		return gfx.EventResult{Handled: true, Repaint: true, Focus: s.TriggerID(count), FocusVisible: true}, true
	case gfx.KeyEscape:
		s.Open = false
		return gfx.EventResult{Handled: true, Repaint: true, Focus: s.TriggerID(count), FocusVisible: true}, true
	}
	return gfx.EventResult{}, false
}

// focus moves the list's focus to option i and keeps it inside the window.
func (s *SelectState) focus(i int) gfx.EventResult {
	s.reveal(i)
	return gfx.EventResult{Handled: true, Repaint: true, Focus: s.scope.At(i), FocusVisible: true}
}

// reveal scrolls option i into the list's window.
func (s *SelectState) reveal(i int) {
	if s.Visible <= 0 {
		return
	}
	if i < s.First {
		s.First = i
	}
	if i >= s.First+s.Visible {
		s.First = i - s.Visible + 1
	}
	s.First = min(max(s.First, 0), max(len(s.labels)-s.Visible, 0))
}

// nextFocus is the enabled option after from in direction step, or -1 at the end.
func (s *SelectState) nextFocus(from, step int) int {
	if from < 0 {
		from = -1
		if step < 0 {
			from = len(s.labels)
		}
	}
	for i := from + step; i >= 0 && i < len(s.labels); i += step {
		if !OptionDisabled(s.disabled, i) {
			return i
		}
	}
	return -1
}

// nextEnabled is the first enabled option from the start end of the list.
func (s *SelectState) nextEnabled(from, step int) int { return s.nextFocus(from, step) }

// firstEnabled and lastEnabled are the ends of the options that can be chosen.
func (s *SelectState) firstEnabled() int { return s.nextFocus(-1, 1) }
func (s *SelectState) lastEnabled() int  { return s.nextFocus(len(s.labels), -1) }

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
		if !OptionDisabled(s.disabled, index) && hasPrefixFold(s.labels[index], prefix) {
			return index
		}
	}
	return -1
}

// hasPrefixFold reports whether label starts with prefix, ignoring case.
func hasPrefixFold(Label string, prefix []byte) bool {
	for len(prefix) > 0 {
		if Label == "" {
			return false
		}
		a, an := utf8.DecodeRuneInString(Label)
		b, bn := utf8.DecodeRune(prefix)
		if unicode.ToLower(a) != unicode.ToLower(b) {
			return false
		}
		Label, prefix = Label[an:], prefix[bn:]
	}
	return true
}

type SelectDecl struct {
	scope    gfx.Scope
	state    *SelectState
	labels   []string
	icons    []gfx.Icon
	disabled []bool // this declaration's DisabledOptions: what the list paints
	hint     string // the trigger's Hint
	selected int
	Surface  *gfx.Canvas
	popup    int32 // the Popup that places the list, or -1 when it is closed
	Visible  int   // how many options the list shows at once
	layer    uint16
	rects    int32 // where the rows of this frame's list start in frame.widgets.rows
	// asked, pick and picked keep what Picked found, so asking again in the same
	// paint reports the same option and does not open or close the list twice.
	asked  bool
	pick   int
	picked bool
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
// frame shows a window of options with scroll arrows, and the list floats over the page:
// inside a scroll area it is not cut by the area's clip. selected is the index of the
// current label, negative for none. Placeholder is the text shown then; Small, XSmall, Disabled
// and Label work as for a button.
//
//	if i, ok := f.Select(sizes, &m.sizeOpen, labels, m.size).Placeholder("Size").Picked(); ok { m.size = i }
//
// Picked does the bookkeeping: a click on the trigger opens or closes the list, a click
// outside it or on an option closes it, and it reports the option chosen. The labels
// slice is kept until End.
func (f *Frame) Select(scope gfx.Scope, state *SelectState, labels []string, selected int) Box {
	b := f.SelectBase(scope, state, labels, selected)
	f.Nodes[b.i].Flags |= flagThemed
	return b
}

// SelectBase adds a drop-down without a look: the state (SelectState, Picked, type-ahead
// and the wheel), the trigger and the list's place under it, and everything the control is to
// the pointer, the keyboard and assistive technology (the trigger as a combobox, the open
// list's focus scope, Escape and the dismiss target, its entrance and scrolling, the options
// as options) are the select's, as for Select, and nothing of it is painted. The trigger, the
// panel and the rows are the caller's: Draw on the select paints the trigger in Rect, and
// Popup is the open list's element, whose Draw paints the panel, with Part(i) the rectangle
// of option i (empty when it is scrolled out). components.SelectRecipe, PopupRecipe and
// MenuItemRecipe give the theme's look to start from.
//
//	sel := f.SelectBase(sizes, &m.sizeOpen, labels, m.size).Draw(func(d *ui.DrawContext) { /* the trigger, in d.Rect */ })
//	if list, ok := sel.Popup(); ok {
//		list.Draw(func(d *ui.DrawContext) { /* the panel in d.Rect, option i in sel.Part(i) */ })
//	}
//	if i, ok := sel.Picked(); ok { m.size = i }
func (f *Frame) SelectBase(scope gfx.Scope, state *SelectState, labels []string, selected int) Box {
	b := f.Box(kindSelect)
	n := &f.Nodes[b.i]
	n.Aux = int32(len(WidgetsOf(f).selects))
	d := SelectDecl{scope: scope, state: state, labels: labels, selected: selected, Surface: f.Surface, popup: -1, layer: f.WidgetLayer() + 1, rects: -1}
	d.Visible = selectRows(len(labels), f.bounds.Height)
	if state.listMaxHeight > 0 {
		d.Visible = min(d.Visible, listRowsForHeight(len(labels), state.listMaxHeight))
	}
	widest := state.WidestOption(f.Surface, labels, f.Palette.Metrics.FontSizeM)
	state.disabled = nil
	state.labels, state.scope, state.selected, state.Visible = labels, scope, selected, d.Visible
	if state.Open && len(labels) > 0 {
		state.First = min(max(state.First, 0), len(labels)-d.Visible)
		height := float32(selectPad + selectItem*d.Visible)
		if d.Visible < len(labels) {
			height += 2 * selectArrow
		}
		width := selectListWidth(widest)
		if state.listMaxWidth > 0 {
			width = min(width, state.listMaxWidth)
		}
		popup := f.Popup().BelowBox(b, selectGap, Start).Size(width, height)
		d.popup = popup.i
	}
	WidgetsOf(f).selects = append(WidgetsOf(f).selects, d)
	return b
}

// widestOption is the widest of labels at size, measured once until the labels
// or the size change. The comparison is against the labels the measurement was
// taken for, which is what the state declared last, so calling before or after
// the frame records its labels both work. It is a ui bridge, not application API.
func (s *SelectState) WidestOption(Surface *gfx.Canvas, labels []string, Size float32) float32 {
	if !s.optionsMeasured || s.optionsSize != Size || !sameOptionLabels(s.labels, labels) {
		var widest float32
		for _, label := range labels {
			widest = max(widest, Surface.TextWidthWeight(label, Size, gfx.WeightRegular))
		}
		s.optionsMeasured, s.optionsWidest, s.optionsSize = true, widest, Size
	}
	return s.optionsWidest
}

// sameOptionLabels reports whether two frames declared the same option labels.
func sameOptionLabels(before, after []string) bool {
	if len(before) != len(after) {
		return false
	}
	for index := range after {
		if before[index] != after[index] {
			return false
		}
	}
	return true
}

// selectListWidth is the width of an open list whose widest label measures widest.
func selectListWidth(widest float32) float32 {
	return max(128, widest+48)
}

// sizeSelectList widens or narrows the open list of n to the font size the Select
// was given after it was declared, which is when the size is known.
func (f *Frame) sizeSelectList(n *Node) {
	d := &WidgetsOf(f).selects[n.Aux]
	if d.popup < 0 {
		return
	}
	width := selectListWidth(d.state.WidestOption(f.Surface, d.labels, n.FontSize))
	if d.state.listMaxWidth > 0 {
		width = min(width, d.state.listMaxWidth)
	}
	f.Nodes[d.popup].Size[AxisX] = Dimension{dimPx, width}
}

// selectRows is how many of count options a list shows in a frame height tall.
func selectRows(count int, Frame float32) int {
	room := Frame - selectMargin
	if float32(selectPad+selectItem*count) <= room {
		return count
	}
	return max(int((room-selectPad-2*selectArrow)/selectItem), 1)
}

// Placeholder is the text a Select or Combobox shows while nothing is selected.
func (b Box) Placeholder(Value string) Box {
	if n := b.N(); n.Kind == kindSelect {
		n.Text = Value
	} else if s := b.ComboState(); s != nil {
		s.placeholder = Value
	}
	return b
}

// Icons sets the icons a Select paints before its chosen value on the trigger and
// before each option's label in the open list, one per option and indexed like the
// labels: one per option, like the labels. A select given none of them paints
// exactly what it always has; a SelectBase keeps them for its caller and paints
// none, as it does the rest of the look.
func (b Box) Icons(icons []gfx.Icon) Box {
	if n := b.N(); n.Kind == kindSelect {
		WidgetsOf(b.f).selects[n.Aux].icons = icons
	}
	return b
}

// DisabledOptions marks options unavailable, one per option and indexed like the
// labels: a true entry is painted dimmed, cannot take the focus or a click, is
// skipped by type-ahead and the arrow keys, and is never reported by Picked or
// Chosen. It is for a Select (whose options are its list's rows) and for a
// Segmented control, which may also declare the state in each option's own
// Disabled field; both read the same look. The slice is the caller's and is read
// until the paint ends; a shorter one leaves the options past its end enabled. An
// option list given none has every option enabled.
func (b Box) DisabledOptions(disabled []bool) Box {
	n := b.N()
	switch n.Kind {
	case kindSelect:
		d := &WidgetsOf(b.f).selects[n.Aux]
		d.disabled, d.state.disabled = disabled, disabled
	case kindSegmented:
		if len(disabled) == 0 {
			return b
		}
		// The merged options go into the choice scratch: the caller's slice stays
		// its own, and a warm paint reuses the same room.
		w := WidgetsOf(b.f)
		d := &w.segmented[n.Aux]
		first := len(w.choices)
		for index, option := range d.options {
			option.Disabled = option.Disabled || OptionDisabled(disabled, index)
			w.choices = append(w.choices, option)
		}
		d.options = w.choices[first:len(w.choices):len(w.choices)]
	}
	return b
}

// Hint is the description assistive technology reads after a Select's name and value, for
// what the value alone does not say, such as the path a chosen shell stands for.
func (b Box) Hint(Text string) Box {
	if n := b.N(); n.Kind == kindSelect {
		WidgetsOf(b.f).selects[n.Aux].hint = Text
	}
	return b
}

// optionDisabled reports whether option i is marked in disabled.
func OptionDisabled(disabled []bool, i int) bool { return i >= 0 && i < len(disabled) && disabled[i] }

// Picked reports which option of a Select this paint is delivering a click on, and
// keeps the select's state: the trigger opens and closes the list, an option or a press
// outside closes it, the arrows move its window.
func (b Box) Picked() (int, bool) {
	n := b.N()
	if n.Kind != kindSelect || b.f.Surface == nil {
		return 0, false
	}
	d := &WidgetsOf(b.f).selects[n.Aux]
	if d.asked {
		return d.pick, d.picked
	}
	d.asked = true
	d.pick, d.picked = d.pickIn()
	return d.pick, d.picked
}

// pickIn is Picked's work, done once a paint.
func (d *SelectDecl) pickIn() (int, bool) {
	if chosen := d.state.chosen; chosen > 0 {
		d.state.chosen, d.state.Open = 0, false
		return chosen - 1, true
	}
	canvas := d.Surface
	count := len(d.labels)
	if trigger := d.state.trigger; !trigger.IsZero() && canvas.Clicked(trigger) {
		d.state.Open = !d.state.Open
		if d.state.Open {
			d.state.First = min(max(d.selected-d.Visible/2, 0), max(count-d.Visible, 0))
		}
		return 0, false
	}
	index, ok := canvas.ClickedIn(d.scope)
	if !ok {
		return 0, false
	}
	switch {
	case index < count:
		d.state.Open = false
		return index, true
	case index == count: // the trigger, when it is the page's own
		d.state.Open = !d.state.Open
		if d.state.Open {
			d.state.First = min(max(d.selected-d.Visible/2, 0), max(count-d.Visible, 0))
		}
	case index == count+1: // a press outside, or Escape
		d.state.Open = false
	case index == count+3:
		d.state.First = max(d.state.First-max(d.Visible-1, 1), 0)
	case index == count+4:
		d.state.First = min(d.state.First+max(d.Visible-1, 1), max(count-d.Visible, 0))
	}
	return 0, false
}

// selectKey gives the keys of a focused trigger or an open option to its Select:
// Enter, Space, the arrows and Home/End open and move its list, and Enter chooses.
func (p *Page) selectKey(event gfx.KeyEvent) (gfx.EventResult, bool) {
	for _, state := range p.selects {
		if state.scope == 0 {
			continue
		}
		if _, ok := event.Target.In(state.scope); !ok {
			continue
		}
		result := state.Key(event)
		if result.Handled || result.Repaint {
			return result, true
		}
		return gfx.EventResult{}, false
	}
	return gfx.EventResult{}, false
}

// triggerID is the id the trigger paints: the application's when it named one.
func (d *SelectDecl) TriggerID(count int) gfx.ID { return d.state.TriggerID(count) }

// triggerID is the trigger's identity for the keys, which see no declaration. It is
// a ui bridge, not application API.
func (s *SelectState) TriggerID(count int) gfx.ID {
	if !s.trigger.IsZero() {
		return s.trigger
	}
	return s.scope.At(count)
}

// Part is where option i of an open SelectBase's list is, valid once the select has been painted
// (in the callbacks of Draw and DrawOver, and after End): the rectangle the caller paints the
// row in. It is empty for a closed select, an option scrolled out of the list, and any other
// element.
func (b Box) Part(i int) Rect { return b.f.SelectPart(b.N(), i) }

// Popup is the element that places the open list of a SelectBase, and false while the list is
// closed. It is painted after the page, over it, so what is drawn on it with Draw is the panel.
func (b Box) Popup() (Box, bool) {
	n := b.N()
	if n.Kind != kindSelect {
		return Box{}, false
	}
	d := &WidgetsOf(b.f).selects[n.Aux]
	if d.popup < 0 {
		return Box{}, false
	}
	return Box{f: b.f, i: d.popup, gen: b.f.gen}, true
}

func (f *Frame) SelectPart(n *Node, i int) Rect {
	if n.Kind != kindSelect {
		return Rect{}
	}
	d := &WidgetsOf(f).selects[n.Aux]
	if d.rects < 0 || i < 0 || i >= len(d.labels) {
		return Rect{}
	}
	return WidgetsOf(f).rows[int(d.rects)+i]
}
