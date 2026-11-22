package codeeditor

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/window"
)

// Shown is a nonzero identity for a FindBar snapshot field whose only meaning is
// that the bar shows its control. FindBarView maps every nonzero snapshot ID to
// one of its own parts, so the value itself is not kept; use this where the
// caller has no identity of its own to name the control with.
var Shown = gfx.Item(1)

// The part of each FindBar control, in the order FindBarView maps a snapshot's
// IDs to Parts().At(i). A pointer event aimed at one carries that identity as
// its Target, and the numbers stay the same when another control is left out.
const (
	PartFind = iota
	PartReplacement
	PartPrevious
	PartNext
	PartClose
	PartMatchCase
	PartWholeWord
	PartRegexp
	PartToggleReplace
	PartReplaceOne
	PartReplaceAll
)

// FindBar is one find and replace bar's page control. The zero value is ready;
// one FindBar serves one place on a page. Keep it where FindBarView finds it: its
// stable handlers and draw function point at this value, so do not copy a
// FindBar after it has been declared.
//
// Snapshot is the presentation painted next. A nonzero ID in it shows its
// control; FindBarView replaces each with its own part, so an application reads
// a click from Parts().At(PartNext), Parts().At(PartClose) and so on rather than
// from the ID it assigned.
type FindBar struct {
	// Snapshot is painted at every paint. Update it when the query, options,
	// status or available controls change.
	Snapshot components.FindBarSnapshot
	// Key receives the keys pressed while the bar or one of its parts has focus.
	// See components.FindBarCommandForKey for the bar's own commands.
	Key func(gfx.KeyEvent) gfx.EventResult
	// Pointer receives the pointer events aimed at the bar or one of its parts.
	Pointer func(gfx.PointerEvent) gfx.EventResult
	// Scroll receives the wheel over the bar.
	Scroll func(gfx.ScrollEvent) gfx.EventResult
	// Text gives the platform's text service the application's editor for the
	// focused query or replacement field. Nil disables native editing.
	Text func(window.TextInputContext) *window.TextInput

	handlers ui.Handlers
	drawFn   func(*ui.DrawContext)
	wired    bool
}

// FindBarView declares the bar at c, sized to components.FindBarHeight. Give it
// a width. The bar paints itself as a floating surface and declares the hits of
// its inputs and buttons; the wrapper owns the control's identity and input
// routing, so a call site never makes a components control itself.
func FindBarView(c ui.C, label string, bar *FindBar) ui.Box {
	if bar == nil {
		panic("codeeditor: FindBarView needs its FindBar")
	}
	bar.prepare()
	return ui.Control(c, label, &bar.handlers).
		Height(components.FindBarHeight(*c.Theme(), bar.Snapshot.Replacing)).
		Draw(bar.drawFn)
}

// Parts is the scope of the bar's own hit regions: its inputs and its buttons,
// named by the Part constants. It is empty until FindBarView has declared the
// control at least once.
func (b *FindBar) Parts() gfx.Scope { return b.handlers.Parts() }

func (b *FindBar) prepare() {
	if b.wired {
		return
	}
	b.wired = true
	b.handlers.Role = gfx.RoleGroup
	b.handlers.Key = b.key
	b.handlers.Pointer = b.pointer
	b.handlers.Scroll = b.scroll
	b.handlers.Text = b.text
	b.drawFn = b.draw
}

func (b *FindBar) key(event gfx.KeyEvent) gfx.EventResult {
	if b.Key == nil {
		return gfx.EventResult{}
	}
	return b.Key(event)
}

func (b *FindBar) pointer(event gfx.PointerEvent) gfx.EventResult {
	if b.Pointer == nil {
		return gfx.EventResult{}
	}
	return b.Pointer(event)
}

func (b *FindBar) scroll(event gfx.ScrollEvent) gfx.EventResult {
	if b.Scroll == nil {
		return gfx.EventResult{}
	}
	return b.Scroll(event)
}

func (b *FindBar) text(context window.TextInputContext) *window.TextInput {
	if b.Text == nil {
		return nil
	}
	return b.Text(context)
}

func (b *FindBar) draw(context *ui.DrawContext) {
	snapshot := b.Snapshot
	parts := b.handlers.Parts()
	snapshot.Find.ID = part(snapshot.Find.ID, parts.At(PartFind))
	snapshot.Replacement.ID = part(snapshot.Replacement.ID, parts.At(PartReplacement))
	snapshot.PreviousID = part(snapshot.PreviousID, parts.At(PartPrevious))
	snapshot.NextID = part(snapshot.NextID, parts.At(PartNext))
	snapshot.CloseID = part(snapshot.CloseID, parts.At(PartClose))
	snapshot.MatchCaseID = part(snapshot.MatchCaseID, parts.At(PartMatchCase))
	snapshot.WholeWordID = part(snapshot.WholeWordID, parts.At(PartWholeWord))
	snapshot.RegexpID = part(snapshot.RegexpID, parts.At(PartRegexp))
	snapshot.ToggleReplaceID = part(snapshot.ToggleReplaceID, parts.At(PartToggleReplace))
	snapshot.ReplaceOneID = part(snapshot.ReplaceOneID, parts.At(PartReplaceOne))
	snapshot.ReplaceAllID = part(snapshot.ReplaceAllID, parts.At(PartReplaceAll))
	components.PaintFindBar(context.Canvas, *context.Theme, context.Rect, snapshot)
}

// part maps a caller's nonzero identity, which only marks the control as shown,
// to the part this control declares for it. A zero identity leaves it out.
func part(id, mapped gfx.ID) gfx.ID {
	if id.IsZero() {
		return gfx.ID{}
	}
	return mapped
}
