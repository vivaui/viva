//go:build darwin || windows || linux

package app

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestFileRefReadsRangesAndTheWholeFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "key.txt")
	want := []byte("0123456789")
	if err := os.WriteFile(path, want, 0o600); err != nil {
		t.Fatal(err)
	}
	ref, ok := nativeFileRefFromPath(path)
	if !ok {
		t.Fatal("nativeFileRefFromPath rejected a readable file")
	}
	file := FileRef{ref: ref}
	if file.Name() != "key.txt" || file.Size() != int64(len(want)) || file.IsDir() {
		t.Fatalf("file = %q size=%d dir=%t", file.Name(), file.Size(), file.IsDir())
	}
	var chunk []byte
	var readErr error
	if err := file.Read(3, 4, func(data []byte, err error) { chunk, readErr = data, err }); err != nil {
		t.Fatal(err)
	}
	if readErr != nil || string(chunk) != "3456" {
		t.Fatalf("Read = %q, %v", chunk, readErr)
	}
	var all []byte
	if err := file.ReadAll(func(data []byte, err error) { all, readErr = data, err }); err != nil {
		t.Fatal(err)
	}
	if readErr != nil || !bytes.Equal(all, want) {
		t.Fatalf("ReadAll = %q, %v", all, readErr)
	}
}

func TestFileSinkWritesThroughAndCloseIsFinal(t *testing.T) {
	path := filepath.Join(t.TempDir(), "export.txt")
	opened, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	sink := &FileSink{sink: &nativeFileSink{name: "export.txt", file: opened}}
	var writeErr, closeErr error
	if err := sink.Write([]byte("hello "), func(err error) { writeErr = err }); err != nil {
		t.Fatal(err)
	}
	if err := sink.Write([]byte("world"), func(err error) { writeErr = err }); err != nil {
		t.Fatal(err)
	}
	if err := sink.Close(func(err error) { closeErr = err }); err != nil {
		t.Fatal(err)
	}
	if writeErr != nil || closeErr != nil {
		t.Fatalf("write=%v close=%v", writeErr, closeErr)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != "hello world" {
		t.Fatalf("written file = %q, %v", got, err)
	}
}

func TestDroppedPathsBecomeReadableFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dropped.txt")
	if err := os.WriteFile(path, []byte("drop"), 0o600); err != nil {
		t.Fatal(err)
	}
	files := fileDropFilesFromPaths([]string{path, dir, filepath.Join(dir, "missing")})
	if len(files) != 2 {
		t.Fatalf("files = %d, want the file and the directory", len(files))
	}
	if files[0].Name() != "dropped.txt" || files[0].IsDir() {
		t.Fatalf("first file = %q dir=%t", files[0].Name(), files[0].IsDir())
	}
	if !files[1].IsDir() {
		t.Fatal("the directory was not reported as one")
	}
	var data []byte
	var readErr error
	if err := files[0].Read(0, 4, func(chunk []byte, err error) { data, readErr = chunk, err }); err != nil {
		t.Fatal(err)
	}
	if readErr != nil || string(data) != "drop" {
		t.Fatalf("Read = %q, %v", data, readErr)
	}
	if err := files[1].(FileRef).ReadAll(func([]byte, error) {}); err == nil {
		t.Fatal("ReadAll accepted a directory")
	}
}

func TestOpenFilesAsyncUsesTheDriverAndReturnsFiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "selected.txt")
	if err := os.WriteFile(path, []byte("selected"), 0o600); err != nil {
		t.Fatal(err)
	}
	driver, err := NewDriver(Config{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	if err := driver.QueueOpenFiles([]string{path}, nil); err != nil {
		t.Fatal(err)
	}
	var files []FileRef
	var openErr error
	called := 0
	if err := OpenFilesAsync(OpenFileOptions{Title: "Import"}, func(got []FileRef, err error) {
		called++
		files, openErr = got, err
	}); err != nil {
		t.Fatal(err)
	}
	if called != 1 || openErr != nil || len(files) != 1 || files[0].Name() != "selected.txt" {
		t.Fatalf("called=%d files=%v err=%v", called, files, openErr)
	}
	var data []byte
	if err := files[0].ReadAll(func(chunk []byte, err error) { data, openErr = chunk, err }); err != nil {
		t.Fatal(err)
	}
	if openErr != nil || string(data) != "selected" {
		t.Fatalf("read selected = %q, %v", data, openErr)
	}
}

func TestSaveFileAsyncWritesTheQueuedPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "saved.txt")
	driver, err := NewDriver(Config{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	if err := driver.QueueSaveFile(path, nil); err != nil {
		t.Fatal(err)
	}
	var sink *FileSink
	var saveErr error
	if err := SaveFileAsync(SaveFileOptions{SuggestedName: "saved.txt"}, func(got *FileSink, err error) {
		sink, saveErr = got, err
	}); err != nil {
		t.Fatal(err)
	}
	if saveErr != nil || sink == nil {
		t.Fatalf("sink=%v err=%v", sink, saveErr)
	}
	if err := sink.Write([]byte("saved"), func(error) {}); err != nil {
		t.Fatal(err)
	}
	if err := sink.Close(func(error) {}); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != "saved" {
		t.Fatalf("written file = %q, %v", got, err)
	}
}

func TestAsyncFileValidation(t *testing.T) {
	if err := OpenFilesAsync(OpenFileOptions{Filters: []FileFilter{{Name: "", Pattern: "*.txt"}}}, func([]FileRef, error) {}); err == nil {
		t.Fatal("OpenFilesAsync accepted an empty filter name")
	}
	if err := SaveFileAsync(SaveFileOptions{}, nil); err == nil {
		t.Fatal("SaveFileAsync accepted a nil callback")
	}
	var zero FileRef
	if err := zero.Read(0, 1, func([]byte, error) {}); err == nil {
		t.Fatal("a zero FileRef started a read")
	}
	if err := zero.ReadAll(func([]byte, error) {}); err == nil {
		t.Fatal("a zero FileRef started ReadAll")
	}
	if err := zero.Read(-1, 1, func([]byte, error) {}); err == nil {
		t.Fatal("a negative offset started a read")
	}
}
