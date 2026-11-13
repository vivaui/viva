//go:build darwin || windows || linux

package app

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestDriverInterceptsSaveFile(t *testing.T) {
	driver, err := NewDriver(Config{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()

	options := SaveFileOptions{
		Title: "Download", SuggestedName: "report.txt", DefaultExtension: "txt",
		Filters: []FileFilter{{Name: "Text", Pattern: "*.txt"}},
	}
	if err := driver.QueueSaveFile("/tmp/report.txt", nil); err != nil {
		t.Fatal(err)
	}
	path, err := SaveFile(options)
	if err != nil {
		t.Fatal(err)
	}
	if path != "/tmp/report.txt" {
		t.Fatalf("path = %q, want /tmp/report.txt", path)
	}
	request, ok := driver.LastSaveFileRequest()
	if !ok || request.Title != options.Title || request.SuggestedName != options.SuggestedName || len(request.Filters) != 1 {
		t.Fatalf("request = %#v, %t", request, ok)
	}

	if err := driver.QueueSaveFile("", ErrCanceled); err != nil {
		t.Fatal(err)
	}
	if _, err := SaveFile(SaveFileOptions{}); !errors.Is(err, ErrCanceled) {
		t.Fatalf("canceled SaveFile error = %v", err)
	}
	if _, err := SaveFile(SaveFileOptions{}); !errors.Is(err, ErrNoDriverResponse) {
		t.Fatalf("unanswered SaveFile error = %v", err)
	}
}

func TestDriverInterceptsOpenFiles(t *testing.T) {
	driver, err := NewDriver(Config{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()

	queued := []string{"/tmp/id_ed25519", "/tmp/id_rsa"}
	if err := driver.QueueOpenFiles(queued, nil); err != nil {
		t.Fatal(err)
	}
	queued[0] = "changed"
	options := OpenFileOptions{Title: "Import keys", AllowMultiple: true}
	paths, err := OpenFiles(options)
	if err != nil || len(paths) != 2 || paths[0] != "/tmp/id_ed25519" || paths[1] != "/tmp/id_rsa" {
		t.Fatalf("OpenFiles = %v, %v", paths, err)
	}
	request, ok := driver.LastOpenFilesRequest()
	if !ok || request.Title != options.Title || !request.AllowMultiple {
		t.Fatalf("request = %#v, %t", request, ok)
	}

	if err := driver.QueueOpenFiles(nil, ErrCanceled); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenFiles(OpenFileOptions{}); !errors.Is(err, ErrCanceled) {
		t.Fatalf("canceled OpenFiles error = %v", err)
	}
	if _, err := OpenFiles(OpenFileOptions{}); !errors.Is(err, ErrNoDriverResponse) {
		t.Fatalf("unanswered OpenFiles error = %v", err)
	}
}

func TestDriverInterceptsClipboardAndOpenURL(t *testing.T) {
	driver, err := NewDriver(Config{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()

	driver.SetClipboard("before")
	if value, err := ClipboardText(); err != nil || value != "before" {
		t.Fatalf("ClipboardText = %q, %v", value, err)
	}
	if err := SetClipboardText("remote text"); err != nil {
		t.Fatal(err)
	}
	if driver.Clipboard() != "remote text" {
		t.Fatalf("driver clipboard = %q", driver.Clipboard())
	}

	const target = "https://example.test/path"
	if err := OpenURL(target); err != nil {
		t.Fatal(err)
	}
	if opened, ok := driver.LastOpenedURL(); !ok || opened != target {
		t.Fatalf("LastOpenedURL = %q, %t", opened, ok)
	}
}

func TestDriverKeepsSecretsInMemoryAndNeverAsksTheSystem(t *testing.T) {
	driver, err := NewDriver(Config{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()

	service, account := "viva-driver-test", "vault key"
	if _, err := LoadSecret(service, account); !errors.Is(err, ErrSecretNotFound) {
		t.Fatalf("LoadSecret of nothing = %v", err)
	}
	secret := []byte("0123456789abcdef")
	if err := StoreSecret(service, account, secret); err != nil {
		t.Fatal(err)
	}
	secret[0] = 'X' // the store holds its own copy
	got, err := LoadSecret(service, account)
	if err != nil || string(got) != "0123456789abcdef" {
		t.Fatalf("LoadSecret = %q, %v", got, err)
	}
	got[1] = 'Y' // and hands out copies
	if again, _ := LoadSecret(service, account); string(again) != "0123456789abcdef" {
		t.Fatalf("a caller changed the stored secret: %q", again)
	}
	if err := StoreSecret(service, account, []byte("replaced")); err != nil {
		t.Fatal(err)
	}
	if got, _ := LoadSecret(service, account); string(got) != "replaced" {
		t.Fatalf("after replace = %q", got)
	}
	if err := DeleteSecret(service, account); err != nil {
		t.Fatal(err)
	}
	if err := DeleteSecret(service, account); err != nil {
		t.Fatalf("deleting what is gone = %v", err)
	}
	if _, err := LoadSecret(service, account); !errors.Is(err, ErrSecretNotFound) {
		t.Fatalf("after delete = %v", err)
	}
}

func TestDriverSystemValidationStillApplies(t *testing.T) {
	driver, err := NewDriver(Config{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()

	if err := SetClipboardText("bad\x00text"); err == nil {
		t.Fatal("SetClipboardText accepted NUL")
	}
	if err := OpenURL("file:///tmp/secret"); err == nil {
		t.Fatal("OpenURL accepted file URL")
	}
	if _, ok := driver.LastOpenedURL(); ok {
		t.Fatal("invalid URL reached Driver")
	}
}

func TestDriverSaveFileQueueValidation(t *testing.T) {
	driver, err := NewDriver(Config{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()

	if err := driver.QueueSaveFile("", nil); err == nil {
		t.Fatal("empty successful path was accepted")
	}
	for range driverSaveFileQueueSize {
		if err := driver.QueueSaveFile("/tmp/file", nil); err != nil {
			t.Fatal(err)
		}
	}
	if err := driver.QueueSaveFile("/tmp/overflow", nil); err == nil {
		t.Fatal("queue overflow was accepted")
	}
}

func TestDriverOpenFilesQueueValidation(t *testing.T) {
	driver, err := NewDriver(Config{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	if err := driver.QueueOpenFiles(nil, nil); err == nil {
		t.Fatal("empty successful paths were accepted")
	}
	if err := driver.QueueOpenFiles([]string{""}, nil); err == nil {
		t.Fatal("empty path was accepted")
	}
	for range driverOpenFilesQueueSize {
		if err := driver.QueueOpenFiles([]string{"/tmp/key"}, nil); err != nil {
			t.Fatal(err)
		}
	}
	if err := driver.QueueOpenFiles([]string{"/tmp/overflow"}, nil); err == nil {
		t.Fatal("queue overflow was accepted")
	}
}

func TestDriverSystemIntegrationsFollowTheActiveWindow(t *testing.T) {
	first, err := NewDriver(Config{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	second, err := NewDriver(Config{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	if err := first.QueueSaveFile("/tmp/first", nil); err != nil {
		t.Fatal(err)
	}
	if err := second.QueueSaveFile("/tmp/second", nil); err != nil {
		t.Fatal(err)
	}
	if err := first.Pump(); err != nil {
		t.Fatal(err)
	}
	if path, err := SaveFile(SaveFileOptions{}); err != nil || path != "/tmp/first" {
		t.Fatalf("first SaveFile = %q, %v", path, err)
	}
	if err := second.Pump(); err != nil {
		t.Fatal(err)
	}
	if path, err := SaveFile(SaveFileOptions{}); err != nil || path != "/tmp/second" {
		t.Fatalf("second SaveFile = %q, %v", path, err)
	}
}

func BenchmarkDriverSaveFile(b *testing.B) {
	driver, err := NewDriver(Config{}, nil)
	if err != nil {
		b.Fatal(err)
	}
	defer driver.Close()
	options := SaveFileOptions{SuggestedName: "report.txt"}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if err := driver.QueueSaveFile("/tmp/report.txt", nil); err != nil {
			b.Fatal(err)
		}
		if _, err := SaveFile(options); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDriverOpenFiles(b *testing.B) {
	driver, err := NewDriver(Config{}, nil)
	if err != nil {
		b.Fatal(err)
	}
	defer driver.Close()
	paths := []string{"/tmp/id_ed25519"}
	options := OpenFileOptions{AllowMultiple: true}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if err := driver.QueueOpenFiles(paths, nil); err != nil {
			b.Fatal(err)
		}
		if _, err := OpenFiles(options); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDriverClipboardAndOpenURL(b *testing.B) {
	driver, err := NewDriver(Config{}, nil)
	if err != nil {
		b.Fatal(err)
	}
	defer driver.Close()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if err := SetClipboardText("remote text"); err != nil {
			b.Fatal(err)
		}
		if value, err := ClipboardText(); err != nil || value != "remote text" {
			b.Fatalf("ClipboardText = %q, %v", value, err)
		}
		if err := OpenURL("https://example.test/docs"); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDriverClipboardHTML(b *testing.B) {
	driver, err := NewDriver(Config{}, nil)
	if err != nil {
		b.Fatal(err)
	}
	defer driver.Close()
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if err := SetClipboardHTML("<b>remote</b>", "remote text"); err != nil {
			b.Fatal(err)
		}
		if value, err := ClipboardText(); err != nil || value != "remote text" {
			b.Fatalf("ClipboardText = %q, %v", value, err)
		}
		if html := driver.ClipboardHTML(); html != "<b>remote</b>" {
			b.Fatalf("ClipboardHTML = %q", html)
		}
	}
}

func TestDriverInterceptsOpenDirectory(t *testing.T) {
	driver, err := NewDriver(Config{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()

	if err := driver.QueueOpenDirectory("/tmp/project", nil); err != nil {
		t.Fatal(err)
	}
	options := OpenDirectoryOptions{Title: "Choose project"}
	path, err := OpenDirectory(options)
	if err != nil || path != "/tmp/project" {
		t.Fatalf("OpenDirectory = %q, %v", path, err)
	}
	request, ok := driver.LastOpenDirectoryRequest()
	if !ok || request.Title != options.Title {
		t.Fatalf("request = %#v, %t", request, ok)
	}

	if err := driver.QueueOpenDirectory("", ErrCanceled); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenDirectory(OpenDirectoryOptions{}); !errors.Is(err, ErrCanceled) {
		t.Fatalf("canceled OpenDirectory error = %v", err)
	}
	if _, err := OpenDirectory(OpenDirectoryOptions{}); !errors.Is(err, ErrNoDriverResponse) {
		t.Fatalf("unanswered OpenDirectory error = %v", err)
	}

	if err := driver.QueueOpenDirectory("/tmp/first", nil); err != nil {
		t.Fatal(err)
	}
	if err := driver.QueueOpenDirectory("/tmp/second", nil); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"/tmp/first", "/tmp/second"} {
		path, err := OpenDirectory(OpenDirectoryOptions{})
		if err != nil || path != want {
			t.Fatalf("queued OpenDirectory = %q, %v; want %q", path, err, want)
		}
	}
}

func BenchmarkDriverOpenDirectory(b *testing.B) {
	driver, err := NewDriver(Config{}, nil)
	if err != nil {
		b.Fatal(err)
	}
	defer driver.Close()
	options := OpenDirectoryOptions{Title: "Choose project"}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if err := driver.QueueOpenDirectory("/tmp/project", nil); err != nil {
			b.Fatal(err)
		}
		if _, err := OpenDirectory(options); err != nil {
			b.Fatal(err)
		}
	}
}

func TestDriverRecordsOpenPathAndOpensNothing(t *testing.T) {
	driver, err := NewDriver(Config{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()

	file := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := OpenPath(file); err != nil {
		t.Fatal(err)
	}
	if opened, ok := driver.LastOpenedPath(); !ok || opened != file {
		t.Fatalf("LastOpenedPath = %q, %t", opened, ok)
	}
	directory := t.TempDir()
	if err := driver.QueueOpenDirectory(directory, nil); err != nil {
		t.Fatal(err)
	}
	path, err := OpenDirectoryPath(OpenDirectoryOptions{Title: "Choose project"})
	if err != nil || path != directory {
		t.Fatalf("OpenDirectoryPath = %q, %v; want %q", path, err, directory)
	}
	if opened, _ := driver.LastOpenedPath(); opened != file {
		t.Fatalf("OpenDirectoryPath unexpectedly opened %q", opened)
	}
	if err := driver.QueueOpenDirectory("", ErrCanceled); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenDirectoryPath(OpenDirectoryOptions{}); !errors.Is(err, ErrCanceled) {
		t.Fatalf("canceled OpenDirectoryPath error = %v", err)
	}
	if opened, ok := driver.LastOpenedPath(); !ok || opened != file {
		t.Fatalf("LastOpenedPath = %q, %t", opened, ok)
	}
	program := filepath.Join(t.TempDir(), "run.sh")
	if err := os.WriteFile(program, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" {
		if err := OpenPath(program); !errors.Is(err, ErrExecutablePath) {
			t.Fatalf("an executable: %v", err)
		}
		if opened, _ := driver.LastOpenedPath(); opened != file {
			t.Fatalf("a refused file reached the driver: %q", opened)
		}
	}
}

func TestWatchingTheAppearanceOfADriverBackedRunNeverReachesTheSystem(t *testing.T) {
	driver, err := NewDriver(Config{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	seen := 0
	stop := WatchSystemAppearance(func() { seen++ })
	defer stop()
	driver.SetSystemAppearance(AppearanceDark)
	if seen != 1 {
		t.Fatalf("watcher heard %d changes", seen)
	}
}

func TestAToolRunnerAnswersForTheSystemBeforeAnyWindowOpens(t *testing.T) {
	UseApplicationRunner(func(func(Host) error) error { return nil })
	t.Cleanup(func() { UseApplicationRunner(nil) })
	// An application builds its state, and pins the appearance, before it opens
	// a window. With no Driver yet this must not reach the system: on macOS that
	// waits on a main thread that a tool run never serves.
	SetNativeAppearance(AppearanceDark)
	if got := SystemAppearance(); got != AppearanceUnknown {
		t.Fatalf("SystemAppearance = %v, want the driver's unknown, not the machine's", got)
	}
}

func TestDriverKeepsThePrimarySelectionApartFromTheClipboardWhereThereIsOne(t *testing.T) {
	driver, err := NewDriver(Config{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()

	if err := SetClipboardText("copied"); err != nil {
		t.Fatal(err)
	}
	err = SetPrimarySelection("selected")
	if runtime.GOOS != "linux" {
		if !errors.Is(err, ErrUnsupported) {
			t.Fatalf("SetPrimarySelection error = %v, want ErrUnsupported", err)
		}
		if _, err := PrimarySelection(); !errors.Is(err, ErrUnsupported) {
			t.Fatalf("PrimarySelection error = %v, want ErrUnsupported", err)
		}
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	if text, err := PrimarySelection(); err != nil || text != "selected" {
		t.Fatalf("PrimarySelection = %q, %v", text, err)
	}
	if text, err := ClipboardText(); err != nil || text != "copied" {
		t.Fatalf("ClipboardText = %q, %v: selecting changed the clipboard", text, err)
	}
	if err := SetPrimarySelection("a\x00b"); err == nil {
		t.Fatal("text with NUL was accepted")
	}
}
