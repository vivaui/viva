//go:build android

package app

import (
	"os"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform/android"
)

// What the system gives an Android application grows here as its backend lands; the
// rest is answered in mobile_unsupported.go.

func nativeTextEngine() gfx.TextEngine { return android.TextEngine() }

func nativeClipboardText() (string, error) {
	text, ok := android.ClipboardText()
	if !ok {
		return "", nil
	}
	return text, nil
}

func nativeSetClipboardText(text string) error {
	if !android.SetClipboardText(text) {
		return ErrUnsupported
	}
	return nil
}

func nativeOpenURL(url string) error {
	if !android.OpenURL(url) {
		return ErrUnsupported
	}
	return nil
}

func nativePreferredLanguages() []string {
	if languages := android.PreferredLanguages(); len(languages) > 0 {
		return languages
	}
	return languagesFromEnvironment(os.Getenv)
}

func nativeReduceMotion() bool { return android.AnimationsOff() }

func nativeAppearance() Appearance {
	switch android.DarkTheme() {
	case 1:
		return AppearanceDark
	case 0:
		return AppearanceLight
	}
	return AppearanceUnknown
}

// The host tells the application when the theme changes (Activity.onConfigurationChanged),
// whether or not anyone watches.
func nativeWatchAppearance() {}
