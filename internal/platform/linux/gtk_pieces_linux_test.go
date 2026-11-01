//go:build linux && vivagtk

package linux

import "testing"

func TestOverlappingBoxesBecomeOnePiece(t *testing.T) {
	var boxes []box
	boxes = addBox(boxes, box{0, 0, 10, 10})
	boxes = addBox(boxes, box{20, 20, 30, 30})
	boxes = addBox(boxes, box{8, 8, 22, 22}) // bridges the two
	if len(boxes) != 1 || boxes[0] != (box{0, 0, 30, 30}) {
		t.Fatalf("boxes = %v, want one covering 0,0-30,30", boxes)
	}
	boxes = addBox(boxes, box{40, 0, 50, 5})
	if len(boxes) != 2 {
		t.Fatalf("a box apart from the rest joined it: %v", boxes)
	}
}
