package linux

import (
	"fmt"
	"time"
	"unsafe"

	"github.com/vivaui/viva/internal/input"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/ui"
)

const xcbWheelStep = 48

type eventRouter struct {
	config  platform.Config
	present func(platform.Frame) error
	text    *linuxTextInput
	// clicks numbers consecutive presses; X reports none. Nil counts every
	// press as 1.
	clicks *input.ClickCounter
	// ime, when there is an input method daemon to talk to, sees the keys of a
	// focused text field before the field does.
	ime *linuxIME
	// pressed is which keycodes are down, so that a key an input method gave
	// back after its release is not left pressed.
	pressed *[256]bool
	// scale is the window's device pixels per logical pixel; X reports device
	// pixels and the application works in logical ones. Nil means 1.
	scale func() float32
	// settings is the desktop's XSETTINGS as they stand, for the double-click
	// timing. Nil keeps the defaults.
	settings func() xsettings
}

// displayScale is the window's current scale.
func (router eventRouter) displayScale() float32 {
	if router.scale == nil {
		return 1
	}
	return router.scale()
}

// wantsKeyReleases reports whether the focused terminal asked for presses and
// releases of every key (platform.TextInput.KeyReleases).
func (router eventRouter) wantsKeyReleases() bool {
	return router.text != nil && router.text.enabled && router.text.input.Terminal && router.text.input.KeyReleases
}

func (router eventRouter) pointer(kind platform.PointerEventKind, event *xcbInputEvent) error {
	button := xcbPointerButton(event.Detail)
	scale := router.displayScale()
	count := 0
	if router.clicks != nil && (kind == platform.PointerDown || kind == platform.PointerUp) {
		if kind == platform.PointerDown {
			router.applyClickTiming()
			router.clicks.Press(time.Duration(event.Time)*time.Millisecond, uint8(button), float32(event.EventX)/scale, float32(event.EventY)/scale)
		}
		count = router.clicks.Current()
	} else if kind == platform.PointerDown || kind == platform.PointerUp {
		count = 1
	}
	pointerEvent := platform.PointerEvent{
		Kind:       kind,
		Button:     button,
		Modifiers:  xcbModifiers(event.State),
		X:          float32(event.EventX) / scale,
		Y:          float32(event.EventY) / scale,
		ScreenX:    float32(event.RootX) / scale,
		ScreenY:    float32(event.RootY) / scale,
		ClickCount: count,
	}
	if router.config.OnPointer != nil {
		frame, err := router.config.OnPointer(pointerEvent)
		if err := router.presentFrame(frame, err); err != nil {
			return err
		}
	}
	if router.text != nil {
		frame, handled, err := router.text.pointer(pointerEvent)
		if handled || err != nil {
			return router.presentFrame(frame, err)
		}
	}
	return nil
}

func (router eventRouter) pointerExit() error {
	if router.config.OnPointer == nil {
		return nil
	}
	frame, err := router.config.OnPointer(platform.PointerEvent{Kind: platform.PointerExit})
	return router.presentFrame(frame, err)
}

func (router eventRouter) key(keymap *xcbKeymap, event *xcbInputEvent, repeat bool) error {
	key := keymap.key(event.Detail)
	modifiers := xcbModifiers(event.State)
	keyEvent := platform.KeyEvent{Key: key, Modifiers: modifiers, Repeat: repeat}
	if modifiers&(input.ModifierControl|input.ModifierMeta) != 0 {
		// A chord is the same on every layout: Ctrl+C is the key that types c.
		keyEvent.Text = keymap.shortcutText(event.Detail, event.State)
	} else {
		keyEvent.Text = keymap.text(event.Detail, event.State)
	}
	keysym := keymap.keysym(event.Detail, event.State)
	if key.IsModifier() && router.ime != nil {
		// An input method reads a lone modifier by its press and its release (a bare
		// Shift switches between Chinese and English), so it gets both.
		router.ime.pressModifier(keysym, event)
	}
	wanted := router.wantsKeyReleases()
	if wanted {
		keyEvent.Code = uint32(event.Detail)
		keyEvent.Text = keymap.shortcutText(event.Detail, 0)
		keyEvent.Typed = keymap.text(event.Detail, event.State)
		if router.pressed != nil {
			router.pressed[event.Detail] = true
		}
		if key.IsModifier() {
			keyEvent.Modifiers |= key.Modifier()
			return router.deliverKey(keyEvent)
		}
	} else if key.IsModifier() {
		return nil
	}
	if router.config.OnShortcut != nil && platform.ShortcutCandidate(keyEvent) && !router.text.ownsChord(keyEvent) {
		frame, handled, err := router.config.OnShortcut(keyEvent)
		if err := router.presentFrame(frame, err); err != nil {
			return err
		}
		if handled {
			return nil
		}
	}
	// An input method gets the key first, and answers later; a key it does not
	// take comes back through deliverKey, in the order it was typed.
	if router.ime != nil && router.ime.processKey(keysym, event, keyEvent) {
		return nil
	}
	return router.deliverTyped(keyEvent, keysym)
}

