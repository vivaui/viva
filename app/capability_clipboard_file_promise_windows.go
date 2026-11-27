//go:build windows

package app

// capabilityClipboardFilePromise reports that this build can put files on the
// clipboard whose content is produced when they are pasted: Windows keeps the
// OLE data object the promise is, and Explorer asks it for the bytes then.
const capabilityClipboardFilePromise = CapabilityClipboardFilePromise
