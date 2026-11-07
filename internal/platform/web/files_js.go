//go:build js && wasm

package web

import (
	"errors"
	"fmt"
	"strings"
	"syscall/js"

	"github.com/vivaui/viva/internal/platform"
)

// OpenFilesAsync shows the browser's file picker. It uses the File System
// Access API when the browser has it and a file input otherwise. done runs on
// the event loop, exactly once. Cancellation is platform.ErrCanceled and a
// picker refused outside direct input is platform.ErrUserGestureRequired where
// the browser reports it.
func OpenFilesAsync(options platform.OpenFilesOptions, done func([]platform.FileRef, error)) error {
	if done == nil {
		return fmt.Errorf("%w: nil file callback", ErrUnavailable)
	}
	if picker := js.Global().Get("showOpenFilePicker"); picker.Type() == js.TypeFunction {
		if err := openFilesWithPicker(picker, options, done); err == nil {
			return nil
		} else if !isMissingAPI(err) {
			return err
		}
	}
	return openFilesWithInput(options, done)
}

func openFilesWithPicker(picker js.Value, options platform.OpenFilesOptions, done func([]platform.FileRef, error)) error {
	arguments := object()
	arguments.Set("multiple", options.AllowMultiple)
	if types := pickerTypes(options.Filters); types.Truthy() {
		arguments.Set("types", types)
	}
	var promise js.Value
	if err := catchJS("show open file picker", func() { promise = picker.Call("call", js.Global(), arguments) }); err != nil {
		return err
	}
	return awaitPromise(promise, func(handles js.Value) {
		if handles.Length() == 0 {
			done(nil, nil)
			return
		}
		files := make([]platform.FileRef, handles.Length())
		remaining := handles.Length()
		finished := false
		fail := func(err error) {
			if finished {
				return
			}
			finished = true
			done(nil, err)
		}
		for index := 0; index < handles.Length(); index++ {
			handle := handles.Index(index)
			if err := awaitPromise(handle.Call("getFile"), func(file js.Value) {
				if finished {
					return
				}
				files[index] = newDOMFile(file)
				remaining--
				if remaining == 0 {
					finished = true
					done(files, nil)
				}
			}, fail); err != nil {
				fail(err)
				return
			}
		}
	}, func(err error) { done(nil, fileError(err)) })
}

func newDOMFile(file js.Value) platform.FileRef {
	return &domFile{
		file: file,
		name: file.Get("name").String(),
		size: int64(file.Get("size").Float()),
		mime: file.Get("type").String(),
	}
}

// FileRefFromDOM wraps one browser File delivered by a drop.
func FileRefFromDOM(file js.Value) platform.FileRef { return newDOMFile(file) }

type domFile struct {
	file js.Value
	name string
	size int64
	mime string
}

func (file *domFile) Name() string { return file.name }
func (file *domFile) Size() int64  { return file.size }
func (file *domFile) MIME() string { return file.mime }
func (file *domFile) IsDir() bool  { return false }

func (file *domFile) Read(offset int64, length int, done func([]byte, error)) error {
	if done == nil {
		return fmt.Errorf("%w: nil file callback", ErrUnavailable)
	}
	if offset < 0 || length < 0 {
		return fmt.Errorf("web: invalid file range offset=%d length=%d", offset, length)
	}
	if offset >= file.size || length == 0 {
		done([]byte{}, nil)
		return nil
	}
	end := offset + int64(length)
	if end > file.size {
		end = file.size
	}
	var promise js.Value
	if err := catchJS("slice file", func() {
		promise = file.file.Call("slice", float64(offset), float64(end)).Call("arrayBuffer")
	}); err != nil {
		return err
	}
	return awaitPromise(promise, func(buffer js.Value) {
		size := buffer.Get("byteLength").Int()
		data := make([]byte, size)
		if copied := js.CopyBytesToGo(data, js.Global().Get("Uint8Array").New(buffer)); copied != size {
			done(nil, fmt.Errorf("web: copy file bytes: copied %d of %d", copied, size))
			return
		}
		done(data, nil)
	}, func(err error) { done(nil, fileError(err)) })
}

