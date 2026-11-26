//go:build !((darwin && !ios) || windows)

package app

// nativeTextMenus reports that this platform has no menu of its own to show for
// a text field outside a toolkit, so the page draws one.
func nativeTextMenus() bool { return false }
