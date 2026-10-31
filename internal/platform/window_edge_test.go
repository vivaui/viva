package platform

import "testing"

func TestEdgeAtFindsEdgesAndCornersAndLeavesTheInteriorAlone(t *testing.T) {
	for _, test := range []struct {
		name string
		x, y float32
		want WindowEdge
	}{
		{"top left corner", 2, 2, EdgeTopLeft},
		{"top right corner", 797, 2, EdgeTopRight},
		{"bottom left corner", 2, 597, EdgeBottomLeft},
		{"bottom right corner", 797, 597, EdgeBottomRight},
		{"left", 2, 300, EdgeLeft},
		{"right", 797, 300, EdgeRight},
		{"top", 400, 2, EdgeTop},
		{"bottom", 400, 597, EdgeBottom},
		{"left edge near the top is a corner", 2, 10, EdgeTopLeft},
		{"top edge near the corner is a corner", 10, 2, EdgeTopLeft},
		{"interior", 400, 300, EdgeNone},
		{"just inside the border", 6, 300, EdgeNone},
		{"fractional position on the border", 5.9, 300, EdgeLeft},
	} {
		if got := EdgeAt(test.x, test.y, 800, 600, false); got != test.want {
			t.Errorf("%s: got %d, want %d", test.name, got, test.want)
		}
	}
}

func TestTheTopEdgeYieldsToAControlButTheSidesAndBottomDoNot(t *testing.T) {
	for _, test := range []struct {
		name string
		x, y float32
		want WindowEdge
	}{
		{"top under a control", 400, 2, EdgeNone},
		{"top right corner under a control", 790, 2, EdgeNone},
		{"right edge beside a control", 799, 300, EdgeRight},
		{"right edge below the corner zone", 799, 30, EdgeRight},
		{"right edge in the corner zone", 799, 10, EdgeTopRight},
		{"bottom", 400, 599, EdgeBottom},
		{"outer grip above a control does not yield", 400, -2, EdgeTop},
		{"outer top-left grip above a control does not yield", -2, -2, EdgeTopLeft},
	} {
		if got := EdgeAt(test.x, test.y, 800, 600, true); got != test.want {
			t.Errorf("%s: got %d, want %d", test.name, got, test.want)
		}
	}
}

func TestEdgeAtOuterGripZones(t *testing.T) {
	for _, test := range []struct {
		name string
		x, y float32
		want WindowEdge
	}{
		{"outer top margin", 400, -4, EdgeTop},
		{"outer bottom margin", 400, 604, EdgeBottom},
		{"outer left margin", -4, 300, EdgeLeft},
		{"outer right margin", 804, 300, EdgeRight},
		{"outer top-left diagonal", -4, -4, EdgeTopLeft},
		{"outer top-right diagonal", 804, -4, EdgeTopRight},
		{"outer bottom-left diagonal", -4, 604, EdgeBottomLeft},
		{"outer bottom-right diagonal", 804, 604, EdgeBottomRight},
		{"outer left near top corner reach", -4, 10, EdgeTopLeft},
		{"outer top near left corner reach", 10, -4, EdgeTopLeft},
		{"outer right near bottom corner reach", 804, 590, EdgeBottomRight},
		{"degenerate width/height", 10, 10, EdgeNone},
	} {
		w, h := float32(800), float32(600)
		if test.name == "degenerate width/height" {
			w, h = 0, 0
		}
		if got := EdgeAt(test.x, test.y, w, h, false); got != test.want {
			t.Errorf("%s: got %d, want %d", test.name, got, test.want)
		}
	}
}

func TestEdgesRunClockwiseFromTheTopLeftLikeXMoveResizeDirections(t *testing.T) {
	// _NET_WM_MOVERESIZE: 0 top left, 1 top, 2 top right, 3 right, 4 bottom
	// right, 5 bottom, 6 bottom left, 7 left.
	for direction, edge := range []WindowEdge{EdgeTopLeft, EdgeTop, EdgeTopRight, EdgeRight, EdgeBottomRight, EdgeBottom, EdgeBottomLeft, EdgeLeft} {
		if int(edge)-1 != direction {
			t.Errorf("edge %d is direction %d, want %d", edge, int(edge)-1, direction)
		}
	}
}

func BenchmarkEdgeAt(b *testing.B) {
	b.ReportAllocs()
	var edge WindowEdge
	for i := 0; b.Loop(); i++ {
		edge = EdgeAt(float32(i%800), float32(i%600), 800, 600, i%3 == 0)
	}
	_ = edge
}
