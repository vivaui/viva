//go:build js && wasm

package app

import (
	"syscall/js"
	"testing"
)

func TestReduceMotionPreferenceReadsTheBrowserMediaQuery(t *testing.T) {
	window := js.Global().Get("Object").New()
	window.Set("matchMedia", js.Global().Get("Function").New("query",
		`if (query !== "(prefers-reduced-motion: reduce)") throw new Error(query); return { matches: true };`))
	if !reduceMotionPreference(window) {
		t.Fatal("a reduced-motion preference was reported as false")
	}
	window.Set("matchMedia", js.Global().Get("Function").New("query", "return { matches: false };"))
	if reduceMotionPreference(window) {
		t.Fatal("a full-motion preference was reported as true")
	}
}
