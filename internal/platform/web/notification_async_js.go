//go:build js && wasm

package web

import (
	"fmt"
	"syscall/js"
)

// RequestNotificationPermission calls Notification.requestPermission and
// reports whether the page may show notifications. done runs on the event loop,
// exactly once. Browsers that return a Promise and browsers that use the older
// callback form are both accepted.
func RequestNotificationPermission(done func(granted bool, err error)) error {
	if done == nil {
		return fmt.Errorf("%w: nil notification callback", ErrUnavailable)
	}
	notification := js.Global().Get("Notification")
	if notification.Type() != js.TypeFunction {
		return fmt.Errorf("%w: Notification is missing", ErrUnavailable)
	}
	switch notification.Get("permission").String() {
	case "granted":
		done(true, nil)
		return nil
	case "denied":
		done(false, nil)
		return nil
	}
	request := notification.Get("requestPermission")
	if request.Type() != js.TypeFunction {
		return fmt.Errorf("%w: Notification.requestPermission is missing", ErrUnavailable)
	}

	var release []js.Func
	finished := false
	finish := func(granted bool, err error) {
		if finished {
			return
		}
		finished = true
		for _, function := range release {
			function.Release()
		}
		done(granted, err)
	}
	callback := func(handler func([]js.Value) any) js.Func {
		function := js.FuncOf(func(_ js.Value, args []js.Value) any { return handler(args) })
		release = append(release, function)
		return function
	}
	fail := callback(func(args []js.Value) any {
		finish(false, clipboardError(args))
		return nil
	})
	answer := callback(func(args []js.Value) any {
		permission := ""
		if len(args) > 0 {
			permission = args[0].String()
		}
		switch permission {
		case "granted":
			finish(true, nil)
		case "denied":
			finish(false, nil)
		default:
			finish(false, fmt.Errorf("%w: notification permission was not decided", ErrPermissionDenied))
		}
		return nil
	})
	var result js.Value
	if err := catchJS("request notification permission", func() { result = notification.Call("requestPermission", answer) }); err != nil {
		finish(false, err)
		return nil
	}
	if result.Type() == js.TypeObject && result.Get("then").Type() == js.TypeFunction {
		if err := catchJS("wait for notification permission", func() { result.Call("then", answer, fail) }); err != nil {
			finish(false, err)
		}
	}
	return nil
}
