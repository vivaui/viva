//go:build js && wasm

package web

import (
	"fmt"
	"syscall/js"
)

// RequestFullscreen starts entering or leaving the page's fullscreen mode. A
// nil error means the browser accepted the request; the transition itself is
// reported by the document's fullscreenchange event. Browsers require a user
// gesture and may reject the Promise later, which this synchronous form cannot
// wait for.
func RequestFullscreen(fullscreen bool) error {
	document := js.Global().Get("document")
	if document.Type() != js.TypeObject || document.IsNull() {
		return fmt.Errorf("%w: document is not present", ErrUnavailable)
	}
	for _, property := range []string{"fullscreenEnabled", "webkitFullscreenEnabled"} {
		if enabled := document.Get(property); enabled.Type() == js.TypeBoolean && !enabled.Bool() {
			return fmt.Errorf("%w: fullscreen is disabled by policy", ErrPermissionDenied)
		}
	}
	root := document.Get("documentElement")
	if root.Type() != js.TypeObject || root.IsNull() {
		return fmt.Errorf("%w: documentElement is not present", ErrUnavailable)
	}
	if fullscreen {
		if request := root.Get("requestFullscreen"); request.Type() == js.TypeFunction {
			return catchJS("request fullscreen", func() { ignorePromise(request.Call("call", root)) })
		}
		if request := root.Get("webkitRequestFullscreen"); request.Type() == js.TypeFunction {
			return catchJS("request fullscreen", func() { request.Call("call", root) })
		}
		return fmt.Errorf("%w: the Fullscreen API is missing", ErrUnsupported)
	}
	if exit := document.Get("exitFullscreen"); exit.Type() == js.TypeFunction {
		return catchJS("exit fullscreen", func() { ignorePromise(exit.Call("call", document)) })
	}
	if exit := document.Get("webkitExitFullscreen"); exit.Type() == js.TypeFunction {
		return catchJS("exit fullscreen", func() { exit.Call("call", document) })
	}
	return fmt.Errorf("%w: the Fullscreen API is missing", ErrUnsupported)
}

// ignorePromise keeps a rejected Promise from becoming an unhandled rejection;
// the synchronous request cannot report the browser's later answer.
func ignorePromise(promise js.Value) {
	if promise.Type() != js.TypeObject || promise.Get("then").Type() != js.TypeFunction {
		return
	}
	var success, failure js.Func
	release := func() {
		success.Release()
		failure.Release()
	}
	success = js.FuncOf(func(js.Value, []js.Value) any {
		release()
		return nil
	})
	failure = js.FuncOf(func(js.Value, []js.Value) any {
		release()
		return nil
	})
	if err := catchJS("settle fullscreen request", func() { promise.Call("then", success, failure) }); err != nil {
		release()
	}
}
