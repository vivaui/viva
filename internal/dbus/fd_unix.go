//go:build unix

package dbus

import "syscall"

// TakeFD removes and returns the descriptor a body's unix fd index names, or
// -1 when the message carries none at that index.
func (m *Message) TakeFD(index int) int {
	if index < 0 || index >= len(m.FDs) {
		return -1
	}
	fd := m.FDs[index]
	m.FDs = append(m.FDs[:index], m.FDs[index+1:]...)
	return fd
}

// CloseFDs closes every descriptor the message still carries.
func (m *Message) CloseFDs() {
	for _, fd := range m.FDs {
		syscall.Close(fd)
	}
	m.FDs = nil
}
