//go:build darwin || windows || linux

package tag_test

import (
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/tag"
)

// open runs a page of one tag and reports what the page read from it.
func open(t *testing.T, spec tag.Spec, closed, toggled *int) *headless.Session {
	t.Helper()
	page := ui.NewPage(ui.PageConfig{View: func(c ui.C) {
		ui.Column(c).Pad(16).Children(func() {
			result := tag.View(c, "production", spec)
			if result.Closed {
				*closed++
			}
			if result.Toggled {
				*toggled++
			}
		})
	}})
	session, err := headless.Open(ui.Configure(page, app.Config{Title: "Tag", Width: 320, Height: 120}), nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(session.Close)
	return session
}

// A press on the close button is reported as Closed once, and not as a toggle.
func TestCloseButtonIsReported(t *testing.T) {
	var closed, toggled int
	session := open(t, tag.Spec{Closable: true}, &closed, &toggled)
	if err := session.ClickLabel("Remove"); err != nil {
		t.Fatal(err)
	}
	if closed != 1 || toggled != 0 {
		t.Fatalf("closed %d, toggled %d after a press on the close button, want 1, 0", closed, toggled)
	}
}

// A checkable tag is a checkbox that reports each press, and shows its state to
// assistive technology.
func TestCheckableTagToggles(t *testing.T) {
	var closed, toggled int
	session := open(t, tag.Spec{Checkable: true, Checked: true}, &closed, &toggled)
	node, ok := session.AccessByName("production")
	if !ok || node.Role != gfx.RoleCheckbox || !node.Checked {
		t.Fatalf("access node = %+v, %v; want a checked checkbox", node, ok)
	}
	if err := session.ClickLabel("production"); err != nil {
		t.Fatal(err)
	}
	if toggled != 1 {
		t.Fatalf("toggled %d after one press, want 1", toggled)
	}
}

// A plain tag takes no input: no control to press.
func TestPlainTagOffersNothingToPress(t *testing.T) {
	var closed, toggled int
	session := open(t, tag.Spec{}, &closed, &toggled)
	for _, name := range []string{"production", "Remove"} {
		if _, err := session.Locate(name); err == nil {
			t.Fatalf("a plain tag offers %q as a control", name)
		}
	}
}
