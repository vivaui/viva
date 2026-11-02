package app

import (
	"errors"
	"fmt"
	"strings"

	"github.com/vivaui/viva/ui"
)

var (
	// ErrUnsupported reports that the target cannot provide the requested integration.
	ErrUnsupported = errors.New("app: system integration is unsupported on this platform")
	// ErrCanceled reports that the user dismissed a native picker or dialog.
	ErrCanceled = errors.New("app: operation canceled")
	// ErrAsyncRequired reports that a synchronous API cannot represent the target operation.
	ErrAsyncRequired = errors.New("app: operation requires an asynchronous API")
	// ErrUserGestureRequired reports that the target rejected an operation outside direct input handling.
	ErrUserGestureRequired = errors.New("app: operation requires a user gesture")
	// ErrPermissionDenied reports that the user or system denied the requested capability.
	ErrPermissionDenied = errors.New("app: permission denied")
	// ErrNoDriverResponse reports that a Driver-owned system integration was
	// invoked without a queued response. Native application runs never return it.
	ErrNoDriverResponse = errors.New("app: no driver response queued")
)

type Cursor uint8

const (
	CursorArrow Cursor = iota
	CursorText
	CursorHand
	CursorCrosshair
	CursorWait
	CursorResizeHorizontal
	CursorResizeVertical
	CursorResizeNWSE
	CursorResizeNESW
	CursorNotAllowed
	// CursorHidden removes the pointer: a surface that draws its own (a
	// remote desktop picture) uses it to keep the system pointer away.
	CursorHidden
	// CursorImage is a picture the application supplies; see SetCursorImage. SetCursor
	// refuses it, since a shape alone does not say which picture.
	CursorImage
)

type SystemSound uint8

const (
	SoundDefault SystemSound = iota
	SoundInformation
	SoundWarning
	SoundError
	SoundQuestion
)

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

type Notification struct {
	Title string
	Body  string
}

// ReduceMotion reports the system accessibility preference for reduced
// motion. Windows, macOS and Linux are queried (on Linux the settings portal, the
// XSETTINGS manager or KDE's animation speed); other targets report false.
func ReduceMotion() bool { return nativeReduceMotion() }

// PlatformTextEngine is the text engine RunCanvas installs on a canvas: the
// platform's own shaped lines, so widths, carets and hits agree with the glyphs it
// draws. It is nil on a target that has none, whose canvases measure with the
// reference metrics.
func PlatformTextEngine() ui.TextEngine { return nativeTextEngine() }

func ClipboardText() (string, error) {
	if value, err, handled := driverClipboardText(); handled {
		return value, err
	}
	return nativeClipboardText()
}

// OnClipboardChange asks for a call on notify whenever the system clipboard's
// content changes, from any application including this one. The callback runs
// on an unspecified thread and must only hand the news on; reading the
// clipboard belongs in the work it schedules.
//
// Nothing runs until the first call: no timer, no thread. It may be made before
// the first window opens, and watching then starts with the window. One
// registration serves the process and the last wins; OnClipboardChange(nil)
// stops watching. An error wrapping ErrUnsupported means this target cannot
// watch the clipboard (the Linux and web targets today): the clipboard keeps
// working, only the news is missing, and a caller may fall back to reading it
// when the user acts. macOS has no change notification, so it polls the
// pasteboard's change count on a 250 ms timer for as long as a registration is
// held.
func OnClipboardChange(notify func()) error { return nativeOnClipboardChange(notify) }

func SetClipboardText(value string) error {
	if strings.IndexByte(value, 0) >= 0 {
		return fmt.Errorf("app: clipboard text contains NUL")
	}
	if err, handled := driverSetClipboardText(value); handled {
		return err
	}
	return nativeSetClipboardText(value)
}

func SetCursor(cursor Cursor) error {
	if cursor > CursorHidden {
		return fmt.Errorf("app: invalid cursor %d", cursor)
	}
	return nativeSetCursor(cursor)
}

// SetCursorImage makes image the pointer over the window, drawn by the system as any pointer
// is; the same call with a nil image is refused. Hits show a picture without this call
// (ui.HitOptions.CursorImage), which is how a view normally chooses it. An error wrapping
// ErrUnsupported means this target cannot show application pictures as its pointer.
func SetCursorImage(image *ui.CursorImage) error {
	if image == nil {
		return fmt.Errorf("app: cursor image is nil")
	}
	return nativeSetCursorImage(image)
}

func OpenFiles(options OpenFileOptions) ([]string, error) {
	if err := validateFilters(options.Filters); err != nil {
		return nil, err
	}
	if paths, err, handled := driverOpenFiles(options); handled {
		return paths, err
	}
	return nativeOpenFiles(options)
}

// OpenDirectory asks the user to choose one existing local directory.
func OpenDirectory(options OpenDirectoryOptions) (string, error) {
	if path, err, handled := driverOpenDirectory(options); handled {
		return path, err
	}
	return nativeOpenDirectory(options)
}

func SaveFile(options SaveFileOptions) (string, error) {
	if err := validateFilters(options.Filters); err != nil {
		return "", err
	}
	if path, err, handled := driverSaveFile(options); handled {
		return path, err
	}
	return nativeSaveFile(options)
}

func OpenURL(target string) error {
	lower := strings.ToLower(target)
	if !strings.HasPrefix(lower, "https://") && !strings.HasPrefix(lower, "http://") && !strings.HasPrefix(lower, "mailto:") {
		return fmt.Errorf("app: external URL must use http, https, or mailto")
	}
	if strings.IndexByte(target, 0) >= 0 {
		return fmt.Errorf("app: external URL contains NUL")
	}
	if err, handled := driverOpenURL(target); handled {
		return err
	}
	return nativeOpenURL(target)
}

func PlaySystemSound(sound SystemSound) error {
	if sound > SoundQuestion {
		return fmt.Errorf("app: invalid system sound %d", sound)
	}
	return nativePlaySystemSound(sound)
}

func SendNotification(notification Notification) error {
	if notification.Title == "" && notification.Body == "" {
		return fmt.Errorf("app: notification is empty")
	}
	return nativeSendNotification(notification)
}

func validateFilters(filters []FileFilter) error {
	for index, filter := range filters {
		if filter.Name == "" || filter.Pattern == "" {
			return fmt.Errorf("app: file filter %d requires name and pattern", index)
		}
		if strings.IndexByte(filter.Name, 0) >= 0 || strings.IndexByte(filter.Pattern, 0) >= 0 {
			return fmt.Errorf("app: file filter %d contains NUL", index)
		}
	}
	return nil
}
