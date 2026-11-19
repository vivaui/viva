package window

import "testing"

// Appearance names the look the system is set to.
func TestAppearanceString(t *testing.T) {
	for appearance, want := range map[Appearance]string{
		AppearanceUnknown: "unknown",
		AppearanceLight:   "light",
		AppearanceDark:    "dark",
	} {
		if got := appearance.String(); got != want {
			t.Errorf("%d.String() = %q, want %q", appearance, got, want)
		}
	}
}