// deliverTyped passes a key that no input method took through the dead keys and
// the Compose key, which turn a sequence of keys into one character, and then
// delivers it. A key that is part of a sequence delivers nothing itself.
func (router eventRouter) deliverTyped(keyEvent platform.KeyEvent, keysym uint32) error {
	if router.text != nil {
		compose := &router.text.compose
		if keyEvent.Modifiers&(input.ModifierControl|input.ModifierAlt|input.ModifierMeta) != 0 {
			compose.reset()
		} else if composed, handled := compose.feed(keysym); handled {
			if composed == "" {
				return nil
			}
			keyEvent.Text = composed
			if keyEvent.Typed != "" || keyEvent.Code != 0 {
				keyEvent.Typed = composed
			}
		}
	}
	return router.deliverKey(keyEvent)
}

// release delivers a key coming up to a terminal that asked for releases. The
// runtime drops a release whose press it never saw, which is what becomes of
// the key an input method consumed.
func (router eventRouter) release(keymap *xcbKeymap, event *xcbInputEvent) error {
	if router.pressed != nil {
		router.pressed[event.Detail] = false
	}
	if !router.wantsKeyReleases() || router.config.OnKey == nil {
		return nil
	}
	key := keymap.key(event.Detail)
	modifiers := xcbModifiers(event.State)
	if key.IsModifier() {
		modifiers &^= key.Modifier()
	}
	frame, err := router.config.OnKey(platform.KeyEvent{Key: key, Modifiers: modifiers, Release: true, Code: uint32(event.Detail)})
	return router.presentFrame(frame, err)
}

// deliverKey hands a key to the focused text field, then to the application.
func (router eventRouter) deliverKey(keyEvent platform.KeyEvent) error {
	if router.text != nil && !(keyEvent.Code != 0 && router.wantsKeyReleases()) {
		frame, handled, err := router.text.key(keyEvent)
		if handled || err != nil {
			return router.presentFrame(frame, err)
		}
	}
	if router.config.OnKey == nil || keyEvent.Key == input.KeyUnknown && keyEvent.Text == "" {
		return nil
	}
	frame, err := router.config.OnKey(keyEvent)
	return router.presentFrame(frame, err)
}

func xcbPointerButton(button uint8) ui.PointerButton {
	switch button {
	case xcbButtonLeft:
		return ui.PointerButtonPrimary
	case xcbButtonRight:
		return ui.PointerButtonSecondary
	case xcbButtonMiddle:
		return ui.PointerButtonMiddle
	case xcbButtonBack:
		return ui.PointerButtonBack
	case xcbButtonForward:
		return ui.PointerButtonForward
	default:
		return ui.PointerButtonNone
	}
}

func (router eventRouter) scroll(event *xcbInputEvent) (bool, error) {
	if router.config.OnScroll == nil {
		return false, nil
	}
	scroll, ok := xcbScrollEvent(event.Detail)
	if !ok {
		return false, nil
	}
	scroll.Modifiers = xcbModifiers(event.State)
	scale := router.displayScale()
	scroll.X = float32(event.EventX) / scale
	scroll.Y = float32(event.EventY) / scale
	frame, err := router.config.OnScroll(scroll)
	return true, router.presentFrame(frame, err)
}

func (router eventRouter) presentFrame(frame platform.Frame, err error) error {
	if err != nil {
		return err
	}
	return router.present(frame)
}

