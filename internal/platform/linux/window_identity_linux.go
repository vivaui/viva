package linux

import (
	"os"
	"unsafe"
)

// xcbAtomWMClientMachine is the predefined atom of WM_CLIENT_MACHINE.
const xcbAtomWMClientMachine = 36

// setIdentity names the window and its process to the window manager: the title
// (see setTitle), and the process id with the host it runs on, which is how a
// manager offers to end a window that stopped answering its ping.
func (win *window) setIdentity(title string) {
	api, connection := win.api, win.connection
	if title != "" {
		win.setTitle(title)
	}
	if host, err := os.Hostname(); err == nil && host != "" {
		machine := []byte(host)
		api.changeProperty(connection, xcbPropertyReplace, win.id, xcbAtomWMClientMachine, xcbAtomString, 8, uint32(len(machine)), unsafe.Pointer(&machine[0]))
		if pidAtom, err := api.atom(connection, "_NET_WM_PID"); err == nil {
			pid := uint32(os.Getpid())
			api.changeProperty(connection, xcbPropertyReplace, win.id, pidAtom, xcbAtomCardinal, 32, 1, unsafe.Pointer(&pid))
		}
	}
}

// setTitle writes the title in both forms: _NET_WM_NAME is UTF-8, WM_NAME can only
// carry Latin-1 and is what older managers read. An empty title clears both.
func (win *window) setTitle(title string) {
	api, connection := win.api, win.connection
	legacy := latin1(title)
	api.changeProperty(connection, xcbPropertyReplace, win.id, xcbAtomWMName, xcbAtomString, 8, uint32(len(legacy)), propertyData(legacy))
	utf8String, err := api.atom(connection, "UTF8_STRING")
	name, nameErr := api.atom(connection, "_NET_WM_NAME")
	if err == nil && nameErr == nil {
		encoded := []byte(title)
		api.changeProperty(connection, xcbPropertyReplace, win.id, name, utf8String, 8, uint32(len(encoded)), propertyData(encoded))
	}
}

// propertyData is the data pointer for a property of len(value) bytes. An empty value has
// no first byte to point at, and X11 reads nothing from the pointer when the length is zero.
func propertyData(value []byte) unsafe.Pointer {
	if len(value) == 0 {
		return nil
	}
	return unsafe.Pointer(&value[0])
}
