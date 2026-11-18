package ui

import (
	"time"
	"unsafe"

	"github.com/vivaui/viva/components"
)

// Toasts paints the toasts of t where t.Position says, over everything
// else, and takes them down as their time is up. It asks for one frame when the
// next is due and none while there is nothing up. Declare it once, last in the page.
func Toasts(c C, t *Toaster) {
	p := c.page
	p.ForbidInMemo("Toasts")
	t.scope = p.BoundScope(pkToaster, unsafe.Pointer(t))
	canvas := c.f.Surface
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
		evicted, ok := t.queue.Add(t.ID(item.n), now, item.Kind == ToastLoading)
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
			t.queue.Pause(t.ID(item.n), now)
		} else {
			t.queue.Resume(t.ID(item.n), now)
		}
		toast := components.SonnerToast{
			Snapshot: components.ToastSnapshot{Title: item.title, Description: item.description, Type: item.Kind.component()},
			Scope:    t.ID(item.n), DismissID: t.scope.At(int(item.n)*3 + toastClose),
		}
		if item.action != "" {
			toast.ActionID, toast.ActionLabel = t.scope.At(int(item.n)*3+toastAction), item.action
		}
		t.stack = append(t.stack, toast)
	}
	components.PaintSonner(canvas, c.f.Palette, c.f.bounds, t.stack, components.SonnerOptions{
		Layer: ToastLayer, Expanded: held, Position: components.ToastPosition(t.Position),
	})
	if deadline, ok := t.queue.NextDeadline(); ok {
		canvas.RequestFrameIn(max(deadline.Sub(now), time.Millisecond))
	}
}

func (Kind ToastKind) component() components.ToastType {
	switch Kind {
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
