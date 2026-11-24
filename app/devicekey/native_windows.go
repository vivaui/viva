package devicekey

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// The ABI definitions below are the versioned prefixes in Microsoft's
// webauthn.h. We require API version 6, request MakeCredential v6/GetAssertion
// v6, and only read the fields their documented output versions provide.
// https://github.com/microsoft/webauthn/blob/master/webauthn.h
type waCredentials struct {
	Count uint32
	Items *waCredential
}
type waCredential struct {
	Version, Size uint32
	ID            *byte
	Type          *uint16
}
type waExtensions struct {
	Count uint32
	Items *waExtension
}
type waExtension struct {
	Name  *uint16
	Size  uint32
	Value unsafe.Pointer
}
type waRP struct {
	Version        uint32
	ID, Name, Icon *uint16
}
type waUser struct {
	Version, Size           uint32
	ID                      *byte
	Name, Icon, DisplayName *uint16
}
type waClient struct {
	Version, Size uint32
	JSON          *byte
	Algorithm     *uint16
}
type waParameter struct {
	Version   uint32
	Type      *uint16
	Algorithm int32
}
type waParameters struct {
	Count uint32
	Items *waParameter
}
type waMakeOptions struct {
	Version, Timeout                 uint32
	Credentials                      waCredentials
	Extensions                       waExtensions
	Attachment                       uint32
	Resident                         int32
	Verification, Attestation, Flags uint32
	Cancellation                     *windows.GUID
	Exclude                          unsafe.Pointer
	Enterprise, LargeBlob            uint32
	PreferResident, Private, PRF     int32
}
type waGetOptions struct {
	Version, Timeout                  uint32
	Credentials                       waCredentials
	Extensions                        waExtensions
	Attachment, Verification, Flags   uint32
	AppID                             *uint16
	UsedAppID                         *int32
	Cancellation                      *windows.GUID
	Allow                             unsafe.Pointer
	LargeBlobOperation, LargeBlobSize uint32
	LargeBlob                         *byte
	Salts                             *waSaltValues
	Private                           int32
}
type waSalt struct {
	FirstSize  uint32
	First      *byte
	SecondSize uint32
	Second     *byte
}
type waSaltValues struct {
	Global      *waSalt
	Count       uint32
	Credentials unsafe.Pointer
}
type waAttestation struct {
	Version                              uint32
	Format                               *uint16
	AuthSize                             uint32
	Auth                                 *byte
	CBORSize                             uint32
	CBOR                                 *byte
	DecodeType                           uint32
	Decoded                              unsafe.Pointer
	ObjectSize                           uint32
	Object                               *byte
	IDSize                               uint32
	ID                                   *byte
	Extensions                           waExtensions
	Transport                            uint32
	Enterprise, LargeBlob, Resident, PRF int32
}
type waAssertion struct {
	Version, AuthSize uint32
	Auth              *byte
	SignatureSize     uint32
	Signature         *byte
	Credential        waCredential
	UserSize          uint32
	User              *byte
	Extensions        waExtensions
	LargeBlobSize     uint32
	LargeBlob         *byte
	LargeBlobStatus   uint32
	Secret            *waSalt
	Transport         uint32
}

type waListOptions struct {
	Version uint32
	RP      *uint16
	Private int32
}
type waCredentialDetails struct {
	Version, Size uint32
	ID            *byte
	RP            *waRP
	User          *waUser
	Removable     int32
}
type waCredentialList struct {
	Count uint32
	Items **waCredentialDetails
}

var (
	webAuthn          = windows.NewLazySystemDLL("webauthn.dll")
	waVersion         = webAuthn.NewProc("WebAuthNGetApiVersionNumber")
	waAvailable       = webAuthn.NewProc("WebAuthNIsUserVerifyingPlatformAuthenticatorAvailable")
	waMake            = webAuthn.NewProc("WebAuthNAuthenticatorMakeCredential")
	waGet             = webAuthn.NewProc("WebAuthNAuthenticatorGetAssertion")
	waFreeAttestation = webAuthn.NewProc("WebAuthNFreeCredentialAttestation")
	waFreeAssertion   = webAuthn.NewProc("WebAuthNFreeAssertion")
	waCancellation    = webAuthn.NewProc("WebAuthNGetCancellationId")
	waCancel          = webAuthn.NewProc("WebAuthNCancelCurrentOperation")
	waDelete          = webAuthn.NewProc("WebAuthNDeletePlatformCredential")
	waList            = webAuthn.NewProc("WebAuthNGetPlatformCredentialList")
	waFreeList        = webAuthn.NewProc("WebAuthNFreePlatformCredentialList")
	deviceUser32      = windows.NewLazySystemDLL("user32.dll")
	deviceForeground  = deviceUser32.NewProc("GetForegroundWindow")
	deviceWindowPID   = deviceUser32.NewProc("GetWindowThreadProcessId")
	devicePrompts     = make(chan struct{}, 1)
)

