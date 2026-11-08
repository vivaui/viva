//go:build darwin

package filewatch

/*
#cgo LDFLAGS: -framework CoreServices
#include <stdlib.h>
#include "watch_darwin.h"
*/
import "C"

import (
	"errors"
	"os"
	"path/filepath"
	"runtime/cgo"
	"strings"
	"sync"
	"unsafe"
)

// FSEvents flags this package reads; the values are the public constants of
// FSEvents.h, repeated so Go does not depend on their C spelling.
const (
	eventMustScanSubDirs = 0x1
	eventUserDropped     = 0x2
	eventKernelDropped   = 0x4
	eventRootChanged     = 0x20
	eventMount           = 0x40
	eventUnmount         = 0x80
	eventLost            = eventMustScanSubDirs | eventUserDropped | eventKernelDropped | eventRootChanged | eventMount | eventUnmount
)

// fseventsBackend is one FSEvents stream. The stream is recursive; entries of
// subdirectories are filtered out here so every platform reports the same set.
type fseventsBackend struct {
	watch  *Watch
	root   string
	stream *C.viva_fsevents
	handle cgo.Handle
	once   sync.Once
}

func openBackend(dir string, w *Watch) (backend, error) {
	root, err := filepath.EvalSymlinks(filepath.Clean(dir))
	if err != nil {
		return nil, err
	}
	if root, err = filepath.Abs(root); err != nil {
		return nil, err
	}
	if info, err := os.Stat(root); err != nil {
		return nil, err
	} else if !info.IsDir() {
		return nil, &os.PathError{Op: "watch", Path: dir, Err: errors.New("not a directory")}
	}
	b := &fseventsBackend{watch: w, root: root}
	b.handle = cgo.NewHandle(b)
	path := C.CString(root)
	defer C.free(unsafe.Pointer(path))
	b.stream = C.viva_fsevents_open(path, C.uintptr_t(b.handle))
	if b.stream == nil {
		b.handle.Delete()
		return nil, &os.PathError{Op: "FSEventStreamStart", Path: dir, Err: errors.New("the stream did not start")}
	}
	return b, nil
}

func (b *fseventsBackend) close() {
	b.once.Do(func() {
		C.viva_fsevents_close(b.stream)
		b.handle.Delete()
	})
}

// event is called on the stream's dispatch queue.
func (b *fseventsBackend) event(path string, flags int) {
	if flags&eventLost != 0 {
		b.watch.postResync()
		return
	}
	// FSEvents reports on-disk names, which may differ in case from the name the
	// caller used on a case-insensitive volume; an extra name is harmless.
	if len(path) <= len(b.root) || !strings.EqualFold(path[:len(b.root)], b.root) || path[len(b.root)] != '/' {
		return
	}
	rel := path[len(b.root)+1:]
	if b.watch.tree || !strings.Contains(rel, "/") {
		b.watch.post(rel)
	}
}

//export vivaFileEvent
func vivaFileEvent(handle C.uintptr_t, path *C.char, flags C.int) {
	cgo.Handle(handle).Value().(*fseventsBackend).event(C.GoString(path), int(flags))
}
