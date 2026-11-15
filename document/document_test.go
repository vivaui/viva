package document

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadAndSaveKeepTheBytesAndRevision(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(path, []byte("one\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	data, revision, err := Read(path)
	if err != nil || string(data) != "one\n" {
		t.Fatalf("Read = %q, %v", data, err)
	}
	if changed, err := Changed(path, revision); err != nil || changed {
		t.Fatalf("Changed = %v, %v", changed, err)
	}
	next, err := Save(path, []byte("two\n"), revision)
	if err != nil {
		t.Fatal(err)
	}
	if changed, err := Changed(path, next); err != nil || changed {
		t.Fatalf("Changed after Save = %v, %v", changed, err)
	}
	again, err := os.ReadFile(path)
	if err != nil || string(again) != "two\n" {
		t.Fatalf("file = %q, %v", again, err)
	}
}

func TestSaveRefusesAFileAnotherProgramChanged(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(path, []byte("one\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, revision, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("theirs\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Save(path, []byte("ours\n"), revision); !errors.Is(err, ErrChanged) {
		t.Fatalf("Save = %v, want ErrChanged", err)
	}
	actual, _ := os.ReadFile(path)
	if string(actual) != "theirs\n" {
		t.Fatalf("the file was replaced after a conflict: %q", actual)
	}
	temporary, _ := filepath.Glob(filepath.Join(filepath.Dir(path), ".notes.txt.tmp-*"))
	if len(temporary) != 0 {
		t.Fatalf("temporary files leaked: %v", temporary)
	}
}

func TestSaveKeepsTheModeAndLeavesNoTemporaryFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "run.sh")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o750); err != nil {
		t.Fatal(err)
	}
	_, revision, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Save(path, []byte("#!/bin/sh\necho hi\n"), revision); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o750 {
		t.Fatalf("mode = %v, want 0750", info.Mode().Perm())
	}
	temporary, _ := filepath.Glob(filepath.Join(filepath.Dir(path), ".run.sh.tmp-*"))
	if len(temporary) != 0 {
		t.Fatalf("temporary files leaked: %v", temporary)
	}
}

func TestSaveCreatesANewFileOnlyWithTheZeroRevision(t *testing.T) {
	path := filepath.Join(t.TempDir(), "new.txt")
	if _, err := Save(path, []byte("hello\n"), Revision{}); err != nil {
		t.Fatal(err)
	}
	actual, _ := os.ReadFile(path)
	if string(actual) != "hello\n" {
		t.Fatalf("file = %q", actual)
	}
	if _, err := Save(path, []byte("again\n"), Revision{}); !errors.Is(err, ErrChanged) {
		t.Fatalf("Save over an existing file with the zero revision = %v, want ErrChanged", err)
	}
}

func TestReadAndSaveRejectSymbolicLinks(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "real.txt")
	if err := os.WriteFile(target, []byte("real\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link.txt")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symbolic links are unavailable: %v", err)
	}
	if _, _, err := Read(link); !errors.Is(err, ErrNotRegular) {
		t.Fatalf("Read(link) = %v, want ErrNotRegular", err)
	}
	if _, err := Save(link, []byte("x"), Revision{}); !errors.Is(err, ErrNotRegular) {
		t.Fatalf("Save(link) = %v, want ErrNotRegular", err)
	}
}

func TestChangedReportsAMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gone.txt")
	changed, err := Changed(path, Revision{})
	if err != nil || !changed {
		t.Fatalf("Changed = %v, %v; want changed", changed, err)
	}
}

func TestSaveLeavesTheOldFileOnAWriteFailure(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(path, []byte("old\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, revision, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
	if _, err := Save(path, []byte(strings.Repeat("x", 4096)), revision); err == nil {
		t.Fatal("Save succeeded in a directory it cannot write")
	}
	actual, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(actual, []byte("old\n")) {
		t.Fatalf("the old file was damaged: %q, %v", actual, err)
	}
}
