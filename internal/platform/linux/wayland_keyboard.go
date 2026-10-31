//go:build linux

package linux

import (
	"fmt"
	"runtime"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/vivaui/viva/internal/input"
	"github.com/vivaui/viva/internal/platform"
)

const (
	xkbContextNoFlags       uint32 = 0
	xkbKeymapFormatTextV1   uint32 = 1
	xkbKeymapCompileNoFlags uint32 = 0
	xkbStateModsEffective   uint32 = 8
)

var (
	strModShift   = []byte("Shift\x00")
	strModLock    = []byte("Lock\x00")
	strModControl = []byte("Control\x00")
	strModAlt     = []byte("Mod1\x00")
	strModSuper   = []byte("Mod4\x00")
	strModNum     = []byte("Mod2\x00")
	strMod5       = []byte("Mod5\x00")
)

type xkbAPI struct {
	library uintptr

	contextNew           func(flags uint32) uintptr
	contextUnref         func(context uintptr)
	keymapNewFromString  func(context uintptr, str *byte, format uint32, flags uint32) uintptr
	keymapUnref          func(keymap uintptr)
	keymapModGetIndex    func(keymap uintptr, name *byte) uint32
	keymapKeyGetSyms     func(keymap uintptr, key, layout, level uint32, symsOut **uint32) int32
	keymapKeyRepeats     func(keymap uintptr, key uint32) int32
	keymapLayoutsForKey  func(keymap uintptr, key uint32) uint32
	stateNew             func(keymap uintptr) uintptr
	stateUnref           func(state uintptr)
	stateKeyGetOneSym    func(state uintptr, key uint32) uint32
	stateUpdateMask      func(state uintptr, depressed, latched, locked, depLayout, latchLayout, lockLayout uint32) uint32
	stateModNameIsActive func(state uintptr, name *byte, typ uint32) int32
}

func loadXKB() (*xkbAPI, error) {
	lib, err := purego.Dlopen("libxkbcommon.so.0", purego.RTLD_NOW|purego.RTLD_LOCAL)
	if err != nil {
		return nil, fmt.Errorf("linux: failed to load libxkbcommon.so.0: %w", err)
	}
	api := &xkbAPI{library: lib}
	purego.RegisterLibFunc(&api.contextNew, lib, "xkb_context_new")
	purego.RegisterLibFunc(&api.contextUnref, lib, "xkb_context_unref")
	purego.RegisterLibFunc(&api.keymapNewFromString, lib, "xkb_keymap_new_from_string")
	purego.RegisterLibFunc(&api.keymapUnref, lib, "xkb_keymap_unref")
	// These are optional: a libxkbcommon that lacks one still leaves the
	// backend usable through its fallbacks, where RegisterLibFunc would panic.
	registerOptionalLibFunc(&api.keymapModGetIndex, lib, "xkb_keymap_mod_get_index")
	registerOptionalLibFunc(&api.keymapKeyGetSyms, lib, "xkb_keymap_key_get_syms_by_level")
	registerOptionalLibFunc(&api.keymapKeyRepeats, lib, "xkb_keymap_key_repeats")
	registerOptionalLibFunc(&api.keymapLayoutsForKey, lib, "xkb_keymap_num_layouts_for_key")
	purego.RegisterLibFunc(&api.stateNew, lib, "xkb_state_new")
	purego.RegisterLibFunc(&api.stateUnref, lib, "xkb_state_unref")
	purego.RegisterLibFunc(&api.stateKeyGetOneSym, lib, "xkb_state_key_get_one_sym")
	purego.RegisterLibFunc(&api.stateUpdateMask, lib, "xkb_state_update_mask")
	purego.RegisterLibFunc(&api.stateModNameIsActive, lib, "xkb_state_mod_name_is_active")
	return api, nil
}

// registerOptionalLibFunc binds a symbol only when the loaded library has it, so
// an old libxkbcommon degrades to the evdev fallback instead of panicking.
func registerOptionalLibFunc(fptr any, library uintptr, name string) {
	symbol, err := purego.Dlsym(library, name)
	if err != nil {
		return
	}
	purego.RegisterFunc(fptr, symbol)
}

func (api *xkbAPI) close() {
	if api != nil && api.library != 0 {
		purego.Dlclose(api.library)
		api.library = 0
	}
}

var (
	keyboardListenerOnce sync.Once
	rawKeyboardListener  [6]uintptr
	pinnerKeyboard       runtime.Pinner
)

