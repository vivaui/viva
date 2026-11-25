# Presence-bound device keys

`devicekey` protects a small application key using a native, per-device
credential. It does not store a key in the ordinary credential store and then
ask for an unrelated authentication boolean.

Use `Enroll(ctx, Options{Service, Account, Reason}, key)` from a worker after the
user has authenticated with the application's primary password. Persist the
opaque result as a local device slot. `Unseal` releases the key through the
native operation. `Revoke` removes that device credential; the application also
removes its local blob. Keep a separate portable password-protected key slot
for recovery, migration, and a lost/reset device credential.

All calls honor an already-canceled context before native work. Pending Windows
operations use WebAuthn cancellation IDs; macOS invalidates the operation's
`LAContext`. There are no repaint timers or polling workers. Applications must
also reject stale asynchronous results when their own lock generation changes.

`Supported` and `Method` are noninteractive preflights. Enrollment is the final
capability check. A failed/canceled/unavailable native operation never switches
to an unprotected key file, Credential Manager, DPAPI, or an ordinary Keychain
item.

## Windows

The native Win32 WebAuthn API version must be at least 6 and an active foreground
window must belong to the calling process. Enrollment requests a platform-only,
user-verification-required credential with PRF enabled. Registration checks
that Windows confirms a resident credential with PRF enabled. A verified assertion
must actually return a 32-byte PRF result. That result wraps the application key
with AES-256-GCM; it is never persisted. Assertions must match the RP and exact
credential, use the internal transport, and contain user-presence and
user-verification flags. Backup-eligible or backed-up credentials are rejected
because this slot is device-local.

Some Windows/Hello configurations expose WebAuthn but do not provide this PRF
capability. They return `ErrUnavailable`, and the primary password remains the
application's unlock route. `Method` reports Windows Hello, without claiming
that any particular fingerprint reader or face sensor is configured.

Revocation first enumerates credentials for the slot's derived RP and confirms
its ID, rather than deleting an arbitrary ID from editable blob metadata.

References: [Win32 WebAuthn](https://learn.microsoft.com/en-us/windows/win32/webauthn/-webauthn-portal),
[Microsoft's versioned ABI](https://github.com/microsoft/webauthn/blob/master/webauthn.h),
[WebAuthn PRF](https://www.w3.org/TR/webauthn/#prf-extension).

## macOS

Keys use a Data Protection Keychain generic-password item in the signed app's
own application-ID access group, protected by
`SecAccessControl` user presence and `WhenUnlockedThisDeviceOnly`. Items do not
synchronize through iCloud. Copying the opaque ID to another device is not a
recovery mechanism. Each release uses a new `LAContext` with no Touch ID reuse
interval. Native presence may be Touch ID or the device-owner password.
`Method` reports Touch ID only when the actual biometric policy is available;
otherwise it reports the device password when that policy is available.

The host application's **main executable** needs a valid, provisioned signing
identity and an authorized application identifier/keychain access group. An
unsigned/ad-hoc command-line binary cannot supply this guarantee simply by
adding entitlement text. `Supported` checks the application identifier as well
as the native authentication policy; `Enroll` still verifies real Keychain
access before returning a usable slot.

`viva package --provision-profile /path/to/App.provisionprofile` (or the
`APPLE_PROVISION_PROFILE` path) optionally embeds a profile and merges its
authorized app identity with existing entitlements. Explicit shared Keychain
groups are preserved only when authorized by the profile. The profile must
authorize the exact bundle ID and actual signing certificate. Without a
profile, ordinary packaging stays unchanged and this protected-key facility
may be unavailable. Profiles and signing credentials belong in the release
environment, not committed source.

References: [Apple TN3137](https://developer.apple.com/documentation/technotes/tn3137-on-mac-keychains),
[keychain access groups](https://developer.apple.com/documentation/security/sharing-access-to-keychain-items-among-a-collection-of-apps).

## Verification scope

`go test ./app/devicekey` checks bounded/mismatched input, cancellation before
native access, WebAuthn ABI layouts, RP labels, fresh challenges, and refusal of
unverified or syncable authenticator data. Hardware/native acceptance additionally
requires enrollment, lock/unseal, cancellation, restart/cold unlock, and revoke
on a configured Windows Hello device or a provisioned signed macOS application.
Passing unit tests or detecting WebAuthn is not evidence that those interactive
hardware paths were exercised.
