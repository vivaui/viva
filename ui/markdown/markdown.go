// Package markdown is the opt-in page control for the Markdown renderer of
// package components. It owns the parsed document and the MarkdownView's layout
// caches, and paints through components.MarkdownView, so a page declares a
// Markdown document without parsing, laying out or painting it itself:
//
//	answer := &markdown.State{
//		Source: message,
//		Style:  markdown.Style{Size: c.Theme().Metrics.FontSizeM, LineHeight: 20},
//	}
//	...
//	markdown.View(c, "Assistant answer", answer).FillWidth().Height(420)
//
// Source is parsed when it changes; the document is laid out to the control's
// width when the source, the style's sizing fields or the width change. A warm
// paint with the same source, style and width therefore allocates nothing: the
// parse and layout caches live in the wrapper beside the caller's model.
//
// The caller owns the selection, the scroll offset and the view. Fill
// State.Selection to highlight text, set State.Offset to scroll, and read
// State.SelectedText, State.OffsetAt and State.CaretAt to build a
// selection gesture. Links are hit regions of the control itself: an event
// whose target is not the control can be turned back into a link number with
// State.LinkIndex, and the destination with State.Link.
//
// It is a separate package so an application that does not import it pays
// nothing for it, and it depends only on the public gfx, components, markdown
// and ui packages. Keep a State in the model: its handlers and draw function
// are made once, and do not copy it after View has declared it.
//
// The package shares its name with the root markdown parser; a file that
// imports both aliases one of them.
package markdown

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/markdown"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/window"
)

// Style sizes the text of a Markdown document. It is components.MarkdownStyle,
// named here so a page that declares a State reads through this package alone.
type Style = components.MarkdownStyle

// Look colors a Markdown document. It is components.MarkdownLook.
type Look = components.MarkdownLook

// Selection is a range of the rendered text, in byte offsets.
type Selection = components.MarkdownSelection

// Code is a laid-out fenced code block.
type Code = components.MarkdownCode

// CodeRun is a colored stretch of one line of highlighted code.
type CodeRun = components.MarkdownCodeRun

// CodeClass says what a stretch of code is.
type CodeClass = components.MarkdownCodeClass

// Code classes, as components.MarkdownCodeClass names them.
const (
	CodeComment  = components.CodeComment
	CodeString   = components.CodeString
	CodeNumber   = components.CodeNumber
	CodeKeyword  = components.CodeKeyword
	CodeFunction = components.CodeFunction
	CodeVariable = components.CodeVariable
	CodeFlag     = components.CodeFlag
	CodeKey      = components.CodeKey
	CodeAdded    = components.CodeAdded
	CodeRemoved  = components.CodeRemoved
	// CodeClasses is the size of Look.Code.
	CodeClasses = components.MarkdownCodeClasses
)

// State is one document's page control: the caller's source and
// presentation, the renderer's parse and layout caches, and the callbacks that
// take its input.
//
// Fill Source and the presentation fields from the model every paint. The
// cached Document and MarkdownView are owned by the wrapper; do not call their
// methods to paint at the call site, and do not copy a State after View has
// declared it.
type State struct {
	// Source is the Markdown text painted next. It is parsed when it changes.
	Source string
	// Style sizes the text. It is applied on the next layout; a change to Size,
	// LineHeight, MonoFace or CodeInset re-lays the document out. A change to
	// Highlight or Image cannot be observed by comparison, so call Relayout
	// after replacing either function when the source and width stay the same.
	Style Style
	// Look colors the text. The zero value derives the look from the page's
	// theme with components.MarkdownLookFor.
	Look Look
	// View is the renderer's layout and paint cache. The wrapper keeps it
	// across paints; treat it as read-only.
	View components.MarkdownView
	// Selection is highlighted behind its text. It is clamped to the rendered
	// text when it is painted.
	Selection Selection
	// Offset is the caller-owned vertical scroll offset in pixels. The document
	// is painted Offset pixels above the control's top and clipped to the
	// control.
	Offset float32

	// Pointer receives every pointer event aimed at the control or one of its
	// links. State.LinkIndex turns a link target back into a link number.
	Pointer func(gfx.PointerEvent) gfx.EventResult
	// Key receives the keys pressed while the control has the focus.
	Key func(gfx.KeyEvent) gfx.EventResult
	// Scroll receives the wheel over the control. A nil callback leaves it to
	// the page.
	Scroll func(gfx.ScrollEvent) gfx.EventResult
	// Text supplies the platform's text service while the control has the focus.
	// A document is not editable; it lets an application observe or extend the
	// text context.
	Text func(window.TextInputContext) *window.TextInput

	doc          markdown.Document
	handlers     ui.Handlers
	drawFn       func(*ui.DrawContext)
	prepared     bool
	parsed       bool
	parsedSource string
	layout       layoutKey
	laidOut      bool
	dirty        bool
}

// layoutKey is the input that decides the document's layout, apart from the
// parsed source and the two callbacks whose function values cannot be compared.
type layoutKey struct {
	width               float32
	size, line, inset   float32
	face                gfx.MonoFace
	highlight, imageSet bool
}