func initKeyboardListeners() {
	keyboardListenerOnce.Do(func() {
		// 0: keymap(format, fd, size)
		rawKeyboardListener[0] = purego.NewCallback(func(data unsafe.Pointer, keyboard uintptr, format uint32, fd int32, size uint32) {
			if data == nil {
				if fd >= 0 {
					_ = syscall.Close(int(fd))
				}
				return
			}
			app := (*waylandApplication)(data)
			app.handleKeymap(format, fd, size)
		})

		// 1: enter(serial, surface, keys)
		rawKeyboardListener[1] = purego.NewCallback(func(data unsafe.Pointer, keyboard uintptr, serial uint32, surface uintptr, keys unsafe.Pointer) {
			if data == nil {
				return
			}
			app := (*waylandApplication)(data)
			app.lastSerial = serial
			app.keyboardFocus = app.findWindowBySurface(surface)
			if app.keyboardFocus != nil {
				// A new focus span: the next blur is news again.
				app.keyboardFocus.focusLostSent = false
			}
			if app.keyboardFocus != nil && app.keyboardFocus.state != nil {
				app.syncModifierKeys(app.keyboardFocus.state.router, keys)
				app.atspiActive(app.keyboardFocus.state, true)
				if app.keyboardFocus.state.router.ime != nil {
					app.keyboardFocus.state.router.ime.setFocused(true)
				}
			}
		})

		// 2: leave(serial, surface)
		rawKeyboardListener[2] = purego.NewCallback(func(data unsafe.Pointer, keyboard uintptr, serial uint32, surface uintptr) {
			if data == nil {
				return
			}
			app := (*waylandApplication)(data)
			app.handleKeyboardLeave(app.findWindowBySurface(surface))
		})

		// 3: key(serial, time, key, state)
		rawKeyboardListener[3] = purego.NewCallback(func(data unsafe.Pointer, keyboard uintptr, serial uint32, timeMs uint32, scancode uint32, state uint32) {
			if data == nil {
				return
			}
			app := (*waylandApplication)(data)
			app.lastSerial = serial
			app.handleKey(serial, timeMs, scancode, state)
		})

		// 4: modifiers(serial, modsDepressed, modsLatched, modsLocked, group)
		rawKeyboardListener[4] = purego.NewCallback(func(data unsafe.Pointer, keyboard uintptr, serial uint32, depressed, latched, locked, group uint32) {
			if data == nil {
				return
			}
			app := (*waylandApplication)(data)
			app.handleModifiers(depressed, latched, locked, group)
		})

		// 5: repeat_info(rate, delay)
		rawKeyboardListener[5] = purego.NewCallback(func(data unsafe.Pointer, keyboard uintptr, rate int32, delay int32) {
			if data == nil {
				return
			}
			app := (*waylandApplication)(data)
			app.repeatRate = rate
			app.repeatDelay = delay
		})

		pinnerKeyboard.Pin(&rawKeyboardListener)
	})
}

func (app *waylandApplication) setupKeyboard() {
	if app.keyboard != 0 || app.globals.seat == 0 || app.api == nil {
		return
	}
	keyboard := app.api.seatGetKeyboard(app.globals.seat)
	if keyboard == 0 {
		return
	}
	app.keyboard = keyboard

	if app.xkb == nil {
		if xkb, err := loadXKB(); err == nil {
			app.xkb = xkb
			app.xkbContext = xkb.contextNew(xkbContextNoFlags)
		}
	}

	initKeyboardListeners()
	app.api.proxyAddListener(keyboard, uintptr(unsafe.Pointer(&rawKeyboardListener[0])), unsafe.Pointer(app))
}

func (app *waylandApplication) teardownKeyboard() {
	app.stopKeyRepeat()
	focus := app.keyboardFocus
	app.keyboardFocus = nil
	if focus != nil && focus.state != nil {
		app.releaseKeyboardFocus(focus)
		// A failed release tears the window down.
		if focus.state != nil {
			app.atspiActive(focus.state, false)
			// The seat took the keyboard without a leave: the window still
			// needs to hear it lost the focus, unless a deactivation or leave
			// already said so.
			app.fail(notifyFocusLost(focus))
		}
	}
	if app.keyboard != 0 && app.api != nil {
		app.api.keyboardRelease(app.keyboard)
		app.keyboard = 0
	}
	if app.xkbState != 0 && app.xkb != nil {
		app.xkb.stateUnref(app.xkbState)
		app.xkbState = 0
	}
	if app.xkbShortcutState != 0 && app.xkb != nil {
		app.xkb.stateUnref(app.xkbShortcutState)
		app.xkbShortcutState = 0
	}
	app.resetHeldKeyState()
	if app.xkbKeymap != 0 && app.xkb != nil {
		app.xkb.keymapUnref(app.xkbKeymap)
		app.xkbKeymap = 0
	}
	if app.xkbContext != 0 && app.xkb != nil {
		app.xkb.contextUnref(app.xkbContext)
		app.xkbContext = 0
	}
	if app.xkb != nil {
		app.xkb.close()
		app.xkb = nil
	}
}

