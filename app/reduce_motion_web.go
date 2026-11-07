//go:build js && wasm

package app

import "syscall/js"

func platformReduceMotion() bool { return reduceMotionPreference(js.Global().Get("window")) }

func reduceMotionPreference(window js.Value) bool {
	if window.Type() == js.TypeUndefined || window.IsNull() || window.Get("matchMedia").Type() != js.TypeFunction {
		return false
	}
	return window.Call("matchMedia", "(prefers-reduced-motion: reduce)").Get("matches").Bool()
}
