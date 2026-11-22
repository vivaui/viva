package base

import "slices"

// Hide takes the element out of the page: it and what it holds take no room, are
// not painted, take no pointer, keyboard or focus, and are not in the
// accessibility tree, as CSS's display: none. What it holds is still declared,
// so the controls in it keep their identities and state, and a responsive change
// shows it again:
//
//	ui.Text(c, host.Address).Hide().From(ui.Expanded, func(b ui.Box) { b.Show() })
func (b Box) Hide() Box {
	if b.i == 0 {
		panic("ui: the page's root cannot be hidden")
	}
	extra := b.f.Extra(b.i)
	if !extra.hidden {
		extra.hidden = true
		b.f.hidden = append(b.f.hidden, b.i)
	}
	return b
}

// Show undoes Hide.
func (b Box) Show() Box {
	if n := b.N(); n.Ext != 0 {
		b.f.Exts[n.Ext].hidden = false
	}
	return b
}

// detachHidden takes the elements still hidden out of their containers, or out
// of the popups, before the page is laid out: layout and paint walk the tree
// from the root, so a detached subtree is neither laid out nor painted, and so
// gives no hit.
func (f *Frame) detachHidden() {
	for _, i := range f.hidden {
		n := &f.Nodes[i]
		if n.Ext == 0 || !f.Exts[n.Ext].hidden {
			continue
		}
		if n.Parent < 0 {
			if at := slices.Index(f.Floats, i); at >= 0 {
				f.Floats = slices.Delete(f.Floats, at, at+1)
			}
			continue
		}
		parent := &f.Nodes[n.Parent]
		previous := int32(-1)
		for c := parent.First; c >= 0 && c != i; c = f.Nodes[c].Next {
			previous = c
		}
		if previous < 0 {
			parent.First = n.Next
		} else {
			f.Nodes[previous].Next = n.Next
		}
		if parent.Last == i {
			parent.Last = previous
		}
		n.Next = -1
	}
}