// resetHeldKeyState forgets everything a keyboard left behind: held modifiers,
// the modifier masks and shortcut levels its keymap defined. A new keyboard
// must not inherit any of it. Any state added alongside these fields belongs
// here too.
func (app *waylandApplication) resetHeldKeyState() {
	app.xkbDepressed = 0
	app.xkbLatched = 0
	app.xkbLocked = 0
	app.xkbGroup = 0
	app.modifiers = 0
	app.xkbShiftMask = 0
	app.xkbLockMask = 0
	app.xkbNumLockMask = 0
	app.xkbLevel3Mask = 0
	app.xkbLevels = nil
}

// handleKeyboardLeave cleans up after the window the leave names. The
// compositor sends the real releases to the new focus surface, so a terminal
// that saw keys go down is told they came up. The leave may arrive after the
// enter that already moved the focus (the protocol orders the two per surface,
// not across surfaces): the leaving window is cleaned either way, but the
// current focus is cleared only when it is the window that left.
func (app *waylandApplication) handleKeyboardLeave(leaving *waylandWindow) {
	if leaving == nil || leaving.state == nil {
		// A surface this client does not own (or no longer does).
		return
	}
	app.releaseKeyboardFocus(leaving)
	// A failed release tears the window down.
	if leaving.state == nil {
		return
	}
	app.atspiActive(leaving.state, false)
	app.fail(notifyFocusLost(leaving))
}

// releaseKeyboardFocus stops the window's repeat, tells a terminal that the
// keys it saw go down have come up, and forgets the held-key set. It leaves
// the notification that the keyboard left to notifyFocusLost, which the three
// blur paths share.
func (app *waylandApplication) releaseKeyboardFocus(leaving *waylandWindow) {
	if leaving == nil || leaving.state == nil {
		return
	}
	if app.keyboardFocus == leaving {
		app.keyboardFocus = nil
	}
	app.mu.Lock()
	repeatsHere := app.repeatWindow == leaving
	app.mu.Unlock()
	if repeatsHere {
		app.stopKeyRepeat()
	}
	router := leaving.state.router
	app.releaseHeldKeys(router)
	router.modifierKeys.reset()
}

// notifyFocusLost tells a window's IME and the application that the keyboard
// left. One blur can arrive as a toplevel deactivation, a wl_keyboard.leave
// and a seat capability removal; the first one tells them, so cleanup in
// OnFocusLost runs once per focus span. The caller reports the error.
func notifyFocusLost(win *waylandWindow) error {
	if win == nil || win.state == nil || win.focusLostSent {
		return nil
	}
	win.focusLostSent = true
	router := win.state.router
	if router.ime != nil {
		router.ime.setFocused(false)
	}
	if win.state.config.OnFocusLost == nil {
		return nil
	}
	frame, err := win.state.config.OnFocusLost()
	return router.presentFrame(frame, err)
}

