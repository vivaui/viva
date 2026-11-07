package app

import (
	"fmt"
	"math"

	"github.com/vivaui/viva/internal/platform"
)

// FileRef is a file the user chose through OpenFilesAsync or delivered by a
// drop. It is not a path: a browser grants access to the chosen contents and
// nothing else, so the name and size are known but the file cannot be reopened
// after the page reloads. ReadAll is the usual way to take a small file;
// ReadAt-style calls suit a file too large to hold at once.
type FileRef struct {
	ref platform.FileRef
}

// Name is the file's name as the picker or drop reported it, without a
// directory.
func (file FileRef) Name() string {
	if file.ref == nil {
		return ""
	}
	return file.ref.Name()
}

// Size is the file's length in bytes.
func (file FileRef) Size() int64 {
	if file.ref == nil {
		return 0
	}
	return file.ref.Size()
}

// MIME is the browser's type for the file, or "" when it has none. Native
// targets do not guess a type.
func (file FileRef) MIME() string {
	if file.ref == nil {
		return ""
	}
	return file.ref.MIME()
}

// IsDir reports whether the chosen entry is a directory. Directories cannot be
// read.
func (file FileRef) IsDir() bool {
	if file.ref == nil {
		return false
	}
	return file.ref.IsDir()
}

// Read asks for length bytes starting at offset. done runs exactly once with
// at most length bytes; a short read at the end of the file is not an error.
// Native targets call done before Read returns; a browser calls it after its
// Promise settles. A returned error means the read could not start.
func (file FileRef) Read(offset int64, length int, done func([]byte, error)) error {
	if done == nil {
		return fmt.Errorf("app: nil file callback")
	}
	if offset < 0 || length < 0 {
		return fmt.Errorf("app: invalid file range offset=%d length=%d", offset, length)
	}
	if file.ref == nil {
		return fmt.Errorf("app: file is not open")
	}
	if length == 0 || offset >= file.Size() {
		done([]byte{}, nil)
		return nil
	}
	return file.ref.Read(offset, length, done)
}

// readAllChunk is one browser read request. Large enough to keep the number of
// Promise round trips small, small enough not to duplicate a large file in
// temporary buffers.
const readAllChunk = 4 << 20

// ReadAll reads the whole file into memory and calls done exactly once. It is
// for files whose size the application already accepts; a file larger than the
// address space fails instead of truncating. Native targets call done before
// ReadAll returns.
func (file FileRef) ReadAll(done func([]byte, error)) error {
	if done == nil {
		return fmt.Errorf("app: nil file callback")
	}
	if file.ref == nil {
		return fmt.Errorf("app: file is not open")
	}
	if file.IsDir() {
		return fmt.Errorf("app: %q is a directory", file.Name())
	}
	size := file.Size()
	if size < 0 || uint64(size) > uint64(math.MaxInt) {
		return fmt.Errorf("app: file %q is too large to read (%d bytes)", file.Name(), size)
	}
	data := make([]byte, 0, size)
	var read func(offset int64) error
	read = func(offset int64) error {
		if offset >= size {
			done(data, nil)
			return nil
		}
		length := int64(readAllChunk)
		if remaining := size - offset; remaining < length {
			length = remaining
		}
		return file.Read(offset, int(length), func(chunk []byte, err error) {
			if err != nil {
				done(nil, err)
				return
			}
			if len(chunk) == 0 {
				done(nil, fmt.Errorf("app: file %q read made no progress at %d", file.Name(), offset))
				return
			}
			data = append(data, chunk...)
			if err := read(offset + int64(len(chunk))); err != nil {
				done(nil, err)
			}
		})
	}
	return read(0)
}

// FileSink is a file the user chose to save through SaveFileAsync. Writes
// append; Close commits the file and must be called once. Native targets write
// through to the chosen path. A browser without the File System Access API
// keeps the written bytes in memory and starts a download on Close, so a very
// large export should use a browser that can stream.
type FileSink struct {
	sink platform.FileSink
}

// Name is the chosen file's name.
func (sink *FileSink) Name() string {
	if sink == nil || sink.sink == nil {
		return ""
	}
	return sink.sink.Name()
}

// Write appends data. done runs exactly once: native targets before Write
// returns, a browser after the write Promise settles. The caller may reuse data
// as soon as Write returns.
func (sink *FileSink) Write(data []byte, done func(error)) error {
	if done == nil {
		return fmt.Errorf("app: nil file callback")
	}
	if sink == nil || sink.sink == nil {
		return fmt.Errorf("app: file sink is not open")
	}
	return sink.sink.Write(data, done)
}

// Close commits the file. done runs exactly once. Closing an already closed
// sink reports success.
func (sink *FileSink) Close(done func(error)) error {
	if done == nil {
		return fmt.Errorf("app: nil file callback")
	}
	if sink == nil || sink.sink == nil {
		return fmt.Errorf("app: file sink is not open")
	}
	return sink.sink.Close(done)
}

// OpenFilesAsync is OpenFiles with a completion callback, the way to open
// files on the Web target, where the browser answers through a Promise. On
// native targets done runs before OpenFilesAsync returns, with the same result
// as OpenFiles. done runs exactly once. A browser requires the call to happen
// in a user gesture; a refusal is reported as ErrUserGestureRequired where the
// browser reports it. Cancellation is ErrCanceled.
func OpenFilesAsync(options OpenFileOptions, done func([]FileRef, error)) error {
	if done == nil {
		return fmt.Errorf("app: nil file callback")
	}
	if err := validateFilters(options.Filters); err != nil {
		return err
	}
	err := asyncOpenFiles(options, func(files []platform.FileRef, err error) {
		if err != nil {
			done(nil, asyncFileError("open files", err))
			return
		}
		refs := make([]FileRef, len(files))
		for index, file := range files {
			refs[index] = FileRef{ref: file}
		}
		done(refs, nil)
	})
	if err != nil {
		return asyncFileError("open files", err)
	}
	return nil
}

// SaveFileAsync is SaveFile with a completion callback, the way to save a file
// on the Web target. The returned sink writes the chosen file; Close commits
// it. On native targets done runs before SaveFileAsync returns. done runs
// exactly once. Cancellation is ErrCanceled.
func SaveFileAsync(options SaveFileOptions, done func(*FileSink, error)) error {
	if done == nil {
		return fmt.Errorf("app: nil file callback")
	}
	if err := validateFilters(options.Filters); err != nil {
		return err
	}
	err := asyncSaveFile(options, func(sink platform.FileSink, err error) {
		if err != nil {
			done(nil, asyncFileError("save file", err))
			return
		}
		done(&FileSink{sink: sink}, nil)
	})
	if err != nil {
		return asyncFileError("save file", err)
	}
	return nil
}
