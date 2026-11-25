//go:build !windows && (!darwin || ios || !cgo)

package systemauth

import "context"

func nativeMethod() VerificationMethod           { return VerificationUnavailable }
func nativeVerify(context.Context, string) error { return ErrUnavailable }
