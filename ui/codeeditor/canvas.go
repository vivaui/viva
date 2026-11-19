package codeeditor

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/window"
)

// Canvas is a page canvas of the application's own when no typed wrapper fits:
// the caller paints it and takes its input through callbacks, while the wrapper
// owns the control's stable handlers and identity. The zero value is ready; one
// Canvas serves one place on a page.
//
// Draw runs with the control's ui.DrawContext, whose Node.ID is the control's
// hit, so a canvas that declares hits of its own can name them in Parts. Keep
// Draw a function made once (a method value stored on the model), not a closure
// made in the page function, or a warm paint allocates.
type Canvas struct {
	// Role is what assistive technology takes the control for. Zero is a group;
	// RoleTextField or RoleTerminal give typed text to Text.
	Role gfx.Role
	// Draw paints the canvas. Nil paints nothing.
	Draw func(*ui.DrawContext)
	// Key receives the keys pressed while the canvas has focus.
	Key func(gfx.KeyEvent) gfx.EventResult
	// Pointer receives the pointer events addressed to the canvas.
	Pointer func(gfx.PointerEvent) gfx.EventResult
	// Scroll receives the wheel over the canvas.
	Scroll func(gfx.ScrollEvent) gfx.EventResult
	// Text gives the platform's text service the canvas's editor while it has the
	// focus, for a RoleTextField or RoleTerminal canvas.
	Text func(window.TextInputContext) *window.TextInput

	handlers ui.Handlers
	drawFn   func(*ui.DrawContext)
	wired    bool
}

// CanvasView declares the canvas at c. Give it a size; it is a ui.Control whose
// Draw is the caller's, with the caller's callbacks routing its input.
func CanvasView(c ui.C, label string, canvas *Canvas) ui.Box {
	if canvas == nil {
		panic("codeeditor: CanvasView needs its Canvas")
	}
	canvas.prepare()
	return ui.Control(c, label, &canvas.handlers).Draw(canvas.drawFn)
}

// Parts is the scope of this control's own hit regions. A hit the caller's Draw
// declares may name itself in it so the canvas receives its pointer events. It
// is empty until CanvasView has declared the control at least once.
func (c *Canvas) Parts() gfx.Scope { return c.handlers.Parts() }

func (c *Canvas) prepare() {
	if !c.wired {
		c.wired = true
		c.handlers.Key = c.key
		c.handlers.Pointer = c.pointer
		c.handlers.Scroll = c.scroll
		c.handlers.Text = c.text
		c.drawFn = c.draw
	}
	c.handlers.Role = c.Role
}

func (c *Canvas) key(event gfx.KeyEvent) gfx.EventResult {
	if c.Key == nil {
		return gfx.EventResult{}
	}
	return c.Key(event)
}

func (c *Canvas) pointer(event gfx.PointerEvent) gfx.EventResult {
	if c.Pointer == nil {
		return gfx.EventResult{}
	}
	return c.Pointer(event)
}

func (c *Canvas) scroll(event gfx.ScrollEvent) gfx.EventResult {
	if c.Scroll == nil {
		return gfx.EventResult{}
	}
	return c.Scroll(event)
}

func (c *Canvas) text(context window.TextInputContext) *window.TextInput {
	if c.Text == nil {
		return nil
	}
	return c.Text(context)
}

func (c *Canvas) draw(d *ui.DrawContext) {
	if c.Draw != nil {
		c.Draw(d)
	}
}
