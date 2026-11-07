//go:build js && wasm

package web

import (
	"syscall/js"

	"github.com/vivaui/viva/ui"
)

// NewTextEngine returns the browser's shaped-line engine over the page's
// document, or nil when there is no document (for example under Node).
func NewTextEngine() ui.TextEngine {
	document := js.Global().Get("document")
	if document.Type() != js.TypeObject || document.IsNull() {
		return nil
	}
	return newLineEngine(newDOMMeasurer(document))
}
