package windows

import (
	"fmt"
	"path/filepath"
	"strings"
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
		if !strings.EqualFold(filepath.Clean(got), filepath.Clean(directory)) {
			return fmt.Errorf("dialog folder = %q; want %q", got, directory)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