// openFilesWithInput is the fallback for browsers without the File System
// Access API. The browser owns the dialog and exposes only the chosen File
// objects; cancel is reported by the input's cancel event where it exists and,
// on older browsers, when the window regains focus with no files chosen.
func openFilesWithInput(options platform.OpenFilesOptions, done func([]platform.FileRef, error)) error {
	document := js.Global().Get("document")
	if document.Type() != js.TypeObject || document.IsNull() {
		return fmt.Errorf("%w: document is not present", ErrUnavailable)
	}
	input := document.Call("createElement", "input")
	input.Set("type", "file")
	if options.AllowMultiple {
		input.Set("multiple", true)
	}
	if accept := acceptAttribute(options.Filters); accept != "" {
		input.Set("accept", accept)
	}
	style := input.Get("style")
	style.Set("position", "fixed")
	style.Set("left", "-10000px")
	style.Set("top", "0")
	document.Get("body").Call("appendChild", input)

	var change, cancel, focus, timer js.Func
	timerID := 0
	finished := false
	cleanup := func() {
		input.Call("removeEventListener", "change", change)
		input.Call("removeEventListener", "cancel", cancel)
		js.Global().Get("window").Call("removeEventListener", "focus", focus)
		if input.Get("parentNode").Truthy() {
			input.Get("parentNode").Call("removeChild", input)
		}
		if timerID != 0 {
			js.Global().Get("window").Call("clearTimeout", timerID)
			timerID = 0
		}
		for _, function := range []js.Func{change, cancel, focus, timer} {
			if function.Type() == js.TypeFunction {
				function.Release()
			}
		}
	}
	finish := func(files []platform.FileRef, err error) {
		if finished {
			return
		}
		finished = true
		cleanup()
		done(files, err)
	}
	chosen := func() {
		list := input.Get("files")
		if list.Type() == js.TypeUndefined || list.IsNull() || list.Length() == 0 {
			finish(nil, platform.ErrCanceled)
			return
		}
		files := make([]platform.FileRef, list.Length())
		for index := 0; index < list.Length(); index++ {
			files[index] = newDOMFile(list.Index(index))
		}
		finish(files, nil)
	}
	change = js.FuncOf(func(js.Value, []js.Value) any { chosen(); return nil })
	cancel = js.FuncOf(func(js.Value, []js.Value) any { finish(nil, platform.ErrCanceled); return nil })
	// Browsers without the input cancel event return focus to the page when
	// the dialog closes. A zero-delay check after focus sees the final file list.
	focus = js.FuncOf(func(js.Value, []js.Value) any {
		js.Global().Get("window").Call("removeEventListener", "focus", focus)
		timer = js.FuncOf(func(js.Value, []js.Value) any {
			if !finished {
				chosen()
			}
			return nil
		})
		timerID = js.Global().Get("window").Call("setTimeout", timer, 0).Int()
		return nil
	})
	input.Call("addEventListener", "change", change)
	input.Call("addEventListener", "cancel", cancel)
	js.Global().Get("window").Call("addEventListener", "focus", focus)
	if err := catchJS("open file picker", func() { input.Call("click") }); err != nil {
		finish(nil, err)
		return nil
	}
	return nil
}

// SaveFileAsync shows the browser's save picker when the File System Access
// API is available; otherwise it returns a sink that downloads the written
// bytes when closed. done runs on the event loop, exactly once.
func SaveFileAsync(options platform.SaveFileOptions, done func(platform.FileSink, error)) error {
	if done == nil {
		return fmt.Errorf("%w: nil file callback", ErrUnavailable)
	}
	name := options.SuggestedName
	if name != "" && options.DefaultExtension != "" && !strings.Contains(filepathBase(name), ".") {
		if extension := normalizedExtension(options.DefaultExtension); extension != "" {
			name += extension
		}
	}
	if name == "" {
		name = "download"
		if extension := normalizedExtension(options.DefaultExtension); extension != "" {
			name += extension
		}
	}
	picker := js.Global().Get("showSaveFilePicker")
	if picker.Type() != js.TypeFunction {
		done(&downloadSink{name: name}, nil)
		return nil
	}
	arguments := object()
	arguments.Set("suggestedName", name)
	if types := pickerTypes(options.Filters); types.Truthy() {
		arguments.Set("types", types)
	}
	var promise js.Value
	if err := catchJS("show save file picker", func() { promise = picker.Call("call", js.Global(), arguments) }); err != nil {
		return err
	}
	return awaitPromise(promise, func(handle js.Value) {
		if err := awaitPromise(handle.Call("createWritable"), func(writable js.Value) {
			name := handle.Get("name").String()
			if name == "" {
				name = options.SuggestedName
			}
			done(&streamSink{name: name, writable: writable}, nil)
		}, func(err error) { done(nil, fileError(err)) }); err != nil {
			done(nil, err)
		}
	}, func(err error) { done(nil, fileError(err)) })
}

