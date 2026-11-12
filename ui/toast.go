package ui

import (
	"time"
	"unsafe"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
)

// ToastKind is the icon of a toast, and what it tells assistive technology.
type ToastKind uint8

const (
	ToastDefault ToastKind = iota
	ToastSuccess
	ToastInfo
	ToastWarning
	ToastError
	// ToastLoading spins, and the toast does not go by itself: something under way,
	// which Show or Done replaces when it ends.
	ToastLoading
)

// ToastPosition is where a Toaster's stack sits; the zero value is the bottom right.
type ToastPosition uint8

const (
	ToastBottomRight ToastPosition = iota
	ToastBottomLeft
	ToastBottomCenter
	ToastTopRight
	ToastTopLeft
	ToastTopCenter
)

// ToastLayer is the canvas layer toasts are painted on: over the page and its
// overlays, under a Tooltip.
const ToastLayer = TooltipLayer - 8

// Toaster is the toasts of a window, kept in the model, as shadcn's Sonner: Show
// puts one up, and ui.Toasts, declared once in the page, paints them. A toast goes
// away five seconds after it appears, or when its close button is pressed; the
// pointer resting on the stack, or the focus in it, keeps them up. The zero value
// is empty.
//
//	type model struct{ toaster ui.Toaster }
//
//	if ui.Button(c, "Save").Clicked() {
//		m.save()
//		m.toaster.Show("Saved", "", ui.ToastSuccess)
//	}
//	ui.Toasts(c, &m.toaster)
type Toaster struct {
	// Position is where the stack sits; the bottom right when zero.
	Position ToastPosition

	queue   components.ToastQueue
	toasts  []toast // oldest first
	next    uint32
	scope   gfx.Scope
	stack   []components.SonnerToast // scratch for one paint, newest first
	entries []components.ToastQueueEntry
	acted   ToastID // the toast whose action this paint delivered, or 0
}

type toast struct {
	n                  uint32 // its number: ToastID, and its identity in the Toaster's scope
	title, description string
	kind               ToastKind
	action             string // the label of its button; none when empty
	shown              bool   // painted once: in the queue, with its time running
}

// ToastID names a toast for Close.
type ToastID uint32

// Show puts up a toast: a title, and a description under it ("" for none). The
// same title and description already up is not shown twice; its time starts again.
func (t *Toaster) Show(title, description string, kind ToastKind) ToastID {
	return t.ShowAction(title, description, kind, "")
}

// ShowAction is Show with a button labeled action, which Acted reports and which
// takes the toast down: an Undo, a Retry, an Open. The action is not the only way
// to it: what the toast offers is somewhere in the window too, for the toast goes
// by itself.
func (t *Toaster) ShowAction(title, description string, kind ToastKind, action string) ToastID {
	for i := range t.toasts {
		if item := &t.toasts[i]; item.title == title && item.description == description {
			item.kind, item.action = kind, action
			if item.shown {
				t.queue.Remove(t.id(item.n))
				item.shown = false // the next paint queues it again, with a fresh time
			}
			return ToastID(item.n)
		}
	}
	t.next++
	t.toasts = append(t.toasts, toast{n: t.next, title: title, description: description, kind: kind, action: action})
	return ToastID(t.next)
}

// Done replaces toast id, a ToastLoading one, with what it ended in: its title,
// description and kind now, and its time running from now.
func (t *Toaster) Done(id ToastID, title, description string, kind ToastKind) {
	for i := range t.toasts {
		if item := &t.toasts[i]; item.n == uint32(id) {
			item.title, item.description, item.kind = title, description, kind
			if item.shown {
				t.queue.Remove(t.id(item.n))
				item.shown = false
			}
			return
		}
	}
}

// Acted reports the toast whose action button this paint delivers a press on; read
// it after ui.Toasts.
func (t *Toaster) Acted() (ToastID, bool) { return t.acted, t.acted != 0 }

// Close takes a toast down before its time.
func (t *Toaster) Close(id ToastID) {
	for i := range t.toasts {
		if t.toasts[i].n == uint32(id) {
			t.queue.Remove(t.id(t.toasts[i].n))
			t.toasts = append(t.toasts[:i], t.toasts[i+1:]...)
			return
		}
	}
}

