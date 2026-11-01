package raster

import (
	"math"
	"slices"

	"github.com/vivaui/viva/internal/render"
	"github.com/vivaui/viva/internal/scene"
)

// Label is a prepared image placed in the canvas: text the platform's text engine rasterized, or
// a bitmap, already at device resolution. Pix is premultiplied BGRA like the canvas.
type Label struct {
	Pix    []byte
	Stride int // bytes per row of Pix
	Width  int // pixels per row
	Height int
	// X and Y are the device pixel of the image's top-left corner in the canvas.
	X, Y  int
	Layer uint16
	// Clip, when Clipped, is the part of the canvas the image may touch (device pixels).
	Clipped bool
	Clip    render.Scissor
}

// Hole is a part of the canvas that stays transparent for something else to show through: a native
// view the window hosts. Everything of its layer and the layers below is erased inside Box, and the
// layers above are drawn over it, which is how a popover stands over a page.
type Hole struct {
	Box   render.Scissor // device pixels
	Layer uint16
}

// Params says how to draw one frame.
type Params struct {
	// Scale is device pixels per logical pixel (1.5 at 150 percent). Zero means 1.
	Scale float32
	// Clear is written, as it is and without blending, over the damaged part of the canvas before
	// anything is drawn: straight channel values of the target's byte order are the caller's to
	// choose, premultiplied when the target is composited by the system.
	Clear scene.Color
	// Holes are the parts that end up transparent (see Hole).
	Holes []Hole
	// Damage is the part of the canvas to redraw, in device pixels. The zero value redraws all of it.
	// Pixels outside it are not read or written.
	Damage render.Scissor
}

// Rasterizer draws frames. It holds scratch lists so that a steady stream of frames allocates
// nothing; one Rasterizer must not be used from two goroutines at once.
type Rasterizer struct {
	batches []render.Batch
	order   []int // label indexes, by layer
	holes   []Hole
}

// Draw paints the damaged part of c: clear, then for each layer in ascending order the layer's
// rectangle instances in paint order and after them its labels in list order. This is the order
// the GPU backends use, so a popover layer covers the page layer whatever the list order.
func (r *Rasterizer) Draw(c *Canvas, p Params, rects []render.Rectangle, labels []Label) {
	if c == nil || c.Width <= 0 || c.Height <= 0 {
		return
	}
	scale := p.Scale
	if !(scale > 0) {
		scale = 1
	}
	damage := c.Bounds()
	if p.Damage != (render.Scissor{}) {
		damage = intersect(damage, p.Damage)
	}
	if empty(damage) {
		return
	}
	c.clearRegion(damage, p.Clear)

	r.batches = render.GroupBatches(r.batches, rects)
	r.order = r.order[:0]
	for i := range labels {
		r.order = append(r.order, i)
	}
	slices.SortStableFunc(r.order, func(a, b int) int { return int(labels[a].Layer) - int(labels[b].Layer) })

	bi, li, hi := 0, 0, 0
	r.holes = append(r.holes[:0], p.Holes...)
	slices.SortStableFunc(r.holes, func(a, b Hole) int { return int(a.Layer) - int(b.Layer) })
	holes := r.holes
	for bi < len(r.batches) || li < len(r.order) || hi < len(holes) {
		layer := uint16(math.MaxUint16)
		if bi < len(r.batches) {
			layer = r.batches[bi].Layer
		}
		if li < len(r.order) && labels[r.order[li]].Layer < layer {
			layer = labels[r.order[li]].Layer
		}
		if hi < len(holes) && holes[hi].Layer < layer {
			layer = holes[hi].Layer
		}
		for ; bi < len(r.batches) && r.batches[bi].Layer == layer; bi++ {
			batch := r.batches[bi]
			area := damage
			if batch.Clipped {
				clip, visible := render.ScissorOf(batch.Clip, scale, c.Width, c.Height)
				if !visible {
					continue
				}
				area = intersect(area, clip)
			}
			if empty(area) {
				continue
			}
			for k := batch.First; k < batch.First+batch.Count; k++ {
				drawInstance(c, area, &rects[k], scale)
			}
		}
		for ; li < len(r.order) && labels[r.order[li]].Layer == layer; li++ {
			drawLabel(c, damage, &labels[r.order[li]])
		}
		for ; hi < len(holes) && holes[hi].Layer == layer; hi++ {
			if box := intersect(damage, holes[hi].Box); !empty(box) {
				c.clearRegion(box, scene.Color{})
			}
		}
	}
}

