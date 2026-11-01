//go:build linux && vivagtk

package linux

import (
	"unicode/utf8"
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/text"
)

// gtkIME connects a window's focused text field to GTK's input method framework (GtkIMContext),
// which speaks to IBus, Fcitx, the compositor's text-input protocol or the built-in compose tables,
// whichever the desktop has. The keys the field would receive go to the context first; what it
// composes comes back as the text events a native editor produces, so the field shows marked text
// and commits the choice as it does on the other platforms. The input method owns the candidate
// window; this side says when the field has the keyboard and where its caret is.
type gtkIME struct {
	w   *gtkWindow
	g   *gtkLibrary
	ctx uintptr
	// focused is whether the window has the keyboard; active is whether the context was told the
	// field does.
	focused bool
	active  bool
	// engineComposing is whether the context believes a composition is under way.
	engineComposing bool
	cursor          [4]int32
	haveCursor      bool
	sources         surroundingState
	window          surrounding
	sent            bool
	hasSources      bool
}

func newGTKIME(w *gtkWindow) *gtkIME {
	g := w.g
	ime := &gtkIME{w: w, g: g, ctx: g.imMulticontextNew()}
	cb := callbacks()
	g.connect(ime.ctx, "commit", cb.commit, w.id)
	g.connect(ime.ctx, "preedit-changed", cb.preedit, w.id)
	g.connect(ime.ctx, "retrieve-surrounding", cb.surrounding, w.id)
	g.connect(ime.ctx, "delete-surrounding", cb.deleteSurrounding, w.id)
	return ime
}

// realized gives the context its window, which exists once the toplevel is realized.
func (ime *gtkIME) realized() {
	if window := ime.g.widgetGetWindow(ime.w.toplevel); window != 0 {
		ime.g.imSetClientWindow(ime.ctx, window)
	}
}

func (ime *gtkIME) close() {
	if ime.ctx == 0 {
		return
	}
	if ime.active {
		ime.g.imFocusOut(ime.ctx)
	}
	ime.g.imSetClientWindow(ime.ctx, 0)
	ime.g.objectUnref(ime.ctx)
	ime.ctx, ime.active = 0, false
}

// setFocused records whether the window has the keyboard.
func (ime *gtkIME) setFocused(focused bool) {
	ime.focused = focused
	ime.sync()
}

// filtering reports whether keys should go to the context: a field has the keyboard.
func (ime *gtkIME) filtering() bool { return ime.ctx != 0 && ime.active }

// filter gives a key event to the context and reports whether it took it. Whatever it composes
// arrives later as a commit or a preedit change.
func (ime *gtkIME) filter(event uintptr) bool {
	return ime.g.imFilterKeypress(ime.ctx, event) != 0
}

// sync brings the context's idea of focus and caret in line with the field. It runs after every
// frame, which is when the field changes, and does nothing when nothing has.
func (ime *gtkIME) sync() {
	if ime.ctx == 0 {
		return
	}
	text := ime.w.text
	want := ime.focused && text.enabled && !ime.w.nativeKeys
	switch {
	case want && !ime.active:
		ime.realized()
		ime.active, ime.haveCursor, ime.hasSources, ime.sent = true, false, false, false
		ime.g.imFocusIn(ime.ctx)
	case !want && ime.active:
		ime.active = false
		if ime.engineComposing {
			ime.engineComposing = false
			ime.g.imReset(ime.ctx)
		}
		ime.g.imFocusOut(ime.ctx)
	}
	if !ime.active {
		return
	}
	// The field stopped composing without the context (a click moved the caret, the application
	// replaced the text): it must forget its composition or its next key continues one the user no
	// longer sees.
	if ime.engineComposing && !text.state.Composing {
		ime.engineComposing = false
		ime.g.imReset(ime.ctx)
	}
	ime.updateCursor()
	ime.updateSurrounding()
}

// updateCursor tells the input method's window where the caret is, in the toplevel's coordinates.
func (ime *gtkIME) updateCursor() {
	w := ime.w
	x, y, width, height := ime.caretRectangle()
	var left, top int32
	w.g.widgetTranslate(w.area, w.toplevel, int32(x*w.ratio), int32(y*w.ratio), &left, &top)
	location := [4]int32{left, top, int32(width * w.ratio), int32(height * w.ratio)}
	if ime.haveCursor && location == ime.cursor {
		return
	}
	ime.cursor, ime.haveCursor = location, true
	ime.g.imSetCursorRect(ime.ctx, &location)
}

// caretRectangle is the caret in window coordinates: the line the field draws, or for a terminal,
// which draws none, the cell where its input goes.
func (ime *gtkIME) caretRectangle() (x, y, width, height float32) {
	text := ime.w.text
	input := &text.input
	if input.Placed && input.CaretHeight > 0 {
		return input.CaretX, input.CaretY, input.CaretWidth, input.CaretHeight
	}
	if input.Terminal || text.caret.Bounds.Height == 0 {
		return input.X, input.Y, 1, input.Height
	}
	caret := text.caret.Bounds
	return caret.X, caret.Y, caret.Width, caret.Height
}

