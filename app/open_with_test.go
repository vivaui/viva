//go:build darwin || windows || linux

package app

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestOpenWithValidatesBothAbsoluteTargets(t *testing.T) {
	dir := t.TempDir()
	executable := filepath.Join(dir, "Editor With Spaces.exe")
	target := filepath.Join(dir, "notes.yml")
	for _, file := range []string{executable, target} {
		if err := os.WriteFile(file, []byte("x"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	cases := [][2]string{
		{"", target},
		{"relative.exe", target},
		{executable, ""},
		{executable, "relative.yml"},
		{executable, target + "\x00"},
		{executable, target + "\n"},
	}
	for _, pair := range cases {
		if err := OpenWith(pair[0], pair[1]); err == nil {
			t.Errorf("OpenWith(%q, %q) accepted", pair[0], pair[1])
		}
	}
}

func TestDriverRecordsOpenWithAndStartsNothing(t *testing.T) {
	driver, err := NewDriver(Config{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	dir := t.TempDir()
	executable := filepath.Join(dir, "Editor.exe")
	target := filepath.Join(dir, "notes.yml")
	for _, file := range []string{executable, target} {
		if err := os.WriteFile(file, []byte("x"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := OpenWith(executable, target); err != nil {
		t.Fatal(err)
	}
	gotExecutable, gotTarget, ok := driver.LastOpenedWith()
	if !ok || gotExecutable != executable || gotTarget != target {
		t.Fatalf("LastOpenedWith = %q, %q, %t", gotExecutable, gotTarget, ok)
	}
	if runtime.GOOS == "darwin" {
		bundle := filepath.Join(dir, "Editor.app")
		if err := os.Mkdir(bundle, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := OpenWith(bundle, target); err != nil {
			t.Fatalf("app bundle: %v", err)
		}
	}
	if runtime.GOOS == "windows" {
		program := filepath.Join(dir, "program.txt")
		if err := os.WriteFile(program, []byte("x"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := OpenWith(program, target); err == nil {
			t.Fatal("a document was accepted as an executable")
		}
	}
}

func TestOpenWithRejectsControlCharactersInEitherArgument(t *testing.T) {
	for _, value := range []string{"bad\x01", "bad\ndir", "bad\x7f"} {
		if _, err := validateOpenWithTarget("path", filepath.Join(t.TempDir(), value)); err == nil {
			t.Errorf("control character %q was accepted", strings.TrimSpace(value))
		}
	}
}
