//go:build js && wasm

package web

import (
	"syscall/js"

	"github.com/vivaui/viva/gfx"
)

// NewTextEngine returns the browser's shaped-line engine over the page's
// document, or nil when there is no document (for example under Node).
func NewTextEngine() gfx.TextEngine {
	document := js.Global().Get("document")
	if document.Type() != js.TypeObject || document.IsNull() {
		return nil
	}
	return newLineEngine(newDOMMeasurer(document))
}
