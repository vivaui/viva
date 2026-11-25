package base

import (
	"slices"
	"sort"
	"strings"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
)

// TextareaCommand is a request for the caller-owned editor. The component
// never applies commands or retains editor state itself.
type TextareaCommand uint8

const (
	TextareaCommandNone TextareaCommand = iota
	TextareaCommandInsertNewline
)

// TextareaSemantics is the typed semantic projection of a textarea snapshot.
// Selection and composition remain UTF-8 byte ranges matching the paint API.
type TextareaSemantics struct {
	ID               gfx.ID
	Label            string
	Value            string
	Hint             string
	Focused          bool
	Disabled         bool
	ReadOnly         bool
	Invalid          bool
	SelectionStart   int
	SelectionEnd     int
	CompositionStart int
	CompositionEnd   int
}

// TextareaSnapshot is a caller-owned multiline editor presentation. Long lines
// wrap at the width of the box, as in shadcn's textarea, and a newline in
// DisplayValue starts a row; scrolling follows the caret when Layout is set.
// Editing and IME state remain owned by the application/runtime.
type TextareaSnapshot struct {
	PaintLine    func(*gfx.Canvas, string, gfx.Rect, gfx.TextStyle)
	ID           gfx.ID
	Label        string
	Value        string
	DisplayValue string
	Placeholder  string
	Hint         string
	Disabled     bool
	ReadOnly     bool
	Invalid      bool
	// Warning paints the field with the warning border; Invalid wins over it.
	Warning       bool
	Focused       bool
	NativeEditing bool
	ShowCaret     bool
	// HideEditorText makes the platform editor behind a focused textarea
	// invisible: the component paints the text, selection, composition and caret
	// itself and the editor is only where input arrives (set NativeEditing
	// false, ShowCaret true). A platform editor draws one line, centered in its
	// box, which is right for an input and wrong for a box of several rows, where
	// the caret would sit in the middle instead of on the first line.
	HideEditorText bool
	// NoWrap keeps every line on one row, clipped at the edge of the box, as
	// `white-space: pre` does. The default wraps.
	NoWrap bool
	// TextSize and TextWeight style the rows' text; zero keeps the theme's body
	// size and the regular weight. A page's element passes its own FontSize and
	// Weight here, so the rows, the caret and the native editor agree.
	TextSize, TextWeight float32
	// InsetTop reserves room above the editor for caller-owned controls. The
	// native text bounds and caret placement use the same inset as the paint.
	InsetTop float32
	// InsetBottom keeps that much more of the bottom of the box free of text, for
	// controls the caller lays over it, such as an input group's block-end addon:
	// the rows scroll above them rather than behind.
	InsetBottom float32
	// InsetRight reserves room for caller-owned controls such as a scrollbar.
	InsetRight float32
	// Layout makes the rows the application's to move through: it keeps them from
	// one frame to the next (so a steady paint allocates nothing), scrolls to the
	// caret and, held by the application's key and pointer handlers, moves the
	// caret by visual row (see TextareaLayout). Without it the box wraps and
	// paints from its first row, which suits a box that shows what is in it.
	// With it, DisplayValue must be empty or the text of Value, whose byte
	// offsets Layout reports.
	Layout         *TextareaLayout
	Caret          int
	SelectionStart int
	SelectionEnd   int
	// SelectionRanges, when nonempty, replaces the single selection for
	// painting: the selections of a multi-selection editor (see
	// text.Editor.SelectionSpansInto), as byte offsets into Value in order and
	// non-overlapping. Collapsed ones paint nothing; only the primary has Caret.
	SelectionRanges  []text.Span
	CompositionStart int
	CompositionEnd   int
}

// TextareaMovement is a way of moving the caret of a wrapping Textarea that
// depends on how its text is laid out in rows.
type TextareaMovement uint8

