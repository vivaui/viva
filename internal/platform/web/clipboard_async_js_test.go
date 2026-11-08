//go:build js && wasm

package web

import (
	"errors"
	"testing"
	"time"
)

func clipboardText(t *testing.T) (string, error) {
	t.Helper()
	type result struct {
		value string
		err   error
	}
	done := make(chan result, 1)
	if err := ClipboardTextAsync(func(value string, err error) { done <- result{value, err} }); err != nil {
		return "", err
	}
	select {
	case got := <-done:
		return got.value, got.err
	case <-time.After(5 * time.Second):
		t.Fatal("the clipboard read never completed")
		return "", nil
	}
}

func writeClipboardText(t *testing.T, value string) error {
	t.Helper()
	done := make(chan error, 1)
	if err := SetClipboardTextAsync(value, func(err error) { done <- err }); err != nil {
		return err
	}
	select {
	case err := <-done:
		return err
	case <-time.After(5 * time.Second):
		t.Fatal("the clipboard write never completed")
		return nil
	}
}

func TestClipboardTextAsync(t *testing.T) {
	installClipboard(t, `return {
		readText: () => Promise.resolve("hello"),
		writeText: value => { if (value !== "world") return Promise.reject(new Error("wrong value")); return Promise.resolve() },
	}`)
	value, err := clipboardText(t)
	if err != nil || value != "hello" {
		t.Fatalf("read = %q, %v", value, err)
	}
	if err := writeClipboardText(t, "world"); err != nil {
		t.Fatalf("write = %v", err)
	}
}

func TestClipboardTextAsyncReportsBrowserRefusal(t *testing.T) {
	installClipboard(t, `return { readText: () => Promise.reject(Object.assign(new Error("no"), { name: "NotAllowedError" })) }`)
	if _, err := clipboardText(t); !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("read error = %v", err)
	}
	installClipboard(t, `return {}`)
	if _, err := clipboardText(t); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("missing API error = %v", err)
	}
}
