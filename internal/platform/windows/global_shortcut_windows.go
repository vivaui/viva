//go:build windows

package windows

import (
	"fmt"
	"runtime"
	"sync"
	"unsafe"
)

// RegisterGlobalShortcut uses a private, blocking Win32 message queue. The
// thread exists only for a live registration and never polls or paints.
func RegisterGlobalShortcut(modifiers, key uint32, pressed func()) (func(), error) {
	registerGlobalHotKey := user.NewProc("RegisterHotKey")
	unregisterGlobalHotKey := user.NewProc("UnregisterHotKey")
	peekGlobalHotKeyMessage := user.NewProc("PeekMessageW")
	if key == 0x7b {
		return nil, fmt.Errorf("windows: F12 is reserved for debuggers")
	}
	type ready struct {
		thread uintptr
		err    error
	}
	started := make(chan ready, 1)
	done := make(chan struct{})
	events := make(chan struct{}, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		defer close(done)
		var msg message
		peekGlobalHotKeyMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0, 0)
		ok, _, err := registerGlobalHotKey.Call(0, 1, uintptr(modifiers|0x4000), uintptr(key))
		if ok == 0 {
			started <- ready{err: fmt.Errorf("windows: register global shortcut: %w", err)}
			return
		}
		defer unregisterGlobalHotKey.Call(0, 1)
		thread, _, _ := getCurrentThread.Call()
		started <- ready{thread: thread}
		for {
			result, _, _ := getMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
			if result == 0 || int32(result) == -1 {
				return
			}
			if msg.Kind == 0x0312 {
				select {
				case events <- struct{}{}:
				default:
				}
			}
		}
	}()
	r := <-started
	if r.err != nil {
		return nil, r.err
	}
	go func() {
		for {
			select {
			case <-events:
				select {
				case <-done:
					return
				default:
				}
				pressed()
			case <-done:
				return
			}
		}
	}()
	var once sync.Once
	return func() {
		once.Do(func() {
			select {
			case <-done:
				return
			default:
			}
			postThreadMessage.Call(r.thread, 0x0012, 0, 0)
			<-done
		})
	}, nil
}
