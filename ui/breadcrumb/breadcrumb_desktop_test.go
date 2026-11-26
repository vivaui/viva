//go:build darwin || windows || linux

package breadcrumb_test

import (
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/breadcrumb"
)

var chosen []int

func open(t *testing.T, b *breadcrumb.Breadcrumb, spec breadcrumb.Spec) *headless.Session {
	t.Helper()
	chosen = nil
	page := ui.NewPage(ui.PageConfig{View: func(c ui.C) {
		ui.Column(c).Pad(16).Children(func() {
			b.View(c, "Location", spec)
			if i, ok := b.Clicked(); ok {
				chosen = append(chosen, i)
			}
		})
	}})
	session, err := headless.Open(ui.Configure(page, app.Config{Title: "Path", Width: 640, Height: 200}), nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(session.Close)
	return session
}

// Every place but the last is a link that reports its index; the last is the
// current place and is not a control.
func TestLinksReportTheirIndexAndTheCurrentPlaceIsNotOne(t *testing.T) {
	session := open(t, breadcrumb.New(), breadcrumb.Spec{Items: places})
	if err := session.ClickLabel("Hosts"); err != nil {
		t.Fatal(err)
	}
	if len(chosen) != 1 || chosen[0] != 1 {
		t.Fatalf("chosen = %v after a click on Hosts, want [1]", chosen)
	}
	if _, err := session.Locate("Logs"); err == nil {
		t.Fatal("the current place is a control")
	}
	if err := session.Expect("text", "Logs"); err != nil {
		t.Fatal(err)
	}
}

// A disabled place is shown and offers nothing to press.
func TestDisabledPlaceTakesNoPress(t *testing.T) {
	items := []breadcrumb.Item{{Label: "Home"}, {Label: "Archive", Disabled: true}, {Label: "2025"}}
	session := open(t, breadcrumb.New(), breadcrumb.Spec{Items: items})
	if _, err := session.Locate("Archive"); err == nil {
		t.Fatal("a disabled place is a control")
	}
	if err := session.Expect("text", "Archive"); err != nil {
		t.Fatal(err)
	}
}

// With Max the middle places collapse behind an ellipsis; choosing one in its
// menu reports that place's index in the whole path.
func TestCollapsedPlacesAreChosenFromTheMenu(t *testing.T) {
	session := open(t, breadcrumb.New(), breadcrumb.Spec{Items: places, Max: 3})
	for _, hidden := range []string{"Hosts", "Production"} {
		if _, err := session.Locate(hidden); err == nil {
			t.Fatalf("%q is shown though it collapsed", hidden)
		}
	}
	if err := session.ClickLabel("More"); err != nil {
		t.Fatal(err)
	}
	if err := session.ClickLabel("Production"); err != nil {
		t.Fatal(err)
	}
	if len(chosen) != 1 || chosen[0] != 2 {
		t.Fatalf("chosen = %v after choosing Production in the menu, want [2]", chosen)
	}
	for _, shown := range []string{"Home", "build-01", "Logs"} {
		if err := session.Expect("text", shown); err != nil {
			t.Fatal(err)
		}
	}
}
