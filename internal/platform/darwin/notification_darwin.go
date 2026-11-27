//go:build darwin && !ios

package darwin

/*
#include <stdlib.h>
#include "system_darwin.h"
*/
import "C"

import (
	"fmt"
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
)

var notificationCallbacks platform.NotificationRegistry

func ShowNotification(title, body string, events platform.NotificationEvents) (func(), error) {
	id := notificationCallbacks.Add(events)
	titleText, err := systemCString("notification title", title)
	if err != nil {
		notificationCallbacks.Remove(id)
		return nil, err
	}
	bodyText, err := systemCString("notification body", body)
	if err != nil {
		notificationCallbacks.Remove(id)
		C.free(unsafe.Pointer(titleText))
		return nil, err
	}
	identifier := C.CString(id)
	defer C.free(unsafe.Pointer(identifier))
	defer C.free(unsafe.Pointer(titleText))
	defer C.free(unsafe.Pointer(bodyText))
	var message *C.char
	status := C.viva_system_show_interactive_notification(identifier, titleText, bodyText, &message)
	if status != 0 {
		notificationCallbacks.Remove(id)
		if status == -2 {
			if message != nil {
				C.viva_system_free(unsafe.Pointer(message))
			}
			return nil, ErrPermissionDenied
		}
		return nil, systemError("send interactive notification", message)
	}
	return func() {
		notificationCallbacks.Remove(id)
		identifier := C.CString(id)
		defer C.free(unsafe.Pointer(identifier))
		C.viva_system_remove_notification(identifier)
	}, nil
}

//export vivaSystemNotificationActivated
func vivaSystemNotificationActivated(identifier *C.char) {
	activateNotification(C.GoString(identifier))
}

//export vivaSystemNotificationIsActive
func vivaSystemNotificationIsActive(identifier *C.char) C.int {
	if notificationCallbacks.Has(C.GoString(identifier)) {
		return 1
	}
	return 0
}

//export vivaSystemNotificationFailed
func vivaSystemNotificationFailed(identifier, message *C.char, denied C.int) {
	failNotification(C.GoString(identifier), C.GoString(message), denied != 0)
}

func failNotification(identifier, message string, denied bool) {
	events := notificationCallbacks.Take(identifier)
	if events.Failed == nil {
		return
	}
	var err error = fmt.Errorf("darwin: notification delivery failed: %s", message)
	if denied {
		err = ErrPermissionDenied
	}
	events.Failed(err)
}

func activateNotification(identifier string) {
	if activate := notificationCallbacks.Take(identifier).Activated; activate != nil {
		activate()
	}
}
