//go:build darwin || windows || linux

package headless

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/vivaui/viva/app"
)

// expectUsage is what Expect accepts, as the error for a request it cannot read.
const expectUsage = `expect needs one of: text TEXT | no-text TEXT | exists SELECTOR | absent SELECTOR | ` +
	`count SELECTOR N | state SELECTOR FLAG... | focus SELECTOR|none | editor VALUE|none|contains TEXT | ` +
	`layout clean | named; optionally ending in: within MS`

// Expect checks one condition about the current frame and returns nil when it holds, or an error
// that says what was expected, what the frame has instead and, for a control, the control as the
// tree would show it. The arguments are those of the protocol's expect command:
//
//	text TEXT                  a painted text contains TEXT
//	no-text TEXT               no painted text contains TEXT
//	exists SELECTOR            a control the SELECTOR names can be pressed
//	absent SELECTOR            no control at all matches SELECTOR
//	count SELECTOR N           exactly N controls match SELECTOR, covered ones included
//	state SELECTOR FLAG...     the control has every FLAG: checked unchecked mixed enabled
//	                           disabled focused unfocused selected unselected expanded collapsed
//	                           readonly editable covered reachable, or value=TEXT (equal),
//	                           value~=TEXT (contains), label=TEXT (equal)
//	focus SELECTOR | none      the SELECTOR's control has the keyboard; none: nothing has
//	editor VALUE | none        the field being edited holds exactly VALUE; none: no field is edited
//	editor contains TEXT       the field being edited holds TEXT
//	layout clean               CheckLayout finds nothing
//	named                      every interactive control has an accessible name
//
// A selector is a name, "id:IDENTITY", or "role:NAME" (see Locate). Ending the arguments in
// "within MS" makes the check wait up to MS real milliseconds for it to hold, applying the
// application's background work meanwhile, as waitfor does.
func (session *Session) Expect(arguments ...string) error {
	var timeout time.Duration
	if count := len(arguments); count >= 3 && arguments[count-2] == "within" {
		milliseconds, err := strconv.Atoi(arguments[count-1])
		if err != nil || milliseconds < 0 {
			return fmt.Errorf("expect: %q is not a number of milliseconds", arguments[count-1])
		}
		timeout, arguments = time.Duration(milliseconds)*time.Millisecond, arguments[:count-2]
	}
	if len(arguments) == 0 {
		return fmt.Errorf("%s", expectUsage)
	}
	if timeout == 0 {
		return session.evaluate(arguments)
	}
	var last error
	err := session.WaitFor(func(app.Snapshot) bool {
		last = session.evaluate(arguments)
		return last == nil
	}, timeout)
	if err != nil && last != nil {
		return fmt.Errorf("%w (waited %v)", last, timeout)
	}
	return err
}

// evaluate checks the condition against the frame as it is now.
func (session *Session) evaluate(arguments []string) error {
	snapshot := session.Snapshot()
	kind, rest := arguments[0], arguments[1:]
	need := func(count int) error {
		if len(rest) != count {
			return fmt.Errorf("expect %s takes %d argument(s); %s", kind, count, expectUsage)
		}
		return nil
	}
	switch kind {
	case "text", "no-text":
		if err := need(1); err != nil {
			return err
		}
		has := snapshotHasText(snapshot, rest[0])
		if kind == "text" && !has {
			return fmt.Errorf("expected text %q on screen; screen has %q", rest[0], summarizeTexts(snapshot))
		}
		if kind == "no-text" && has {
			return fmt.Errorf("expected no text %q on screen; screen has %q", rest[0], summarizeTexts(snapshot))
		}
		return nil
	case "exists":
		if err := need(1); err != nil {
			return err
		}
		if _, err := session.Locate(rest[0]); err != nil {
			return fmt.Errorf("expected a control %q: %w", rest[0], err)
		}
		return nil
	case "absent":
		if err := need(1); err != nil {
			return err
		}
		s, err := parseSelector(rest[0])
		if err != nil {
			return err
		}
		if found := session.matching(snapshot, s); len(found) > 0 {
			return fmt.Errorf("expected no control %q, found %d: %s", rest[0], len(found), found[len(found)-1].describe(snapshot.Focused))
		}
		return nil
	case "count":
		if err := need(2); err != nil {
			return err
		}
		want, err := strconv.Atoi(rest[1])
		if err != nil {
			return fmt.Errorf("expect count: %q is not a number", rest[1])
		}
		s, err := parseSelector(rest[0])
		if err != nil {
			return err
		}
		if found := session.matching(snapshot, s); len(found) != want {
			return fmt.Errorf("expected %d controls %q, found %d", want, rest[0], len(found))
		}
		return nil
	case "state":
		if len(rest) < 2 {
			return fmt.Errorf("expect state takes a selector and at least one flag; %s", expectUsage)
		}
		return session.expectState(snapshot, rest[0], rest[1:])
	case "focus":
		if err := need(1); err != nil {
			return err
		}
		if rest[0] == "none" {
			if !snapshot.Focused.IsZero() {
				return fmt.Errorf("expected nothing focused; focus is on %s", snapshot.Tree.Text(snapshot.Focused))
			}
			return nil
		}
		return session.expectState(snapshot, rest[0], []string{"focused"})
	case "editor":
		editor := snapshot.Editor
		switch {
		case len(rest) == 2 && rest[0] == "contains":
			if editor == nil || !strings.Contains(editor.Value, rest[1]) {
				return fmt.Errorf("expected the edited field to contain %q; %s", rest[1], describeEditor(editor))
			}
		case len(rest) == 1 && rest[0] == "none":
			if editor != nil {
				return fmt.Errorf("expected no field being edited; %s", describeEditor(editor))
			}
		case len(rest) == 1:
			if editor == nil || editor.Value != rest[0] {
				return fmt.Errorf("expected the edited field to hold %q; %s", rest[0], describeEditor(editor))
			}
		default:
			return fmt.Errorf("expect editor takes VALUE, none, or contains TEXT")
		}
		return nil
	case "layout":
		if len(rest) != 1 || rest[0] != "clean" {
			return fmt.Errorf("expect layout takes clean")
		}
		problems := CheckLayout(snapshot)
		if len(problems) == 0 {
			return nil
		}
		var lines []string
		for index, problem := range problems {
			if index == 5 {
				lines = append(lines, fmt.Sprintf("and %d more", len(problems)-index))
				break
			}
			lines = append(lines, fmt.Sprintf("%s %q at x=%g y=%g (%s)", problem.Kind, problem.Text, problem.Bounds.X, problem.Bounds.Y, problem.Detail))
		}
		return fmt.Errorf("expected a clean layout, found %d problems: %s", len(problems), strings.Join(lines, "; "))
	case "named":
		if len(rest) != 0 {
			return fmt.Errorf("expect named takes no arguments")
		}
		var unnamed []string
		for _, c := range session.controls(snapshot) {
			if c.interactive() && strings.TrimSpace(c.label) == "" {
				unnamed = append(unnamed, roleName(c.hit.Role)+" "+c.id)
			}
		}
		if len(unnamed) > 0 {
			return fmt.Errorf("expected every interactive control to have an accessible name; %d have none: %s", len(unnamed), strings.Join(unnamed, ", "))
		}
		return nil
	}
	return fmt.Errorf("unknown expectation %q; %s", kind, expectUsage)
}

