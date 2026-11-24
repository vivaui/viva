//go:build darwin && !ios && cgo

package devicekey

/*
#cgo LDFLAGS: -framework Foundation -framework Security -framework LocalAuthentication
#include <stdlib.h>
#include "native_darwin.h"
*/
import "C"

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"unsafe"
)

const (
	keychainAdd = iota + 1
	keychainRead
	keychainDelete
)

func nativeSupported() bool { return C.viva_device_key_supported() != 0 }
func nativeMethod() VerificationMethod {
	switch C.viva_device_key_supported() {
	case 2:
		return VerificationTouchID
	case 1:
		return VerificationDevicePassword
	default:
		return VerificationUnavailable
	}
}

func nativeEnroll(ctx context.Context, options Options, key []byte) (envelope, error) {
	return enrollInKeychain(ctx, options, key, keychain)
}

func enrollInKeychain(ctx context.Context, options Options, key []byte, run func(context.Context, Options, []byte, []byte, int) (int64, []byte, error)) (envelope, error) {
	value := envelope{Platform: "darwin-keychain-presence", ID: make([]byte, 32)}
	rand.Read(value.ID)
	status, _, err := run(ctx, options, value.ID, key, keychainAdd)
	if err == nil {
		err = keychainError(status)
	}
	if err != nil {
		return envelope{}, err
	}
	// CopyMatching enforces the item's userPresence access control. Successful
	// storage alone is not treated as proof that native authentication works.
	status, verified, err := run(ctx, options, value.ID, nil, keychainRead)
	if err == nil && status == 0 && !equal(verified, key) {
		err = ErrInvalidBlob
	}
	clear(verified)
	if err == nil {
		err = keychainError(status)
	}
	if err != nil {
		_, _, _ = run(context.Background(), options, value.ID, nil, keychainDelete)
		return envelope{}, err
	}
	return value, nil
}

func nativeUnseal(ctx context.Context, options Options, value envelope) ([]byte, error) {
	if value.Platform != "darwin-keychain-presence" || len(value.ID) != 32 || len(value.Ciphertext) != 0 || len(value.Salt) != 0 || len(value.Nonce) != 0 {
		return nil, ErrInvalidBlob
	}
	status, key, err := keychain(ctx, options, value.ID, nil, keychainRead)
	if err == nil {
		err = keychainError(status)
	}
	if err != nil {
		clear(key)
		return nil, err
	}
	return key, nil
}

func nativeRevoke(ctx context.Context, options Options, value envelope) error {
	if value.Platform != "darwin-keychain-presence" || len(value.ID) != 32 {
		return ErrInvalidBlob
	}
	status, _, err := keychain(ctx, options, value.ID, nil, keychainDelete)
	if err != nil {
		return err
	}
	return keychainError(status)
}

func keychain(ctx context.Context, options Options, id, key []byte, action int) (int64, []byte, error) {
	if err := ctx.Err(); err != nil {
		return 0, nil, err
	}
	operation := C.viva_device_key_operation_create()
	if operation == nil {
		return 0, nil, ErrUnavailable
	}
	finished := make(chan struct{})
	stop := context.AfterFunc(ctx, func() { C.viva_device_key_operation_cancel(operation); close(finished) })
	defer func() {
		if !stop() {
			<-finished
		}
		C.viva_device_key_operation_release(operation)
	}()
	service := C.CString(options.Service + ".viva.devicekey.v1")
	account := C.CString(hex.EncodeToString(binding(options)) + "." + hex.EncodeToString(id))
	reason := C.CString(options.Reason)
	defer C.free(unsafe.Pointer(service))
	defer C.free(unsafe.Pointer(account))
	defer C.free(unsafe.Pointer(reason))
	var input unsafe.Pointer
	if len(key) != 0 {
		input = C.CBytes(key)
		defer C.viva_device_key_bytes_release(input, C.size_t(len(key)))
	}
	var output unsafe.Pointer
	var size C.size_t
	status := C.viva_device_key_operation_run(operation, service, account, reason, C.int(action), input, C.size_t(len(key)), &output, &size)
	defer C.viva_device_key_bytes_release(output, size)
	// Successful adds and deletes already changed the Keychain. Report the
	// committed outcome even if cancellation arrives as Security returns.
	if err := ctx.Err(); err != nil && (action == keychainRead || status != 0) {
		return 0, nil, err
	}
	if size > MaxKeySize {
		return 0, nil, ErrInvalidBlob
	}
	var result []byte
	if output != nil && size > 0 {
		result = C.GoBytes(output, C.int(size))
	}
	return int64(status), result, nil
}

func keychainError(status int64) error {
	switch status {
	case 0:
		return nil
	case -128:
		return ErrCanceled
	case -25300:
		return ErrNotFound
	default:
		return fmt.Errorf("%w: Keychain OSStatus %d", ErrUnavailable, status)
	}
}
