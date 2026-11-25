package base

import "github.com/vivaui/viva/gfx"

// FileDropState is one area of a page that takes files dragged from the
// system: whether files are over it, and the files dropped on it. Keep one in
// the model, next to the state the drop changes, and declare the area every
// paint:
//
//	ui.Column(c).FillWidth().DropFiles(&m.drop)
//	// in the page, after the area:
//	if m.drop.Over() { /* paint the highlight */ }
//	if files, paths := m.drop.Dropped(); len(files) > 0 { /* take them */ }
//
// The engine owns the drag's position: while files are over the area, Over is
// true and the system shows the drop as accepted; leaving the area, or the
// drop, ends it. Dropped returns the files of a drop once, then nothing until
// the next drop. The areas of a page are tested from the last one declared, the
// one painted on top; an area that covers another takes the files. A drop on
// no area is left to the window's OnFileDrop.
//
// The files are the window's (gfx.FileDropEvent): Files read the same way on
// every platform, and Paths are absolute file system paths on a native target
// and empty in a browser. Do not declare an area inside a Memo: a copied area
// would not declare it again. One state is one area.
type FileDropState struct {
	frame *Frame
	gen   uint32
	epoch uint32 // the page paint this state was registered for

	node     int32
	declared bool
	over     bool
	files    []gfx.FileDropFile
	paths    []string
}

// DropFiles makes the element an area for state that takes files dragged from
// the system. It does not need a Hit identity: the area is the element's
// rectangle, including HitOutset.
func (b Box) DropFiles(state *FileDropState) Box {
	if state == nil {
		panic("ui: DropFiles needs its FileDropState")
	}
	p := b.f.page
	if p == nil {
		panic("ui: DropFiles is for a page's element; a view surface has no window to drop on")
	}
	p.ForbidInMemo("DropFiles")
	state.begin(b.f)
	state.node, state.declared = b.i, true
	return b
}

// Over reports whether files dragged from the system are over the area.
func (s *FileDropState) Over() bool { return s.over }

// Dropped returns the files dropped on the area, and their paths where the
// platform has them, and forgets them: it returns nothing until the next drop.
// Files that are folders have IsDir set and cannot be read.
func (s *FileDropState) Dropped() (files []gfx.FileDropFile, paths []string) {
	files, paths = s.files, s.paths
	s.files, s.paths = nil, nil
	return files, paths
}

// begin records the frame a paint declares the state in, and adds the state to
// the page that routes the files. The first declaration of a paint clears the
// last one's.
func (s *FileDropState) begin(f *Frame) {
	if s.frame == f && s.gen == f.gen {
		return
	}
	s.frame, s.gen, s.declared = f, f.gen, false
	if p := f.page; p != nil && s.epoch != p.epoch {
		s.epoch = p.epoch
		p.fileDrops = append(p.fileDrops, s)
	}
}

// contains reports whether the point is over the declared area.
func (s *FileDropState) contains(x, y float32) bool {
	if !s.declared || s.frame == nil || int(s.node) >= len(s.frame.Nodes) {
		return false
	}
	n := &s.frame.Nodes[s.node]
	r := n.Rect
	if n.Ext != 0 {
		h := s.frame.Exts[n.Ext].hitOut
		r = Rect{X: r.X - h[3], Y: r.Y - h[0], Width: r.Width + h[1] + h[3], Height: r.Height + h[0] + h[2]}
	}
	return pointInRect(x, y, r)
}

// onFileDrop gives one step of a drag of files to the area under the point,
// then to the host's own OnFileDrop when no area took it.
func (p *Page) onFileDrop(event gfx.FileDropEvent) gfx.EventResult {
	var result gfx.EventResult
	var taken *FileDropState
	if event.Phase != gfx.FileDropLeave {
		for i := len(p.fileDrops) - 1; i >= 0; i-- {
			if s := p.fileDrops[i]; s.contains(event.X, event.Y) {
				taken = s
				break
			}
		}
	}
	for _, s := range p.fileDrops {
		over := s == taken && event.Phase == gfx.FileDropOver
		if s.over != over {
			s.over, result.Repaint = over, true
		}
	}
	if taken != nil {
		result.Handled = true
		if event.Phase == gfx.FileDropDrop {
			taken.files, taken.paths = event.Files, event.Paths
			result.Repaint = true
		}
		return result
	}
	if p.baseFileDrop != nil {
		other := p.baseFileDrop(event)
		other.Repaint = other.Repaint || result.Repaint
		return other
	}
	return result
}
