//go:build linux

package linux

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/vivaui/viva/internal/dbus"
)

func TestRevealUsesFileManager1ReplyWithoutOpeningTheDirectory(t *testing.T) {
	uri := "file:///tmp/report%20one.txt"
	if err := revealThroughDesktop(uri, "/tmp", func(got string) error {
		if got != uri {
			t.Errorf("ShowItems URI = %q", got)
		}
		return nil
	}, func(string) (string, error) {
		t.Fatal("looked for a fallback after FileManager1 succeeded")
		return "", nil
	}, func(string, ...string) error {
		t.Fatal("launched a fallback after FileManager1 succeeded")
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestRevealFallsBackWhenFileManager1RepliesWithError(t *testing.T) {
	var called []string
	busError := &dbus.Error{Name: "org.freedesktop.DBus.Error.ServiceUnknown"}
	err := revealThroughDesktop("file:///tmp/report%20one.txt", "/tmp", func(string) error {
		called = append(called, "FileManager1")
		return busError
	}, func(name string) (string, error) {
		return name, nil
	}, func(program string, args ...string) error {
		called = append(called, program+":"+fmt.Sprint(args))
		if program == "xdg-open" {
			return errors.New("desktop opener rejected the directory")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"FileManager1", "xdg-open:[/tmp]", "gio:[open /tmp]"}
	if !reflect.DeepEqual(called, want) {
		t.Fatalf("attempts = %q, want %q", called, want)
	}
}

func TestRevealReportsAllFailuresWithoutRunningAnItem(t *testing.T) {
	busError := &dbus.Error{Name: "org.freedesktop.DBus.Error.NoReply"}
	err := revealThroughDesktop("file:///tmp/script.sh", "/tmp", func(string) error { return busError },
		func(name string) (string, error) { return name, nil },
		func(string, ...string) error { return errors.New("opener failed") })
	if !errors.Is(err, busError) {
		t.Fatalf("lost FileManager1 failure: %v", err)
	}
}

func TestRevealFallbackOpensContainingDirectoryWithLiteralArguments(t *testing.T) {
	directory := t.TempDir()
	file := filepath.Join(directory, "--do not execute.sh")
	if err := os.WriteFile(file, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	var program string
	var args []string
	err := revealThroughDesktop("file:///tmp/unused", directory, func(string) error { return errors.New("missing bus") },
		func(name string) (string, error) { return name, nil },
		func(name string, got ...string) error { program, args = name, got; return nil })
	if err != nil {
		t.Fatal(err)
	}
	if program != "xdg-open" || !reflect.DeepEqual(args, []string{directory}) {
		t.Fatalf("launched %q with %q instead of the containing directory", program, args)
	}
}