// expectState checks flags of the control a selector names. A control that is covered is still
// checked, so that "covered" can be expected; the others need one that can be pressed to exist.
func (session *Session) expectState(snapshot app.Snapshot, text string, flags []string) error {
	s, err := parseSelector(text)
	if err != nil {
		return err
	}
	found := session.matching(snapshot, s)
	if len(found) == 0 {
		return fmt.Errorf("expected a control %q: %w", text, session.noControl(text))
	}
	// The front-most that can be pressed, else the front-most there is.
	target := found[len(found)-1]
	for index := len(found) - 1; index >= 0; index-- {
		if !found[index].covered {
			target = found[index]
			break
		}
	}
	for _, flag := range flags {
		if ok, known := session.holds(snapshot, target, flag); !known {
			return fmt.Errorf("unknown state %q", flag)
		} else if !ok {
			return fmt.Errorf("expected %q to be %s; it is: %s", text, flag, target.describe(snapshot.Focused))
		}
	}
	return nil
}

// holds evaluates one flag of expect state on a control.
func (session *Session) holds(snapshot app.Snapshot, c control, flag string) (ok, known bool) {
	focused := !c.hit.ID.IsZero() && c.hit.ID == snapshot.Focused
	switch flag {
	case "checked":
		return c.hit.Checked, true
	case "unchecked":
		return !c.hit.Checked && !c.hit.Mixed, true
	case "mixed":
		return c.hit.Mixed, true
	case "enabled":
		return !c.hit.Disabled, true
	case "disabled":
		return c.hit.Disabled, true
	case "focused":
		return focused, true
	case "unfocused":
		return !focused, true
	case "selected":
		return c.hit.Selected, true
	case "unselected":
		return !c.hit.Selected, true
	case "expanded":
		return c.hit.Expanded, true
	case "collapsed":
		return !c.hit.Expanded, true
	case "readonly":
		return c.hit.ReadOnly, true
	case "editable":
		return !c.hit.ReadOnly && !c.hit.Disabled, true
	case "covered":
		return c.covered, true
	case "reachable":
		return !c.covered, true
	}
	switch {
	case strings.HasPrefix(flag, "value~="):
		return strings.Contains(c.hit.Value, strings.TrimPrefix(flag, "value~=")), true
	case strings.HasPrefix(flag, "value="):
		return c.hit.Value == strings.TrimPrefix(flag, "value="), true
	case strings.HasPrefix(flag, "label="):
		return c.label == strings.TrimPrefix(flag, "label="), true
	}
	return false, false
}

// describeEditor says what the edited field holds, for a failed expectation.
func describeEditor(editor *app.Editor) string {
	if editor == nil {
		return "no field is being edited"
	}
	value := editor.Value
	if editor.Secure {
		value = strings.Repeat("•", utf8.RuneCountInString(value))
	}
	return fmt.Sprintf("it holds %q", value)
}