const (
	TextareaMoveNone TextareaMovement = iota
	// TextareaMoveUp and TextareaMoveDown go one row, to the place nearest the
	// column the caret had when the run of vertical moves began.
	TextareaMoveUp
	TextareaMoveDown
	// TextareaMoveLineStart and TextareaMoveLineEnd go to the ends of the
	// visual row, not of the line up to a newline.
	TextareaMoveLineStart
	TextareaMoveLineEnd
	// TextareaMovePageUp and TextareaMovePageDown go a box of rows.
	TextareaMovePageUp
	TextareaMovePageDown
	TextareaMoveTextStart
	TextareaMoveTextEnd
)

// TextareaLayout is the visual-row layout of one Textarea, held by the
// application next to its editor and given to every paint through
// TextareaSnapshot.Layout. The text and its byte offsets stay the application's;
// the layout knows only where they fall on the screen:
//
//   - it keeps the wrapped rows and the scroll position between frames, so a
//     steady paint builds nothing, and scrolls to keep the caret in view when
//     the caret moves;
//   - Move turns an arrow, Home, End or Page key into the byte offset it leads
//     to, keeping the column of a run of Up and Down;
//   - OffsetAt and CaretBounds, which the runtime reads through the hit region
//     (gfx.TextPlacement), put the caret where a press lands and the
//     input-method window where the caret is;
//   - ContentHeight sizes a box that grows with its text.
//
// The zero value is ready. It is not safe for concurrent use, and one layout
// serves one textarea.
type TextareaLayout struct {
	canvas *gfx.Canvas
	text   string
	Rows   []gfx.TextRow
	wrap   bool
	size   float32
	width  float32 // the width rows were wrapped to
	fonts  uint32  // the canvas's FontGeneration the rows were measured in
	// exact tells for each row whether the canvas wrapped it (Show) or it is still
	// the estimate Fit made; estimated counts the estimated ones.
	exact     []bool
	estimated int
	fitted    bool
	// boxWidth is the outer width the box was last laid out in, which AutoGrow
	// measures the next time the page lays the box out. It is zero before the
	// first paint.
	boxWidth float32

	// What the last paint showed.
	Painted    bool
	Content    gfx.Rect
	LineHeight float32
	Scroll     float32
	Caret      gfx.Rect
	CaretShown bool

	// The caret the view last followed.
	followed      int
	followedRow   int
	FollowedValid bool

	// A run of Up and Down keeps the column the caret began at.
	goal       float32
	goalCaret  int
	goalActive bool
	// A caret at the end of a wrapped row is also the start of the next; it is
	// drawn at the end of the first when it got there by going to that end.
	Upstream      bool
	UpstreamCaret int
}

// Semantics returns a normalized, allocation-free semantic snapshot. It does
// not claim that a platform bridge exposes selection or invalid state yet.
func (snapshot TextareaSnapshot) Semantics() TextareaSemantics {
	display := snapshot.DisplayValue
	if display == "" {
		display = snapshot.Value
	}
	selectionStart, selectionEnd := textareaRange(display, snapshot.SelectionStart, snapshot.SelectionEnd)
	compositionStart, compositionEnd := textareaRange(display, snapshot.CompositionStart, snapshot.CompositionEnd)
	return TextareaSemantics{
		ID: snapshot.ID, Label: snapshot.Label, Value: snapshot.Value, Hint: snapshot.Hint,
		Focused: snapshot.Focused && !snapshot.Disabled, Disabled: snapshot.Disabled,
		ReadOnly: snapshot.ReadOnly, Invalid: snapshot.Invalid,
		SelectionStart: selectionStart, SelectionEnd: selectionEnd,
		CompositionStart: compositionStart, CompositionEnd: compositionEnd,
	}
}

// layoutText is the text a Layout lays out: the one the box paints.
func (snapshot TextareaSnapshot) LayoutText() string {
	if snapshot.DisplayValue != "" {
		return snapshot.DisplayValue
	}
	return snapshot.Value
}

// ScrollBy scrolls the box by dy pixels, down when positive, as a wheel does.
// The next paint keeps the offset until the caret moves.
func (layout *TextareaLayout) ScrollBy(dy float32) {
	if !layout.Painted {
		return
	}
	layout.Scroll = layout.clampScroll(layout.Scroll + dy)
}

