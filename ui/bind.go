package ui

// A bound control writes what the user did into a value the application owns, as a
// Screen declares its controls: a Checkbox or Switch flips a bool, a Choice, Tabs or
// Select sets an int. The value is written in the paint that delivers the click, and
// only there: a Checkbox or Switch writes the first time the control is asked (Clicked,
// Changed) or else when the page ends, so reading the value before asking sees the old
// one and a Disabled given after the control was declared is still obeyed. A Choice,
// Tabs or Select writes where it is declared. The event pass is not shown: the repaint
// that follows paints the new value.

// bindDecl is one bound control of the frame.
type bindDecl struct {
	flag    *bool
	index   *int
	field   *Field
	settled bool
	changed bool
}

// bindBool makes the Checkbox or Switch b flip value when it is clicked.
func (f *frame) bindBool(b Box, value *bool) {
	f.extra(b.i).bind = int32(len(f.binds)) + 1
	f.settleFn = (*frame).settleAt
	f.binds = append(f.binds, bindDecl{flag: value})
}

// bindIndex makes the Choice, Tabs or Select b set value to the option chosen.
func (f *frame) bindIndex(b Box, value *int) {
	f.extra(b.i).bind = int32(len(f.binds)) + 1
	f.settleFn = (*frame).settleAt
	f.binds = append(f.binds, bindDecl{index: value})
}

// bindField makes Changed on the Field b report an edit of field.
func (f *frame) bindField(b Box, field *Field) {
	f.extra(b.i).bind = int32(len(f.binds)) + 1
	f.settleFn = (*frame).settleAt
	f.binds = append(f.binds, bindDecl{field: field})
}

// bindDone records that element b wrote its value where it was declared, and
// whether it changed it, for Changed.
func (f *frame) bindDone(b Box, changed bool) {
	f.extra(b.i).bind = int32(len(f.binds)) + 1
	f.binds = append(f.binds, bindDecl{settled: true, changed: changed})
	f.settleFn = (*frame).settleAt
}

// bound returns the binding of element i, or nil.
func (f *frame) bound(i int32) *bindDecl {
	n := &f.nodes[i]
	if n.ext == 0 || f.exts[n.ext].bind == 0 {
		return nil
	}
	return &f.binds[f.exts[n.ext].bind-1]
}

// settle writes what this paint delivers to element i into its bound value, once.
func (f *frame) settle(i int32) *bindDecl {
	d := f.bound(i)
	if d == nil || d.settled {
		return d
	}
	d.settled = true
	b := Box{f: f, i: i, gen: f.gen}
	n := &f.nodes[i]
	switch {
	case d.field != nil:
		d.changed = d.field.changed
	case n.flags&flagDisabled != 0 || f.canvas == nil:
	case d.flag != nil:
		if b.Clicked() {
			*d.flag = !*d.flag
			d.changed = true
		}
	case d.index != nil:
		var index int
		var ok bool
		switch n.kind {
		case kindSegmented:
			index, ok = b.ChosenIndex()
		case kindTabs:
			index, ok = b.TabClicked()
		case kindSelect:
			index, ok = b.Picked()
		}
		if ok && index != *d.index {
			*d.index = index
			d.changed = true
		}
	}
	return d
}

// settleAt settles element i, or with i < 0 every bound value not asked about yet,
// when the page ends.
func (f *frame) settleAt(i int32) *bindDecl {
	if i >= 0 {
		return f.settle(i)
	}
	f.settleBinds()
	return nil
}

// settleBinds writes every bound value not asked about yet.
func (f *frame) settleBinds() {
	for i := range f.nodes {
		if n := &f.nodes[i]; n.ext != 0 && f.exts[n.ext].bind != 0 {
			f.settle(int32(i))
		}
	}
}
