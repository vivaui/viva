package windows

import (
	"errors"
	"fmt"
	"syscall"
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
)

const (
	credTypeGeneric         = 1
	credPersistLocalMachine = 2
	errorNotFound           = 1168 // ERROR_NOT_FOUND
	errorNoSuchLogonSession = 1312 // ERROR_NO_SUCH_LOGON_SESSION
	errorBadUsername        = 2202 // ERROR_BAD_USERNAME
	errorAccessDenied       = 5
)

// credential is CREDENTIALW.
type credential struct {
	Flags              uint32
	Type               uint32
	TargetName         *uint16
	Comment            *uint16
	LastWrittenLow     uint32
	LastWrittenHigh    uint32
	CredentialBlobSize uint32
	CredentialBlob     *byte
	Persist            uint32
	AttributeCount     uint32
	Attributes         uintptr
	TargetAlias        *uint16
	UserName           *uint16
}

var advapi = systemDLL("advapi32.dll")
var credWrite = advapi.NewProc("CredWriteW")
var credRead = advapi.NewProc("CredReadW")
var credDelete = advapi.NewProc("CredDeleteW")
var credFree = advapi.NewProc("CredFree")

// secretTarget is the name the credential is stored under: the service and the
// account, so two uses of the store never meet.
func secretTarget(service, account string) (*uint16, error) {
	return syscall.UTF16PtrFromString(service + ":" + account)
}

func secretError(operation string, code syscall.Errno) error {
	switch code {
	case errorNotFound:
		if operation == "write" {
			// Nothing was being looked for: a write the store cannot place
			// means it has nowhere to keep credentials for this logon, which
			// is "no secret store" to the caller, not a missing secret.
			return fmt.Errorf("%w: %s: %v", platform.ErrSecretStoreUnavailable, operation, code)
		}
		return platform.ErrSecretNotFound
	case errorAccessDenied:
		return fmt.Errorf("%w: %s: %v", platform.ErrSecretDenied, operation, code)
	case errorNoSuchLogonSession:
		return fmt.Errorf("%w: %s: %v", platform.ErrSecretStoreUnavailable, operation, code)
	}
	return fmt.Errorf("windows: credential %s: %w", operation, code)
}

// StoreSecret saves a generic credential in the Credential Manager of the
// current user, replacing one of the same service and account.
func StoreSecret(service, account string, secret []byte) error {
	target, err := secretTarget(service, account)
	if err != nil {
		return err
	}
	user, err := syscall.UTF16PtrFromString(account)
	if err != nil {
		return err
	}
	entry := credential{
		Type: credTypeGeneric, TargetName: target, UserName: user,
		CredentialBlobSize: uint32(len(secret)), CredentialBlob: &secret[0], Persist: credPersistLocalMachine,
	}
	ok, _, callErr := credWrite.Call(uintptr(unsafe.Pointer(&entry)), 0)
	if ok == 0 {
		return secretError("write", callErr.(syscall.Errno))
	}
	return nil
}

// LoadSecret reads the generic credential saved for the service and account.
func LoadSecret(service, account string) ([]byte, error) {
	target, err := secretTarget(service, account)
	if err != nil {
		return nil, err
	}
	var entry *credential
	ok, _, callErr := credRead.Call(uintptr(unsafe.Pointer(target)), credTypeGeneric, 0, uintptr(unsafe.Pointer(&entry)))
	if ok == 0 {
		return nil, secretError("read", callErr.(syscall.Errno))
	}
	defer credFree.Call(uintptr(unsafe.Pointer(entry)))
	if entry.CredentialBlobSize == 0 || entry.CredentialBlob == nil {
		return nil, errors.New("windows: the credential is empty")
	}
	return append([]byte(nil), unsafe.Slice(entry.CredentialBlob, int(entry.CredentialBlobSize))...), nil
}

// DeleteSecret removes the credential; one that is not there is not an error.
func DeleteSecret(service, account string) error {
	target, err := secretTarget(service, account)
	if err != nil {
		return err
	}
	ok, _, callErr := credDelete.Call(uintptr(unsafe.Pointer(target)), credTypeGeneric, 0)
	if ok == 0 {
		if err := secretError("delete", callErr.(syscall.Errno)); !errors.Is(err, platform.ErrSecretNotFound) {
			return err
		}
	}
	return nil
}