// updateSurrounding keeps the context's copy of the text around the caret current. A password, and
// a terminal whose text is not a document, tell it nothing.
func (ime *gtkIME) updateSurrounding() {
	text := ime.w.text
	var sources surroundingState
	if !text.input.Secure && !text.input.Terminal {
		state := text.state
		sources = surroundingState{Value: state.Value, Selection: state.Selection, Composition: state.Composition, Composing: state.Composing}
	}
	if ime.hasSources && sources == ime.sources {
		return
	}
	ime.sources, ime.hasSources = sources, true
	ime.window = surroundingOf(sources)
	ime.sent = false
}

// retrieveSurrounding answers the context asking for the text around the caret.
func (ime *gtkIME) retrieveSurrounding() uintptr {
	if !ime.active || !ime.hasSources {
		return 0
	}
	window := ime.window
	runes := []rune(window.Text)
	cursor := min(max(window.Cursor, 0), len(runes))
	bytesBefore := len(string(runes[:cursor]))
	data := cString(window.Text)
	ime.g.imSetSurrounding(ime.ctx, &data[0], int32(len(data)-1), int32(bytesBefore))
	ime.sent = true
	return 1
}

// deleteSurrounding deletes the text around the caret the context is about to convert again.
func (ime *gtkIME) deleteSurrounding(offset, count int) uintptr {
	if !ime.active || !ime.hasSources {
		return 0
	}
	r, ok := ime.window.fieldRange(offset, count)
	if !ok {
		return 0
	}
	ime.apply(platform.TextEvent{Kind: platform.TextInsert, Replacement: r, HasReplacement: true})
	return 1
}

func (ime *gtkIME) commit(value string) {
	ime.engineComposing = false
	ime.apply(platform.TextEvent{Kind: platform.TextInsert, Text: value})
}

// preeditChanged shows the text being composed as marked text, or removes it.
func (ime *gtkIME) preeditChanged() {
	var str, attrs uintptr
	var cursor int32
	ime.g.imGetPreedit(ime.ctx, &str, &attrs, &cursor)
	value := goString(str)
	if str != 0 {
		ime.g.gFree(str)
	}
	selection := text.Range{}
	length := utf8.RuneCountInString(value)
	selection.Start = min(max(int(cursor), 0), length)
	selection.End = selection.Start
	if attrs != 0 {
		if clause, ok := ime.clause(attrs, value); ok {
			selection = clause
		}
		ime.g.attrListUnref(attrs)
	}
	if value == "" {
		ime.engineComposing = false
		if ime.w.text.state.Composing {
			ime.apply(platform.TextEvent{Kind: platform.TextSetMarked})
			ime.apply(platform.TextEvent{Kind: platform.TextUnmark})
		}
		return
	}
	ime.engineComposing = true
	ime.apply(platform.TextEvent{Kind: platform.TextSetMarked, Text: value, MarkedSelection: selection})
}

// Pango attribute types the input method marks the clause it is converting with.
const (
	pangoAttrBackground = 10
	pangoAttrUnderline  = 11
)

// clause is the part of the preedit the input method shows as selected: the clause being
// converted, which it marks with a background attribute (or, in some modules, a stronger
// underline). The underline an input method puts under the whole preedit is what marked text is
// already drawn with.
func (ime *gtkIME) clause(attrs uintptr, value string) (text.Range, bool) {
	list := ime.g.attrListGetAttributes(attrs)
	defer ime.g.slistFree(list)
	offsets := make([]int, 0, len(value)+1)
	for index := range value {
		offsets = append(offsets, index)
	}
	offsets = append(offsets, len(value))
	runeOf := func(byteIndex uint32) int {
		for rune, offset := range offsets {
			if uint32(offset) >= byteIndex {
				return rune
			}
		}
		return len(offsets) - 1
	}
	for node := list; node != 0; node = *(*uintptr)(unsafe.Add(unsafePointer(node), 8)) {
		attribute := *(*uintptr)(unsafePointer(node))
		kind := *(*uint32)(unsafePointer(*(*uintptr)(unsafePointer(attribute))))
		start := *(*uint32)(unsafe.Add(unsafePointer(attribute), 8))
		end := *(*uint32)(unsafe.Add(unsafePointer(attribute), 12))
		if kind == pangoAttrBackground && end > start {
			return text.Range{Start: runeOf(start), End: runeOf(end)}, true
		}
	}
	return text.Range{}, false
}

// apply runs one text event through the field and presents the result.
func (ime *gtkIME) apply(event platform.TextEvent) {
	frame, err := ime.w.text.textEvent(event)
	ime.w.present(frame, err)
}
