// Package devicekey seals small keys to this device with operating-system user
// presence. It is separate from app's ordinary credential store: a successful
// authentication boolean is never sufficient to recover a sealed key.
package devicekey

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

var (
	ErrUnavailable = errors.New("devicekey: presence-bound key protection is unavailable")
	ErrCanceled    = errors.New("devicekey: verification was canceled")
	ErrNotFound    = errors.New("devicekey: device credential was not found")
	ErrInvalidBlob = errors.New("devicekey: invalid or mismatched sealed key")
)

// MaxKeySize bounds the plaintext accepted by Enroll. These are keys or tokens,
// not documents. The caller owns and should clear all returned plaintext.
const MaxKeySize = 2048

// Options identifies an application's key slot. Service and Account are stable
// identifiers; Reason is the localized explanation shown by the operating
// system. Device credentials are local and must not be copied into a backup as
// a substitute for an application's portable password-protected key slot.
type Options struct{ Service, Account, Reason string }

// VerificationMethod names the native method currently available. Windows
// chooses among enrolled Hello methods; Touch ID is reported only after the
// native biometric policy succeeds, not merely because the OS is macOS.
type VerificationMethod uint8

const (
	VerificationUnavailable VerificationMethod = iota
	VerificationWindowsHello
	VerificationTouchID
	VerificationDevicePassword
)

func Method() VerificationMethod { return nativeMethod() }

type envelope struct {
	Version    int    `json:"version"`
	Platform   string `json:"platform"`
	Binding    []byte `json:"binding"`
	ID         []byte `json:"id"`
	Salt       []byte `json:"salt,omitempty"`
	Nonce      []byte `json:"nonce,omitempty"`
	Ciphertext []byte `json:"ciphertext,omitempty"`
}

// Supported is a noninteractive platform/device capability check. An
// authenticator may still refuse the required PRF or user-presence policy at
// enrollment; Enroll reports ErrUnavailable rather than weakening that policy.
func Supported() bool { return nativeSupported() }

// Enroll creates a new presence-bound device credential and seals key to it.
// It may display system UI and must be called from a worker goroutine. On
// Windows an active foreground window belonging to this process is required.
// The opaque returned blob is safe to persist; it contains no plaintext key or
// silent credential-store fallback. Existing enrollment is never overwritten.
func Enroll(ctx context.Context, options Options, key []byte) ([]byte, error) {
	if err := validate(ctx, options); err != nil {
		return nil, err
	}
	if len(key) == 0 || len(key) > MaxKeySize {
		return nil, fmt.Errorf("devicekey: key must contain 1 through %d bytes", MaxKeySize)
	}
	value, err := nativeEnroll(ctx, options, key)
	if err != nil {
		return nil, err
	}
	value.Version, value.Binding = 1, binding(options)
	blob, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	if err = ctx.Err(); err != nil {
		_ = nativeRevoke(context.Background(), options, value)
		return nil, err
	}
	return blob, nil
}

// Unseal asks the OS to release the key after user presence. It fails closed
// when verification is canceled, the device credential disappeared, or the blob
// came from another slot/device. It never treats a generic OS login as proof
// that this particular key was released.
func Unseal(ctx context.Context, options Options, blob []byte) ([]byte, error) {
	if err := validate(ctx, options); err != nil {
		return nil, err
	}
	value, err := parse(options, blob)
	if err != nil {
		return nil, err
	}
	key, err := nativeUnseal(ctx, options, value)
	if err != nil {
		clear(key)
		return nil, err
	}
	if err = ctx.Err(); err != nil {
		clear(key)
		return nil, err
	}
	if len(key) == 0 || len(key) > MaxKeySize {
		clear(key)
		return nil, ErrInvalidBlob
	}
	return key, nil
}

// Revoke removes the native device credential. Callers must also remove the
// opaque blob from their own storage. ErrNotFound means native removal is
// already complete. This does not revoke an application's password key slot.
func Revoke(ctx context.Context, options Options, blob []byte) error {
	if err := validate(ctx, options); err != nil {
		return err
	}
	value, err := parse(options, blob)
	if err != nil {
		return err
	}
	return nativeRevoke(ctx, options, value)
}

func validate(ctx context.Context, options Options) error {
	if ctx == nil {
		return errors.New("devicekey: a context is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(options.Service) > 256 || len(options.Account) > 256 || len(options.Reason) > 1024 ||
		strings.TrimSpace(options.Service) == "" || strings.TrimSpace(options.Account) == "" {
		return errors.New("devicekey: bounded, nonempty service and account identifiers are required")
	}
	for _, field := range []string{options.Service, options.Account, options.Reason} {
		if !utf8.ValidString(field) || strings.IndexByte(field, 0) >= 0 {
			return errors.New("devicekey: invalid native string")
		}
	}
	return nil
}

func binding(options Options) []byte {
	sum := sha256.Sum256([]byte("viva/devicekey/v1\x00" + options.Service + "\x00" + options.Account))
	return sum[:]
}

func parse(options Options, blob []byte) (envelope, error) {
	var value envelope
	if len(blob) == 0 || len(blob) > 16*1024 || json.Unmarshal(blob, &value) != nil || value.Version != 1 ||
		len(value.ID) == 0 || len(value.ID) > 4096 || !equal(value.Binding, binding(options)) {
		return envelope{}, ErrInvalidBlob
	}
	return value, nil
}

func equal(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	var difference byte
	for i := range a {
		difference |= a[i] ^ b[i]
	}
	return difference == 0
}
