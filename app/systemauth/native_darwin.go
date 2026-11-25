//go:build darwin && !ios && cgo

package systemauth

/*
#cgo LDFLAGS: -framework Foundation -framework LocalAuthentication
#include <stdlib.h>
#include "native_darwin.h"
*/
import "C"

import (
	"context"
	"fmt"
	"unsafe"
)

func nativeMethod() VerificationMethod { return VerificationMethod(C.viva_systemauth_method()) }

func nativeVerify(ctx context.Context, reason string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	auth := C.viva_systemauth_new()
	if auth == nil {
		return ErrUnavailable
	}
	defer C.viva_systemauth_free(auth)
	prompt := C.CString(reason)
	defer C.free(unsafe.Pointer(prompt))
	done := make(chan struct{})
	stop := context.AfterFunc(ctx, func() { C.viva_systemauth_cancel(auth); close(done) })
	status := int(C.viva_systemauth_verify(auth, prompt))
	if !stop() {
		<-done
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	switch status {
	case 0:
		return nil
	case -2, -4, -9:
		return ErrCanceled // user, system or app canceled
	case -3, -5, -6, -7, -8, -10:
		return fmt.Errorf("%w (LocalAuthentication %d)", ErrUnavailable, status)
	default:
		return fmt.Errorf("%w (LocalAuthentication %d)", ErrFailed, status)
	}
}
