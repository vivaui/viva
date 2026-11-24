//go:build !android

package linux

import (
	"testing"
	"time"

	"github.com/vivaui/viva/internal/fcitx"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/text"
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

func TestFcitxPreeditClearKeepsSelectionAndHistory(test *testing.T) {
	for _, selection := range []struct {
		name       string
		rangeValue text.Range
	}{
		{"forward", text.Range{Start: 1, End: 2}},
		{"backward", text.Range{Start: 2, End: 1}},
	} {
		for _, ending := range []string{"cancel", "clear-before-commit", "commit-before-clear"} {
			test.Run(selection.name+"/"+ending, func(test *testing.T) {
				harness := newIMEHarness(test)
				harness.editor = text.NewEditor("abc")
				harness.editor.SetSelection(selection.rangeValue)
				harness.input.state = platform.EditorState(harness.editor)
				harness.focus(test)
				handler := harness.ime.fcitxHandler()
				assertSelection := func() {
					test.Helper()
					if harness.editor.Selection() != (text.Range{Start: 1, End: 2}) || harness.editor.Caret() != selection.rangeValue.End {
						test.Fatalf("selection = %+v, caret = %d", harness.editor.Selection(), harness.editor.Caret())
					}
				}
				handler.Preedit(fcitx.PreeditEvent{Text: "\u4f60", Visible: true, Cursor: 1})
				if harness.editor.String() != "a\u4f60c" || harness.editor.CommittedString() != "abc" || !harness.ime.composing() {
					test.Fatalf("preedit did not preserve the committed text: %+v", platform.EditorState(harness.editor))
				}
				if ending == "commit-before-clear" {
					handler.Commit("\u4f60\u597d")
				}
				handler.Preedit(fcitx.PreeditEvent{})
				if ending != "commit-before-clear" {
					if harness.editor.String() != "abc" || harness.ime.composing() || harness.editor.CanUndo() {
						test.Fatalf("clear changed the selection's text or history: %q", harness.editor.String())
					}
					assertSelection()
				}
				if ending == "cancel" {
					handler.Preedit(fcitx.PreeditEvent{})
					if harness.ime.composing() || harness.editor.CanUndo() || harness.editor.String() != "abc" {
						test.Fatal("second clear changed an already canceled editor")
					}
					assertSelection()
					return
				}
				if ending == "clear-before-commit" {
					handler.Commit("\u4f60\u597d")
				}
				if harness.editor.CommittedString() != "a\u4f60\u597dc" || harness.ime.composing() {
					test.Fatalf("commit = %q or still composing", harness.editor.CommittedString())
				}
				if !harness.editor.Undo() || harness.editor.String() != "abc" || harness.editor.CanUndo() {
					test.Fatalf("one Undo did not restore the original text: %q", harness.editor.String())
				}
				assertSelection()
				if !harness.editor.Redo() || harness.editor.String() != "a\u4f60\u597dc" {
					test.Fatalf("Redo = %q", harness.editor.String())
				}
			})
		}
	}
}