func (app *waylandApplication) handleKeymap(format uint32, fd int32, size uint32) {
	defer func() {
		if app.libc != nil && app.libc.close != nil && fd >= 0 {
			app.libc.close(fd)
		} else if fd >= 0 {
			_ = syscall.Close(int(fd))
		}
	}()

	if size == 0 || app.xkb == nil || app.xkbContext == 0 || app.libc == nil || app.libc.read == nil {
		return
	}

	_, _ = syscall.Seek(int(fd), 0, 0)
	data := make([]byte, size+1)
	totalRead := 0
	for totalRead < int(size) {
		n := app.libc.read(fd, unsafe.Pointer(&data[totalRead]), uintptr(int(size)-totalRead))
		if n <= 0 {
			break
		}
		totalRead += n
	}
	data[totalRead] = 0 // null terminate

	keymap := app.xkb.keymapNewFromString(app.xkbContext, &data[0], xkbKeymapFormatTextV1, xkbKeymapCompileNoFlags)
	if keymap == 0 {
		return
	}
	state := app.xkb.stateNew(keymap)
	if state == 0 {
		app.xkb.keymapUnref(keymap)
		return
	}
	// A second state reads shortcut characters from the layout's Latin levels
	// without disturbing the layout the person is typing in.
	shortcutState := app.xkb.stateNew(keymap)

	app.xkbShiftMask = modMask(app.xkb, keymap, strModShift)
	app.xkbLockMask = modMask(app.xkb, keymap, strModLock)
	app.xkbNumLockMask, app.xkbLevel3Mask = modifierMasks(app.xkb, keymap, data)
	app.xkbLevels = make(map[uint32][][2]uint32)

	if app.xkbState != 0 {
		app.xkb.stateUnref(app.xkbState)
	}
	if app.xkbShortcutState != 0 {
		app.xkb.stateUnref(app.xkbShortcutState)
	}
	if app.xkbKeymap != 0 {
		app.xkb.keymapUnref(app.xkbKeymap)
	}
	app.xkbKeymap = keymap
	app.xkbState = state
	app.xkbShortcutState = shortcutState
	// A new keymap starts with no modifiers; reapply the ones that were held so
	// typing stays correct until the next wl_keyboard.modifiers event.
	app.xkb.stateUpdateMask(state, app.xkbDepressed, app.xkbLatched, app.xkbLocked, 0, 0, app.xkbGroup)
	if shortcutState != 0 {
		app.xkb.stateUpdateMask(shortcutState, app.xkbDepressed, app.xkbLatched, app.xkbLocked, 0, 0, app.xkbGroup)
	}
	app.updateModifiers()
	// A held key can change class with the layout: AltGr may become Alt.
	app.refreshModifierKeys()
}

// refreshModifierKeys re-reads the bit each held key sets under the current
// keymap.
func (app *waylandApplication) refreshModifierKeys() {
	focus := app.keyboardFocus
	if focus == nil || focus.state == nil || focus.state.router.modifierKeys == nil {
		return
	}
	tracker := focus.state.router.modifierKeys
	for scancode := range tracker.down {
		keysym := app.resolveKeysym(scancode)
		tracker.down[scancode] = modifierBit(keysym, normalizedKeysym(keysym))
	}
}

func (app *waylandApplication) handleModifiers(depressed, latched, locked, group uint32) {
	app.xkbDepressed, app.xkbLatched, app.xkbLocked, app.xkbGroup = depressed, latched, locked, group
	if app.xkb != nil && app.xkbState != 0 {
		app.xkb.stateUpdateMask(app.xkbState, depressed, latched, locked, 0, 0, group)
		app.updateModifiers()
	}
}

// updateModifiers recomputes the named modifiers from the state, so a keymap
// reload that changes modifier mappings is reflected without waiting for the
// next wl_keyboard.modifiers event.
func (app *waylandApplication) updateModifiers() {
	if app.xkb == nil || app.xkbState == 0 || app.xkb.stateModNameIsActive == nil {
		return
	}
	var mods input.Modifiers
	if app.xkb.stateModNameIsActive(app.xkbState, &strModShift[0], xkbStateModsEffective) > 0 {
		mods |= input.ModifierShift
	}
	if app.xkb.stateModNameIsActive(app.xkbState, &strModControl[0], xkbStateModsEffective) > 0 {
		mods |= input.ModifierControl
	}
	if app.xkb.stateModNameIsActive(app.xkbState, &strModAlt[0], xkbStateModsEffective) > 0 {
		mods |= input.ModifierAlt
	}
	if app.xkb.stateModNameIsActive(app.xkbState, &strModSuper[0], xkbStateModsEffective) > 0 {
		mods |= input.ModifierMeta
	}
	app.modifiers = mods
}

