//go:build darwin || windows || linux

package headless

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
)

// expectApp has a button, a checkbox with no name of its own, a field, a heading, and a button
// that a panel in front of it covers.
type expectApp struct {
	tree    gfx.Tree
	auto    bool
	saved   int
	status  string
	invalid app.Invalidator
}

func (a *expectApp) session(t *testing.T) *Session {
	t.Helper()
	home := a.tree.Feature("home")
	save, auto, name, behind := a.tree.ID(home, "save"), a.tree.ID(home, "auto"), a.tree.ID(home, "name"), a.tree.ID(home, "behind")
	config := app.Config{Title: "expect", Width: 320, Height: 200, Tree: &a.tree, Invalidator: &a.invalid}
	config.OnWake = func() {}
	config.Paint = func(canvas *gfx.Canvas, size gfx.Size) {
		canvas.HitWithOptions(save, gfx.Rect{X: 10, Y: 10, Width: 80, Height: 30}, "Save", gfx.HitOptions{Role: gfx.RoleButton})
		canvas.HitWithOptions(auto, gfx.Rect{X: 10, Y: 50, Width: 20, Height: 20}, "", gfx.HitOptions{Role: gfx.RoleCheckbox, Checked: a.auto})
		canvas.HitWithOptions(name, gfx.Rect{X: 10, Y: 80, Width: 200, Height: 30}, "Host", gfx.HitOptions{Role: gfx.RoleTextField, Value: "example.com"})
		canvas.HitWithOptions(behind, gfx.Rect{X: 220, Y: 10, Width: 80, Height: 30}, "Behind", gfx.HitOptions{Role: gfx.RoleButton})
		canvas.Block(gfx.Rect{X: 220, Y: 10, Width: 80, Height: 30})
		canvas.HitWithOptions(a.tree.ID(home, "off"), gfx.Rect{X: 100, Y: 10, Width: 80, Height: 30}, "Off", gfx.HitOptions{Role: gfx.RoleButton, Disabled: true})
		canvas.HitWithOptions(a.tree.ID(home, "panel"), gfx.Rect{X: 215, Y: 5, Width: 90, Height: 40}, "Panel", gfx.HitOptions{Role: gfx.RoleGroup})
		canvas.Label(a.status, gfx.Rect{X: 10, Y: 150, Width: 200, Height: 20}, gfx.TextStyle{Size: 13, Color: gfx.Color{Alpha: 1}})
		if canvas.Clicked(auto) {
			a.auto = !a.auto
		}
		if canvas.Clicked(save) {
			a.saved++
		}
	}
	session, err := Open(config, nil, Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(session.Close)
	return session
}

func TestTreeShowsRoleIdentityNameAndStateOfEveryControl(t *testing.T) {
	a := &expectApp{}
	session := a.session(t)
	tree := strings.Join(session.Tree(""), "\n")
	for _, want := range []string{
		`role=button id=home/save label="Save"`,
		`role=checkbox id=home/auto label=""`,
		`role=textfield id=home/name label="Host" value="example.com"`,
		`role=button id=home/behind label="Behind" state=focusable,covered`,
	} {
		if !strings.Contains(tree, want) {
			t.Errorf("the tree lacks %q:\n%s", want, tree)
		}
	}
	if filtered := session.Tree("checkbox"); len(filtered) != 1 || !strings.Contains(filtered[0], "home/auto") {
		t.Errorf("filtering by role gave %q", filtered)
	}
}

func TestSelectorsReachControlsThatHaveNoName(t *testing.T) {
	a := &expectApp{}
	session := a.session(t)
	for _, selector := range []string{"id:home/auto", "id:auto", "checkbox:"} {
		hit, err := session.Locate(selector)
		if err != nil || hit.ID != a.tree.ID(a.tree.Feature("home"), "auto") {
			t.Errorf("%s: %+v, %v", selector, hit, err)
		}
	}
	if err := session.ClickLabel("id:auto"); err != nil {
		t.Fatal(err)
	}
	if !a.auto {
		t.Fatal("a click by identity did not reach the checkbox")
	}
	if err := session.Expect("state", "checkbox:", "checked"); err != nil {
		t.Fatal(err)
	}
	if _, err := session.Locate("button:Behind"); err == nil || !strings.Contains(err.Error(), "controls:") {
		t.Fatalf("a covered control was located: %v", err)
	}
	if _, err := session.Locate("button:Missing"); err == nil || !strings.Contains(err.Error(), "button:Save") {
		t.Fatalf("a miss should say what the frame offers: %v", err)
	}
}

func TestExpectHoldsOrSaysWhatTheFrameHasInstead(t *testing.T) {
	a := &expectApp{status: "Connected"}
	session := a.session(t)
	for _, test := range []struct {
		expect []string
		fail   string // a part of the error, or "" when the check holds
	}{
		{expect: []string{"text", "Connected"}},
		{expect: []string{"text", "Failed"}, fail: `screen has "Connected"`},
		{expect: []string{"no-text", "Failed"}},
		{expect: []string{"no-text", "Connected"}, fail: "expected no text"},
		{expect: []string{"exists", "button:Save"}},
		{expect: []string{"exists", "button:Behind"}, fail: "controls:"},
		{expect: []string{"absent", "button:Nothing"}},
		{expect: []string{"absent", "button:Save"}, fail: "expected no control"},
		{expect: []string{"count", "button:", "3"}},
		{expect: []string{"count", "button:", "4"}, fail: "found 3"},
		{expect: []string{"state", "id:off", "disabled", "reachable"}},
		{expect: []string{"state", "id:auto", "unchecked", "enabled", "unfocused"}},
		{expect: []string{"state", "id:auto", "checked"}, fail: "to be checked; it is: role=checkbox id=home/auto"},
		{expect: []string{"state", "id:behind", "covered"}},
		{expect: []string{"state", "id:behind", "reachable"}, fail: "to be reachable"},
		{expect: []string{"state", "textfield:Host", "value=example.com", "value~=ample"}},
		{expect: []string{"state", "textfield:Host", "value=other"}, fail: "to be value=other"},
		{expect: []string{"state", "id:save", "flying"}, fail: `unknown state "flying"`},
		{expect: []string{"focus", "none"}},
		{expect: []string{"focus", "id:save"}, fail: "to be focused"},
		{expect: []string{"editor", "none"}},
		{expect: []string{"editor", "x"}, fail: "no field is being edited"},
		{expect: []string{"named"}, fail: "have none: checkbox home/auto"},
		{expect: []string{"layout", "clean"}},
		{expect: []string{"nonsense"}, fail: "unknown expectation"},
		{expect: []string{"count", "button:"}, fail: "takes 2 argument"},
		{expect: nil, fail: "expect needs one of"},
	} {
		err := session.Expect(test.expect...)
		switch {
		case test.fail == "" && err != nil:
			t.Errorf("expect %q: %v", test.expect, err)
		case test.fail != "" && (err == nil || !strings.Contains(err.Error(), test.fail)):
			t.Errorf("expect %q: error %v, want one containing %q", test.expect, err, test.fail)
		}
	}
}

func TestExpectWithinWaitsForWhatBackgroundWorkBrings(t *testing.T) {
	a := &expectApp{}
	arrived := make(chan string, 1)
	config := app.Config{Title: "bg", Width: 200, Height: 80, Invalidator: &a.invalid}
	config.OnWake = func() {
		select {
		case a.status = <-arrived:
		default:
		}
	}
	config.Paint = func(canvas *gfx.Canvas, size gfx.Size) {
		canvas.Label(a.status, gfx.Rect{Width: 100, Height: 20}, gfx.TextStyle{Size: 13, Color: gfx.Color{Alpha: 1}})
	}
	background, err := Open(config, nil, Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(background.Close)
	go func() {
		time.Sleep(30 * time.Millisecond)
		arrived <- "Connected"
		a.invalid.Invalidate()
	}()
	if err := background.Expect("text", "Connected"); err == nil {
		t.Fatal("the text was there before the background work ran")
	}
	if err := background.Expect("text", "Connected", "within", "3000"); err != nil {
		t.Fatal(err)
	}
	err = background.Expect("text", "Never", "within", "60")
	if err == nil || !strings.Contains(err.Error(), "waited 60ms") {
		t.Fatalf("a wait that ran out should say so: %v", err)
	}
}

func TestServeSpeaksTreeAndExpect(t *testing.T) {
	a := &expectApp{}
	session := a.session(t)
	script := strings.Join([]string{
		`tree save`,
		`click id:home/auto`,
		`expect state id:auto checked`,
		`expect state id:auto unchecked`,
		`find button:Save`,
		`quit`,
	}, "\n")
	var out bytes.Buffer
	if err := Serve(session, strings.NewReader(script), &out); err != nil {
		t.Fatal(err)
	}
	replies := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(replies) != 7 || !strings.HasPrefix(replies[0], `role=button id=home/save label="Save"`) || replies[1] != "ok" ||
		replies[2] != "ok" || replies[3] != "ok" || !strings.HasPrefix(replies[4], `error expected "id:auto" to be unchecked; it is: role=checkbox`) ||
		!strings.HasPrefix(replies[5], `ok id=home/save`) || replies[6] != "ok bye" {
		t.Fatalf("replies:\n%s", out.String())
	}
}
