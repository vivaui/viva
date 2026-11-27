package app

import (
	"fmt"

	"github.com/vivaui/viva/internal/platform"
)

// SetClipboardFilePromises puts files on the system clipboard that do not exist
// on this computer yet: their content is produced by their Open callbacks when
// the user pastes them, and nothing is read before. A remote desktop client
// uses it for files copied inside the session, so copying a large file there
// transfers nothing until it is pasted here, as the platform's own client does.
//
// The files are one tree, each Name a single path component; a file's Open
// returns its bytes, a directory's List its children. The tree is described to
// the platform before the call returns, so a directory's List may run here, but
// Open runs only while a paste reads the file, possibly on another goroutine
// and more than once. released, when not nil, runs once when the clipboard no
// longer holds these files because something else replaced or cleared it; it
// may run on any goroutine and must not block. It tells the application when
// the content is no longer needed, for example to release the remote clipboard
// it was read from.
//
// A tree of more than MaxDragFiles items is refused. An error wrapping
// ErrUnsupported means this target cannot put promised files on the clipboard;
// the application keeps its eager copy in that case. macOS and Linux file
// managers read files by path and have no way to ask for a paste's content
// later, so only Windows takes this. The call is made on the event-loop thread,
// like StartFilePromiseDrag, because the platform clipboard is owned there.
func SetClipboardFilePromises(files []FilePromise, released func()) error {
	if len(files) == 0 {
		return fmt.Errorf("app: no files to put on the clipboard")
	}
	if err := platform.ValidateFilePromises(files); err != nil {
		return err
	}
	if err, handled := driverSetClipboardFilePromises(files, released); handled {
		return err
	}
	return nativeSetClipboardFilePromises(files, released)
}
