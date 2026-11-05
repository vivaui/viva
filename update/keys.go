package update

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
)

// ParsePublicKey reads an Ed25519 public key from the text an application
// stores: standard base64 or hexadecimal, of the key's 32 raw bytes. It is
// how a key written in a configuration file ("public_key = \"...\"") becomes
// the value Config.PublicKeys holds.
func ParsePublicKey(text string) (ed25519.PublicKey, error) {
	raw, err := decodeKey(text, ed25519.PublicKeySize)
	if err != nil {
		return nil, fmt.Errorf("update: public key: %w", err)
	}
	return ed25519.PublicKey(raw), nil
}

// ParsePrivateKey reads an Ed25519 private key (the 64-byte form, seed and
// public key) from standard base64 or hexadecimal. It is for a signing tool
// such as `viva feed`, not for an application that only checks updates.
func ParsePrivateKey(text string) (ed25519.PrivateKey, error) {
	raw, err := decodeKey(text, ed25519.PrivateKeySize)
	if err != nil {
		return nil, fmt.Errorf("update: private key: %w", err)
	}
	return ed25519.PrivateKey(raw), nil
}

// Sign returns the standard-base64 Ed25519 signature of body, the text
// latest.json.sig holds. The signature covers exactly the bytes of the feed,
// so the file is signed as it is written and never re-encoded first.
func Sign(body []byte, key ed25519.PrivateKey) (string, error) {
	if len(key) != ed25519.PrivateKeySize {
		return "", fmt.Errorf("update: private key is %d bytes, want %d", len(key), ed25519.PrivateKeySize)
	}
	return base64.StdEncoding.EncodeToString(ed25519.Sign(key, body)), nil
}

func decodeKey(text string, size int) ([]byte, error) {
	text = strings.TrimSpace(text)
	if raw, err := base64.StdEncoding.DecodeString(text); err == nil && len(raw) == size {
		return raw, nil
	}
	if raw, err := base64.RawStdEncoding.DecodeString(text); err == nil && len(raw) == size {
		return raw, nil
	}
	if raw, err := hex.DecodeString(text); err == nil && len(raw) == size {
		return raw, nil
	}
	return nil, fmt.Errorf("must be base64 or hexadecimal of %d bytes", size)
}