// Wheel scrolls the box by a wheel event over the text it last painted, as ScrollBy does, and
// reports whether the box moved. A wheel the box cannot move with (it is at the end the wheel
// points to, or it shows all its rows) reports false, so the page scrolls instead.
func (layout *TextareaLayout) Wheel(event gfx.ScrollEvent) bool {
	content := layout.Content
	if !layout.Painted || event.X < content.X || event.X >= content.X+content.Width ||
		event.Y < content.Y || event.Y >= content.Y+content.Height {
		return false
	}
	dy := event.DeltaY
	if !event.Precise && dy > 0 && dy < 24 {
		dy = 24
	} else if !event.Precise && dy < 0 && dy > -24 {
		dy = -24
	}
	before := layout.Scroll
	layout.ScrollBy(dy)
	return layout.Scroll != before
}

// RowCount is how many rows text takes in a box of outer width width: one for
// each line that fits and as many as a longer one wraps to, and an empty one
// after a final newline. The rows it wraps are the ones the next paint at that
// width uses.
func (layout *TextareaLayout) RowCount(canvas *gfx.Canvas, theme gfx.Theme, text string, width float32, noWrap bool) int {
	return layout.rowCount(canvas, theme, text, width, noWrap, -1)
}

func (layout *TextareaLayout) rowCount(canvas *gfx.Canvas, theme gfx.Theme, text string, width float32, noWrap bool, most int) int {
	layout.Fit(canvas, text, theme.Metrics.FontSizeM, textareaMax(0, width-2*theme.Metrics.TextInsetX()), !noWrap)
	return layout.realizeUpTo(most)
}

// ContentHeight is the height a box of outer width width needs to show text
// without scrolling, never below the 64 pixels of the minimum: the
// `field-sizing: content` of shadcn's textarea. The application caps it where it
// wants the box to start scrolling.
func (layout *TextareaLayout) ContentHeight(canvas *gfx.Canvas, theme gfx.Theme, text string, width float32, noWrap bool) float32 {
	return layout.ContentHeightUpTo(canvas, theme, text, width, noWrap, -1)
}

// ContentHeightUpTo is ContentHeight for a box that grows to at most maxRows rows: the
// rows past that are not wrapped with the canvas, so a long text in an AutoGrow box costs
// the rows it can show. The height is exact while the text takes no more than maxRows
// rows, and taller than maxRows rows when it takes more. A negative maxRows is
// ContentHeight.
func (layout *TextareaLayout) ContentHeightUpTo(canvas *gfx.Canvas, theme gfx.Theme, text string, width float32, noWrap bool, maxRows int) float32 {
	rows := layout.rowCount(canvas, theme, text, width, noWrap, maxRows)
	return textareaMax(64, float32(rows)*(theme.Metrics.FontSizeM+7)+2*textareaPadY)
}

// BoxWidth is the outer width the box was last laid out in, which AutoGrow
// measures the next time the page lays it out. It is zero before the first
// paint.
func (layout *TextareaLayout) BoxWidth() float32 { return layout.boxWidth }

// SetBoxWidth records the outer width of a layout, which the page's AutoGrow
// measurement reads.
func (layout *TextareaLayout) SetBoxWidth(width float32) { layout.boxWidth = width }

// RequestRelayout has the page lay itself out and paint again within the same
// Draw: for a measurement that could only be made after the first layout, such
// as a TextArea whose content height follows the width it was given. A frame
// that belongs to no page (a bare preview) has nothing to relayout.
func (f *Frame) RequestRelayout() {
	if f.page != nil {
		f.page.retry = true
	}
}

// OffsetAt is the byte offset of the caret position nearest to a window point,
// for the runtime to place the caret on a press or drag. It implements
// gfx.TextPlacement.
func (layout *TextareaLayout) OffsetAt(point gfx.Point) (int, bool) {
	if !layout.Painted || !layout.fitted || len(layout.Rows) == 0 {
		return 0, false
	}
	row := int((point.Y - layout.Content.Y + layout.Scroll) / layout.LineHeight)
	if point.Y < layout.Content.Y-layout.Scroll {
		row = 0
	}
	row = max(0, min(row, len(layout.Rows)-1))
	offset, upstream := layout.offsetInRow(row, point.X-layout.Content.X)
	layout.setUpstream(offset, upstream)
	return offset, true
}