type streamSink struct {
	name     string
	writable js.Value
	closed   bool
}

func (sink *streamSink) Name() string { return sink.name }

func (sink *streamSink) Write(data []byte, done func(error)) error {
	if done == nil {
		return fmt.Errorf("%w: nil file callback", ErrUnavailable)
	}
	if sink.closed {
		return fmt.Errorf("web: file %q is closed", sink.name)
	}
	buffer := js.Global().Get("Uint8Array").New(len(data))
	js.CopyBytesToJS(buffer, data)
	var promise js.Value
	if err := catchJS("write file", func() { promise = sink.writable.Call("write", buffer) }); err != nil {
		return err
	}
	return awaitPromise(promise, func(js.Value) { done(nil) }, func(err error) { done(fileError(err)) })
}

func (sink *streamSink) Close(done func(error)) error {
	if done == nil {
		return fmt.Errorf("%w: nil file callback", ErrUnavailable)
	}
	if sink.closed {
		done(nil)
		return nil
	}
	sink.closed = true
	var promise js.Value
	if err := catchJS("close file", func() { promise = sink.writable.Call("close") }); err != nil {
		return err
	}
	return awaitPromise(promise, func(js.Value) { done(nil) }, func(err error) { done(fileError(err)) })
}

type downloadSink struct {
	name   string
	chunks [][]byte
	closed bool
}

func (sink *downloadSink) Name() string { return sink.name }

func (sink *downloadSink) Write(data []byte, done func(error)) error {
	if done == nil {
		return fmt.Errorf("%w: nil file callback", ErrUnavailable)
	}
	if sink.closed {
		return fmt.Errorf("web: file %q is closed", sink.name)
	}
	sink.chunks = append(sink.chunks, append([]byte(nil), data...))
	done(nil)
	return nil
}

func (sink *downloadSink) Close(done func(error)) error {
	if done == nil {
		return fmt.Errorf("%w: nil file callback", ErrUnavailable)
	}
	if sink.closed {
		done(nil)
		return nil
	}
	sink.closed = true
	if err := download(sink.name, sink.chunks); err != nil {
		return err
	}
	sink.chunks = nil
	done(nil)
	return nil
}

func download(name string, chunks [][]byte) error {
	document := js.Global().Get("document")
	if document.Type() != js.TypeObject || document.IsNull() {
		return fmt.Errorf("%w: document is not present", ErrUnavailable)
	}
	parts := js.Global().Get("Array").New()
	for _, chunk := range chunks {
		buffer := js.Global().Get("Uint8Array").New(len(chunk))
		js.CopyBytesToJS(buffer, chunk)
		parts.Call("push", buffer)
	}
	var url string
	var anchor js.Value
	if err := catchJS("start download", func() {
		blob := js.Global().Get("Blob").New(parts, objectWith("type", "application/octet-stream"))
		url = js.Global().Get("URL").Call("createObjectURL", blob).String()
		anchor = document.Call("createElement", "a")
		anchor.Set("href", url)
		anchor.Set("download", name)
		document.Get("body").Call("appendChild", anchor)
		anchor.Call("click")
	}); err != nil {
		return err
	}
	if anchor.Truthy() && anchor.Get("parentNode").Truthy() {
		anchor.Get("parentNode").Call("removeChild", anchor)
	}
	if url != "" {
		var revoke js.Func
		revoke = js.FuncOf(func(js.Value, []js.Value) any {
			js.Global().Get("URL").Call("revokeObjectURL", url)
			revoke.Release()
			return nil
		})
		js.Global().Call("setTimeout", revoke, 0)
	}
	return nil
}

func object() js.Value { return js.Global().Get("Object").New() }

func objectWith(key string, value any) js.Value {
	object := object()
	object.Set(key, js.ValueOf(value))
	return object
}

