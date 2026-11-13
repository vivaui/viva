//go:build js && wasm

package web

import (
	"errors"
	"syscall/js"
	"testing"
	"time"
)

// writeClipboardHTML runs the rich copy against the installed stub and waits
// for the callback.
func writeClipboardHTML(t *testing.T, html, alt string) error {
	t.Helper()
	done := make(chan error, 1)
	if err := SetClipboardHTMLAsync(html, alt, func(err error) { done <- err }); err != nil {
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

// installClipboardItem gives the runtime under test a ClipboardItem, which the
// js/wasm test environment itself does not provide, built the way the browser
// builds one: an object that answers getType with the flavour's Blob.
func installClipboardItem(t *testing.T) {
	t.Helper()
	class := js.Global().Get("Function").New(`
		return class ClipboardItem {
			constructor(flavours) { this.flavours = flavours; }
			get types() { return Object.keys(this.flavours); }
			getType(type) { return this.flavours[type]; }
		}`).Invoke()
	js.Global().Set("ClipboardItem", class)
}

func TestSetClipboardHTMLAsyncWritesBothFlavours(t *testing.T) {
	installClipboardItem(t)
	installClipboard(t, `return {
		write: items => {
			const flavours = {};
			for (const type of items[0].types) {
				flavours[type] = items[0].getType(type);
			}
			globalThis.vivaWritten = flavours;
			return Promise.resolve();
		},
	}`)
	if err := writeClipboardHTML(t, "<b>加粗</b>", "加粗"); err != nil {
		t.Fatalf("write = %v", err)
	}
	value := js.Global().Get("vivaWritten")
	if value.Type() != js.TypeObject {
		t.Fatal("the ClipboardItem was not written")
	}
	// Both flavours must be on the one item, as the ClipboardItem spec takes them.
	if value.Get("text/html").Type() != js.TypeObject || value.Get("text/plain").Type() != js.TypeObject {
		t.Fatalf("the ClipboardItem carries neither MIME flavour: %v", value)
	}
}

func TestSetClipboardHTMLAsyncReportsBrowserRefusal(t *testing.T) {
	installClipboard(t, `return { write: items => Promise.reject(Object.assign(new Error("no"), { name: "NotAllowedError" })) }`)
	if err := writeClipboardHTML(t, "<b>x</b>", "x"); !errors.Is(err, ErrPermissionDenied) {
		t.Fatalf("refused write error = %v", err)
	}
	installClipboard(t, `return {}`)
	if err := writeClipboardHTML(t, "<b>x</b>", "x"); err == nil {
		t.Fatal("a missing clipboard API reported success")
	}
}
