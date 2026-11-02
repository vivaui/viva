//go:build js && wasm

package web

import (
	"errors"
	"fmt"
	"sync"
	"syscall/js"

	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/ui"
)

var (
	ErrUnsupported         = errors.New("web: browser capability is unsupported")
	ErrUnavailable         = errors.New("web: browser capability is unavailable")
	ErrAsyncRequired       = errors.New("web: browser capability requires an asynchronous API")
	ErrUserGestureRequired = errors.New("web: browser capability requires a user gesture")
	ErrPermissionDenied    = errors.New("web: browser permission was denied")
)

func ClipboardText() (string, error) {
	return "", fmt.Errorf("%w: Clipboard.readText returns a Promise", ErrAsyncRequired)
}

func SetClipboardText(string) error {
	return fmt.Errorf("%w: Clipboard.writeText returns a Promise", ErrAsyncRequired)
}

// ClipboardImagePNG cannot answer synchronously: Clipboard.read returns a Promise.
func ClipboardImagePNG() error {
	return fmt.Errorf("%w: Clipboard.read returns a Promise", ErrAsyncRequired)
}

// ReadClipboardImagePNG reads the first image/png of navigator.clipboard.read()
// and calls done with its bytes once the browser has delivered them: with
// platform.ErrNoClipboardImage when the clipboard holds none,
// platform.ErrClipboardImageTooLarge above the cap, and with
// ErrPermissionDenied or ErrUnavailable when the browser refuses or has no such
// API. The browser asks for permission and wants a user gesture, so call it
// from an input handler. done runs on the event loop, exactly once.
func ReadClipboardImagePNG(done func(png []byte, err error)) {
	navigator := js.Global().Get("navigator")
	var clipboard js.Value
	if navigator.Type() != js.TypeUndefined && !navigator.IsNull() {
		clipboard = navigator.Get("clipboard")
	}
	if clipboard.Type() != js.TypeObject || clipboard.Get("read").Type() != js.TypeFunction {
		done(nil, fmt.Errorf("%w: navigator.clipboard.read is missing", ErrUnavailable))
		return
	}
	var release []js.Func
	finish := func(png []byte, err error) {
		for _, function := range release {
			function.Release()
		}
		done(png, err)
	}
	callback := func(handler func(args []js.Value) any) js.Func {
		function := js.FuncOf(func(_ js.Value, args []js.Value) any { return handler(args) })
		release = append(release, function)
		return function
	}
	fail := callback(func(args []js.Value) any {
		err := ErrUnavailable
		if len(args) > 0 && args[0].Type() == js.TypeObject && args[0].Get("name").String() == "NotAllowedError" {
			err = ErrPermissionDenied
		}
		finish(nil, err)
		return nil
	})
	received := callback(func(args []js.Value) any {
		buffer := args[0]
		if size := buffer.Get("byteLength").Int(); size > platform.MaxClipboardImageBytes {
			finish(nil, platform.ErrClipboardImageTooLarge)
			return nil
		}
		png := make([]byte, buffer.Get("byteLength").Int())
		js.CopyBytesToGo(png, js.Global().Get("Uint8Array").New(buffer))
		finish(png, platform.CheckClipboardPNG(png))
		return nil
	})
	gotBlob := callback(func(args []js.Value) any {
		args[0].Call("arrayBuffer").Call("then", received, fail)
		return nil
	})
	gotItems := callback(func(args []js.Value) any {
		items := args[0]
		for index := 0; index < items.Length(); index++ {
			item := items.Index(index)
			types := item.Get("types")
			for typeIndex := 0; typeIndex < types.Length(); typeIndex++ {
				if types.Index(typeIndex).String() == "image/png" {
					item.Call("getType", "image/png").Call("then", gotBlob, fail)
					return nil
				}
			}
		}
		finish(nil, platform.ErrNoClipboardImage)
		return nil
	})
	if err := catchJS("read clipboard image", func() { clipboard.Call("read").Call("then", gotItems, fail) }); err != nil {
		finish(nil, err)
	}
}

func setClipboardTextFromGesture(value string) {
	navigator := js.Global().Get("navigator")
	if navigator.Type() == js.TypeUndefined || navigator.IsNull() {
		return
	}
	clipboard := navigator.Get("clipboard")
	if clipboard.Type() == js.TypeUndefined || clipboard.IsNull() {
		return
	}
	_ = catchJS("set clipboard from input event", func() { clipboard.Call("writeText", value) })
}

// PreferredLanguages is the browser's navigator.languages, most preferred first.
func PreferredLanguages() []string {
	navigator := js.Global().Get("navigator")
	if navigator.Type() == js.TypeUndefined || navigator.IsNull() {
		return nil
	}
	list := navigator.Get("languages")
	if list.Type() != js.TypeObject {
		return nil
	}
	names := make([]string, 0, list.Length())
	for index := 0; index < list.Length(); index++ {
		if name := list.Index(index); name.Type() == js.TypeString {
			names = append(names, name.String())
		}
	}
	return names
}

