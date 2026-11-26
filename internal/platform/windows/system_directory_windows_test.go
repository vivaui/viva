package windows

import (
	"fmt"
	"os"
	"testing"
	"unsafe"
)

func TestFileDialogInitialDirectory(t *testing.T) {
	directory := t.TempDir()
	err := withSTA(func() error {
		dialog, err := createFileDialog(&openDialogClass, &openDialogInterface)
		if err != nil {
			return err
		}
		defer release(&dialog)
		if err := setFileDialogDirectory(dialog, directory); err != nil {
			return err
		}
		var item uintptr
		if err := hresult(comCall1(dialog, 13, uintptr(unsafe.Pointer(&item)))); err != nil {
			return err
		}
		defer release(&item)
		got, err := shellItemPath(item)
		if err != nil {
			return err
		}
		// The temporary directory may be named in its 8.3 short form (RUNNER~1) while
		// the shell reports the long one: they are the same folder when the file
		// system says so, whatever the spelling.
		gotInfo, err := os.Stat(got)
		if err != nil {
			return err
		}
		wantInfo, err := os.Stat(directory)
		if err != nil {
			return err
		}
		if !os.SameFile(gotInfo, wantInfo) {
			return fmt.Errorf("dialog folder = %q; want %q", got, directory)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