// Len is how many toasts are up.
func (t *Toaster) Len() int { return len(t.toasts) }

// id is the identity of toast n; zero before the Toaster was first painted.
func (t *Toaster) id(n uint32) gfx.ID {
	if t.scope == 0 {
		return gfx.ID{}
	}
	return t.scope.At(int(n) * 3)
}

// A toast's identities in the Toaster's scope, after its own (n*3): its close button
// and its action.
const (
	toastClose  = 1
	toastAction = 2
)

// clockEpoch turns the canvas's monotonic time into the times the components keep,
// a toast queue's or a typeahead's: only their differences matter.
var clockEpoch = time.Unix(0, 0)

// Toasts paints the toasts of t where t.Position says, over everything
// else, and takes them down as their time is up. It asks for one frame when the
// next is due and none while there is nothing up. Declare it once, last in the page.
func Toasts(c C, t *Toaster) {
	p := c.page
	p.forbidInMemo("Toasts")
	t.scope = p.boundScope(pkToaster, unsafe.Pointer(t))
	canvas := c.f.canvas
	t.acted = 0
	if canvas == nil || len(t.toasts) == 0 {
		return
	}
	now := clockEpoch.Add(canvas.Now())
	// A press on a close or an action button, delivered here.
	if id, ok := canvas.ClickTarget(); ok {
		if at, inside := id.In(t.scope); inside && at%3 != 0 {
			if at%3 == toastAction {
				t.acted = ToastID(at / 3)
			}
			t.Close(ToastID(at / 3))
		}
	}
	for {
		id, ok := t.queue.Expire(now)
		if !ok {
			break
		}
		at, _ := id.In(t.scope)
		t.Close(ToastID(at / 3))
	}
	for i := 0; i < len(t.toasts); i++ {
		item := &t.toasts[i]
		if item.shown {
			continue
		}
		evicted, ok := t.queue.Add(t.id(item.n), now, item.kind == ToastLoading)
		item.shown = ok
		if at, inside := evicted.In(t.scope); inside {
			// The queue was full: the oldest goes, which is ahead of this one.
			t.Close(ToastID(at / 3))
			i--
		}
	}
	// The pointer on the stack, or the focus in it, holds every toast's time.
	held := false
	if at, ok := canvas.Hovered().In(t.scope); ok && at > 0 {
		held = true
	}
	if at, ok := canvas.Focused().In(t.scope); ok && at > 0 {
		held = true
	}
	t.stack = t.stack[:0]
	for i := len(t.toasts) - 1; i >= 0; i-- {
		item := &t.toasts[i]
		if held {
			t.queue.Pause(t.id(item.n), now)
		} else {
			t.queue.Resume(t.id(item.n), now)
		}
		toast := components.SonnerToast{
			Snapshot: components.ToastSnapshot{Title: item.title, Description: item.description, Type: item.kind.component()},
			Scope:    t.id(item.n), DismissID: t.scope.At(int(item.n)*3 + toastClose),
		}
		if item.action != "" {
			toast.ActionID, toast.ActionLabel = t.scope.At(int(item.n)*3+toastAction), item.action
		}
		t.stack = append(t.stack, toast)
	}
	components.PaintSonner(canvas, c.f.theme, c.f.bounds, t.stack, components.SonnerOptions{
		Layer: ToastLayer, Expanded: held, Position: components.ToastPosition(t.Position),
	})
	if deadline, ok := t.queue.NextDeadline(); ok {
		canvas.RequestFrameIn(max(deadline.Sub(now), time.Millisecond))
	}
}

func (kind ToastKind) component() components.ToastType {
	switch kind {
	case ToastSuccess:
		return components.ToastSuccess
	case ToastInfo:
		return components.ToastInfo
	case ToastWarning:
		return components.ToastWarning
	case ToastError:
		return components.ToastError
	case ToastLoading:
		return components.ToastLoading
	}
	return components.ToastDefault
}
