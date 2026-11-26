//go:build darwin || windows || linux

package inputnumber_test

import (
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/headless"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/inputnumber"
)

// changes counts the paints that reported a change, as a page reads Changed.
var changes int

// open runs a page holding one number field over *value and a button to move the
// focus to, headlessly.
func open(t *testing.T, n *inputnumber.InputNumber, value *float64) *headless.Session {
	t.Helper()
	changes = 0
	page := ui.NewPage(ui.PageConfig{View: func(c ui.C) {
		ui.Column(c).Pad(16).Gap(8).Children(func() {
			n.View(c, "Port", value).Width(200)
			if n.Changed() {
				changes++
			}
			ui.Button(c, "Elsewhere")
		})
	}})
	session, err := headless.Open(ui.Configure(page, app.Config{Title: "Number", Width: 360, Height: 200}), nil, headless.Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(session.Close)
	return session
}

func shown(t *testing.T, session *headless.Session) string {
	t.Helper()
	node, ok := session.AccessByName("Port")
	if !ok {
		t.Fatal("the field is not in the accessibility tree")
	}
	return node.Value
}

// The buttons step the number, and the one at an end of the range is dimmed and
// cannot be pressed.
func TestButtonsStepWithinTheRange(t *testing.T) {
	value := 8.0
	session := open(t, inputnumber.New().Min(0).Max(10).Step(2), &value)
	if err := session.ClickLabel("Increase"); err != nil {
		t.Fatal(err)
	}
	if value != 10 {
		t.Fatalf("value = %v after Increase from 8 by 2, want 10", value)
	}
	if err := session.ClickLabel("Increase"); err == nil {
		t.Fatal("Increase could be pressed at the top of the range")
	}
	if err := session.ClickLabel("Decrease"); err != nil {
		t.Fatal(err)
	}
	if value != 8 {
		t.Fatalf("value = %v after Decrease from 10, want 8", value)
	}
	if got := shown(t, session); got != "8" {
		t.Fatalf("the field shows %q, want 8", got)
	}
	if changes == 0 {
		t.Fatal("Changed was never reported for a step")
	}
}

// A number is typed in and taken on Enter; text that is not a number is put back
// to the number the field held.
func TestEnterTakesTheTypedNumber(t *testing.T) {
	value := 22.0
	session := open(t, inputnumber.New().Min(1).Max(100), &value)
	if err := session.ClickLabel("Port"); err != nil {
		t.Fatal(err)
	}
	session.Keys("cmd+a")
	if err := session.Type("42"); err != nil {
		t.Fatal(err)
	}
	if err := session.Keys("enter"); err != nil {
		t.Fatal(err)
	}
	if value != 42 {
		t.Fatalf("value = %v after typing 42 and Enter, want 42", value)
	}
	session.Keys("cmd+a")
	if err := session.Type("abc"); err != nil {
		t.Fatal(err)
	}
	if err := session.Keys("enter"); err != nil {
		t.Fatal(err)
	}
	if value != 42 || shown(t, session) != "42" {
		t.Fatalf("value = %v, shown %q after text that is not a number, want 42 put back", value, shown(t, session))
	}
}

// A number outside the range is brought in when the field loses the focus.
func TestLeavingTheFieldTakesTheNumberInRange(t *testing.T) {
	value := 22.0
	session := open(t, inputnumber.New().Min(1).Max(100), &value)
	if err := session.ClickLabel("Port"); err != nil {
		t.Fatal(err)
	}
	session.Keys("cmd+a")
	if err := session.Type("999"); err != nil {
		t.Fatal(err)
	}
	if err := session.ClickLabel("Elsewhere"); err != nil {
		t.Fatal(err)
	}
	if value != 100 || shown(t, session) != "100" {
		t.Fatalf("value = %v, shown %q after leaving the field with 999, want 100", value, shown(t, session))
	}
}

// Up and Down step the number while the field has the focus, PageUp by ten.
func TestArrowKeysStepTheNumber(t *testing.T) {
	value := 5.0
	session := open(t, inputnumber.New(), &value)
	if err := session.ClickLabel("Port"); err != nil {
		t.Fatal(err)
	}
	for _, step := range []struct {
		key  string
		want float64
	}{{"up", 6}, {"down", 5}, {"pageup", 15}, {"pagedown", 5}} {
		if err := session.Keys(step.key); err != nil {
			t.Fatal(err)
		}
		if value != step.want {
			t.Fatalf("value = %v after %q, want %v", value, step.key, step.want)
		}
	}
}

// The decimals follow the step, so a tenth steps without drifting.
func TestPrecisionFollowsTheStep(t *testing.T) {
	value := 0.5
	session := open(t, inputnumber.New().Step(0.1), &value)
	for range 3 {
		if err := session.ClickLabel("Increase"); err != nil {
			t.Fatal(err)
		}
	}
	if value != 0.8 {
		t.Fatalf("value = %v after three steps of 0.1 from 0.5, want exactly 0.8", value)
	}
	if got := shown(t, session); got != "0.8" {
		t.Fatalf("the field shows %q, want 0.8", got)
	}
}

// A number the page changes shows in the field while it is not being edited.
func TestTheFieldFollowsThePage(t *testing.T) {
	value := 1.0
	session := open(t, inputnumber.New(), &value)
	value = 7
	if err := session.Frame(); err != nil {
		t.Fatal(err)
	}
	if got := shown(t, session); got != "7" {
		t.Fatalf("the field shows %q after the page set 7", got)
	}
}

// A read only field takes no input and offers no step.
func TestReadOnlyTakesNoInput(t *testing.T) {
	value := 3.0
	session := open(t, inputnumber.New().ReadOnly(), &value)
	session.ClickLabel("Increase")
	if err := session.ClickLabel("Port"); err != nil {
		t.Fatal(err)
	}
	session.Keys("up")
	if value != 3 {
		t.Fatalf("value = %v after input on a read only field, want 3", value)
	}
}
