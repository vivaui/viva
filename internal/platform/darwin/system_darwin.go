//go:build darwin

package darwin

/*
#cgo LDFLAGS: -framework Cocoa -framework UserNotifications
#include <stdlib.h>
#include "system_darwin.h"
*/
import "C"

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
)

var ErrPermissionDenied = errors.New("darwin: permission denied")

type FileFilter struct {
	Name    string
	Pattern string
}

type OpenFileOptions struct {
	Title         string
	Filters       []FileFilter
	AllowMultiple bool
}

type OpenDirectoryOptions struct {
	Title string
}

type SaveFileOptions struct {
	Title            string
	SuggestedName    string
	DefaultExtension string
	Filters          []FileFilter
}

func ClipboardText() (string, error) {
	var value *C.char
	var nativeError *C.char
	status := C.viva_system_clipboard_text(&value, &nativeError)
	if status != 0 {
		return "", systemError("read clipboard", nativeError)
	}
	defer C.viva_system_free(unsafe.Pointer(value))
	return C.GoString(value), nil
}

func SetClipboardText(value string) error {
	nativeValue, err := systemCString("clipboard text", value)
	if err != nil {
		return err
	}
	defer C.free(unsafe.Pointer(nativeValue))
	var nativeError *C.char
	if C.viva_system_set_clipboard_text(nativeValue, &nativeError) != 0 {
		return systemError("set clipboard", nativeError)
	}
	return nil
}

// HasClipboardImage reports whether the general pasteboard holds public.png or
// public.tiff data. It reads no pixels.
func HasClipboardImage() bool { return C.viva_system_clipboard_has_image(nil) != 0 }

// ClipboardImagePNG returns the general pasteboard's image as PNG bytes: the
// public.png data as it is, or public.tiff converted by NSBitmapImageRep.
func ClipboardImagePNG() ([]byte, error) { return pasteboardImagePNG(nil) }

func pasteboardImagePNG(name *C.char) ([]byte, error) {
	var data *C.uint8_t
	var length C.size_t
	var nativeError *C.char
	switch C.viva_system_clipboard_image_png(name, C.size_t(platform.MaxClipboardImageBytes), C.size_t(platform.MaxClipboardRawBytes), &data, &length, &nativeError) {
	case 0:
		defer C.viva_system_free(unsafe.Pointer(data))
		png := C.GoBytes(unsafe.Pointer(data), C.int(length))
		if err := platform.CheckClipboardPNG(png); err != nil {
			return nil, err
		}
		return png, nil
	case 1:
		return nil, platform.ErrNoClipboardImage
	case 2:
		return nil, platform.ErrClipboardImageTooLarge
	}
	return nil, systemError("read clipboard image", nativeError)
}

// ReduceMotion reports the system "Reduce motion" accessibility preference.
func ReduceMotion() bool {
	return C.viva_system_reduce_motion() != 0
}

// PreferredLanguages is the user's language list, most preferred first, as macOS
// reports it ("zh-Hans-CN", "en-US").
func PreferredLanguages() []string {
	value := C.viva_system_preferred_languages()
	if value == nil {
		return nil
	}
	defer C.viva_system_free(unsafe.Pointer(value))
	text := C.GoString(value)
	if text == "" {
		return nil
	}
	return strings.Split(text, "\n")
}

// DarkAppearance reports whether the system is set to its dark look.
func DarkAppearance() bool {
	return C.viva_system_dark_appearance() != 0
}

// EffectiveDark reports whether what the application draws resolves to dark:
// the system's look, or the one SetAppearance pinned.
func EffectiveDark() bool {
	return C.viva_system_effective_dark() != 0
}

// SetAppearance pins the look of what the system draws for the application:
// light, dark, or (neither) the system's own.
func SetAppearance(light, dark bool) {
	mode := 0
	switch {
	case light:
		mode = 1
	case dark:
		mode = 2
	}
	C.viva_system_set_appearance(C.int(mode))
}

// WatchAppearance has the platform call NotifyAppearanceChanged whenever the
// system switches between its light and dark looks. It starts once.
func WatchAppearance() {
	C.viva_system_watch_appearance()
}

//export vivaSystemAppearanceChanged
func vivaSystemAppearanceChanged() {
	platform.NotifyAppearanceChanged()
}

// lastCursorShape is the highest shape number the native side knows. A number
// beyond it is refused here, without a trip to the main thread, which the native
// side would also refuse with the same words.
const lastCursorShape = 9

