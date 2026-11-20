package app

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/vivaui/viva/internal/platform"
)

// FileDragEffect is what letting go of dragged files does to them where they
// land: a copy leaves them where they were, a move hands them to the target, and
// a link leaves a reference to them behind. The values are the OLE DROPEFFECT
// flags Windows reports and asks for, so one value is also the mask of the
// effects a drag offers.
type FileDragEffect = platform.DragEffect

const (
	// FileDragNone is what a drag that ended nowhere, or that the user
	// cancelled, reports.
	FileDragNone FileDragEffect = platform.DragEffectNone
	// FileDragCopy copies the files; the ones dragged out stay where they are.
	FileDragCopy FileDragEffect = platform.DragEffectCopy
	// FileDragMove moves the files to where they land.
	FileDragMove FileDragEffect = platform.DragEffectMove
	// FileDragLink puts a link to the files where they land.
	FileDragLink FileDragEffect = platform.DragEffectLink
)

// MaxDragFiles is the most paths one drag takes; a longer list is an error
// rather than a silent cut. It is the bound a clipboard read uses too, and for
// the same reason: it limits what a huge selection can make an application hold.
const MaxDragFiles = platform.MaxClipboardFiles

// StartFileDrag takes paths out of the application's window, as dragging a file
// from a file manager to another window does. The drag lasts until the pointer
// is released or the user presses Escape, and the effect the target took is
// returned — FileDragNone when it took nothing, which is also what a cancelled
// drag reports. An error wrapping ErrUnsupported means this target cannot drag
// files out of a window at all, and the application should offer them some other
// way (SetClipboardFiles tells the user something else, a copy).
//
// allowed is the mask of effects the application offers: a target takes at most
// one of them, and only those. Windows reports the effect the drop performed,
// and a target that claims one the application never offered counts as taking
// nothing.
//
// The files are not read here, and nothing is copied when the drag starts: a
// target that takes a copy or a move reads the paths while the drop happens, so
// the files must still be where they are when the user lets go. The paths must be
// absolute, local and free of NUL, and at most MaxDragFiles are accepted.
//
// Windows starts an OLE drag (IDropSource and IDataObject, DoDragDrop), so it is
// called on the event-loop thread, from the handler of the pointer event that
// starts the drag, and does not return until the drag has ended; the drag's own
// message loop dispatches the messages that arrive meanwhile, so a handler can
// be reentered by one. macOS needs an NSDraggingSession and Linux the XDND
// protocol as a source, and neither adapter binds them yet; the Web target has
// no DOM DataTransfer behind a canvas. Those three return an error wrapping
// ErrUnsupported.
func StartFileDrag(paths []string, allowed FileDragEffect) (FileDragEffect, error) {
	if len(paths) == 0 {
		return FileDragNone, fmt.Errorf("app: no files to drag out of the window")
	}
	if len(paths) > MaxDragFiles {
		return FileDragNone, fmt.Errorf("app: a drag takes at most %d files", MaxDragFiles)
	}
	if allowed == FileDragNone || !platform.DragEffectKnown(allowed) {
		return FileDragNone, fmt.Errorf("app: a drag of files offers FileDragCopy, FileDragMove or FileDragLink")
	}
	for index, path := range paths {
		if path == "" || strings.IndexByte(path, 0) >= 0 || !filepath.IsAbs(path) {
			return FileDragNone, fmt.Errorf("app: dragged file %d is not an absolute path: %q", index, path)
		}
	}
	if effect, err, handled := driverStartFileDrag(paths, allowed); handled {
		return effect, err
	}
	effect, err := nativeStartFileDrag(paths, allowed)
	if errors.Is(err, platform.ErrDragUnsupported) {
		return FileDragNone, fmt.Errorf("%w: %v", ErrUnsupported, err)
	}
	return effect, err
}
