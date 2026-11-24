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
	// FileDragNone names no effect: a target told it may take nothing takes
	// nothing.
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

// FilePromise is one item of a drag whose content the application produces when
// the target asks for it, instead of a path that already exists on this
// computer. Dragging a file off a remote server is this: the file is downloaded
// where the user lets go, while the target copies, so nothing is fetched when
// the drag starts.
//
// Name is one path component, unique among its siblings. A file's Open returns
// its bytes when the target reads them; a directory's List returns its children
// on demand, each named relative to it. Both callbacks may run on a goroutine
// other than the one that started the drag, and may run more than once.
type FilePromise = platform.FilePromise

// StartFileDrag takes paths out of the application's window, as dragging a file
// from a file manager to another window does. The drag lasts until the pointer
// is released or the user presses Escape; the effect the target takes is the
// platform's own and is not reported. An error wrapping ErrUnsupported means
// this target cannot drag files out of a window at all, and the application
// should offer them some other way (SetClipboardFiles tells the user something
// else, a copy).
//
// allowed is the mask of effects the application offers: a target takes at most
// one of them, and only those. A target that claims one the application never
// offered counts as taking nothing.
//
// The files are not read here, and nothing is copied when the drag starts: a
// target that takes a copy or a move reads the paths while the drop happens, so
// the files must still be where they are when the user lets go. The paths must be
// absolute and free of NUL, and at most MaxDragFiles are accepted.
//
// Windows starts an OLE drag (IDropSource and IDataObject, DoDragDrop), so it is
// called on the event-loop thread, from the handler of the pointer event that
// starts the drag, and does not return until the drag has ended; the drag's own
// message loop dispatches the messages that arrive meanwhile, so a handler can
// be reentered by one. macOS starts an NSDraggingSession and Linux an XDND drag
// and returns once it is running; both take the pointer over from the press. The
// Web target has no DOM DataTransfer behind a canvas, and returns an error
// wrapping ErrUnsupported.
func StartFileDrag(paths []string, allowed FileDragEffect) error {
	if len(paths) == 0 {
		return fmt.Errorf("app: no files to drag out of the window")
	}
	if len(paths) > MaxDragFiles {
		return fmt.Errorf("app: a drag takes at most %d files", MaxDragFiles)
	}
	if allowed == FileDragNone || !platform.DragEffectKnown(allowed) {
		return fmt.Errorf("app: a drag of files offers FileDragCopy, FileDragMove or FileDragLink")
	}
	for index, path := range paths {
		if path == "" || strings.IndexByte(path, 0) >= 0 || !filepath.IsAbs(path) {
			return fmt.Errorf("app: dragged file %d is not an absolute path: %q", index, path)
		}
	}
	if err, handled := driverStartFileDrag(paths, allowed); handled {
		return err
	}
	err := nativeStartFileDrag(paths, allowed)
	if errors.Is(err, platform.ErrDragUnsupported) {
		return fmt.Errorf("%w: %v", ErrUnsupported, err)
	}
	return err
}

// StartFilePromiseDrag takes files out of the application's window that do not
// exist as local paths yet: each is produced when the target asks for it, by
// Open for a file or by List for a directory, and written where the user lets
// go. A target takes a copy only, so nothing is moved or deleted at the source.
//
// It is called on the event-loop thread, from the handler of the pointer event
// that starts the drag, like StartFileDrag. Windows enumerates the whole tree
// before the drag starts and blocks until the drop ends; macOS and Linux return
// once the drag is running, and may call Open and List after it ends. An error
// wrapping ErrUnsupported means this target cannot take files out of a window.
// A tree of more than MaxDragFiles items is refused.
func StartFilePromiseDrag(files []FilePromise) error {
	if len(files) == 0 {
		return fmt.Errorf("app: no files to drag out of the window")
	}
	if err := platform.ValidateFilePromises(files); err != nil {
		return err
	}
	if err, handled := driverStartFilePromiseDrag(files); handled {
		return err
	}
	err := nativeStartFilePromiseDrag(files)
	if errors.Is(err, platform.ErrDragUnsupported) {
		return fmt.Errorf("%w: %v", ErrUnsupported, err)
	}
	return err
}
