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
	// scale is the window's device pixels per logical pixel; X reports device
	// pixels and the application works in logical ones. Nil means 1.
	scale func() float32
	// settings is the desktop's XSETTINGS as they stand, for the double-click
	// timing. Nil keeps the defaults.
	settings func() xsettings
	// modifierKeys are the keys that are down, so a modifier release knows
	// whether the other side of a pair still holds it.
	modifierKeys *modifierTracker
}

// modifierTracker follows the keys that are down by scancode.
type modifierTracker struct {
	down map[uint32]input.Modifiers
}

func newModifierTracker() *modifierTracker {
	return &modifierTracker{down: make(map[uint32]input.Modifiers)}
}

// modifierBit is the modifier a key sets while held. AltGr and Mode_switch are
// layout levels, not Alt, whatever key position they share.
func modifierBit(keysym uint32, key input.Key) input.Modifiers {
	if keysym == keysymLevel3 || keysym == keysymModeSwitch {
		return 0
	}
	return key.Modifier()
}

// press records a key going down.
func (tracker *modifierTracker) press(scancode uint32, keysym uint32, key input.Key) {
	if tracker == nil {
		return
	}
	tracker.down[scancode] = modifierBit(keysym, key)
}

// release forgets a key coming up and reports the bit stored for it, and
// whether the key was held. Reading the stored bit keeps a release in step
// with its press even where the key's normalized modifier differs (AltGr).
func (tracker *modifierTracker) release(scancode uint32) (input.Modifiers, bool) {
	if tracker == nil {
		return 0, false
	}
	bit, known := tracker.down[scancode]
	delete(tracker.down, scancode)
	return bit, known
}

// holds reports whether another key still sets the same modifier.
func (tracker *modifierTracker) holds(bit input.Modifiers) bool {
	if tracker == nil || bit == 0 {
		return false
	}
	for _, held := range tracker.down {
		if held == bit {
			return true
		}
	}
	return false
}

func (tracker *modifierTracker) reset() {
	if tracker == nil {
		return
	}
	tracker.down = make(map[uint32]input.Modifiers)
}

// seed rebuilds the set from the keys the server reports down: entries it no
// longer has go, and modifier keys already held when the window gained the
// focus are added so their pair's release still finds them.
func (tracker *modifierTracker) seed(keys [32]byte, bit func(scancode uint32) input.Modifiers) {
	if tracker == nil {
		return
	}
	tracker.retain(keys)
	for scancode := uint32(0); scancode < 256; scancode++ {
		if keys[scancode/8]&(1<<(scancode%8)) == 0 {
			continue
		}
		if _, held := tracker.down[scancode]; held {
			continue
		}
		// Only keys that set a modifier matter; a held letter adds nothing.
		if value := bit(scancode); value != 0 {
			tracker.down[scancode] = value
		}
	}
}

