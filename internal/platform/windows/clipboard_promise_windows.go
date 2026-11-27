//go:build windows

package windows

import (
	"fmt"
	"runtime"
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
)

// Put files on the clipboard as what the user pasted: unlike SetClipboardFiles,
// which builds a CF_HDROP list of paths that must already exist, this hands OLE
// the same data object a promised drag uses. Explorer's paste asks the object
// for the file group descriptor and then for one IStream per file, so the
// content is produced only when the paste reads it. The object stays the
// clipboard's until something else replaces or clears the clipboard; that is
// when OLE releases it and the caller's release callback runs.

var (
	oleSetClipboard = ole.NewProc("OleSetClipboard")
	oleGetClipboard = ole.NewProc("OleGetClipboard")
)

// SetClipboardFilePromises makes files whose content the application produces
// on demand what this computer's clipboard holds, as Windows' own desktop
// client does for files copied inside a remote session. The tree is described
// to the shell before the call returns; each file's Open runs when a paste reads
// it. released, when not nil, runs once when the clipboard no longer holds the
// files, on whatever thread OLE releases the object.
func SetClipboardFilePromises(files []platform.FilePromise, released func()) error {
	state := active.Load()
	if state == nil || state.window == 0 {
		return fmt.Errorf("windows: no window to put files on the clipboard from")
	}
	thread, _, _ := getCurrentThread.Call()
	if uint32(thread) != state.application.thread {
		return fmt.Errorf("windows: clipboard files are promised on the event-loop thread that owns the window")
	}
	if len(files) == 0 {
		return fmt.Errorf("windows: no files to put on the clipboard")
	}
	descriptor, contents := fileGroupDescriptorFormat(), fileContentsFormat()
	if descriptor == 0 || contents == 0 {
		return fmt.Errorf("windows: the shell's file formats could not be registered")
	}
	items, err := promiseItems(files)
	if err != nil {
		return err
	}
	if !state.application.oleReady {
		if status, _, _ := oleInitialize.Call(0); int32(uint32(status)) < 0 {
			return hresult(status)
		}
		state.application.oleReady = true
	}
	dataTableOnce.Do(newFileDataTable)
	data := &fileDataObject{table: &dataTable, refs: 1}
	data.promise = &promiseDrag{items: items, descriptor: descriptor, contents: contents, released: released}
	pinPromiseDrag(data)
	status, _, _ := oleSetClipboard.Call(uintptr(unsafe.Pointer(data)))
	if err := hresult(status); err != nil {
		// The clipboard never held the object, so it is not "released".
		data.promise.released = nil
		dataRelease(uintptr(unsafe.Pointer(data)))
		return fmt.Errorf("windows: put promised files on the clipboard: %w", err)
	}
	// OLE took its own reference; ours goes now.
	dataRelease(uintptr(unsafe.Pointer(data)))
	runtime.KeepAlive(data)
	return nil
}
