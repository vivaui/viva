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
	p := c.GetPage()
	p.ForbidInMemo("Toasts")
	t.SetScope(p.BoundScope(PKToaster, unsafe.Pointer(t)))
	canvas := c.GetF().Surface
	t.SetActed(0)
	if canvas == nil || len(t.GetToasts()) == 0 {
		return
	}
	now := ClockEpoch.Add(canvas.Now())
	// A press on a close or an action button, delivered here.
	if id, ok := canvas.ClickTarget(); ok {
		if at, inside := id.In(t.GetScope()); inside && at%3 != 0 {
			if at%3 == ToastAction {
				t.SetActed(ToastID(at / 3))
			}
			t.Close(ToastID(at / 3))
		}
	}
	for {
		id, ok := t.QueuePtr().Expire(now)
		if !ok {
			break
		}
		at, _ := id.In(t.GetScope())
		t.Close(ToastID(at / 3))
	}
	for i := 0; i < len(t.GetToasts()); i++ {
		item := &t.GetToasts()[i]
		if item.GetShown() {
			continue
		}
		evicted, ok := t.QueuePtr().Add(t.ID(item.GetN()), now, item.Kind == ToastLoading)
		item.SetShown(ok)
		if at, inside := evicted.In(t.GetScope()); inside {
			// The queue was full: the oldest goes, which is ahead of this one.
			t.Close(ToastID(at / 3))
			i--
		}
	}
	// The pointer on the stack, or the focus in it, holds every toast's time.
	held := false
	if at, ok := canvas.Hovered().In(t.GetScope()); ok && at > 0 {
		held = true
	}
	if at, ok := canvas.Focused().In(t.GetScope()); ok && at > 0 {
		held = true
	}
	t.SetStack(t.GetStack()[:0])
	for i := len(t.GetToasts()) - 1; i >= 0; i-- {
		item := &t.GetToasts()[i]
		if held {
			t.QueuePtr().Pause(t.ID(item.GetN()), now)
		} else {
			t.QueuePtr().Resume(t.ID(item.GetN()), now)
		}
		toast := components.SonnerToast{
			Snapshot: components.ToastSnapshot{
				Title: item.GetTitle(), Description: item.GetDescription(), Type: toastKindComponent(item.Kind),
				SwipeX: t.SwipeX(ToastID(item.GetN())),
			},
			Scope: t.ID(item.GetN()), DismissID: t.GetScope().At(int(item.GetN())*3 + ToastClose),
		}
		if item.GetAction() != "" {
			toast.ActionID, toast.ActionLabel = t.GetScope().At(int(item.GetN())*3+ToastAction), item.GetAction()
		}
		t.SetStack(append(t.GetStack(), toast))
	}
	components.PaintSonner(canvas, c.GetF().Palette, c.GetF().GetBounds(), t.GetStack(), components.SonnerOptions{
		Layer: ToastLayer, Expanded: held || t.Expanded, Position: components.ToastPosition(t.Position),
	})
	if deadline, ok := t.QueuePtr().NextDeadline(); ok {
		canvas.RequestFrameIn(max(deadline.Sub(now), time.Millisecond))
	}
}

func toastKindComponent(Kind ToastKind) components.ToastType {
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
