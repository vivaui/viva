package base

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
func (f *Frame) bindBool(b Box, Value *bool) {
	f.Extra(b.i).bind = int32(len(f.Binds)) + 1
	f.settleFn = (*Frame).settleAt
	f.Binds = append(f.Binds, bindDecl{flag: Value})
}

// bindIndex makes the Choice, Tabs or Select b set value to the option chosen.
func (f *Frame) bindIndex(b Box, Value *int) {
	f.Extra(b.i).bind = int32(len(f.Binds)) + 1
	f.settleFn = (*Frame).settleAt
	f.Binds = append(f.Binds, bindDecl{index: Value})
}

// bindField makes Changed on the Field b report an edit of field.
func (f *Frame) bindField(b Box, field *Field) {
	f.Extra(b.i).bind = int32(len(f.Binds)) + 1
	f.settleFn = (*Frame).settleAt
	f.Binds = append(f.Binds, bindDecl{field: field})
}

// bindDone records that element b wrote its value where it was declared, and
// whether it changed it, for Changed.
func (f *Frame) BindDone(b Box, changed bool) {
	f.Extra(b.i).bind = int32(len(f.Binds)) + 1
	f.Binds = append(f.Binds, bindDecl{settled: true, changed: changed})
	f.settleFn = (*Frame).settleAt
}

// bound returns the binding of element i, or nil.
func (f *Frame) Bound(i int32) *bindDecl {
	n := &f.Nodes[i]
	if n.Ext == 0 || f.Exts[n.Ext].bind == 0 {
		return nil
	}
	return &f.Binds[f.Exts[n.Ext].bind-1]
}

// settle writes what this paint delivers to element i into its bound value, once.
func (f *Frame) settle(i int32) *bindDecl {
	d := f.Bound(i)
	if d == nil || d.settled {
		return d
	}
	d.settled = true
	b := Box{f: f, i: i, gen: f.gen}
	n := &f.Nodes[i]
	switch {
	case d.field != nil:
		d.changed = d.field.changed
	case n.Flags&flagDisabled != 0 || f.Surface == nil:
	case d.flag != nil:
		if b.Clicked() {
			*d.flag = !*d.flag
			d.changed = true
		}
	case d.index != nil:
		var index int
		var ok bool
		switch n.Kind {
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
func (f *Frame) settleAt(i int32) *bindDecl {
	if i >= 0 {
		return f.settle(i)
	}
	f.settleBinds()
	return nil
}

// settleBinds writes every bound value not asked about yet.
func (f *Frame) settleBinds() {
	for i := range f.Nodes {
		if n := &f.Nodes[i]; n.Ext != 0 && f.Exts[n.Ext].bind != 0 {
			f.settle(int32(i))
		}
	}
}
