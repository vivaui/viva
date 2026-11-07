//go:build js && wasm

package web

import (
	"fmt"
	"syscall/js"
)

// ClipboardTextAsync reads navigator.clipboard.readText. The browser resolves a
// Promise; done runs on the event loop, exactly once. The call itself retains
// the user gesture, so invoke it from an input handler when the browser asks
// for permission.
func ClipboardTextAsync(done func(string, error)) error {
	if done == nil {
		return fmt.Errorf("%w: nil clipboard callback", ErrUnavailable)
	}
	promise, err := clipboardCall("readText")
	if err != nil {
		return err
	}
	var release []js.Func
	finish := func(value string, err error) {
		for _, function := range release {
			function.Release()
		}
		done(value, err)
	}
	callback := func(handler func([]js.Value) any) js.Func {
		function := js.FuncOf(func(_ js.Value, args []js.Value) any { return handler(args) })
		release = append(release, function)
		return function
	}
	fail := callback(func(args []js.Value) any {
		finish("", clipboardError(args))
		return nil
	})
	then := callback(func(args []js.Value) any {
		value := ""
		if len(args) > 0 {
			value = args[0].String()
		}
		finish(value, nil)
		return nil
	})
	if err := catchJS("read clipboard text", func() { promise.Call("then", then, fail) }); err != nil {
		finish("", err)
	}
	return nil
}

// SetClipboardTextAsync writes navigator.clipboard.writeText. The browser
// resolves a Promise; done runs on the event loop, exactly once.
func SetClipboardTextAsync(value string, done func(error)) error {
	if done == nil {
		return fmt.Errorf("%w: nil clipboard callback", ErrUnavailable)
	}
	promise, err := clipboardCall("writeText", value)
	if err != nil {
		return err
	}
	var release []js.Func
	finish := func(err error) {
		for _, function := range release {
			function.Release()
		}
		done(err)
	}
	callback := func(handler func([]js.Value) any) js.Func {
		function := js.FuncOf(func(_ js.Value, args []js.Value) any { return handler(args) })
		release = append(release, function)
		return function
	}
	fail := callback(func(args []js.Value) any {
		finish(clipboardError(args))
		return nil
	})
	then := callback(func([]js.Value) any {
		finish(nil)
		return nil
	})
	if err := catchJS("write clipboard text", func() { promise.Call("then", then, fail) }); err != nil {
		finish(err)
	}
	return nil
}

func clipboardCall(name string, arguments ...any) (js.Value, error) {
	navigator := js.Global().Get("navigator")
	if navigator.Type() != js.TypeObject || navigator.IsNull() {
		return js.Value{}, fmt.Errorf("%w: navigator is missing", ErrUnavailable)
	}
	clipboard := navigator.Get("clipboard")
	if clipboard.Type() != js.TypeObject || clipboard.IsNull() {
		return js.Value{}, fmt.Errorf("%w: navigator.clipboard is missing", ErrUnavailable)
	}
	if clipboard.Get(name).Type() != js.TypeFunction {
		return js.Value{}, fmt.Errorf("%w: navigator.clipboard.%s is missing", ErrUnavailable, name)
	}
	var promise js.Value
	if err := catchJS("call clipboard."+name, func() { promise = clipboard.Call(name, arguments...) }); err != nil {
		return js.Value{}, err
	}
	return promise, nil
}

func clipboardError(args []js.Value) error {
	if len(args) > 0 && args[0].Type() == js.TypeObject && args[0].Get("name").String() == "NotAllowedError" {
		return ErrPermissionDenied
	}
	return ErrUnavailable
}
