// Package dbus is the smallest D-Bus client a status icon needs: connect to the
// session or system bus, authenticate as the current user, call methods, answer
// the calls an object receives, and emit signals. Values are encoded by typed
// methods, with no reflection; only little-endian messages are understood, and
// file descriptors can be received but not sent, which is what a status icon
// and logind's inhibitor lock need.
//
// A Conn is not safe for concurrent use; its owner drives it from one thread,
// waiting on FD for the bus to have something to read.
package dbus