// CaretBounds is the row the caret was painted on, one pixel wide, in window
// coordinates; false while the caret is scrolled out of view. It implements
// gfx.TextPlacement.
func (layout *TextareaLayout) CaretBounds() (gfx.Rect, bool) {
	return layout.Caret, layout.Painted && layout.CaretShown
}

// Move returns the byte offset of text that movement leads to from caret, laid
// out as the last paint laid it out. Up on the first row goes to the start of
// the text and Down on the last to its end. It records the column of a run of
// vertical moves and where a caret at the end of a wrapped row is drawn, so call
// it for every movement and then put the caret at the offset it returns.
func (layout *TextareaLayout) Move(text string, caret int, movement TextareaMovement) int {
	caret = textareaBoundary(text, caret)
	switch movement {
	case TextareaMoveTextStart:
		return layout.arrive(0, false)
	case TextareaMoveTextEnd:
		return layout.arrive(len(text), false)
	}
	if !layout.Painted || layout.canvas == nil {
		return caret
	}
	layout.Fit(layout.canvas, text, layout.size, layout.width, layout.wrap)
	row := layout.RowOf(caret)
	switch movement {
	case TextareaMoveLineStart:
		return layout.arrive(layout.Rows[row].Start, false)
	case TextareaMoveLineEnd:
		end, upstream := layout.rowEnd(row)
		return layout.arrive(end, upstream)
	case TextareaMoveUp, TextareaMoveDown, TextareaMovePageUp, TextareaMovePageDown:
	default:
		return caret
	}
	step := 1
	if movement == TextareaMovePageUp || movement == TextareaMovePageDown {
		step = max(1, int(layout.Content.Height/layout.LineHeight)-1)
	}
	if movement == TextareaMoveUp || movement == TextareaMovePageUp {
		step = -step
	}
	if !layout.goalActive || layout.goalCaret != caret {
		layout.goal = layout.columnOf(row, caret)
	}
	target := row + step
	var offset int
	var upstream bool
	switch {
	case target < 0:
		offset = 0
	case target >= len(layout.Rows):
		offset = len(text)
	default:
		offset, upstream = layout.offsetInRow(target, layout.goal)
	}
	goal := layout.goal
	offset = layout.arrive(offset, upstream)
	layout.goal, layout.goalCaret, layout.goalActive = goal, offset, true
	return offset
}

// arrive notes where the caret ended up and returns it.
func (layout *TextareaLayout) arrive(offset int, upstream bool) int {
	layout.setUpstream(offset, upstream)
	layout.goalActive = false
	return offset
}

func (layout *TextareaLayout) setUpstream(offset int, upstream bool) {
	layout.Upstream, layout.UpstreamCaret = upstream, offset
}

// Fit lays text out in rows at width unless they already are, and keeps what it
// laid them out with for the events that follow.
//
// The rows are estimated with the reference metrics (gfx.WrapRowEstimate), which shape
// nothing, and Show wraps the ones the box shows with the canvas: opening a long text
// costs the rows in view, not one shaped line for every row of the text. A paragraph
// wraps on its own, so when only the text changed since the last fit, only the
// paragraphs an edit touched are laid out again: the rows before them are kept and the
// rows after them move by the length the edit added or removed. Other fonts, a size or
// a width lay the whole text out again.
func (layout *TextareaLayout) Fit(canvas *gfx.Canvas, text string, size, width float32, wrap bool) {
	fonts := canvas.FontGeneration()
	same := layout.fitted && layout.canvas == canvas && layout.fonts == fonts && layout.wrap == wrap && layout.size == size && layout.width == width
	layout.canvas = canvas
	if same && layout.text == text {
		return
	}
	layout.size, layout.width, layout.wrap = size, width, wrap
	if same {
		layout.rewrap(text)
	} else {
		layout.Rows, layout.exact, layout.estimated = layout.Rows[:0], layout.exact[:0], 0
		layout.estimate(text, 0, len(text)+1)
	}
	layout.fitted, layout.fonts, layout.text = true, fonts, text
}

