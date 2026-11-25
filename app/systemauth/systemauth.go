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
	ErrUnavailable        = errors.New("system authentication is unavailable")
	ErrCanceled           = errors.New("system authentication was canceled")
	ErrForegroundRequired = errors.New("system authentication requires an active window")
	ErrFailed             = errors.New("system authentication failed")
)

type VerificationMethod uint8

const (
	VerificationUnavailable VerificationMethod = iota
	VerificationWindowsHello
	VerificationTouchID
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
