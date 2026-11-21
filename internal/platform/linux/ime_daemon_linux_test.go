package linux

import (
	"testing"
	"time"
)

// stubDaemon stands in for a framework's connection in a selection test.
type stubDaemon struct{ name string }

func (stubDaemon) FD() int                                    { return -1 }
func (stubDaemon) Dispatch() error                            { return nil }
func (stubDaemon) Err() error                                 { return nil }
func (stubDaemon) Deadline() (time.Time, bool)                { return time.Time{}, false }
func (stubDaemon) Expire()                                    {}
func (stubDaemon) open(*linuxIME) (inputMethodContext, error) { return nil, nil }

func TestInputMethodFrameworkFollowsXModifiers(t *testing.T) {
	for value, want := range map[string]string{
		"@im=fcitx":             "fcitx",
		"@im=ibus":              "ibus",
		" @im=fcitx ":           "fcitx",
		"@im=FCITX":             "fcitx",
		"@im=fcitx;@im=ibus":    "fcitx",
		"@im=xic":               "xic",
		"":                      "",
		"@im":                   "",
		"@im=fcitx, @im=ibus":   "fcitx",
		"gtk-im-context-simple": "",
	} {
		getenv := func(name string) string {
			if name == "XMODIFIERS" {
				return value
			}
			return ""
		}
		if got := inputMethodFramework(getenv); got != want {
			t.Errorf("XMODIFIERS=%q framework=%q, want %q", value, got, want)
		}
	}
}

func TestDialPrefersTheFrameworkTheSessionNames(t *testing.T) {
	ibus := func() inputMethodDaemon { return stubDaemon{"ibus"} }
	fcitx := func() inputMethodDaemon { return stubDaemon{"fcitx"} }
	cases := []struct {
		framework string
		want      string
	}{
		{"fcitx", "fcitx"},
		{"ibus", "ibus"},
		{"", "ibus"},
		{"xic", "ibus"},
	}
	for _, test := range cases {
		daemon := dialPreferred(test.framework, ibus, fcitx)
		if daemon == nil || daemon.(stubDaemon).name != test.want {
			t.Errorf("framework=%q chose %v, want %s", test.framework, daemon, test.want)
		}
	}
}

func TestDialFallsBackWhenTheNamedFrameworkIsAbsent(t *testing.T) {
	missing := func() inputMethodDaemon { return nil }
	present := func(name string) func() inputMethodDaemon {
		return func() inputMethodDaemon { return stubDaemon{name} }
	}
	if daemon := dialPreferred("fcitx", present("ibus"), missing); daemon == nil || daemon.(stubDaemon).name != "ibus" {
		t.Fatalf("a named Fcitx that is not running did not fall back to IBus: %v", daemon)
	}
	if daemon := dialPreferred("ibus", missing, present("fcitx")); daemon == nil || daemon.(stubDaemon).name != "fcitx" {
		t.Fatalf("a named IBus that is not running did not fall back to Fcitx: %v", daemon)
	}
	if daemon := dialPreferred("", missing, missing); daemon != nil {
		t.Fatalf("no daemon was reported as %v", daemon)
	}
}