// darkMedia is the browser's prefers-color-scheme: dark query, the one list the
// page's setting and the system's both feed.
func darkMedia() js.Value {
	window := js.Global()
	if window.Type() == js.TypeUndefined || window.Get("matchMedia").Type() != js.TypeFunction {
		return js.Undefined()
	}
	return window.Call("matchMedia", "(prefers-color-scheme: dark)")
}

// DarkAppearance reports whether the browser asks for dark pages. known is false
// where the browser has no such query.
func DarkAppearance() (dark, known bool) {
	media := darkMedia()
	if media.Type() != js.TypeObject {
		return false, false
	}
	return media.Get("matches").Bool(), true
}

var watchAppearance sync.Once

// WatchAppearance has the browser call NotifyAppearanceChanged when the
// preference changes. It starts once.
func WatchAppearance() {
	watchAppearance.Do(func() {
		media := darkMedia()
		if media.Type() != js.TypeObject {
			return
		}
		media.Call("addEventListener", "change", js.FuncOf(func(js.Value, []js.Value) any {
			platform.NotifyAppearanceChanged()
			return nil
		}))
	})
}

func OpenFiles() ([]string, error) {
	return nil, fmt.Errorf("%w: browser file selection completes through a Promise or DOM event", ErrAsyncRequired)
}

func SaveFile() (string, error) {
	return "", fmt.Errorf("%w: browser save selection completes through a Promise", ErrAsyncRequired)
}

func SetCursorShape(shape uint8) error {
	cursor, err := cursorCSS(shape)
	if err != nil {
		return err
	}
	document := js.Global().Get("document")
	if document.Type() == js.TypeUndefined || document.IsNull() {
		return fmt.Errorf("%w: document is not present", ErrUnavailable)
	}
	root := document.Get("documentElement")
	if root.Type() == js.TypeUndefined || root.IsNull() {
		return fmt.Errorf("%w: documentElement is not present", ErrUnavailable)
	}
	return catchJS("set cursor", func() {
		root.Get("style").Set("cursor", cursor)
	})
}

// SetCursorImage makes the picture the pointer over the page, through the CSS cursor property.
func SetCursorImage(image *ui.CursorImage) error {
	cursor, err := cursorImageCSS(image)
	if err != nil {
		return err
	}
	document := js.Global().Get("document")
	if document.Type() == js.TypeUndefined || document.IsNull() {
		return fmt.Errorf("%w: document is not present", ErrUnavailable)
	}
	root := document.Get("documentElement")
	if root.Type() == js.TypeUndefined || root.IsNull() {
		return fmt.Errorf("%w: documentElement is not present", ErrUnavailable)
	}
	return catchJS("set cursor image", func() {
		root.Get("style").Set("cursor", cursor)
	})
}

func OpenURL(target string) error {
	window := js.Global().Get("window")
	if window.Type() == js.TypeUndefined || window.IsNull() {
		return fmt.Errorf("%w: window is not present", ErrUnavailable)
	}
	var opened js.Value
	if err := catchJS("open external URL", func() {
		opened = window.Call("open", target, "_blank")
	}); err != nil {
		return err
	}
	if opened.Type() == js.TypeUndefined || opened.IsNull() {
		return fmt.Errorf("%w: the browser blocked window.open", ErrUserGestureRequired)
	}
	return catchJS("isolate external window", func() {
		opened.Set("opener", js.Null())
	})
}

func PlaySystemSound(uint8) error {
	return fmt.Errorf("%w: browsers expose no system-sound API", ErrUnsupported)
}

func SendNotification(title, body string) error {
	notification := js.Global().Get("Notification")
	if notification.Type() == js.TypeUndefined || notification.IsNull() {
		return fmt.Errorf("%w: Notification is not present", ErrUnavailable)
	}
	switch notification.Get("permission").String() {
	case "granted":
		return catchJS("send notification", func() {
			notification.New(title, map[string]any{"body": body})
		})
	case "denied":
		return fmt.Errorf("%w: notification permission", ErrPermissionDenied)
	default:
		return fmt.Errorf("%w: Notification.requestPermission returns a Promise", ErrAsyncRequired)
	}
}

func cursorCSS(shape uint8) (string, error) {
	values := [...]string{
		"default",
		"text",
		"pointer",
		"crosshair",
		"wait",
		"ew-resize",
		"ns-resize",
		"nwse-resize",
		"nesw-resize",
		"not-allowed",
		"none",
	}
	if int(shape) >= len(values) {
		return "", fmt.Errorf("web: invalid cursor %d", shape)
	}
	return values[shape], nil
}

func catchJS(action string, run func()) (err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("web: %s: %v", action, recovered)
		}
	}()
	run()
	return nil
}
