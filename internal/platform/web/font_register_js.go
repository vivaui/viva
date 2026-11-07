//go:build js && wasm

package web

import (
	"crypto/sha256"
	"fmt"
	"strconv"
	"syscall/js"

	"github.com/vivaui/viva/internal/fontname"
)

// fontInvalidators clear the measurements of every engine in the page. The
// page normally has one canvas; the slice exists so a test or a host that
// mounts more than one still sees a registration take effect everywhere.
var fontInvalidators []func()

var registeredFonts = struct {
	byHash map[[32]byte]*registeredFont
}{byHash: map[[32]byte]*registeredFont{}}

type registeredFont struct {
	family string
	face   js.Value
	refs   int
}

// RegisterFontAsync loads data through the browser's FontFace API. The browser
// resolves the load through a Promise, so the synchronous RegisterFont cannot
// report its result. done runs on the event loop exactly once; the returned
// error means the request could not start. Registering the same bytes again
// shares the registration, and the last unregister removes the face.
func RegisterFontAsync(data []byte, done func(string, func(), error)) error {
	if done == nil {
		return fmt.Errorf("%w: nil font callback", ErrUnavailable)
	}
	parsed, ok := fontname.Read(data)
	if !ok || parsed.Family == "" {
		return fmt.Errorf("%w: the data is not a TrueType or OpenType font with a family name", ErrUnsupported)
	}
	hash := sha256.Sum256(data)
	if existing, ok := registeredFonts.byHash[hash]; ok {
		existing.refs++
		done(existing.family, unregisterFont(hash), nil)
		return nil
	}
	document := js.Global().Get("document")
	if document.Type() != js.TypeObject || document.IsNull() {
		return fmt.Errorf("%w: document is not present", ErrUnavailable)
	}
	fonts := document.Get("fonts")
	if fonts.Type() != js.TypeObject || fonts.IsNull() || fonts.Get("add").Type() != js.TypeFunction {
		return fmt.Errorf("%w: document.fonts is missing", ErrUnavailable)
	}
	fontFace := js.Global().Get("FontFace")
	if fontFace.Type() != js.TypeFunction {
		return fmt.Errorf("%w: FontFace is missing", ErrUnavailable)
	}
	family := parsed.Family
	buffer := js.Global().Get("Uint8Array").New(len(data))
	js.CopyBytesToJS(buffer, data)
	descriptors := object()
	descriptors.Set("weight", strconv.Itoa(normalizedWeight(parsed.Weight)))
	style := "normal"
	if parsed.Italic {
		style = "italic"
	}
	descriptors.Set("style", style)
	var face js.Value
	if err := catchJS("create font face", func() {
		face = fontFace.New(family, buffer, descriptors)
	}); err != nil {
		return err
	}
	var promise js.Value
	if err := catchJS("load font face", func() { promise = face.Call("load") }); err != nil {
		return err
	}
	if err := awaitPromise(promise, func(loaded js.Value) {
		entry := &registeredFont{family: family, face: face, refs: 1}
		registeredFonts.byHash[hash] = entry
		if err := catchJS("add font face", func() { fonts.Call("add", face) }); err != nil {
			delete(registeredFonts.byHash, hash)
			done("", nil, err)
			return
		}
		invalidateFonts()
		done(family, unregisterFont(hash), nil)
	}, func(err error) {
		done("", nil, fmt.Errorf("%w: loading the font: %v", ErrUnsupported, err))
	}); err != nil {
		return err
	}
	return nil
}

func unregisterFont(hash [32]byte) func() {
	registered := false
	return func() {
		if registered {
			return
		}
		registered = true
		entry, ok := registeredFonts.byHash[hash]
		if !ok {
			return
		}
		entry.refs--
		if entry.refs > 0 {
			return
		}
		if document := js.Global().Get("document"); document.Type() == js.TypeObject && !document.IsNull() {
			if fonts := document.Get("fonts"); fonts.Type() == js.TypeObject && !fonts.IsNull() {
				fonts.Call("delete", entry.face)
			}
		}
		delete(registeredFonts.byHash, hash)
		invalidateFonts()
	}
}

func normalizedWeight(weight int) int {
	if weight < 100 || weight > 900 {
		return 400
	}
	return (weight + 50) / 100 * 100
}

func invalidateFonts() {
	fontGeneration++
	for _, invalidate := range fontInvalidators {
		invalidate()
	}
}