// releaseHeldKeys tells a terminal that the keys it saw go down have come up,
// so a keyboard that disappears without a leave leaves no key stuck. The router
// is a copy, so a failed delivery that tears its window down leaves this loop
// with nothing dangling.
func (app *waylandApplication) releaseHeldKeys(router eventRouter) {
	if router.modifierKeys == nil || !router.wantsKeyReleases() {
		return
	}
	// Each synthetic release reports the modifiers left after the ones already
	// released, so a batch never re-introduces a modifier it reported up. The
	// keys go in scancode order so the same transition reports the same events
	// on every run.
	scancodes := make([]uint32, 0, len(router.modifierKeys.down))
	for scancode := range router.modifierKeys.down {
		scancodes = append(scancodes, scancode)
	}
	sort.Slice(scancodes, func(i, j int) bool { return scancodes[i] < scancodes[j] })
	modifiers := app.modifiers
	for _, scancode := range scancodes {
		keyEvent, _ := app.releaseEvent(scancode, router, modifiers)
		modifiers = keyEvent.Modifiers
		// The engine did not see the presses these releases stand for, and the
		// key may still be down; tell the terminal only. Stop on an error,
		// since app.fail tears the windows down.
		if err := router.deliverRelease(keyEvent); err != nil {
			app.fail(err)
			return
		}
	}
}

// releaseEvent builds the event for a key coming up, from the state as it
// stands after the key left.
func (app *waylandApplication) releaseEvent(scancode uint32, router eventRouter, modifiers input.Modifiers) (platform.KeyEvent, uint32) {
	keysym := app.resolveKeysym(scancode)
	keyEvent := platform.KeyEvent{
		Key:       normalizedKeysym(keysym),
		Modifiers: modifiers,
		Release:   true,
	}
	keyEvent.Modifiers = router.releaseModifiers(keyEvent.Modifiers, keyEvent.Key, keysym, scancode)
	if router.wantsKeyReleases() {
		keyEvent.Code = scancode
	}
	return keyEvent, keysym
}

// syncModifierKeys rebuilds the router's held-key set from the keys
// wl_keyboard reports as down when focus enters, so a release can tell whether
// the other side of a pair still holds the modifier.
func (app *waylandApplication) syncModifierKeys(router eventRouter, keys unsafe.Pointer) {
	tracker := router.modifierKeys
	if tracker == nil {
		return
	}
	tracker.reset()
	if keys == nil {
		return
	}
	array := (*wlArray)(keys)
	if array.Data == nil || array.Size == 0 {
		return
	}
	count := array.Size / unsafe.Sizeof(uint32(0))
	for _, scancode := range unsafe.Slice((*uint32)(array.Data), count) {
		keysym := app.resolveKeysym(scancode)
		tracker.press(scancode, keysym, normalizedKeysym(keysym))
	}
}

func ibusModifierMask(modifiers input.Modifiers) uint32 {
	var state uint32
	if modifiers&input.ModifierShift != 0 {
		state |= xcbModShift
	}
	if modifiers&input.ModifierControl != 0 {
		state |= xcbModControl
	}
	if modifiers&input.ModifierAlt != 0 {
		state |= xcbMod1
	}
	if modifiers&input.ModifierMeta != 0 {
		state |= xcbMod4
	}
	return state
}

func (app *waylandApplication) resolveKeysym(scancode uint32) uint32 {
	if app.xkb != nil && app.xkbState != 0 {
		return app.xkb.stateKeyGetOneSym(app.xkbState, scancode+8)
	}
	return fallbackEvdevToKeysym(scancode)
}

// shortcutText is the character a key stands for in a shortcut: the Latin one
// of the first layout that has it, as X11's keymap.shortcutText, so Ctrl+C
// stays C on a Cyrillic layout instead of the character that layout types. A
// key no layout gives an ASCII character stands for what it types.
func (app *waylandApplication) shortcutText(scancode uint32, depressed, latched, locked, layout uint32) string {
	key := scancode + 8
	// Read the levels without Level3/AltGr: Ctrl+AltGr+Q stands for Q, not for
	// the character AltGr types, as with X11's keymap.shortcutText.
	if app.xkb != nil && app.xkbKeymap != 0 && app.xkb.keymapKeyGetSyms != nil {
		// Locked Shift counts as Shift, as the core modifier state does on X11.
		shift := (depressed|latched|locked)&app.xkbShiftMask != 0
		lock := locked&app.xkbLockMask != 0
		numLock := locked&app.xkbNumLockMask != 0
		if character, ok := shortcutFromLevels(app.keyLevels(key), shift, lock, numLock); ok {
			return string(character)
		}
		// A key no level scan gives ASCII text stands for what the passed
		// modifiers type in the requested layout, as X11's shortcutText falls
		// back to keymap.text. The live state describes the layout the person is
		// typing in, not the requested one, so without a shortcut state the
		// honest answer is nothing.
		if app.xkbShortcutState == 0 {
			return ""
		}
		return app.shortcutStateText(key, depressed&^app.xkbLevel3Mask, latched&^app.xkbLevel3Mask, locked, layout)
	}
	if app.xkb != nil && app.xkbShortcutState != 0 {
		return app.shortcutStateText(key, depressed&^app.xkbLevel3Mask, latched&^app.xkbLevel3Mask, locked, layout)
	}
	if character, ok := keysymRune(app.resolveKeysym(scancode)); ok {
		return string(character)
	}
	return ""
}

