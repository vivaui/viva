package windows

import (
	"fmt"
	"os"
	"testing"
	"unsafe"
)

func TestFileDialogInitialDirectory(t *testing.T) {
	directory := t.TempDir()
	want, err := os.Stat(directory)
	if err != nil {
		t.Fatal(err)
	}
	err = withSTA(func() error {
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
		actual, err := os.Stat(got)
		if err != nil {
			return fmt.Errorf("stat dialog folder %q: %w", got, err)
		}
		// The shell can expand a short path to its long name.
		if !os.SameFile(actual, want) {
			return fmt.Errorf("dialog folder = %q; want %q", got, directory)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
