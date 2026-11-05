package update

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"testing"
)

func TestParseKeysRoundTrip(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{
		base64.StdEncoding.EncodeToString(public),
		base64.RawStdEncoding.EncodeToString(public),
		hex.EncodeToString(public),
		"  " + base64.StdEncoding.EncodeToString(public) + "\n",
	} {
		parsed, err := ParsePublicKey(text)
		if err != nil {
			t.Fatalf("ParsePublicKey(%q): %v", text, err)
		}
		if !parsed.Equal(public) {
			t.Errorf("ParsePublicKey(%q) returned another key", text)
		}
	}
	for _, text := range []string{
		base64.StdEncoding.EncodeToString(private),
		hex.EncodeToString(private),
	} {
		parsed, err := ParsePrivateKey(text)
		if err != nil {
			t.Fatalf("ParsePrivateKey: %v", err)
		}
		if !parsed.Equal(private) {
			t.Errorf("ParsePrivateKey returned another key")
		}
	}
}

func TestParseKeysRefuseTheWrongLength(t *testing.T) {
	if _, err := ParsePublicKey("not a key"); err == nil {
		t.Error("ParsePublicKey accepted text")
	}
	if _, err := ParsePublicKey(base64.StdEncoding.EncodeToString(make([]byte, 31))); err == nil {
		t.Error("ParsePublicKey accepted 31 bytes")
	}
	if _, err := ParsePrivateKey(base64.StdEncoding.EncodeToString(make([]byte, 32))); err == nil {
		t.Error("ParsePrivateKey accepted 32 bytes")
	}
}

func TestSignVerifies(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	body := []byte(`{"schema":1}`)
	signature, err := Sign(body, private)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decodeSignature([]byte(signature))
	if err != nil {
		t.Fatal(err)
	}
	if !ed25519.Verify(public, body, decoded) {
		t.Error("the signature does not verify")
	}
	if ed25519.Verify(public, append(body, ' '), decoded) {
		t.Error("the signature verifies another body")
	}
}
