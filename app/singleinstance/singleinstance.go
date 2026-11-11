//go:build darwin || windows || linux

// Package singleinstance keeps an application to one running copy per user.
// It is separate from package app because it uses a local socket and the net
// package: an application that does not import this package does not link
// them.
package singleinstance

import (
	"errors"

	"github.com/vivaui/viva/internal/instance"
)

// Instance is this process's claim to be the one running copy of an
// application. Close releases it.
type Instance struct{ inner *instance.Instance }

// Close releases the claim, so that the next launch starts afresh. It is safe
// to call on a nil Instance and more than once.
func (claim *Instance) Close() {
	if claim != nil {
		claim.inner.Close()
	}
}

// SingleInstance keeps an application to one running copy per user. Call it
// first thing in main, before any window exists.
//
// If no other copy is running, it returns a claim and primary true; onLaunch is
// then called, on a goroutine of its own, with the arguments of every later
// launch (as the user typed them, without the program name). A function that
// wakes the window, such as an app.Invalidator, is the way to get the work back
// to the event loop. If a copy is already running, args are handed to it,
// primary is false, and the caller should exit without opening a window.
//
// id names the application and must be the same for every copy, as its bundle
// or reverse-DNS identifier. Copies run by different users do not meet. The
// claim lives in a per-user directory the platform keeps across launches, so
// copies started from a desktop and from a terminal meet whatever temporary
// directories their shells set. An error means the claim could not be made
// either way; the caller may carry on as an unclaimed copy.
func SingleInstance(id string, args []string, onLaunch func(args []string)) (claim *Instance, primary bool, err error) {
	if onLaunch == nil {
		return nil, false, errors.New("singleinstance: a launch handler is required")
	}
	inner, primary, err := instance.Claim(id, args, onLaunch)
	if err != nil || !primary {
		return nil, false, err
	}
	return &Instance{inner: inner}, true, nil
}