// View declares the Markdown document at c. Give it a height: the control lays
// the document out to the width it is given, clips what does not fit, and leaves
// the height to the caller (Height, Fill or the page's flow). label names the
// control for assistive technology.
//
// Declare the view on its own, not inside a Memo.
func View(c ui.C, label string, state *State) ui.Box {
	if state == nil {
		panic("markdown: View needs its State")
	}
	state.prepare()
	return ui.Control(c, label, &state.handlers).Draw(state.drawFn)
}

// Relayout lays the document out again on the next paint even when the source,
// the style's sizing fields and the width are unchanged. Call it after the
// answer of Style.Image changes or after replacing Style.Highlight or
// Style.Image with a different function.
func (m *State) Relayout() { m.dirty = true }

// Height is the height of the document as last laid out, which may be larger
// than the control; use it to clamp Offset.
func (m *State) Height() float32 { return m.View.Height() }

// TextLen is the length in bytes of the rendered text that can be selected.
func (m *State) TextLen() int { return m.View.TextLen() }

// SelectedText is what a copy of Selection puts on the clipboard.
func (m *State) SelectedText() string { return m.View.SelectedText(m.Selection) }

// OffsetAt maps a window point to the nearest rendered text offset. It uses the
// geometry of the last paint, so it is valid from that paint until the next
// layout or paint.
func (m *State) OffsetAt(point gfx.Point) (int, bool) { return m.View.OffsetAt(point) }

// CaretAt is the window rectangle of a rendered byte offset after a paint.
func (m *State) CaretAt(offset int) (gfx.Rect, bool) { return m.View.CaretAt(offset) }

// Codes lists the fenced code blocks of the laid-out document, in reading
// order. Their bounds are relative to the painted document origin, which is the
// control's top-left less Offset.
func (m *State) Codes() []components.MarkdownCode { return m.View.Codes() }

// LinkCount is how many link fragments the document has.
func (m *State) LinkCount() int { return m.View.LinkCount() }

// Link is the destination of link fragment i, as LinkIndex numbers it, or "".
func (m *State) Link(i int) string { return m.View.Link(i) }

// LinkIndex returns the link fragment an event target names, and false for the
// control itself or any other target. The number is the one Link and LinkCount
// use.
func (m *State) LinkIndex(target gfx.ID) (int, bool) {
	if i, ok := target.In(m.handlers.Parts()); ok && i < m.View.LinkCount() {
		return i, true
	}
	return 0, false
}

func (m *State) prepare() {
	if m.prepared {
		return
	}
	m.prepared = true
	m.handlers.Pointer = m.pointer
	m.handlers.Key = m.key
	m.handlers.Scroll = m.scroll
	m.handlers.Text = m.text
	m.drawFn = m.draw
}

func (m *State) pointer(event gfx.PointerEvent) gfx.EventResult {
	if m.Pointer == nil {
		return gfx.EventResult{}
	}
	return m.Pointer(event)
}

func (m *State) key(event gfx.KeyEvent) gfx.EventResult {
	if m.Key == nil {
		return gfx.EventResult{}
	}
	return m.Key(event)
}

func (m *State) scroll(event gfx.ScrollEvent) gfx.EventResult {
	if m.Scroll == nil {
		return gfx.EventResult{}
	}
	return m.Scroll(event)
}

func (m *State) text(context window.TextInputContext) *window.TextInput {
	if m.Text == nil {
		return nil
	}
	return m.Text(context)
}

func (m *State) draw(d *ui.DrawContext) {
	width := d.Rect.Width
	if width <= 0 || d.Rect.Height <= 0 {
		return
	}
	m.ensureLayout(d.Canvas, width)
	look := m.Look
	if look == (components.MarkdownLook{}) {
		look = components.MarkdownLookFor(*d.Theme)
	}
	m.View.Paint(d.Canvas, components.MarkdownPaint{
		Look:      look,
		X:         d.Rect.X,
		Y:         d.Rect.Y - m.Offset,
		Clip:      d.Rect,
		Links:     m.handlers.Parts(),
		Selection: m.Selection,
	})
}

// ensureLayout parses a new source and lays the document out when its source,
// style or width changed. Both caches belong to the wrapper, so a repaint with
// the same input does no parse, no measurement and no allocation.
func (m *State) ensureLayout(canvas *gfx.Canvas, width float32) {
	if !m.parsed || m.Source != m.parsedSource {
		m.doc.Parse(m.Source)
		m.parsedSource = m.Source
		m.parsed = true
		m.dirty = true
	}
	key := layoutKey{
		width: width, size: m.Style.Size, line: m.Style.LineHeight,
		inset: m.Style.CodeInset, face: m.Style.MonoFace,
		highlight: m.Style.Highlight != nil, imageSet: m.Style.Image != nil,
	}
	if m.dirty || !m.laidOut || key != m.layout {
		m.View.Layout(canvas, m.Style, &m.doc, width)
		m.layout = key
		m.laidOut = true
		m.dirty = false
	}
}
