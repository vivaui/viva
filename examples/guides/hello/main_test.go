package main

import (
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/flex"
)

// open runs the page in a headless window: the real painter, router and focus
// rules, driven by explicit calls.
func open(t *testing.T, m *model) *app.Driver {
	t.Helper()
	screen := flex.NewScreen(flex.ScreenConfig{Body: m.page})
	driver, err := app.NewDriver(screen.Config(app.Config{Width: 480, Height: 240}), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)
	return driver
}

// click presses the middle of the control a user would find by its name.
func click(t *testing.T, driver *app.Driver, name string) {
	t.Helper()
	for _, hit := range driver.Snapshot().Hits {
		if strings.HasPrefix(hit.Label, name) {
			b := hit.Bounds
			if err := driver.Click(b.X+b.Width/2, b.Y+b.Height/2); err != nil {
				t.Fatal(err)
			}
			return
		}
	}
	t.Fatalf("no control named %q on screen", name)
}

func shows(driver *app.Driver, want string) bool {
	for _, label := range driver.Snapshot().Labels {
		if strings.Contains(label.Value, want) {
			return true
		}
	}
	return false
}

func TestClickCounts(t *testing.T) {
	driver := open(t, &model{})
	if !shows(driver, "Clicked 0 times") {
		t.Fatal("the button should start at zero")
	}
	click(t, driver, "Clicked")
	if !shows(driver, "Clicked 1 times") {
		t.Fatal("the click did not reach the button")
	}
}