// wrapWidth is the width rows wrap at, 0 for rows that never wrap.
func (layout *TextareaLayout) wrapWidth() float32 {
	if !layout.wrap {
		return 0
	}
	return layout.width
}

// estimate appends the estimated rows of the paragraphs of text from the one starting
// at from to the one before resume (len(text)+1 for the last).
func (layout *TextareaLayout) estimate(text string, from, resume int) {
	width := layout.wrapWidth()
	for at := from; at >= 0 && at < resume; {
		var row gfx.TextRow
		row, at = gfx.WrapRowEstimate(text, at, layout.size, gfx.WeightRegular, width)
		layout.Rows = append(layout.Rows, row)
		// A row that does not wrap is measured by nothing, so it is what the canvas gives.
		layout.exact = append(layout.exact, width <= 0)
		if width > 0 {
			layout.estimated++
		}
	}
}

// rewrap lays out again the paragraphs of the last fitted text that differ in text,
// keeping the rows of the others.
func (layout *TextareaLayout) rewrap(text string) {
	old := layout.text
	prefix := commonPrefix(old, text)
	suffix := commonSuffix(old[prefix:], text[prefix:])
	// The edit spans the paragraphs from the one the first changed byte is in to the
	// one the last is in; the newline that ends that one is the first in the suffix.
	from := strings.LastIndexByte(text[:prefix], '\n') + 1
	oldResume, newResume := len(old)+1, len(text)+1 // where the next untouched paragraph starts
	if newline := strings.IndexByte(old[len(old)-suffix:], '\n'); newline >= 0 {
		oldResume = len(old) - suffix + newline + 1
		newResume = len(text) - suffix + newline + 1
	}
	rows := layout.Rows
	kept := sort.Search(len(rows), func(index int) bool { return rows[index].Start >= from })
	tail := len(rows)
	if oldResume <= len(old) {
		tail = sort.Search(len(rows), func(index int) bool { return rows[index].Start >= oldResume })
	}
	end := len(rows)
	layout.estimate(text, from, newResume)
	edited := len(layout.Rows) - end
	layout.splice(kept, tail, end)
	if delta := len(text) - len(old); delta != 0 {
		for index := kept + edited; index < len(layout.Rows); index++ {
			layout.Rows[index].Start += delta
			layout.Rows[index].End += delta
		}
	}
}

// splice replaces rows [from, to) with the rows appended after end, the length the rows
// had before: the new rows are turned to sit before the rows after to, and the old ones
// are dropped. The rows' exactness moves with them.
func (layout *TextareaLayout) splice(from, to, end int) {
	for index := from; index < to; index++ {
		if !layout.exact[index] {
			layout.estimated--
		}
	}
	rows, exact := layout.Rows, layout.exact
	slices.Reverse(rows[to:end])
	slices.Reverse(rows[end:])
	slices.Reverse(rows[to:])
	slices.Reverse(exact[to:end])
	slices.Reverse(exact[end:])
	slices.Reverse(exact[to:])
	layout.Rows = append(rows[:from], rows[to:]...)
	layout.exact = append(exact[:from], exact[to:]...)
}

// paragraphRows widens rows [first, last] to the whole paragraphs they are in, among
// the first count rows.
func (layout *TextareaLayout) paragraphRows(first, last, count int) (int, int) {
	rows, text := layout.Rows[:count], layout.text
	for first > 0 && text[rows[first].Start-1] != '\n' {
		first--
	}
	for last+1 < len(rows) && text[rows[last+1].Start-1] != '\n' {
		last++
	}
	return first, last
}

