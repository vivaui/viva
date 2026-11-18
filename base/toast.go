package base

import (
	"time"

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

	queue   ToastQueue
	toasts  []toast // oldest first
	Next    uint32
	scope   gfx.Scope
	stack   []SonnerToast // scratch for one paint, newest first
	entries []ToastQueueEntry
	acted   ToastID // the toast whose action this paint delivered, or 0
}

type toast struct {
	n                  uint32 // its number: ToastID, and its identity in the Toaster's scope
	title, description string
	Kind               ToastKind
	action             string // the label of its button; none when empty
	shown              bool   // painted once: in the queue, with its time running
}

// ToastID names a toast for Close.
type ToastID uint32

// Show puts up a toast: a title, and a description under it ("" for none). The
// same title and description already up is not shown twice; its time starts again.
func (t *Toaster) Show(title, description string, Kind ToastKind) ToastID {
	return t.ShowAction(title, description, Kind, "")
}

// ShowAction is Show with a button labeled action, which Acted reports and which
// takes the toast down: an Undo, a Retry, an Open. The action is not the only way
// to it: what the toast offers is somewhere in the window too, for the toast goes
// by itself.
func (t *Toaster) ShowAction(title, description string, Kind ToastKind, action string) ToastID {
	for i := range t.toasts {
		if item := &t.toasts[i]; item.title == title && item.description == description {
			item.Kind, item.action = Kind, action
			if item.shown {
				t.queue.Remove(t.ID(item.n))
				item.shown = false // the next paint queues it again, with a fresh time
			}
			return ToastID(item.n)
		}
	}
	t.Next++
	t.toasts = append(t.toasts, toast{n: t.Next, title: title, description: description, Kind: Kind, action: action})
	return ToastID(t.Next)
}

// Done replaces toast id, a ToastLoading one, with what it ended in: its title,
// description and kind now, and its time running from now.
func (t *Toaster) Done(ID ToastID, title, description string, Kind ToastKind) {
	for i := range t.toasts {
		if item := &t.toasts[i]; item.n == uint32(ID) {
			item.title, item.description, item.Kind = title, description, Kind
			if item.shown {
				t.queue.Remove(t.ID(item.n))
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
func (t *Toaster) Close(ID ToastID) {
	for i := range t.toasts {
		if t.toasts[i].n == uint32(ID) {
			t.queue.Remove(t.ID(t.toasts[i].n))
			t.toasts = append(t.toasts[:i], t.toasts[i+1:]...)
			return
		}
	}
}

// Len is how many toasts are up.
func (t *Toaster) Len() int { return len(t.toasts) }

// id is the identity of toast n; zero before the Toaster was first painted.
func (t *Toaster) ID(n uint32) gfx.ID {
	if t.scope == 0 {
		return gfx.ID{}
	}
	return t.scope.At(int(n) * 3)
}

// A toast's identities in the Toaster's scope, after its own (n*3): its close button
// and its action.
const (
	ToastClose  = 1
	ToastAction = 2
)

// clockEpoch turns the canvas's monotonic time into the times the components keep,
// a toast queue's or a typeahead's: only their differences matter.
var ClockEpoch = time.Unix(0, 0)
