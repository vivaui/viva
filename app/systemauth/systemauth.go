// Package systemauth asks the operating system to verify the current user.
// It returns a verification result for an application UI lock. It does not
// create, store, or release cryptographic keys.
package systemauth

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	// ErrUnavailable means the system cannot verify the user here: the
	// platform has no such facility, or the user has not set one up, or policy
	// turns it off. Retrying does not help until that changes.
	ErrUnavailable = errors.New("system authentication is unavailable")
	// ErrCanceled means the user, or the system on the user's behalf, dismissed
	// the prompt. It is a refusal, never a success.
	ErrCanceled = errors.New("system authentication was canceled")
	// ErrForegroundRequired means no window of this process was in the
	// foreground, so the system had nothing to attach its prompt to. Call Verify
	// again once the application's window is active.
	ErrForegroundRequired = errors.New("system authentication requires an active window")
	// ErrFailed means the prompt ran and the user was not verified, or the
	// request could not be made because another verification is already showing.
	ErrFailed = errors.New("system authentication failed")
)

// VerificationMethod is how the system verifies the current user.
type VerificationMethod uint8

const (
	// VerificationUnavailable is the zero value: the system offers no way to
	// verify the user.
	VerificationUnavailable VerificationMethod = iota
	// VerificationWindowsHello is Windows Hello (face, fingerprint or PIN).
	VerificationWindowsHello
	// VerificationTouchID is Touch ID, with the system password as the fallback
	// the system itself offers.
	VerificationTouchID
	// VerificationDevicePassword is the device-owner password, on a Mac without
	// Touch ID.
	VerificationDevicePassword
)

// Method checks the system's authentication configuration without opening a prompt.
func Method() VerificationMethod { return nativeMethod() }

var prompt = make(chan struct{}, 1)

// Verify requests a fresh system prompt. Call from a worker goroutine while
// the application's window is in the foreground. Cancellation never returns
// success, even if the native prompt completed at the same time.
func Verify(ctx context.Context, reason string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if strings.TrimSpace(reason) == "" || len(reason) > 1024 || !utf8.ValidString(reason) || strings.ContainsRune(reason, 0) {
		return errors.New("system authentication requires a valid, nonempty reason")
	}
	select {
	case prompt <- struct{}{}:
		defer func() { <-prompt }()
	default:
		return ErrFailed
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	err := nativeVerify(ctx, reason)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return err
}
