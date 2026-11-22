package platform

import "errors"

// ErrDragUnsupported reports that this target has no way to take files out of a
// window by dragging them: the application can still offer the files, but only
// by copying them (see SetClipboardFiles), which tells the user something else.
// The Web target has no drag of its own to start, macOS needs a dragging session
// and Linux a drag-and-drop protocol, and neither is bound yet.
var ErrDragUnsupported = errors.New("dragging files out of a window is unsupported")

// DragEffect is what letting go of dragged files does to them where they land: a
// copy leaves them where they were, a move hands them to the target, and a link
// leaves a reference behind. The values are the OLE DROPEFFECT flags Windows
// reports and asks for, so one value is also the mask of the effects an
// operation allows.
type DragEffect uint32

const (
	// DragEffectNone is what a drag that ended nowhere, or that the user
	// cancelled with Escape, reports.
	DragEffectNone DragEffect = 0
	// DragEffectCopy copies the files; the ones dragged out stay where they are.
	DragEffectCopy DragEffect = 1
	// DragEffectMove moves the files to where they land.
	DragEffectMove DragEffect = 2
	// DragEffectLink puts a link to the files where they land.
	DragEffectLink DragEffect = 4

	// dragEffects is every effect named here.
	dragEffects = DragEffectCopy | DragEffectMove | DragEffectLink
)

// DragEffectKnown reports whether every effect in a mask is one this package
// names, so that an operation asking for an effect no platform understands is
// refused instead of having the bits silently dropped.
func DragEffectKnown(mask DragEffect) bool { return mask&^dragEffects == 0 }
