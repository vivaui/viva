package gfx

// OneTabStop makes the focusable hits declared since from one stop of Tab, as the
// buttons of a toolbar or the tabs of a list are natively: Tab reaches the one that
// has the focus, else last (the stop of the paint before), else the first, and the
// others stay focusable by program (an arrow key can move the focus to them) but
// Tab passes over them. It appends the identities of the members, in order, to
// members, and returns them with the stop; no member is the zero ID.
//
// Members are the hits of the layer and focus scope the stretch started in that
// take the focus by Tab, other than those that take the arrows themselves (a text
// field, a combobox, a slider, a terminal): those keep their own Tab stops.
func (canvas *Canvas) OneTabStop(from Mark, last ID, members []ID) (stop ID, out []ID) {
	first := len(members)
	if int(from.hits) > len(canvas.hits) {
		return ID{}, members
	}
	for index := int(from.hits); index < len(canvas.hits); index++ {
		hit := &canvas.hits[index]
		if !hit.Focusable || hit.SkipTab || hit.Disabled || hit.Block || hit.Backdrop || hit.ID.IsZero() ||
			hit.Layer != from.layer || hit.Scope != from.scope || takesArrows(hit.Role) {
			continue
		}
		members = append(members, hit.ID)
	}
	group := members[first:]
	if len(group) == 0 {
		return ID{}, members
	}
	focused := canvas.Focused()
	for _, id := range group {
		if id == focused {
			stop = id
		}
	}
	if stop.IsZero() {
		for _, id := range group {
			if id == last {
				stop = id
			}
		}
	}
	if stop.IsZero() {
		stop = group[0]
	}
	for index := int(from.hits); index < len(canvas.hits); index++ {
		hit := &canvas.hits[index]
		if hit.ID != stop && containsID(group, hit.ID) {
			hit.SkipTab = true
		}
	}
	return stop, members
}

// takesArrows says a hit of this role uses the arrow keys for itself.
func takesArrows(role Role) bool {
	switch role {
	case RoleTextField, RoleCombobox, RoleSlider, RoleTerminal:
		return true
	}
	return false
}

func containsID(ids []ID, id ID) bool {
	for _, candidate := range ids {
		if candidate == id {
			return true
		}
	}
	return false
}
