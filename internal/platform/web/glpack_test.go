package web

import (
	"reflect"
	"testing"

	"github.com/vivaui/viva/internal/render"
	"github.com/vivaui/viva/internal/scene"
)

func TestTileRangesCoverTheWidthWithOverlap(t *testing.T) {
	got := tileRanges(100, 40)
	want := [][2]int{{0, 40}, {38, 78}, {76, 100}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("tile ranges = %v, want %v", got, want)
	}
	if single := tileRanges(30, 40); !reflect.DeepEqual(single, [][2]int{{0, 30}}) {
		t.Fatalf("a label inside one texture = %v", single)
	}
	if empty := tileRanges(0, 40); len(empty) != 0 {
		t.Fatalf("empty label = %v", empty)
	}
}

func TestPackInstanceRectangle(t *testing.T) {
	dst := make([]float32, 20)
	packInstance(dst, render.Rectangle{
		Kind:   scene.KindRectangle,
		Bounds: scene.Rect{X: 10, Y: 20, Width: 30, Height: 40},
		Color:  scene.Color{Red: 1, Green: 2, Blue: 3, Alpha: 4},
		Radius: 6,
		Layer:  2,
	})
	want := make([]float32, 20)
	want[0], want[1], want[2], want[3] = 10, 20, 30, 40
	want[4], want[5], want[6], want[7] = 1, 2, 3, 4
	want[8] = 6
	if !reflect.DeepEqual(dst, want) {
		t.Fatalf("packed rectangle = %v, want %v", dst, want)
	}
}

func TestPackInstanceTriangle(t *testing.T) {
	dst := make([]float32, 20)
	packInstance(dst, render.Rectangle{
		Kind:     scene.KindTriangle,
		Points:   [3]scene.Point{{X: 1, Y: 2}, {X: 3, Y: 4}, {X: 5, Y: 6}},
		Color:    scene.Color{Red: 0.1, Green: 0.2, Blue: 0.3, Alpha: 1},
		Shade:    scene.Shade{On: true, Colors: [2]scene.Color{{Red: 0.4, Green: 0.5, Blue: 0.6, Alpha: 1}, {Red: 0.7, Green: 0.8, Blue: 0.9, Alpha: 1}}},
		EdgeMask: 5,
		Layer:    7,
	})
	want := make([]float32, 20)
	want[0], want[1], want[2], want[3] = 1, 2, 3, 4
	want[4], want[5], want[6], want[7] = 0.1, 0.2, 0.3, 1
	want[8], want[9], want[10], want[11] = 5, 7, 6, 6
	want[12], want[13], want[14], want[15] = 0.4, 0.5, 0.6, 1
	want[16], want[17], want[18], want[19] = 0.7, 0.8, 0.9, 1
	if !reflect.DeepEqual(dst, want) {
		t.Fatalf("packed triangle = %v, want %v", dst, want)
	}
}

func TestPackInstanceShadow(t *testing.T) {
	dst := make([]float32, 20)
	packInstance(dst, render.Rectangle{
		Kind:   scene.KindShadow,
		Bounds: scene.Rect{X: 1, Y: 2, Width: 3, Height: 4},
		Color:  scene.Color{Red: 0, Green: 0, Blue: 0, Alpha: 0.5},
		Radius: 5,
		Shadow: scene.Shadow{OffsetX: 6, OffsetY: 7, Blur: 8, Spread: 9},
	})
	want := make([]float32, 20)
	want[0], want[1], want[2], want[3] = 1, 2, 3, 4
	want[4], want[5], want[6], want[7] = 0, 0, 0, 0.5
	want[8], want[9], want[10], want[11] = 5, 0, 8, 0
	want[12], want[13], want[14], want[15] = 6, 7, 9, 0
	if !reflect.DeepEqual(dst, want) {
		t.Fatalf("packed shadow = %v, want %v", dst, want)
	}
}
