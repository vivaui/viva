package codeeditor

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/window"
)

// Merge is one three-way merge's page control. Its center pane is the editable
// result, so keep it beside the application's text.Editor and point Text at it.
// The zero value is ready; one Merge serves one place on a page.
//
// Code keeps the component's layout and scroll caches across paints. Snapshot
// is the presentation painted next; the wrapper sets the editor snapshot's ID
// and Label, which the component uses for the editable result pane.
type Merge struct {
	// Code is the caller-owned merge view.
	Code components.CodeMerge
	// Snapshot is painted at every paint.
	Snapshot components.CodeMergeSnapshot
	// Key receives the keys pressed while the merge has focus.
	Key func(gfx.KeyEvent) gfx.EventResult
	// Pointer receives the pointer events the merge's scrollbars did not handle,
	// including presses on the current and incoming action regions.
	Pointer func(gfx.PointerEvent) gfx.EventResult
	// Scroll receives the wheel the merge did not scroll.
	Scroll func(gfx.ScrollEvent) gfx.EventResult
	// Text gives the platform's text service the application's editor for the
	// focused control. Nil disables native editing and input methods.
	Text func(window.TextInputContext) *window.TextInput

	handlers ui.Handlers
	drawFn   func(*ui.DrawContext)
	wired    bool
}

// MergeView declares the merge at c. Give it a size; it is a ui.Control with a
// text-field role, painted through the component. The wrapper sets the editor
// snapshot's ID and Label, and its scrollbar IDs when the snapshot enables them.
func MergeView(c ui.C, label string, merge *Merge) ui.Box {
	if merge == nil {
		panic("codeeditor: MergeView needs its Merge")
	}
	merge.prepare()
	return ui.Control(c, label, &merge.handlers).Draw(merge.drawFn)
}

// Parts is the scope of this control's own hit regions, for caller-owned actions.
// The scrollbars use At(0) and At(1) when the snapshot enables them. It is empty
// until MergeView has declared the control at least once.
func (m *Merge) Parts() gfx.Scope { return m.handlers.Parts() }

func (m *Merge) prepare() {
	if m.wired {
		return
	}
	m.wired = true
	m.handlers.Role = gfx.RoleTextField
	m.handlers.Key = m.key
	m.handlers.Pointer = m.pointer
	m.handlers.Scroll = m.scroll
	m.handlers.Text = m.text
	m.drawFn = m.draw
}

func (m *Merge) key(event gfx.KeyEvent) gfx.EventResult {
	if m.Key == nil {
		return gfx.EventResult{}
	}
	return m.Key(event)
}

func (m *Merge) pointer(event gfx.PointerEvent) gfx.EventResult {
	if result := m.Code.Pointer(event); result.Handled {
		return result
	}
	if m.Pointer == nil {
		return gfx.EventResult{}
	}
	return m.Pointer(event)
}

func (m *Merge) scroll(event gfx.ScrollEvent) gfx.EventResult {
	if result := m.Code.Scroll(event); result.Handled {
		return result
	}
	if m.Scroll == nil {
		return gfx.EventResult{}
	}
	return m.Scroll(event)
}

func (m *Merge) text(context window.TextInputContext) *window.TextInput {
	if m.Text == nil {
		return nil
	}
	return m.Text(context)
}

func (m *Merge) draw(context *ui.DrawContext) {
	snapshot := m.Snapshot
	snapshot.Editor.ID = context.Node.ID
	snapshot.Editor.Label = context.Label
	parts := m.handlers.Parts()
	if !snapshot.Editor.VerticalScrollbarID.IsZero() {
		snapshot.Editor.VerticalScrollbarID = parts.At(0)
	}
	if !snapshot.Editor.HorizontalScrollbarID.IsZero() {
		snapshot.Editor.HorizontalScrollbarID = parts.At(1)
	}
	m.Code.Paint(context.Canvas, *context.Theme, context.Rect, snapshot)
}