// realize wraps with the canvas the paragraphs of rows [first, last] whose rows are
// estimated, and reports whether it changed any. The text at the top of the box stays
// there, and the row of the caret the box last followed is the row it is on now.
func (layout *TextareaLayout) realize(first, last int) bool {
	if layout.estimated == 0 || len(layout.Rows) == 0 {
		return false
	}
	first, last = max(first, 0), min(last, len(layout.Rows)-1)
	if first > last {
		return false
	}
	first, last = layout.paragraphRows(first, last, len(layout.Rows))
	if !slices.Contains(layout.exact[first:last+1], false) {
		return false
	}
	var top int
	var within float32
	if layout.LineHeight > 0 {
		row := min(int(layout.Scroll/layout.LineHeight), len(layout.Rows)-1)
		top, within = layout.Rows[row].Start, layout.Scroll-float32(row)*layout.LineHeight
	}
	// The block is laid out again on the end of the rows, a paragraph at a time: one
	// that is exact is copied, an estimated one is wrapped with the canvas.
	text, width, end := layout.text, layout.wrapWidth(), len(layout.Rows)
	for row := first; row <= last; {
		_, after := layout.paragraphRows(row, row, end)
		if !slices.Contains(layout.exact[row:after+1], false) {
			layout.Rows = append(layout.Rows, layout.Rows[row:after+1]...)
			layout.exact = append(layout.exact, layout.exact[row:after+1]...)
		} else {
			resume := len(text) + 1
			if newline := strings.IndexByte(text[layout.Rows[row].Start:], '\n'); newline >= 0 {
				resume = layout.Rows[row].Start + newline + 1
			}
			for at := layout.Rows[row].Start; at >= 0 && at < resume; {
				var wrapped gfx.TextRow
				wrapped, at = layout.canvas.WrapRow(text, at, layout.size, gfx.WeightRegular, width)
				layout.Rows = append(layout.Rows, wrapped)
				layout.exact = append(layout.exact, true)
			}
		}
		row = after + 1
	}
	layout.splice(first, last+1, end)
	if layout.LineHeight > 0 {
		layout.Scroll = float32(layout.rowAt(top))*layout.LineHeight + within
	}
	if layout.FollowedValid {
		layout.followedRow = layout.RowOf(layout.followed)
	}
	return true
}

// rowAt is the row that holds offset.
func (layout *TextareaLayout) rowAt(offset int) int {
	return max(sort.Search(len(layout.Rows), func(index int) bool { return layout.Rows[index].Start > offset })-1, 0)
}

// Show settles what the box paints after Fit: the rows in view are wrapped with the
// canvas, the text at the top of the box stays where it is as they change, and a caret
// that moved since the last paint is kept in view as Follow keeps it. Content and
// LineHeight are the box's for this paint.
func (layout *TextareaLayout) Show(caret int) {
	moved := !layout.FollowedValid || layout.followed != caret || layout.followedRow != layout.RowOf(caret)
	for {
		layout.Follow(caret, layout.RowOf(caret))
		if layout.LineHeight <= 0 {
			return
		}
		first := int(layout.Scroll / layout.LineHeight)
		last := int((layout.Scroll + layout.Content.Height) / layout.LineHeight)
		if !layout.realize(first, last) {
			return
		}
		if moved {
			layout.FollowedValid = false // the rows the caret was followed to have changed
		}
	}
}

// realizeUpTo wraps with the canvas the rows from the first until more than most are
// exact, or all of them for most < 0, and returns how many rows the text takes: exactly
// while that is no more than most.
func (layout *TextareaLayout) realizeUpTo(most int) int {
	if most < 0 {
		most = len(layout.Rows)
	}
	for layout.realize(0, most) {
	}
	return len(layout.Rows)
}

// commonPrefix is the length of the longest prefix a and b share, compared a block
// at a time so a long text costs a memory comparison, not a loop over its bytes.
func commonPrefix(a, b string) int {
	n := min(len(a), len(b))
	at := 0
	for block := 4096; at+block <= n && a[at:at+block] == b[at:at+block]; at += block {
	}
	for at < n && a[at] == b[at] {
		at++
	}
	return at
}

