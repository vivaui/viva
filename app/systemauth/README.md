# System authentication

`Method()` queries the operating system without showing a prompt. `Verify(ctx, reason)`
returns success after the OS verifies the current user. Call it from a worker
goroutine with an active application window. Callers should also discard results
that belong to a closed window or an earlier application lock. This API is for
an application UI lock. It handles no encryption or key release.

- Windows uses UserConsentVerifier, with the HWND-owned desktop method where
  available. The static WinRT method is tried only if the interop interface is
  absent. Some older Windows desktop contexts cannot present that static
  prompt. A missing active window, ordinary cancellation, and unavailable
  authentication are reported separately.
- macOS uses LocalAuthentication and a fresh device-owner authentication
  context. The OS can offer Touch ID or the system password. No Keychain
  entitlement is needed for this prompt.
- On other platforms, the method is unavailable.

References: [Windows desktop verification](https://learn.microsoft.com/windows/win32/api/userconsentverifierinterop/nf-userconsentverifierinterop-iuserconsentverifierinterop-requestverificationforwindowasync),
[Apple LocalAuthentication](https://developer.apple.com/documentation/localauthentication/lacontext/evaluatepolicy(_:localizedreason:reply:)).
