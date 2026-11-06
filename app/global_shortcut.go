package app

import (
	"fmt"
	"github.com/vivaui/viva/ui"
	"strconv"
	"strings"
)

// GlobalShortcut is an optional system-wide key binding. It is independent of
// Config.OnShortcut, which handles keys delivered to a focused window.
type GlobalShortcut struct {
	Modifiers ui.Modifiers
	Key       string
}

func normalizeGlobalShortcut(s GlobalShortcut) (GlobalShortcut, error) {
	s.Key = strings.ToUpper(strings.TrimSpace(s.Key))
	if s.Modifiers == 0 || s.Modifiers & ^(ui.ModifierControl|ui.ModifierAlt|ui.ModifierShift|ui.ModifierMeta) != 0 {
		return s, fmt.Errorf("app: global shortcut requires valid modifiers")
	}
	if len(s.Key) == 1 && (s.Key[0] >= 'A' && s.Key[0] <= 'Z' || s.Key[0] >= '0' && s.Key[0] <= '9') {
		return s, nil
	}
	if strings.HasPrefix(s.Key, "F") {
		if n, e := strconv.Atoi(s.Key[1:]); e == nil && n >= 1 && n <= 24 {
			return s, nil
		}
	}
	switch s.Key {
	case "SPACE", "ENTER", "ESCAPE", "TAB":
		return s, nil
	}
	return s, fmt.Errorf("app: unsupported global shortcut key %q", s.Key)
}

// RegisterGlobalShortcut registers a binding until the returned stop is called.
// Stop is idempotent. The callback may run off the window thread: use a window's
// thread-safe Show/Hide or an Invalidator to update UI state. No registration,
// worker or native resource exists until this function is used. Driver-backed
// runs keep bindings private and expose Driver.PressGlobalShortcut for tests.
// The Windows backend supports system registration. Other native backends
// currently return ErrUnsupported; normal focused-window shortcuts still work.
func RegisterGlobalShortcut(s GlobalShortcut, pressed func()) (stop func(), err error) {
	s, err = normalizeGlobalShortcut(s)
	if err != nil {
		return nil, err
	}
	if pressed == nil {
		return nil, fmt.Errorf("app: global shortcut callback is nil")
	}
	if stop, err, handled := driverRegisterGlobalShortcut(s, pressed); handled {
		return stop, err
	}
	return nativeRegisterGlobalShortcut(s, pressed)
}
