//go:build darwin || windows || linux

package app

import (
	"io"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestStartFileDragOfADriverRun(t *testing.T) {
	driver := newFilesDriver(t)
	want := []string{filepath.Join(t.TempDir(), "report.pdf"), filepath.Join(t.TempDir(), "文件夹")}
	if err := StartFileDrag(want, FileDragCopy|FileDragMove); err != nil {
		t.Fatal(err)
	}
	// A folder window that may copy takes the copy, as DropEffect prefers, and
	// the driver's target does the same.
	got, taken := driver.DraggedFiles()
	if !slices.Equal(got, want) || taken != FileDragCopy {
		t.Fatalf("the driver's drag = %q taking %d, want %q taking a copy", got, taken, want)
	}
	want[0] = "changed"
	if again, _ := driver.DraggedFiles(); again[0] != got[0] {
		t.Fatal("a caller changed the driver's drag through the slice it was given")
	}

	// An application that offers no copy is taken at its word.
	if err := StartFileDrag(got, FileDragMove|FileDragLink); err != nil {
		t.Fatal(err)
	}
	if _, taken := driver.DraggedFiles(); taken != FileDragLink {
		t.Fatalf("a drag offering a link and a move took %d", taken)
	}
	if err := StartFileDrag(got, FileDragMove); err != nil {
		t.Fatal(err)
	}
	if _, taken := driver.DraggedFiles(); taken != FileDragMove {
		t.Fatalf("a drag offering only a move took %d", taken)
	}
}

func TestStartFileDragRefusesWhatItCannotDrag(t *testing.T) {
	newFilesDriver(t)
	if err := StartFileDrag(nil, FileDragCopy); err == nil {
		t.Fatal("a drag of no files was accepted")
	}
	if err := StartFileDrag([]string{"relative.txt"}, FileDragCopy); err == nil {
		t.Fatal("a relative path was accepted")
	}
	// A drag that offers no effect cannot end anywhere.
	if err := StartFileDrag([]string{filepath.Join(t.TempDir(), "one.txt")}, FileDragNone); err == nil {
		t.Fatal("a drag offering no effect was accepted")
	}
	if err := StartFileDrag([]string{filepath.Join(t.TempDir(), "one.txt")}, FileDragEffect(8)); err == nil {
		t.Fatal("a drag offering an effect this platform does not name was accepted")
	}
	dir := t.TempDir()
	paths := make([]string, MaxDragFiles+1)
	for index := range paths {
		paths[index] = filepath.Join(dir, "f")
	}
	if err := StartFileDrag(paths, FileDragCopy); err == nil {
		t.Fatal("a drag of more files than it takes was accepted")
	}
}

func promiseFixture(t *testing.T) (files []FilePromise, contents map[string]string) {
	t.Helper()
	contents = map[string]string{"notes.txt": "hello", "inner/second.txt": "second"}
	file := func(name, value string) FilePromise {
		return FilePromise{Name: name, Size: int64(len(value)), Open: func() (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader(value)), nil
		}}
	}
	directory := FilePromise{Name: "docs", Directory: true, List: func() ([]FilePromise, error) {
		return []FilePromise{
			file("inner/second.txt", contents["inner/second.txt"]),
			file("guide.md", "guide"),
		}, nil
	}}
	return []FilePromise{file("notes.txt", contents["notes.txt"]), directory}, contents
}

func TestStartFilePromiseDragOfADriverRun(t *testing.T) {
	driver := newFilesDriver(t)
	files, _ := promiseFixture(t)
	if err := StartFilePromiseDrag(files); err != nil {
		t.Fatal(err)
	}
	recorded := driver.DraggedPromises()
	if len(recorded) != len(files) || recorded[0].Name != files[0].Name || recorded[1].Name != files[1].Name {
		t.Fatalf("the driver recorded %+v, want %+v", recorded, files)
	}
	// The content is still the application's: the driver's target asks through
	// the same callbacks a real one would.
	reader, err := recorded[0].Open()
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(reader)
	reader.Close()
	if err != nil || string(body) != "hello" {
		t.Fatalf("the promised file read %q, %v", body, err)
	}
	children, err := recorded[1].List()
	if err != nil || len(children) != 2 {
		t.Fatalf("the promised folder listed %d children, %v", len(children), err)
	}
}

func TestStartFilePromiseDragRefusesWhatItCannotPromise(t *testing.T) {
	newFilesDriver(t)
	open := func() (io.ReadCloser, error) { return io.NopCloser(strings.NewReader("")), nil }
	list := func() ([]FilePromise, error) { return nil, nil }
	for name, files := range map[string][]FilePromise{
		"no files":               nil,
		"no name":                {{Open: open}},
		"dot":                    {{Name: ".", Open: open}},
		"component":              {{Name: "a/b", Open: open}},
		"backslash":              {{Name: `a\b`, Open: open}},
		"no content":             {{Name: "a"}},
		"directory without list": {{Name: "d", Directory: true}},
		"directory with content": {{Name: "d", Directory: true, Open: open, List: list}},
		"file with list":         {{Name: "a", Open: open, List: list}},
		"size below -1":          {{Name: "a", Size: -2, Open: open}},
		"duplicate names":        {{Name: "a", Open: open}, {Name: "a", Open: open}},
	} {
		if err := StartFilePromiseDrag(files); err == nil {
			t.Fatalf("%s was accepted as a drag", name)
		}
	}
}
