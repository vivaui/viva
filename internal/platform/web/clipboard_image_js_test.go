//go:build js && wasm

package web

import (
	"errors"
	"syscall/js"
	"testing"
	"time"

	"github.com/vivaui/viva/internal/platform"
)

// installClipboard replaces navigator.clipboard with a stub whose read() resolves
// to one item offering the given types, or rejects with the named error.
func installClipboard(t *testing.T, script string) {
	t.Helper()
	stub := js.Global().Get("Function").New(script).Invoke()
	navigator := js.Global().Get("Object").New()
	navigator.Set("clipboard", stub)
	js.Global().Get("Object").Call("defineProperty", js.Global(), "navigator",
		map[string]any{"value": navigator, "configurable": true, "writable": true})
}

func readImage(t *testing.T) ([]byte, error) {
	t.Helper()
	type result struct {
		png []byte
		err error
	}
	done := make(chan result, 1)
	ReadClipboardImagePNG(func(png []byte, err error) { done <- result{png, err} })
	select {
	case got := <-done:
		return got.png, got.err
	case <-time.After(5 * time.Second):
		t.Fatal("the clipboard read never completed")
		return nil, nil
	}
}

// pngPicture is a 1x1 PNG: the clipboard check reads its header, so the bytes
// must decode as one.
const pngPicture = "new Uint8Array([0x89,0x50,0x4e,0x47,0x0d,0x0a,0x1a,0x0a,0x00,0x00,0x00,0x0d,0x49,0x48,0x44,0x52,0x00,0x00,0x00,0x01,0x00,0x00,0x00,0x01,0x08,0x06,0x00,0x00,0x00,0x1f,0x15,0xc4,0x89,0x00,0x00,0x00,0x0b,0x49,0x44,0x41,0x54,0x78,0x9c,0x63,0x60,0x00,0x02,0x00,0x00,0x05,0x00,0x01,0x7a,0x5e,0xab,0x3f,0x00,0x00,0x00,0x00,0x49,0x45,0x4e,0x44,0xae,0x42,0x60,0x82])"

func TestReadClipboardImagePNG(t *testing.T) {
	installClipboard(t, `return { read: () => Promise.resolve([
		{ types: ["text/plain"], getType: () => Promise.reject(new Error("text")) },
		{ types: ["image/png"], getType: () => Promise.resolve(new Blob([`+pngPicture+`])) },
	]) }`)
	png, err := readImage(t)
	if err != nil || len(png) != 68 || png[1] != 'P' {
		t.Fatalf("png = %d bytes, %v", len(png), err)
	}
}

func TestReadClipboardImageReportsEachFailure(t *testing.T) {
	installClipboard(t, `return { read: () => Promise.resolve([{ types: ["text/plain"] }]) }`)
	if _, err := readImage(t); !errors.Is(err, platform.ErrNoClipboardImage) {
		t.Fatalf("no image: error = %v", err)
	}
	installClipboard(t, `return { read: () => Promise.resolve([
		{ types: ["image/png"], getType: () => Promise.resolve(new Blob([new Uint8Array(12)])) }]) }`)
	if _, err := readImage(t); err == nil {
		t.Fatal("bytes that are not a PNG were accepted")
	}
	installClipboard(t, `return { read: () => Promise.reject(Object.assign(new Error("no"), { name: "NotAllowedError" })) }`)
	if _, err := readImage(t); !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("denied: error = %v", err)
	}
	installClipboard(t, `return {}`)
	if _, err := readImage(t); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("no API: error = %v", err)
	}
}
