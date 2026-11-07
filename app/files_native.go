//go:build !(js && wasm)

package app

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/ui"
)

func asyncOpenFiles(options OpenFileOptions, done func([]platform.FileRef, error)) error {
	paths, err := OpenFiles(options)
	if err != nil {
		done(nil, err)
		return nil
	}
	files := make([]platform.FileRef, 0, len(paths))
	for _, path := range paths {
		if file, ok := nativeFileRefFromPath(path); ok {
			files = append(files, file)
			continue
		}
		done(nil, fmt.Errorf("app: cannot open selected file %q", path))
		return nil
	}
	done(files, nil)
	return nil
}

func asyncSaveFile(options SaveFileOptions, done func(platform.FileSink, error)) error {
	path, err := SaveFile(options)
	if err != nil {
		done(nil, err)
		return nil
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		done(nil, err)
		return nil
	}
	done(&nativeFileSink{name: filepath.Base(path), file: file}, nil)
	return nil
}

func asyncFileError(_ string, err error) error { return err }

type nativeFileRef struct {
	path string
	info os.FileInfo
}

func nativeFileRefFromPath(path string) (platform.FileRef, bool) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, false
	}
	return &nativeFileRef{path: path, info: info}, true
}

func (file *nativeFileRef) Name() string { return file.info.Name() }
func (file *nativeFileRef) Size() int64  { return file.info.Size() }
func (file *nativeFileRef) MIME() string { return "" }
func (file *nativeFileRef) IsDir() bool  { return file.info.IsDir() }

func (file *nativeFileRef) Read(offset int64, length int, done func([]byte, error)) error {
	if file.info.IsDir() {
		done(nil, fmt.Errorf("app: %q is a directory", file.path))
		return nil
	}
	opened, err := os.Open(file.path)
	if err != nil {
		done(nil, err)
		return nil
	}
	defer opened.Close()
	buffer := make([]byte, length)
	read, err := opened.ReadAt(buffer, offset)
	if err == io.EOF {
		err = nil
	}
	done(buffer[:read], err)
	return nil
}

type nativeFileSink struct {
	mu     sync.Mutex
	name   string
	file   *os.File
	closed bool
}

func (sink *nativeFileSink) Name() string { return sink.name }

func (sink *nativeFileSink) Write(data []byte, done func(error)) error {
	sink.mu.Lock()
	if sink.closed {
		sink.mu.Unlock()
		done(nil)
		return nil
	}
	_, err := sink.file.Write(data)
	sink.mu.Unlock()
	done(err)
	return nil
}

func (sink *nativeFileSink) Close(done func(error)) error {
	sink.mu.Lock()
	if sink.closed {
		sink.mu.Unlock()
		done(nil)
		return nil
	}
	sink.closed = true
	err := sink.file.Close()
	sink.mu.Unlock()
	done(err)
	return nil
}

// fileDropFilesFromPaths makes the files of a native drop readable through the
// same interface a browser drop uses.
func fileDropFilesFromPaths(paths []string) []ui.FileDropFile {
	files := make([]ui.FileDropFile, 0, len(paths))
	for _, path := range paths {
		if file, ok := nativeFileRefFromPath(path); ok {
			files = append(files, FileRef{ref: file})
		}
	}
	return files
}
