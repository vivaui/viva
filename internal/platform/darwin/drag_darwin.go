//go:build darwin && !ios

package darwin

/*
#include <stdlib.h>
#include "native.h"
*/
import "C"

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
)

// fileDragStore keeps the promises of the drags this process has started until
// the last provider that may ask for them is gone. The native side passes its
// key back with each request and releases it from the context that the
// providers keep alive; the content then runs on the promise operation queue.
var fileDragStore struct {
	mu   sync.Mutex
	next int64
	byID map[int64][]platform.FilePromise
}

// StartFileDrag takes files that exist at paths out of the window handling the
// current press. The session consumes the press, so the application is told the
// press ended when the session ends.
func StartFileDrag(paths []string, allowed platform.DragEffect) error {
	if len(paths) == 0 {
		return fmt.Errorf("darwin: no files to drag out of the window")
	}
	strings := make([]*C.char, len(paths))
	for index, path := range paths {
		strings[index] = C.CString(path)
	}
	defer func() {
		for _, path := range strings {
			C.free(unsafe.Pointer(path))
		}
	}()
	status := C.viva_start_file_drag((**C.char)(unsafe.Pointer(&strings[0])), C.size_t(len(strings)), C.uint(allowed))
	if status != 0 {
		return dragStartError(int(status))
	}
	return nil
}

// StartFilePromiseDrag takes promised files out of the window handling the
// current press: each is written where the target lets go, by asking the
// application for its content then.
func StartFilePromiseDrag(files []platform.FilePromise) error {
	if len(files) == 0 {
		return fmt.Errorf("darwin: no files to drag out of the window")
	}
	fileDragStore.mu.Lock()
	fileDragStore.next++
	drag := fileDragStore.next
	if fileDragStore.byID == nil {
		fileDragStore.byID = make(map[int64][]platform.FilePromise)
	}
	fileDragStore.byID[drag] = append([]platform.FilePromise(nil), files...)
	fileDragStore.mu.Unlock()

	names := make([]*C.char, len(files))
	items := make([]C.viva_drag_item, len(files))
	for index, promise := range files {
		names[index] = C.CString(promise.Name)
		items[index] = C.viva_drag_item{name: names[index], directory: 0, item: C.int(index)}
		if promise.Directory {
			items[index].directory = 1
		}
	}
	status := C.viva_start_file_promise_drag(C.intptr_t(drag), &items[0], C.size_t(len(items)))
	for _, name := range names {
		C.free(unsafe.Pointer(name))
	}
	if status != 0 {
		finishFileDrag(drag)
		return dragStartError(int(status))
	}
	return nil
}

func dragStartError(status int) error {
	switch status {
	case -1:
		return fmt.Errorf("darwin: no window is handling the drag")
	case -2:
		return fmt.Errorf("darwin: the drag did not start from a mouse press")
	case -3:
		return fmt.Errorf("darwin: another file drag is already running")
	default:
		return fmt.Errorf("darwin: the file drag could not start (%d)", status)
	}
}

func finishFileDrag(drag int64) {
	fileDragStore.mu.Lock()
	delete(fileDragStore.byID, drag)
	fileDragStore.mu.Unlock()
}

//export vivaFilePromiseContent
func vivaFilePromiseContent(drag C.intptr_t, item C.int, path *C.char) C.int {
	if path == nil {
		return 1
	}
	fileDragStore.mu.Lock()
	files := fileDragStore.byID[int64(drag)]
	fileDragStore.mu.Unlock()
	index := int(item)
	if index < 0 || index >= len(files) {
		return 1
	}
	if err := writeFilePromise(files[index], C.GoString(path)); err != nil {
		return 1
	}
	return 0
}

//export vivaFileDragFinished
func vivaFileDragFinished(drag C.intptr_t) {
	finishFileDrag(int64(drag))
}

// writeFilePromise writes promise where the target asked: a file's bytes, or a
// directory and everything List produces under it.
func writeFilePromise(promise platform.FilePromise, destination string) error {
	if !promise.Directory {
		return writePromiseFile(promise, destination)
	}
	if err := os.MkdirAll(destination, 0o755); err != nil {
		return err
	}
	children, err := promise.List()
	if err != nil {
		return err
	}
	return platform.WalkFilePromises(children, func(relative string, child platform.FilePromise) error {
		target := filepath.Join(destination, filepath.FromSlash(relative))
		if child.Directory {
			return os.MkdirAll(target, 0o755)
		}
		return writePromiseFile(child, target)
	})
}

func writePromiseFile(promise platform.FilePromise, target string) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	reader, err := promise.Open()
	if err != nil {
		return err
	}
	defer reader.Close()
	file, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(file, reader); err != nil {
		file.Close()
		os.Remove(target)
		return err
	}
	if err := file.Close(); err != nil {
		os.Remove(target)
		return err
	}
	return nil
}
