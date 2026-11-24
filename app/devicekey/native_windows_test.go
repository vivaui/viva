package devicekey

import (
	"crypto/sha256"
	"encoding/json"
	"strings"
	"testing"
	"unsafe"
)

func TestNativeCapabilityProbeDoesNotRequestPresence(t *testing.T) {
	if waVersion.Find() != nil {
		t.Log("WebAuthn DLL not installed")
		return
	}
	version, _, _ := waVersion.Call()
	t.Logf("WebAuthn API version=%d; platform preflight=%v (PRF still requires enrollment)", version, nativeSupported())
}

func TestRelyingPartyUsesLegalDNSLabels(t *testing.T) {
	rp := relyingParty(Options{Service: "tests", Account: "slot"})
	for _, label := range strings.Split(rp, ".") {
		if len(label) == 0 || len(label) > 63 {
			t.Fatalf("invalid RP label length %d", len(label))
		}
	}
}

func TestRevocationCannotSelectAnotherRelyingParty(t *testing.T) {
	id := []byte("credential")
	rp := waRP{Version: 1, ID: wide("other.example")}
	credential := waCredentialDetails{Version: 1, Size: uint32(len(id)), ID: &id[0], RP: &rp}
	if matchesCredential(&credential, id, "devicekey.example") {
		t.Fatal("another RP's credential was selected for deletion")
	}
	rp.ID = wide("devicekey.example")
	if !matchesCredential(&credential, id, "devicekey.example") {
		t.Fatal("own credential was rejected")
	}
	if matchesCredential(&credential, []byte("forged"), "devicekey.example") {
		t.Fatal("another credential ID was selected")
	}
}

func TestWebAuthnVersionedStructureABI(t *testing.T) {
	if unsafe.Sizeof(uintptr(0)) != 8 {
		t.Skip("64-bit Windows SDK layout")
	}
	for _, value := range []struct {
		name      string
		got, want uintptr
	}{
		{"make options v6 size", unsafe.Sizeof(waMakeOptions{}), 104},
		{"make cancellation", unsafe.Offsetof(waMakeOptions{}.Cancellation), 64},
		{"make PRF", unsafe.Offsetof(waMakeOptions{}.PRF), 96},
		{"get options v6 size", unsafe.Sizeof(waGetOptions{}), 120},
		{"get salts", unsafe.Offsetof(waGetOptions{}.Salts), 104},
		{"attestation credential ID", unsafe.Offsetof(waAttestation{}.ID), 88},
		{"attestation PRF", unsafe.Offsetof(waAttestation{}.PRF), 128},
		{"assertion secret", unsafe.Offsetof(waAssertion{}.Secret), 112},
		{"assertion transport", unsafe.Offsetof(waAssertion{}.Transport), 120},
	} {
		if value.got != value.want {
			t.Errorf("%s = %d, want %d", value.name, value.got, value.want)
		}
	}
}

func TestAuthenticatorMustVerifyUserAndStayDeviceBound(t *testing.T) {
	rp := "devicekey.test"
	hash := sha256.Sum256([]byte(rp))
	data := make([]byte, 37)
	copy(data, hash[:])
	for _, flags := range []byte{0, 1, 4, 5 | 8, 5 | 16, 5} {
		data[32] = flags
		if got := verifiedData(&data[0], uint32(len(data)), rp); got != (flags == 5) {
			t.Errorf("flags %#x accepted = %v", flags, got)
		}
	}
	if verifiedData(&data[0], uint32(len(data)), "other.test") {
		t.Error("accepted another RP")
	}
}

func TestClientDataContainsFreshChallenge(t *testing.T) {
	first, second := clientJSON("webauthn.get", "devicekey.test"), clientJSON("webauthn.get", "devicekey.test")
	var data struct {
		Type, Challenge, Origin string
		CrossOrigin             bool
	}
	if err := json.Unmarshal(first, &data); err != nil {
		t.Fatal(err)
	}
	if data.Type != "webauthn.get" || data.Origin != "https://devicekey.test" || data.Challenge == "" || data.CrossOrigin {
		t.Fatalf("invalid client data: %+v", data)
	}
	if equal(first, second) {
		t.Error("challenge reused")
	}
}
