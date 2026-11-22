//go:build ios

package app

import (
	"os"

	"github.com/vivaui/viva/gfx"
	darwinplatform "github.com/vivaui/viva/internal/platform/darwin"
)

// What the system gives an iPhone application grows here as its backend lands; the
// rest is answered in mobile_unsupported.go.

func nativeTextEngine() gfx.TextEngine { return darwinplatform.SharedTextEngine() }

func nativeClipboardText() (string, error) {
	text, _ := darwinplatform.ClipboardText()
	return text, nil
}

func nativeSetClipboardText(text string) error {
	darwinplatform.SetClipboardText(text)
	return nil
}

func nativeOpenURL(url string) error {
	if !darwinplatform.OpenURL(url) {
		return ErrUnsupported
	}
	return nil
}

func nativePreferredLanguages() []string {
	if languages := darwinplatform.PreferredLanguages(); len(languages) > 0 {
		return languages
	}
	return languagesFromEnvironment(os.Getenv)
}

func nativeReduceMotion() bool { return darwinplatform.ReduceMotion() }

func nativeAppearance() Appearance {
	switch darwinplatform.DarkTheme() {
	case 1:
		return AppearanceDark
	case 0:
		return AppearanceLight
	}
	return AppearanceUnknown
}

// The view controller tells the application when the look changes, whether or not
// anyone watches.
func nativeWatchAppearance() {}