func SetCursorShape(shape uint8) error {
	if shape > lastCursorShape {
		return fmt.Errorf("darwin: set cursor: invalid cursor shape %d", shape)
	}
	var nativeError *C.char
	if C.viva_system_set_cursor(C.uint8_t(shape), &nativeError) != 0 {
		return systemError("set cursor", nativeError)
	}
	return nil
}

func OpenFiles(options OpenFileOptions) ([]string, bool, error) {
	title, err := systemCString("open file title", options.Title)
	if err != nil {
		return nil, false, err
	}
	defer C.free(unsafe.Pointer(title))
	extensions, err := allowedExtensions(options.Filters)
	if err != nil {
		return nil, false, err
	}
	nativeExtensions, freeExtensions, err := systemCStrings("file extension", extensions)
	if err != nil {
		return nil, false, err
	}
	defer freeExtensions()
	var result C.viva_system_string_array
	var nativeError *C.char
	status := C.viva_system_open_files(
		title,
		nativeStringPointer(nativeExtensions),
		C.size_t(len(nativeExtensions)),
		C.int(systemBoolInt(options.AllowMultiple)),
		0,
		&result,
		&nativeError,
	)
	runtime.KeepAlive(nativeExtensions)
	if status == 1 {
		return nil, true, nil
	}
	if status != 0 {
		return nil, false, systemError("open files", nativeError)
	}
	defer C.viva_system_free_string_array(result)
	items := unsafe.Slice((**C.char)(unsafe.Pointer(result.items)), int(result.count))
	paths := make([]string, len(items))
	for index, item := range items {
		paths[index] = C.GoString(item)
	}
	return paths, false, nil
}

func OpenDirectory(options OpenDirectoryOptions) (string, bool, error) {
	title, err := systemCString("open directory title", options.Title)
	if err != nil {
		return "", false, err
	}
	defer C.free(unsafe.Pointer(title))
	var result C.viva_system_string_array
	var nativeError *C.char
	status := C.viva_system_open_files(title, nil, 0, 0, 1, &result, &nativeError)
	if status == 1 {
		return "", true, nil
	}
	if status != 0 {
		return "", false, systemError("open directory", nativeError)
	}
	defer C.viva_system_free_string_array(result)
	if result.count != 1 {
		return "", false, fmt.Errorf("darwin: open directory returned %d paths", result.count)
	}
	items := unsafe.Slice((**C.char)(unsafe.Pointer(result.items)), 1)
	return C.GoString(items[0]), false, nil
}

func SaveFile(options SaveFileOptions) (string, bool, error) {
	title, err := systemCString("save file title", options.Title)
	if err != nil {
		return "", false, err
	}
	defer C.free(unsafe.Pointer(title))
	name, err := systemCString("suggested file name", options.SuggestedName)
	if err != nil {
		return "", false, err
	}
	defer C.free(unsafe.Pointer(name))
	defaultExtension, err := normalizeExtension(options.DefaultExtension)
	if err != nil {
		return "", false, fmt.Errorf("darwin: default extension: %w", err)
	}
	nativeDefaultExtension, err := systemCString("default extension", defaultExtension)
	if err != nil {
		return "", false, err
	}
	defer C.free(unsafe.Pointer(nativeDefaultExtension))
	extensions, err := allowedExtensions(options.Filters)
	if err != nil {
		return "", false, err
	}
	nativeExtensions, freeExtensions, err := systemCStrings("file extension", extensions)
	if err != nil {
		return "", false, err
	}
	defer freeExtensions()
	var path *C.char
	var nativeError *C.char
	status := C.viva_system_save_file(
		title,
		name,
		nativeDefaultExtension,
		nativeStringPointer(nativeExtensions),
		C.size_t(len(nativeExtensions)),
		&path,
		&nativeError,
	)
	runtime.KeepAlive(nativeExtensions)
	if status == 1 {
		return "", true, nil
	}
	if status != 0 {
		return "", false, systemError("save file", nativeError)
	}
	defer C.viva_system_free(unsafe.Pointer(path))
	return C.GoString(path), false, nil
}

func OpenURL(target string) error {
	nativeTarget, err := systemCString("external URL", target)
	if err != nil {
		return err
	}
	defer C.free(unsafe.Pointer(nativeTarget))
	var nativeError *C.char
	if C.viva_system_open_url(nativeTarget, &nativeError) != 0 {
		return systemError("open external URL", nativeError)
	}
	return nil
}

// SchemeHandlerAvailable reports whether an application is registered for a URL
// scheme such as "vscode", as the system's open would route it.
func SchemeHandlerAvailable(scheme string) (bool, error) {
	nativeScheme, err := systemCString("URL scheme", scheme)
	if err != nil {
		return false, err
	}
	defer C.free(unsafe.Pointer(nativeScheme))
	var nativeError *C.char
	status := C.viva_system_scheme_handler(nativeScheme, &nativeError)
	if status < 0 {
		return false, systemError("look up URL scheme handler", nativeError)
	}
	return status == 1, nil
}

