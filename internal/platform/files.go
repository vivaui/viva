package platform

import "errors"

// ErrCanceled reports that the user dismissed a file picker.
var ErrCanceled = errors.New("platform: operation canceled")

// FileFilter describes one file picker filter. Pattern follows the native
// pickers: one or more glob patterns separated by semicolons.
type FileFilter struct {
	Name    string
	Pattern string
}

// OpenFilesOptions describes an open-file picker.
type OpenFilesOptions struct {
	Title         string
	Filters       []FileFilter
	AllowMultiple bool
}

// SaveFileOptions describes a save-file picker.
type SaveFileOptions struct {
	Title            string
	SuggestedName    string
	DefaultExtension string
	Filters          []FileFilter
}

// FileRef is a file the user chose. Name, Size, MIME and IsDir are known
// without reading; Read delivers bytes asynchronously so a browser can settle
// its Promise without blocking the event loop. A directory cannot be read.
type FileRef interface {
	Name() string
	Size() int64
	MIME() string
	IsDir() bool
	Read(offset int64, length int, done func([]byte, error)) error
}

// FileSink is a writable file the user chose. Writes append; Close commits the
// file or, for a browser download, starts it. done runs exactly once.
type FileSink interface {
	Name() string
	Write(data []byte, done func(error)) error
	Close(done func(error)) error
}