// commonSuffix is the length of the longest suffix a and b share.
func commonSuffix(a, b string) int {
	n := min(len(a), len(b))
	at := 0
	for block := 4096; at+block <= n && a[len(a)-at-block:len(a)-at] == b[len(b)-at-block:len(b)-at]; at += block {
	}
	for at < n && a[len(a)-at-1] == b[len(b)-at-1] {
		at++
	}
	return at
}

// rowOf is the row the caret at offset is drawn on.
func (layout *TextareaLayout) RowOf(offset int) int {
	row := sort.Search(len(layout.Rows), func(index int) bool { return layout.Rows[index].Start > offset }) - 1
	row = max(row, 0)
	if layout.Upstream && layout.UpstreamCaret == offset && row > 0 && layout.Rows[row-1].End == offset && layout.Rows[row].Start == offset {
		row--
	}
	return row
}

// next is the offset the row after row starts at, and false for the last row.
func (layout *TextareaLayout) next(row int) (int, bool) {
	if row+1 < len(layout.Rows) {
		return layout.Rows[row+1].Start, true
	}
	return 0, false
}

// shown is the text a row paints and measures: a row that wrapped does not show
// the spaces it left hanging.
func (layout *TextareaLayout) shown(row int) string {
	r := layout.Rows[row]
	line := layout.text[r.Start:r.End]
	if next, ok := layout.next(row); ok && next == r.End {
		line = trimTrailingSpaces(line)
	}
	return line
}

// rowEnd is the offset of the End key on a row, and whether it must be drawn at
// the end of this row though it is also the start of the next.
func (layout *TextareaLayout) rowEnd(row int) (int, bool) {
	r := layout.Rows[row]
	next, wrapped := layout.next(row)
	if !wrapped || next != r.End {
		return r.End, false
	}
	shown := len(layout.shown(row))
	return r.Start + shown, shown == r.End-r.Start
}

// columnOf is the distance from the left edge of the content to the caret.
func (layout *TextareaLayout) columnOf(row, offset int) float32 {
	r := layout.Rows[row]
	line := layout.shown(row)
	origin := layout.origin(line)
	local := textareaBoundary(line, min(max(offset-r.Start, 0), len(line)))
	return origin + layout.canvas.TextCaret(line, layout.size, gfx.WeightRegular, local) - layout.Content.X
}

// origin is the left end of a line as drawn in the content.
func (layout *TextareaLayout) origin(line string) float32 {
	return layout.canvas.LabelOrigin(line, gfx.Rect{X: layout.Content.X, Width: layout.Content.Width}, gfx.TextStyle{Size: layout.size, Weight: gfx.WeightRegular}, gfx.AlignStart)
}

// offsetInRow is the offset on row nearest to column x from the left edge of the
// content, and whether it is drawn at the end of the row.
func (layout *TextareaLayout) offsetInRow(row int, x float32) (int, bool) {
	r := layout.Rows[row]
	line := layout.shown(row)
	local := layout.canvas.TextOffsetAt(line, layout.size, gfx.WeightRegular, x+layout.Content.X-layout.origin(line))
	local = textareaBoundary(line, min(max(local, 0), len(line)))
	offset := r.Start + local
	next, wrapped := layout.next(row)
	return offset, wrapped && next == r.End && offset == r.End
}

func (layout *TextareaLayout) clampScroll(scroll float32) float32 {
	most := float32(len(layout.Rows))*layout.LineHeight - layout.Content.Height
	return textareaMax(0, min(scroll, most))
}

// follow scrolls the box to keep the row of caret in view when the caret, or the
// row it is on, is not where the last paint left it. A wheel or a drag of a
// scroll bar moves the box and the next paint leaves it there until then.
func (layout *TextareaLayout) Follow(caret, row int) {
	if !layout.FollowedValid || layout.followed != caret || layout.followedRow != row {
		top := float32(row) * layout.LineHeight
		if top < layout.Scroll {
			layout.Scroll = top
		} else if bottom := top + layout.LineHeight; bottom > layout.Scroll+layout.Content.Height {
			layout.Scroll = bottom - layout.Content.Height
		}
	}
	layout.followed, layout.followedRow, layout.FollowedValid = caret, row, true
	layout.Scroll = layout.clampScroll(layout.Scroll)
}

