package app

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/vivaui/viva/internal/platform"
)

// ErrNoFiles reports that the clipboard holds no files.
var ErrNoFiles = platform.ErrNoClipboardFiles

// MaxClipboardFiles is the most paths ClipboardFiles returns; a longer list is
// cut at it.
const MaxClipboardFiles = platform.MaxClipboardFiles

// ClipboardFiles returns the paths of the files the clipboard holds, as a file
// manager puts them there when the user copies a selection, and ErrNoFiles when
// it holds none. The paths are absolute and local: macOS reads the pasteboard's
// file URLs, Windows the CF_HDROP list, and Linux the x-special/gnome-copied-files
// or text/uri-list target of the CLIPBOARD owner. Nothing is retained, and the
// files themselves are not opened, so a path can name a file that has gone since
// it was copied.
//
// Files a source promises but has not written (a browser's dragged image, a
// macOS file promise) have no path and are not listed. An application that
// shares the clipboard should ask for the files before it asks for text,
// because most file managers put the file's name on the clipboard as text too.
//
// The Web target cannot read files from the clipboard and returns an error
// wrapping ErrUnsupported. On Linux a clipboard owned by another application is
// asked for its formats, which waits for that application for up to three
// seconds, so read when the clipboard changed or the user acts, not on every
// frame; OnClipboardChange says when it changed where the target can tell.
func ClipboardFiles() ([]string, error) {
	if paths, err, handled := driverClipboardFiles(); handled {
		return paths, err
	}
	return nativeClipboardFiles()
}

// SetClipboardFiles makes paths the files the system clipboard holds, as a
// file manager does after the user copies a selection. The files are not
// copied: a paste reads them from these paths, so they must stay where they
// are until the paste ends or something else replaces the clipboard. As Finder
// and Explorer do, the files' names are also put on the clipboard as text.
// The paths must be absolute, local and free of NUL; at most
// MaxClipboardFiles are accepted, and a longer list is an error rather than a
// silent cut. An error wrapping ErrUnsupported means this target cannot put
// files on the clipboard.
func SetClipboardFiles(paths []string) error {
	if len(paths) == 0 {
		return fmt.Errorf("app: no files to put on the clipboard")
	}
	if len(paths) > MaxClipboardFiles {
		return fmt.Errorf("app: the clipboard takes at most %d files", MaxClipboardFiles)
	}
	for index, path := range paths {
		if path == "" || strings.IndexByte(path, 0) >= 0 || !filepath.IsAbs(path) {
			return fmt.Errorf("app: clipboard file %d is not an absolute path: %q", index, path)
		}
	}
	if err, handled := driverSetClipboardFiles(paths); handled {
		return err
	}
	return nativeSetClipboardFiles(paths)
}
