package raster

import (
	"bytes"
	"math/rand"
	"testing"

	"github.com/vivaui/viva/internal/render"
	"github.com/vivaui/viva/internal/scene"
)

func randomInstance(random *rand.Rand, width, height float32) render.Rectangle {
	x, y := random.Float32()*width, random.Float32()*height
	w, h := 1+random.Float32()*width/2, 1+random.Float32()*height/2
	color := scene.Color{Red: random.Float32(), Green: random.Float32(), Blue: random.Float32(), Alpha: 0.3 + 0.7*random.Float32()}
	instance := render.Rectangle{Bounds: scene.Rect{X: x, Y: y, Width: w, Height: h}, Color: color, Layer: uint16(random.Intn(3))}
	switch random.Intn(4) {
	case 0:
		instance.Radius = random.Float32() * 12
	case 1:
		instance.Kind = scene.KindShadow
		instance.Radius = random.Float32() * 8
		instance.Shadow = scene.Shadow{OffsetX: random.Float32()*6 - 3, OffsetY: random.Float32() * 6, Blur: 1 + random.Float32()*10, Spread: random.Float32()*4 - 1}
	case 2:
		instance.Kind = scene.KindTriangle
		instance.Points = [3]scene.Point{{X: x, Y: y}, {X: x + w, Y: y + h/3}, {X: x + w/4, Y: y + h}}
		instance.EdgeMask = uint8(random.Intn(8))
	}
	if random.Intn(4) == 0 {
		instance.Clipped = true
		instance.Clip = scene.Rect{X: x + w/4, Y: y + h/4, Width: w / 2, Height: h / 2}
	}
	return instance
}

// Redrawing only the Box of what changed gives the pixels a full redraw gives: this is what lets a
// presenter keep a damage region instead of repainting the window.
func TestRedrawingTheBoxOfAChangeEqualsRedrawingAll(t *testing.T) {
	const width, height = 96, 72
	for _, scale := range []float32{1, 1.25, 1.5, 2} {
		random := rand.New(rand.NewSource(int64(scale * 100)))
		for round := 0; round < 60; round++ {
			count := 3 + random.Intn(10)
			scene1 := make([]render.Rectangle, count)
			for i := range scene1 {
				scene1[i] = randomInstance(random, width, height)
			}
			scene2 := append([]render.Rectangle(nil), scene1...)
			var damage render.Scissor
			for _, index := range random.Perm(count)[:1+random.Intn(3)] {
				scene2[index] = randomInstance(random, width, height)
				for _, instance := range []*render.Rectangle{&scene1[index], &scene2[index]} {
					damage = union(damage, Box(instance, scale))
				}
			}
			size := func() (int, int) { return int(width * scale), int(height * scale) }
			w, h := size()
			partial, full := NewCanvas(w, h), NewCanvas(w, h)
			var drawer Rasterizer
			params := Params{Scale: scale, Clear: white}
			drawer.Draw(partial, params, scene1, nil)
			params.Damage = intersect(partial.Bounds(), damage)
			drawer.Draw(partial, params, scene2, nil)
			drawer.Draw(full, Params{Scale: scale, Clear: white}, scene2, nil)
			if !bytes.Equal(partial.Pix, full.Pix) {
				t.Fatalf("scale %g round %d: redrawing %+v differs from redrawing all", scale, round, damage)
			}
		}
	}
}

func union(a, b render.Scissor) render.Scissor {
	if empty(a) {
		return b
	}
	if empty(b) {
		return a
	}
	return render.Scissor{Left: min(a.Left, b.Left), Top: min(a.Top, b.Top), Right: max(a.Right, b.Right), Bottom: max(a.Bottom, b.Bottom)}
}

// Nothing an instance paints lies outside its Box, whatever the shape.
func TestAnInstancePaintsOnlyInsideItsBox(t *testing.T) {
	const width, height = 80, 60
	random := rand.New(rand.NewSource(7))
	for _, scale := range []float32{1, 1.5, 2} {
		w, h := int(width*scale), int(height*scale)
		blank := NewCanvas(w, h)
		var drawer Rasterizer
		drawer.Draw(blank, Params{Scale: scale, Clear: white}, nil, nil)
		for round := 0; round < 200; round++ {
			instance := randomInstance(random, width, height)
			instance.Clipped = false
			canvas := NewCanvas(w, h)
			drawer.Draw(canvas, Params{Scale: scale, Clear: white}, []render.Rectangle{instance}, nil)
			box := Box(&instance, scale)
			for y := 0; y < h; y++ {
				for x := 0; x < w; x++ {
					if !bytes.Equal(canvas.Pix[y*canvas.Stride+x*4:][:4], blank.Pix[y*blank.Stride+x*4:][:4]) &&
						(x < box.Left || x >= box.Right || y < box.Top || y >= box.Bottom) {
						t.Fatalf("scale %g: %+v changed pixel %d,%d outside its box %+v", scale, instance, x, y, box)
					}
				}
			}
		}
	}
}