func pickerTypes(filters []platform.FileFilter) js.Value {
	array := js.Global().Get("Array").New()
	for _, filter := range filters {
		extensions := patternExtensions(filter.Pattern)
		if len(extensions) == 0 {
			continue
		}
		accept := object()
		list := js.Global().Get("Array").New()
		for _, extension := range extensions {
			list.Call("push", extension)
		}
		accept.Set("application/octet-stream", list)
		entry := object()
		entry.Set("description", filter.Name)
		entry.Set("accept", accept)
		array.Call("push", entry)
	}
	if array.Length() == 0 {
		return js.Undefined()
	}
	return array
}

func acceptAttribute(filters []platform.FileFilter) string {
	patterns := make([]string, 0, len(filters))
	for _, filter := range filters {
		for _, pattern := range strings.Split(filter.Pattern, ";") {
			if pattern = strings.TrimSpace(pattern); pattern != "" {
				patterns = append(patterns, pattern)
			}
		}
	}
	return strings.Join(patterns, ",")
}

// patternExtensions turns semicolon-separated globs into the extensions the
// File System Access API takes. A pattern that is not a plain "*.<ext>" is left
// to the fallback input, whose accept attribute understands globs itself.
func patternExtensions(pattern string) []string {
	var extensions []string
	for _, part := range strings.Split(pattern, ";") {
		part = strings.TrimSpace(part)
		switch {
		case strings.HasPrefix(part, "*.") && !strings.ContainsAny(part[2:], "*?[]"):
			extensions = append(extensions, part[1:])
		case strings.HasPrefix(part, ".") && !strings.ContainsAny(part[1:], "*?[]"):
			extensions = append(extensions, part)
		}
	}
	return extensions
}

func normalizedExtension(extension string) string {
	extension = strings.TrimSpace(extension)
	if extension == "" {
		return ""
	}
	if !strings.HasPrefix(extension, ".") {
		extension = "." + extension
	}
	if strings.ContainsAny(extension, `/\*?[]:`) {
		return ""
	}
	return extension
}

func filepathBase(name string) string {
	if index := strings.LastIndexAny(name, `/\`); index >= 0 {
		return name[index+1:]
	}
	return name
}

// awaitPromise registers one-shot success and failure handlers. It releases
// both js.Func values before calling either handler, so a callback that starts
// another promise leaves nothing registered in the Go runtime.
func awaitPromise(promise js.Value, success func(js.Value), failure func(error)) error {
	if promise.Type() != js.TypeObject || promise.Get("then").Type() != js.TypeFunction {
		return fmt.Errorf("%w: the browser did not return a Promise", ErrUnavailable)
	}
	var onSuccess, onFailure js.Func
	released := false
	release := func() {
		if released {
			return
		}
		released = true
		onSuccess.Release()
		onFailure.Release()
	}
	onSuccess = js.FuncOf(func(_ js.Value, args []js.Value) any {
		release()
		value := js.Undefined()
		if len(args) > 0 {
			value = args[0]
		}
		success(value)
		return nil
	})
	onFailure = js.FuncOf(func(_ js.Value, args []js.Value) any {
		release()
		failure(jsFailure(args))
		return nil
	})
	if err := catchJS("wait for a browser Promise", func() { promise.Call("then", onSuccess, onFailure) }); err != nil {
		release()
		return err
	}
	return nil
}

func jsFailure(args []js.Value) error {
	if len(args) == 0 {
		return &browserError{message: "the browser rejected the operation"}
	}
	value := args[0]
	browser := &browserError{}
	if value.Type() == js.TypeObject {
		browser.name = value.Get("name").String()
		browser.message = value.Get("message").String()
	}
	if browser.message == "" {
		browser.message = value.String()
	}
	return browser
}

func fileError(err error) error {
	if err == nil {
		return nil
	}
	var browser *browserError
	if errors.As(err, &browser) {
		switch browser.name {
		case "AbortError":
			return platform.ErrCanceled
		case "NotAllowedError", "SecurityError":
			return fmt.Errorf("%w: %v", ErrPermissionDenied, err)
		}
		return fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	return err
}

type browserError struct {
	name    string
	message string
}

func (err *browserError) Error() string {
	if err.name != "" && err.message != "" {
		return err.name + ": " + err.message
	}
	if err.message != "" {
		return err.message
	}
	return err.name
}

func isMissingAPI(err error) bool {
	return strings.Contains(err.Error(), "not a function") || strings.Contains(err.Error(), "undefined")
}
