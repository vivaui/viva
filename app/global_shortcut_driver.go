//go:build darwin || windows || linux

package app

import (
	"fmt"
	"sync"
)

func driverRegisterGlobalShortcut(s GlobalShortcut, pressed func()) (func(), error, bool) {
	system := activeDriverSystem.Load()
	if system == nil {
		if nonNativeSystemUsed.Load() {
			return nil, ErrUnsupported, true
		}
		return nil, nil, false
	}
	system.globalMu.Lock()
	defer system.globalMu.Unlock()
	for _, b := range system.globalBindings {
		if b.shortcut == s {
			return nil, fmt.Errorf("app: global shortcut is already registered"), true
		}
	}
	binding := &driverGlobalBinding{shortcut: s, pressed: pressed}
	system.globalBindings = append(system.globalBindings, binding)
	var once sync.Once
	return func() {
		once.Do(func() {
			system.globalMu.Lock()
			defer system.globalMu.Unlock()
			for i, b := range system.globalBindings {
				if b == binding {
					copy(system.globalBindings[i:], system.globalBindings[i+1:])
					system.globalBindings[len(system.globalBindings)-1] = nil
					system.globalBindings = system.globalBindings[:len(system.globalBindings)-1]
					return
				}
			}
		})
	}, nil, true
}

// PressGlobalShortcut invokes a private registration without touching the OS.
// Like other Driver input operations it must be called on the driver thread.
func (driver *Driver) PressGlobalShortcut(s GlobalShortcut) (bool, error) {
	if driver.closed {
		return false, fmt.Errorf("app: driver is closed")
	}
	s, err := normalizeGlobalShortcut(s)
	if err != nil {
		return false, err
	}
	driver.activateSystem()
	driver.system.globalMu.Lock()
	var pressed func()
	for _, b := range driver.system.globalBindings {
		if b.shortcut == s {
			pressed = b.pressed
			break
		}
	}
	driver.system.globalMu.Unlock()
	if pressed != nil {
		pressed()
		driver.requestWake()
		return true, nil
	}
	return false, nil
}
