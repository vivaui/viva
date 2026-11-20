//go:build darwin || windows || linux

package app

import (
	"path/filepath"
	"slices"
	"testing"
)

func TestStartFileDragOfADriverRun(t *testing.T) {
	driver := newFilesDriver(t)
	want := []string{filepath.Join(t.TempDir(), "report.pdf"), filepath.Join(t.TempDir(), "文件夹")}
	effect, err := StartFileDrag(want, FileDragCopy|FileDragMove)
	if err != nil {
		t.Fatal(err)
	}
	// A folder window that may copy takes the copy, as DropEffect prefers, and
	// the driver's target does the same.
	if effect != FileDragCopy {
		t.Fatalf("the drag took effect %d, want a copy", effect)
	}
	got, taken := driver.DraggedFiles()
	if !slices.Equal(got, want) || taken != FileDragCopy {
		t.Fatalf("the driver's drag = %q taking %d, want %q taking a copy", got, taken, want)
	}
	want[0] = "changed"
	if again, _ := driver.DraggedFiles(); again[0] != got[0] {
		t.Fatal("a caller changed the driver's drag through the slice it was given")
	}

	// An application that offers no copy is taken at its word.
	if effect, err := StartFileDrag(got, FileDragMove|FileDragLink); err != nil || effect != FileDragLink {
		t.Fatalf("a drag offering a link and a move took %d, %v", effect, err)
	}
	if effect, err := StartFileDrag(got, FileDragMove); err != nil || effect != FileDragMove {
		t.Fatalf("a drag offering only a move took %d, %v", effect, err)
	}
}

func TestStartFileDragRefusesWhatItCannotDrag(t *testing.T) {
	newFilesDriver(t)
	if _, err := StartFileDrag(nil, FileDragCopy); err == nil {
		t.Fatal("a drag of no files was accepted")
	}
	if _, err := StartFileDrag([]string{"relative.txt"}, FileDragCopy); err == nil {
		t.Fatal("a relative path was accepted")
	}
	// A drag that offers no effect cannot end anywhere.
	if _, err := StartFileDrag([]string{filepath.Join(t.TempDir(), "one.txt")}, FileDragNone); err == nil {
		t.Fatal("a drag offering no effect was accepted")
	}
	if _, err := StartFileDrag([]string{filepath.Join(t.TempDir(), "one.txt")}, FileDragEffect(8)); err == nil {
		t.Fatal("a drag offering an effect this platform does not name was accepted")
	}
	dir := t.TempDir()
	paths := make([]string, MaxDragFiles+1)
	for index := range paths {
		paths[index] = filepath.Join(dir, "f")
	}
	if _, err := StartFileDrag(paths, FileDragCopy); err == nil {
		t.Fatal("a drag of more files than it takes was accepted")
	}
}
