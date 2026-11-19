package codeeditor

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/window"
)

// Diff is one code diff's page control. Its right pane is the editable source,
// so keep it beside the application's text.Editor and point Text at it. The
// zero value is ready; one Diff serves one place on a page. Keep it where
// DiffView finds it: its stable handlers and draw function point at this value,
// so do not copy a Diff after it has been declared.
//
// Code keeps the component's layout and scroll caches across paints. Snapshot
// is the presentation painted next; the wrapper sets its ID and Label.
type Diff struct {
	// Code is the caller-owned diff view.
	Code components.CodeDiff
	// Snapshot is painted at every paint.
	Snapshot components.CodeDiffSnapshot
	// Key receives the keys pressed while the diff has focus.
	Key func(gfx.KeyEvent) gfx.EventResult
	// Pointer receives the pointer events the diff's scrollbars did not handle,
	// including presses on gutter regions.
	Pointer func(gfx.PointerEvent) gfx.EventResult
	// Scroll receives the wheel the diff did not scroll.
	Scroll func(gfx.ScrollEvent) gfx.EventResult
	// Text gives the platform's text service the application's editor for the
	// focused control. Nil disables native editing and input methods.
	Text func(window.TextInputContext) *window.TextInput

	handlers ui.Handlers
	drawFn   func(*ui.DrawContext)
	wired    bool
}

// DiffView declares the diff at c. Give it a size; it is a ui.Control with a
// text-field role, painted through the component. The wrapper sets the
// snapshot's ID and Label, and the scrollbar IDs of a snapshot that enables
// them.
func DiffView(c ui.C, label string, diff *Diff) ui.Box {
	if diff == nil {
		panic("codeeditor: DiffView needs its Diff")
	}
	diff.prepare()
	return ui.Control(c, label, &diff.handlers).Draw(diff.drawFn)
}

// Parts is the scope of this control's own hit regions, for caller-owned gutter
// actions. The scrollbars use At(0) and At(1) when the snapshot enables them.
// It is empty until DiffView has declared the control at least once.
func (d *Diff) Parts() gfx.Scope { return d.handlers.Parts() }

func (d *Diff) prepare() {
	if d.wired {
		return
	}
	d.wired = true
	d.handlers.Role = gfx.RoleTextField
	d.handlers.Key = d.key
	d.handlers.Pointer = d.pointer
	d.handlers.Scroll = d.scroll
	d.handlers.Text = d.text
	d.drawFn = d.draw
}

func (d *Diff) key(event gfx.KeyEvent) gfx.EventResult {
	if d.Key == nil {
		return gfx.EventResult{}
	}
	return d.Key(event)
}

func (d *Diff) pointer(event gfx.PointerEvent) gfx.EventResult {
	if result := d.Code.Pointer(event); result.Handled {
		return result
	}
	if d.Pointer == nil {
		return gfx.EventResult{}
	}
	return d.Pointer(event)
}

func (d *Diff) scroll(event gfx.ScrollEvent) gfx.EventResult {
	if result := d.Code.Scroll(event); result.Handled {
		return result
	}
	if d.Scroll == nil {
		return gfx.EventResult{}
	}
	return d.Scroll(event)
}

func (d *Diff) text(context window.TextInputContext) *window.TextInput {
	if d.Text == nil {
		return nil
	}
	return d.Text(context)
}

func (d *Diff) draw(context *ui.DrawContext) {
	snapshot := d.Snapshot
	snapshot.ID = context.Node.ID
	snapshot.Label = context.Label
	parts := d.handlers.Parts()
	if !snapshot.VerticalScrollbarID.IsZero() {
		snapshot.VerticalScrollbarID = parts.At(0)
	}
	if !snapshot.HorizontalScrollbarID.IsZero() {
		snapshot.HorizontalScrollbarID = parts.At(1)
	}
	d.Code.Paint(context.Canvas, *context.Theme, context.Rect, snapshot)
}
