//go:build !windows && (!darwin || ios || !cgo)

package devicekey

import "context"

func nativeSupported() bool            { return false }
func nativeMethod() VerificationMethod { return VerificationUnavailable }
func nativeEnroll(context.Context, Options, []byte) (envelope, error) {
	return envelope{}, ErrUnavailable
}
func nativeUnseal(context.Context, Options, envelope) ([]byte, error) { return nil, ErrUnavailable }
func nativeRevoke(context.Context, Options, envelope) error           { return ErrUnavailable }
