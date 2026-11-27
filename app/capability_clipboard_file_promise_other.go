//go:build !windows

package app

// capabilityClipboardFilePromise is absent where the clipboard holds paths
// rather than a data object read on paste; see
// capability_clipboard_file_promise_windows.go.
const capabilityClipboardFilePromise Capability = 0