func (win *window) reloadKeymap() error {
	setup := win.api.getSetup(win.connection)
	if setup == nil {
		return fmt.Errorf("linux: XCB returned no connection setup")
	}
	if setup.MaximumKeycode < setup.MinimumKeycode {
		return fmt.Errorf("linux: invalid X keyboard keycode range %d..%d", setup.MinimumKeycode, setup.MaximumKeycode)
	}
	count := uint16(setup.MaximumKeycode) - uint16(setup.MinimumKeycode) + 1
	if count == 0 || count > 255 {
		return fmt.Errorf("linux: unsupported X keyboard keycode count %d", count)
	}
	cookie := win.api.getKeyboardMapping(win.connection, setup.MinimumKeycode, uint8(count))
	var nativeError unsafe.Pointer
	reply := win.api.keyboardMappingReply(win.connection, cookie, &nativeError)
	if nativeError != nil {
		win.api.free(nativeError)
		if reply != nil {
			win.api.free(unsafe.Pointer(reply))
		}
		return fmt.Errorf("linux: query X keyboard mapping")
	}
	if reply == nil || reply.KeysymsPerKeycode == 0 {
		if reply != nil {
			win.api.free(unsafe.Pointer(reply))
		}
		return fmt.Errorf("linux: X server returned an empty keyboard mapping")
	}
	defer win.api.free(unsafe.Pointer(reply))
	total := int(count) * int(reply.KeysymsPerKeycode)
	keysyms := win.api.keyboardMappingKeysyms(reply)
	if keysyms == nil || total > int(reply.Length) {
		return fmt.Errorf("linux: X server returned an invalid keyboard mapping")
	}
	win.keymap.first = setup.MinimumKeycode
	win.keymap.last = setup.MaximumKeycode
	win.keymap.perKeycode = reply.KeysymsPerKeycode
	win.keymap.keysyms = append(win.keymap.keysyms[:0], unsafe.Slice(keysyms, total)...)
	win.readModifierMap()
	return nil
}

// readModifierMap finds which modifier bits AltGr, Mode_switch and Num Lock are
// on. A server that does not answer leaves the conventional bits in force.
func (win *window) readModifierMap() {
	cookie := win.api.getModifierMapping(win.connection)
	var nativeError unsafe.Pointer
	reply := win.api.modifierMappingReply(win.connection, cookie, &nativeError)
	if nativeError != nil {
		win.api.free(nativeError)
	}
	if reply == nil {
		return
	}
	defer win.api.free(unsafe.Pointer(reply))
	perModifier := int(reply.KeycodesPerModifier)
	keycodes := win.api.modifierMappingKeycodes(reply)
	if keycodes == nil || perModifier == 0 {
		return
	}
	win.keymap.setModifierMap(unsafe.Slice(keycodes, perModifier*8), perModifier)
}

func xcbModifiers(state uint16) input.Modifiers {
	var modifiers input.Modifiers
	if state&xcbModShift != 0 {
		modifiers |= input.ModifierShift
	}
	if state&xcbModControl != 0 {
		modifiers |= input.ModifierControl
	}
	// Core X11 does not name Alt or Meta. The conventional desktop mapping is
	// Mod1=Alt and Mod4=Meta; layouts requiring XKB remapping remain unsupported.
	if state&xcbMod1 != 0 {
		modifiers |= input.ModifierAlt
	}
	if state&xcbMod4 != 0 {
		modifiers |= input.ModifierMeta
	}
	return modifiers
}

func xcbScrollEvent(button uint8) (platform.ScrollEvent, bool) {
	switch button {
	case xcbButtonScrollUp:
		return platform.ScrollEvent{DeltaY: -xcbWheelStep}, true
	case xcbButtonScrollDown:
		return platform.ScrollEvent{DeltaY: xcbWheelStep}, true
	case xcbButtonScrollLeft:
		return platform.ScrollEvent{DeltaX: -xcbWheelStep}, true
	case xcbButtonScrollRight:
		return platform.ScrollEvent{DeltaX: xcbWheelStep}, true
	default:
		return platform.ScrollEvent{}, false
	}
}

// applyClickTiming brings the click counter in line with the settings before a press
// is counted, so a change the user makes applies to the next click.
func (router eventRouter) applyClickTiming() {
	if router.settings == nil || router.clicks == nil {
		return
	}
	router.clicks.Interval, router.clicks.Distance = clickTiming(router.settings(), router.displayScale())
}

// inputSettings is the window's current XSETTINGS.
func (win *window) inputSettings() xsettings {
	if win.tracker == nil {
		return xsettings{}
	}
	return win.tracker.settings
}
