package base

import (
	"math"

	"github.com/vivaui/viva/gfx"
)

// Breakpoint is where a responsive change begins (From, AtLeast): a window size
// class, which measures the page's width, or Width(n), which measures the
// nearest Container's.
type Breakpoint uint32

// The window size classes (Material 3's window size classes), named by the
// devices they fit; each begins at its width in logical pixels.
const (
	Compact    Breakpoint = 0    // phones held upright
	Medium     Breakpoint = 600  // phones on their side, tablets held upright, narrow windows
	Expanded   Breakpoint = 840  // tablets on their side, most desktop windows
	Large      Breakpoint = 1200 // large desktop windows
	ExtraLarge Breakpoint = 1600 // wide screens
)

// containerQuery marks a Breakpoint that measures the nearest Container.
const containerQuery Breakpoint = 1 << 31

// Width is the breakpoint at which the nearest Container is n logical pixels
// wide inside its padding, as a CSS container query's min-width.
func Width(n float32) Breakpoint {
	return containerQuery | Breakpoint(min(math.Ceil(float64(max(n, 0))), float64(containerQuery-1)))
}

func (bp Breakpoint) minimum() float32 { return float32(bp &^ containerQuery) }

// SizeClass is the window size class the page's width is in: Compact, Medium,
// Expanded, Large or ExtraLarge.
func (c *Ctx) SizeClass() Breakpoint {
	width := c.f.Bounds().Width
	for _, class := range [...]Breakpoint{ExtraLarge, Large, Expanded, Medium} {
		if width >= class.minimum() {
			return class
		}
	}
	return Compact
}

// AtLeast reports whether the page, or for Width the nearest Container around
// what is declared next, is at least as wide as bp.
func (c *Ctx) AtLeast(bp Breakpoint) bool { return c.page.reaches(bp, c.f.Parent) }

// From applies change to the element when it is at least as wide as bp: the
// page, for a window size class, or the nearest Container around the element,
// for Width. Declare the narrowest form first and widen it with From, smallest
// breakpoint first, as mobile-first styles do; change uses the Box's own
// methods.
//
//	ui.Column(c).Gap(8).From(ui.Medium, func(b ui.Box) { b.Row().Gap(16) })
func (b Box) From(bp Breakpoint, change func(Box)) Box {
	if b.f.page.reaches(bp, b.N().Parent) {
		change(b)
	}
	return b
}

// Container is a Column that Width breakpoints inside it measure. Its width
// comes from where it is placed, never from what it holds (CSS's inline-size
// containment), so what its queries change cannot change the width they read.
func Container(c C) Box {
	b := Column(c)
	c.f.Extra(b.i).contained = true
	named := c.page.Nodes[b.i]
	id := c.page.ID(Place{Node: named.parentNode, scope: named.parentScope, Name: named.Name})
	c.page.containers = append(c.page.containers, containerWidth{node: b.i, id: id})
	return b
}

// containerWidth is a Container: its element this paint, its identity across
// paints, and its width inside its padding the last time it was laid out.
type containerWidth struct {
	node  int32
	id    gfx.ID
	width float32
}

// containerAnswer is a container query asked this paint and the answer it got,
// checked against the layout once it is done.
type containerAnswer struct {
	node    int32
	minimum float32
	yes     bool
}

// reaches answers bp for an element whose parent is parent.
func (p *Page) reaches(bp Breakpoint, parent int32) bool {
	if bp&containerQuery == 0 {
		return p.ctx.f.Bounds().Width >= bp.minimum()
	}
	f := p.ctx.f
	node := parent
	for node >= 0 && (f.Nodes[node].Ext == 0 || !f.Exts[f.Nodes[node].Ext].contained) {
		node = f.Nodes[node].Parent
	}
	if node < 0 {
		return false // no Container around it: nothing to measure
	}
	width, known := p.lastContainerWidth(p.containerID(node))
	yes := known && width >= bp.minimum()
	p.answers = append(p.answers, containerAnswer{node: node, minimum: bp.minimum(), yes: yes})
	return yes
}

// containerID is the identity of the Container that is element node.
func (p *Page) containerID(node int32) gfx.ID {
	for _, container := range p.containers {
		if container.node == node {
			return container.id
		}
	}
	return gfx.ID{}
}

func (p *Page) lastContainerWidth(id gfx.ID) (float32, bool) {
	for _, entry := range p.containerWidths {
		if entry.id == id {
			return entry.width, true
		}
	}
	return 0, false
}

// settleContainers keeps each Container's laid-out width for the next paint and
// reports whether a container query asked this paint got another answer than
// the layout gives: the paint is done again, which then answers from these
// widths. A Container's width does not depend on what it holds, so the second
// paint agrees.
func (p *Page) settleContainers() (again bool) {
	f := p.ctx.f
	p.containerWidths = p.containerWidths[:0]
	for _, container := range p.containers {
		n := &f.Nodes[container.node]
		container.width = max(n.Rect.Width-n.PadX(), 0)
		p.containerWidths = append(p.containerWidths, container)
	}
	for _, answer := range p.answers {
		n := &f.Nodes[answer.node]
		if (max(n.Rect.Width-n.PadX(), 0) >= answer.minimum) != answer.yes {
			again = true
		}
	}
	p.containers, p.answers = p.containers[:0], p.answers[:0]
	return again
}