// shortcutStateText is what one key types in the requested layout with the
// passed modifiers, from the state kept off the typing layout.
func (app *waylandApplication) shortcutStateText(keycode uint32, depressed, latched, locked, layout uint32) string {
	app.xkb.stateUpdateMask(app.xkbShortcutState, depressed, latched, locked, 0, 0, layout)
	if character, ok := keysymRune(app.xkb.stateKeyGetOneSym(app.xkbShortcutState, keycode)); ok {
		return string(character)
	}
	return ""
}

// keyLevels returns the low and shifted symbol of every layout for a key,
// cached until the keymap changes. The shortcut path is hot, and each press
// would otherwise walk the keymap twice per layout.
func (app *waylandApplication) keyLevels(key uint32) [][2]uint32 {
	if app.xkb == nil || app.xkbKeymap == 0 || app.xkb.keymapKeyGetSyms == nil {
		return nil
	}
	if levels, ok := app.xkbLevels[key]; ok {
		return levels
	}
	layouts := uint32(2)
	if app.xkb.keymapLayoutsForKey != nil {
		if n := app.xkb.keymapLayoutsForKey(app.xkbKeymap, key); n > 0 {
			layouts = n
		}
	}
	var levels [][2]uint32
	for layout := uint32(0); layout < layouts; layout++ {
		var pair [2]uint32
		var lowSym, highSym *uint32
		if app.xkb.keymapKeyGetSyms(app.xkbKeymap, key, layout, 0, &lowSym) > 0 && lowSym != nil {
			pair[0] = *lowSym
		}
		if app.xkb.keymapKeyGetSyms(app.xkbKeymap, key, layout, 1, &highSym) > 0 && highSym != nil {
			pair[1] = *highSym
		}
		// A layout with only the shifted level still has a shortcut character.
		if pair[0] == 0 && pair[1] == 0 {
			continue
		}
		levels = append(levels, pair)
	}
	if app.xkbLevels == nil {
		app.xkbLevels = make(map[uint32][][2]uint32)
	}
	app.xkbLevels[key] = levels
	return levels
}

// modifierMasks finds the Num Lock and AltGr/Mode_switch bits from one parse of
// the keymap; a layout switch runs this on the main loop while the user types.
// Num Lock is read the way X11 reads the modifier map: a keymap may bind it to
// Mod3 rather than Mod2, and xkb_state does not compute the virtual NumLock
// modifier, so the modifier map is the only source. AltGr falls back to Mod5
// as X11 does.
func modifierMasks(xkb *xkbAPI, keymap uintptr, data []byte) (numLock, level3 uint32) {
	if xkb.keymapModGetIndex == nil {
		// Match X11's conventional bits when an old libxkbcommon cannot answer.
		return modMask(xkb, keymap, strModNum), modMask(xkb, keymap, strMod5)
	}
	index := newKeymapIndex(data)
	numLock = index.modifierMaskForSymbol(xkb, keymap, "Num_Lock")
	if numLock == 0 {
		numLock = modMask(xkb, keymap, strModNum)
	}
	level3 = index.modifierMaskForSymbol(xkb, keymap, "ISO_Level3_Shift")
	if level3 == 0 {
		level3 = index.modifierMaskForSymbol(xkb, keymap, "Mode_switch")
	}
	if level3 == 0 {
		level3 = modMask(xkb, keymap, strMod5)
	}
	return numLock, level3
}

// keymapIndex is the serialized keymap split at its declarations once.
type keymapIndex struct {
	declarations []string
	modifierMaps []string
}

func newKeymapIndex(data []byte) keymapIndex {
	index := keymapIndex{}
	for _, declaration := range strings.Split(string(data), ";") {
		index.declarations = append(index.declarations, declaration)
		if start := strings.Index(declaration, "modifier_map"); start >= 0 {
			index.modifierMaps = append(index.modifierMaps, declaration[start+len("modifier_map"):])
		}
	}
	return index
}