// retain drops the keys the server says are no longer down. X reports keys
// pressed before this window had the focus only by their release, so a stale
// entry must go, while a key still held across the focus change must stay.
func (tracker *modifierTracker) retain(keys [32]byte) {
	if tracker == nil {
		return
	}
	for scancode := range tracker.down {
		if scancode >= 256 || keys[scancode/8]&(1<<(scancode%8)) == 0 {
			delete(tracker.down, scancode)
		}
	}
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

func (router eventRouter) dispatchPointer(event platform.PointerEvent) error {
	if router.config.OnPointer != nil {
		frame, err := router.config.OnPointer(event)
		if err := router.presentFrame(frame, err); err != nil {
			return err
		}
	}
	if router.text != nil {
		frame, handled, err := router.text.pointer(event)
		if handled || err != nil {
			return router.presentFrame(frame, err)
		}
	}
	return nil
}

func (router eventRouter) dispatchScroll(event platform.ScrollEvent) error {
	if router.config.OnScroll == nil {
		return nil
	}
	frame, err := router.config.OnScroll(event)
	return router.presentFrame(frame, err)
}

// waylandKeyPress applies a Wayland key press in the order key() uses for X11:
// a modifier the focused field did not ask to see stops here, a shortcut is
// offered before the input method, and a key no input method took passes
// through the dead keys and Compose before it is delivered.
func (router eventRouter) waylandKeyPress(keyEvent platform.KeyEvent, keysym, keycode uint32) error {
	if handled, err := router.modifierPress(keyEvent, keysym, keycode); handled {
		return err
	}
	return router.keyOrder(keyEvent, keysym, func() bool {
		return router.ime != nil && router.ime.processKeyDirect(keysym, keycode, ibusModifierMask(keyEvent.Modifiers), keyEvent)
	})
}

// modifierPress handles a modifier press the same way on both backends: a
// terminal that asked for releases receives it with its bit set; anything else
// drops it before the input method or the application sees it.
func (router eventRouter) modifierPress(keyEvent platform.KeyEvent, keysym, scancode uint32) (bool, error) {
	router.modifierKeys.press(scancode, keysym, keyEvent.Key)
	if !keyEvent.Key.IsModifier() {
		return false, nil
	}
	if !router.wantsKeyReleases() {
		return true, nil
	}
	// AltGr and Mode_switch are not Alt: delivering them with Alt set would
	// leave the release unbalanced, since the modifier state never has it.
	if bit := modifierBit(keysym, keyEvent.Key); bit != 0 {
		keyEvent.Modifiers |= bit
	}
	return true, router.deliverKey(keyEvent)
}

// releaseModifiers reports the modifiers after a key came up. The compositor's
// modifier event may not have arrived yet, so drop the released bit here unless
// the other side of a pair still holds it. keysym names the key the press was
// tracked under; for a press this window never saw it keeps AltGr from becoming
// Alt.
func (router eventRouter) releaseModifiers(modifiers input.Modifiers, key input.Key, keysym uint32, scancode uint32) input.Modifiers {
	bit, known := router.modifierKeys.release(scancode)
	if !known {
		// A key whose press this window never saw still reports its class.
		bit = modifierBit(keysym, key)
	}
	if bit == 0 || router.modifierKeys.holds(bit) {
		return modifiers
	}
	return modifiers &^ bit
}

// waylandKeyRelease tells the input method a key came up and, for a terminal
// that asked for releases, delivers the release. A release never offers a
// shortcut. modifiers is the state before the key came up: as on X11, the
// engine sees the raw state, so a lone Shift release still carries Shift and
// engines can tell it from Shift+other.
func (router eventRouter) waylandKeyRelease(keyEvent platform.KeyEvent, keysym, keycode uint32, modifiers input.Modifiers) error {
	// As on X11, the input method hears every release, including a modifier
	// whose press the router stopped short of; engines use a lone Shift release
	// to switch input mode.
	if router.ime != nil {
		router.ime.releaseDirect(keysym, keycode, ibusModifierMask(modifiers))
	}
	return router.deliverRelease(keyEvent)
}

// deliverRelease delivers a key coming up to a terminal that asked for
// releases, without telling the input method: a release synthesized when focus
// is lost stands for a key the engine never saw go down.
func (router eventRouter) deliverRelease(keyEvent platform.KeyEvent) error {
	if !router.wantsKeyReleases() || router.config.OnKey == nil {
		return nil
	}
	// The event already carries the modifiers after the key came up; a modifier
	// whose other side is still held stays set.
	keyEvent.Release = true
	keyEvent.Repeat = false
	keyEvent.Text = ""
	keyEvent.Typed = ""
	return router.deliverReleaseToApplication(keyEvent)
}

func (router eventRouter) key(keymap *xcbKeymap, event *xcbInputEvent, repeat bool) error {
	key := keymap.key(event.Detail)
	modifiers := xcbModifiers(event.State)
	wanted := router.wantsKeyReleases()
	texts := keyText{text: keymap.text(event.Detail, event.State)}
	if wanted {
		texts.base = keymap.shortcutText(event.Detail, 0)
	} else if modifiers&(input.ModifierControl|input.ModifierMeta) != 0 {
		// A chord is the same on every layout: Ctrl+C is the key that types c.
		texts.shortcut = keymap.shortcutText(event.Detail, event.State)
	}
	keyEvent := keyEventFor(key, modifiers, uint32(event.Detail), texts, wanted)
	keyEvent.Repeat = repeat
	keysym := keymap.keysym(event.Detail, event.State)
	if handled, err := router.modifierPress(keyEvent, keysym, uint32(event.Detail)); handled {
		return err
	}
	// An input method gets the key after the application's shortcuts, and answers
	// later; a key it does not take comes back through deliverKey, in the order
	// it was typed.
	return router.keyOrder(keyEvent, keysym, func() bool {
		return router.ime != nil && router.ime.processKey(keysym, event, keyEvent)
	})
}

// keyText is what a backend's keymap says about one press.
type keyText struct {
	// text is the character the key types with the modifiers in effect.
	text string
	// shortcut is the chord's character under the modifiers in effect.
	shortcut string
	// base is the chord's character without modifiers, for a raw terminal key.
	base string
}

// keyEventFor builds a press the same way on every backend: a raw terminal key
// carries its code, the base chord character in Text and what it types in Typed;
// any other press carries the shortcut or typed character in Text.
func keyEventFor(key input.Key, modifiers input.Modifiers, code uint32, texts keyText, wanted bool) platform.KeyEvent {
	event := platform.KeyEvent{Key: key, Modifiers: modifiers}
	switch {
	case wanted:
		event.Code = code
		event.Text = texts.base
		// A Control, Alt or Meta chord types no character of its own.
		if modifiers&(input.ModifierControl|input.ModifierAlt|input.ModifierMeta) == 0 {
			event.Typed = texts.text
		}
	case modifiers&(input.ModifierControl|input.ModifierMeta) != 0 && texts.shortcut != "":
		event.Text = texts.shortcut
	default:
		event.Text = texts.text
	}
	return event
}

// keyOrder offers a key press through the order both backends share: a shortcut
// first, unless the focused field owns the chord, then the input method, then
// the dead keys and Compose.
func (router eventRouter) keyOrder(keyEvent platform.KeyEvent, keysym uint32, imeTakes func() bool) error {
	if router.config.OnShortcut != nil && platform.ShortcutCandidate(keyEvent) && !router.text.ownsChord(keyEvent) {
		frame, handled, err := router.config.OnShortcut(keyEvent)
		if err := router.presentFrame(frame, err); err != nil {
			return err
		}
		if handled {
			return nil
		}
	}
	if imeTakes != nil && imeTakes() {
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
	key := keymap.key(event.Detail)
	// The symbol at the state without the AltGr level: for the AltGr key itself
	// that is ISO_Level3_Shift, which is not Alt.
	keysym := keymap.keysym(event.Detail, event.State&^keymap.level3Mask())
	modifiers := router.releaseModifiers(xcbModifiers(event.State), key, keysym, uint32(event.Detail))
	if !router.wantsKeyReleases() || router.config.OnKey == nil {
		return nil
	}
	return router.deliverReleaseToApplication(platform.KeyEvent{Key: key, Modifiers: modifiers, Release: true, Code: uint32(event.Detail)})
}

// deliverKey hands a key to the focused text field, then to the application.
func (router eventRouter) deliverKey(keyEvent platform.KeyEvent) error {
	if router.text != nil && !(keyEvent.Code != 0 && router.wantsKeyReleases()) {
		frame, handled, err := router.text.key(keyEvent)
		if handled || err != nil {
			return router.presentFrame(frame, err)
		}
	}
	return router.deliverToApplication(keyEvent)
}

// deliverToApplication hands an event to OnKey, dropping a key the application
// has no use for.
func (router eventRouter) deliverToApplication(keyEvent platform.KeyEvent) error {
	if router.config.OnKey == nil || keyEvent.Key == input.KeyUnknown && keyEvent.Text == "" {
		return nil
	}
	frame, err := router.config.OnKey(keyEvent)
	return router.presentFrame(frame, err)
}

// deliverReleaseToApplication hands a key release to OnKey. A release names its
// key by code, not by text, so a code keeps it.
func (router eventRouter) deliverReleaseToApplication(keyEvent platform.KeyEvent) error {
	if router.config.OnKey == nil {
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
	if router.present != nil {
		return router.present(frame)
	}
	return nil
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
