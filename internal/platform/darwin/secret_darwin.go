//go:build darwin && !ios

package darwin

/*
#cgo LDFLAGS: -framework Foundation -framework Security
#include <stdlib.h>
#include "secret_darwin.h"
*/
import "C"

import (
	"fmt"
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
)

// OSStatus values the secret store answers with that an application acts on.
const (
	statusNotFound      = -25300 // errSecItemNotFound
	statusUserCanceled  = -128   // errSecUserCanceled
	statusAuthFailed    = -25293 // errSecAuthFailed
	statusNoInteraction = -25308 // errSecInteractionNotAllowed
	statusNotAvailable  = -25291 // errSecNotAvailable: no keychain, as in a headless session
)

func secretStatus(operation string, status C.int) error {
	switch int(status) {
	case 0:
		return nil
	case statusNotFound:
		return platform.ErrSecretNotFound
	case statusUserCanceled, statusAuthFailed, statusNoInteraction:
		return fmt.Errorf("%w: %s (status %d)", platform.ErrSecretDenied, operation, int(status))
	case statusNotAvailable:
		return fmt.Errorf("%w: %s (status %d)", platform.ErrSecretStoreUnavailable, operation, int(status))
	}
	return fmt.Errorf("darwin: keychain %s failed (status %d)", operation, int(status))
}

// StoreSecret saves a generic password in the user's login keychain, replacing
// one of the same service and account.
func StoreSecret(service, account string, secret []byte) error {
	cservice, caccount := C.CString(service), C.CString(account)
	defer C.free(unsafe.Pointer(cservice))
	defer C.free(unsafe.Pointer(caccount))
	data := C.CBytes(secret)
	defer C.free(data)
	return secretStatus("store", C.viva_secret_store(cservice, caccount, (*C.uchar)(data), C.size_t(len(secret))))
}

// LoadSecret reads the generic password saved for the service and account.
func LoadSecret(service, account string) ([]byte, error) {
	cservice, caccount := C.CString(service), C.CString(account)
	defer C.free(unsafe.Pointer(cservice))
	defer C.free(unsafe.Pointer(caccount))
	var data *C.uchar
	var length C.size_t
	if err := secretStatus("load", C.viva_secret_load(cservice, caccount, &data, &length)); err != nil {
		return nil, err
	}
	defer C.free(unsafe.Pointer(data))
	return C.GoBytes(unsafe.Pointer(data), C.int(length)), nil
}

// DeleteSecret removes the generic password; one that is not there is not an
// error.
func DeleteSecret(service, account string) error {
	cservice, caccount := C.CString(service), C.CString(account)
	defer C.free(unsafe.Pointer(cservice))
	defer C.free(unsafe.Pointer(caccount))
	if err := secretStatus("delete", C.viva_secret_delete(cservice, caccount)); err != nil && err != platform.ErrSecretNotFound {
		return err
	}
	return nil
}