func nativeSupported() bool {
	for _, proc := range []*windows.LazyProc{waVersion, waAvailable, waMake, waGet, waFreeAttestation, waFreeAssertion, waCancellation, waCancel, waDelete, waList, waFreeList} {
		if proc.Find() != nil {
			return false
		}
	}
	version, _, _ := waVersion.Call()
	if version < 6 {
		return false
	}
	var available int32
	status, _, _ := waAvailable.Call(uintptr(unsafe.Pointer(&available)))
	return uint32(status) == 0 && available != 0
}

func nativeMethod() VerificationMethod {
	if nativeSupported() {
		return VerificationWindowsHello
	}
	return VerificationUnavailable
}

func nativeEnroll(ctx context.Context, options Options, key []byte) (result envelope, err error) {
	if !nativeSupported() {
		return result, ErrUnavailable
	}
	release, err := prompt(ctx)
	if err != nil {
		return result, err
	}
	defer release()
	owner, err := foregroundWindow()
	if err != nil {
		return result, err
	}
	rpID := relyingParty(options)
	name := options.Reason
	if name == "" {
		name = options.Service
	}
	rp := waRP{Version: 1, ID: wide(rpID), Name: wide(name)}
	userID := make([]byte, 32)
	rand.Read(userID)
	user := waUser{Version: 1, Size: uint32(len(userID)), ID: &userID[0], Name: wide(options.Account), DisplayName: wide(options.Account)}
	clientData := clientJSON("webauthn.create", rpID)
	client := waClient{Version: 1, Size: uint32(len(clientData)), JSON: &clientData[0], Algorithm: wide("SHA-256")}
	parameter := waParameter{Version: 1, Type: wide("public-key"), Algorithm: -7}
	parameters := waParameters{Count: 1, Items: &parameter}
	var value *waAttestation
	err = webOperation(ctx, func(cancellation *windows.GUID) uintptr {
		settings := waMakeOptions{Version: 6, Timeout: timeout(ctx), Attachment: 1,
			Resident: 1, Verification: 1, Attestation: 1, Cancellation: cancellation, PRF: 1}
		status, _, _ := waMake.Call(owner, uintptr(unsafe.Pointer(&rp)), uintptr(unsafe.Pointer(&user)), uintptr(unsafe.Pointer(&parameters)),
			uintptr(unsafe.Pointer(&client)), uintptr(unsafe.Pointer(&settings)), uintptr(unsafe.Pointer(&value)))
		runtime.KeepAlive(settings)
		return status
	})
	runtime.KeepAlive(rp)
	runtime.KeepAlive(user)
	runtime.KeepAlive(parameters)
	runtime.KeepAlive(client)
	if value != nil {
		defer waFreeAttestation.Call(uintptr(unsafe.Pointer(value)))
	}
	if err != nil {
		if value != nil && value.ID != nil && value.IDSize > 0 && value.IDSize <= 4096 {
			_ = deleteCredential(append([]byte(nil), unsafe.Slice(value.ID, value.IDSize)...))
		}
		return result, err
	}
	if value == nil || value.IDSize == 0 || value.IDSize > 4096 || value.ID == nil {
		return result, ErrUnavailable
	}
	result = envelope{Platform: "windows-webauthn-prf", ID: append([]byte(nil), unsafe.Slice(value.ID, value.IDSize)...), Binding: binding(options)}
	// Syncable/backup-eligible credentials are unsuitable for a device-only slot.
	if value.Version < 3 || value.Transport != 0x10 || !verifiedData(value.Auth, value.AuthSize, rpID) {
		_ = deleteCredential(result.ID)
		return envelope{}, fmt.Errorf("%w: platform authenticator did not provide a local user-verified PRF credential", ErrUnavailable)
	}
	result.Salt = make([]byte, 32)
	rand.Read(result.Salt)
	secret, err := getSecret(ctx, owner, options, result)
	if err != nil {
		_ = deleteCredential(result.ID)
		return envelope{}, err
	}
	defer clear(secret)
	block, err := aes.NewCipher(secret)
	if err != nil {
		_ = deleteCredential(result.ID)
		return envelope{}, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		_ = deleteCredential(result.ID)
		return envelope{}, err
	}
	result.Nonce = make([]byte, aead.NonceSize())
	rand.Read(result.Nonce)
	result.Ciphertext = aead.Seal(nil, result.Nonce, key, append(binding(options), result.ID...))
	return result, nil
}