// modifierMaskForSymbol is the real modifier bit of the modifier_map entry
// holding a key bound to symbol, or zero. xkbcommon serializes modifier_map
// entries by key name (<NMLK>), not keysym, so the keys carrying the symbol are
// found first. xkb_state does not compute virtual modifiers such as NumLock, so
// querying them reports inactive; the modifier map is the only source.
func (index keymapIndex) modifierMaskForSymbol(xkb *xkbAPI, keymap uintptr, symbol string) uint32 {
	if xkb.keymapModGetIndex == nil {
		return 0
	}
	keyNames := map[string]bool{}
	for _, declaration := range index.declarations {
		if start := strings.Index(declaration, "key "); start >= 0 && strings.Contains(declaration, symbol) {
			fields := strings.Fields(declaration[start+len("key "):])
			if len(fields) > 0 && strings.HasPrefix(fields[0], "<") {
				keyNames[fields[0]] = true
			}
		}
	}
	for _, body := range index.modifierMaps {
		for key := range keyNames {
			if !strings.Contains(body, key) {
				continue
			}
			fields := strings.Fields(body)
			if len(fields) == 0 {
				continue
			}
			name := append([]byte(fields[0]), 0)
			if index := xkb.keymapModGetIndex(keymap, &name[0]); index < 32 {
				return 1 << index
			}
		}
	}
	return 0
}

// modMask is the real modifier bit a modifier name stands for, or zero. The
// real names have fixed bits in XKB, which is the fallback when an old
// libxkbcommon cannot answer.
func modMask(xkb *xkbAPI, keymap uintptr, name []byte) uint32 {
	if xkb.keymapModGetIndex != nil {
		if index := xkb.keymapModGetIndex(keymap, &name[0]); index < 32 {
			return 1 << index
		}
		return 0
	}
	switch string(name[:len(name)-1]) {
	case "Shift":
		return 1 << 0
	case "Lock":
		return 1 << 1
	case "Control":
		return 1 << 2
	case "Mod1":
		return 1 << 3
	case "Mod2":
		return 1 << 4
	case "Mod4":
		return 1 << 6
	case "Mod5":
		return 1 << 7
	}
	return 0
}

// keyPressEvent builds the event for a key going down, from the layout state as
// it stands now, so a repeat follows modifiers pressed after the key went down.
func (app *waylandApplication) keyPressEvent(scancode uint32, router eventRouter) (platform.KeyEvent, uint32) {
	keysym := app.resolveKeysym(scancode)
	text := ""
	if r, ok := keysymRune(keysym); ok {
		text = string(r)
	}
	wanted := router.wantsKeyReleases()
	texts := keyText{text: text}
	if wanted {
		// A raw terminal key names the character without the chord's modifiers
		// in Text and what the modifiers type in Typed, as on X11.
		texts.base = app.shortcutText(scancode, 0, 0, 0, 0)
	} else if app.modifiers&(input.ModifierControl|input.ModifierMeta) != 0 {
		texts.shortcut = app.shortcutText(scancode, app.xkbDepressed, app.xkbLatched, app.xkbLocked, app.xkbGroup)
	}
	return keyEventFor(normalizedKeysym(keysym), app.modifiers, scancode, texts, wanted), keysym
}

// keyRepeats reports whether the keymap marks the key repeatable; with no
// keymap to say otherwise, a key repeats.
func (app *waylandApplication) keyRepeats(scancode uint32) bool {
	if app.xkb != nil && app.xkbKeymap != 0 && app.xkb.keymapKeyRepeats != nil {
		return app.xkb.keymapKeyRepeats(app.xkbKeymap, scancode+8) != 0
	}
	return true
}

// keepsRepeat reports whether pressing a key leaves another held key's repeat
// running: modifiers and the lock keys do, as on X11.
func (app *waylandApplication) keepsRepeat(key input.Key, keysym, scancode uint32) bool {
	if key.IsModifier() {
		return true
	}
	switch keysym {
	case keysymCapsLock, keysymNumLock, keysymScrollLock, keysymModeSwitch:
		return true
	}
	// Without a keymap the evdev fallback leaves these keysyms at zero, so the
	// scancode decides; with one, the key's own symbols have already said it.
	if keysym != 0 {
		return false
	}
	switch scancode {
	case 29, 97, 42, 54, 56, 100, 125, 126, 58, 69, 70:
		return true
	}
	return false
}

