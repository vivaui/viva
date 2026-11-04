//go:build windows

package windows

import (
	"fmt"
	"path/filepath"
	"syscall"
	"unsafe"
)

var (
	itemIDFromPath = shell.NewProc("ILCreateFromPathW")
	lastItemID     = shell.NewProc("ILFindLastID")
	freeItemID     = shell.NewProc("ILFree")
	showSelected   = shell.NewProc("SHOpenFolderAndSelectItems")
)

// RevealPath asks Explorer to select an existing directory entry. PIDLs, not
// command-line arguments, carry the target: a filename cannot be interpreted as
// an explorer.exe switch or as something to run.
func RevealPath(path string) error {
	return withSTA(func() error {
		name, err := syscall.UTF16PtrFromString(path)
		if err != nil {
			return err
		}
		item, _, callErr := itemIDFromPath.Call(uintptr(unsafe.Pointer(name)))
		if item == 0 {
			return windowsCallError("find item to reveal", callErr)
		}
		defer freeItemID.Call(item)

		parent := filepath.Dir(path)
		if parent == path { // volume root: show the directory itself
			status, _, _ := showSelected.Call(item, 0, 0, 0)
			if err := hresult(status); err != nil {
				return fmt.Errorf("windows: reveal volume: %w", err)
			}
			return nil
		}
		folderName, err := syscall.UTF16PtrFromString(parent)
		if err != nil {
			return err
		}
		folder, _, callErr := itemIDFromPath.Call(uintptr(unsafe.Pointer(folderName)))
		if folder == 0 {
			return windowsCallError("find containing folder", callErr)
		}
		defer freeItemID.Call(folder)
		child, _, _ := lastItemID.Call(item)
		if child == 0 {
			return fmt.Errorf("windows: no item to select in %q", parent)
		}
		status, _, _ := showSelected.Call(folder, 1, uintptr(unsafe.Pointer(&child)), 0)
		if err := hresult(status); err != nil {
			return fmt.Errorf("windows: reveal item: %w", err)
		}
		return nil
	})
}
