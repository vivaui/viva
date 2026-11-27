package windows

import (
	"fmt"
	"sync"
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
)

const (
	wmNotificationActivation        = 0x8014
	notificationBalloonClick        = 0x0405 // NIN_BALLOONUSERCLICK
	notificationIDNamespace  uint32 = 1 << 31
)

type interactiveNotification struct {
	host     uintptr
	activate func()
}

var interactiveNotifications = struct {
	sync.Mutex
	next    uint32
	entries map[uint32]interactiveNotification
}{}

// ShowNotification serializes Shell and window access on the owning event loop.
// Closing invalidates callbacks immediately, even before queued native work runs.
func ShowNotification(title, body string, events platform.NotificationEvents) (func(), error) {
	application := wakeOwner.Load()
	if application == nil {
		return nil, fmt.Errorf("windows: notification requires a running application")
	}
	var mu sync.Mutex
	closed := false
	var nativeCancel func()
	activate := func() {
		mu.Lock()
		active := !closed
		closed = true
		mu.Unlock()
		if active && events.Activated != nil {
			events.Activated()
		}
	}
	application.Post(func() {
		mu.Lock()
		stopped := closed
		mu.Unlock()
		if stopped {
			return
		}
		cancel, err := showNotification(application, title, body, activate)
		mu.Lock()
		stopped = closed
		if !stopped {
			nativeCancel = cancel
		}
		mu.Unlock()
		if stopped && cancel != nil {
			cancel()
		}
		if err != nil {
			mu.Lock()
			active := !closed
			closed = true
			mu.Unlock()
			if active && events.Failed != nil {
				events.Failed(err)
			}
		}
	})
	return func() {
		mu.Lock()
		closed = true
		cancel := nativeCancel
		nativeCancel = nil
		mu.Unlock()
		if cancel != nil {
			application.Post(cancel)
		}
	}, nil
}

func showNotification(application *windowsApplication, title, body string, activate func()) (func(), error) {
	// The event-loop carrier outlives ordinary windows. Closing the window that
	// happened to be active must not retire an application-owned notification.
	host := application.wakeWindow.Load()
	if host == 0 {
		return nil, fmt.Errorf("windows: notification requires a running application")
	}
	icon := notificationIcon(notificationHost())
	if icon == 0 {
		var err error
		icon, _, err = loadIcon.Call(0, 32512)
		if icon == 0 {
			return nil, windowsCallError("load notification icon", err)
		}
	}
	interactiveNotifications.Lock()
	if interactiveNotifications.next >= notificationIDNamespace-1 {
		interactiveNotifications.Unlock()
		return nil, fmt.Errorf("windows: notification identifiers exhausted")
	}
	interactiveNotifications.next++
	id := notificationIDNamespace | interactiveNotifications.next
	if interactiveNotifications.entries == nil {
		interactiveNotifications.entries = make(map[uint32]interactiveNotification)
	}
	interactiveNotifications.entries[id] = interactiveNotification{host: host, activate: activate}
	interactiveNotifications.Unlock()
	// NIF_INFO with an empty body withdraws a balloon. Preserve title-only
	// requests as body text rather than accidentally asking the Shell to remove it.
	if body == "" {
		body = title
		title = ""
	}
	data := notificationIconData(host, icon, title, body)
	data.ID = id
	data.Flags |= notifyMessage
	data.CallbackMessage = wmNotificationActivation
	// Retain the legacy callback layout deliberately: wParam contains the full
	// 32-bit uID. Version 4 truncates it to 16 bits and can alias stale callbacks.
	if ok, _, err := callNotifyIcon(notifyAdd, &data); ok == 0 {
		forgetInteractiveNotification(host, id)
		return nil, windowsCallError("send interactive notification", err)
	}
	return func() { forgetInteractiveNotification(host, id) }, nil
}

func forgetInteractiveNotification(host uintptr, id uint32) func() {
	interactiveNotifications.Lock()
	entry, ok := interactiveNotifications.entries[id]
	if ok && entry.host == host {
		delete(interactiveNotifications.entries, id)
	} else {
		ok = false
	}
	interactiveNotifications.Unlock()
	if !ok {
		return nil
	}
	data := notifyIconData{Size: uint32(unsafe.Sizeof(notifyIconData{})), Window: host, ID: id}
	callNotifyIcon(notifyDelete, &data)
	return entry.activate
}

func activateSystemNotification(host uintptr, id, event uint32) {
	if event != notificationBalloonClick {
		return
	}
	if activate := forgetInteractiveNotification(host, id); activate != nil {
		activate()
	}
}

func removeInteractiveNotifications(host uintptr) {
	interactiveNotifications.Lock()
	var ids []uint32
	for id, entry := range interactiveNotifications.entries {
		if entry.host == host {
			ids = append(ids, id)
		}
	}
	interactiveNotifications.Unlock()
	for _, id := range ids {
		forgetInteractiveNotification(host, id)
	}
}