// The small text helpers the moved textarea types use, with the meaning they
// have in components.

func textareaRange(value string, start, end int) (int, int) {
	start = textareaBoundary(value, start)
	end = textareaBoundary(value, end)
	if start > end {
		start, end = end, start
	}
	return start, end
}

func textareaBoundary(value string, offset int) int {
	if offset <= 0 {
		return 0
	}
	if offset >= len(value) {
		return len(value)
	}
	for offset > 0 && value[offset]&0xc0 == 0x80 {
		offset--
	}
	return offset
}

func textareaMax(left, right float32) float32 {
	if left > right {
		return left
	}
	return right
}

const textareaPadY = 8

func trimTrailingSpaces(value string) string {
	for len(value) > 0 && value[len(value)-1] == ' ' {
		value = value[:len(value)-1]
	}
	return value
}

// TextareaPadY is the space above the first row and below the last (py-2).
const TextareaPadY = 8

// AutoGrow makes a TextArea as tall as the rows its text needs, from the
// control's 64 pixel minimum up to maxRows rows; past that the box stops
// growing and the rows scroll inside it. The page measures the rows at the width
// the box is laid out in and sizes the box before placement, so the text is not
// measured twice: the same TextareaLayout the paint uses keeps the rows.
//
//	ui.TextArea(c, &m.notes, "Notes", "").AutoGrow(5)
//
// An explicit Height still fixes the box, as a declared size wins over a
// measured one, and MaxHeight or MinHeight bound the measured height. It is for
// a TextArea of a page (one declared with a TextareaLayout); a box without the
// layout keeps its declared size.
func (b Box) AutoGrow(maxRows int) Box {
	n := b.N()
	if n.Kind != kindTextarea {
		panic("ui: AutoGrow is for a TextArea")
	}
	if maxRows <= 0 {
		panic("ui: AutoGrow needs at least one row")
	}
	// The rows a TextArea may take; a wrapped Text's line limit uses the same
	// field, and a TextArea never wraps as a declarative Text does.
	n.MaxLines = int32(maxRows)
	return b
}

// TextareaMovementForKey says which movement a key asks for in a multiline
// editor, and whether it extends the selection (Shift). Home and End, Cmd+Left
// and Cmd+Right are the ends of a row; Cmd+Up and Cmd+Down, Ctrl+Home and
// Ctrl+End the ends of the text. Left, Right and the editing keys stay with the
// text system. Pass the key event of the focused textarea to it, then the result
// to TextareaLayout.Move, then the new offset to the editor.
func TextareaMovementForKey(key gfx.Key, modifiers gfx.Modifiers) (movement TextareaMovement, extend bool) {
	extend = modifiers&gfx.ModifierShift != 0
	command := modifiers&gfx.ModifierMeta != 0
	control := modifiers&gfx.ModifierControl != 0
	switch {
	case key == gfx.KeyUp && command:
		return TextareaMoveTextStart, extend
	case key == gfx.KeyDown && command:
		return TextareaMoveTextEnd, extend
	case key == gfx.KeyUp:
		return TextareaMoveUp, extend
	case key == gfx.KeyDown:
		return TextareaMoveDown, extend
	case key == gfx.KeyLeft && command:
		return TextareaMoveLineStart, extend
	case key == gfx.KeyRight && command:
		return TextareaMoveLineEnd, extend
	case key == gfx.KeyHome && control:
		return TextareaMoveTextStart, extend
	case key == gfx.KeyEnd && control:
		return TextareaMoveTextEnd, extend
	case key == gfx.KeyHome:
		return TextareaMoveLineStart, extend
	case key == gfx.KeyEnd:
		return TextareaMoveLineEnd, extend
	case key == gfx.KeyPageUp:
		return TextareaMovePageUp, extend
	case key == gfx.KeyPageDown:
		return TextareaMovePageDown, extend
	}
	return TextareaMoveNone, false
}
