package ui

import (
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
)

// OpenFileOptions and FileFilter describe the system's open-file picker: its
// title, the kinds of file it offers and whether it takes several.
type (
	OpenFileOptions = app.OpenFileOptions
	FileFilter      = app.FileFilter
)

// ErrOpenCanceled is what OpenFiles reports when the user closes the picker
// without choosing.
var ErrOpenCanceled = app.ErrCanceled

// OpenFiles presents the system's open-file picker and calls done once with the
// files the user chose, or with an error: ErrOpenCanceled when the picker was
// closed. The files read the same way as the ones dropped on a FileDropState.
// A native picker answers before OpenFiles returns; a browser's answers later,
// on the window's thread, and the window is painted again when it does, so done
// only has to record the result. A browser asks for the call to be made while a
// click is handled, so call it from the code that reacts to Clicked.
func OpenFiles(c C, options OpenFileOptions, done func([]gfx.FileDropFile, error)) error {
	wake := c.GetPage().GetInvalidator()
	return app.OpenFilesAsync(options, func(refs []app.FileRef, err error) {
		if err != nil {
			done(nil, err)
		} else {
			files := make([]gfx.FileDropFile, len(refs))
			for i, ref := range refs {
				files[i] = ref
			}
			done(files, nil)
		}
		if wake != nil {
			wake.Invalidate()
		}
	})
}

// FilePath is the absolute path of a file OpenFiles or a FileDropState gave, or
// the empty string where the platform does not give one (a browser).
func FilePath(file gfx.FileDropFile) string {
	if located, ok := file.(interface{ Path() string }); ok {
		return located.Path()
	}
	return ""
}
