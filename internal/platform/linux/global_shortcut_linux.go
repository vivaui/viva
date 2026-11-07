package linux

import (
	"fmt"
	"os"
	"runtime"
	"sync"
	"syscall"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/vivaui/viva/ui"
)

// RegisterGlobalShortcut grabs the specified key on an independent X11
// connection, so it can receive the chord even when all app windows are hidden.
// The stop pipe interrupts a blocking poll. Closing the connection releases
// every grab, including the ones made before a conflict.
func RegisterGlobalShortcut(modifiers ui.Modifiers, symbol uint32, pressed func()) (func(), error) {
	api, connection, screen, err := openSystemConnection()
	if err != nil {
		return nil, err
	}
	closeConnection := func() { api.disconnect(connection); api.close() }
	success := false
	defer func() {
		if !success {
			closeConnection()
		}
	}()

	w := window{api: api, connection: connection}
	if err = w.reloadKeymap(); err != nil {
		return nil, err
	}
	key := shortcutKeycode(&w.keymap, symbol)
	if key == 0 {
		return nil, fmt.Errorf("linux: global shortcut key is absent from the X keyboard map")
	}
	var grab func(uintptr, uint8, uint32, uint16, uint8, uint8, uint8) xcbVoidCookie
	var ungrab func(uintptr, uint8, uint32, uint16) xcbVoidCookie
	purego.RegisterLibFunc(&grab, api.library, "xcb_grab_key_checked")
	purego.RegisterLibFunc(&ungrab, api.library, "xcb_ungrab_key_checked")
	root := screen.Root
	var grabbed []uint16
	release := func() {
		for _, m := range grabbed {
			if reply := api.requestCheck(connection, ungrab(connection, key, root, m)); reply != nil {
				api.free(unsafe.Pointer(reply))
			}
		}
	}
	defer func() {
		if !success {
			release()
		}
	}()
	mask := shortcutModifierMask(modifiers)
	ignore := uint16(xcbModLock) | w.keymap.numLockMask()
	ignore &^= mask
	for locks := ignore; ; locks = (locks - 1) & ignore {
		const asyncGrab = 1
		cookie := grab(connection, 0, root, mask|locks, key, asyncGrab, asyncGrab)
		if reply := api.requestCheck(connection, cookie); reply != nil {
			code := reply.ErrorCode
			api.free(unsafe.Pointer(reply))
			if code == 10 {
				return nil, fmt.Errorf("linux: global shortcut is already grabbed")
			}
			return nil, fmt.Errorf("linux: grab global shortcut (X11 error %d)", code)
		}
		grabbed = append(grabbed, mask|locks)
		if locks == 0 {
			break
		}
	}
	read, write, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	success = true
	done, callbacks := make(chan struct{}), make(chan struct{}, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		defer close(done)
		defer closeConnection()
		defer release()
		defer read.Close()
		defer write.Close()
		runShortcutEvents(api, connection, int32(read.Fd()), key, mask, ignore, callbacks)
	}()
	go func() {
		for {
			select {
			case <-done:
				return
			case <-callbacks:
				select {
				case <-done:
					return
				default:
				}
				pressed()
			}
		}
	}()
	var once sync.Once
	return func() { once.Do(func() { _ = write.Close(); <-done }) }, nil
}

func shortcutModifierMask(modifiers ui.Modifiers) (mask uint16) {
	if modifiers&ui.ModifierShift != 0 {
		mask |= xcbModShift
	}
	if modifiers&ui.ModifierControl != 0 {
		mask |= xcbModControl
	}
	if modifiers&ui.ModifierAlt != 0 {
		mask |= xcbMod1
	}
	if modifiers&ui.ModifierMeta != 0 {
		mask |= xcbMod4
	}
	return
}

func shortcutKeycode(keymap *xcbKeymap, symbol uint32) uint8 {
	for code := int(keymap.first); code <= int(keymap.last); code++ {
		keys := keymap.symbols(uint8(code))
		for _, value := range keys {
			if value == symbol {
				return uint8(code)
			}
		}
	}
	return 0
}

type shortcutKeyState struct {
	down, released bool
	releasedAt     uint32
}

func (s *shortcutKeyState) event(kind uint8, timestamp uint32) bool {
	switch kind {
	case xcbKeyRelease:
		s.down, s.released, s.releasedAt = false, true, timestamp
	case xcbKeyPress:
		if s.down {
			return false
		}
		s.down = true
		return !s.released || s.releasedAt != timestamp
	}
	return false
}

func runShortcutEvents(api *xcbAPI, connection uintptr, wakeFD int32, key uint8, mask, ignore uint16, callbacks chan<- struct{}) {
	const pollRead int16 = 1
	fds := [2]pollFD{{FD: api.fileDescriptor(connection), Events: pollRead}, {FD: wakeFD, Events: pollRead}}
	var state shortcutKeyState
	for {
		for event := api.pollForEvent(connection); event != nil; event = api.pollForEvent(connection) {
			input := (*xcbInputEvent)(event)
			kind := input.ResponseType
			// Ignore synthetic SendEvent traffic. The server emits grabbed events.
			if (kind == xcbKeyPress || kind == xcbKeyRelease) && input.Detail == key {
				if kind == xcbKeyRelease || input.State&0xff&^ignore == mask {
					if state.event(kind, input.Time) {
						select {
						case callbacks <- struct{}{}:
						default:
						}
					}
				}
			}
			api.free(event)
		}
		if api.connectionHasError(connection) != 0 {
			return
		}
		if api.poll(&fds[0], 2, -1) < 0 {
			if *api.errnoLocation() == int32(syscall.EINTR) {
				continue
			}
			return
		}
		if fds[1].REvents != 0 || fds[0].REvents & ^pollRead != 0 {
			return
		}
	}
}
