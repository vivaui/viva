//go:build darwin || windows || linux

package steps_test

import (
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/steps"
)

// A clickable step is a button for the keyboard too: Tab reaches it and Enter
// chooses it.
func TestStepsChooseWithTheKeyboard(t *testing.T) {
	current := 0
	page := ui.NewPage(ui.PageConfig{View: func(c ui.C) {
		steps.View(c, "Checkout", &current, steps.Spec{Steps: items, Clickable: true}).FillWidth()
	}})
	session, err := headless.Open(ui.Configure(page, app.Config{Title: "Steps", Width: 480, Height: 200}), nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(session.Close)
	session.Keys("tab")
	session.Keys("tab")
	session.Keys("enter")
	if current != 1 {
		t.Fatalf("current = %d after Tab, Tab and Enter, want the second step", current)
	}
}
