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
// away after Timeout (five seconds when zero), or when its close button is
// pressed; the pointer resting on the stack, or the focus in it, keeps them up.
// Limit is how many are visible at once (three when zero). Swipe carries a
// caller's horizontal drag snapshot for one toast, and Expanded keeps the stack
// open for a page that cannot hold the pointer on it. The zero value is empty.
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
	// Expanded keeps the stack open, as the pointer resting on it or the focus
	// in it does: a caller-owned flag, for a static page whose stack cannot be
	// hovered. The zero value is collapsed.
	Expanded bool
	// Timeout is the lifetime of a toast that does not name one of its own; zero
	// is five seconds, as Base UI's.
	Timeout time.Duration
	// Limit is the most toasts visible at once; zero is three, as Base UI's.
	Limit int

	queue   ToastQueue
	toasts  []toast // oldest first
	next    uint32  // the number the next toast gets
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
	timeout            time.Duration
	hideClose          bool    // no close button: the toast leaves on its time or Close
	swipeX             float32 // the caller's horizontal snapshot offset, from Swipe
	shown              bool    // painted once: in the queue, with its time running
}

// ToastConfig is what one show call sets on its toast: an action button, a
// lifetime of its own, or no close button. The zero value is a toast with the
// Toaster's lifetime and its close button.
type ToastConfig struct {
	// Action labels the toast's button; none when empty.
	Action string
	// Timeout overrides the Toaster's Timeout for this toast; zero or less uses
	// the Toaster's.
	Timeout time.Duration
	// HideClose leaves the close button off. The toast then leaves when its time
	// is up, when its action is pressed, or when the application calls Close.
	HideClose bool
}

// ToastID names a toast for Close.
type ToastID uint32

// Show puts up a toast: a title, and a description under it ("" for none). The
// same title and description already up is not shown twice; its time starts again.
func (t *Toaster) Show(title, description string, Kind ToastKind) ToastID {
	return t.ShowConfig(title, description, Kind, ToastConfig{})
}

// ShowAction is Show with a button labeled action, which Acted reports and which
// takes the toast down: an Undo, a Retry, an Open. The action is not the only way
// to it: what the toast offers is somewhere in the window too, for the toast goes
// by itself.
func (t *Toaster) ShowAction(title, description string, Kind ToastKind, action string) ToastID {
	return t.ShowConfig(title, description, Kind, ToastConfig{Action: action})
}

// ShowConfig is Show with what this toast alone sets: an action button, its own
// lifetime, or no close button. A toast that hides its close button still goes
// by its time, by its action, or by Close.
func (t *Toaster) ShowConfig(title, description string, Kind ToastKind, config ToastConfig) ToastID {
	for i := range t.toasts {
		if item := &t.toasts[i]; item.title == title && item.description == description {
			item.Kind, item.action, item.timeout, item.hideClose, item.swipeX = Kind, config.Action, config.Timeout, config.HideClose, 0
			if item.shown {
				t.queue.Remove(t.ID(item.n))
				item.shown = false // the next paint queues it again, with a fresh time
			}
			return ToastID(item.n)
		}
	}
	t.next++
	t.toasts = append(t.toasts, toast{
		n: t.next, title: title, description: description, Kind: Kind,
		action: config.Action, timeout: config.Timeout, hideClose: config.HideClose,
	})
	return ToastID(t.next)
}

// Swipe moves toast id x pixels horizontally on this and later paints: the
// caller's gesture snapshot, which ui.Toasts paints as it is. Positive moves it
// right; zero puts it back. It reads no pointer, interprets no gesture and
// schedules no frame; the caller owns the drag and its threshold. Nothing
// happens when the toast is not up.
func (t *Toaster) Swipe(id ToastID, x float32) {
	for i := range t.toasts {
		if t.toasts[i].n == uint32(id) {
			t.toasts[i].swipeX = x
			return
		}
	}
}

// SwipeX is how far Swipe moves toast id on the next paint, or zero when the
// toast is not up.
func (t *Toaster) SwipeX(id ToastID) float32 {
	for i := range t.toasts {
		if t.toasts[i].n == uint32(id) {
			return t.toasts[i].swipeX
		}
	}
	return 0
}

// Done replaces toast id, a ToastLoading one, with what it ended in: its title,
// description and kind now, and its time running from now.
func (t *Toaster) Done(ID ToastID, title, description string, Kind ToastKind) {
	for i := range t.toasts {
		if item := &t.toasts[i]; item.n == uint32(ID) {
			item.title, item.description, item.Kind, item.swipeX = title, description, Kind, 0
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
