package render

import (
	"reflect"
	"testing"

	"github.com/vivaui/viva/internal/scene"
)

func TestPlanWithoutLabelsIsTheBatches(t *testing.T) {
	batches := []Batch{
		{Layer: 0, First: 0, Count: 3},
		{Layer: 1, Clipped: true, Clip: scene.Rect{X: 1, Y: 2, Width: 3, Height: 4}, First: 3, Count: 2},
	}
	got := Plan(nil, batches, nil)
	want := []PlanOp{
		{Kind: PlanShapes, Layer: 0, First: 0, Count: 3},
		{Kind: PlanShapes, Layer: 1, Clipped: true, Clip: scene.Rect{X: 1, Y: 2, Width: 3, Height: 4}, First: 3, Count: 2},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("plan = %#v, want %#v", got, want)
	}
}

func TestPlanInterleavesLabelsByShapesBefore(t *testing.T) {
	batches := []Batch{{Layer: 0, First: 0, Count: 5}}
	labels := []LabelOrder{
		{Layer: 0, ShapesBefore: 2},
		{Layer: 0, ShapesBefore: 0},
		{Layer: 0, ShapesBefore: 5},
	}
	got := Plan(nil, batches, labels)
	want := []PlanOp{
		{Kind: PlanLabels, Layer: 0, Label: 1},
		{Kind: PlanShapes, Layer: 0, First: 0, Count: 2},
		{Kind: PlanLabels, Layer: 0, Label: 0},
		{Kind: PlanShapes, Layer: 0, First: 2, Count: 3},
		{Kind: PlanLabels, Layer: 0, Label: 2},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("plan = %#v, want %#v", got, want)
	}
}

func TestPlanOrdersLayersAndKeepsLabelIndices(t *testing.T) {
	batches := []Batch{
		{Layer: 0, First: 0, Count: 2},
		{Layer: 1, First: 2, Count: 2},
	}
	labels := []LabelOrder{
		{Layer: 1, ShapesBefore: 0},
		{Layer: 0, ShapesBefore: 1},
	}
	got := Plan(nil, batches, labels)
	want := []PlanOp{
		{Kind: PlanShapes, Layer: 0, First: 0, Count: 1},
		{Kind: PlanLabels, Layer: 0, Label: 1},
		{Kind: PlanShapes, Layer: 0, First: 1, Count: 1},
		{Kind: PlanLabels, Layer: 1, Label: 0},
		{Kind: PlanShapes, Layer: 1, First: 2, Count: 2},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("plan = %#v, want %#v", got, want)
	}
}

func TestPlanHandlesShapesBeforePastTheEnd(t *testing.T) {
	batches := []Batch{{Layer: 0, First: 0, Count: 2}}
	labels := []LabelOrder{{Layer: 0, ShapesBefore: 10}}
	got := Plan(nil, batches, labels)
	want := []PlanOp{
		{Kind: PlanShapes, Layer: 0, First: 0, Count: 2},
		{Kind: PlanLabels, Layer: 0, Label: 0},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("plan = %#v, want %#v", got, want)
	}
}

func TestPlanWithOrderReusesTheScratch(t *testing.T) {
	order := make([]int, 0, 2)
	batches := []Batch{{Layer: 0, First: 0, Count: 1}}
	labels := []LabelOrder{{Layer: 0, ShapesBefore: 0}, {Layer: 2, ShapesBefore: 0}}
	got, order := PlanWithOrder(nil, order, batches, labels)
	if len(order) != 2 || cap(order) < 2 {
		t.Fatalf("order = %v, want two entries retained", order)
	}
	if len(got) != 3 {
		t.Fatalf("plan = %#v, want three ops", got)
	}
}

func TestPlanOfNothingIsEmpty(t *testing.T) {
	if got := Plan(nil, nil, nil); len(got) != 0 {
		t.Fatalf("plan = %#v, want empty", got)
	}
}