func (app *waylandApplication) handleKey(serial, timeMs, scancode, state uint32) {
	if app.keyboardFocus == nil || app.keyboardFocus.state == nil {
		return
	}
	win := app.keyboardFocus
	isDown := (state == 1)
	router := win.state.router

	if isDown {
		keyEvent, keysym := app.keyPressEvent(scancode, router)
		// A modifier or lock key pressed during a repeat leaves the repeating
		// key alone, as server-side repeat does on X11; any other key stops it,
		// repeating or not. Modifiers and lock keys never repeat themselves,
		// even where a keymap marks one repeatable (<SCLK> is).
		if !app.keepsRepeat(keyEvent.Key, keysym, scancode) {
			app.stopKeyRepeat()
			if app.keyRepeats(scancode) {
				app.startKeyRepeat(scancode)
			}
		}
		app.fail(router.waylandKeyPress(keyEvent, keysym, scancode))
		return
	}

	keyEvent, keysym := app.releaseEvent(scancode, router, app.modifiers)
	app.mu.Lock()
	isRepeatingKey := (scancode == app.repeatScancode)
	app.mu.Unlock()
	if isRepeatingKey {
		app.stopKeyRepeat()
	}
	// The input method hears the modifiers as they stood when the key came up,
	// before this release dropped its bit, as X11 forwards the raw event state.
	app.fail(router.waylandKeyRelease(keyEvent, keysym, scancode, app.modifiers))
}

func (app *waylandApplication) startKeyRepeat(scancode uint32) {
	if app.repeatRate <= 0 {
		return
	}

	delay := time.Duration(app.repeatDelay) * time.Millisecond
	if delay <= 0 {
		delay = 400 * time.Millisecond
	}
	interval := time.Second / time.Duration(app.repeatRate)
	if interval <= 0 {
		interval = 40 * time.Millisecond
	}
	now := app.timeNow()

	app.mu.Lock()
	app.repeatScancode = scancode
	app.repeatWindow = app.keyboardFocus
	app.repeatInterval = interval
	app.repeatDeadline = now.Add(delay)
	app.mu.Unlock()
}

func (app *waylandApplication) runKeyRepeat(now time.Time) {
	app.mu.Lock()
	if app.repeatScancode == 0 || now.Before(app.repeatDeadline) {
		app.mu.Unlock()
		return
	}
	win := app.repeatWindow
	scancode := app.repeatScancode
	generation := app.repeatGeneration
	deadline := app.repeatDeadline
	interval := app.repeatInterval
	app.mu.Unlock()

	if win == nil || win.closed.Load() || win.state == nil {
		app.stopKeyRepeat()
		return
	}
	app.dispatchKeyRepeat(win, scancode, generation)
	if interval <= 0 {
		interval = 40 * time.Millisecond
	}

	app.mu.Lock()
	if app.repeatGeneration == generation && app.repeatScancode == scancode && app.repeatWindow == win {
		next := deadline.Add(interval)
		if !next.After(now) {
			next = now.Add(interval)
		}
		app.repeatDeadline = next
	}
	app.mu.Unlock()
}

func (app *waylandApplication) dispatchKeyRepeat(win *waylandWindow, scancode uint32, generation uint64) {
	app.mu.Lock()
	active := app.repeatScancode == scancode && app.repeatGeneration == generation && app.repeatWindow == win && app.keyboardFocus == win
	app.mu.Unlock()
	if !active || win == nil || win.closed.Load() || win.state == nil {
		return
	}
	// Rebuild the event so a repeat follows modifiers and layout changes made
	// while the key was held.
	keyEvent, keysym := app.keyPressEvent(scancode, win.state.router)
	keyEvent.Repeat = true
	app.fail(win.state.router.waylandKeyPress(keyEvent, keysym, scancode))
}

func (app *waylandApplication) stopKeyRepeat() {
	app.mu.Lock()
	defer app.mu.Unlock()
	app.repeatGeneration++
	app.repeatScancode = 0
	app.repeatWindow = nil
	app.repeatDeadline = time.Time{}
	app.repeatInterval = 0
}

// fallbackEvdevToKeysym handles basic ASCII keys when libxkbcommon is not present.
func fallbackEvdevToKeysym(scancode uint32) uint32 {
	switch scancode {
	case 1:
		return keysymEscape
	case 14:
		return keysymBackspace
	case 15:
		return keysymTab
	case 28:
		return keysymEnter
	case 57:
		return ' '
	case 103:
		return keysymUp
	case 105:
		return keysymLeft
	case 106:
		return keysymRight
	case 108:
		return keysymDown
	case 111:
		return keysymDelete
	default:
		return 0
	}
}
