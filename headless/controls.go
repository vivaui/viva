//go:build darwin || windows || linux

package headless

import (
	"fmt"
	"strings"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

// roleName is the protocol's name for role; a region that declared none is a plain "region".
func roleName(role ui.Role) string { return role.String() }

// roleByName is the role a selector names.
func roleByName(name string) (ui.Role, bool) { return ui.RoleNamed(name) }

// control is one thing the application declared on the frame, as the protocol sees it: its
// accessible name settled, its identity written out, and whether a press at its center reaches it.
type control struct {
	hit ui.Hit
	// label is the accessible name: the hit's own, or that of a label declared for it.
	label string
	// id is the identity as the protocol writes it ("home/save", "#3").
	id string
	// covered is set when something in front of the control takes a press at its center.
	covered bool
}

// controls lists what the frame declares, back to front: every interactive region and every
// container region (a heading, a status area), without the opaque blocks, window-drag areas and
// label helpers that are no controls. A label declared for a control names it.
func (session *Session) controls(snapshot app.Snapshot) []control {
	names := make(map[ui.ID]string)
	for _, hit := range snapshot.Hits {
		if hit.IsLabel && !hit.ID.IsZero() {
			if _, taken := names[hit.ID]; !taken {
				names[hit.ID] = hit.Label
			}
		}
	}
	var found []control
	for _, hit := range snapshot.Hits {
		if hit.Block || hit.MovesWindow || hit.IsLabel || hit.ID.IsZero() && !hit.Container {
			continue
		}
		c := control{hit: hit, label: hit.Label, id: snapshot.Tree.Text(hit.ID)}
		if name, ok := names[hit.ID]; ok {
			c.label = name
		}
		// A press never reaches a disabled control, whatever is in front of it, so whether
		// something covers it is not told by where a press goes; it is not called covered.
		if !hit.Container && !hit.Disabled {
			c.covered = !session.reachable(hit)
		}
		found = append(found, c)
	}
	return found
}

// flags are the states of a control worth reading at a glance.
func (c control) flags(focused ui.ID) []string {
	var flags []string
	add := func(on bool, name string) {
		if on {
			flags = append(flags, name)
		}
	}
	add(!c.hit.ID.IsZero() && c.hit.ID == focused, "focused")
	add(c.hit.Disabled, "disabled")
	add(c.hit.Checked, "checked")
	add(c.hit.Mixed, "mixed")
	add(c.hit.Selected, "selected")
	add(c.hit.Expandable && c.hit.Expanded, "expanded")
	add(c.hit.Expandable && !c.hit.Expanded, "collapsed")
	add(c.hit.ReadOnly, "readonly")
	add(c.hit.Secure, "secure")
	add(c.hit.Multiline, "multiline")
	add(c.hit.Focusable && !c.hit.Disabled, "focusable")
	add(c.covered, "covered")
	return flags
}

// describe is the control as one line of the tree.
func (c control) describe(focused ui.ID) string {
	var line strings.Builder
	fmt.Fprintf(&line, "role=%s id=%s label=%q", roleName(c.hit.Role), c.id, c.label)
	if c.hit.Value != "" && !c.hit.Secure {
		fmt.Fprintf(&line, " value=%q", c.hit.Value)
	}
	if c.hit.Hint != "" {
		fmt.Fprintf(&line, " hint=%q", c.hit.Hint)
	}
	if c.hit.Level != 0 {
		fmt.Fprintf(&line, " level=%d", c.hit.Level)
	}
	if c.hit.SetSize != 0 {
		fmt.Fprintf(&line, " position=%d/%d", c.hit.PosInSet, c.hit.SetSize)
	}
	if c.hit.Range.Set {
		fmt.Fprintf(&line, " range=%g in %g..%g", c.hit.Range.Now, c.hit.Range.Min, c.hit.Range.Max)
	}
	if flags := c.flags(focused); len(flags) > 0 {
		fmt.Fprintf(&line, " state=%s", strings.Join(flags, ","))
	}
	fmt.Fprintf(&line, " x=%g y=%g w=%g h=%g", c.hit.Bounds.X, c.hit.Bounds.Y, c.hit.Bounds.Width, c.hit.Bounds.Height)
	return line.String()
}

// interactive reports whether the control is something a user operates, as against a container
// region that only structures the frame.
func (c control) interactive() bool {
	return !c.hit.Container && c.hit.Role != ui.RoleGroup && c.hit.Role != ui.RoleSeparator &&
		(c.hit.Role != 0 || c.hit.Focusable)
}

// Tree returns the controls of the current frame, one line each, back to front: role, identity,
// accessible name, value and state (focused, disabled, checked, covered ...), and where it is.
// A filter keeps the lines whose role, identity, name or value contain it, ignoring case. It is
// what an agent reads to learn what a frame can do, and what to name in a selector.
func (session *Session) Tree(filter string) []string {
	snapshot := session.Snapshot()
	filter = strings.ToLower(filter)
	var lines []string
	for _, c := range session.controls(snapshot) {
		line := c.describe(snapshot.Focused)
		if filter == "" || strings.Contains(strings.ToLower(roleName(c.hit.Role)+" "+c.id+" "+c.label+" "+c.hit.Value), filter) {
			lines = append(lines, line)
		}
	}
	return lines
}

// selector says which control a command means. A bare argument is the control's accessible name:
// the exact name first, then any name containing it, ignoring case. "id:home/save" is its
// identity, in full or from any "/" on ("id:save"); "id:#3" an item. "button:Save" is the name
// restricted to a role, and "checkbox:" any checkbox.
type selector struct {
	text  string
	id    string
	role  ui.Role
	label string
	// plain is a bare name, which the session resolves as it always has.
	plain bool
	// restrictsRole and byID say which of role and id apply.
	restrictsRole, byID bool
}

func parseSelector(text string) (selector, error) {
	if text == "" {
		return selector{}, fmt.Errorf("an empty selector")
	}
	s := selector{text: text, plain: true, label: text}
	if id, ok := strings.CutPrefix(text, "id:"); ok {
		if id == "" {
			return selector{}, fmt.Errorf("selector %q names no identity", text)
		}
		return selector{text: text, id: id, byID: true}, nil
	}
	if name, label, ok := strings.Cut(text, ":"); ok {
		if role, known := roleByName(name); known {
			return selector{text: text, role: role, restrictsRole: true, label: label}, nil
		}
	}
	return s, nil
}

// matches reports whether c is what the selector names; exact asks for the name to be equal and
// not only contain the selector's.
func (s selector) matches(c control, exact bool) bool {
	switch {
	case s.byID:
		return c.id == s.id || strings.HasSuffix(c.id, "/"+s.id)
	case s.restrictsRole && c.hit.Role != s.role:
		return false
	case s.label == "":
		return true
	case exact:
		return c.label == s.label
	}
	return strings.Contains(strings.ToLower(c.label), strings.ToLower(s.label))
}

// matching returns every control the selector names, back to front, covered ones included.
func (session *Session) matching(snapshot app.Snapshot, s selector) []control {
	var found []control
	for _, c := range session.controls(snapshot) {
		if s.matches(c, false) {
			found = append(found, c)
		}
	}
	return found
}

// Locate returns the front-most reachable control a selector names (see selector for the forms).
// A bare name is resolved exactly as Hit does.
func (session *Session) Locate(text string) (ui.Hit, error) {
	s, err := parseSelector(text)
	if err != nil {
		return ui.Hit{}, err
	}
	if s.plain {
		if hit, ok := session.Hit(text); ok {
			return hit, nil
		}
		return ui.Hit{}, session.noControl(text)
	}
	snapshot := session.Snapshot()
	all := session.controls(snapshot)
	for _, exact := range []bool{true, false} {
		for index := len(all) - 1; index >= 0; index-- {
			if s.matches(all[index], exact) && !all[index].covered && !all[index].hit.Container {
				return all[index].hit, nil
			}
		}
		if s.byID || s.label == "" {
			break
		}
	}
	return ui.Hit{}, session.noControl(text)
}

// noControl is the error for a selector nothing answers to, with what the frame does offer.
func (session *Session) noControl(text string) error {
	snapshot := session.Snapshot()
	var offered []string
	for _, c := range session.controls(snapshot) {
		if !c.interactive() || c.covered {
			continue
		}
		name := c.id
		if c.label != "" {
			name = roleName(c.hit.Role) + ":" + c.label
		}
		offered = append(offered, name)
		if len(offered) == 12 {
			break
		}
	}
	if len(offered) == 0 {
		return fmt.Errorf("no control matches %q; the frame declares no interactive control", text)
	}
	return fmt.Errorf("no control matches %q; controls: %s", text, strings.Join(offered, " | "))
}
