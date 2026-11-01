//go:build js && wasm

package web

import (
	"sync/atomic"
	"syscall/js"
)

// bindWaker lets background goroutines request a repaint through config.Waker.
// A wake is posted to the browser's event loop rather than run inline, so
// OnWake and the paint that follows never interleave with an input handler;
// wakes that arrive before it runs share one repaint. The returned function
// unbinds the waker.
func (runtime *runtime) bindWaker() (unbind func()) {
	waker := runtime.config.Waker
	if waker == nil {
		return func() {}
	}
	var scheduled atomic.Bool
	callback := js.FuncOf(func(js.Value, []js.Value) any {
		scheduled.Store(false)
		if runtime.config.OnWake != nil {
			runtime.config.OnWake()
		}
		runtime.painter.repaint()
		runtime.requestFrame()
		return nil
	})
	waker.Bind(func() {
		if scheduled.CompareAndSwap(false, true) {
			js.Global().Call("setTimeout", callback, 0)
		}
	})
	return func() {
		waker.Unbind()
		callback.Release()
	}
}