func nativeUnseal(ctx context.Context, options Options, value envelope) ([]byte, error) {
	if value.Platform != "windows-webauthn-prf" || len(value.Salt) != 32 || len(value.Nonce) != 12 || len(value.Ciphertext) < 17 || len(value.Ciphertext) > MaxKeySize+16 {
		return nil, ErrInvalidBlob
	}
	if !nativeSupported() {
		return nil, ErrUnavailable
	}
	release, err := prompt(ctx)
	if err != nil {
		return nil, err
	}
	defer release()
	owner, err := foregroundWindow()
	if err != nil {
		return nil, err
	}
	secret, err := getSecret(ctx, owner, options, value)
	if err != nil {
		return nil, err
	}
	defer clear(secret)
	block, err := aes.NewCipher(secret)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	key, err := aead.Open(nil, value.Nonce, value.Ciphertext, append(binding(options), value.ID...))
	if err != nil {
		return nil, ErrInvalidBlob
	}
	return key, nil
}

func nativeRevoke(ctx context.Context, options Options, value envelope) error {
	if value.Platform != "windows-webauthn-prf" {
		return ErrInvalidBlob
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if waDelete.Find() != nil || waList.Find() != nil || waFreeList.Find() != nil {
		return ErrUnavailable
	}
	// Metadata on disk is editable. Confirm this ID belongs to the slot's RP
	// before deleting it, so a substituted ID cannot revoke another app's key.
	settings := waListOptions{Version: 1, RP: wide(relyingParty(options))}
	var list *waCredentialList
	status, _, _ := waList.Call(uintptr(unsafe.Pointer(&settings)), uintptr(unsafe.Pointer(&list)))
	runtime.KeepAlive(settings)
	if list != nil {
		defer waFreeList.Call(uintptr(unsafe.Pointer(list)))
	}
	if err := webError(status); err != nil {
		return err
	}
	if list == nil || list.Count == 0 || list.Items == nil {
		return ErrNotFound
	}
	if list.Count > 65536 {
		return ErrUnavailable
	}
	found := false
	for _, item := range unsafe.Slice(list.Items, list.Count) {
		if matchesCredential(item, value.ID, relyingParty(options)) {
			found = true
			break
		}
	}
	if !found {
		return ErrNotFound
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return deleteCredential(value.ID)
}

func matchesCredential(item *waCredentialDetails, id []byte, rpID string) bool {
	return item != nil && item.Version >= 1 && item.ID != nil && item.Size == uint32(len(id)) &&
		item.RP != nil && item.RP.ID != nil && windows.UTF16PtrToString(item.RP.ID) == rpID && equal(unsafe.Slice(item.ID, item.Size), id)
}

func getSecret(ctx context.Context, owner uintptr, options Options, value envelope) ([]byte, error) {
	rpID := relyingParty(options)
	data := clientJSON("webauthn.get", rpID)
	client := waClient{Version: 1, Size: uint32(len(data)), JSON: &data[0], Algorithm: wide("SHA-256")}
	credential := waCredential{Version: 1, Size: uint32(len(value.ID)), ID: &value.ID[0], Type: wide("public-key")}
	salt := waSalt{FirstSize: uint32(len(value.Salt)), First: &value.Salt[0]}
	salts := waSaltValues{Global: &salt}
	var assertion *waAssertion
	err := webOperation(ctx, func(cancellation *windows.GUID) uintptr {
		settings := waGetOptions{Version: 6, Timeout: timeout(ctx), Credentials: waCredentials{Count: 1, Items: &credential}, Attachment: 1, Verification: 1, Cancellation: cancellation, Salts: &salts}
		status, _, _ := waGet.Call(owner, uintptr(unsafe.Pointer(wide(rpID))), uintptr(unsafe.Pointer(&client)), uintptr(unsafe.Pointer(&settings)), uintptr(unsafe.Pointer(&assertion)))
		runtime.KeepAlive(settings)
		return status
	})
	runtime.KeepAlive(client)
	runtime.KeepAlive(credential)
	runtime.KeepAlive(salts)
	runtime.KeepAlive(value)
	if assertion != nil {
		defer func() {
			if assertion.Version >= 3 && assertion.Secret != nil && assertion.Secret.First != nil && assertion.Secret.FirstSize == 32 {
				clear(unsafe.Slice(assertion.Secret.First, 32))
			}
			waFreeAssertion.Call(uintptr(unsafe.Pointer(assertion)))
		}()
	}
	if err != nil {
		return nil, err
	}
	if assertion == nil || assertion.Version < 3 || assertion.Secret == nil || assertion.Secret.FirstSize != 32 || assertion.Secret.First == nil ||
		!verifiedData(assertion.Auth, assertion.AuthSize, rpID) || assertion.Credential.Size != uint32(len(value.ID)) || assertion.Credential.ID == nil ||
		!equal(unsafe.Slice(assertion.Credential.ID, assertion.Credential.Size), value.ID) {
		return nil, ErrUnavailable
	}
	if assertion.Version >= 4 && assertion.Transport != 0x10 {
		return nil, ErrUnavailable
	}
	output := unsafe.Slice(assertion.Secret.First, 32)
	secret := append([]byte(nil), output...)
	clear(output)
	return secret, nil
}

func verifiedData(pointer *byte, size uint32, rpID string) bool {
	if pointer == nil || size < 37 || size > 64*1024 {
		return false
	}
	data := unsafe.Slice(pointer, size)
	hash := sha256.Sum256([]byte(rpID))
	return equal(data[:32], hash[:]) && data[32]&5 == 5 && data[32]&0x18 == 0
}

func prompt(ctx context.Context) (func(), error) {
	select {
	case devicePrompts <- struct{}{}:
		return func() { <-devicePrompts }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func foregroundWindow() (uintptr, error) {
	owner, _, _ := deviceForeground.Call()
	var process uint32
	if owner != 0 {
		deviceWindowPID.Call(owner, uintptr(unsafe.Pointer(&process)))
	}
	if owner == 0 || process != windows.GetCurrentProcessId() {
		return 0, fmt.Errorf("%w: this process needs an active foreground window", ErrUnavailable)
	}
	return owner, nil
}

func webOperation(ctx context.Context, call func(*windows.GUID) uintptr) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	var id windows.GUID
	status, _, _ := waCancellation.Call(uintptr(unsafe.Pointer(&id)))
	if uint32(status) != 0 {
		return webError(status)
	}
	finished := make(chan struct{})
	stop := context.AfterFunc(ctx, func() { waCancel.Call(uintptr(unsafe.Pointer(&id))); close(finished) })
	defer func() {
		if !stop() {
			<-finished
		}
		runtime.KeepAlive(id)
	}()
	if err := ctx.Err(); err != nil {
		return err
	}
	status = call(&id)
	if err := ctx.Err(); err != nil {
		return err
	}
	return webError(status)
}

func deleteCredential(id []byte) error {
	status, _, _ := waDelete.Call(uintptr(len(id)), uintptr(unsafe.Pointer(&id[0])))
	runtime.KeepAlive(id)
	return webError(status)
}

func webError(status uintptr) error {
	switch uint32(status) {
	case 0:
		return nil
	case 0x800704c7, 0x80090036:
		return ErrCanceled
	case 0x80070490, 0x80090011:
		return ErrNotFound
	default:
		return fmt.Errorf("%w: WebAuthn HRESULT 0x%08x", ErrUnavailable, uint32(status))
	}
}

func relyingParty(options Options) string {
	return strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(binding(options))) + ".devicekey.viva.local"
}
func wide(value string) *uint16 { return windows.StringToUTF16Ptr(value) }
func timeout(ctx context.Context) uint32 {
	duration := 2 * time.Minute
	if deadline, ok := ctx.Deadline(); ok && time.Until(deadline) < duration {
		duration = time.Until(deadline)
	}
	return uint32(max(1, duration.Milliseconds()))
}
func clientJSON(kind, rp string) []byte {
	challenge := make([]byte, 32)
	rand.Read(challenge)
	data, _ := json.Marshal(struct {
		Type        string `json:"type"`
		Challenge   string `json:"challenge"`
		Origin      string `json:"origin"`
		CrossOrigin bool   `json:"crossOrigin"`
	}{kind, base64.RawURLEncoding.EncodeToString(challenge), "https://" + rp, false})
	return data
}