func (c *Canvas) clearRegion(s render.Scissor, color scene.Color) {
	px := [4]byte{unorm8(color.Blue), unorm8(color.Green), unorm8(color.Red), unorm8(color.Alpha)}
	row := c.Pix[s.Top*c.Stride+s.Left*4 : s.Top*c.Stride+s.Right*4]
	for i := 0; i < len(row); i += 4 {
		copy(row[i:i+4], px[:])
	}
	for y := s.Top + 1; y < s.Bottom; y++ {
		copy(c.Pix[y*c.Stride+s.Left*4:], row)
	}
}

func drawLabel(c *Canvas, damage render.Scissor, l *Label) {
	if l.Width <= 0 || l.Height <= 0 || len(l.Pix) < (l.Height-1)*l.Stride+l.Width*4 {
		return
	}
	area := intersect(damage, render.Scissor{Left: l.X, Top: l.Y, Right: l.X + l.Width, Bottom: l.Y + l.Height})
	if l.Clipped {
		area = intersect(area, l.Clip)
	}
	if empty(area) {
		return
	}
	const inv = 1.0 / 255
	for y := area.Top; y < area.Bottom; y++ {
		src := l.Pix[(y-l.Y)*l.Stride+(area.Left-l.X)*4:]
		di := y*c.Stride + area.Left*4
		for x := area.Left; x < area.Right; x++ {
			a := src[3]
			switch a {
			case 0:
			case 255:
				c.Pix[di], c.Pix[di+1], c.Pix[di+2], c.Pix[di+3] = src[0], src[1], src[2], 255
			default:
				k := 1 - float32(a)*inv
				c.Pix[di] = unorm8(float32(src[0])*inv + float32(c.Pix[di])*inv*k)
				c.Pix[di+1] = unorm8(float32(src[1])*inv + float32(c.Pix[di+1])*inv*k)
				c.Pix[di+2] = unorm8(float32(src[2])*inv + float32(c.Pix[di+2])*inv*k)
				c.Pix[di+3] = unorm8(float32(a)*inv + float32(c.Pix[di+3])*inv*k)
			}
			src = src[4:]
			di += 4
		}
	}
}

// drawInstance paints one scene instance, cut to area.
func drawInstance(c *Canvas, area render.Scissor, rc *render.Rectangle, scale float32) {
	switch rc.Kind {
	case scene.KindTriangle:
		drawTriangle(c, area, rc, scale)
	case scene.KindShadow:
		if rc.Shadow.Blur > 0 {
			drawShadow(c, area, rc, scale)
			return
		}
		drawRectangle(c, area, rc, scale) // a shadow without blur is its casting rectangle
	default:
		drawRectangle(c, area, rc, scale)
	}
}

// pixelSpan is the range of pixel indexes [first, last) whose centers lie in [lo, hi) logical
// pixels, with the edges first snapped to a 1/256 device pixel grid as GPU rasterizers snap
// vertices, so that a shared edge is owned by exactly one side.
func pixelSpan(lo, hi, scale float32) (first, last int) {
	snap := func(v float32) float64 { return math.Floor(float64(v)*float64(scale)*256+0.5) / 256 }
	return int(math.Ceil(snap(lo) - 0.5)), int(math.Ceil(snap(hi) - 0.5))
}