func PlaySystemSound(sound uint8) error {
	var nativeError *C.char
	if C.viva_system_play_sound(C.uint8_t(sound), &nativeError) != 0 {
		return systemError("play system sound", nativeError)
	}
	return nil
}

// ShowFatalError shows a modal alert. It needs the main thread and does nothing
// on any other.
func ShowFatalError(title, message string) {
	nativeTitle, err := systemCString("alert title", title)
	if err != nil {
		return
	}
	defer C.free(unsafe.Pointer(nativeTitle))
	nativeMessage, err := systemCString("alert message", message)
	if err != nil {
		return
	}
	defer C.free(unsafe.Pointer(nativeMessage))
	C.viva_system_show_fatal_error(nativeTitle, nativeMessage)
}

func SendNotification(title, body string) error {
	nativeTitle, err := systemCString("notification title", title)
	if err != nil {
		return err
	}
	defer C.free(unsafe.Pointer(nativeTitle))
	nativeBody, err := systemCString("notification body", body)
	if err != nil {
		return err
	}
	defer C.free(unsafe.Pointer(nativeBody))
	var nativeError *C.char
	status := C.viva_system_send_notification(nativeTitle, nativeBody, &nativeError)
	if status == -2 {
		if nativeError != nil {
			C.viva_system_free(unsafe.Pointer(nativeError))
		}
		return ErrPermissionDenied
	}
	if status != 0 {
		return systemError("send notification", nativeError)
	}
	return nil
}

func allowedExtensions(filters []FileFilter) ([]string, error) {
	var extensions []string
	seen := make(map[string]struct{}, len(filters))
	for filterIndex, filter := range filters {
		patterns := strings.Split(filter.Pattern, ";")
		for patternIndex, pattern := range patterns {
			pattern = strings.TrimSpace(pattern)
			if pattern == "" {
				return nil, fmt.Errorf("darwin: file filter %d pattern %d is empty", filterIndex, patternIndex)
			}
			if pattern == "*" || pattern == "*.*" {
				return nil, nil
			}
			extension := pattern
			if strings.HasPrefix(extension, "*.") {
				extension = extension[2:]
			} else {
				extension = strings.TrimPrefix(extension, ".")
			}
			extension, err := normalizeExtension(extension)
			if err != nil {
				return nil, fmt.Errorf("darwin: file filter %d pattern %q: %w", filterIndex, pattern, err)
			}
			if extension == "" {
				return nil, fmt.Errorf("darwin: file filter %d pattern %q has no extension", filterIndex, pattern)
			}
			key := strings.ToLower(extension)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			extensions = append(extensions, extension)
		}
	}
	return extensions, nil
}

func normalizeExtension(value string) (string, error) {
	value = strings.TrimSpace(value)
	value = strings.TrimSpace(strings.TrimPrefix(value, "."))
	if value == "" {
		return "", nil
	}
	if strings.ContainsAny(value, "*?[]/\\;\x00") {
		return "", fmt.Errorf("extension %q contains wildcard or path syntax", value)
	}
	return value, nil
}

func systemCString(name, value string) (*C.char, error) {
	if strings.IndexByte(value, 0) >= 0 {
		return nil, fmt.Errorf("darwin: %s contains NUL", name)
	}
	return C.CString(value), nil
}

func systemCStrings(name string, values []string) ([]*C.char, func(), error) {
	result := make([]*C.char, 0, len(values))
	free := func() {
		for _, value := range result {
			C.free(unsafe.Pointer(value))
		}
	}
	for _, value := range values {
		nativeValue, err := systemCString(name, value)
		if err != nil {
			free()
			return nil, func() {}, err
		}
		result = append(result, nativeValue)
	}
	return result, free, nil
}

func nativeStringPointer(values []*C.char) **C.char {
	if len(values) == 0 {
		return nil
	}
	return (**C.char)(unsafe.Pointer(&values[0]))
}

func systemError(operation string, nativeError *C.char) error {
	if nativeError == nil {
		return fmt.Errorf("darwin: %s failed", operation)
	}
	message := C.GoString(nativeError)
	C.viva_system_free(unsafe.Pointer(nativeError))
	return fmt.Errorf("darwin: %s: %s", operation, message)
}

func systemBoolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func systemStringArrayABI() (size, countOffset uintptr) {
	var value C.viva_system_string_array
	return unsafe.Sizeof(value), unsafe.Offsetof(value.count)
}
